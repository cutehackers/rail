package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunInitUsesExplicitProjectRootFlag(t *testing.T) {
	projectRoot := t.TempDir()

	if err := RunInit([]string{"--project-root", projectRoot}); err != nil {
		t.Fatalf("RunInit returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(projectRoot, ".harness", "project.yaml")); err != nil {
		t.Fatalf("expected explicit project root to be initialized: %v", err)
	}
}
