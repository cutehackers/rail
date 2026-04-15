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
	want := []string{"compose-request", "validate-request", "init", "run", "execute", "route-evaluation"}
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
