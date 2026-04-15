package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRouteEvaluationMapsFixtureToTightenValidation(t *testing.T) {
	router, err := NewRouter(testRepoRoot(t))
	if err != nil {
		t.Fatalf("NewRouter returned error: %v", err)
	}

	artifactPath := copyRouteFixture(t, "tighten_validation")
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

	router, err := NewRouter(testRepoRoot(t))
	if err != nil {
		t.Fatalf("NewRouter returned error: %v", err)
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			artifactPath := copyRouteFixture(t, tc.fixture)
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
		})
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
