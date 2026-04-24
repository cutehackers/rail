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
		"version",
		"init-request",
		"compose-request",
		"validate-request",
		"validate-artifact",
		"init",
		"install-codex-skill",
		"doctor",
		"init-user-outcome-feedback",
		"init-learning-review",
		"init-hardening-review",
		"run",
		"execute",
		"status",
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

func TestAppRunVersion(t *testing.T) {
	originalStdout := os.Stdout
	stdoutRead, stdoutWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	os.Stdout = stdoutWrite
	t.Cleanup(func() {
		os.Stdout = originalStdout
	})

	if got := NewApp().Run([]string{"version"}); got != 0 {
		t.Fatalf("expected zero exit code for version, got %d", got)
	}
	_ = stdoutWrite.Close()

	var stdout bytes.Buffer
	if _, err := stdout.ReadFrom(stdoutRead); err != nil {
		t.Fatalf("failed to read stdout: %v", err)
	}
	if got := stdout.String(); got == "" {
		t.Fatalf("expected version output")
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
	if _, err := os.Stat(filepath.Join(os.Getenv("CODEX_HOME"), "skills", "rail", "SKILL.md")); err != nil {
		t.Fatalf("expected init to register Codex skill: %v", err)
	}
}

func TestAppRunInstallsAndDoctorsCodexSkill(t *testing.T) {
	codexHome := t.TempDir()

	originalStdout := os.Stdout
	stdoutRead, stdoutWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	os.Stdout = stdoutWrite
	t.Cleanup(func() {
		os.Stdout = originalStdout
	})

	if got := NewApp().Run([]string{"install-codex-skill", "--codex-home", codexHome}); got != 0 {
		t.Fatalf("expected zero exit code for install-codex-skill, got %d", got)
	}
	if got := NewApp().Run([]string{"doctor", "--codex-home", codexHome}); got != 0 {
		t.Fatalf("expected zero exit code for doctor, got %d", got)
	}
	_ = stdoutWrite.Close()

	var stdout bytes.Buffer
	if _, err := stdout.ReadFrom(stdoutRead); err != nil {
		t.Fatalf("failed to read stdout: %v", err)
	}
	if !strings.Contains(stdout.String(), "Codex skill installed:") {
		t.Fatalf("unexpected install-codex-skill output: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Codex skill: installed") {
		t.Fatalf("unexpected doctor output: %q", stdout.String())
	}

	info, err := os.Lstat(filepath.Join(codexHome, "skills", "rail", "SKILL.md"))
	if err != nil {
		t.Fatalf("expected installed skill file: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("expected installed SKILL.md to be a regular file")
	}
}

func TestRunDoctorRepairHintPreservesCodexHomeOverride(t *testing.T) {
	codexHome := t.TempDir()
	var stdout bytes.Buffer

	err := RunDoctor([]string{"--codex-home", codexHome}, &stdout)
	if err == nil {
		t.Fatal("expected doctor to report missing Codex skill")
	}
	if !strings.Contains(err.Error(), "--codex-home") || !strings.Contains(err.Error(), codexHome) {
		t.Fatalf("expected repair hint to preserve custom Codex home, got %q", err.Error())
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

func TestAppRunPrintsArtifactStatusForStatusCommand(t *testing.T) {
	projectRoot, requestPath := prepareSmokeProjectForCLI(t)

	if got := NewApp().Run([]string{
		"run",
		"--request", requestPath,
		"--project-root", projectRoot,
		"--task-id", "cli-status-smoke",
	}); got != 0 {
		t.Fatalf("expected zero exit code for run, got %d", got)
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
		"status",
		"--artifact", filepath.Join(projectRoot, ".harness", "artifacts", "cli-status-smoke"),
	}); got != 0 {
		t.Fatalf("expected zero exit code for status, got %d", got)
	}
	_ = stdoutWrite.Close()

	var stdout bytes.Buffer
	if _, err := stdout.ReadFrom(stdoutRead); err != nil {
		t.Fatalf("failed to read stdout: %v", err)
	}
	for _, fragment := range []string{"status: initialized", "phase: bootstrap", "artifact:"} {
		if !strings.Contains(stdout.String(), fragment) {
			t.Fatalf("expected status output to contain %q, got:\n%s", fragment, stdout.String())
		}
	}
}

func TestAppRunStatusResolvesArtifactPathFromProjectSubdirectory(t *testing.T) {
	projectRoot, requestPath := prepareSmokeProjectForCLI(t)

	if got := NewApp().Run([]string{
		"run",
		"--request", requestPath,
		"--project-root", projectRoot,
		"--task-id", "cli-status-subdir",
	}); got != 0 {
		t.Fatalf("expected zero exit code for run, got %d", got)
	}
	if err := os.Remove(filepath.Join(projectRoot, ".harness", "artifacts", "cli-status-subdir", "run_status.yaml")); err != nil {
		t.Fatalf("failed to remove run_status.yaml fixture: %v", err)
	}

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir(filepath.Join(projectRoot, "smoke")); err != nil {
		t.Fatalf("failed to change to project subdirectory: %v", err)
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

	if got := NewApp().Run([]string{
		"status",
		"--artifact", ".harness/artifacts/cli-status-subdir",
	}); got != 0 {
		t.Fatalf("expected zero exit code for status from subdirectory, got %d", got)
	}
	_ = stdoutWrite.Close()

	var stdout bytes.Buffer
	if _, err := stdout.ReadFrom(stdoutRead); err != nil {
		t.Fatalf("failed to read stdout: %v", err)
	}
	for _, fragment := range []string{"status: initialized", "phase: bootstrap", "current actor: planner"} {
		if !strings.Contains(stdout.String(), fragment) {
			t.Fatalf("expected synthesized status output to contain %q, got:\n%s", fragment, stdout.String())
		}
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

	terminalSummaryPath := filepath.Join(projectRoot, ".harness", "artifacts", "cli-execute-smoke", "terminal_summary.md")
	if _, err := os.Stat(terminalSummaryPath); err != nil {
		t.Fatalf("expected execute command to create terminal summary: %v", err)
	}
	terminalSummary, err := os.ReadFile(terminalSummaryPath)
	if err != nil {
		t.Fatalf("expected terminal summary to be readable: %v", err)
	}
	if !strings.Contains(string(terminalSummary), "critic") {
		t.Fatalf("expected execute terminal summary to include critic traversal, got:\n%s", string(terminalSummary))
	}
}

func TestAppRunExecutePrintsStatusWhenHarnessIsInterrupted(t *testing.T) {
	projectRoot, requestPath := prepareSmokeProjectForCLI(t)

	if got := NewApp().Run([]string{
		"run",
		"--request", requestPath,
		"--project-root", projectRoot,
		"--task-id", "cli-execute-interrupted",
	}); got != 0 {
		t.Fatalf("expected zero exit code for run, got %d", got)
	}

	artifactPath := filepath.Join(projectRoot, ".harness", "artifacts", "cli-execute-interrupted")
	statePath := filepath.Join(artifactPath, "state.json")
	stateData, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("failed to read state fixture: %v", err)
	}
	updatedState := strings.Replace(string(stateData), `"currentActor": "planner"`, `"currentActor": "missing_actor"`, 1)
	if updatedState == string(stateData) {
		t.Fatalf("failed to mutate state fixture currentActor")
	}
	if err := os.WriteFile(statePath, []byte(updatedState), 0o644); err != nil {
		t.Fatalf("failed to write mutated state fixture: %v", err)
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
		"execute",
		"--artifact", artifactPath,
	}); got == 0 {
		t.Fatalf("expected non-zero exit code for interrupted execute")
	}
	_ = stdoutWrite.Close()

	var stdout bytes.Buffer
	if _, err := stdout.ReadFrom(stdoutRead); err != nil {
		t.Fatalf("failed to read stdout: %v", err)
	}
	for _, fragment := range []string{"Harness status", "status: interrupted", "phase: actor_resolution", "current actor: missing_actor"} {
		if !strings.Contains(stdout.String(), fragment) {
			t.Fatalf("expected interrupted execute output to contain %q, got:\n%s", fragment, stdout.String())
		}
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
	projectRoot := t.TempDir()
	if err := RunInit([]string{"--project-root", projectRoot}); err != nil {
		t.Fatalf("RunInit returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".git"), []byte("gitdir: test\n"), 0o644); err != nil {
		t.Fatalf("failed to write git marker: %v", err)
	}
	writeEmptyLearningSnapshotsForCLI(t, projectRoot)

	learningFiles := []string{
		filepath.Join(projectRoot, ".harness", "learning", "review_queue.yaml"),
		filepath.Join(projectRoot, ".harness", "learning", "hardening_queue.yaml"),
		filepath.Join(projectRoot, ".harness", "learning", "family_evidence_index.yaml"),
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
	if err := os.Chdir(projectRoot); err != nil {
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

func TestRunRouteEvaluationRewritesExecutionReportWithCriticReporting(t *testing.T) {
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

	artifactPath := copyStandardRouteFixtureForCLI(t, repoRoot, projectRoot, "blocked_environment")
	var stdout bytes.Buffer
	if err := RunRouteEvaluation([]string{"--artifact", artifactPath}, &stdout); err != nil {
		t.Fatalf("RunRouteEvaluation returned error: %v", err)
	}

	executionReport, err := os.ReadFile(filepath.Join(artifactPath, "execution_report.yaml"))
	if err != nil {
		t.Fatalf("failed to read rewritten execution_report.yaml: %v", err)
	}
	for _, fragment := range []string{
		"actor_profiles_used:",
		"critic_findings_applied:",
		"critic_to_evaluator_delta:",
	} {
		if !strings.Contains(string(executionReport), fragment) {
			t.Fatalf("expected execution report to contain %q, got:\n%s", fragment, string(executionReport))
		}
	}
}

func TestRunRouteEvaluationFailsWhenRequiredCriticReportIsMissing(t *testing.T) {
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

	artifactPath := copyStandardRouteFixtureForCLI(t, repoRoot, projectRoot, "blocked_environment")
	if err := os.Remove(filepath.Join(artifactPath, "critic_report.yaml")); err != nil {
		t.Fatalf("failed to remove critic_report.yaml: %v", err)
	}
	var stdout bytes.Buffer

	err = RunRouteEvaluation([]string{"--artifact", artifactPath}, &stdout)
	if err == nil {
		t.Fatalf("expected RunRouteEvaluation to fail without required critic_report")
	}
	if !strings.Contains(err.Error(), "critic_report") {
		t.Fatalf("expected critic_report error, got %v", err)
	}
}

func TestRunRouteEvaluationFailsWhenRequiredCriticReportIsMalformed(t *testing.T) {
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

	artifactPath := copyStandardRouteFixtureForCLI(t, repoRoot, projectRoot, "blocked_environment")
	if err := os.WriteFile(filepath.Join(artifactPath, "critic_report.yaml"), []byte(`priority_focus:
  - Preserve bounded supervisor routing.
missing_requirements: invalid
risk_hypotheses: []
validation_expectations: []
generator_guardrails: []
blocked_assumptions: []
`), 0o644); err != nil {
		t.Fatalf("failed to write malformed critic_report.yaml: %v", err)
	}
	var stdout bytes.Buffer

	err = RunRouteEvaluation([]string{"--artifact", artifactPath}, &stdout)
	if err == nil {
		t.Fatalf("expected RunRouteEvaluation to fail for malformed critic_report")
	}
	if !strings.Contains(err.Error(), "critic_report") {
		t.Fatalf("expected critic_report validation error, got %v", err)
	}
}

func TestRunRouteEvaluationFailsNonTerminalWhenRequiredCriticReportIsMissing(t *testing.T) {
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
	if err := os.Remove(filepath.Join(artifactPath, "critic_report.yaml")); err != nil {
		t.Fatalf("failed to remove critic_report.yaml: %v", err)
	}
	var stdout bytes.Buffer

	err = RunRouteEvaluation([]string{"--artifact", artifactPath}, &stdout)
	if err == nil {
		t.Fatalf("expected RunRouteEvaluation to fail without required non-terminal critic_report")
	}
	if !strings.Contains(err.Error(), "critic_report") {
		t.Fatalf("expected critic_report error, got %v", err)
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

	requestBody, err := os.ReadFile(filepath.Join(repoRootFromCLIPackage(t), "examples", "smoke-target", ".harness", "requests", "valid_request.yaml"))
	if err != nil {
		t.Fatalf("failed to read smoke request fixture: %v", err)
	}
	requestPath := filepath.Join(projectRoot, ".harness", "requests", "rail-bootstrap-smoke.yaml")
	if err := os.WriteFile(requestPath, requestBody, 0o644); err != nil {
		t.Fatalf("failed to write smoke request fixture: %v", err)
	}

	return projectRoot, requestPath
}

func writeEmptyLearningSnapshotsForCLI(t *testing.T, projectRoot string) {
	t.Helper()
	snapshots := map[string]string{
		filepath.Join(".harness", "learning", "review_queue.yaml"): `pending_candidate_groups: []
queue_generated_at: derived:empty
queue_sequence: 0
`,
		filepath.Join(".harness", "learning", "hardening_queue.yaml"): `pending_hardening_entries: []
queue_generated_at: derived:empty
queue_sequence: 0
`,
		filepath.Join(".harness", "learning", "family_evidence_index.yaml"): `latest_approved_memory_refs_by_family: {}
latest_confirmed_success_refs_by_family: {}
latest_failure_refs_by_family: {}
latest_review_decision_refs_by_family: {}
latest_provisional_candidate_dispositions_by_family: {}
index_generated_at: derived:empty
index_sequence: 0
`,
	}
	for relPath, body := range snapshots {
		if err := os.WriteFile(filepath.Join(projectRoot, relPath), []byte(body), 0o644); err != nil {
			t.Fatalf("failed to write %s: %v", relPath, err)
		}
	}
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
