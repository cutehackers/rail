package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"
)

type CommandResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

type CommandRunner interface {
	RunShell(command, workingDirectory string, timeout time.Duration) (CommandResult, error)
}

type subprocessRunner struct{}

func (subprocessRunner) RunShell(command, workingDirectory string, timeout time.Duration) (CommandResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	shell := "/bin/sh"
	if _, err := os.Stat(shell); err != nil {
		shell = "sh"
	}
	cmd := exec.CommandContext(ctx, shell, "-lc", command)
	cmd.Dir = workingDirectory

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := CommandResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if ctx.Err() == context.DeadlineExceeded {
		result.ExitCode = -1
		return result, nil
	}

	if err == nil {
		return result, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}

	return CommandResult{}, fmt.Errorf("run shell command %q: %w", command, err)
}
