package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCreatesMinimalHarnessWorkspace(t *testing.T) {
	projectRoot := t.TempDir()

	if err := Init(projectRoot); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	projectFile := filepath.Join(projectRoot, ".harness", "project.yaml")
	data, err := os.ReadFile(projectFile)
	if err != nil {
		t.Fatalf("failed to read scaffold project file: %v", err)
	}

	wantProjectName := filepath.Base(projectRoot)
	wantLines := []string{
		"schema_version: 1",
		"project_name: " + wantProjectName,
		"rail_compat_version: 1",
		"default_validation_profile: standard",
	}
	got := string(data)
	for _, want := range wantLines {
		if !strings.Contains(got, want) {
			t.Fatalf("project.yaml missing %q in:\n%s", want, got)
		}
	}

	requiredPaths := []string{
		".harness/requests",
		".harness/artifacts",
		".harness/learning/feedback",
		".harness/learning/reviews",
		".harness/learning/hardening-reviews",
		".harness/learning/approved",
		".harness/learning/review_queue.yaml",
		".harness/learning/hardening_queue.yaml",
		".harness/learning/family_evidence_index.yaml",
	}
	for _, relPath := range requiredPaths {
		if _, err := os.Stat(filepath.Join(projectRoot, relPath)); err != nil {
			t.Fatalf("expected %s to exist: %v", relPath, err)
		}
	}
}

func TestInitDoesNotCreateOverrideDirectoriesByDefault(t *testing.T) {
	projectRoot := t.TempDir()

	if err := Init(projectRoot); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	overrideDirs := []string{
		".harness/actors",
		".harness/rules",
		".harness/templates",
		".harness/supervisor",
	}
	for _, relPath := range overrideDirs {
		if _, err := os.Stat(filepath.Join(projectRoot, relPath)); err == nil {
			t.Fatalf("did not expect %s to be created", relPath)
		} else if !os.IsNotExist(err) {
			t.Fatalf("unexpected error checking %s: %v", relPath, err)
		}
	}
}

func TestDiscoverProjectPrefersHarnessProjectFileOverGitRoot(t *testing.T) {
	repoRoot := t.TempDir()
	gitRoot := filepath.Join(repoRoot, "git-root")
	nested := filepath.Join(gitRoot, "nested")

	if err := os.MkdirAll(filepath.Join(gitRoot, ".git"), 0o755); err != nil {
		t.Fatalf("failed to create git root: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, ".harness"), 0o755); err != nil {
		t.Fatalf("failed to create harness dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, ".harness", "project.yaml"), []byte("schema_version: 1\n"), 0o644); err != nil {
		t.Fatalf("failed to write harness project file: %v", err)
	}
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("failed to create nested path: %v", err)
	}

	got, err := DiscoverProject(nested)
	if err != nil {
		t.Fatalf("DiscoverProject returned error: %v", err)
	}

	if got.Root != repoRoot {
		t.Fatalf("unexpected project root: got %q want %q", got.Root, repoRoot)
	}
}
