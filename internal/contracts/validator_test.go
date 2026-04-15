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

func testRepoRootFromContracts(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("failed to resolve repo root: %v", err)
	}
	return root
}
