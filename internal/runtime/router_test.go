package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
		"terminal_status: blocked_environment",
	} {
		if !strings.Contains(string(executionReport), fragment) {
			t.Fatalf("expected enriched execution report to contain %q, got:\n%s", fragment, string(executionReport))
		}
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
