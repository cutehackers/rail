package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestAppRegistersCoreCommands(t *testing.T) {
	app := NewApp()
	got := app.CommandNames()
	want := []string{
		"init-request",
		"compose-request",
		"validate-request",
		"validate-artifact",
		"init",
		"init-user-outcome-feedback",
		"init-learning-review",
		"init-hardening-review",
		"run",
		"execute",
		"route-evaluation",
		"integrate",
		"apply-user-outcome-feedback",
		"apply-learning-review",
		"apply-hardening-review",
		"verify-learning-state",
	}
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

func TestAppRunWritesRequestTemplateForInitRequest(t *testing.T) {
	projectRoot := t.TempDir()
	if err := RunInit([]string{"--project-root", projectRoot}); err != nil {
		t.Fatalf("RunInit returned error: %v", err)
	}

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

	if got := NewApp().Run([]string{"init-request"}); got != 0 {
		t.Fatalf("expected zero exit code for init-request, got %d", got)
	}

	if _, err := os.Stat(filepath.Join(projectRoot, ".harness", "request.template.yaml")); err != nil {
		t.Fatalf("expected init-request to create request template: %v", err)
	}
}

func TestAppRunBootstrapsArtifactForRunCommand(t *testing.T) {
	projectRoot, requestPath := prepareSmokeProjectForCLI(t)

	if got := NewApp().Run([]string{
		"run",
		"--request", requestPath,
		"--project-root", projectRoot,
		"--task-id", "cli-run-smoke",
	}); got != 0 {
		t.Fatalf("expected zero exit code for run, got %d", got)
	}

	if _, err := os.Stat(filepath.Join(projectRoot, ".harness", "artifacts", "cli-run-smoke", "state.json")); err != nil {
		t.Fatalf("expected run command to create artifact state: %v", err)
	}
}

func TestAppRunExecutesArtifactForExecuteCommand(t *testing.T) {
	projectRoot, requestPath := prepareSmokeProjectForCLI(t)

	if got := NewApp().Run([]string{
		"run",
		"--request", requestPath,
		"--project-root", projectRoot,
		"--task-id", "cli-execute-smoke",
	}); got != 0 {
		t.Fatalf("expected zero exit code for run, got %d", got)
	}

	if got := NewApp().Run([]string{
		"execute",
		"--artifact", filepath.Join(projectRoot, ".harness", "artifacts", "cli-execute-smoke"),
	}); got != 0 {
		t.Fatalf("expected zero exit code for execute, got %d", got)
	}

	if _, err := os.Stat(filepath.Join(projectRoot, ".harness", "artifacts", "cli-execute-smoke", "terminal_summary.md")); err != nil {
		t.Fatalf("expected execute command to create terminal summary: %v", err)
	}
}

func TestAppRunValidatesArtifactForIntegrationResultSchema(t *testing.T) {
	projectRoot := t.TempDir()
	if err := copyDirectory(
		filepath.Join(repoRootFromCLIPackage(t), ".harness", "templates"),
		filepath.Join(projectRoot, ".harness", "templates"),
	); err != nil {
		t.Fatalf("failed to copy templates: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".git"), []byte("gitdir: test\n"), 0o644); err != nil {
		t.Fatalf("failed to write git marker: %v", err)
	}
	artifactPath := filepath.Join(projectRoot, ".harness", "artifacts", "sample", "integration_result.yaml")
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatalf("failed to create artifact directory: %v", err)
	}
	if err := os.WriteFile(artifactPath, []byte(`summary: handoff
files_changed: []
validation: []
risks: []
follow_up: []
evidence_quality: draft
release_readiness: ready
blocking_issues: []
`), 0o644); err != nil {
		t.Fatalf("failed to write integration_result fixture: %v", err)
	}
	originalStdout := os.Stdout
	stdoutRead, stdoutWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	os.Stdout = stdoutWrite
	t.Cleanup(func() {
		os.Stdout = originalStdout
	})

	if got := NewApp().Run([]string{
		"validate-artifact",
		"--file", artifactPath,
		"--schema", "integration_result",
	}); got != 0 {
		t.Fatalf("expected zero exit code for validate-artifact, got %d", got)
	}
	_ = stdoutWrite.Close()
	var stdout bytes.Buffer
	if _, err := stdout.ReadFrom(stdoutRead); err != nil {
		t.Fatalf("failed to read stdout: %v", err)
	}

	if !strings.Contains(stdout.String(), "Artifact is valid") {
		t.Fatalf("unexpected validate-artifact output: %q", stdout.String())
	}
}

func TestAppRunIntegrateProducesIntegrationResult(t *testing.T) {
	projectRoot, requestPath := prepareSmokeProjectForCLI(t)

	if got := NewApp().Run([]string{
		"run",
		"--request", requestPath,
		"--project-root", projectRoot,
		"--task-id", "cli-integrate-smoke",
	}); got != 0 {
		t.Fatalf("expected zero exit code for run, got %d", got)
	}
	if got := NewApp().Run([]string{
		"execute",
		"--artifact", filepath.Join(projectRoot, ".harness", "artifacts", "cli-integrate-smoke"),
	}); got != 0 {
		t.Fatalf("expected zero exit code for execute, got %d", got)
	}

	fakeBin := t.TempDir()
	fakeCodex := filepath.Join(fakeBin, "codex")
	if err := os.WriteFile(fakeCodex, []byte(`#!/usr/bin/env python3
import json
import os
import sys

output_path = None
for index, value in enumerate(sys.argv):
    if value == "--output-last-message" and index + 1 < len(sys.argv):
        output_path = sys.argv[index + 1]
        break

os.makedirs(os.path.dirname(output_path), exist_ok=True)
with open(output_path, "w", encoding="utf-8") as handle:
    json.dump({
        "summary": "Smoke integrator handoff.",
        "files_changed": [],
        "validation": [],
        "risks": [],
        "follow_up": [],
        "evidence_quality": "adequate",
        "release_readiness": "conditional",
        "blocking_issues": []
    }, handle)
`), 0o755); err != nil {
		t.Fatalf("failed to write fake codex: %v", err)
	}

	originalPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", fakeBin+string(os.PathListSeparator)+originalPath); err != nil {
		t.Fatalf("failed to set PATH: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Setenv("PATH", originalPath)
	})

	originalStdout := os.Stdout
	stdoutRead, stdoutWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	os.Stdout = stdoutWrite
	t.Cleanup(func() {
		os.Stdout = originalStdout
	})

	if got := NewApp().Run([]string{
		"integrate",
		"--artifact", filepath.Join(projectRoot, ".harness", "artifacts", "cli-integrate-smoke"),
		"--project-root", projectRoot,
	}); got != 0 {
		t.Fatalf("expected zero exit code for integrate, got %d", got)
	}
	_ = stdoutWrite.Close()
	var stdout bytes.Buffer
	if _, err := stdout.ReadFrom(stdoutRead); err != nil {
		t.Fatalf("failed to read stdout: %v", err)
	}

	if _, err := os.Stat(filepath.Join(projectRoot, ".harness", "artifacts", "cli-integrate-smoke", "integration_result.yaml")); err != nil {
		t.Fatalf("expected integrate command to create integration_result.yaml: %v", err)
	}
	if !strings.Contains(stdout.String(), "integration completed") {
		t.Fatalf("unexpected integrate output: %q", stdout.String())
	}
}

func TestAppRunVerifiesLearningStateWithoutMutatingSnapshots(t *testing.T) {
	repoRoot := repoRootFromCLIPackage(t)
	learningFiles := []string{
		filepath.Join(repoRoot, ".harness", "learning", "review_queue.yaml"),
		filepath.Join(repoRoot, ".harness", "learning", "hardening_queue.yaml"),
		filepath.Join(repoRoot, ".harness", "learning", "family_evidence_index.yaml"),
	}
	before := make(map[string][]byte, len(learningFiles))
	for _, file := range learningFiles {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("failed to read %s: %v", file, err)
		}
		before[file] = data
	}

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("failed to change working directory: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	originalStdout := os.Stdout
	stdoutRead, stdoutWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	os.Stdout = stdoutWrite
	t.Cleanup(func() {
		os.Stdout = originalStdout
	})

	if got := NewApp().Run([]string{"verify-learning-state"}); got != 0 {
		t.Fatalf("expected zero exit code for verify-learning-state, got %d", got)
	}
	_ = stdoutWrite.Close()
	var stdout bytes.Buffer
	if _, err := stdout.ReadFrom(stdoutRead); err != nil {
		t.Fatalf("failed to read stdout: %v", err)
	}
	if !strings.Contains(stdout.String(), "Learning state verification passed") {
		t.Fatalf("unexpected verify-learning-state output: %q", stdout.String())
	}

	for _, file := range learningFiles {
		after, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("failed to read %s after verification: %v", file, err)
		}
		if string(after) != string(before[file]) {
			t.Fatalf("expected %s to remain unchanged", file)
		}
	}
}

func TestAppRunInitializesUserOutcomeFeedbackDraft(t *testing.T) {
	projectRoot := t.TempDir()
	if err := RunInit([]string{"--project-root", projectRoot}); err != nil {
		t.Fatalf("RunInit returned error: %v", err)
	}

	repoRoot := repoRootFromCLIPackage(t)
	artifactPath := copyStandardRouteFixtureForCLI(t, repoRoot, projectRoot, "tighten_validation")

	originalStdout := os.Stdout
	stdoutRead, stdoutWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	os.Stdout = stdoutWrite
	t.Cleanup(func() {
		os.Stdout = originalStdout
	})

	if got := NewApp().Run([]string{
		"init-user-outcome-feedback",
		"--artifact", artifactPath,
	}); got != 0 {
		t.Fatalf("expected zero exit code for init-user-outcome-feedback, got %d", got)
	}
	_ = stdoutWrite.Close()
	var stdout bytes.Buffer
	if _, err := stdout.ReadFrom(stdoutRead); err != nil {
		t.Fatalf("failed to read stdout: %v", err)
	}

	if !strings.Contains(stdout.String(), ".harness/learning/feedback/") {
		t.Fatalf("unexpected init-user-outcome-feedback output: %q", stdout.String())
	}
}

func TestAppRunRejectsNonEmptyExistingArtifactDirectory(t *testing.T) {
	projectRoot, requestPath := prepareSmokeProjectForCLI(t)
	artifactPath := filepath.Join(projectRoot, ".harness", "artifacts", "cli-run-smoke")
	if err := os.MkdirAll(artifactPath, 0o755); err != nil {
		t.Fatalf("failed to create artifact directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(artifactPath, "terminal_summary.md"), []byte("stale terminal summary\n"), 0o644); err != nil {
		t.Fatalf("failed to seed stale terminal summary: %v", err)
	}

	if got := NewApp().Run([]string{
		"run",
		"--request", requestPath,
		"--project-root", projectRoot,
		"--task-id", "cli-run-smoke",
	}); got == 0 {
		t.Fatalf("expected non-zero exit code for non-empty artifact directory")
	}

	summary, err := os.ReadFile(filepath.Join(artifactPath, "terminal_summary.md"))
	if err != nil {
		t.Fatalf("expected stale terminal summary to remain readable: %v", err)
	}
	if string(summary) != "stale terminal summary\n" {
		t.Fatalf("expected stale terminal summary to remain unchanged, got %q", string(summary))
	}
}

func TestAppRunPrintsComposeRequestErrorsToStderr(t *testing.T) {
	originalStdin := os.Stdin
	originalStdout := os.Stdout
	originalStderr := os.Stderr

	inputFile, err := os.CreateTemp(t.TempDir(), "draft-*.json")
	if err != nil {
		t.Fatalf("failed to create temp input file: %v", err)
	}
	if _, err := inputFile.WriteString(`{"task_type":"bug_fix","goal":"Fix the bug"}`); err != nil {
		t.Fatalf("failed to write draft: %v", err)
	}
	if _, err := inputFile.Seek(0, 0); err != nil {
		t.Fatalf("failed to rewind draft: %v", err)
	}
	t.Cleanup(func() {
		_ = inputFile.Close()
		os.Stdin = originalStdin
		os.Stdout = originalStdout
		os.Stderr = originalStderr
	})

	stderrRead, stderrWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stderr pipe: %v", err)
	}
	defer stderrRead.Close()

	os.Stdin = inputFile
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("failed to open devnull: %v", err)
	}
	defer devNull.Close()
	os.Stdout = devNull
	os.Stderr = stderrWrite

	exitCode := NewApp().Run([]string{"compose-request", "--stdin"})
	_ = stderrWrite.Close()

	if exitCode == 0 {
		t.Fatalf("expected non-zero exit code for compose-request failure, got %d", exitCode)
	}

	var stderr bytes.Buffer
	if _, err := stderr.ReadFrom(stderrRead); err != nil {
		t.Fatalf("failed to read stderr: %v", err)
	}
	if !strings.Contains(stderr.String(), "project_root is required") {
		t.Fatalf("expected compose-request error on stderr, got %q", stderr.String())
	}
}

func TestRunValidateRequestAcceptsFixture(t *testing.T) {
	repoRoot := repoRootFromCLIPackage(t)
	projectRoot := t.TempDir()
	requestPath := filepath.Join(projectRoot, ".harness", "requests", "request.yaml")
	if err := os.MkdirAll(filepath.Dir(requestPath), 0o755); err != nil {
		t.Fatalf("failed to create request directory: %v", err)
	}
	requestBody, err := os.ReadFile(filepath.Join(repoRoot, "test", "fixtures", "valid_request.yaml"))
	if err != nil {
		t.Fatalf("failed to read fixture request: %v", err)
	}
	if err := os.WriteFile(requestPath, requestBody, 0o644); err != nil {
		t.Fatalf("failed to write fixture request: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".git"), []byte("gitdir: test\n"), 0o644); err != nil {
		t.Fatalf("failed to write git marker: %v", err)
	}
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
	var stdout bytes.Buffer

	err = RunValidateRequest([]string{"--request", requestPath}, &stdout)
	if err != nil {
		t.Fatalf("RunValidateRequest returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "Request is valid") {
		t.Fatalf("unexpected validate-request output: %q", stdout.String())
	}
}

func TestRunValidateRequestDiscoversProjectFromAbsoluteRequestPath(t *testing.T) {
	repoRoot := repoRootFromCLIPackage(t)
	projectRoot := t.TempDir()
	requestPath := filepath.Join(projectRoot, ".harness", "requests", "request.yaml")
	if err := os.MkdirAll(filepath.Dir(requestPath), 0o755); err != nil {
		t.Fatalf("failed to create request directory: %v", err)
	}
	requestBody, err := os.ReadFile(filepath.Join(repoRoot, "test", "fixtures", "valid_request.yaml"))
	if err != nil {
		t.Fatalf("failed to read fixture request: %v", err)
	}
	if err := os.WriteFile(requestPath, requestBody, 0o644); err != nil {
		t.Fatalf("failed to write fixture request: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".git"), []byte("gitdir: test\n"), 0o644); err != nil {
		t.Fatalf("failed to write git marker: %v", err)
	}

	outsideWD := t.TempDir()
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir(outsideWD); err != nil {
		t.Fatalf("failed to change working directory: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	var stdout bytes.Buffer
	err = RunValidateRequest([]string{"--request", requestPath}, &stdout)
	if err != nil {
		t.Fatalf("RunValidateRequest returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "Request is valid") {
		t.Fatalf("unexpected validate-request output: %q", stdout.String())
	}
}

func TestRunValidateRequestRejectsRequestOutsideDiscoveredProjectRoot(t *testing.T) {
	repoRoot := repoRootFromCLIPackage(t)
	projectRoot := t.TempDir()
	outsideRoot := t.TempDir()
	requestPath := filepath.Join(outsideRoot, "request.yaml")
	requestBody, err := os.ReadFile(filepath.Join(repoRoot, "test", "fixtures", "valid_request.yaml"))
	if err != nil {
		t.Fatalf("failed to read fixture request: %v", err)
	}
	if err := os.WriteFile(requestPath, requestBody, 0o644); err != nil {
		t.Fatalf("failed to write request fixture: %v", err)
	}

	if err := os.WriteFile(filepath.Join(projectRoot, ".git"), []byte("gitdir: test\n"), 0o644); err != nil {
		t.Fatalf("failed to write git marker: %v", err)
	}
	symlinkPath := filepath.Join(projectRoot, ".harness", "requests", "external-request.yaml")
	if err := os.MkdirAll(filepath.Dir(symlinkPath), 0o755); err != nil {
		t.Fatalf("failed to create symlink directory: %v", err)
	}
	if err := os.Symlink(requestPath, symlinkPath); err != nil {
		t.Fatalf("failed to create symlinked request path: %v", err)
	}

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

	var stdout bytes.Buffer
	err = RunValidateRequest([]string{"--request", symlinkPath}, &stdout)
	if err == nil {
		t.Fatalf("expected validate-request to reject a request outside %q", projectRoot)
	}
	if !strings.Contains(err.Error(), "project root") {
		t.Fatalf("expected project-root confinement error, got %v", err)
	}
}

func TestRunRouteEvaluationAcceptsEvaluationFilePath(t *testing.T) {
	repoRoot := repoRootFromCLIPackage(t)
	projectRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectRoot, ".git"), []byte("gitdir: test\n"), 0o644); err != nil {
		t.Fatalf("failed to write git marker: %v", err)
	}
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

	artifactPath := copyStandardRouteFixtureForCLI(t, repoRoot, projectRoot, "tighten_validation")
	var stdout bytes.Buffer

	err = RunRouteEvaluation(
		[]string{"--artifact", filepath.Join(artifactPath, "evaluation_result.yaml")},
		&stdout,
	)
	if err != nil {
		t.Fatalf("RunRouteEvaluation returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "status=tightening_validation") {
		t.Fatalf("unexpected route-evaluation output: %q", stdout.String())
	}
}

func TestRunRouteEvaluationDiscoversProjectFromAbsoluteArtifactPath(t *testing.T) {
	repoRoot := repoRootFromCLIPackage(t)
	projectRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectRoot, ".git"), []byte("gitdir: test\n"), 0o644); err != nil {
		t.Fatalf("failed to write git marker: %v", err)
	}

	outsideWD := t.TempDir()
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir(outsideWD); err != nil {
		t.Fatalf("failed to change working directory: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	artifactPath := copyStandardRouteFixtureForCLI(t, repoRoot, projectRoot, "tighten_validation")
	var stdout bytes.Buffer

	err = RunRouteEvaluation(
		[]string{"--artifact", filepath.Join(artifactPath, "evaluation_result.yaml")},
		&stdout,
	)
	if err != nil {
		t.Fatalf("RunRouteEvaluation returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "status=tightening_validation") {
		t.Fatalf("unexpected route-evaluation output: %q", stdout.String())
	}
}

func repoRootFromCLIPackage(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("failed to resolve repo root: %v", err)
	}
	return root
}

func copyStandardRouteFixtureForCLI(t *testing.T, repoRoot, projectRoot, fixtureName string) string {
	t.Helper()
	sourceRoot := filepath.Join(repoRoot, "test", "fixtures", "standard_route", fixtureName)
	targetRoot := filepath.Join(projectRoot, ".harness", "artifacts", fixtureName)

	if err := filepath.Walk(sourceRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(targetRoot, relative)
		if info.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(destination, data, info.Mode())
	}); err != nil {
		t.Fatalf("failed to copy standard-route fixture %q: %v", fixtureName, err)
	}

	return targetRoot
}

func prepareSmokeProjectForCLI(t *testing.T) (string, string) {
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

	requestBody, err := os.ReadFile(filepath.Join(repoRootFromCLIPackage(t), ".harness", "requests", "rail-bootstrap-smoke.yaml"))
	if err != nil {
		t.Fatalf("failed to read smoke request fixture: %v", err)
	}
	requestPath := filepath.Join(projectRoot, ".harness", "requests", "rail-bootstrap-smoke.yaml")
	if err := os.WriteFile(requestPath, requestBody, 0o644); err != nil {
		t.Fatalf("failed to write smoke request fixture: %v", err)
	}

	return projectRoot, requestPath
}

func copyDirectory(sourcePath, destinationPath string) error {
	if err := os.MkdirAll(destinationPath, 0o755); err != nil {
		return err
	}
	return filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(sourcePath, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destinationPath, relative)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}
