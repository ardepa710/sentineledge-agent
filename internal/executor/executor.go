package executor

import (
	"bytes"
	"context"
	"os/exec"
	"time"

	"github.com/sentineledge/agent/pkg/models"
)

func Execute(cmd models.Command) models.Result {
	result := models.Result{
		JobID: cmd.ID,
	}

	timeout := time.Duration(cmd.Timeout) * time.Second
	if cmd.Timeout == 0 {
		timeout = 5 * time.Minute
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var execCmd *exec.Cmd

	if cmd.Type == "powershell" {
		execCmd = exec.CommandContext(ctx,
			"powershell.exe",
			"-NonInteractive",
			"-NoProfile",
			"-ExecutionPolicy", "Bypass",
			"-Command", cmd.Payload,
		)
	} else {
		execCmd = exec.CommandContext(ctx,
			"/bin/bash", "-c", cmd.Payload,
		)
	}

	var stdout, stderr bytes.Buffer
	execCmd.Stdout = &stdout
	execCmd.Stderr = &stderr

	err := execCmd.Run()

	result.Stdout = stdout.String()
	result.Stderr = stderr.String()
	result.FinishedAt = time.Now().UTC()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
			result.Error = err.Error()
		}
	}

	return result
}
