package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateRequestRejectsPathOutsideProjectRoot(t *testing.T) {
	projectRoot := t.TempDir()
	validator, err := NewValidator(projectRoot)
	if err != nil {
		t.Fatalf("NewValidator returned error: %v", err)
	}

	outsideDir := t.TempDir()
	requestPath := filepath.Join(outsideDir, "request.yaml")
	requestBody, err := os.ReadFile(filepath.Join(testRepoRootFromContracts(t), "test", "fixtures", "valid_request.yaml"))
	if err != nil {
		t.Fatalf("failed to read fixture request: %v", err)
	}
	if err := os.WriteFile(requestPath, requestBody, 0o644); err != nil {
		t.Fatalf("failed to write request fixture: %v", err)
	}

	_, err = validator.ValidateRequestFile(requestPath)
	if err == nil {
		t.Fatalf("expected ValidateRequestFile to reject a request outside %q", projectRoot)
	}
	if !strings.Contains(err.Error(), "project root") {
		t.Fatalf("expected project-root confinement error, got %v", err)
	}
}

func TestResolvePathWithinRootCanonicalizesSymlinkedPaths(t *testing.T) {
	projectRoot := t.TempDir()
	requestDir := filepath.Join(projectRoot, ".harness", "requests")
	if err := os.MkdirAll(requestDir, 0o755); err != nil {
		t.Fatalf("failed to create request directory: %v", err)
	}

	symlinkParent := t.TempDir()
	symlinkRoot := filepath.Join(symlinkParent, "workspace")
	if err := os.Symlink(projectRoot, symlinkRoot); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	symlinkPath := filepath.Join(symlinkRoot, ".harness", "requests", "request.yaml")
	resolved, err := ResolvePathWithinRoot(projectRoot, symlinkPath)
	if err != nil {
		t.Fatalf("ResolvePathWithinRoot returned error: %v", err)
	}

	canonicalRoot, err := filepath.EvalSymlinks(projectRoot)
	if err != nil {
		t.Fatalf("failed to canonicalize project root: %v", err)
	}
	want := filepath.Join(canonicalRoot, ".harness", "requests", "request.yaml")
	if resolved != want {
		t.Fatalf("unexpected resolved path: got %q want %q", resolved, want)
	}
}

func testRepoRootFromContracts(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("failed to resolve repo root: %v", err)
	}
	return root
}
