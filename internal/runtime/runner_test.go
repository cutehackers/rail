package runtime

import (
	"io"
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
	runStatus, err := ReadRunStatus(artifactPath)
	if err != nil {
		t.Fatalf("expected run_status.yaml to be readable: %v", err)
	}
	if runStatus.Status != "initialized" {
		t.Fatalf("unexpected run status: got %q want initialized", runStatus.Status)
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

	criticReportPath := filepath.Join(artifactPath, "critic_report.yaml")
	criticReportData, err := os.ReadFile(criticReportPath)
	if err != nil {
		t.Fatalf("expected critic_report.yaml to exist after smoke execution: %v", err)
	}
	var criticReport map[string]any
	if err := yaml.Unmarshal(criticReportData, &criticReport); err != nil {
		t.Fatalf("failed to decode smoke critic_report.yaml: %v", err)
	}
	if got, want := criticReport["priority_focus"].([]any), []any{"Keep the smoke-path execution bounded, deterministic, and reviewable."}; !slices.Equal(got, want) {
		t.Fatalf("unexpected smoke critic priority_focus: got %#v want %#v", got, want)
	}
	if got, want := criticReport["generator_guardrails"].([]any), []any{"Do not edit files outside the scoped target."}; !slices.Equal(got, want) {
		t.Fatalf("unexpected smoke critic generator_guardrails: got %#v want %#v", got, want)
	}

	trace, err := os.ReadFile(filepath.Join(artifactPath, "supervisor_trace.md"))
	if err != nil {
		t.Fatalf("expected supervisor_trace.md to exist: %v", err)
	}
	for _, fragment := range []string{
		"# Supervisor Decision Trace",
		"## Iteration 1",
		"critic",
		"- decision: `pass`",
		"- selected_action: `pass`",
		"- terminal_status: `passed`",
	} {
		if !strings.Contains(string(trace), fragment) {
			t.Fatalf("expected supervisor trace to contain %q, got:\n%s", fragment, string(trace))
		}
	}
}

func TestExecuteFailsBeforeGeneratorWhenCriticReportMissing(t *testing.T) {
	projectRoot, requestPath := prepareSmokeProject(t)

	runner, err := NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}

	artifactPath, err := runner.Run(requestPath, "go-smoke")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	statePath := filepath.Join(artifactPath, "state.json")
	state, err := readState(statePath)
	if err != nil {
		t.Fatalf("failed to read state: %v", err)
	}
	state.CurrentActor = stringPtr("generator")
	state.CompletedActors = []string{"planner", "context_builder", "critic"}
	if err := writeJSON(statePath, state); err != nil {
		t.Fatalf("failed to persist generator state: %v", err)
	}

	criticReportPath := filepath.Join(artifactPath, "critic_report.yaml")
	if err := os.Remove(criticReportPath); err != nil {
		t.Fatalf("failed to remove critic_report.yaml: %v", err)
	}

	_, err = runner.Execute(artifactPath)
	if err == nil {
		t.Fatalf("expected Execute to fail before generator when critic_report is missing")
	}
	if !strings.Contains(err.Error(), "critic_report") {
		t.Fatalf("expected missing critic_report error, got %v", err)
	}
}

func TestRouteEvaluationSkippedNonTerminalDoesNotRewriteRunStatus(t *testing.T) {
	projectRoot, requestPath := prepareSmokeProject(t)

	runner, err := NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}

	artifactPath, err := runner.Run(requestPath, "go-skipped-route-status")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	summary, err := runner.router.RouteEvaluation(artifactPath)
	if err != nil {
		t.Fatalf("RouteEvaluation returned error for skipped non-terminal artifact: %v", err)
	}
	if !strings.Contains(summary, "routing skipped") {
		t.Fatalf("expected skipped routing summary, got %q", summary)
	}

	runStatus, err := ReadRunStatus(artifactPath)
	if err != nil {
		t.Fatalf("expected run_status.yaml to be readable: %v", err)
	}
	if runStatus.Status != "initialized" || runStatus.Phase != "bootstrap" || runStatus.CurrentActor != "planner" {
		t.Fatalf("expected skipped route-evaluation to preserve bootstrap run status, got %#v", runStatus)
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
	if err := writeRunStatus(artifactPath, RunStatus{
		Status:           "interrupted",
		Phase:            "actor_execution",
		CurrentActor:     "evaluator",
		InterruptionKind: "actor_failed",
		Message:          "stale interruption from a prior attempt",
	}); err != nil {
		t.Fatalf("failed to seed stale run status: %v", err)
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
	runStatus, err := ReadRunStatus(artifactPath)
	if err != nil {
		t.Fatalf("expected run_status.yaml to remain readable after refresh: %v", err)
	}
	if runStatus.Status != "passed" || runStatus.Phase != "terminal" || runStatus.InterruptionKind != "" {
		t.Fatalf("expected refresh to replace stale interrupted run status, got %#v", runStatus)
	}
	if !slices.Contains(runStatus.Evidence, "terminal_summary.md") {
		t.Fatalf("expected refreshed run status to include terminal_summary.md evidence, got %#v", runStatus.Evidence)
	}
}

func TestExecuteRefreshesMissingExecutionReportForCompletedArtifacts(t *testing.T) {
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

	executionReportPath := filepath.Join(artifactPath, "execution_report.yaml")
	if err := os.Remove(executionReportPath); err != nil {
		t.Fatalf("failed to remove execution_report.yaml: %v", err)
	}

	summary, err := runner.Execute(artifactPath)
	if err != nil {
		t.Fatalf("refresh Execute returned error: %v", err)
	}
	if strings.Contains(summary, "already completed") {
		t.Fatalf("expected Execute to refresh missing execution_report instead of returning early, got %q", summary)
	}

	executionReport, err := os.ReadFile(executionReportPath)
	if err != nil {
		t.Fatalf("expected execution_report.yaml to be recreated: %v", err)
	}
	for _, fragment := range []string{
		"actor_graph:",
		"actor_profiles_used:",
		"critic_findings_applied:",
		"critic_to_evaluator_delta:",
		"terminal_status: passed",
	} {
		if !strings.Contains(string(executionReport), fragment) {
			t.Fatalf("expected recovered execution report to contain %q, got:\n%s", fragment, string(executionReport))
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
	t.Setenv("RAIL_ACTOR_MODEL", "wrong-model")
	t.Setenv("RAIL_ACTOR_REASONING_EFFORT", "low")
	profilesPath := filepath.Join(projectRoot, ".harness", "supervisor", "actor_profiles.yaml")
	distinctProfiles := `version: 1
actors:
  planner: { model: gpt-5.4-planner, reasoning: high }
  context_builder: { model: gpt-5.4-mini-context, reasoning: medium }
  critic: { model: gpt-5.4-critic, reasoning: xhigh }
  generator: { model: gpt-5.4-generator, reasoning: low }
  evaluator: { model: gpt-5.4-evaluator, reasoning: none }
  integrator: { model: gpt-5.4-integrator, reasoning: minimal }
`
	if err := os.WriteFile(profilesPath, []byte(distinctProfiles), 0o644); err != nil {
		t.Fatalf("failed to write distinct actor profiles: %v", err)
	}

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
	if got, want := strings.TrimSpace(string(actorLog)), strings.Join([]string{
		"planner|gpt-5.4-planner|high|workspace-write|json=true",
		"context_builder|gpt-5.4-mini-context|medium|workspace-write|json=true",
		"critic|gpt-5.4-critic|xhigh|workspace-write|json=true",
		"generator|gpt-5.4-generator|low|workspace-write|json=true",
		"evaluator|gpt-5.4-evaluator|none|workspace-write|json=true",
	}, "\n"); got != want {
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
	if got := len(state.ActorProfilesUsed); got != 5 {
		t.Fatalf("expected persisted actorProfilesUsed snapshot, got %#v", state.ActorProfilesUsed)
	}

	criticReport, err := os.ReadFile(filepath.Join(artifactPath, "critic_report.yaml"))
	if err != nil {
		t.Fatalf("failed to read real-mode critic_report.yaml: %v", err)
	}
	for _, fragment := range []string{
		"priority_focus:",
		"generator_guardrails:",
		"Keep the change scoped to feature/ready.go.",
	} {
		if !strings.Contains(string(criticReport), fragment) {
			t.Fatalf("expected real-mode critic_report.yaml to contain %q, got:\n%s", fragment, string(criticReport))
		}
	}
}

func TestExecuteRejectsUnsafeBackendPolicyBeforeCodex(t *testing.T) {
	projectRoot, requestPath := prepareRealProject(t)
	actorLogPath := installFakeCodexForRealMode(t, projectRoot)
	unsafePolicy := `version: 1
execution_environment: local
default_backend: codex_cli

backends:
  codex_cli:
    command: codex
    subcommand: exec
    sandbox: danger-full-access
    approval_policy: never
    session_mode: per_actor
    ephemeral: true
    capture_json_events: true
    skip_git_repo_check: true

execution_environments:
  local:
    allowed_sandboxes:
      - workspace-write
`
	if err := os.WriteFile(filepath.Join(projectRoot, ".harness", "supervisor", "actor_backend.yaml"), []byte(unsafePolicy), 0o644); err != nil {
		t.Fatalf("failed to write unsafe actor backend policy: %v", err)
	}

	runner, err := NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}

	artifactPath, err := runner.Run(requestPath, "go-real-unsafe-backend")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	_, err = runner.Execute(artifactPath)
	if err == nil {
		t.Fatalf("expected Execute to reject unsafe actor backend policy")
	}
	if !strings.Contains(err.Error(), "sandbox danger-full-access is not allowed") {
		t.Fatalf("expected unsafe sandbox error, got %v", err)
	}
	if actorLog, readErr := os.ReadFile(actorLogPath); readErr == nil && strings.TrimSpace(string(actorLog)) != "" {
		t.Fatalf("expected unsafe backend policy to fail before invoking fake codex, got actor log:\n%s", string(actorLog))
	} else if readErr != nil && !os.IsNotExist(readErr) {
		t.Fatalf("failed to inspect fake codex actor log: %v", readErr)
	}
}

func TestExecuteRoutesAuditViolationToTerminalSummary(t *testing.T) {
	projectRoot, requestPath := prepareRealProject(t)
	installFakeCodexForRealMode(t, projectRoot)
	t.Setenv("RAIL_TEST_CODEX_VIOLATION_ACTOR", "evaluator")

	runner, err := NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}

	artifactPath, err := runner.Run(requestPath, "go-real-audit-violation")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	summary, err := runner.Execute(artifactPath)
	if err != nil {
		t.Fatalf("expected Execute to route audit violation to terminal summary, got error: %v", err)
	}
	if !strings.Contains(summary, "status=blocked_environment") {
		t.Fatalf("expected blocked_environment summary, got %q", summary)
	}

	terminalSummary, err := os.ReadFile(filepath.Join(artifactPath, "terminal_summary.md"))
	if err != nil {
		t.Fatalf("expected terminal_summary.md to exist: %v", err)
	}
	for _, fragment := range []string{
		"## Reporting Limits",
		"backend_policy_violation",
		"Final answer must not claim successful implementation",
	} {
		if !strings.Contains(string(terminalSummary), fragment) {
			t.Fatalf("expected terminal summary to contain %q, got:\n%s", fragment, string(terminalSummary))
		}
	}
}

func TestExecuteRecordsRunStatusForActorFailure(t *testing.T) {
	projectRoot, requestPath := prepareRealProject(t)
	installFakeCodexForRealMode(t, projectRoot)
	t.Setenv("RAIL_TEST_CODEX_FAIL_ACTOR", "planner")

	runner, err := NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}

	artifactPath, err := runner.Run(requestPath, "go-real-actor-failure")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	_, err = runner.Execute(artifactPath)
	if err == nil {
		t.Fatalf("expected Execute to fail when actor process fails")
	}

	runStatus, statusErr := ReadRunStatus(artifactPath)
	if statusErr != nil {
		t.Fatalf("expected run_status.yaml to be readable after interruption: %v", statusErr)
	}
	if runStatus.Status != "interrupted" {
		t.Fatalf("unexpected run status: got %q want interrupted", runStatus.Status)
	}
	if runStatus.Phase != "actor_execution" {
		t.Fatalf("unexpected phase: got %q want actor_execution", runStatus.Phase)
	}
	if runStatus.CurrentActor != "planner" {
		t.Fatalf("unexpected current actor: got %q want planner", runStatus.CurrentActor)
	}
	if runStatus.InterruptionKind != "actor_failed" {
		t.Fatalf("unexpected interruption kind: got %q want actor_failed", runStatus.InterruptionKind)
	}
	if !strings.Contains(runStatus.Message, "intentional fake codex failure") {
		t.Fatalf("expected run status message to include actor failure, got %q", runStatus.Message)
	}

	summary := FormatRunStatusSummary(runStatus)
	for _, fragment := range []string{"status: interrupted", "phase: actor_execution", "current actor: planner"} {
		if !strings.Contains(summary, fragment) {
			t.Fatalf("expected summary to contain %q, got:\n%s", fragment, summary)
		}
	}
}

func TestSuperviseRetriesTransientActorFailureToTerminal(t *testing.T) {
	projectRoot, requestPath := prepareRealProject(t)
	actorLogPath := installFakeCodexForRealMode(t, projectRoot)
	t.Setenv("RAIL_TEST_CODEX_FAIL_ONCE_ACTOR", "planner")

	runner, err := NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}

	artifactPath, err := runner.Run(requestPath, "go-real-supervise-retry")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	summary, err := runner.Supervise(artifactPath, SuperviseOptions{RetryBudget: 1})
	if err != nil {
		t.Fatalf("Supervise returned error for transient actor failure: %v", err)
	}
	if !strings.Contains(summary, "status=passed") || !strings.Contains(summary, "supervised") {
		t.Fatalf("expected supervised passing summary, got %q", summary)
	}

	runStatus, err := ReadRunStatus(artifactPath)
	if err != nil {
		t.Fatalf("expected run_status.yaml to be readable: %v", err)
	}
	if runStatus.Status != "passed" || runStatus.Phase != "terminal" {
		t.Fatalf("expected supervised run to finish terminal passed, got %#v", runStatus)
	}

	actorLog, err := os.ReadFile(actorLogPath)
	if err != nil {
		t.Fatalf("failed to read fake codex actor log: %v", err)
	}
	if got := strings.Count(string(actorLog), "planner|"); got != 2 {
		t.Fatalf("expected planner to run twice after one supervised retry, got %d log:\n%s", got, string(actorLog))
	}
}

func TestSuperviseDoesNotRetryNonRetryableStateErrors(t *testing.T) {
	projectRoot, requestPath := prepareSmokeProject(t)

	runner, err := NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}

	artifactPath, err := runner.Run(requestPath, "go-supervise-non-retryable")
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

	_, err = runner.Supervise(artifactPath, SuperviseOptions{RetryBudget: 2})
	if err == nil {
		t.Fatalf("expected Supervise to fail without retrying non-retryable state error")
	}

	runStatus, statusErr := ReadRunStatus(artifactPath)
	if statusErr != nil {
		t.Fatalf("expected run_status.yaml to be readable: %v", statusErr)
	}
	if runStatus.Status != "interrupted" || runStatus.InterruptionKind != "execution_error" || runStatus.Phase != "actor_resolution" {
		t.Fatalf("expected actor resolution interruption without retry, got %#v", runStatus)
	}
}

func TestExecuteUsesWorkflowProjectRootActorProfiles(t *testing.T) {
	targetRoot, requestPath := prepareRealProject(t)
	actorLogPath := installFakeCodexForRealMode(t, targetRoot)

	targetProfiles := `version: 1
actors:
  planner: { model: gpt-5.4-target-planner, reasoning: high }
  context_builder: { model: gpt-5.4-target-context, reasoning: medium }
  critic: { model: gpt-5.4-target-critic, reasoning: xhigh }
  generator: { model: gpt-5.4-target-generator, reasoning: low }
  evaluator: { model: gpt-5.4-target-evaluator, reasoning: none }
  integrator: { model: gpt-5.4-target-integrator, reasoning: minimal }
`
	if err := os.WriteFile(filepath.Join(targetRoot, ".harness", "supervisor", "actor_profiles.yaml"), []byte(targetProfiles), 0o644); err != nil {
		t.Fatalf("failed to write target actor profiles: %v", err)
	}

	targetRunner, err := NewRunner(targetRoot)
	if err != nil {
		t.Fatalf("NewRunner(targetRoot) returned error: %v", err)
	}
	artifactPath, err := targetRunner.Run(requestPath, "go-real-cross-root")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	controlRoot := t.TempDir()
	for _, relPath := range []string{
		filepath.Join(".harness", "supervisor"),
	} {
		if err := os.MkdirAll(filepath.Join(controlRoot, relPath), 0o755); err != nil {
			t.Fatalf("failed to create %q: %v", relPath, err)
		}
	}
	badProfiles := `version: 1
actors:
  planner: { model: wrong-control-planner, reasoning: critical }
`
	if err := os.WriteFile(filepath.Join(controlRoot, ".harness", "supervisor", "actor_profiles.yaml"), []byte(badProfiles), 0o644); err != nil {
		t.Fatalf("failed to write control-root actor profiles: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(controlRoot, ".harness", "artifacts"), 0o755); err != nil {
		t.Fatalf("failed to create control-root artifacts directory: %v", err)
	}
	controlArtifactPath := filepath.Join(controlRoot, ".harness", "artifacts", "go-real-cross-root")
	if err := copyDirectory(artifactPath, controlArtifactPath); err != nil {
		t.Fatalf("failed to copy artifact into control root: %v", err)
	}

	controlRunner, err := NewRunner(controlRoot)
	if err != nil {
		t.Fatalf("NewRunner(controlRoot) returned error: %v", err)
	}

	summary, err := controlRunner.Execute(filepath.ToSlash(filepath.Join(".harness", "artifacts", "go-real-cross-root")))
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
	if got, want := strings.TrimSpace(string(actorLog)), strings.Join([]string{
		"planner|gpt-5.4-target-planner|high|workspace-write|json=true",
		"context_builder|gpt-5.4-target-context|medium|workspace-write|json=true",
		"critic|gpt-5.4-target-critic|xhigh|workspace-write|json=true",
		"generator|gpt-5.4-target-generator|low|workspace-write|json=true",
		"evaluator|gpt-5.4-target-evaluator|none|workspace-write|json=true",
	}, "\n"); got != want {
		t.Fatalf("unexpected actor execution order: got %q want %q", got, want)
	}

	readySource, err := os.ReadFile(filepath.Join(targetRoot, "feature", "ready.go"))
	if err != nil {
		t.Fatalf("failed to read target ready.go: %v", err)
	}
	if !strings.Contains(string(readySource), "Real-mode actor path verified.") {
		t.Fatalf("expected generator actor to update target ready.go, got:\n%s", string(readySource))
	}
}

func TestExecutePersistsHistoricalActorProfilesForRouteEvaluation(t *testing.T) {
	projectRoot, requestPath := prepareRealProject(t)
	installFakeCodexForRealMode(t, projectRoot)

	originalProfiles := `version: 1
actors:
  planner: { model: gpt-5.4-history-planner, reasoning: high }
  context_builder: { model: gpt-5.4-history-context, reasoning: medium }
  critic: { model: gpt-5.4-history-critic, reasoning: xhigh }
  generator: { model: gpt-5.4-history-generator, reasoning: low }
  evaluator: { model: gpt-5.4-history-evaluator, reasoning: none }
  integrator: { model: gpt-5.4-history-integrator, reasoning: minimal }
`
	if err := os.WriteFile(filepath.Join(projectRoot, ".harness", "supervisor", "actor_profiles.yaml"), []byte(originalProfiles), 0o644); err != nil {
		t.Fatalf("failed to write original actor profiles: %v", err)
	}

	runner, err := NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}
	artifactPath, err := runner.Run(requestPath, "go-real-profile-history")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if _, err := runner.Execute(artifactPath); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	driftedProfiles := `version: 1
actors:
  planner: { model: drifted-planner, reasoning: low }
  context_builder: { model: drifted-context, reasoning: low }
  critic: { model: drifted-critic, reasoning: low }
  generator: { model: drifted-generator, reasoning: low }
  evaluator: { model: drifted-evaluator, reasoning: low }
  integrator: { model: drifted-integrator, reasoning: low }
`
	if err := os.WriteFile(filepath.Join(projectRoot, ".harness", "supervisor", "actor_profiles.yaml"), []byte(driftedProfiles), 0o644); err != nil {
		t.Fatalf("failed to write drifted actor profiles: %v", err)
	}
	if err := os.Remove(filepath.Join(artifactPath, "terminal_summary.md")); err != nil {
		t.Fatalf("failed to remove terminal_summary.md: %v", err)
	}

	router, err := NewRouter(projectRoot)
	if err != nil {
		t.Fatalf("NewRouter returned error: %v", err)
	}
	if _, err := router.RouteEvaluation(artifactPath); err != nil {
		t.Fatalf("RouteEvaluation returned error: %v", err)
	}

	executionReport, err := os.ReadFile(filepath.Join(artifactPath, "execution_report.yaml"))
	if err != nil {
		t.Fatalf("failed to read execution_report.yaml: %v", err)
	}
	if !strings.Contains(string(executionReport), "gpt-5.4-history-planner") {
		t.Fatalf("expected historical actor profile to remain in execution report, got:\n%s", string(executionReport))
	}
	if strings.Contains(string(executionReport), "drifted-planner") {
		t.Fatalf("expected route evaluation to avoid live-reloaded drifted profile, got:\n%s", string(executionReport))
	}
}

func TestExecuteUsesPersistedActorProfilesSnapshotAfterProfileDrift(t *testing.T) {
	projectRoot, requestPath := prepareRealProject(t)
	actorLogPath := installFakeCodexForRealMode(t, projectRoot)

	runner, err := NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}
	artifactPath, err := runner.Run(requestPath, "go-real-resume-profile-snapshot")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	statePath := filepath.Join(artifactPath, "state.json")
	state, err := readState(statePath)
	if err != nil {
		t.Fatalf("failed to read state: %v", err)
	}
	state.ActorProfilesUsed = []ActorProfileUsed{
		{Actor: "planner", Model: "snapshot-planner", Reasoning: "high"},
		{Actor: "context_builder", Model: "snapshot-context", Reasoning: "medium"},
		{Actor: "critic", Model: "snapshot-critic", Reasoning: "xhigh"},
		{Actor: "generator", Model: "snapshot-generator", Reasoning: "low"},
		{Actor: "evaluator", Model: "snapshot-evaluator", Reasoning: "none"},
	}
	if err := writeJSON(statePath, state); err != nil {
		t.Fatalf("failed to rewrite state.json: %v", err)
	}

	driftedProfiles := `version: 1
actors:
  planner: { model: drifted-planner, reasoning: low }
  context_builder: { model: drifted-context, reasoning: low }
  critic: { model: drifted-critic, reasoning: low }
  generator: { model: drifted-generator, reasoning: low }
  evaluator: { model: drifted-evaluator, reasoning: low }
  integrator: { model: drifted-integrator, reasoning: low }
`
	if err := os.WriteFile(filepath.Join(projectRoot, ".harness", "supervisor", "actor_profiles.yaml"), []byte(driftedProfiles), 0o644); err != nil {
		t.Fatalf("failed to write drifted actor profiles: %v", err)
	}

	if _, err := runner.Execute(artifactPath); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	actorLog, err := os.ReadFile(actorLogPath)
	if err != nil {
		t.Fatalf("failed to read fake codex actor log: %v", err)
	}
	if strings.Contains(string(actorLog), "drifted-planner") || strings.Contains(string(actorLog), "drifted-generator") {
		t.Fatalf("expected resumed execution to avoid live drifted profiles, got:\n%s", string(actorLog))
	}
	for _, fragment := range []string{
		"planner|snapshot-planner|high",
		"context_builder|snapshot-context|medium",
		"critic|snapshot-critic|xhigh",
		"generator|snapshot-generator|low",
		"evaluator|snapshot-evaluator|none",
	} {
		if !strings.Contains(string(actorLog), fragment) {
			t.Fatalf("expected actor log to contain %q, got:\n%s", fragment, string(actorLog))
		}
	}
}

func TestExecuteFailsWhenPersistedActorProfilesSnapshotIsIncomplete(t *testing.T) {
	projectRoot, requestPath := prepareRealProject(t)
	installFakeCodexForRealMode(t, projectRoot)

	runner, err := NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}
	artifactPath, err := runner.Run(requestPath, "go-real-incomplete-profile-snapshot")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	statePath := filepath.Join(artifactPath, "state.json")
	state, err := readState(statePath)
	if err != nil {
		t.Fatalf("failed to read state: %v", err)
	}
	state.ActorProfilesUsed = []ActorProfileUsed{
		{Actor: "planner", Model: "gpt-5.4-planner", Reasoning: "high"},
	}
	if err := writeJSON(statePath, state); err != nil {
		t.Fatalf("failed to rewrite state.json: %v", err)
	}

	_, err = runner.Execute(artifactPath)
	if err == nil {
		t.Fatalf("expected Execute to fail for incomplete actorProfilesUsed snapshot")
	}
	if !strings.Contains(err.Error(), "actorProfilesUsed") {
		t.Fatalf("expected actorProfilesUsed error, got %v", err)
	}
}

func TestExecuteFailsWhenPersistedActorProfilesSnapshotHasUnsupportedReasoning(t *testing.T) {
	projectRoot, requestPath := prepareRealProject(t)
	installFakeCodexForRealMode(t, projectRoot)

	runner, err := NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}
	artifactPath, err := runner.Run(requestPath, "go-real-invalid-profile-snapshot")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	statePath := filepath.Join(artifactPath, "state.json")
	state, err := readState(statePath)
	if err != nil {
		t.Fatalf("failed to read state: %v", err)
	}
	state.ActorProfilesUsed = []ActorProfileUsed{
		{Actor: "planner", Model: "gpt-5.4-planner", Reasoning: "high"},
		{Actor: "context_builder", Model: "gpt-5.4-context", Reasoning: "medium"},
		{Actor: "critic", Model: "gpt-5.4-critic", Reasoning: "critical"},
		{Actor: "generator", Model: "gpt-5.4-generator", Reasoning: "low"},
		{Actor: "evaluator", Model: "gpt-5.4-evaluator", Reasoning: "none"},
	}
	if err := writeJSON(statePath, state); err != nil {
		t.Fatalf("failed to rewrite state.json: %v", err)
	}

	_, err = runner.Execute(artifactPath)
	if err == nil {
		t.Fatalf("expected Execute to fail for unsupported actorProfilesUsed reasoning")
	}
	if !strings.Contains(err.Error(), "unsupported reasoning") {
		t.Fatalf("expected unsupported reasoning error, got %v", err)
	}
}

func TestExecuteFailsForInvalidActorProfiles(t *testing.T) {
	t.Run("missing required actor profile", func(t *testing.T) {
		projectRoot, requestPath := prepareRealProject(t)
		profilesPath := filepath.Join(projectRoot, ".harness", "supervisor", "actor_profiles.yaml")
		missingActorProfiles := `version: 1
actors:
  planner: { model: gpt-5.4, reasoning: high }
  context_builder: { model: gpt-5.4-mini, reasoning: high }
  generator: { model: gpt-5.4, reasoning: high }
  evaluator: { model: gpt-5.4, reasoning: high }
`
		if err := os.WriteFile(profilesPath, []byte(missingActorProfiles), 0o644); err != nil {
			t.Fatalf("failed to write missing-actor profiles: %v", err)
		}

		runner, err := NewRunner(projectRoot)
		if err != nil {
			t.Fatalf("NewRunner returned error: %v", err)
		}

		artifactPath, err := runner.Run(requestPath, "go-real-missing-profiles")
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}

		_, err = runner.Execute(artifactPath)
		if err == nil {
			t.Fatalf("expected Execute to fail for missing required actor profile")
		}
		if !strings.Contains(err.Error(), "missing required actors") {
			t.Fatalf("expected missing required actor error, got %v", err)
		}
	})

	t.Run("invalid actor profiles file", func(t *testing.T) {
		projectRoot, requestPath := prepareRealProject(t)
		profilesPath := filepath.Join(projectRoot, ".harness", "supervisor", "actor_profiles.yaml")
		invalidProfiles := `version: 1
actors:
  planner: { model: gpt-5.4, reasoning: high }
  context_builder: { model: gpt-5.4-mini, reasoning: high }
  critic: { model: gpt-5.4, reasoning: critical }
  generator: { model: gpt-5.4, reasoning: high }
  evaluator: { model: gpt-5.4, reasoning: high }
`
		if err := os.WriteFile(profilesPath, []byte(invalidProfiles), 0o644); err != nil {
			t.Fatalf("failed to write invalid actor profiles: %v", err)
		}

		runner, err := NewRunner(projectRoot)
		if err != nil {
			t.Fatalf("NewRunner returned error: %v", err)
		}

		artifactPath, err := runner.Run(requestPath, "go-real-invalid-profiles")
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}

		_, err = runner.Execute(artifactPath)
		if err == nil {
			t.Fatalf("expected Execute to fail for invalid actor profiles")
		}
		if !strings.Contains(err.Error(), "unsupported reasoning") {
			t.Fatalf("expected invalid actor profiles error, got %v", err)
		}
	})
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

	requestBody, err := os.ReadFile(filepath.Join(testRepoRoot(t), "examples", "smoke-target", ".harness", "requests", "valid_request.yaml"))
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
		filepath.Join(".harness", "supervisor"),
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
	actorProfilesBody, err := os.ReadFile(filepath.Join(testRepoRoot(t), ".harness", "supervisor", "actor_profiles.yaml"))
	if err != nil {
		t.Fatalf("failed to read checked-in actor profiles: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".harness", "supervisor", "actor_profiles.yaml"), actorProfilesBody, 0o644); err != nil {
		t.Fatalf("failed to write checked-in actor profiles: %v", err)
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

model = ""
reasoning = ""
sandbox = ""
has_json = "--json" in sys.argv
for index, value in enumerate(sys.argv):
    if value == "-m" and index + 1 < len(sys.argv):
        model = sys.argv[index + 1]
    if value == "-s" and index + 1 < len(sys.argv):
        sandbox = sys.argv[index + 1]
    if value == "-c" and index + 1 < len(sys.argv):
        config_value = sys.argv[index + 1]
        config_match = re.match(r'model_reasoning_effort="([^"]+)"', config_value)
        if config_match:
            reasoning = config_match.group(1)

if project_root:
    with open(os.path.join(project_root, ".actor-log"), "a", encoding="utf-8") as handle:
        handle.write(actor + "|" + model + "|" + reasoning + "|" + sandbox + "|json=" + str(has_json).lower() + "\n")

fail_actor = os.environ.get("RAIL_TEST_CODEX_FAIL_ACTOR", "")
if actor == fail_actor:
    print("intentional fake codex failure for " + actor, file=sys.stderr)
    raise SystemExit(42)

fail_once_actor = os.environ.get("RAIL_TEST_CODEX_FAIL_ONCE_ACTOR", "")
if actor == fail_once_actor and project_root:
    marker_path = os.path.join(project_root, ".actor-fail-once-" + actor)
    if not os.path.exists(marker_path):
        with open(marker_path, "w", encoding="utf-8") as marker:
            marker.write("failed\n")
        print("intentional one-time fake codex failure for " + actor, file=sys.stderr)
        raise SystemExit(43)

violation_actor = os.environ.get("RAIL_TEST_CODEX_VIOLATION_ACTOR", "")
if actor == violation_actor:
    print(json.dumps({"type": "item.started", "item": {"type": "command_execution", "command": "sed -n '1,40p' /tmp/.codex/superpowers/skills/using-superpowers/SKILL.md"}}))
else:
    print(json.dumps({"type": "thread.started", "thread_id": actor}))

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
elif actor == "critic":
    response = {
        "priority_focus": ["Keep the change scoped to feature/ready.go."],
        "missing_requirements": [],
        "risk_hypotheses": ["Editing outside the feature package would broaden the diff."],
        "validation_expectations": ["Keep go test coverage green for the feature package."],
        "generator_guardrails": ["Do not modify unrelated files."],
        "blocked_assumptions": []
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

func copyDirectory(source string, destination string) error {
	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relativePath, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(destination, relativePath)
		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}
		return copyFile(path, targetPath, info.Mode())
	})
}

func copyFile(source string, destination string, mode os.FileMode) error {
	sourceFile, err := os.Open(source)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	destinationFile, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer destinationFile.Close()

	_, err = io.Copy(destinationFile, sourceFile)
	return err
}

func (s *stubCommandRunner) RunShell(command, workingDirectory string, timeout time.Duration) (CommandResult, error) {
	if s.call >= len(s.results) {
		return CommandResult{}, nil
	}
	result := s.results[s.call]
	s.call++
	return result, nil
}
