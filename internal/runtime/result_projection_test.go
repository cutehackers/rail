package runtime

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestProjectHarnessResultInitializedArtifactAfterRun(t *testing.T) {
	projectRoot, requestPath := prepareSmokeProject(t)

	runner, err := NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}
	artifactPath, err := runner.Run(requestPath, "go-result-initialized")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	result, err := ProjectHarnessResultForArtifact(projectRoot, artifactPath)
	if err != nil {
		t.Fatalf("ProjectHarnessResultForArtifact returned error: %v", err)
	}

	if result.Status != "initialized" {
		t.Fatalf("unexpected status: got %q want initialized", result.Status)
	}
	if result.Phase != "bootstrap" {
		t.Fatalf("unexpected phase: got %q want bootstrap", result.Phase)
	}
	if result.Terminal {
		t.Fatalf("expected initialized projection to be non-terminal")
	}
	if result.CurrentActor != "planner" {
		t.Fatalf("unexpected current actor: got %q want planner", result.CurrentActor)
	}
	assertResultEvidence(t, result, "run_status.yaml", "run_status")
}

func TestProjectHarnessResultInterruptedArtifactAfterExecuteFailure(t *testing.T) {
	projectRoot, requestPath := prepareSmokeProject(t)

	runner, err := NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}
	artifactPath, err := runner.Run(requestPath, "go-result-interrupted")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	statePath := filepath.Join(artifactPath, "state.json")
	state, err := readState(statePath)
	if err != nil {
		t.Fatalf("failed to read state: %v", err)
	}
	state.CurrentActor = stringPtr("missing_actor")
	if err := writeJSON(statePath, state); err != nil {
		t.Fatalf("failed to persist invalid state: %v", err)
	}

	_, err = runner.Execute(artifactPath)
	if err == nil {
		t.Fatalf("expected Execute to fail for missing actor")
	}

	result, err := ProjectHarnessResultForArtifact(projectRoot, artifactPath)
	if err != nil {
		t.Fatalf("ProjectHarnessResultForArtifact returned error: %v", err)
	}

	if result.Status != "interrupted" {
		t.Fatalf("unexpected status: got %q want interrupted", result.Status)
	}
	if result.Phase != "actor_resolution" {
		t.Fatalf("unexpected phase: got %q want actor_resolution", result.Phase)
	}
	if result.CurrentActor != "missing_actor" {
		t.Fatalf("unexpected current actor: got %q want missing_actor", result.CurrentActor)
	}
	if strings.TrimSpace(result.RecommendedNextStep) == "" {
		t.Fatalf("expected interrupted result to include a recommended next step")
	}
	assertResultEvidence(t, result, "run_status.yaml", "run_status")
}

func TestProjectHarnessResultTerminalArtifactAfterSmokeExecute(t *testing.T) {
	projectRoot, requestPath := prepareSmokeProject(t)

	runner, err := NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}
	artifactPath, err := runner.Run(requestPath, "go-result-terminal")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if _, err := runner.Execute(artifactPath); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	result, err := ProjectHarnessResultForArtifact(projectRoot, artifactPath)
	if err != nil {
		t.Fatalf("ProjectHarnessResultForArtifact returned error: %v", err)
	}

	if result.Status != "passed" {
		t.Fatalf("unexpected status: got %q want passed", result.Status)
	}
	if !result.Terminal {
		t.Fatalf("expected terminal projection")
	}
	if strings.TrimSpace(result.HumanSummary) == "" {
		t.Fatalf("expected terminal projection to include a human summary")
	}
	assertResultEvidence(t, result, "run_status.yaml", "run_status")
	assertResultEvidence(t, result, "terminal_summary.md", "terminal_summary")
	assertResultEvidence(t, result, "evaluation_result.yaml", "evaluation_result")
	assertResultEvidence(t, result, "execution_report.yaml", "execution_report")
	assertResultEvidence(t, result, "supervisor_trace.md", "supervisor_trace")
}

func TestProjectHarnessResultDoesNotCreatePersistedResultFiles(t *testing.T) {
	projectRoot, requestPath := prepareSmokeProject(t)

	runner, err := NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}
	artifactPath, err := runner.Run(requestPath, "go-result-read-only")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if _, err := ProjectHarnessResultForArtifact(projectRoot, artifactPath); err != nil {
		t.Fatalf("ProjectHarnessResultForArtifact returned error: %v", err)
	}

	for _, name := range []string{"result.json", "result.md"} {
		if _, err := os.Stat(filepath.Join(artifactPath, name)); !os.IsNotExist(err) {
			t.Fatalf("expected %s not to be created, stat err=%v", name, err)
		}
	}
}

func TestProjectHarnessResultFailedStatusIsNotTerminalWithoutTerminalPhase(t *testing.T) {
	projectRoot, requestPath := prepareSmokeProject(t)

	runner, err := NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}
	artifactPath, err := runner.Run(requestPath, "go-result-failed-nonterminal")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if err := writeRunStatus(artifactPath, RunStatus{
		Status:       "failed",
		Phase:        "actor_execution",
		CurrentActor: "planner",
		Evidence:     []string{"state.json"},
		NextStep:     "Inspect run_status.yaml and continue.",
	}); err != nil {
		t.Fatalf("failed to seed failed run status: %v", err)
	}

	result, err := ProjectHarnessResultForArtifact(projectRoot, artifactPath)
	if err != nil {
		t.Fatalf("ProjectHarnessResultForArtifact returned error: %v", err)
	}

	if result.Status != "failed" {
		t.Fatalf("unexpected status: got %q want failed", result.Status)
	}
	if result.Terminal {
		t.Fatalf("expected failed status without terminal phase to remain non-terminal")
	}
}

func TestProjectHarnessResultRetryingWithInterruptionKindStaysRetrying(t *testing.T) {
	projectRoot, requestPath := prepareSmokeProject(t)

	runner, err := NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}
	artifactPath, err := runner.Run(requestPath, "go-result-retrying")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if err := writeRunStatus(artifactPath, RunStatus{
		Status:           "retrying",
		Phase:            "actor_execution",
		CurrentActor:     "planner",
		InterruptionKind: "actor_failed",
		Evidence:         []string{"state.json", "runs/"},
		NextStep:         "Rail supervise is retrying the interrupted actor loop.",
	}); err != nil {
		t.Fatalf("failed to seed retrying run status: %v", err)
	}

	result, err := ProjectHarnessResultForArtifact(projectRoot, artifactPath)
	if err != nil {
		t.Fatalf("ProjectHarnessResultForArtifact returned error: %v", err)
	}

	if result.Status != "retrying" {
		t.Fatalf("unexpected status: got %q want retrying", result.Status)
	}
	if result.Terminal {
		t.Fatalf("expected retrying status to remain non-terminal")
	}
}

func TestProjectHarnessResultFailedTerminalPhaseIsNotTerminal(t *testing.T) {
	projectRoot, requestPath := prepareSmokeProject(t)

	runner, err := NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}
	artifactPath, err := runner.Run(requestPath, "go-result-failed-terminal-phase")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if err := writeRunStatus(artifactPath, RunStatus{
		Status:       "failed",
		Phase:        "terminal",
		CurrentActor: "planner",
		Evidence:     []string{"state.json"},
		NextStep:     "Inspect run_status.yaml and continue.",
	}); err != nil {
		t.Fatalf("failed to seed failed run status: %v", err)
	}

	result, err := ProjectHarnessResultForArtifact(projectRoot, artifactPath)
	if err != nil {
		t.Fatalf("ProjectHarnessResultForArtifact returned error: %v", err)
	}

	if result.Terminal {
		t.Fatalf("expected failed status with terminal phase to remain non-terminal")
	}
}

func TestProjectHarnessResultNonTerminalDoesNotIncludeListedTerminalSummary(t *testing.T) {
	projectRoot, requestPath := prepareSmokeProject(t)

	runner, err := NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}
	artifactPath, err := runner.Run(requestPath, "go-result-nonterminal-terminal-summary")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(artifactPath, "terminal_summary.md"), []byte("# stale summary\n"), 0o644); err != nil {
		t.Fatalf("failed to seed terminal summary: %v", err)
	}
	if err := writeRunStatus(artifactPath, RunStatus{
		Status:       "in_progress",
		Phase:        "actor_execution",
		CurrentActor: "planner",
		Evidence:     []string{"state.json", "terminal_summary.md"},
		NextStep:     "Continue execution.",
	}); err != nil {
		t.Fatalf("failed to seed non-terminal run status: %v", err)
	}

	result, err := ProjectHarnessResultForArtifact(projectRoot, artifactPath)
	if err != nil {
		t.Fatalf("ProjectHarnessResultForArtifact returned error: %v", err)
	}

	if slices.Contains(result.Evidence, "terminal_summary.md") {
		t.Fatalf("expected non-terminal evidence to omit terminal_summary.md, got %#v", result.Evidence)
	}
	if _, ok := result.SourceArtifacts["terminal_summary"]; ok {
		t.Fatalf("expected non-terminal source artifacts to omit terminal_summary, got %#v", result.SourceArtifacts)
	}
}

func TestProjectHarnessResultSynthesizedStatusCitesStateInsteadOfMissingRunStatus(t *testing.T) {
	projectRoot, requestPath := prepareSmokeProject(t)

	runner, err := NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}
	artifactPath, err := runner.Run(requestPath, "go-result-synthesized-status")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if err := os.Remove(filepath.Join(artifactPath, runStatusFileName)); err != nil {
		t.Fatalf("failed to remove run status: %v", err)
	}

	result, err := ProjectHarnessResultForArtifact(projectRoot, artifactPath)
	if err != nil {
		t.Fatalf("ProjectHarnessResultForArtifact returned error: %v", err)
	}

	if slices.Contains(result.Evidence, runStatusFileName) {
		t.Fatalf("expected synthesized status evidence to omit run_status.yaml, got %#v", result.Evidence)
	}
	if _, ok := result.SourceArtifacts["run_status"]; ok {
		t.Fatalf("expected synthesized status sources to omit run_status, got %#v", result.SourceArtifacts)
	}
	assertResultEvidence(t, result, "state.json", "state")
}

func TestProjectHarnessResultNonTerminalDoesNotBackfillStaleSupervisorArtifacts(t *testing.T) {
	projectRoot, requestPath := prepareSmokeProject(t)

	runner, err := NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}
	artifactPath, err := runner.Run(requestPath, "go-result-stale-supervisor-artifacts")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	for _, name := range []string{"evaluation_result.yaml", "execution_report.yaml", "supervisor_trace.md"} {
		if err := os.WriteFile(filepath.Join(artifactPath, name), []byte("stale\n"), 0o644); err != nil {
			t.Fatalf("failed to seed stale %s: %v", name, err)
		}
	}
	if err := writeRunStatus(artifactPath, RunStatus{
		Status:       "in_progress",
		Phase:        "actor_execution",
		CurrentActor: "planner",
		Evidence:     []string{"state.json"},
		NextStep:     "Continue execution.",
	}); err != nil {
		t.Fatalf("failed to seed non-terminal run status: %v", err)
	}

	result, err := ProjectHarnessResultForArtifact(projectRoot, artifactPath)
	if err != nil {
		t.Fatalf("ProjectHarnessResultForArtifact returned error: %v", err)
	}

	for _, name := range []string{"evaluation_result.yaml", "execution_report.yaml", "supervisor_trace.md"} {
		if slices.Contains(result.Evidence, name) {
			t.Fatalf("expected non-terminal evidence to omit stale %s, got %#v", name, result.Evidence)
		}
	}
	for _, key := range []string{"evaluation_result", "execution_report", "supervisor_trace"} {
		if _, ok := result.SourceArtifacts[key]; ok {
			t.Fatalf("expected non-terminal sources to omit stale %s, got %#v", key, result.SourceArtifacts)
		}
	}
	assertResultEvidence(t, result, "state.json", "state")
}

func TestProjectHarnessResultInitializedFallbackNextStepUsesResolvedArtifactDirectory(t *testing.T) {
	projectRoot, requestPath := prepareSmokeProject(t)

	runner, err := NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}
	artifactPath, err := runner.Run(requestPath, "go-result-stale-artifact-dir")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if err := writeYAML(filepath.Join(artifactPath, runStatusFileName), RunStatus{
		Status:       "initialized",
		Phase:        "bootstrap",
		CurrentActor: "planner",
		ArtifactDir:  "/stale/artifact",
		Evidence:     []string{"state.json"},
	}); err != nil {
		t.Fatalf("failed to seed stale-artifact run status: %v", err)
	}

	result, err := ProjectHarnessResultForArtifact(projectRoot, artifactPath)
	if err != nil {
		t.Fatalf("ProjectHarnessResultForArtifact returned error: %v", err)
	}

	if !strings.Contains(result.RecommendedNextStep, artifactPath) {
		t.Fatalf("expected next step to use resolved artifact path %q, got %q", artifactPath, result.RecommendedNextStep)
	}
	if strings.Contains(result.RecommendedNextStep, "/stale/artifact") {
		t.Fatalf("expected next step not to use stale artifact path, got %q", result.RecommendedNextStep)
	}
}

func TestProjectLatestHarnessResultSelectsNewestUpdatedAt(t *testing.T) {
	projectRoot := t.TempDir()
	oldArtifact := writeLatestResultArtifact(t, projectRoot, "old", RunStatus{
		Status:       "initialized",
		Phase:        "bootstrap",
		CurrentActor: "planner",
		UpdatedAt:    "2026-04-27T01:00:00Z",
	})
	newArtifact := writeLatestResultArtifact(t, projectRoot, "new", RunStatus{
		Status:       "in_progress",
		Phase:        "actor_execution",
		CurrentActor: "executor",
		UpdatedAt:    "2026-04-27T02:00:00Z",
	})

	result, err := ProjectLatestHarnessResult(projectRoot)
	if err != nil {
		t.Fatalf("ProjectLatestHarnessResult returned error: %v", err)
	}

	if result.ArtifactDir != newArtifact {
		t.Fatalf("expected newest artifact %q, got %q (old artifact %q)", newArtifact, result.ArtifactDir, oldArtifact)
	}
	if result.Status != "in_progress" {
		t.Fatalf("expected newest status in_progress, got %q", result.Status)
	}
	if result.CurrentActor != "executor" {
		t.Fatalf("expected newest current actor executor, got %q", result.CurrentActor)
	}
	if result.UpdatedAt != "2026-04-27T02:00:00Z" {
		t.Fatalf("expected newest updated_at, got %q", result.UpdatedAt)
	}
}

func TestProjectLatestHarnessResultIgnoresInvalidCandidates(t *testing.T) {
	projectRoot := t.TempDir()
	validArtifact := writeLatestResultArtifact(t, projectRoot, "valid", RunStatus{
		Status:       "initialized",
		Phase:        "bootstrap",
		CurrentActor: "planner",
		UpdatedAt:    "2026-04-27T03:00:00Z",
	})

	invalidArtifact := filepath.Join(projectRoot, ".harness", "artifacts", "invalid")
	if err := os.MkdirAll(invalidArtifact, 0o755); err != nil {
		t.Fatalf("failed to create invalid artifact: %v", err)
	}
	if err := os.WriteFile(filepath.Join(invalidArtifact, runStatusFileName), []byte("updated_at: [\n"), 0o644); err != nil {
		t.Fatalf("failed to write invalid run status: %v", err)
	}

	unreadableArtifact := filepath.Join(projectRoot, ".harness", "artifacts", "unreadable")
	if err := os.MkdirAll(filepath.Join(unreadableArtifact, runStatusFileName), 0o755); err != nil {
		t.Fatalf("failed to create unreadable run status candidate: %v", err)
	}

	badTimeArtifact := filepath.Join(projectRoot, ".harness", "artifacts", "bad-time")
	if err := os.MkdirAll(badTimeArtifact, 0o755); err != nil {
		t.Fatalf("failed to create bad-time artifact: %v", err)
	}
	if err := writeYAML(filepath.Join(badTimeArtifact, runStatusFileName), RunStatus{
		Status:    "passed",
		Phase:     "terminal",
		UpdatedAt: "not-a-timestamp",
	}); err != nil {
		t.Fatalf("failed to write bad-time run status: %v", err)
	}

	result, err := ProjectLatestHarnessResult(projectRoot)
	if err != nil {
		t.Fatalf("ProjectLatestHarnessResult returned error: %v", err)
	}
	if result.ArtifactDir != validArtifact {
		t.Fatalf("expected valid artifact %q, got %q", validArtifact, result.ArtifactDir)
	}
}

func TestProjectLatestHarnessResultSkipsEscapedSymlinkCandidate(t *testing.T) {
	projectRoot := t.TempDir()
	validArtifact := writeLatestResultArtifact(t, projectRoot, "valid", RunStatus{
		Status:       "initialized",
		Phase:        "bootstrap",
		CurrentActor: "planner",
		UpdatedAt:    "2026-04-27T03:00:00Z",
	})

	outsideArtifact := t.TempDir()
	if err := writeYAML(filepath.Join(outsideArtifact, runStatusFileName), RunStatus{
		Status:       "in_progress",
		Phase:        "actor_execution",
		CurrentActor: "escaped",
		UpdatedAt:    "2026-04-27T04:00:00Z",
	}); err != nil {
		t.Fatalf("failed to write escaped run status: %v", err)
	}
	escapedLink := filepath.Join(projectRoot, ".harness", "artifacts", "escaped")
	if err := os.Symlink(outsideArtifact, escapedLink); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	result, err := ProjectLatestHarnessResult(projectRoot)
	if err != nil {
		t.Fatalf("ProjectLatestHarnessResult returned error: %v", err)
	}
	if result.ArtifactDir != validArtifact {
		t.Fatalf("expected escaped symlink to be skipped and valid artifact %q selected, got %q", validArtifact, result.ArtifactDir)
	}
	if result.CurrentActor != "planner" {
		t.Fatalf("expected in-root current actor planner, got %q", result.CurrentActor)
	}
}

func TestProjectLatestHarnessResultTieBreaksEqualUpdatedAtByNewestRunStatusMTime(t *testing.T) {
	projectRoot := t.TempDir()
	olderArtifact := writeLatestResultArtifact(t, projectRoot, "aaa", RunStatus{
		Status:       "initialized",
		Phase:        "bootstrap",
		CurrentActor: "planner",
		UpdatedAt:    "2026-04-27T05:00:00Z",
	})
	newerArtifact := writeLatestResultArtifact(t, projectRoot, "zzz", RunStatus{
		Status:       "in_progress",
		Phase:        "actor_execution",
		CurrentActor: "executor",
		UpdatedAt:    "2026-04-27T05:00:00Z",
	})
	setLatestResultRunStatusMTime(t, olderArtifact, time.Date(2026, 4, 27, 5, 0, 0, 0, time.UTC))
	setLatestResultRunStatusMTime(t, newerArtifact, time.Date(2026, 4, 27, 5, 1, 0, 0, time.UTC))

	result, err := ProjectLatestHarnessResult(projectRoot)
	if err != nil {
		t.Fatalf("ProjectLatestHarnessResult returned error: %v", err)
	}
	if result.ArtifactDir != newerArtifact {
		t.Fatalf("expected newer run_status mtime artifact %q, got %q", newerArtifact, result.ArtifactDir)
	}
}

func TestProjectLatestHarnessResultTieBreaksEqualUpdatedAtAndMTimeByLexicalArtifactPath(t *testing.T) {
	projectRoot := t.TempDir()
	lowerArtifact := writeLatestResultArtifact(t, projectRoot, "aaa", RunStatus{
		Status:       "initialized",
		Phase:        "bootstrap",
		CurrentActor: "planner",
		UpdatedAt:    "2026-04-27T06:00:00Z",
	})
	higherArtifact := writeLatestResultArtifact(t, projectRoot, "zzz", RunStatus{
		Status:       "in_progress",
		Phase:        "actor_execution",
		CurrentActor: "executor",
		UpdatedAt:    "2026-04-27T06:00:00Z",
	})
	tiedMTime := time.Date(2026, 4, 27, 6, 0, 0, 0, time.UTC)
	setLatestResultRunStatusMTime(t, lowerArtifact, tiedMTime)
	setLatestResultRunStatusMTime(t, higherArtifact, tiedMTime)

	result, err := ProjectLatestHarnessResult(projectRoot)
	if err != nil {
		t.Fatalf("ProjectLatestHarnessResult returned error: %v", err)
	}
	if result.ArtifactDir != higherArtifact {
		t.Fatalf("expected lexicographically greater artifact %q, got %q (lower %q)", higherArtifact, result.ArtifactDir, lowerArtifact)
	}
}

func TestProjectLatestHarnessResultErrorsWhenArtifactsDirectoryIsAbsent(t *testing.T) {
	projectRoot := t.TempDir()

	_, err := ProjectLatestHarnessResult(projectRoot)
	if err == nil {
		t.Fatalf("expected no-valid-candidates error")
	}
	if !strings.Contains(err.Error(), "no valid run_status.yaml candidates") {
		t.Fatalf("expected clear no-valid-candidates error, got %v", err)
	}
}

func TestProjectLatestHarnessResultErrorsWhenNoValidRunStatusCandidatesExist(t *testing.T) {
	projectRoot := t.TempDir()
	invalidArtifact := filepath.Join(projectRoot, ".harness", "artifacts", "invalid")
	if err := os.MkdirAll(invalidArtifact, 0o755); err != nil {
		t.Fatalf("failed to create invalid artifact: %v", err)
	}
	if err := os.WriteFile(filepath.Join(invalidArtifact, runStatusFileName), []byte("updated_at: [\n"), 0o644); err != nil {
		t.Fatalf("failed to write invalid run status: %v", err)
	}

	_, err := ProjectLatestHarnessResult(projectRoot)
	if err == nil {
		t.Fatalf("expected no-valid-candidates error")
	}
	if !strings.Contains(err.Error(), "no valid run_status.yaml candidates") {
		t.Fatalf("expected clear no-valid-candidates error, got %v", err)
	}
}

func assertResultEvidence(t *testing.T, result HarnessResult, evidenceName string, sourceKey string) {
	t.Helper()

	if !slices.Contains(result.Evidence, evidenceName) {
		t.Fatalf("expected evidence to contain %q, got %#v", evidenceName, result.Evidence)
	}
	if result.SourceArtifacts[sourceKey] != evidenceName {
		t.Fatalf("expected source artifact %q to be %q, got %#v", sourceKey, evidenceName, result.SourceArtifacts)
	}
}

func writeLatestResultArtifact(t *testing.T, projectRoot string, artifactID string, status RunStatus) string {
	t.Helper()

	artifactPath := filepath.Join(projectRoot, ".harness", "artifacts", artifactID)
	if err := os.MkdirAll(artifactPath, 0o755); err != nil {
		t.Fatalf("failed to create artifact %q: %v", artifactID, err)
	}
	if status.Evidence == nil {
		status.Evidence = []string{runStatusFileName}
	}
	if status.NextStep == "" {
		status.NextStep = "Continue the harness workflow."
	}
	if err := writeYAML(filepath.Join(artifactPath, runStatusFileName), status); err != nil {
		t.Fatalf("failed to write run status for %q: %v", artifactID, err)
	}
	resolvedArtifactPath, err := filepath.EvalSymlinks(artifactPath)
	if err != nil {
		t.Fatalf("failed to resolve artifact %q: %v", artifactID, err)
	}
	return resolvedArtifactPath
}

func setLatestResultRunStatusMTime(t *testing.T, artifactPath string, mtime time.Time) {
	t.Helper()

	runStatusPath := filepath.Join(artifactPath, runStatusFileName)
	if err := os.Chtimes(runStatusPath, mtime, mtime); err != nil {
		t.Fatalf("failed to set run status mtime for %q: %v", artifactPath, err)
	}
}
