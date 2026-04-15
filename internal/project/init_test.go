package project

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
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

	var got struct {
		SchemaVersion            int    `yaml:"schema_version"`
		ProjectName              string `yaml:"project_name"`
		RailCompatVersion        int    `yaml:"rail_compat_version"`
		DefaultValidationProfile string `yaml:"default_validation_profile"`
	}
	if err := yaml.Unmarshal(data, &got); err != nil {
		t.Fatalf("failed to unmarshal project file: %v", err)
	}
	if got.SchemaVersion != 1 {
		t.Fatalf("unexpected schema version: got %d want %d", got.SchemaVersion, 1)
	}
	if got.ProjectName != filepath.Base(projectRoot) {
		t.Fatalf("unexpected project name: got %q want %q", got.ProjectName, filepath.Base(projectRoot))
	}
	if got.RailCompatVersion != 1 {
		t.Fatalf("unexpected rail compat version: got %d want %d", got.RailCompatVersion, 1)
	}
	if got.DefaultValidationProfile != "standard" {
		t.Fatalf("unexpected validation profile: got %q want %q", got.DefaultValidationProfile, "standard")
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

func TestInitQuotesYamlSensitiveProjectName(t *testing.T) {
	parent := t.TempDir()
	projectRoot := filepath.Join(parent, "project:one")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("failed to create project root: %v", err)
	}

	if err := Init(projectRoot); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(projectRoot, ".harness", "project.yaml"))
	if err != nil {
		t.Fatalf("failed to read project file: %v", err)
	}

	var got struct {
		ProjectName string `yaml:"project_name"`
	}
	if err := yaml.Unmarshal(data, &got); err != nil {
		t.Fatalf("failed to unmarshal project file: %v", err)
	}
	if got.ProjectName != "project:one" {
		t.Fatalf("unexpected project name: got %q want %q", got.ProjectName, "project:one")
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

func TestInitIsIdempotentWhenProjectFileExists(t *testing.T) {
	projectRoot := t.TempDir()
	projectFile := filepath.Join(projectRoot, ".harness", "project.yaml")

	if err := os.MkdirAll(filepath.Dir(projectFile), 0o755); err != nil {
		t.Fatalf("failed to create harness dir: %v", err)
	}
	if err := os.WriteFile(projectFile, []byte("preexisting project file"), 0o644); err != nil {
		t.Fatalf("failed to seed project file: %v", err)
	}

	if err := Init(projectRoot); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	data, err := os.ReadFile(projectFile)
	if err != nil {
		t.Fatalf("failed to read project file: %v", err)
	}
	if string(data) != "preexisting project file" {
		t.Fatalf("expected existing project file to be preserved, got %q", string(data))
	}
}
