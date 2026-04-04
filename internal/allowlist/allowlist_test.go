package allowlist

import (
	"testing"
)

// ── Approved types ────────────────────────────────────────────────────────

func TestValidate_UpdateTypeAlwaysApproved(t *testing.T) {
	if err := Validate("update", ""); err != nil {
		t.Errorf("'update' type must always pass, got: %v", err)
	}
}

func TestValidate_InventoryTypeAlwaysApproved(t *testing.T) {
	if err := Validate("inventory", ""); err != nil {
		t.Errorf("'inventory' type must always pass, got: %v", err)
	}
}

// ── Empty payload ────────────────────────────────────────────────────────

func TestValidate_EmptyPowershellPayloadRejected(t *testing.T) {
	err := Validate("powershell", "")
	if err == nil {
		t.Error("empty powershell payload must be rejected")
	}
	if rejected, ok := err.(*ErrCommandRejected); ok {
		if rejected.Type != "powershell" {
			t.Errorf("expected Type=powershell, got %q", rejected.Type)
		}
	}
}

func TestValidate_WhitespaceOnlyPowershellPayloadRejected(t *testing.T) {
	err := Validate("powershell", "   \t\n  ")
	if err == nil {
		t.Error("whitespace-only powershell payload must be rejected")
	}
}

// ── Pattern allowlist ─────────────────────────────────────────────────────

var approvedPayloads = []struct {
	name    string
	payload string
}{
	{"Get-Service no args", "Get-Service"},
	{"Get-Service with name", "Get-Service WinRM"},
	{"Get-Service dash name", "Get-Service print-spooler"},
	{"Restart-Service", "Restart-Service Spooler"},
	{"Restart-Service with -Force", "Restart-Service Spooler -Force"},
	{"Stop-Service", "Stop-Service Spooler"},
	{"Start-Service", "Start-Service Spooler"},
	{"Get-ComputerInfo", "Get-ComputerInfo"},
	{"Get-ComputerInfo -Property", "Get-ComputerInfo -Property OsName, OsVersion"},
	{"Get-Disk", "Get-Disk"},
	{"Get-Volume", "Get-Volume"},
	{"Get-Process", "Get-Process"},
	{"WMI OS", "Get-WmiObject Win32_OperatingSystem"},
	{"WMI CPU", "Get-WmiObject Win32_Processor"},
	{"WMI RAM", "Get-WmiObject Win32_PhysicalMemory"},
	{"WMI Disk", "Get-WmiObject Win32_LogicalDisk"},
	{"OSVersion string", "[System.Environment]::OSVersion.VersionString"},
	{"Get-EventLog App", "Get-EventLog -LogName Application -Newest 50"},
	{"Get-EventLog System", "Get-EventLog -LogName System -Newest 100"},
	{"Get-EventLog Security", "Get-EventLog -LogName Security -Newest 9999"},
	{"Test-Connection", "Test-Connection google.com"},
	{"Test-Connection count", "Test-Connection 192.168.1.1 -Count 4"},
	{"Test-Connection quiet", "Test-Connection host.local -Count 2 -Quiet"},
	{"Get-NetAdapter", "Get-NetAdapter"},
	{"Get-NetIPAddress", "Get-NetIPAddress"},
	{"Resolve-DnsName", "Resolve-DnsName google.com"},
	{"WU count query", "(New-Object -ComObject Microsoft.Update.Session).CreateUpdateSearcher().Search(\"IsInstalled=0\").Updates.Count"},
	{"GetEnvironmentVariable", "[System.Environment]::GetEnvironmentVariable(\"PATH\")"},
	{"Get-Date", "Get-Date"},
	{"PSVersion", "$PSVersionTable.PSVersion"},
}

func TestValidate_ApprovedPatterns(t *testing.T) {
	for _, tc := range approvedPayloads {
		t.Run(tc.name, func(t *testing.T) {
			if err := Validate("powershell", tc.payload); err != nil {
				t.Errorf("expected approved, got rejected: %v", err)
			}
		})
	}
}

var dangerousPayloads = []struct {
	name    string
	payload string
}{
	{"semicolon chain", "Get-Date; Remove-Item C:\\"},
	{"pipe to evil", "Get-Date | Invoke-Expression evil"},
	{"subshell", "$(Remove-Item C:\\)"},
	{"ampersand chain", "Get-Date && evil"},
	{"backtick", "Get-Date`evil"},
	{"redirect output", "Get-Process > C:\\out.txt"},
	{"redirect input", "echo x < C:\\secret"},
	{"arbitrary PS script", "Write-Host 'pwned'; Restart-Computer"},
	{"Get-EventLog too many", "Get-EventLog -LogName Application -Newest 99999"},
	{"Get-EventLog unknown log", "Get-EventLog -LogName Security1 -Newest 50"},
	{"unknown command", "Invoke-Expression 'evil'"},
	{"WMI arbitrary class", "Get-WmiObject Win32_Process"},
	{"Restart-Service no name", "Restart-Service"},
	{"Stop-Service no name", "Stop-Service"},
	{"GetEnvVar with spaces injection", "[System.Environment]::GetEnvironmentVariable(\"PATH; evil\")"},
}

func TestValidate_DangerousPayloadsRejected(t *testing.T) {
	for _, tc := range dangerousPayloads {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate("powershell", tc.payload)
			if err == nil {
				t.Errorf("dangerous payload should be rejected: %q", tc.payload)
			}
		})
	}
}

// ── Unknown command types ─────────────────────────────────────────────────

func TestValidate_UnknownTypesRejected(t *testing.T) {
	types := []string{"bash", "cmd", "shell", "sh", "exec", "python", "ruby", ""}
	for _, cmdType := range types {
		err := Validate(cmdType, "some payload")
		if err == nil {
			t.Errorf("type %q should be rejected, but was allowed", cmdType)
		}
	}
}

func TestValidate_ErrorTypeIsErrCommandRejected(t *testing.T) {
	err := Validate("bash", "ls")
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(*ErrCommandRejected); !ok {
		t.Errorf("expected *ErrCommandRejected, got %T: %v", err, err)
	}
}

func TestErrCommandRejected_ErrorMessage(t *testing.T) {
	e := &ErrCommandRejected{Type: "bash", Reason: "not permitted"}
	msg := e.Error()
	if msg == "" {
		t.Error("ErrCommandRejected.Error() should not return empty string")
	}
}

// ── Hash registry ─────────────────────────────────────────────────────────

func TestRegisterApprovedScript_AllowsMatchingPayload(t *testing.T) {
	script := "Get-WindowsUpdateHistory | Select-Object -First 10 | Format-List"
	RegisterApprovedScript(script, "test-windows-update-history")

	err := Validate("powershell", script)
	if err != nil {
		t.Errorf("registered script should pass validation, got: %v", err)
	}
}

func TestRegisterApprovedScript_WhitespaceNormalized(t *testing.T) {
	script := "  Get-SpecialReport  "
	RegisterApprovedScript(script, "test-special-report")

	// Payload with different surrounding whitespace should still match
	if err := Validate("powershell", "Get-SpecialReport"); err != nil {
		t.Errorf("trimmed script should match registered hash, got: %v", err)
	}
	if err := Validate("powershell", "  Get-SpecialReport  "); err != nil {
		t.Errorf("whitespace-padded script should match registered hash, got: %v", err)
	}
}

func TestRegisterApprovedScript_UnregisteredScriptRejected(t *testing.T) {
	script := "Some-RandomCommand -That -Was -Never -Registered"
	err := Validate("powershell", script)
	if err == nil {
		t.Error("unregistered complex script should be rejected")
	}
}

// ── HashPayload ───────────────────────────────────────────────────────────

func TestHashPayload_IsConsistent(t *testing.T) {
	payload := "Get-Process"
	h1 := HashPayload(payload)
	h2 := HashPayload(payload)

	if h1 != h2 {
		t.Errorf("HashPayload must be deterministic: %q != %q", h1, h2)
	}
}

func TestHashPayload_DifferentInputsDifferentHashes(t *testing.T) {
	h1 := HashPayload("Get-Process")
	h2 := HashPayload("Get-Service")

	if h1 == h2 {
		t.Error("different payloads must produce different hashes")
	}
}

func TestHashPayload_TrimsWhitespace(t *testing.T) {
	h1 := HashPayload("Get-Process")
	h2 := HashPayload("  Get-Process  ")

	if h1 != h2 {
		t.Error("HashPayload should trim whitespace before hashing")
	}
}

func TestHashPayload_IsHex64Chars(t *testing.T) {
	h := HashPayload("anything")
	if len(h) != 64 {
		t.Errorf("SHA256 hex digest must be 64 chars, got %d: %q", len(h), h)
	}
	for _, c := range h {
		if !('0' <= c && c <= '9') && !('a' <= c && c <= 'f') {
			t.Errorf("HashPayload should return lowercase hex, found char %q in %q", c, h)
		}
	}
}
