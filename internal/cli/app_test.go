package cli

import (
	"os"
	"path/filepath"
	"slices"
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
