package cli

import (
	"bytes"
	"encoding/json"
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
		"auth",
		"init-user-outcome-feedback",
		"init-learning-review",
		"init-hardening-review",
		"run",
		"execute",
		"supervise",
		"status",
		"result",
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

func runAppAndCaptureStdout(t *testing.T, args []string) (int, string) {
	t.Helper()
	return captureAppStdout(t, func() int {
		return NewApp().Run(args)
	})
}

func runAppAndCaptureStderr(t *testing.T, args []string) (int, string) {
	t.Helper()

	originalStderr := os.Stderr
	stderrRead, stderrWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stderr pipe: %v", err)
	}
	os.Stderr = stderrWrite
	defer func() {
		os.Stderr = originalStderr
		_ = stderrRead.Close()
		_ = stderrWrite.Close()
	}()

	var stderr bytes.Buffer
	readDone := make(chan error, 1)
	go func() {
		_, err := stderr.ReadFrom(stderrRead)
		readDone <- err
	}()

	exitCode := NewApp().Run(args)
	_ = stderrWrite.Close()
	os.Stderr = originalStderr
	if err := <-readDone; err != nil {
		t.Fatalf("failed to read stderr: %v", err)
	}
	return exitCode, stderr.String()
}

func captureAppStdout(t *testing.T, run func() int) (int, string) {
	t.Helper()

	originalStdout := os.Stdout
	stdoutRead, stdoutWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	os.Stdout = stdoutWrite
	defer func() {
		os.Stdout = originalStdout
		_ = stdoutRead.Close()
		_ = stdoutWrite.Close()
	}()

	var stdout bytes.Buffer
	readDone := make(chan error, 1)
	go func() {
		_, err := stdout.ReadFrom(stdoutRead)
		readDone <- err
	}()

	exitCode := run()
	_ = stdoutWrite.Close()
	os.Stdout = originalStdout
	if err := <-readDone; err != nil {
		t.Fatalf("failed to read stdout: %v", err)
	}
	return exitCode, stdout.String()
}

func TestAppRunVersion(t *testing.T) {
	got, output := runAppAndCaptureStdout(t, []string{"version"})
	if got != 0 {
		t.Fatalf("expected zero exit code for version, got %d", got)
	}
	if output == "" {
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

	got, output := captureAppStdout(t, func() int {
		if got := NewApp().Run([]string{"install-codex-skill", "--codex-home", codexHome}); got != 0 {
			return got
		}
		return NewApp().Run([]string{"doctor", "--codex-home", codexHome})
	})
	if got != 0 {
		t.Fatalf("expected zero exit code for install-codex-skill/doctor, got %d", got)
	}
	if !strings.Contains(output, "Codex skill installed:") {
		t.Fatalf("unexpected install-codex-skill output: %q", output)
	}
	if !strings.Contains(output, "Codex skill: installed") {
		t.Fatalf("unexpected doctor output: %q", output)
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

func TestAppRunAllocatesImplicitTaskArtifactWhenDefaultExists(t *testing.T) {
	projectRoot, requestPath := prepareSmokeProjectForCLIWithDefaultRequestName(t)
	seedOccupiedCLIArtifact(t, projectRoot, "request")

	got, output := runAppAndCaptureStdout(t, []string{
		"run",
		"--request", requestPath,
		"--project-root", projectRoot,
	})
	if got != 0 {
		t.Fatalf("expected zero exit code for implicit run allocation, got %d", got)
	}
	artifactPath := strings.TrimSpace(output)
	if !strings.HasSuffix(filepath.ToSlash(artifactPath), "/.harness/artifacts/request-2") {
		t.Fatalf("expected run output to point at request-2 artifact, got %q", output)
	}
	if _, err := os.Stat(filepath.Join(artifactPath, "workflow.json")); err != nil {
		t.Fatalf("expected request-2 workflow artifact: %v", err)
	}
	if _, err := os.Stat(filepath.Join(artifactPath, "state.json")); err != nil {
		t.Fatalf("expected request-2 state artifact: %v", err)
	}
	assertCLITaskID(t, filepath.Join(artifactPath, "workflow.json"), "request-2")
	assertCLITaskID(t, filepath.Join(artifactPath, "state.json"), "request-2")
}

func TestAppRunRejectsBlankTaskID(t *testing.T) {
	projectRoot, requestPath := prepareSmokeProjectForCLIWithDefaultRequestName(t)

	got, output := runAppAndCaptureStderr(t, []string{
		"run",
		"--request", requestPath,
		"--project-root", projectRoot,
		"--task-id", "   ",
	})
	if got == 0 {
		t.Fatalf("expected non-zero exit code for blank task id")
	}
	if !strings.Contains(output, "task id must not be blank") {
		t.Fatalf("expected blank task id error, got %q", output)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, ".harness", "artifacts", "request")); !os.IsNotExist(err) {
		t.Fatalf("expected blank explicit task id not to create request artifact, stat err=%v", err)
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

	got, output := runAppAndCaptureStdout(t, []string{
		"status",
		"--artifact", filepath.Join(projectRoot, ".harness", "artifacts", "cli-status-smoke"),
	})
	if got != 0 {
		t.Fatalf("expected zero exit code for status, got %d", got)
	}
	for _, fragment := range []string{"status: initialized", "phase: bootstrap", "artifact:"} {
		if !strings.Contains(output, fragment) {
			t.Fatalf("expected status output to contain %q, got:\n%s", fragment, output)
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

	got, output := runAppAndCaptureStdout(t, []string{
		"status",
		"--artifact", ".harness/artifacts/cli-status-subdir",
	})
	if got != 0 {
		t.Fatalf("expected zero exit code for status from subdirectory, got %d", got)
	}
	for _, fragment := range []string{"status: initialized", "phase: bootstrap", "current actor: planner"} {
		if !strings.Contains(output, fragment) {
			t.Fatalf("expected synthesized status output to contain %q, got:\n%s", fragment, output)
		}
	}
}

func TestAppRunPrintsHarnessResultForResultCommand(t *testing.T) {
	projectRoot, requestPath := prepareSmokeProjectForCLI(t)

	if got := NewApp().Run([]string{
		"run",
		"--request", requestPath,
		"--project-root", projectRoot,
		"--task-id", "cli-result-smoke",
	}); got != 0 {
		t.Fatalf("expected zero exit code for run, got %d", got)
	}
	artifactPath := filepath.Join(projectRoot, ".harness", "artifacts", "cli-result-smoke")
	if got := NewApp().Run([]string{
		"execute",
		"--artifact", artifactPath,
	}); got != 0 {
		t.Fatalf("expected zero exit code for execute, got %d", got)
	}

	got, output := runAppAndCaptureStdout(t, []string{
		"result",
		"--artifact", artifactPath,
	})
	if got != 0 {
		t.Fatalf("expected zero exit code for result, got %d", got)
	}
	for _, fragment := range []string{"Rail result: passed", "What happened:", "terminal_summary.md"} {
		if !strings.Contains(output, fragment) {
			t.Fatalf("expected result output to contain %q, got:\n%s", fragment, output)
		}
	}
}

func TestAppRunPrintsHarnessResultJSONForResultCommand(t *testing.T) {
	projectRoot, requestPath := prepareSmokeProjectForCLI(t)

	if got := NewApp().Run([]string{
		"run",
		"--request", requestPath,
		"--project-root", projectRoot,
		"--task-id", "cli-result-json",
	}); got != 0 {
		t.Fatalf("expected zero exit code for run, got %d", got)
	}

	got, output := runAppAndCaptureStdout(t, []string{
		"result",
		"--artifact", filepath.Join(projectRoot, ".harness", "artifacts", "cli-result-json"),
		"--json",
	})
	if got != 0 {
		t.Fatalf("expected zero exit code for result --json, got %d", got)
	}

	var decoded struct {
		SchemaVersion int    `json:"schema_version"`
		Status        string `json:"status"`
		Terminal      bool   `json:"terminal"`
	}
	if err := json.NewDecoder(strings.NewReader(output)).Decode(&decoded); err != nil {
		t.Fatalf("failed to decode result JSON: %v", err)
	}
	if decoded.SchemaVersion != 1 {
		t.Fatalf("expected schema_version 1, got %d", decoded.SchemaVersion)
	}
	if decoded.Status != "initialized" {
		t.Fatalf("expected initialized status, got %q", decoded.Status)
	}
	if decoded.Terminal {
		t.Fatalf("expected non-terminal result before execution")
	}
}

func TestAppRunPrintsLatestHarnessResultForResultCommand(t *testing.T) {
	projectRoot := t.TempDir()
	writeCLIResultArtifact(t, projectRoot, "older", "initialized", "bootstrap", "planner", "2026-04-27T01:00:00Z")
	writeCLIResultArtifact(t, projectRoot, "newer", "in_progress", "actor_execution", "executor", "2026-04-27T02:00:00Z")

	got, output := runAppAndCaptureStdout(t, []string{
		"result",
		"--latest",
		"--project-root", projectRoot,
	})
	if got != 0 {
		t.Fatalf("expected zero exit code for result --latest, got %d", got)
	}
	for _, fragment := range []string{"Rail result: in_progress", "Phase: actor_execution", "Current actor: executor"} {
		if !strings.Contains(output, fragment) {
			t.Fatalf("expected latest result output to contain %q, got:\n%s", fragment, output)
		}
	}
}

func TestRunResultPrintsLatestHarnessResultJSON(t *testing.T) {
	projectRoot := t.TempDir()
	writeCLIResultArtifact(t, projectRoot, "older", "initialized", "bootstrap", "planner", "2026-04-27T01:00:00Z")
	writeCLIResultArtifact(t, projectRoot, "newer", "interrupted", "actor_execution", "generator", "2026-04-27T02:00:00Z")

	var stdout bytes.Buffer
	if err := RunResult([]string{
		"--latest",
		"--project-root", projectRoot,
		"--json",
	}, &stdout); err != nil {
		t.Fatalf("RunResult returned error: %v", err)
	}

	var decoded struct {
		Status       string `json:"status"`
		CurrentActor string `json:"current_actor"`
		UpdatedAt    string `json:"updated_at"`
	}
	if err := json.NewDecoder(&stdout).Decode(&decoded); err != nil {
		t.Fatalf("failed to decode latest result JSON: %v", err)
	}
	if decoded.Status != "interrupted" {
		t.Fatalf("expected interrupted status, got %q", decoded.Status)
	}
	if decoded.CurrentActor != "generator" {
		t.Fatalf("expected generator current actor, got %q", decoded.CurrentActor)
	}
	if decoded.UpdatedAt != "2026-04-27T02:00:00Z" {
		t.Fatalf("expected newest updated_at, got %q", decoded.UpdatedAt)
	}
}

func TestRunResultRejectsArtifactAndLatest(t *testing.T) {
	var stdout bytes.Buffer
	err := RunResult([]string{
		"--artifact", filepath.Join(t.TempDir(), ".harness", "artifacts", "sample"),
		"--latest",
		"--project-root", t.TempDir(),
	}, &stdout)
	if err == nil {
		t.Fatalf("expected mutually exclusive artifact/latest error")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("expected mutually exclusive error, got %v", err)
	}
}

func TestRunResultRejectsLatestWithoutProjectRoot(t *testing.T) {
	var stdout bytes.Buffer
	err := RunResult([]string{"--latest"}, &stdout)
	if err == nil {
		t.Fatalf("expected missing project-root error")
	}
	if !strings.Contains(err.Error(), "requires --project-root") {
		t.Fatalf("expected project-root error, got %v", err)
	}
}

func TestRunResultRejectsProjectRootWithoutLatest(t *testing.T) {
	var stdout bytes.Buffer
	err := RunResult([]string{"--project-root", t.TempDir()}, &stdout)
	if err == nil {
		t.Fatalf("expected project-root without latest error")
	}
	if !strings.Contains(err.Error(), "--project-root requires --latest") {
		t.Fatalf("expected project-root without latest error, got %v", err)
	}
}

func TestRunResultRejectsUnknownFlag(t *testing.T) {
	var stdout bytes.Buffer
	err := RunResult([]string{"--future-result-mode"}, &stdout)
	if err == nil {
		t.Fatalf("expected unknown flag error")
	}
	if !strings.Contains(err.Error(), "unknown result flag") {
		t.Fatalf("expected unknown flag error, got %v", err)
	}
}

func TestAppRunResultResolvesArtifactPathFromProjectSubdirectory(t *testing.T) {
	projectRoot, requestPath := prepareSmokeProjectForCLI(t)

	if got := NewApp().Run([]string{
		"run",
		"--request", requestPath,
		"--project-root", projectRoot,
		"--task-id", "cli-result-subdir",
	}); got != 0 {
		t.Fatalf("expected zero exit code for run, got %d", got)
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

	got, output := runAppAndCaptureStdout(t, []string{
		"result",
		"--artifact", ".harness/artifacts/cli-result-subdir",
	})
	if got != 0 {
		t.Fatalf("expected zero exit code for result from subdirectory, got %d", got)
	}
	for _, fragment := range []string{"Rail result: initialized", "What happened:", "Current actor: planner"} {
		if !strings.Contains(output, fragment) {
			t.Fatalf("expected result output to contain %q, got:\n%s", fragment, output)
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

func TestAppRunSupervisesArtifactForSuperviseCommand(t *testing.T) {
	projectRoot, requestPath := prepareSmokeProjectForCLI(t)

	if got := NewApp().Run([]string{
		"run",
		"--request", requestPath,
		"--project-root", projectRoot,
		"--task-id", "cli-supervise-smoke",
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
	var stdout bytes.Buffer
	readDone := make(chan error, 1)
	go func() {
		_, err := stdout.ReadFrom(stdoutRead)
		readDone <- err
	}()

	got := NewApp().Run([]string{
		"supervise",
		"--artifact", filepath.Join(projectRoot, ".harness", "artifacts", "cli-supervise-smoke"),
	})
	_ = stdoutWrite.Close()
	if err := <-readDone; err != nil {
		t.Fatalf("failed to read stdout: %v", err)
	}
	if got != 0 {
		t.Fatalf("expected zero exit code for supervise, got %d", got)
	}
	for _, fragment := range []string{"supervised", "Rail result: passed", "What happened:", "terminal_summary.md"} {
		if !strings.Contains(stdout.String(), fragment) {
			t.Fatalf("expected supervise output to contain %q, got:\n%s", fragment, stdout.String())
		}
	}
}

func TestAppRunSuperviseAcceptsRelativeArtifactPathFromParent(t *testing.T) {
	projectRoot, requestPath := prepareSmokeProjectForCLI(t)

	if got := NewApp().Run([]string{
		"run",
		"--request", requestPath,
		"--project-root", projectRoot,
		"--task-id", "cli-supervise-relative-parent",
	}); got != 0 {
		t.Fatalf("expected zero exit code for run, got %d", got)
	}

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir(filepath.Dir(projectRoot)); err != nil {
		t.Fatalf("failed to change to project parent: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	relativeArtifactPath := filepath.Join(filepath.Base(projectRoot), ".harness", "artifacts", "cli-supervise-relative-parent")
	if got := NewApp().Run([]string{
		"supervise",
		"--artifact", relativeArtifactPath,
	}); got != 0 {
		t.Fatalf("expected zero exit code for supervise with parent-relative artifact path, got %d", got)
	}
}

func TestAppRunSupervisePrintsStatusWhenBlocked(t *testing.T) {
	projectRoot, requestPath := prepareSmokeProjectForCLI(t)

	if got := NewApp().Run([]string{
		"run",
		"--request", requestPath,
		"--project-root", projectRoot,
		"--task-id", "cli-supervise-blocked",
	}); got != 0 {
		t.Fatalf("expected zero exit code for run, got %d", got)
	}

	artifactPath := filepath.Join(projectRoot, ".harness", "artifacts", "cli-supervise-blocked")
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
	var stdout bytes.Buffer
	readDone := make(chan error, 1)
	go func() {
		_, err := stdout.ReadFrom(stdoutRead)
		readDone <- err
	}()

	got := NewApp().Run([]string{
		"supervise",
		"--artifact", artifactPath,
		"--retry-budget", "2",
	})
	_ = stdoutWrite.Close()
	if err := <-readDone; err != nil {
		t.Fatalf("failed to read stdout: %v", err)
	}
	if got == 0 {
		t.Fatalf("expected non-zero exit code for blocked supervise")
	}
	for _, fragment := range []string{"Rail result: interrupted", "Phase: actor_resolution", "Current actor: missing_actor"} {
		if !strings.Contains(stdout.String(), fragment) {
			t.Fatalf("expected blocked supervise output to contain %q, got:\n%s", fragment, stdout.String())
		}
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

	got, output := runAppAndCaptureStdout(t, []string{
		"execute",
		"--artifact", artifactPath,
	})
	if got == 0 {
		t.Fatalf("expected non-zero exit code for interrupted execute")
	}
	for _, fragment := range []string{"Harness status", "status: interrupted", "phase: actor_resolution", "current actor: missing_actor"} {
		if !strings.Contains(output, fragment) {
			t.Fatalf("expected interrupted execute output to contain %q, got:\n%s", fragment, output)
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
	got, output := runAppAndCaptureStdout(t, []string{
		"validate-artifact",
		"--file", artifactPath,
		"--schema", "integration_result",
	})
	if got != 0 {
		t.Fatalf("expected zero exit code for validate-artifact, got %d", got)
	}

	if !strings.Contains(output, "Artifact is valid") {
		t.Fatalf("unexpected validate-artifact output: %q", output)
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
	resolvedFakeCodex, err := filepath.EvalSymlinks(fakeCodex)
	if err != nil {
		t.Fatalf("failed to resolve fake codex: %v", err)
	}
	if err := os.WriteFile(filepath.Join(fakeBin, ".rail-internal-test-codex"), []byte(filepath.Clean(resolvedFakeCodex)+"\n"), 0o600); err != nil {
		t.Fatalf("failed to write fake codex marker: %v", err)
	}

	originalPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", fakeBin+string(os.PathListSeparator)+originalPath); err != nil {
		t.Fatalf("failed to set PATH: %v", err)
	}
	t.Setenv("RAIL_INTERNAL_TEST_ALLOW_UNTRUSTED_CODEX_PATH", "rail-internal-tests-only")
	t.Setenv("RAIL_INTERNAL_TEST_CODEX_PATH", resolvedFakeCodex)
	t.Cleanup(func() {
		_ = os.Setenv("PATH", originalPath)
	})
	t.Setenv("RAIL_CODEX_AUTH_HOME", testRailCodexAuthHome(t))

	got, output := runAppAndCaptureStdout(t, []string{
		"integrate",
		"--artifact", filepath.Join(projectRoot, ".harness", "artifacts", "cli-integrate-smoke"),
		"--project-root", projectRoot,
	})
	if got != 0 {
		t.Fatalf("expected zero exit code for integrate, got %d", got)
	}

	if _, err := os.Stat(filepath.Join(projectRoot, ".harness", "artifacts", "cli-integrate-smoke", "integration_result.yaml")); err != nil {
		t.Fatalf("expected integrate command to create integration_result.yaml: %v", err)
	}
	if !strings.Contains(output, "integration completed") {
		t.Fatalf("unexpected integrate output: %q", output)
	}
}

func testRailCodexAuthHome(t *testing.T) string {
	t.Helper()
	authHome := t.TempDir()
	if err := os.Chmod(authHome, 0o700); err != nil {
		t.Fatalf("chmod fake auth home: %v", err)
	}
	if err := os.WriteFile(filepath.Join(authHome, ".rail-auth-home"), []byte("version: 1\n"), 0o600); err != nil {
		t.Fatalf("write fake rail auth marker: %v", err)
	}
	if err := os.WriteFile(filepath.Join(authHome, "auth.json"), []byte(`{"fake":"auth"}`), 0o600); err != nil {
		t.Fatalf("write fake auth.json: %v", err)
	}
	return authHome
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

	got, output := runAppAndCaptureStdout(t, []string{"verify-learning-state"})
	if got != 0 {
		t.Fatalf("expected zero exit code for verify-learning-state, got %d", got)
	}
	if !strings.Contains(output, "Learning state verification passed") {
		t.Fatalf("unexpected verify-learning-state output: %q", output)
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

	got, output := runAppAndCaptureStdout(t, []string{
		"init-user-outcome-feedback",
		"--artifact", artifactPath,
	})
	if got != 0 {
		t.Fatalf("expected zero exit code for init-user-outcome-feedback, got %d", got)
	}

	if !strings.Contains(output, ".harness/learning/feedback/") {
		t.Fatalf("unexpected init-user-outcome-feedback output: %q", output)
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

func TestRunValidateRequestRejectsCommandLikeValidationTargets(t *testing.T) {
	projectRoot := t.TempDir()
	requestPath := filepath.Join(projectRoot, ".harness", "requests", "request.yaml")
	if err := os.MkdirAll(filepath.Dir(requestPath), 0o755); err != nil {
		t.Fatalf("failed to create request directory: %v", err)
	}
	requestBody := `task_type: test_repair
goal: reject command-like validation targets
context:
  validation_targets:
    - go test ./...
constraints: []
definition_of_done:
  - reject command-like validation target
priority: medium
risk_tolerance: low
validation_profile: standard
`
	if err := os.WriteFile(requestPath, []byte(requestBody), 0o644); err != nil {
		t.Fatalf("failed to write request fixture: %v", err)
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
	if err == nil {
		t.Fatalf("expected validate-request to reject command-like validation_targets")
	}
	if !strings.Contains(err.Error(), "validation_targets") {
		t.Fatalf("expected validation_targets error, got %v", err)
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

func prepareSmokeProjectForCLIWithDefaultRequestName(t *testing.T) (string, string) {
	t.Helper()

	projectRoot, requestPath := prepareSmokeProjectForCLI(t)
	requestBody, err := os.ReadFile(requestPath)
	if err != nil {
		t.Fatalf("failed to read smoke request: %v", err)
	}
	defaultRequestPath := filepath.Join(projectRoot, ".harness", "requests", "request.yaml")
	if err := os.WriteFile(defaultRequestPath, requestBody, 0o644); err != nil {
		t.Fatalf("failed to write default request: %v", err)
	}
	return projectRoot, defaultRequestPath
}

func seedOccupiedCLIArtifact(t *testing.T, projectRoot, taskID string) {
	t.Helper()

	artifactPath := filepath.Join(projectRoot, ".harness", "artifacts", taskID)
	if err := os.MkdirAll(artifactPath, 0o755); err != nil {
		t.Fatalf("failed to create occupied artifact %q: %v", taskID, err)
	}
	if err := os.WriteFile(filepath.Join(artifactPath, "occupied.txt"), []byte("occupied: "+taskID+"\n"), 0o644); err != nil {
		t.Fatalf("failed to seed occupied artifact %q: %v", taskID, err)
	}
}

func assertCLITaskID(t *testing.T, path string, want string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("failed to decode %s: %v", path, err)
	}
	if got, _ := value["taskId"].(string); got != want {
		t.Fatalf("unexpected taskId in %s: got %q want %q", path, got, want)
	}
}

func writeCLIResultArtifact(t *testing.T, projectRoot, artifactID, status, phase, currentActor, updatedAt string) string {
	t.Helper()

	artifactPath := filepath.Join(projectRoot, ".harness", "artifacts", artifactID)
	if err := os.MkdirAll(artifactPath, 0o755); err != nil {
		t.Fatalf("failed to create result artifact %q: %v", artifactID, err)
	}
	body := strings.Join([]string{
		"status: " + status,
		"phase: " + phase,
		"current_actor: " + currentActor,
		"artifact_dir: " + artifactPath,
		"evidence:",
		"  - run_status.yaml",
		"next_step: Continue the harness workflow.",
		"updated_at: " + updatedAt,
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(artifactPath, "run_status.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("failed to write result artifact %q: %v", artifactID, err)
	}
	resolvedArtifactPath, err := filepath.EvalSymlinks(artifactPath)
	if err != nil {
		t.Fatalf("failed to resolve result artifact %q: %v", artifactID, err)
	}
	return resolvedArtifactPath
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
