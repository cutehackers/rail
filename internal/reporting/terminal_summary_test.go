package reporting

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rail/internal/runtime"
)

func TestExecuteProducesTerminalSummaryAndState(t *testing.T) {
	projectRoot, requestPath := prepareReportingSmokeProject(t)

	runner, err := runtime.NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}

	artifactPath, err := runner.Run(requestPath, "reporting-smoke")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if _, err := runner.Execute(artifactPath); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	state, err := LoadState(filepath.Join(artifactPath, "state.json"))
	if err != nil {
		t.Fatalf("LoadState returned error: %v", err)
	}
	if state.Status != "passed" {
		t.Fatalf("unexpected status: got %q want %q", state.Status, "passed")
	}
	if state.CurrentActor != nil {
		t.Fatalf("expected terminal state to clear current actor, got %v", state.CurrentActor)
	}

	summary, err := ReadTerminalSummary(filepath.Join(artifactPath, "terminal_summary.md"))
	if err != nil {
		t.Fatalf("ReadTerminalSummary returned error: %v", err)
	}
	for _, fragment := range []string{
		"# Terminal Outcome",
		"- status: `passed`",
		"## Recommended Next Step",
	} {
		if !strings.Contains(summary, fragment) {
			t.Fatalf("expected terminal summary to contain %q, got:\n%s", fragment, summary)
		}
	}
}

func prepareReportingSmokeProject(t *testing.T) (string, string) {
	t.Helper()

	projectRoot := t.TempDir()
	for _, relPath := range []string{
		filepath.Join(".harness", "requests"),
		filepath.Join(".harness", "artifacts"),
		"test",
	} {
		if err := os.MkdirAll(filepath.Join(projectRoot, relPath), 0o755); err != nil {
			t.Fatalf("failed to create %q: %v", relPath, err)
		}
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".git"), []byte("gitdir: test\n"), 0o644); err != nil {
		t.Fatalf("failed to write git marker: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "pubspec.yaml"), []byte("name: smoke_project\n"), 0o644); err != nil {
		t.Fatalf("failed to write pubspec.yaml: %v", err)
	}

	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("failed to resolve repo root: %v", err)
	}
	requestBody, err := os.ReadFile(filepath.Join(repoRoot, ".harness", "requests", "rail-bootstrap-smoke.yaml"))
	if err != nil {
		t.Fatalf("failed to read smoke request fixture: %v", err)
	}
	requestPath := filepath.Join(projectRoot, ".harness", "requests", "rail-bootstrap-smoke.yaml")
	if err := os.WriteFile(requestPath, requestBody, 0o644); err != nil {
		t.Fatalf("failed to write smoke request fixture: %v", err)
	}

	return projectRoot, requestPath
}
