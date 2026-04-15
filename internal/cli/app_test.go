package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestAppRegistersCoreCommands(t *testing.T) {
	app := NewApp()
	got := app.CommandNames()
	want := []string{"compose-request", "validate-request", "init", "run", "execute", "route-evaluation"}
	if !slices.Equal(want, got) {
		t.Fatalf("unexpected commands: want %v got %v", want, got)
	}
}

func TestAppRunRejectsUnknownCommand(t *testing.T) {
	if got := NewApp().Run([]string{"unknown-command"}); got == 0 {
		t.Fatalf("expected non-zero exit code for unknown command, got %d", got)
	}
}

func TestAppRunAcceptsKnownCommand(t *testing.T) {
	projectRoot := t.TempDir()
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir(projectRoot); err != nil {
		t.Fatalf("failed to change working directory: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	if got := NewApp().Run([]string{"init"}); got != 0 {
		t.Fatalf("expected zero exit code for known command, got %d", got)
	}

	if _, err := os.Stat(filepath.Join(projectRoot, ".harness", "project.yaml")); err != nil {
		t.Fatalf("expected init to create project scaffold: %v", err)
	}
}

func TestAppRunRejectsRegisteredButUnimplementedCommands(t *testing.T) {
	for _, command := range []string{"validate-request", "run", "execute", "route-evaluation"} {
		if got := NewApp().Run([]string{command}); got == 0 {
			t.Fatalf("expected non-zero exit code for unimplemented command %q, got %d", command, got)
		}
	}
}

func TestAppRunPrintsComposeRequestErrorsToStderr(t *testing.T) {
	originalStdin := os.Stdin
	originalStdout := os.Stdout
	originalStderr := os.Stderr

	inputFile, err := os.CreateTemp(t.TempDir(), "draft-*.json")
	if err != nil {
		t.Fatalf("failed to create temp input file: %v", err)
	}
	if _, err := inputFile.WriteString(`{"task_type":"bug_fix","goal":"Fix the bug"}`); err != nil {
		t.Fatalf("failed to write draft: %v", err)
	}
	if _, err := inputFile.Seek(0, 0); err != nil {
		t.Fatalf("failed to rewind draft: %v", err)
	}
	t.Cleanup(func() {
		_ = inputFile.Close()
		os.Stdin = originalStdin
		os.Stdout = originalStdout
		os.Stderr = originalStderr
	})

	stderrRead, stderrWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stderr pipe: %v", err)
	}
	defer stderrRead.Close()

	os.Stdin = inputFile
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("failed to open devnull: %v", err)
	}
	defer devNull.Close()
	os.Stdout = devNull
	os.Stderr = stderrWrite

	exitCode := NewApp().Run([]string{"compose-request", "--stdin"})
	_ = stderrWrite.Close()

	if exitCode == 0 {
		t.Fatalf("expected non-zero exit code for compose-request failure, got %d", exitCode)
	}

	var stderr bytes.Buffer
	if _, err := stderr.ReadFrom(stderrRead); err != nil {
		t.Fatalf("failed to read stderr: %v", err)
	}
	if !strings.Contains(stderr.String(), "project_root is required") {
		t.Fatalf("expected compose-request error on stderr, got %q", stderr.String())
	}
}
