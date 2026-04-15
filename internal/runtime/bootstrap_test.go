package runtime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

	for _, relPath := range []string{
		filepath.Join(".harness", "requests"),
		filepath.Join("lib"),
		filepath.Join("packages", "app", "test"),
	} {
		if err := os.MkdirAll(filepath.Join(projectRoot, relPath), 0o755); err != nil {
			t.Fatalf("failed to create %q: %v", relPath, err)
		}
	}

	requestPath := filepath.Join(projectRoot, ".harness", "requests", "request.yaml")
	requestBody := `task_type: bug_fix
goal: tighten the validation plan for profile changes
context:
  feature: profile
  suspected_files:
    - ` + filepath.ToSlash(filepath.Join(projectRoot, "lib", "profile.dart")) + `
  related_files:
    - packages/app/lib/profile_state.dart
  validation_roots:
    - ` + filepath.ToSlash(filepath.Join(projectRoot, "packages", "app")) + `
  validation_targets:
    - ` + filepath.ToSlash(filepath.Join(projectRoot, "packages", "app", "test", "profile_test.dart")) + `
constraints: []
definition_of_done:
  - validate the intended target set
risk_tolerance: low
validation_profile: standard
`
	if err := os.WriteFile(requestPath, []byte(requestBody), 0o644); err != nil {
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
		"workflow_steps.md",
		"plan.yaml",
		"context_pack.yaml",
		"implementation_result.yaml",
		"execution_report.yaml",
		"evaluation_result.yaml",
		filepath.Join("inputs", "architecture_rules.md"),
		filepath.Join("inputs", "project_rules.md"),
		filepath.Join("inputs", "forbidden_changes.md"),
		filepath.Join("inputs", "execution_policy.yaml"),
		filepath.Join("inputs", "rubric.yaml"),
		filepath.Join("actor_briefs", "01_planner.md"),
		filepath.Join("actor_briefs", "05_evaluator.md"),
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
	if workflow.RequestPath != ".harness/requests/request.yaml" {
		t.Fatalf("unexpected requestPath: got %q want %q", workflow.RequestPath, ".harness/requests/request.yaml")
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
	if got, want := workflow.ChangedFileHints, []string{"lib/profile.dart", "packages/app/lib/profile_state.dart"}; !slices.Equal(got, want) {
		t.Fatalf("unexpected changedFileHints: got %v want %v", got, want)
	}
	if got, want := workflow.InferredTestTargets, []string{"packages/app/test/profile_test.dart"}; !slices.Equal(got, want) {
		t.Fatalf("unexpected inferredTestTargets: got %v want %v", got, want)
	}
	if got, want := workflow.TerminationConditions, []string{
		"evaluator_decision == reject",
		"retries_exhausted == true",
		"evaluator_decision == pass",
	}; !slices.Equal(got, want) {
		t.Fatalf("unexpected termination conditions: got %v want %v", got, want)
	}

	executionPlan, err := readExecutionPlan(filepath.Join(artifactPath, "execution_plan.json"))
	if err != nil {
		t.Fatalf("failed to read execution plan: %v", err)
	}
	wantFormat := "dart format 'lib/profile.dart' 'packages/app/lib/profile_state.dart'"
	if executionPlan.FormatCommand != wantFormat {
		t.Fatalf("unexpected formatCommand: got %q want %q", executionPlan.FormatCommand, wantFormat)
	}
	wantAnalyze := []string{"cd '" + filepath.ToSlash(filepath.Join(projectRoot, "packages", "app")) + "' && flutter analyze . --fatal-infos"}
	if !slices.Equal(executionPlan.AnalyzeCommands, wantAnalyze) {
		t.Fatalf("unexpected analyzeCommands: got %v want %v", executionPlan.AnalyzeCommands, wantAnalyze)
	}
	wantTests := []string{"cd '" + filepath.ToSlash(filepath.Join(projectRoot, "packages", "app")) + "' && flutter test 'test/profile_test.dart'"}
	if !slices.Equal(executionPlan.TestCommands, wantTests) {
		t.Fatalf("unexpected testCommands: got %v want %v", executionPlan.TestCommands, wantTests)
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

	workflowSteps, err := os.ReadFile(filepath.Join(artifactPath, "workflow_steps.md"))
	if err != nil {
		t.Fatalf("failed to read workflow_steps.md: %v", err)
	}
	for _, fragment := range []string{
		"# Workflow Steps",
		"`lib/profile.dart`",
		"`packages/app/test/profile_test.dart`",
		"`evaluator_decision == pass`",
	} {
		if !strings.Contains(string(workflowSteps), fragment) {
			t.Fatalf("expected workflow_steps.md to contain %q, got:\n%s", fragment, string(workflowSteps))
		}
	}

	actorBrief, err := os.ReadFile(filepath.Join(artifactPath, "actor_briefs", "01_planner.md"))
	if err != nil {
		t.Fatalf("failed to read planner brief: %v", err)
	}
	for _, fragment := range []string{
		"# PLANNER Brief",
		"inputs/architecture_rules.md",
		"plan.yaml",
	} {
		if !strings.Contains(string(actorBrief), fragment) {
			t.Fatalf("expected planner brief to contain %q, got:\n%s", fragment, string(actorBrief))
		}
	}

	executionReport, err := os.ReadFile(filepath.Join(artifactPath, "execution_report.yaml"))
	if err != nil {
		t.Fatalf("failed to read execution_report.yaml: %v", err)
	}
	for _, fragment := range []string{
		"approved_memory_consideration:",
		"considered_ref: \"\"",
		"lookup_key: \"\"",
		"disposition: drop",
		"originating_candidate_refs: []",
	} {
		if !strings.Contains(string(executionReport), fragment) {
			t.Fatalf("expected execution_report placeholder to contain %q, got:\n%s", fragment, string(executionReport))
		}
	}
}

func TestBootstrapRejectsEscapingValidationInputs(t *testing.T) {
	projectRoot := t.TempDir()
	bootstrapper, err := NewBootstrapper(projectRoot)
	if err != nil {
		t.Fatalf("NewBootstrapper returned error: %v", err)
	}

	outsideRoot := t.TempDir()
	for _, relPath := range []string{
		filepath.Join(".harness", "requests"),
		filepath.Join("lib"),
	} {
		if err := os.MkdirAll(filepath.Join(projectRoot, relPath), 0o755); err != nil {
			t.Fatalf("failed to create %q: %v", relPath, err)
		}
	}

	requestPath := filepath.Join(projectRoot, ".harness", "requests", "request.yaml")
	requestBody := `task_type: bug_fix
goal: reject validation paths that escape the project root
context:
  suspected_files:
    - lib/profile.dart
  validation_roots:
    - ../outside
  validation_targets:
    - ` + filepath.ToSlash(filepath.Join(outsideRoot, "test", "evil_test.dart")) + `
constraints: []
definition_of_done:
  - reject unsafe validation paths
risk_tolerance: low
validation_profile: standard
`
	if err := os.WriteFile(requestPath, []byte(requestBody), 0o644); err != nil {
		t.Fatalf("failed to write request fixture: %v", err)
	}

	_, err = bootstrapper.Bootstrap(requestPath, "bootstrap-rejects-escaping-validation-inputs")
	if err == nil {
		t.Fatalf("expected Bootstrap to reject validation inputs outside %q", projectRoot)
	}
	if !strings.Contains(err.Error(), "project root") {
		t.Fatalf("expected project-root confinement error, got %v", err)
	}
}

type executionPlanJSON struct {
	FormatCommand   string   `json:"formatCommand"`
	AnalyzeCommands []string `json:"analyzeCommands"`
	TestCommands    []string `json:"testCommands"`
}

func readExecutionPlan(path string) (executionPlanJSON, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return executionPlanJSON{}, err
	}
	var plan executionPlanJSON
	if err := json.Unmarshal(data, &plan); err != nil {
		return executionPlanJSON{}, err
	}
	return plan, nil
}

func testRepoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("failed to resolve repo root: %v", err)
	}
	return root
}
