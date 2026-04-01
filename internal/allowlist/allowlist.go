// Package allowlist enforces a deny-by-default command allowlist for the SentinelEdge Agent.
//
// Security model:
//   - Only explicitly approved command types and payloads may execute.
//   - PowerShell payloads are validated against two layers:
//     1. SHA256 hash registry (pre-approved scripts, hash registered at startup).
//     2. Pattern allowlist (anchored regex for known-safe single-line commands).
//   - Any command that does not match either layer is rejected with a clear error.
//   - Unknown command types are rejected by default.
//
// This prevents arbitrary code execution even if the API server or transport layer
// is compromised and sends a malicious payload to a legitimate agent token.
package allowlist

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// approvedTypes lists command types that have fixed, hardcoded behavior in the executor.
// These types do not carry a user-supplied payload that reaches a shell, so they are safe.
var approvedTypes = map[string]bool{
	"update":    true,
	"inventory": true,
}

// approvedPatterns contains anchored regex patterns that describe safe, read-only or
// narrowly-scoped PowerShell commands used in MSP operations.
//
// Design rules for adding patterns:
//   - Must be anchored (matched against the full trimmed payload, not a substring).
//   - Must not allow pipes, semicolons, backticks, $(), &, |, >, <, or other shell
//     metacharacters that could chain additional commands.
//   - Prefer read-only Get-* commands; mutating commands (Restart-Service, etc.) must
//     be restricted to a specific service name character class [\w\-].
var approvedPatterns = []*regexp.Regexp{
	// --- Service management (restricted to alphanumeric/dash service names) ---
	regexp.MustCompile(`(?i)^Get-Service\s+[\w\-]+$`),
	regexp.MustCompile(`(?i)^Get-Service$`),
	regexp.MustCompile(`(?i)^Restart-Service\s+[\w\-]+(\s+-Force)?$`),
	regexp.MustCompile(`(?i)^Stop-Service\s+[\w\-]+(\s+-Force)?$`),
	regexp.MustCompile(`(?i)^Start-Service\s+[\w\-]+$`),

	// --- System information (read-only) ---
	regexp.MustCompile(`(?i)^Get-ComputerInfo$`),
	regexp.MustCompile(`(?i)^Get-ComputerInfo\s+-Property\s+[\w,\s]+$`),
	regexp.MustCompile(`(?i)^Get-Disk$`),
	regexp.MustCompile(`(?i)^Get-Volume$`),
	regexp.MustCompile(`(?i)^Get-Process$`),
	regexp.MustCompile(`(?i)^Get-WmiObject\s+Win32_OperatingSystem$`),
	regexp.MustCompile(`(?i)^Get-WmiObject\s+Win32_Processor$`),
	regexp.MustCompile(`(?i)^Get-WmiObject\s+Win32_PhysicalMemory$`),
	regexp.MustCompile(`(?i)^Get-WmiObject\s+Win32_LogicalDisk$`),
	regexp.MustCompile(`(?i)^\[System\.Environment\]::OSVersion\.VersionString$`),

	// --- Event log (limited to known log names, capped entry count) ---
	regexp.MustCompile(`(?i)^Get-EventLog\s+-LogName\s+(Application|System|Security)\s+-Newest\s+\d{1,4}$`),

	// --- Network diagnostics (read-only) ---
	regexp.MustCompile(`(?i)^Test-Connection\s+[\w\-\.]+(\s+-Count\s+\d+)?(\s+-Quiet)?$`),
	regexp.MustCompile(`(?i)^Get-NetAdapter$`),
	regexp.MustCompile(`(?i)^Get-NetIPAddress$`),
	regexp.MustCompile(`(?i)^Resolve-DnsName\s+[\w\-\.]+$`),

	// --- Windows Update check (read-only count query) ---
	regexp.MustCompile(`(?i)^\(New-Object\s+-ComObject\s+Microsoft\.Update\.Session\)\.CreateUpdateSearcher\(\)\.Search\("IsInstalled=0"\)\.Updates\.Count$`),

	// --- Environment info ---
	regexp.MustCompile(`(?i)^\[System\.Environment\]::GetEnvironmentVariable\("[\w]+"\)$`),
	regexp.MustCompile(`(?i)^Get-Date$`),
	regexp.MustCompile(`(?i)^\$PSVersionTable\.PSVersion$`),
}

// approvedHashes maps SHA256 hex digests (of trimmed payload strings) to a human-readable
// description of the approved script. This allows complex multi-line scripts to be
// pre-approved without writing a regex for each one.
//
// Hashes are populated at agent startup via RegisterApprovedScript().
// The registry is intentionally empty by default — operators must explicitly register scripts.
//
// approvedHashesMu protects concurrent access to approvedHashes (FINDING-03).
var (
	approvedHashes   = map[string]string{}
	approvedHashesMu sync.RWMutex
)

// ErrCommandRejected is returned when a command does not match the allowlist.
type ErrCommandRejected struct {
	Type    string
	Reason  string
}

func (e *ErrCommandRejected) Error() string {
	return fmt.Sprintf("command type=%q rejected by allowlist: %s", e.Type, e.Reason)
}

// Validate checks whether a command (type + payload) is permitted to execute.
// Returns nil if the command is allowed, or an *ErrCommandRejected describing the rejection.
//
// Call this before any exec.Command invocation. A rejected command must NOT be executed.
func Validate(cmdType, payload string) error {
	// Fixed-behavior types: payload is never passed to a shell.
	if approvedTypes[cmdType] {
		return nil
	}

	if cmdType == "powershell" {
		trimmed := strings.TrimSpace(payload)

		if trimmed == "" {
			return &ErrCommandRejected{Type: cmdType, Reason: "empty payload"}
		}

		// Layer 1: SHA256 hash registry (pre-approved complex scripts).
		hash := hashString(trimmed)
		approvedHashesMu.RLock()
		desc, ok := approvedHashes[hash]
		approvedHashesMu.RUnlock()
		if ok {
			_ = desc // hash matched; description is for audit logging only
			return nil
		}

		// Layer 2: Pattern allowlist (safe single-line commands).
		for _, pattern := range approvedPatterns {
			if pattern.MatchString(trimmed) {
				return nil
			}
		}

		return &ErrCommandRejected{
			Type:   cmdType,
			Reason: "payload does not match any approved pattern or registered hash",
		}
	}

	// All other types (bash, cmd, shell, etc.) are denied by default on Windows agents.
	return &ErrCommandRejected{
		Type:   cmdType,
		Reason: "command type is not permitted on this agent",
	}
}

// RegisterApprovedScript computes the SHA256 hash of the given script payload and adds it
// to the approved-hashes registry with the provided description.
//
// This should be called during agent startup for each script that is known to be safe
// but cannot be expressed as a simple pattern. The description is logged on hash match
// to provide an audit trail of which approved script ran.
//
// Example:
//
//	allowlist.RegisterApprovedScript(
//	    "Get-WindowsUpdateHistory | ...",
//	    "windows-update-history-report",
//	)
func RegisterApprovedScript(payload, description string) {
	hash := hashString(strings.TrimSpace(payload))
	approvedHashesMu.Lock()
	defer approvedHashesMu.Unlock()
	approvedHashes[hash] = description
}

// HashPayload returns the SHA256 hex digest of a payload string (trimmed).
// Use this when generating hashes for RegisterApprovedScript or for offline approval workflows.
func HashPayload(payload string) string {
	return hashString(strings.TrimSpace(payload))
}

func hashString(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
