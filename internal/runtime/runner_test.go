package runtime

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
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
	workflowPath := filepath.Join(artifactPath, workflowArtifactFileName)
	if _, err := os.Stat(workflowPath); err != nil {
		t.Fatalf("expected workflow artifact %q to exist: %v", workflowPath, err)
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

func TestExecutePreservesDistinctLogsAcrossRepeatedActorPasses(t *testing.T) {
	projectRoot, requestPath := prepareSmokeProject(t)

	runner, err := NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}
	runner.commands = &stubCommandRunner{
		results: []CommandResult{
			{ExitCode: 0},
			{ExitCode: 1},
			{ExitCode: 0},
			{ExitCode: 0},
		},
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

	runEntries, err := os.ReadDir(filepath.Join(artifactPath, "runs"))
	if err != nil {
		t.Fatalf("failed to read runs directory: %v", err)
	}

	executorLogs := []string{}
	for _, entry := range runEntries {
		if entry.IsDir() {
			continue
		}
		if strings.Contains(entry.Name(), "executor") && strings.HasSuffix(entry.Name(), "-last-message.txt") {
			executorLogs = append(executorLogs, entry.Name())
		}
	}
	slices.Sort(executorLogs)
	if len(executorLogs) != 2 {
		t.Fatalf("expected 2 executor logs after repeated executor passes, got %d (%v)", len(executorLogs), executorLogs)
	}

	firstLog, err := os.ReadFile(filepath.Join(artifactPath, "runs", executorLogs[0]))
	if err != nil {
		t.Fatalf("failed to read first executor log: %v", err)
	}
	secondLog, err := os.ReadFile(filepath.Join(artifactPath, "runs", executorLogs[1]))
	if err != nil {
		t.Fatalf("failed to read second executor log: %v", err)
	}
	if !strings.Contains(string(firstLog), `"analyze": "pass"`) || !strings.Contains(string(firstLog), `"tests": {`) || !strings.Contains(string(firstLog), `"failed": 1`) {
		t.Fatalf("expected first executor log to preserve the failing pass, got:\n%s", string(firstLog))
	}
	if !strings.Contains(string(secondLog), `"analyze": "pass"`) || !strings.Contains(string(secondLog), `"tests": {`) || !strings.Contains(string(secondLog), `"failed": 0`) {
		t.Fatalf("expected second executor log to preserve the passing pass, got:\n%s", string(secondLog))
	}
}

func TestExecuteRefreshesPersistedOutputsForCompletedArtifacts(t *testing.T) {
	projectRoot, requestPath := prepareSmokeProject(t)

	runner, err := NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}

	artifactPath, err := runner.Run(requestPath, "go-smoke")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if _, err := runner.Execute(artifactPath); err != nil {
		t.Fatalf("initial Execute returned error: %v", err)
	}

	for _, name := range []string{"supervisor_trace.md", "terminal_summary.md"} {
		if err := os.Remove(filepath.Join(artifactPath, name)); err != nil {
			t.Fatalf("failed to remove %s: %v", name, err)
		}
	}

	summary, err := runner.Execute(artifactPath)
	if err != nil {
		t.Fatalf("refresh Execute returned error: %v", err)
	}
	if strings.Contains(summary, "already completed") {
		t.Fatalf("expected Execute to refresh persisted outputs instead of returning early, got %q", summary)
	}
	if !strings.Contains(summary, "status=passed") {
		t.Fatalf("expected refresh summary to include passed status, got %q", summary)
	}

	for _, name := range []string{"supervisor_trace.md", "terminal_summary.md"} {
		if _, err := os.Stat(filepath.Join(artifactPath, name)); err != nil {
			t.Fatalf("expected %s to be recreated: %v", name, err)
		}
	}
}

func TestRunRejectsNonEmptyExistingArtifactDirectory(t *testing.T) {
	projectRoot, requestPath := prepareSmokeProject(t)
	artifactPath := filepath.Join(projectRoot, ".harness", "artifacts", "go-smoke")
	if err := os.MkdirAll(artifactPath, 0o755); err != nil {
		t.Fatalf("failed to create artifact directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(artifactPath, "supervisor_trace.md"), []byte("stale trace\n"), 0o644); err != nil {
		t.Fatalf("failed to seed stale supervisor trace: %v", err)
	}

	runner, err := NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}

	_, err = runner.Run(requestPath, "go-smoke")
	if err == nil {
		t.Fatalf("expected Run to reject non-empty artifact directory")
	}
	if !strings.Contains(err.Error(), "already exists and is not empty") {
		t.Fatalf("expected non-empty artifact directory error, got %v", err)
	}

	trace, err := os.ReadFile(filepath.Join(artifactPath, "supervisor_trace.md"))
	if err != nil {
		t.Fatalf("expected stale supervisor trace to remain readable: %v", err)
	}
	if string(trace) != "stale trace\n" {
		t.Fatalf("expected stale supervisor trace to remain unchanged, got %q", string(trace))
	}
}

func TestBuildSmokeEvaluationResultRejectsFormatFailure(t *testing.T) {
	artifactDirectory := t.TempDir()
	executionReport := map[string]any{
		"format":  "fail",
		"analyze": "pass",
		"tests": map[string]any{
			"total":  1,
			"passed": 1,
			"failed": 0,
		},
		"failure_details": []string{"Format command failed: gofmt -w foo.go"},
		"logs":            []string{"gofmt -w foo.go (exit=1)"},
	}
	data, err := yaml.Marshal(executionReport)
	if err != nil {
		t.Fatalf("failed to marshal execution report: %v", err)
	}
	if err := os.WriteFile(filepath.Join(artifactDirectory, "execution_report.yaml"), data, 0o644); err != nil {
		t.Fatalf("failed to write execution report: %v", err)
	}

	result, err := buildSmokeEvaluationResult(artifactDirectory)
	if err != nil {
		t.Fatalf("buildSmokeEvaluationResult returned error: %v", err)
	}

	decision, ok := result["decision"].(string)
	if !ok {
		t.Fatalf("expected decision to be a string, got %T", result["decision"])
	}
	if decision != "revise" {
		t.Fatalf("expected format failure to force revise, got %q", decision)
	}
}

func TestExecuteRunsRealActorPathThroughCodex(t *testing.T) {
	projectRoot, requestPath := prepareRealProject(t)
	actorLogPath := installFakeCodexForRealMode(t, projectRoot)

	runner, err := NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}

	artifactPath, err := runner.Run(requestPath, "go-real")
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

	actorLog, err := os.ReadFile(actorLogPath)
	if err != nil {
		t.Fatalf("failed to read fake codex actor log: %v", err)
	}
	if got, want := strings.TrimSpace(string(actorLog)), "planner\ncontext_builder\ngenerator\nevaluator"; got != want {
		t.Fatalf("unexpected actor execution order: got %q want %q", got, want)
	}

	readySource, err := os.ReadFile(filepath.Join(projectRoot, "feature", "ready.go"))
	if err != nil {
		t.Fatalf("failed to read real-mode source file: %v", err)
	}
	if !strings.Contains(string(readySource), "Real-mode actor path verified.") {
		t.Fatalf("expected generator actor to update ready.go, got:\n%s", string(readySource))
	}

	state, err := readState(filepath.Join(artifactPath, "state.json"))
	if err != nil {
		t.Fatalf("failed to read state: %v", err)
	}
	if state.Status != "passed" {
		t.Fatalf("unexpected terminal status: got %q want %q", state.Status, "passed")
	}
}

func prepareSmokeProject(t *testing.T) (string, string) {
	t.Helper()

	projectRoot := t.TempDir()
	for _, relPath := range []string{
		filepath.Join(".harness", "requests"),
		filepath.Join(".harness", "artifacts"),
		"smoke",
	} {
		if err := os.MkdirAll(filepath.Join(projectRoot, relPath), 0o755); err != nil {
			t.Fatalf("failed to create %q: %v", relPath, err)
		}
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".git"), []byte("gitdir: test\n"), 0o644); err != nil {
		t.Fatalf("failed to write git marker: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "go.mod"), []byte("module smokeproject\n\ngo 1.25.0\n"), 0o644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "smoke", "smoke.go"), []byte("package smoke\n\nfunc Ready() bool { return true }\n"), 0o644); err != nil {
		t.Fatalf("failed to write smoke.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "smoke", "smoke_test.go"), []byte("package smoke\n\nimport \"testing\"\n\nfunc TestReady(t *testing.T) {\n\tif !Ready() {\n\t\tt.Fatal(\"expected Ready to return true\")\n\t}\n}\n"), 0o644); err != nil {
		t.Fatalf("failed to write smoke_test.go: %v", err)
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

func prepareRealProject(t *testing.T) (string, string) {
	t.Helper()

	projectRoot := t.TempDir()
	for _, relPath := range []string{
		filepath.Join(".harness", "requests"),
		filepath.Join(".harness", "artifacts"),
		"feature",
	} {
		if err := os.MkdirAll(filepath.Join(projectRoot, relPath), 0o755); err != nil {
			t.Fatalf("failed to create %q: %v", relPath, err)
		}
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".git"), []byte("gitdir: test\n"), 0o644); err != nil {
		t.Fatalf("failed to write git marker: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "go.mod"), []byte("module realproject\n\ngo 1.25.0\n"), 0o644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "feature", "ready.go"), []byte("package feature\n\nfunc Ready() bool { return true }\n"), 0o644); err != nil {
		t.Fatalf("failed to write ready.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "feature", "ready_test.go"), []byte("package feature\n\nimport \"testing\"\n\nfunc TestReady(t *testing.T) {\n\tif !Ready() {\n\t\tt.Fatal(\"expected Ready to return true\")\n\t}\n}\n"), 0o644); err != nil {
		t.Fatalf("failed to write ready_test.go: %v", err)
	}

	requestPath := filepath.Join(projectRoot, ".harness", "requests", "real-request.yaml")
	requestBody := `task_type: safe_refactor
goal: verify the real actor path against a tracked Go target
context:
  feature: feature
  suspected_files:
    - feature/ready.go
  validation_roots:
    - feature
  validation_targets:
    - feature/ready_test.go
constraints:
  - keep behavior unchanged
definition_of_done:
  - target file is updated through the real actor path
  - tests remain green
  - analyze remains green
priority: medium
risk_tolerance: low
validation_profile: standard
`
	if err := os.WriteFile(requestPath, []byte(requestBody), 0o644); err != nil {
		t.Fatalf("failed to write real request fixture: %v", err)
	}

	return projectRoot, requestPath
}

func installFakeCodexForRealMode(t *testing.T, projectRoot string) string {
	t.Helper()

	fakeBin := t.TempDir()
	actorLogPath := filepath.Join(projectRoot, ".actor-log")
	fakeCodex := filepath.Join(fakeBin, "codex")
	script := `#!/usr/bin/env python3
import json
import os
import re
import sys

project_root = None
output_path = None
prompt = sys.argv[-1] if len(sys.argv) > 1 else ""
for index, value in enumerate(sys.argv):
    if value == "--output-last-message" and index + 1 < len(sys.argv):
        output_path = sys.argv[index + 1]
        break

match = re.search(r"Actor name: ([a-z_]+)", prompt)
actor = match.group(1) if match else "unknown"
root_match = re.search(r"Project root: (.+)", prompt)
if root_match:
    project_root = root_match.group(1).strip()

if project_root:
    with open(os.path.join(project_root, ".actor-log"), "a", encoding="utf-8") as handle:
        handle.write(actor + "\n")

response = {}
if actor == "planner":
    response = {
        "summary": "Real actor path plan.",
        "likely_files": ["feature/ready.go", "feature/ready_test.go"],
        "assumptions": ["Go package layout stays local."],
        "substeps": ["Inspect the target file.", "Update the target file narrowly.", "Run focused validation."],
        "risks": ["Unnecessary edits could broaden the diff."],
        "acceptance_criteria_refined": ["target file is updated through the real actor path", "tests remain green", "analyze remains green"]
    }
elif actor == "context_builder":
    response = {
        "relevant_files": [{"path": "feature/ready.go", "why": "Primary file under change."}, {"path": "feature/ready_test.go", "why": "Focused regression coverage."}],
        "repo_patterns": ["Keep changes inside the feature package."],
        "test_patterns": ["Use package-local Go tests."],
        "forbidden_changes": ["No unrelated files."],
        "implementation_hints": ["Add a narrow comment-only change."]
    }
elif actor == "generator":
    ready_path = os.path.join(project_root, "feature", "ready.go")
    with open(ready_path, "r", encoding="utf-8") as handle:
        original = handle.read()
    if "Real-mode actor path verified." not in original:
        updated = original.replace("func Ready() bool { return true }", "// Real-mode actor path verified.\nfunc Ready() bool { return true }")
        with open(ready_path, "w", encoding="utf-8") as handle:
            handle.write(updated)
    response = {
        "changed_files": ["feature/ready.go"],
        "patch_summary": ["Added a narrow comment proving the real actor path touched the target file."],
        "tests_added_or_updated": [],
        "known_limits": ["Test fixture uses a fake codex executable."]
    }
elif actor == "evaluator":
    response = {
        "decision": "pass",
        "scores": {"requirements": 1.0, "architecture": 1.0, "regression_risk": 1.0},
        "findings": ["Real actor path completed with passing validation evidence."],
        "reason_codes": [],
        "quality_confidence": "high"
    }
else:
    response = {"summary": "unexpected actor"}

os.makedirs(os.path.dirname(output_path), exist_ok=True)
with open(output_path, "w", encoding="utf-8") as handle:
    json.dump(response, handle)
`
	if err := os.WriteFile(fakeCodex, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake codex: %v", err)
	}

	originalPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", fakeBin+string(os.PathListSeparator)+originalPath); err != nil {
		t.Fatalf("failed to set PATH: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Setenv("PATH", originalPath)
	})
	return actorLogPath
}

type stubCommandRunner struct {
	results []CommandResult
	call    int
}

func (s *stubCommandRunner) RunShell(command, workingDirectory string, timeout time.Duration) (CommandResult, error) {
	if s.call >= len(s.results) {
		return CommandResult{}, nil
	}
	result := s.results[s.call]
	s.call++
	return result, nil
}
