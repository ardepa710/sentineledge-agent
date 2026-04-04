package executor

import (
	"testing"

	"github.com/sentineledge/agent/pkg/models"
)

// TestExecute_UnknownTypeRejected verifies that commands with unknown types
// are rejected before any execution occurs.
func TestExecute_UnknownTypeRejected(t *testing.T) {
	cmd := models.Command{ID: "job-001", Type: "bash", Payload: "ls"}
	result := Execute(cmd)

	if result.ExitCode != 1 {
		t.Errorf("expected ExitCode=1 for unknown type, got %d", result.ExitCode)
	}
	if result.Error != "command_not_allowed" {
		t.Errorf("expected Error='command_not_allowed', got %q", result.Error)
	}
	if result.FinishedAt.IsZero() {
		t.Error("FinishedAt should be set even for rejected commands")
	}
}

// TestExecute_InventoryTypeReturnsError verifies that the "inventory" command type
// does not silently run a shell command.
func TestExecute_InventoryTypeReturnsError(t *testing.T) {
	cmd := models.Command{ID: "job-004", Type: "inventory", Payload: ""}
	result := Execute(cmd)

	if result.ExitCode == 0 && result.Error == "" && result.Stderr == "" {
		t.Error("Execute with type 'inventory' must not silently succeed via shell; expected a non-zero exit or error")
	}
	if result.FinishedAt.IsZero() {
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
