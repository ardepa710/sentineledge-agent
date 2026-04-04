package system

import (
	"runtime"
	"testing"
	"time"
)

// ── runCmd ────────────────────────────────────────────────────────────────

func TestRunCmd_BasicCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Linux-only test")
	}
	out, err := runCmd("echo", "hello")
	if err != nil {
		t.Fatalf("runCmd echo failed: %v", err)
	}
	if out != "hello" {
		t.Errorf("expected 'hello', got %q", out)
	}
}

func TestRunCmd_FailingCommandReturnsError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Linux-only test")
	}
	_, err := runCmd("false")
	if err == nil {
		t.Error("runCmd should return error for non-zero exit commands")
	}
}

func TestRunCmd_NonexistentCommandReturnsError(t *testing.T) {
	_, err := runCmd("nonexistent_binary_xyz_sentinel")
	if err == nil {
		t.Error("runCmd should return error for nonexistent binary")
	}
}

// TestRunCmd_HangingCommandTimesOut verifies that runCmd respects a timeout
// and does not block indefinitely on a hanging command.
//
// RED: This test currently FAILS (or hangs for 10s) because runCmd has no
// timeout. After the fix (adding context.WithTimeout), it must complete quickly.
func TestRunCmd_HangingCommandTimesOut(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Linux-only test")
	}

	start := time.Now()
	done := make(chan error, 1)

	go func() {
		_, err := runCmd("sleep", "10")
		done <- err
	}()

	select {
	case err := <-done:
		elapsed := time.Since(start)
		if err == nil {
			t.Error("runCmd('sleep 10') should return an error due to timeout")
		}
		// Should timeout in ~2s (our chosen timeout for runCmd), not 10s
		if elapsed > 5*time.Second {
			t.Errorf("runCmd took too long (%v) — timeout not working", elapsed)
		}
	case <-time.After(12 * time.Second):
		t.Error("runCmd hung for 12s — no timeout implemented")
	}
}

// ── CollectInventory ──────────────────────────────────────────────────────

func TestCollectInventory_ReturnsNonNilOnLinux(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Linux-only test")
	}

	inv, err := CollectInventory("test-agent-id", "test-host")
	if err != nil {
		t.Fatalf("CollectInventory failed: %v", err)
	}
	if inv == nil {
		t.Fatal("CollectInventory returned nil inventory")
	}
	if inv.AgentID != "test-agent-id" {
		t.Errorf("expected AgentID='test-agent-id', got %q", inv.AgentID)
	}
	if inv.OS == "" {
		t.Error("OS field should be set")
	}
}

func TestCollectInventory_LinuxHasCPUInfo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Linux-only test")
	}

	inv, err := CollectInventory("agent-1", "localhost")
	if err != nil {
		t.Fatalf("CollectInventory failed: %v", err)
	}
	if inv.CPU.Name == "" {
		t.Error("Linux inventory should populate CPU.Name")
	}
}

func TestCollectInventory_LinuxHasRAMInfo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Linux-only test")
	}

	inv, err := CollectInventory("agent-1", "localhost")
	if err != nil {
		t.Fatalf("CollectInventory failed: %v", err)
	}
	if inv.RAM.TotalPhysicalMemoryGB <= 0 {
		t.Errorf("RAM.TotalPhysicalMemoryGB should be > 0, got %f", inv.RAM.TotalPhysicalMemoryGB)
	}
}
