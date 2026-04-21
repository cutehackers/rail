package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"rail/internal/contracts"
	"gopkg.in/yaml.v3"
)

func TestRouteEvaluationMapsFixtureToTightenValidation(t *testing.T) {
	projectRoot := t.TempDir()
	router, err := NewRouter(projectRoot)
	if err != nil {
		t.Fatalf("NewRouter returned error: %v", err)
	}

	artifactPath := copyRouteFixtureIntoProject(t, projectRoot, "tighten_validation")
	summary, err := router.RouteEvaluation(filepath.Join(artifactPath, "evaluation_result.yaml"))
	if err != nil {
		t.Fatalf("RouteEvaluation returned error: %v", err)
	}

	if !strings.Contains(summary, "status=tightening_validation") {
		t.Fatalf("expected summary to include tightening_validation, got %q", summary)
	}
	if !strings.Contains(summary, "action=tighten_validation") {
		t.Fatalf("expected summary to include tighten_validation action, got %q", summary)
	}

	state, err := readState(filepath.Join(artifactPath, "state.json"))
	if err != nil {
		t.Fatalf("failed to read state: %v", err)
	}
	if state.Status != "tightening_validation" {
		t.Fatalf("unexpected status: got %q want %q", state.Status, "tightening_validation")
	}
	if state.CurrentActor == nil || *state.CurrentActor != "executor" {
		t.Fatalf("unexpected currentActor: got %v want %q", state.CurrentActor, "executor")
	}
	executionReport, err := os.ReadFile(filepath.Join(artifactPath, "execution_report.yaml"))
	if err != nil {
		t.Fatalf("failed to read non-terminal execution_report.yaml: %v", err)
	}
	for _, fragment := range []string{
		"actor_graph:",
		"actor_profiles_used:",
		"critic_findings_applied:",
		"critic_to_evaluator_delta:",
		"quality_trajectory:",
		"terminal_status: tightening_validation",
	} {
		if !strings.Contains(string(executionReport), fragment) {
			t.Fatalf("expected non-terminal execution report to contain %q, got:\n%s", fragment, string(executionReport))
		}
	}
}

func TestRouteEvaluationCreatesTerminalSummaryForTerminalFixtures(t *testing.T) {
	tests := []struct {
		name                string
		fixture             string
		expectedStatus      string
		expectedAction      string
		expectedSummaryText string
	}{
		{
			name:                "split_task",
			fixture:             "split_task",
			expectedStatus:      "split_required",
			expectedAction:      "split_task",
			expectedSummaryText: "should be decomposed before continuing",
		},
		{
			name:                "blocked_environment",
			fixture:             "blocked_environment",
			expectedStatus:      "blocked_environment",
			expectedAction:      "block_environment",
			expectedSummaryText: "blocked by environment",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			projectRoot := t.TempDir()
			router, err := NewRouter(projectRoot)
			if err != nil {
				t.Fatalf("NewRouter returned error: %v", err)
			}
			artifactPath := copyRouteFixtureIntoProject(t, projectRoot, tc.fixture)
			summary, err := router.RouteEvaluation(artifactPath)
			if err != nil {
				t.Fatalf("RouteEvaluation returned error: %v", err)
			}
			if !strings.Contains(summary, "status="+tc.expectedStatus) {
				t.Fatalf("expected summary to include status %q, got %q", tc.expectedStatus, summary)
			}
			if !strings.Contains(summary, "action="+tc.expectedAction) {
				t.Fatalf("expected summary to include action %q, got %q", tc.expectedAction, summary)
			}

			terminalSummary, err := os.ReadFile(filepath.Join(artifactPath, "terminal_summary.md"))
			if err != nil {
				t.Fatalf("expected terminal_summary.md to exist: %v", err)
			}
			if !strings.Contains(string(terminalSummary), tc.expectedSummaryText) {
				t.Fatalf("unexpected terminal summary: %q", string(terminalSummary))
			}
			for _, fragment := range []string{
				"# Terminal Outcome",
				"## Guardrail Cost",
				"## Guardrail Value",
				"## Failure Details",
			} {
				if !strings.Contains(string(terminalSummary), fragment) {
					t.Fatalf("expected terminal summary to contain %q, got:\n%s", fragment, string(terminalSummary))
				}
			}
		})
	}
}

func TestRouteEvaluationRecoversTerminalSummaryOnRerun(t *testing.T) {
	projectRoot := t.TempDir()
	router, err := NewRouter(projectRoot)
	if err != nil {
		t.Fatalf("NewRouter returned error: %v", err)
	}

	artifactPath := copyRouteFixtureIntoProject(t, projectRoot, "blocked_environment")
	executionReportPath := filepath.Join(artifactPath, "execution_report.yaml")
	executionReportBody, err := os.ReadFile(executionReportPath)
	if err != nil {
		t.Fatalf("failed to read fixture execution_report.yaml: %v", err)
	}
	if err := os.Remove(executionReportPath); err != nil {
		t.Fatalf("failed to remove execution_report.yaml: %v", err)
	}

	_, err = router.RouteEvaluation(artifactPath)
	if err == nil {
		t.Fatalf("expected RouteEvaluation to fail when terminal summary inputs are missing")
	}
	if !strings.Contains(err.Error(), "execution_report.yaml") {
		t.Fatalf("expected missing execution report error, got %v", err)
	}

	state, err := readState(filepath.Join(artifactPath, "state.json"))
	if err != nil {
		t.Fatalf("failed to read state after failed terminal summary: %v", err)
	}
	if state.Status != "blocked_environment" {
		t.Fatalf("unexpected terminal status after failed summary write: got %q want %q", state.Status, "blocked_environment")
	}
	if state.CurrentActor != nil {
		t.Fatalf("expected CurrentActor to be nil after terminal routing failure, got %v", state.CurrentActor)
	}

	if err := os.WriteFile(executionReportPath, executionReportBody, 0o644); err != nil {
		t.Fatalf("failed to restore execution_report.yaml: %v", err)
	}

	summary, err := router.RouteEvaluation(artifactPath)
	if err != nil {
		t.Fatalf("expected rerun to recover terminal summary, got error: %v", err)
	}
	if strings.Contains(summary, "skipped") {
		t.Fatalf("expected rerun to refresh terminal outputs instead of skipping, got %q", summary)
	}
	if !strings.Contains(summary, "status=blocked_environment") {
		t.Fatalf("expected summary to include blocked_environment status, got %q", summary)
	}
	if !strings.Contains(summary, "action=block_environment") {
		t.Fatalf("expected summary to include block_environment action, got %q", summary)
	}

	terminalSummary, err := os.ReadFile(filepath.Join(artifactPath, "terminal_summary.md"))
	if err != nil {
		t.Fatalf("expected terminal_summary.md to exist after recovery: %v", err)
	}
	if !strings.Contains(string(terminalSummary), "blocked by environment") {
		t.Fatalf("unexpected recovered terminal summary contents:\n%s", string(terminalSummary))
	}
}

func TestRouteEvaluationRepairsTerminalExecutionReportOnRerun(t *testing.T) {
	projectRoot := t.TempDir()
	router, err := NewRouter(projectRoot)
	if err != nil {
		t.Fatalf("NewRouter returned error: %v", err)
	}

	artifactPath := copyRouteFixtureIntoProject(t, projectRoot, "blocked_environment")
	if _, err := router.RouteEvaluation(artifactPath); err != nil {
		t.Fatalf("RouteEvaluation returned error: %v", err)
	}

	executionReportPath := filepath.Join(artifactPath, "execution_report.yaml")
	staleExecutionReport := `format: fail
analyze: pass
tests:
  total: 3
  passed: 1
  failed: 2
failure_details:
  - stale terminal base report
logs:
  - go test ./...
`
	if err := os.WriteFile(executionReportPath, []byte(staleExecutionReport), 0o644); err != nil {
		t.Fatalf("failed to write stale terminal execution_report.yaml: %v", err)
	}

	summary, err := router.RouteEvaluation(artifactPath)
	if err != nil {
		t.Fatalf("expected rerun to repair stale terminal execution report, got error: %v", err)
	}
	if strings.Contains(summary, "skipped") {
		t.Fatalf("expected rerun to refresh stale terminal execution report instead of skipping, got %q", summary)
	}

	executionReport, err := os.ReadFile(executionReportPath)
	if err != nil {
		t.Fatalf("expected execution_report.yaml to exist after repair: %v", err)
	}
	for _, fragment := range []string{
		"actor_graph:",
		"actor_profiles_used:",
		"critic_findings_applied:",
		"critic_to_evaluator_delta:",
		"quality_trajectory:",
		"terminal_status: blocked_environment",
	} {
		if !strings.Contains(string(executionReport), fragment) {
			t.Fatalf("expected repaired terminal execution report to contain %q, got:\n%s", fragment, string(executionReport))
		}
	}
}

func TestRouteEvaluationRepairsMissingTerminalExecutionReportOnRerun(t *testing.T) {
	projectRoot := t.TempDir()
	router, err := NewRouter(projectRoot)
	if err != nil {
		t.Fatalf("NewRouter returned error: %v", err)
	}

	artifactPath := copyRouteFixtureIntoProject(t, projectRoot, "blocked_environment")
	if _, err := router.RouteEvaluation(artifactPath); err != nil {
		t.Fatalf("RouteEvaluation returned error: %v", err)
	}

	executionReportPath := filepath.Join(artifactPath, "execution_report.yaml")
	if err := os.Remove(executionReportPath); err != nil {
		t.Fatalf("failed to remove terminal execution_report.yaml: %v", err)
	}

	summary, err := router.RouteEvaluation(artifactPath)
	if err != nil {
		t.Fatalf("expected rerun to repair missing terminal execution report, got error: %v", err)
	}
	if strings.Contains(summary, "skipped") {
		t.Fatalf("expected rerun to refresh missing terminal execution report instead of skipping, got %q", summary)
	}

	executionReport, err := os.ReadFile(executionReportPath)
	if err != nil {
		t.Fatalf("expected execution_report.yaml to exist after repair: %v", err)
	}
	for _, fragment := range []string{
		"actor_graph:",
		"actor_profiles_used:",
		"critic_findings_applied:",
		"critic_to_evaluator_delta:",
		"terminal_status: blocked_environment",
	} {
		if !strings.Contains(string(executionReport), fragment) {
			t.Fatalf("expected repaired terminal execution report to contain %q, got:\n%s", fragment, string(executionReport))
		}
	}
}

func TestRouteEvaluationRecoversSupervisorTraceOnRerun(t *testing.T) {
	projectRoot := t.TempDir()
	router, err := NewRouter(projectRoot)
	if err != nil {
		t.Fatalf("NewRouter returned error: %v", err)
	}

	artifactPath := copyRouteFixtureIntoProject(t, projectRoot, "tighten_validation")
	summary, err := router.RouteEvaluation(artifactPath)
	if err != nil {
		t.Fatalf("RouteEvaluation returned error: %v", err)
	}
	if !strings.Contains(summary, "status=tightening_validation") {
		t.Fatalf("expected initial summary to include tightening_validation, got %q", summary)
	}

	tracePath := filepath.Join(artifactPath, "supervisor_trace.md")
	if err := os.Remove(tracePath); err != nil {
		t.Fatalf("failed to remove supervisor_trace.md: %v", err)
	}

	summary, err = router.RouteEvaluation(artifactPath)
	if err != nil {
		t.Fatalf("expected rerun to recover supervisor trace, got error: %v", err)
	}
	if strings.Contains(summary, "skipped") {
		t.Fatalf("expected rerun to refresh supervisor trace instead of skipping, got %q", summary)
	}
	if !strings.Contains(summary, "status=tightening_validation") {
		t.Fatalf("expected rerun summary to include tightening_validation, got %q", summary)
	}
	if !strings.Contains(summary, "action=tighten_validation") {
		t.Fatalf("expected rerun summary to include tighten_validation, got %q", summary)
	}

	trace, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatalf("expected supervisor_trace.md to exist after recovery: %v", err)
	}
	for _, fragment := range []string{
		"# Supervisor Decision Trace",
		"## Iteration 1",
		"- selected_action: `tighten_validation`",
	} {
		if !strings.Contains(string(trace), fragment) {
			t.Fatalf("expected recovered supervisor trace to contain %q, got:\n%s", fragment, string(trace))
		}
	}
}

func TestRouteEvaluationRecoversNonTerminalExecutionReportOnRerun(t *testing.T) {
	projectRoot := t.TempDir()
	router, err := NewRouter(projectRoot)
	if err != nil {
		t.Fatalf("NewRouter returned error: %v", err)
	}

	artifactPath := copyRouteFixtureIntoProject(t, projectRoot, "tighten_validation")
	if _, err := router.RouteEvaluation(artifactPath); err != nil {
		t.Fatalf("RouteEvaluation returned error: %v", err)
	}

	executionReportPath := filepath.Join(artifactPath, "execution_report.yaml")
	if err := os.Remove(executionReportPath); err != nil {
		t.Fatalf("failed to remove execution_report.yaml: %v", err)
	}

	summary, err := router.RouteEvaluation(artifactPath)
	if err != nil {
		t.Fatalf("expected rerun to recover execution report, got error: %v", err)
	}
	if strings.Contains(summary, "skipped") {
		t.Fatalf("expected rerun to refresh execution report instead of skipping, got %q", summary)
	}

	executionReport, err := os.ReadFile(executionReportPath)
	if err != nil {
		t.Fatalf("expected execution_report.yaml to exist after recovery: %v", err)
	}
	for _, fragment := range []string{
		"actor_graph:",
		"actor_profiles_used:",
		"critic_findings_applied:",
		"critic_to_evaluator_delta:",
		"terminal_status: tightening_validation",
	} {
		if !strings.Contains(string(executionReport), fragment) {
			t.Fatalf("expected recovered execution report to contain %q, got:\n%s", fragment, string(executionReport))
		}
	}
}

func TestRouteEvaluationRepairsStaleNonTerminalExecutionReportOnRerun(t *testing.T) {
	projectRoot := t.TempDir()
	router, err := NewRouter(projectRoot)
	if err != nil {
		t.Fatalf("NewRouter returned error: %v", err)
	}

	artifactPath := copyRouteFixtureIntoProject(t, projectRoot, "tighten_validation")
	if _, err := router.RouteEvaluation(artifactPath); err != nil {
		t.Fatalf("RouteEvaluation returned error: %v", err)
	}

	executionReportPath := filepath.Join(artifactPath, "execution_report.yaml")
	staleExecutionReport := `format: fail
analyze: pass
tests:
  total: 2
  passed: 1
  failed: 1
failure_details:
  - stale base report
logs:
  - go test ./...
`
	if err := os.WriteFile(executionReportPath, []byte(staleExecutionReport), 0o644); err != nil {
		t.Fatalf("failed to write stale execution_report.yaml: %v", err)
	}

	summary, err := router.RouteEvaluation(artifactPath)
	if err != nil {
		t.Fatalf("expected rerun to repair stale execution report, got error: %v", err)
	}
	if strings.Contains(summary, "skipped") {
		t.Fatalf("expected rerun to refresh stale execution report instead of skipping, got %q", summary)
	}

	executionReport, err := os.ReadFile(executionReportPath)
	if err != nil {
		t.Fatalf("expected execution_report.yaml to exist after repair: %v", err)
	}
	for _, fragment := range []string{
		"actor_graph:",
		"actor_profiles_used:",
		"critic_findings_applied:",
		"critic_to_evaluator_delta:",
		"quality_trajectory:",
		"terminal_status: tightening_validation",
	} {
		if !strings.Contains(string(executionReport), fragment) {
			t.Fatalf("expected repaired execution report to contain %q, got:\n%s", fragment, string(executionReport))
		}
	}
}

func TestRouteEvaluationRepairsMalformedNonTerminalExecutionReportOnRerun(t *testing.T) {
	projectRoot := t.TempDir()
	router, err := NewRouter(projectRoot)
	if err != nil {
		t.Fatalf("NewRouter returned error: %v", err)
	}

	artifactPath := copyRouteFixtureIntoProject(t, projectRoot, "tighten_validation")
	if _, err := router.RouteEvaluation(artifactPath); err != nil {
		t.Fatalf("RouteEvaluation returned error: %v", err)
	}

	executionReportPath := filepath.Join(artifactPath, "execution_report.yaml")
	malformedExecutionReport := `format: fail
analyze: pass
tests:
  total: 2
  passed: 1
  failed: 1
failure_details:
  - malformed base report
logs:
  - go test ./...
actor_graph: [
`
	if err := os.WriteFile(executionReportPath, []byte(malformedExecutionReport), 0o644); err != nil {
		t.Fatalf("failed to write malformed execution_report.yaml: %v", err)
	}

	summary, err := router.RouteEvaluation(artifactPath)
	if err != nil {
		t.Fatalf("expected rerun to repair malformed execution report, got error: %v", err)
	}
	if strings.Contains(summary, "skipped") {
		t.Fatalf("expected rerun to refresh malformed execution report instead of skipping, got %q", summary)
	}

	executionReport, err := os.ReadFile(executionReportPath)
	if err != nil {
		t.Fatalf("expected execution_report.yaml to exist after repair: %v", err)
	}
	for _, fragment := range []string{
		"actor_graph:",
		"actor_profiles_used:",
		"critic_findings_applied:",
		"critic_to_evaluator_delta:",
		"quality_trajectory:",
		"terminal_status: tightening_validation",
	} {
		if !strings.Contains(string(executionReport), fragment) {
			t.Fatalf("expected repaired execution report to contain %q, got:\n%s", fragment, string(executionReport))
		}
	}
}

func TestRouteEvaluationDoesNotFabricatePriorTraceDuringEvaluatorRerun(t *testing.T) {
	projectRoot := t.TempDir()
	router, err := NewRouter(projectRoot)
	if err != nil {
		t.Fatalf("NewRouter returned error: %v", err)
	}

	artifactPath := copyRouteFixtureIntoProject(t, projectRoot, "tighten_validation")
	firstSummary, err := router.RouteEvaluation(artifactPath)
	if err != nil {
		t.Fatalf("first RouteEvaluation returned error: %v", err)
	}
	if !strings.Contains(firstSummary, "status=tightening_validation") {
		t.Fatalf("expected first summary to include tightening_validation, got %q", firstSummary)
	}

	statePath := filepath.Join(artifactPath, "state.json")
	state, err := readState(statePath)
	if err != nil {
		t.Fatalf("failed to read state after first routing: %v", err)
	}
	state.CurrentActor = stringPtr("evaluator")
	if err := writeJSON(statePath, state); err != nil {
		t.Fatalf("failed to persist evaluator reentry state: %v", err)
	}

	passEvaluation := `decision: pass
scores:
  requirements: 0.9
  architecture: 0.8
  regression_risk: 0.8
quality_confidence: high
findings:
  - Validation completed successfully.
reason_codes: []
`
	if err := os.WriteFile(filepath.Join(artifactPath, "evaluation_result.yaml"), []byte(passEvaluation), 0o644); err != nil {
		t.Fatalf("failed to write pass evaluation_result.yaml: %v", err)
	}
	tracePath := filepath.Join(artifactPath, "supervisor_trace.md")
	if err := os.Remove(tracePath); err != nil {
		t.Fatalf("failed to remove supervisor_trace.md: %v", err)
	}

	secondSummary, err := router.RouteEvaluation(artifactPath)
	if err != nil {
		t.Fatalf("second RouteEvaluation returned error: %v", err)
	}
	if !strings.Contains(secondSummary, "status=passed") {
		t.Fatalf("expected second summary to include passed status, got %q", secondSummary)
	}
	if !strings.Contains(secondSummary, "action=pass") {
		t.Fatalf("expected second summary to include pass action, got %q", secondSummary)
	}

	executedState, err := readState(statePath)
	if err != nil {
		t.Fatalf("failed to read state after evaluator rerun: %v", err)
	}
	if executedState.Status != "passed" {
		t.Fatalf("unexpected state status after evaluator rerun: got %q want %q", executedState.Status, "passed")
	}
	if executedState.CurrentActor != nil {
		t.Fatalf("expected CurrentActor to be nil after pass, got %v", executedState.CurrentActor)
	}

	trace, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatalf("expected supervisor_trace.md to exist after rerun: %v", err)
	}
	for _, fragment := range []string{
		"## Iteration 2",
		"- decision: `pass`",
		"- selected_action: `pass`",
		"- terminal_status: `passed`",
	} {
		if !strings.Contains(string(trace), fragment) {
			t.Fatalf("expected rerun trace to contain %q, got:\n%s", fragment, string(trace))
		}
	}
	for _, fragment := range []string{
		"## Iteration 1\n\n- decision: `pass`",
		"## Iteration 1\n\n- selected_action: `pass`",
	} {
		if strings.Contains(string(trace), fragment) {
			t.Fatalf("expected rerun trace to avoid fabricating prior pass iteration, got:\n%s", string(trace))
		}
	}
}

func TestRouteEvaluationWritesConcreteSupervisorTrace(t *testing.T) {
	projectRoot := t.TempDir()
	router, err := NewRouter(projectRoot)
	if err != nil {
		t.Fatalf("NewRouter returned error: %v", err)
	}

	artifactPath := copyRouteFixtureIntoProject(t, projectRoot, "tighten_validation")
	if _, err := router.RouteEvaluation(artifactPath); err != nil {
		t.Fatalf("RouteEvaluation returned error: %v", err)
	}

	trace, err := os.ReadFile(filepath.Join(artifactPath, "supervisor_trace.md"))
	if err != nil {
		t.Fatalf("expected supervisor_trace.md to exist: %v", err)
	}
	for _, fragment := range []string{
		"# Supervisor Decision Trace",
		"## Iteration 1",
		"- selected_action: `tighten_validation`",
		"- reason_codes: `validation_scope_missing_target`",
		"- guardrail_cost: `generator_revisions_used=0, context_rebuilds_used=0, validation_tightenings_used=0`",
		"- budgets_remaining: `generator=1, context=1, validation=0`",
	} {
		if !strings.Contains(string(trace), fragment) {
			t.Fatalf("expected supervisor trace to contain %q, got:\n%s", fragment, string(trace))
		}
	}
}

func TestRouteEvaluationKeepsContextRefreshPendingUntilExecuted(t *testing.T) {
	projectRoot := t.TempDir()
	router, err := NewRouter(projectRoot)
	if err != nil {
		t.Fatalf("NewRouter returned error: %v", err)
	}

	artifactPath := copyRouteFixtureIntoProject(t, projectRoot, "rebuild_context")
	summary, err := router.RouteEvaluation(artifactPath)
	if err != nil {
		t.Fatalf("RouteEvaluation returned error: %v", err)
	}
	if !strings.Contains(summary, "status=rebuilding_context") {
		t.Fatalf("expected summary to include rebuilding_context, got %q", summary)
	}

	state, err := readState(filepath.Join(artifactPath, "state.json"))
	if err != nil {
		t.Fatalf("failed to read state: %v", err)
	}
	if state.ContextRefreshCount != 0 {
		t.Fatalf("unexpected ContextRefreshCount: got %d want %d", state.ContextRefreshCount, 0)
	}
	if state.LastContextRefreshTrigger != nil {
		t.Fatalf("expected LastContextRefreshTrigger to remain nil before execution, got %v", state.LastContextRefreshTrigger)
	}
	if state.LastContextRefreshReasonFamily != nil {
		t.Fatalf("expected LastContextRefreshReasonFamily to remain nil before execution, got %v", state.LastContextRefreshReasonFamily)
	}
	if state.PendingContextRefreshTrigger == nil || *state.PendingContextRefreshTrigger != "reason_codes" {
		t.Fatalf(
			"unexpected PendingContextRefreshTrigger: got %v want %q",
			state.PendingContextRefreshTrigger,
			"reason_codes",
		)
	}
	if state.PendingContextRefreshReasonFamily == nil || *state.PendingContextRefreshReasonFamily != "context" {
		t.Fatalf(
			"unexpected PendingContextRefreshReasonFamily: got %v want %q",
			state.PendingContextRefreshReasonFamily,
			"context",
		)
	}

	trace, err := os.ReadFile(filepath.Join(artifactPath, "supervisor_trace.md"))
	if err != nil {
		t.Fatalf("expected supervisor_trace.md to exist: %v", err)
	}
	for _, fragment := range []string{
		"- context_refresh: `count=0, last_trigger=none, last_reason_family=none`",
		"- guardrail_cost: `generator_revisions_used=0, context_rebuilds_used=0, validation_tightenings_used=0`",
	} {
		if !strings.Contains(string(trace), fragment) {
			t.Fatalf("expected supervisor trace to contain %q, got:\n%s", fragment, string(trace))
		}
	}
}

func TestRouteEvaluationLeavesGeneratorRevisionCountAtSelectedWork(t *testing.T) {
	projectRoot := t.TempDir()
	router, err := NewRouter(projectRoot)
	if err != nil {
		t.Fatalf("NewRouter returned error: %v", err)
	}

	artifactPath := copyRouteFixtureIntoProject(t, projectRoot, "tighten_validation")
	evaluationBody := `decision: revise
scores:
  requirements: 0.4
  architecture: 0.8
  regression_risk: 0.3
quality_confidence: medium
findings:
  - Generator output still violates the requested behavior.
reason_codes:
  - implementation_patch_invalid
`
	if err := os.WriteFile(filepath.Join(artifactPath, "evaluation_result.yaml"), []byte(evaluationBody), 0o644); err != nil {
		t.Fatalf("failed to override evaluation_result.yaml: %v", err)
	}

	summary, err := router.RouteEvaluation(artifactPath)
	if err != nil {
		t.Fatalf("RouteEvaluation returned error: %v", err)
	}
	if !strings.Contains(summary, "action=revise_generator") {
		t.Fatalf("expected summary to include revise_generator, got %q", summary)
	}

	state, err := readState(filepath.Join(artifactPath, "state.json"))
	if err != nil {
		t.Fatalf("failed to read state: %v", err)
	}
	if state.GeneratorRevisionsUsed != 0 {
		t.Fatalf("unexpected GeneratorRevisionsUsed: got %d want %d", state.GeneratorRevisionsUsed, 0)
	}

	trace, err := os.ReadFile(filepath.Join(artifactPath, "supervisor_trace.md"))
	if err != nil {
		t.Fatalf("expected supervisor_trace.md to exist: %v", err)
	}
	if !strings.Contains(string(trace), "- guardrail_cost: `generator_revisions_used=0, context_rebuilds_used=0, validation_tightenings_used=0`") {
		t.Fatalf("expected supervisor trace to report generator revision usage, got:\n%s", string(trace))
	}
}

func TestRouteEvaluationCountsCorrectiveWorkOnlyAfterEvaluatorReentry(t *testing.T) {
	tests := []struct {
		name                  string
		fixture               string
		overrideEvaluation    string
		selectedAction        string
		selectedStatus        string
		assertSelectedZero    func(t *testing.T, state State)
		assertExecutedCount   func(t *testing.T, state State)
		assertExecutedTrace   string
		assertTerminalSummary string
	}{
		{
			name:           "context_rebuild",
			fixture:        "rebuild_context",
			selectedAction: "rebuild_context",
			selectedStatus: "rebuilding_context",
			assertSelectedZero: func(t *testing.T, state State) {
				t.Helper()
				if state.ContextRefreshCount != 0 {
					t.Fatalf("unexpected ContextRefreshCount at selection: got %d want %d", state.ContextRefreshCount, 0)
				}
			},
			assertExecutedCount: func(t *testing.T, state State) {
				t.Helper()
				if state.ContextRefreshCount != 1 {
					t.Fatalf("unexpected ContextRefreshCount after evaluator reentry: got %d want %d", state.ContextRefreshCount, 1)
				}
				if state.LastContextRefreshTrigger == nil || *state.LastContextRefreshTrigger != "reason_codes" {
					t.Fatalf("unexpected LastContextRefreshTrigger: got %v want %q", state.LastContextRefreshTrigger, "reason_codes")
				}
				if state.LastContextRefreshReasonFamily == nil || *state.LastContextRefreshReasonFamily != "context" {
					t.Fatalf(
						"unexpected LastContextRefreshReasonFamily: got %v want %q",
						state.LastContextRefreshReasonFamily,
						"context",
					)
				}
				if state.PendingContextRefreshTrigger != nil {
					t.Fatalf("expected PendingContextRefreshTrigger to be cleared, got %v", state.PendingContextRefreshTrigger)
				}
				if state.PendingContextRefreshReasonFamily != nil {
					t.Fatalf(
						"expected PendingContextRefreshReasonFamily to be cleared, got %v",
						state.PendingContextRefreshReasonFamily,
					)
				}
			},
			assertExecutedTrace:   "- guardrail_cost: `generator_revisions_used=0, context_rebuilds_used=1, validation_tightenings_used=0`",
			assertTerminalSummary: "- action: `pass`",
		},
		{
			name:           "validation_tightening",
			fixture:        "tighten_validation",
			selectedAction: "tighten_validation",
			selectedStatus: "tightening_validation",
			assertSelectedZero: func(t *testing.T, state State) {
				t.Helper()
				if state.ValidationTighteningsUsed != 0 {
					t.Fatalf(
						"unexpected ValidationTighteningsUsed at selection: got %d want %d",
						state.ValidationTighteningsUsed,
						0,
					)
				}
			},
			assertExecutedCount: func(t *testing.T, state State) {
				t.Helper()
				if state.ValidationTighteningsUsed != 1 {
					t.Fatalf(
						"unexpected ValidationTighteningsUsed after evaluator reentry: got %d want %d",
						state.ValidationTighteningsUsed,
						1,
					)
				}
			},
			assertExecutedTrace:   "- guardrail_cost: `generator_revisions_used=0, context_rebuilds_used=0, validation_tightenings_used=1`",
			assertTerminalSummary: "- action: `pass`",
		},
		{
			name:    "generator_revision",
			fixture: "tighten_validation",
			overrideEvaluation: `decision: revise
scores:
  requirements: 0.4
  architecture: 0.8
  regression_risk: 0.3
quality_confidence: medium
findings:
  - Generator output still violates the requested behavior.
reason_codes:
  - implementation_patch_invalid
`,
			selectedAction: "revise_generator",
			selectedStatus: "revising",
			assertSelectedZero: func(t *testing.T, state State) {
				t.Helper()
				if state.GeneratorRevisionsUsed != 0 {
					t.Fatalf("unexpected GeneratorRevisionsUsed at selection: got %d want %d", state.GeneratorRevisionsUsed, 0)
				}
			},
			assertExecutedCount: func(t *testing.T, state State) {
				t.Helper()
				if state.GeneratorRevisionsUsed != 1 {
					t.Fatalf(
						"unexpected GeneratorRevisionsUsed after evaluator reentry: got %d want %d",
						state.GeneratorRevisionsUsed,
						1,
					)
				}
			},
			assertExecutedTrace:   "- guardrail_cost: `generator_revisions_used=1, context_rebuilds_used=0, validation_tightenings_used=0`",
			assertTerminalSummary: "- action: `pass`",
		},
	}

	passEvaluation := `decision: pass
scores:
  requirements: 1
  architecture: 1
  regression_risk: 0
quality_confidence: high
findings: []
reason_codes: []
`

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			projectRoot := t.TempDir()
			router, err := NewRouter(projectRoot)
			if err != nil {
				t.Fatalf("NewRouter returned error: %v", err)
			}

			artifactPath := copyRouteFixtureIntoProject(t, projectRoot, tc.fixture)
			if tc.overrideEvaluation != "" {
				if err := os.WriteFile(filepath.Join(artifactPath, "evaluation_result.yaml"), []byte(tc.overrideEvaluation), 0o644); err != nil {
					t.Fatalf("failed to override evaluation_result.yaml: %v", err)
				}
			}

			summary, err := router.RouteEvaluation(artifactPath)
			if err != nil {
				t.Fatalf("first RouteEvaluation returned error: %v", err)
			}
			if !strings.Contains(summary, "status="+tc.selectedStatus) {
				t.Fatalf("expected first summary to include status %q, got %q", tc.selectedStatus, summary)
			}
			if !strings.Contains(summary, "action="+tc.selectedAction) {
				t.Fatalf("expected first summary to include action %q, got %q", tc.selectedAction, summary)
			}

			statePath := filepath.Join(artifactPath, "state.json")
			state, err := readState(statePath)
			if err != nil {
				t.Fatalf("failed to read state after first routing: %v", err)
			}
			tc.assertSelectedZero(t, state)

			state.CurrentActor = stringPtr("evaluator")
			if err := writeJSON(statePath, state); err != nil {
				t.Fatalf("failed to persist evaluator reentry state: %v", err)
			}
			if err := os.WriteFile(filepath.Join(artifactPath, "evaluation_result.yaml"), []byte(passEvaluation), 0o644); err != nil {
				t.Fatalf("failed to write pass evaluation_result.yaml: %v", err)
			}

			secondSummary, err := router.RouteEvaluation(artifactPath)
			if err != nil {
				t.Fatalf("second RouteEvaluation returned error: %v", err)
			}
			if !strings.Contains(secondSummary, "status=passed") {
				t.Fatalf("expected second summary to include passed status, got %q", secondSummary)
			}
			if !strings.Contains(secondSummary, "action=pass") {
				t.Fatalf("expected second summary to include pass action, got %q", secondSummary)
			}

			executedState, err := readState(statePath)
			if err != nil {
				t.Fatalf("failed to read state after evaluator reentry: %v", err)
			}
			tc.assertExecutedCount(t, executedState)

			trace, err := os.ReadFile(filepath.Join(artifactPath, "supervisor_trace.md"))
			if err != nil {
				t.Fatalf("expected supervisor_trace.md to exist: %v", err)
			}
			if !strings.Contains(string(trace), tc.assertExecutedTrace) {
				t.Fatalf("expected supervisor trace to contain %q, got:\n%s", tc.assertExecutedTrace, string(trace))
			}

			terminalSummary, err := os.ReadFile(filepath.Join(artifactPath, "terminal_summary.md"))
			if err != nil {
				t.Fatalf("expected terminal_summary.md to exist: %v", err)
			}
			if !strings.Contains(string(terminalSummary), tc.assertTerminalSummary) {
				t.Fatalf("expected terminal summary to contain %q, got:\n%s", tc.assertTerminalSummary, string(terminalSummary))
			}
		})
	}
}

func TestRouteEvaluationPreservesSelectedActionOnBudgetExhaustion(t *testing.T) {
	tests := []struct {
		name                  string
		fixture               string
		overrideEvaluation    string
		prepareState          func(state *State)
		expectedStatus        string
		expectedAction        string
		expectedTraceStatus   string
		expectedTerminalLabel string
	}{
		{
			name:                  "generator_revision_budget_exhausted",
			fixture:               "tighten_validation",
			expectedStatus:        "revise_exhausted",
			expectedAction:        "revise_generator",
			expectedTraceStatus:   "- terminal_status: `revise_exhausted`",
			expectedTerminalLabel: "- action: `revise_generator`",
			overrideEvaluation: `decision: revise
scores:
  requirements: 0.4
  architecture: 0.8
  regression_risk: 0.3
quality_confidence: medium
findings:
  - Generator output still violates the requested behavior.
reason_codes:
  - implementation_patch_invalid
`,
			prepareState: func(state *State) {
				state.GeneratorRetriesRemaining = 0
			},
		},
		{
			name:                  "context_rebuild_budget_exhausted",
			fixture:               "rebuild_context",
			expectedStatus:        "evolution_exhausted",
			expectedAction:        "rebuild_context",
			expectedTraceStatus:   "- terminal_status: `evolution_exhausted`",
			expectedTerminalLabel: "- action: `rebuild_context`",
			prepareState: func(state *State) {
				state.ContextRebuildsRemaining = 0
			},
		},
		{
			name:                  "validation_tightening_budget_exhausted",
			fixture:               "tighten_validation",
			expectedStatus:        "evolution_exhausted",
			expectedAction:        "tighten_validation",
			expectedTraceStatus:   "- terminal_status: `evolution_exhausted`",
			expectedTerminalLabel: "- action: `tighten_validation`",
			prepareState: func(state *State) {
				state.ValidationTighteningsRemaining = 0
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			projectRoot := t.TempDir()
			router, err := NewRouter(projectRoot)
			if err != nil {
				t.Fatalf("NewRouter returned error: %v", err)
			}

			artifactPath := copyRouteFixtureIntoProject(t, projectRoot, tc.fixture)
			if tc.overrideEvaluation != "" {
				if err := os.WriteFile(filepath.Join(artifactPath, "evaluation_result.yaml"), []byte(tc.overrideEvaluation), 0o644); err != nil {
					t.Fatalf("failed to override evaluation_result.yaml: %v", err)
				}
			}

			statePath := filepath.Join(artifactPath, "state.json")
			state, err := readState(statePath)
			if err != nil {
				t.Fatalf("failed to read initial state: %v", err)
			}
			tc.prepareState(&state)
			if err := writeJSON(statePath, state); err != nil {
				t.Fatalf("failed to write prepared state: %v", err)
			}

			summary, err := router.RouteEvaluation(artifactPath)
			if err != nil {
				t.Fatalf("RouteEvaluation returned error: %v", err)
			}
			if !strings.Contains(summary, "status="+tc.expectedStatus) {
				t.Fatalf("expected summary to include status %q, got %q", tc.expectedStatus, summary)
			}
			if !strings.Contains(summary, "action="+tc.expectedAction) {
				t.Fatalf("expected summary to include action %q, got %q", tc.expectedAction, summary)
			}

			exhaustedState, err := readState(statePath)
			if err != nil {
				t.Fatalf("failed to read exhausted state: %v", err)
			}
			if exhaustedState.Status != tc.expectedStatus {
				t.Fatalf("unexpected status: got %q want %q", exhaustedState.Status, tc.expectedStatus)
			}
			if len(exhaustedState.ActionHistory) == 0 || exhaustedState.ActionHistory[len(exhaustedState.ActionHistory)-1] != tc.expectedAction {
				t.Fatalf("unexpected action history: got %v want last %q", exhaustedState.ActionHistory, tc.expectedAction)
			}

			trace, err := os.ReadFile(filepath.Join(artifactPath, "supervisor_trace.md"))
			if err != nil {
				t.Fatalf("expected supervisor_trace.md to exist: %v", err)
			}
			if !strings.Contains(string(trace), "- selected_action: `"+tc.expectedAction+"`") {
				t.Fatalf("expected supervisor trace to record selected action %q, got:\n%s", tc.expectedAction, string(trace))
			}
			if !strings.Contains(string(trace), tc.expectedTraceStatus) {
				t.Fatalf("expected supervisor trace to contain %q, got:\n%s", tc.expectedTraceStatus, string(trace))
			}

			terminalSummary, err := os.ReadFile(filepath.Join(artifactPath, "terminal_summary.md"))
			if err != nil {
				t.Fatalf("expected terminal_summary.md to exist: %v", err)
			}
			if !strings.Contains(string(terminalSummary), tc.expectedTerminalLabel) {
				t.Fatalf("expected terminal summary to contain %q, got:\n%s", tc.expectedTerminalLabel, string(terminalSummary))
			}
			if !strings.Contains(string(terminalSummary), "- last_intervention: `"+tc.expectedAction+"`") {
				t.Fatalf("expected terminal summary to preserve last intervention %q, got:\n%s", tc.expectedAction, string(terminalSummary))
			}
		})
	}
}

func TestRouteEvaluationRejectsArtifactOutsideProjectRoot(t *testing.T) {
	projectRoot := t.TempDir()
	router, err := NewRouter(projectRoot)
	if err != nil {
		t.Fatalf("NewRouter returned error: %v", err)
	}

	artifactPath := copyRouteFixture(t, "tighten_validation")
	_, err = router.RouteEvaluation(artifactPath)
	if err == nil {
		t.Fatalf("expected RouteEvaluation to reject artifact outside %q", projectRoot)
	}
	if !strings.Contains(err.Error(), "project root") {
		t.Fatalf("expected project-root confinement error, got %v", err)
	}
}

func TestRouteEvaluationEnrichesExecutionReportForTerminalOutcome(t *testing.T) {
	projectRoot := t.TempDir()
	router, err := NewRouter(projectRoot)
	if err != nil {
		t.Fatalf("NewRouter returned error: %v", err)
	}

	artifactPath := copyRouteFixtureIntoProject(t, projectRoot, "blocked_environment")
	if _, err := router.RouteEvaluation(artifactPath); err != nil {
		t.Fatalf("RouteEvaluation returned error: %v", err)
	}

	executionReport, err := os.ReadFile(filepath.Join(artifactPath, "execution_report.yaml"))
	if err != nil {
		t.Fatalf("expected enriched execution_report.yaml to exist: %v", err)
	}
	for _, fragment := range []string{
		"executed_intervention_count: 0",
		"context_refresh:",
		"count: 0",
		"guardrail_cost:",
		"generator_revisions_used: 0",
		"context_rebuilds_used: 0",
		"validation_tightenings_used: 0",
		"guardrail_value:",
		"trigger_failure_or_risk:",
		"environment_permission_denied",
		"trigger_reason_codes:",
		"- environment_permission_denied",
		"trigger_reason_category: environment",
		"last_intervention: block_environment",
		"quality_confidence: low",
		"outcome: bounded_refusal",
		"actor_profiles_used:",
		"- actor: planner",
		"- actor: critic",
		"critic_findings_applied:",
		"total_findings: 6",
		"unmet_count: 6",
		"critic_to_evaluator_delta:",
		"confirmed_count: 0",
		"path:",
		"- planner",
		"- critic",
		"terminal_status: blocked_environment",
	} {
		if !strings.Contains(string(executionReport), fragment) {
			t.Fatalf("expected enriched execution report to contain %q, got:\n%s", fragment, string(executionReport))
		}
	}
}

func TestRouteEvaluationFailsWhenRequiredCriticReportIsMissing(t *testing.T) {
	projectRoot := t.TempDir()
	router, err := NewRouter(projectRoot)
	if err != nil {
		t.Fatalf("NewRouter returned error: %v", err)
	}

	artifactPath := copyRouteFixtureIntoProject(t, projectRoot, "blocked_environment")
	if err := os.Remove(filepath.Join(artifactPath, "critic_report.yaml")); err != nil {
		t.Fatalf("failed to remove critic_report.yaml: %v", err)
	}

	_, err = router.RouteEvaluation(artifactPath)
	if err == nil {
		t.Fatalf("expected RouteEvaluation to fail without required critic_report")
	}
	if !strings.Contains(err.Error(), "critic_report") {
		t.Fatalf("expected missing critic_report error, got %v", err)
	}
}

func TestRouteEvaluationFailsNonTerminalWhenRequiredCriticReportIsMissing(t *testing.T) {
	projectRoot := t.TempDir()
	router, err := NewRouter(projectRoot)
	if err != nil {
		t.Fatalf("NewRouter returned error: %v", err)
	}

	artifactPath := copyRouteFixtureIntoProject(t, projectRoot, "tighten_validation")
	if err := os.Remove(filepath.Join(artifactPath, "critic_report.yaml")); err != nil {
		t.Fatalf("failed to remove critic_report.yaml: %v", err)
	}

	_, err = router.RouteEvaluation(artifactPath)
	if err == nil {
		t.Fatalf("expected non-terminal RouteEvaluation to fail without required critic_report")
	}
	if !strings.Contains(err.Error(), "critic_report") {
		t.Fatalf("expected missing critic_report error, got %v", err)
	}
}

func TestRouteEvaluationFailsWhenCriticIsInGraphButWorkflowOmitsCriticReportOutput(t *testing.T) {
	projectRoot := t.TempDir()
	router, err := NewRouter(projectRoot)
	if err != nil {
		t.Fatalf("NewRouter returned error: %v", err)
	}

	artifactPath := copyRouteFixtureIntoProject(t, projectRoot, "tighten_validation")
	workflowPath := filepath.Join(artifactPath, "workflow.json")
	workflow, err := readWorkflow(workflowPath)
	if err != nil {
		t.Fatalf("failed to read workflow: %v", err)
	}
	workflow.RequiredOutputs = []string{"plan", "context_pack", "implementation_result", "execution_report", "evaluation_result"}
	if err := writeJSON(workflowPath, workflow); err != nil {
		t.Fatalf("failed to rewrite workflow.json: %v", err)
	}
	if err := os.Remove(filepath.Join(artifactPath, "critic_report.yaml")); err != nil {
		t.Fatalf("failed to remove critic_report.yaml: %v", err)
	}

	_, err = router.RouteEvaluation(artifactPath)
	if err == nil {
		t.Fatalf("expected RouteEvaluation to fail when critic remains in graph but critic_report is missing")
	}
	if !strings.Contains(err.Error(), "critic_report") {
		t.Fatalf("expected critic_report error, got %v", err)
	}
}

func TestRouteEvaluationFailsWhenRequiredCriticReportIsMalformed(t *testing.T) {
	projectRoot := t.TempDir()
	router, err := NewRouter(projectRoot)
	if err != nil {
		t.Fatalf("NewRouter returned error: %v", err)
	}

	artifactPath := copyRouteFixtureIntoProject(t, projectRoot, "blocked_environment")
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

	_, err = router.RouteEvaluation(artifactPath)
	if err == nil {
		t.Fatalf("expected RouteEvaluation to fail for malformed critic_report")
	}
	if !strings.Contains(err.Error(), "critic_report") {
		t.Fatalf("expected critic_report validation error, got %v", err)
	}
}

func TestRouteEvaluationFailsNonTerminalWhenRequiredCriticReportIsMalformed(t *testing.T) {
	projectRoot := t.TempDir()
	router, err := NewRouter(projectRoot)
	if err != nil {
		t.Fatalf("NewRouter returned error: %v", err)
	}

	artifactPath := copyRouteFixtureIntoProject(t, projectRoot, "tighten_validation")
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

	_, err = router.RouteEvaluation(artifactPath)
	if err == nil {
		t.Fatalf("expected non-terminal RouteEvaluation to fail for malformed critic_report")
	}
	if !strings.Contains(err.Error(), "critic_report") {
		t.Fatalf("expected critic_report validation error, got %v", err)
	}
}

func TestRouteEvaluationDoesNotMarkUnrelatedPassPathCriticFindingsResolved(t *testing.T) {
	projectRoot := t.TempDir()
	router, err := NewRouter(projectRoot)
	if err != nil {
		t.Fatalf("NewRouter returned error: %v", err)
	}

	artifactPath := copyRouteFixtureIntoProject(t, projectRoot, "tighten_validation")
	if err := os.WriteFile(filepath.Join(artifactPath, "critic_report.yaml"), []byte(`priority_focus:
  - Keep repository docs unchanged.
missing_requirements:
  - Add CLI analytics dashboard support.
risk_hypotheses: []
validation_expectations: []
generator_guardrails: []
blocked_assumptions: []
`), 0o644); err != nil {
		t.Fatalf("failed to write critic_report.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(artifactPath, "evaluation_result.yaml"), []byte(`decision: pass
scores:
  requirements: 0.9
  architecture: 0.9
  regression_risk: 0.9
quality_confidence: high
findings:
  - Validation completed successfully.
reason_codes: []
`), 0o644); err != nil {
		t.Fatalf("failed to write evaluation_result.yaml: %v", err)
	}

	if _, err := router.RouteEvaluation(artifactPath); err != nil {
		t.Fatalf("RouteEvaluation returned error: %v", err)
	}

	executionReport, err := os.ReadFile(filepath.Join(artifactPath, "execution_report.yaml"))
	if err != nil {
		t.Fatalf("failed to read execution_report.yaml: %v", err)
	}
	if strings.Contains(string(executionReport), "status: resolved") {
		t.Fatalf("expected unrelated critic findings to avoid resolved status, got:\n%s", string(executionReport))
	}
	if !strings.Contains(string(executionReport), "status: unmet") {
		t.Fatalf("expected unrelated critic findings to remain unmet, got:\n%s", string(executionReport))
	}
}

func TestRouteEvaluationDoesNotTreatNegatedResolutionLanguageAsResolved(t *testing.T) {
	projectRoot := t.TempDir()
	router, err := NewRouter(projectRoot)
	if err != nil {
		t.Fatalf("NewRouter returned error: %v", err)
	}

	artifactPath := copyRouteFixtureIntoProject(t, projectRoot, "tighten_validation")
	if err := os.WriteFile(filepath.Join(artifactPath, "critic_report.yaml"), []byte(`priority_focus: []
missing_requirements:
  - Add CLI analytics dashboard support.
risk_hypotheses: []
validation_expectations: []
generator_guardrails: []
blocked_assumptions: []
`), 0o644); err != nil {
		t.Fatalf("failed to write critic_report.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(artifactPath, "evaluation_result.yaml"), []byte(`decision: pass
scores:
  requirements: 0.9
  architecture: 0.9
  regression_risk: 0.9
quality_confidence: high
findings:
  - CLI analytics dashboard support is not resolved and still not covered.
reason_codes: []
`), 0o644); err != nil {
		t.Fatalf("failed to write evaluation_result.yaml: %v", err)
	}

	if _, err := router.RouteEvaluation(artifactPath); err != nil {
		t.Fatalf("RouteEvaluation returned error: %v", err)
	}

	executionReport, err := os.ReadFile(filepath.Join(artifactPath, "execution_report.yaml"))
	if err != nil {
		t.Fatalf("failed to read execution_report.yaml: %v", err)
	}
	if strings.Contains(string(executionReport), "status: resolved") {
		t.Fatalf("expected negated resolution language to avoid resolved status, got:\n%s", string(executionReport))
	}
	if !strings.Contains(string(executionReport), "status: confirmed") {
		t.Fatalf("expected negated resolution language to remain confirmed-only, got:\n%s", string(executionReport))
	}
}

func TestRouteEvaluationDoesNotConfirmAllCategoryFindingsFromOneSignal(t *testing.T) {
	projectRoot := t.TempDir()
	router, err := NewRouter(projectRoot)
	if err != nil {
		t.Fatalf("NewRouter returned error: %v", err)
	}

	artifactPath := copyRouteFixtureIntoProject(t, projectRoot, "tighten_validation")
	if err := os.WriteFile(filepath.Join(artifactPath, "critic_report.yaml"), []byte(`priority_focus: []
missing_requirements:
  - Add validation target coverage for the CLI route.
  - Add regression coverage for environment failures.
risk_hypotheses: []
validation_expectations: []
generator_guardrails: []
blocked_assumptions: []
`), 0o644); err != nil {
		t.Fatalf("failed to write critic_report.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(artifactPath, "evaluation_result.yaml"), []byte(`decision: reject
scores:
  requirements: 0.5
  architecture: 0.9
  regression_risk: 0.4
quality_confidence: low
findings:
  - Validation target coverage is still missing for the CLI route.
reason_codes:
  - requirements_coverage_cli_route
`), 0o644); err != nil {
		t.Fatalf("failed to write evaluation_result.yaml: %v", err)
	}

	if _, err := router.RouteEvaluation(artifactPath); err != nil {
		t.Fatalf("RouteEvaluation returned error: %v", err)
	}

	executionReport, err := os.ReadFile(filepath.Join(artifactPath, "execution_report.yaml"))
	if err != nil {
		t.Fatalf("failed to read execution_report.yaml: %v", err)
	}
	if count := strings.Count(string(executionReport), "status: confirmed"); count != 1 {
		t.Fatalf("expected exactly one confirmed finding, got %d:\n%s", count, string(executionReport))
	}
	if count := strings.Count(string(executionReport), "status: unmet"); count == 0 {
		t.Fatalf("expected unmatched findings to remain unmet, got:\n%s", string(executionReport))
	}
}

func TestRouteEvaluationRejectsEnrichedExecutionReportMissingRequiredField(t *testing.T) {
	projectRoot := t.TempDir()
	router, err := NewRouter(projectRoot)
	if err != nil {
		t.Fatalf("NewRouter returned error: %v", err)
	}

	artifactPath := copyRouteFixtureIntoProject(t, projectRoot, "blocked_environment")
	if _, err := router.RouteEvaluation(artifactPath); err != nil {
		t.Fatalf("RouteEvaluation returned error: %v", err)
	}

	executionReportPath := filepath.Join(artifactPath, "execution_report.yaml")
	decoded, err := contracts.ReadYAMLFile(executionReportPath)
	if err != nil {
		t.Fatalf("failed to decode execution_report.yaml: %v", err)
	}
	reportMap, err := contracts.AsMap(decoded, "execution_report")
	if err != nil {
		t.Fatalf("failed to convert execution_report to map: %v", err)
	}
	delete(reportMap, "critic_to_evaluator_delta")
	body, err := yaml.Marshal(reportMap)
	if err != nil {
		t.Fatalf("failed to marshal execution report: %v", err)
	}
	if err := os.WriteFile(executionReportPath, body, 0o644); err != nil {
		t.Fatalf("failed to rewrite execution_report.yaml: %v", err)
	}

	if _, err := router.validator.ValidateArtifactFile(executionReportPath, "execution_report"); err == nil {
		t.Fatalf("expected execution_report validation to fail when critic_to_evaluator_delta is missing")
	}
}

func TestRouteEvaluationRejectsEnrichedExecutionReportMissingNestedRequiredField(t *testing.T) {
	projectRoot := t.TempDir()
	router, err := NewRouter(projectRoot)
	if err != nil {
		t.Fatalf("NewRouter returned error: %v", err)
	}

	artifactPath := copyRouteFixtureIntoProject(t, projectRoot, "blocked_environment")
	if _, err := router.RouteEvaluation(artifactPath); err != nil {
		t.Fatalf("RouteEvaluation returned error: %v", err)
	}

	executionReportPath := filepath.Join(artifactPath, "execution_report.yaml")
	decoded, err := contracts.ReadYAMLFile(executionReportPath)
	if err != nil {
		t.Fatalf("failed to decode execution_report.yaml: %v", err)
	}
	reportMap, err := contracts.AsMap(decoded, "execution_report")
	if err != nil {
		t.Fatalf("failed to convert execution_report to map: %v", err)
	}
	delta, err := contracts.AsMap(reportMap["critic_to_evaluator_delta"], "critic_to_evaluator_delta")
	if err != nil {
		t.Fatalf("failed to convert critic_to_evaluator_delta to map: %v", err)
	}
	delete(delta, "summary")
	body, err := yaml.Marshal(reportMap)
	if err != nil {
		t.Fatalf("failed to marshal execution report: %v", err)
	}
	if err := os.WriteFile(executionReportPath, body, 0o644); err != nil {
		t.Fatalf("failed to rewrite execution_report.yaml: %v", err)
	}

	if _, err := router.validator.ValidateArtifactFile(executionReportPath, "execution_report"); err == nil {
		t.Fatalf("expected execution_report validation to fail when critic_to_evaluator_delta.summary is missing")
	}
}

func TestRouteEvaluationRejectsMalformedActorProfilesUsedEntries(t *testing.T) {
	projectRoot := t.TempDir()
	router, err := NewRouter(projectRoot)
	if err != nil {
		t.Fatalf("NewRouter returned error: %v", err)
	}

	artifactPath := copyRouteFixtureIntoProject(t, projectRoot, "blocked_environment")
	if _, err := router.RouteEvaluation(artifactPath); err != nil {
		t.Fatalf("RouteEvaluation returned error: %v", err)
	}

	executionReportPath := filepath.Join(artifactPath, "execution_report.yaml")
	decoded, err := contracts.ReadYAMLFile(executionReportPath)
	if err != nil {
		t.Fatalf("failed to decode execution_report.yaml: %v", err)
	}
	reportMap, err := contracts.AsMap(decoded, "execution_report")
	if err != nil {
		t.Fatalf("failed to convert execution_report to map: %v", err)
	}
	entries, ok := reportMap["actor_profiles_used"].([]any)
	if !ok || len(entries) == 0 {
		t.Fatalf("expected actor_profiles_used entries, got %T", reportMap["actor_profiles_used"])
	}
	entry, ok := entries[0].(map[string]any)
	if !ok {
		t.Fatalf("expected actor_profiles_used[0] to be a map, got %T", entries[0])
	}
	delete(entry, "model")
	body, err := yaml.Marshal(reportMap)
	if err != nil {
		t.Fatalf("failed to marshal execution report: %v", err)
	}
	if err := os.WriteFile(executionReportPath, body, 0o644); err != nil {
		t.Fatalf("failed to rewrite execution_report.yaml: %v", err)
	}

	if _, err := router.validator.ValidateArtifactFile(executionReportPath, "execution_report"); err == nil {
		t.Fatalf("expected execution_report validation to fail when actor_profiles_used.model is missing")
	}
}

func TestRouteEvaluationFailsWhenActorProfilesSnapshotIsIncomplete(t *testing.T) {
	projectRoot := t.TempDir()
	router, err := NewRouter(projectRoot)
	if err != nil {
		t.Fatalf("NewRouter returned error: %v", err)
	}

	artifactPath := copyRouteFixtureIntoProject(t, projectRoot, "blocked_environment")
	statePath := filepath.Join(artifactPath, "state.json")
	state, err := readState(statePath)
	if err != nil {
		t.Fatalf("failed to read state: %v", err)
	}
	state.ActorProfilesUsed = state.ActorProfilesUsed[:2]
	if err := writeJSON(statePath, state); err != nil {
		t.Fatalf("failed to rewrite state.json: %v", err)
	}

	_, err = router.RouteEvaluation(artifactPath)
	if err == nil {
		t.Fatalf("expected RouteEvaluation to fail for incomplete actorProfilesUsed snapshot")
	}
	if !strings.Contains(err.Error(), "actorProfilesUsed") {
		t.Fatalf("expected actorProfilesUsed error, got %v", err)
	}
}

func TestRouteEvaluationFailsWhenActorProfilesSnapshotHasUnsupportedReasoning(t *testing.T) {
	projectRoot := t.TempDir()
	router, err := NewRouter(projectRoot)
	if err != nil {
		t.Fatalf("NewRouter returned error: %v", err)
	}

	artifactPath := copyRouteFixtureIntoProject(t, projectRoot, "blocked_environment")
	statePath := filepath.Join(artifactPath, "state.json")
	state, err := readState(statePath)
	if err != nil {
		t.Fatalf("failed to read state: %v", err)
	}
	state.ActorProfilesUsed[0].Reasoning = "critical"
	if err := writeJSON(statePath, state); err != nil {
		t.Fatalf("failed to rewrite state.json: %v", err)
	}

	_, err = router.RouteEvaluation(artifactPath)
	if err == nil {
		t.Fatalf("expected RouteEvaluation to fail for unsupported actorProfilesUsed reasoning")
	}
	if !strings.Contains(err.Error(), "unsupported reasoning") {
		t.Fatalf("expected unsupported reasoning error, got %v", err)
	}
}

func TestRouteEvaluationRequiresExecutionReportForTerminalOutcome(t *testing.T) {
	projectRoot := t.TempDir()
	router, err := NewRouter(projectRoot)
	if err != nil {
		t.Fatalf("NewRouter returned error: %v", err)
	}

	artifactPath := copyRouteFixtureIntoProject(t, projectRoot, "blocked_environment")
	executionReportPath := filepath.Join(artifactPath, "execution_report.yaml")
	if err := os.Remove(executionReportPath); err != nil {
		t.Fatalf("failed to remove execution_report.yaml: %v", err)
	}

	_, err = router.RouteEvaluation(artifactPath)
	if err == nil {
		t.Fatalf("expected RouteEvaluation to fail without execution_report.yaml")
	}
	if !strings.Contains(err.Error(), "execution_report.yaml") {
		t.Fatalf("expected missing execution report error, got %v", err)
	}
}

func TestRouteEvaluationPreservesApprovedMemoryTimestamp(t *testing.T) {
	projectRoot := t.TempDir()
	router, err := NewRouter(projectRoot)
	if err != nil {
		t.Fatalf("NewRouter returned error: %v", err)
	}

	artifactPath := copyRouteFixtureIntoProject(t, projectRoot, "blocked_environment")
	executionReportPath := filepath.Join(artifactPath, "execution_report.yaml")
	executionReportBody := `format: fail
analyze: fail
tests:
  total: 1
  passed: 0
  failed: 1
failure_details:
  - Permission denied while reading the target workspace.
logs:
  - chmod denied
  - analyzer could not inspect target files
approved_memory_consideration:
  considered_ref: memory/ref
  lookup_key: guardrail-key
  task_family_source: task_type
  disposition: reuse
  reasons:
    - preserved
  originating_candidate_refs:
    - candidate/ref
  current_state_refresh_ref: refresh/ref
  current_state_refresh_generated_at: 2026-04-15T12:34:56Z
`
	if err := os.WriteFile(executionReportPath, []byte(executionReportBody), 0o644); err != nil {
		t.Fatalf("failed to override execution_report.yaml: %v", err)
	}

	if _, err := router.RouteEvaluation(artifactPath); err != nil {
		t.Fatalf("RouteEvaluation returned error: %v", err)
	}

	decodedExecutionReport, err := contracts.ReadYAMLFile(executionReportPath)
	if err != nil {
		t.Fatalf("failed to decode enriched execution_report.yaml: %v", err)
	}
	executionReportMap, err := contracts.AsMap(decodedExecutionReport, "execution_report")
	if err != nil {
		t.Fatalf("failed to convert execution_report to map: %v", err)
	}
	approvedMemory, err := contracts.AsMap(executionReportMap["approved_memory_consideration"], "approved_memory_consideration")
	if err != nil {
		t.Fatalf("failed to convert approved_memory_consideration to map: %v", err)
	}

	generatedAt, ok := approvedMemory["current_state_refresh_generated_at"].(string)
	if !ok {
		t.Fatalf(
			"expected current_state_refresh_generated_at to round-trip as a string, got %T (%v)",
			approvedMemory["current_state_refresh_generated_at"],
			approvedMemory["current_state_refresh_generated_at"],
		)
	}
	expected := time.Date(2026, 4, 15, 12, 34, 56, 0, time.UTC).Format(time.RFC3339)
	if generatedAt != expected {
		t.Fatalf("unexpected current_state_refresh_generated_at: got %s want %s", generatedAt, expected)
	}
}

func copyRouteFixture(t *testing.T, fixtureName string) string {
	t.Helper()
	sourceRoot := filepath.Join(testRepoRoot(t), "test", "fixtures", "standard_route", fixtureName)
	targetRoot := filepath.Join(t.TempDir(), fixtureName)

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
		t.Fatalf("failed to copy fixture %q: %v", fixtureName, err)
	}

	return targetRoot
}

func copyRouteFixtureIntoProject(t *testing.T, projectRoot, fixtureName string) string {
	t.Helper()
	sourceRoot := filepath.Join(testRepoRoot(t), "test", "fixtures", "standard_route", fixtureName)
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
		t.Fatalf("failed to copy fixture %q into project: %v", fixtureName, err)
	}

	return targetRoot
}
