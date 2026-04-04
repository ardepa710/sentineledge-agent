package executor

import (
	"strings"
	"testing"

	"github.com/sentineledge/agent/pkg/models"
)

// TestExecute_AllowlistRejectedType verifies that commands with unknown types
// are blocked by the allowlist before any execution occurs.
func TestExecute_AllowlistRejectedType(t *testing.T) {
	cmd := models.Command{ID: "job-001", Type: "bash", Payload: "ls"}
	result := Execute(cmd)

	if result.ExitCode != 1 {
		t.Errorf("expected ExitCode=1 for rejected type, got %d", result.ExitCode)
	}
	if result.Error != "command_not_allowed" {
		t.Errorf("expected Error='command_not_allowed', got %q", result.Error)
	}
	if result.FinishedAt.IsZero() {
		t.Error("FinishedAt should be set even for rejected commands")
	}
}

// TestExecute_EmptyPowershellPayloadRejected verifies that an empty PowerShell
// payload is blocked by the allowlist.
func TestExecute_EmptyPowershellPayloadRejected(t *testing.T) {
	cmd := models.Command{ID: "job-002", Type: "powershell", Payload: ""}
	result := Execute(cmd)

	if result.ExitCode != 1 {
		t.Errorf("expected ExitCode=1 for empty payload, got %d", result.ExitCode)
	}
	if result.Error != "command_not_allowed" {
		t.Errorf("expected Error='command_not_allowed', got %q", result.Error)
	}
}

// TestExecute_AllowlistedSingleLineCommand verifies that a safe pattern-matched
// PowerShell command is allowed through the allowlist and executed.
// On Linux this will fail at execution (no powershell) but must NOT be rejected
// by the allowlist — the allowlist error field must be empty.
func TestExecute_AllowlistedSingleLineCommand_AllowlistPasses(t *testing.T) {
	cmd := models.Command{ID: "job-003", Type: "powershell", Payload: "Get-Date"}
	result := Execute(cmd)

	// The command should NOT be blocked by the allowlist
	if result.Error == "command_not_allowed" {
		t.Error("Get-Date is an approved pattern; allowlist should not block it")
	}
	// ExitCode may be non-zero on Linux (no powershell), that is expected
}

// TestExecute_InventoryTypeReturnsError verifies that the "inventory" command type,
// while approved by the allowlist, does NOT silently run an empty shell command.
// Instead it should return a clear error.
//
// RED: This test currently FAILS because the executor falls through to /bin/bash
// for non-powershell types, running "bash -c ''" which exits 0 on Linux.
func TestExecute_InventoryTypeReturnsError(t *testing.T) {
	cmd := models.Command{ID: "job-004", Type: "inventory", Payload: ""}
	result := Execute(cmd)

	// "inventory" is not a user-triggered shell command.
	// The executor must not run a shell for it.
	if result.ExitCode == 0 && result.Error == "" && result.Stderr == "" {
		t.Error("Execute with type 'inventory' must not silently succeed via shell; expected a non-zero exit or error")
	}
	if !result.FinishedAt.IsZero() == false {
		// FinishedAt should always be set
		t.Error("FinishedAt must be set")
	}
}

// TestExecute_JobIDIsPreserved verifies that the result always carries back the
// original command ID regardless of outcome.
func TestExecute_JobIDIsPreserved(t *testing.T) {
	cmd := models.Command{ID: "unique-job-xyz", Type: "unknown_type", Payload: "anything"}
	result := Execute(cmd)

	if result.JobID != "unique-job-xyz" {
		t.Errorf("expected JobID=%q, got %q", "unique-job-xyz", result.JobID)
	}
}

// TestExecute_ShellInjectionPatternRejected verifies that command payloads
// containing shell metacharacters are blocked by the allowlist.
func TestExecute_ShellInjectionPatternRejected(t *testing.T) {
	dangerous := []string{
		"Get-Date; rm -rf /",
		"Get-Date | evil",
		"$(malicious)",
		"Get-Date && bad",
	}

	for _, payload := range dangerous {
		cmd := models.Command{ID: "job-sec", Type: "powershell", Payload: payload}
		result := Execute(cmd)

		if result.ExitCode == 0 {
			t.Errorf("dangerous payload %q should be rejected, got ExitCode=0", payload)
		}
		if !strings.Contains(result.Stderr, "allowlist") {
			t.Errorf("dangerous payload %q should mention 'allowlist' in Stderr, got: %q", payload, result.Stderr)
		}
	}
}
