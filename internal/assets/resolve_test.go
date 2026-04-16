package assets

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveUsesEmbeddedDefaultWhenLocalOverrideMissing(t *testing.T) {
	projectRoot := t.TempDir()

	got, source, err := Resolve(projectRoot, ".harness/rules/project_rules.md")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}

	if !strings.Contains(string(got), "Go control-plane project profile") {
		t.Fatalf("unexpected content: %q", string(got))
	}
	if source != "embedded" {
		t.Fatalf("unexpected source: got %q want %q", source, "embedded")
	}
}

func TestResolveUsesProjectLocalFileWhenOverrideExists(t *testing.T) {
	projectRoot := t.TempDir()

	localPath := filepath.Join(projectRoot, ".harness", "rules", "project_rules.md")
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		t.Fatalf("failed to create local override dir: %v", err)
	}
	if err := os.WriteFile(localPath, []byte("local override"), 0o644); err != nil {
		t.Fatalf("failed to write local override: %v", err)
	}

	got, source, err := Resolve(projectRoot, ".harness/rules/project_rules.md")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}

	if string(got) != "local override" {
		t.Fatalf("unexpected content: got %q want %q", string(got), "local override")
	}
	if source != "local" {
		t.Fatalf("unexpected source: got %q want %q", source, "local")
	}
}

func TestResolveDoesNotFallbackForStateDirectories(t *testing.T) {
	projectRoot := t.TempDir()

	_, source, err := Resolve(projectRoot, ".harness/artifacts/example.yaml")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrNoFallback) {
		t.Fatalf("expected ErrNoFallback, got %v", err)
	}
	if source != "none" {
		t.Fatalf("unexpected source: got %q want %q", source, "none")
	}
}

func TestResolveRejectsTraversalRelativePath(t *testing.T) {
	projectRoot := t.TempDir()

	_, source, err := Resolve(projectRoot, ".harness/../../go.mod")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrInvalidPath) {
		t.Fatalf("expected ErrInvalidPath, got %v", err)
	}
	if source != "none" {
		t.Fatalf("unexpected source: got %q want %q", source, "none")
	}
}

func TestResolveReturnsLocalStatFailureWhenNotNotExist(t *testing.T) {
	originalStat := statPath
	t.Cleanup(func() { statPath = originalStat })

	statPath = func(string) (os.FileInfo, error) {
		return nil, fmt.Errorf("permission denied")
	}

	_, source, err := Resolve(t.TempDir(), ".harness/rules/project_rules.md")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if source != "none" {
		t.Fatalf("unexpected source: got %q want %q", source, "none")
	}
	if strings.Contains(err.Error(), "permission denied") == false {
		t.Fatalf("expected stat error to be returned, got %v", err)
	}
	if strings.Contains(err.Error(), "embedded") {
		t.Fatalf("unexpected embedded fallback in error: %v", err)
	}
}
