package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunBootstrapsSmokeArtifact(t *testing.T) {
	projectRoot, requestPath := prepareSmokeProject(t)

	runner, err := NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}

	artifactPath, err := runner.Run(requestPath, "go-smoke")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(artifactPath, "request.yaml")); err != nil {
		t.Fatalf("expected request snapshot to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(artifactPath, "resolved_workflow.json")); err != nil {
		t.Fatalf("expected resolved_workflow.json to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(artifactPath, "state.json")); err != nil {
		t.Fatalf("expected state.json to exist: %v", err)
	}

	state, err := readState(filepath.Join(artifactPath, "state.json"))
	if err != nil {
		t.Fatalf("failed to read state: %v", err)
	}
	if state.Status != "initialized" {
		t.Fatalf("unexpected status: got %q want %q", state.Status, "initialized")
	}
	if state.CurrentActor == nil || *state.CurrentActor != "planner" {
		t.Fatalf("unexpected current actor: got %v want %q", state.CurrentActor, "planner")
	}
}

func TestExecutePreservesSupervisorTraceability(t *testing.T) {
	projectRoot, requestPath := prepareSmokeProject(t)

	runner, err := NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}

	artifactPath, err := runner.Run(requestPath, "go-smoke")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	summary, err := runner.Execute(artifactPath)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(summary, "status=passed") {
		t.Fatalf("expected execution summary to contain passed status, got %q", summary)
	}

	trace, err := os.ReadFile(filepath.Join(artifactPath, "supervisor_trace.md"))
	if err != nil {
		t.Fatalf("expected supervisor_trace.md to exist: %v", err)
	}
	for _, fragment := range []string{
		"# Supervisor Decision Trace",
		"## Iteration 1",
		"- decision: `pass`",
		"- selected_action: `pass`",
		"- terminal_status: `passed`",
	} {
		if !strings.Contains(string(trace), fragment) {
			t.Fatalf("expected supervisor trace to contain %q, got:\n%s", fragment, string(trace))
		}
	}
}

func prepareSmokeProject(t *testing.T) (string, string) {
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

	requestBody, err := os.ReadFile(filepath.Join(testRepoRoot(t), ".harness", "requests", "rail-bootstrap-smoke.yaml"))
	if err != nil {
		t.Fatalf("failed to read smoke request fixture: %v", err)
	}
	requestPath := filepath.Join(projectRoot, ".harness", "requests", "rail-bootstrap-smoke.yaml")
	if err := os.WriteFile(requestPath, requestBody, 0o644); err != nil {
		t.Fatalf("failed to write smoke request fixture: %v", err)
	}

	return projectRoot, requestPath
}
