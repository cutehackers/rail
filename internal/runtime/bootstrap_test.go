package runtime

import (
	"os"
	"path/filepath"
	"testing"

	"rail/internal/contracts"
)

func TestValidateRequestAcceptsCurrentValidFixture(t *testing.T) {
	repoRoot := testRepoRoot(t)
	validator, err := contracts.NewValidator(repoRoot)
	if err != nil {
		t.Fatalf("NewValidator returned error: %v", err)
	}

	requestPath := filepath.Join(repoRoot, "test", "fixtures", "valid_request.yaml")
	requestValue, err := validator.ValidateRequestFile(requestPath)
	if err != nil {
		t.Fatalf("ValidateRequestFile returned error: %v", err)
	}

	if requestValue.TaskType != "test_repair" {
		t.Fatalf("unexpected task_type: got %q want %q", requestValue.TaskType, "test_repair")
	}
	if requestValue.ValidationProfile != "smoke" {
		t.Fatalf("unexpected validation_profile: got %q want %q", requestValue.ValidationProfile, "smoke")
	}
}

func TestBootstrapCreatesExpectedArtifactSkeleton(t *testing.T) {
	projectRoot := t.TempDir()
	bootstrapper, err := NewBootstrapper(projectRoot)
	if err != nil {
		t.Fatalf("NewBootstrapper returned error: %v", err)
	}

	requestPath := filepath.Join(projectRoot, "request.yaml")
	requestBody, err := os.ReadFile(filepath.Join(testRepoRoot(t), "test", "fixtures", "standard_route", "tighten_validation", "request.yaml"))
	if err != nil {
		t.Fatalf("failed to read fixture request: %v", err)
	}
	if err := os.WriteFile(requestPath, requestBody, 0o644); err != nil {
		t.Fatalf("failed to write request fixture: %v", err)
	}

	artifactPath, err := bootstrapper.Bootstrap(requestPath, "bootstrap-smoke")
	if err != nil {
		t.Fatalf("Bootstrap returned error: %v", err)
	}

	for _, relPath := range []string{
		"request.yaml",
		"resolved_workflow.json",
		"state.json",
		"execution_plan.json",
		"plan.yaml",
		"context_pack.yaml",
		"implementation_result.yaml",
		"execution_report.yaml",
		"evaluation_result.yaml",
	} {
		if _, err := os.Stat(filepath.Join(artifactPath, relPath)); err != nil {
			t.Fatalf("expected bootstrap artifact %q to exist: %v", relPath, err)
		}
	}

	workflow, err := readResolvedWorkflow(filepath.Join(artifactPath, "resolved_workflow.json"))
	if err != nil {
		t.Fatalf("failed to read resolved workflow: %v", err)
	}
	if workflow.TaskType != "bug_fix" {
		t.Fatalf("unexpected taskType: got %q want %q", workflow.TaskType, "bug_fix")
	}
	if workflow.GeneratorRetryBudget != 1 {
		t.Fatalf("unexpected generatorRetryBudget: got %d want %d", workflow.GeneratorRetryBudget, 1)
	}
	if workflow.ContextRebuildBudget != 1 || workflow.ValidationTightenBudget != 1 {
		t.Fatalf(
			"unexpected supervisor budgets: context=%d validation=%d",
			workflow.ContextRebuildBudget,
			workflow.ValidationTightenBudget,
		)
	}

	state, err := readState(filepath.Join(artifactPath, "state.json"))
	if err != nil {
		t.Fatalf("failed to read state: %v", err)
	}
	if state.Status != "initialized" {
		t.Fatalf("unexpected state status: got %q want %q", state.Status, "initialized")
	}
	if state.CurrentActor == nil || *state.CurrentActor != "planner" {
		t.Fatalf("unexpected currentActor: got %v want %q", state.CurrentActor, "planner")
	}
}

func testRepoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("failed to resolve repo root: %v", err)
	}
	return root
}
