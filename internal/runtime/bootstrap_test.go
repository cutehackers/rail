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
	repoRoot := testRepoRoot(t)
	projectRoot := t.TempDir()
	bootstrapper, err := NewBootstrapper(projectRoot)
	if err != nil {
		t.Fatalf("NewBootstrapper returned error: %v", err)
	}

	for _, relPath := range []string{
		filepath.Join(".harness", "requests"),
		filepath.Join("internal", "runtime"),
		filepath.Join("cmd", "rail"),
	} {
		if err := os.MkdirAll(filepath.Join(projectRoot, relPath), 0o755); err != nil {
			t.Fatalf("failed to create %q: %v", relPath, err)
		}
	}

	for _, relPath := range []string{
		filepath.Join(".harness", "actors", "critic.md"),
		filepath.Join(".harness", "actors", "generator.md"),
		filepath.Join(".harness", "supervisor", "actor_profiles.yaml"),
		filepath.Join(".harness", "supervisor", "task_router.yaml"),
		filepath.Join(".harness", "supervisor", "registry.yaml"),
		filepath.Join(".harness", "supervisor", "context_contract.yaml"),
		filepath.Join(".harness", "templates", "critic_report.schema.yaml"),
	} {
		sourcePath := filepath.Join(repoRoot, relPath)
		body, err := os.ReadFile(sourcePath)
		if err != nil {
			t.Fatalf("failed to read checked-in contract %q: %v", relPath, err)
		}
		destinationPath := filepath.Join(projectRoot, relPath)
		if err := os.MkdirAll(filepath.Dir(destinationPath), 0o755); err != nil {
			t.Fatalf("failed to create contract directory for %q: %v", relPath, err)
		}
		if err := os.WriteFile(destinationPath, body, 0o644); err != nil {
			t.Fatalf("failed to write contract %q: %v", relPath, err)
		}
	}

	requestPath := filepath.Join(projectRoot, ".harness", "requests", "request.yaml")
	requestBody := `task_type: bug_fix
goal: tighten the validation plan for profile changes
context:
  feature: runtime
  suspected_files:
    - ` + filepath.ToSlash(filepath.Join(projectRoot, "internal", "runtime", "runner.go")) + `
  related_files:
    - cmd/rail/main.go
  validation_roots:
    - ` + filepath.ToSlash(filepath.Join(projectRoot, "internal")) + `
  validation_targets:
    - ` + filepath.ToSlash(filepath.Join(projectRoot, "internal", "runtime", "runner_test.go")) + `
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
		workflowArtifactFileName,
		"state.json",
		"execution_plan.json",
		"workflow_steps.md",
		"plan.yaml",
		"context_pack.yaml",
		"critic_report.yaml",
		"implementation_result.yaml",
		"execution_report.yaml",
		"evaluation_result.yaml",
		filepath.Join("inputs", "architecture_rules.md"),
		filepath.Join("inputs", "project_rules.md"),
		filepath.Join("inputs", "forbidden_changes.md"),
		filepath.Join("inputs", "execution_policy.yaml"),
		filepath.Join("inputs", "rubric.yaml"),
		filepath.Join("actor_briefs", "01_planner.md"),
		filepath.Join("actor_briefs", "02_context_builder.md"),
		filepath.Join("actor_briefs", "03_critic.md"),
		filepath.Join("actor_briefs", "04_generator.md"),
		filepath.Join("actor_briefs", "05_executor.md"),
		filepath.Join("actor_briefs", "06_evaluator.md"),
	} {
		if _, err := os.Stat(filepath.Join(artifactPath, relPath)); err != nil {
			t.Fatalf("expected bootstrap artifact %q to exist: %v", relPath, err)
		}
	}

	validator, err := contracts.NewValidator(projectRoot)
	if err != nil {
		t.Fatalf("NewValidator returned error: %v", err)
	}
	criticReport, err := validator.ValidateArtifactFile(filepath.Join(".harness", "artifacts", "bootstrap-smoke", "critic_report.yaml"), "critic_report")
	if err != nil {
		t.Fatalf("ValidateArtifactFile returned error for critic_report placeholder: %v", err)
	}
	wantCriticPlaceholder := map[string]any{
		"priority_focus":          []any{},
		"missing_requirements":    []any{},
		"risk_hypotheses":         []any{},
		"validation_expectations": []any{},
		"generator_guardrails":    []any{},
		"blocked_assumptions":     []any{},
	}
	if !mapsEqual(criticReport, wantCriticPlaceholder) {
		t.Fatalf("unexpected critic_report placeholder: got %#v want %#v", criticReport, wantCriticPlaceholder)
	}

	workflowPath := filepath.Join(artifactPath, workflowArtifactFileName)
	workflow, err := readWorkflow(workflowPath)
	if err != nil {
		t.Fatalf("failed to read workflow: %v", err)
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
	if got, want := workflow.ChangedFileHints, []string{"cmd/rail/main.go", "internal/runtime/runner.go"}; !slices.Equal(got, want) {
		t.Fatalf("unexpected changedFileHints: got %v want %v", got, want)
	}
	if got, want := workflow.InferredTestTargets, []string{"internal/runtime/runner_test.go"}; !slices.Equal(got, want) {
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
	wantFormat := "gofmt -w 'cmd/rail/main.go' 'internal/runtime/runner.go'"
	if executionPlan.FormatCommand != wantFormat {
		t.Fatalf("unexpected formatCommand: got %q want %q", executionPlan.FormatCommand, wantFormat)
	}
	wantAnalyze := []string{"cd '" + filepath.ToSlash(filepath.Join(projectRoot, "internal")) + "' && go build ./..."}
	if !slices.Equal(executionPlan.AnalyzeCommands, wantAnalyze) {
		t.Fatalf("unexpected analyzeCommands: got %v want %v", executionPlan.AnalyzeCommands, wantAnalyze)
	}
	wantTests := []string{"cd '" + filepath.ToSlash(filepath.Join(projectRoot, "internal", "runtime")) + "' && go test ./..."}
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
	if len(state.ActorProfilesUsed) != 0 {
		t.Fatalf("expected initial actorProfilesUsed snapshot to be empty, got %#v", state.ActorProfilesUsed)
	}
	if state.CurrentActor == nil || *state.CurrentActor != "planner" {
		t.Fatalf("unexpected currentActor: got %v want %q", state.CurrentActor, "planner")
	}
	if got, want := workflow.Actors, []string{"planner", "context_builder", "critic", "generator", "executor", "evaluator"}; !slices.Equal(got, want) {
		t.Fatalf("unexpected actors: got %v want %v", got, want)
	}

	workflowSteps, err := os.ReadFile(filepath.Join(artifactPath, "workflow_steps.md"))
	if err != nil {
		t.Fatalf("failed to read workflow_steps.md: %v", err)
	}
	for _, fragment := range []string{
		"# Workflow Steps",
		"`internal/runtime/runner.go`",
		"`internal/runtime/runner_test.go`",
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

	criticBrief, err := os.ReadFile(filepath.Join(artifactPath, "actor_briefs", "03_critic.md"))
	if err != nil {
		t.Fatalf("failed to read critic brief: %v", err)
	}
	for _, fragment := range []string{
		"priority_focus",
		"missing_requirements",
		"risk_hypotheses",
		"validation_expectations",
		"generator_guardrails",
		"blocked_assumptions",
	} {
		if !strings.Contains(string(criticBrief), fragment) {
			t.Fatalf("expected critic brief to contain %q, got:\n%s", fragment, string(criticBrief))
		}
	}

	generatorBrief, err := os.ReadFile(filepath.Join(artifactPath, "actor_briefs", "04_generator.md"))
	if err != nil {
		t.Fatalf("failed to read generator brief: %v", err)
	}
	for _, fragment := range []string{
		"critic_report.yaml",
		"forbidden_changes",
		"constraints",
		"Treat all inputs above as required",
	} {
		if !strings.Contains(string(generatorBrief), fragment) {
			t.Fatalf("expected generator brief to contain %q, got:\n%s", fragment, string(generatorBrief))
		}
	}
}

func TestBootstrapCreatesExpectedArtifactSkeletonWithEmbeddedDefaults(t *testing.T) {
	projectRoot := t.TempDir()
	bootstrapper, err := NewBootstrapper(projectRoot)
	if err != nil {
		t.Fatalf("NewBootstrapper returned error: %v", err)
	}

	for _, relPath := range []string{
		filepath.Join(".harness", "requests"),
		filepath.Join("internal", "runtime"),
		filepath.Join("cmd", "rail"),
	} {
		if err := os.MkdirAll(filepath.Join(projectRoot, relPath), 0o755); err != nil {
			t.Fatalf("failed to create %q: %v", relPath, err)
		}
	}

	requestPath := filepath.Join(projectRoot, ".harness", "requests", "request.yaml")
	requestBody := `task_type: feature_addition
goal: verify embedded defaults still bootstrap the critic graph
context:
  feature: runtime
  suspected_files:
    - internal/runtime/bootstrap.go
  related_files:
    - cmd/rail/main.go
  validation_roots:
    - internal
  validation_targets:
    - internal/runtime/bootstrap_test.go
constraints: []
definition_of_done:
  - bootstrap should use embedded defaults when repo-owned contract files are absent
risk_tolerance: low
validation_profile: standard
`
	if err := os.WriteFile(requestPath, []byte(requestBody), 0o644); err != nil {
		t.Fatalf("failed to write request fixture: %v", err)
	}

	artifactPath, err := bootstrapper.Bootstrap(requestPath, "bootstrap-defaults")
	if err != nil {
		t.Fatalf("Bootstrap returned error: %v", err)
	}

	for _, relPath := range []string{
		"critic_report.yaml",
		filepath.Join("actor_briefs", "03_critic.md"),
		filepath.Join("actor_briefs", "04_generator.md"),
		filepath.Join("actor_briefs", "06_evaluator.md"),
	} {
		if _, err := os.Stat(filepath.Join(artifactPath, relPath)); err != nil {
			t.Fatalf("expected embedded-default bootstrap artifact %q to exist: %v", relPath, err)
		}
	}

	generatorBrief, err := os.ReadFile(filepath.Join(artifactPath, "actor_briefs", "04_generator.md"))
	if err != nil {
		t.Fatalf("failed to read generator brief: %v", err)
	}
	for _, fragment := range []string{
		"critic_report.yaml",
		"forbidden_changes",
	} {
		if !strings.Contains(string(generatorBrief), fragment) {
			t.Fatalf("expected embedded-default generator brief to contain %q, got:\n%s", fragment, string(generatorBrief))
		}
	}

	workflowPath := filepath.Join(artifactPath, workflowArtifactFileName)
	workflow, err := readWorkflow(workflowPath)
	if err != nil {
		t.Fatalf("failed to read embedded-default workflow: %v", err)
	}
	if got, want := workflow.Actors, []string{"planner", "context_builder", "critic", "generator", "executor", "evaluator"}; !slices.Equal(got, want) {
		t.Fatalf("unexpected embedded-default actors: got %v want %v", got, want)
	}

	validator, err := contracts.NewValidator(projectRoot)
	if err != nil {
		t.Fatalf("NewValidator returned error: %v", err)
	}
	if _, err := validator.ValidateArtifactFile(filepath.Join(".harness", "artifacts", "bootstrap-defaults", "critic_report.yaml"), "critic_report"); err != nil {
		t.Fatalf("ValidateArtifactFile returned error for embedded-default critic_report placeholder: %v", err)
	}
}

func TestBootstrapNormalizesCanonicalPathsWithinSymlinkedRoot(t *testing.T) {
	projectRoot := t.TempDir()
	symlinkParent := t.TempDir()
	symlinkRoot := filepath.Join(symlinkParent, "workspace")
	if err := os.Symlink(projectRoot, symlinkRoot); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	bootstrapper, err := NewBootstrapper(symlinkRoot)
	if err != nil {
		t.Fatalf("NewBootstrapper returned error: %v", err)
	}

	for _, relPath := range []string{
		filepath.Join(".harness", "requests"),
		filepath.Join("internal", "runtime"),
		filepath.Join("cmd", "rail"),
	} {
		if err := os.MkdirAll(filepath.Join(projectRoot, relPath), 0o755); err != nil {
			t.Fatalf("failed to create %q: %v", relPath, err)
		}
	}

	requestPath := filepath.Join(projectRoot, ".harness", "requests", "request.yaml")
	requestBody := `task_type: bug_fix
goal: normalize canonical inputs in a symlinked checkout
context:
  feature: runtime
  suspected_files:
    - ` + filepath.ToSlash(filepath.Join(projectRoot, "internal", "runtime", "runner.go")) + `
  related_files:
    - ` + filepath.ToSlash(filepath.Join(projectRoot, "cmd", "rail", "main.go")) + `
  validation_roots:
    - ` + filepath.ToSlash(filepath.Join(projectRoot, "internal")) + `
  validation_targets:
    - ` + filepath.ToSlash(filepath.Join(projectRoot, "internal", "runtime", "runner_test.go")) + `
constraints: []
definition_of_done:
  - keep normalized paths inside the repo
risk_tolerance: low
validation_profile: standard
`
	if err := os.WriteFile(requestPath, []byte(requestBody), 0o644); err != nil {
		t.Fatalf("failed to write request fixture: %v", err)
	}

	artifactPath, err := bootstrapper.Bootstrap(requestPath, "bootstrap-symlink-root")
	if err != nil {
		t.Fatalf("Bootstrap returned error: %v", err)
	}

	workflowPath := filepath.Join(artifactPath, workflowArtifactFileName)
	workflow, err := readWorkflow(workflowPath)
	if err != nil {
		t.Fatalf("failed to read workflow: %v", err)
	}
	if workflow.RequestPath != ".harness/requests/request.yaml" {
		t.Fatalf("unexpected requestPath: got %q want %q", workflow.RequestPath, ".harness/requests/request.yaml")
	}
	if got, want := workflow.ChangedFileHints, []string{"cmd/rail/main.go", "internal/runtime/runner.go"}; !slices.Equal(got, want) {
		t.Fatalf("unexpected changedFileHints: got %v want %v", got, want)
	}
	if got, want := workflow.InferredTestTargets, []string{"internal/runtime/runner_test.go"}; !slices.Equal(got, want) {
		t.Fatalf("unexpected inferredTestTargets: got %v want %v", got, want)
	}
	for _, got := range append(append([]string{}, workflow.ChangedFileHints...), workflow.InferredTestTargets...) {
		if strings.HasPrefix(got, "..") {
			t.Fatalf("expected in-repo relative path, got %q", got)
		}
	}

	executionPlan, err := readExecutionPlan(filepath.Join(artifactPath, "execution_plan.json"))
	if err != nil {
		t.Fatalf("failed to read execution plan: %v", err)
	}
	wantAnalyze := []string{"cd '" + filepath.ToSlash(filepath.Join(symlinkRoot, "internal")) + "' && go build ./..."}
	if !slices.Equal(executionPlan.AnalyzeCommands, wantAnalyze) {
		t.Fatalf("unexpected analyzeCommands: got %v want %v", executionPlan.AnalyzeCommands, wantAnalyze)
	}
	wantTests := []string{"cd '" + filepath.ToSlash(filepath.Join(symlinkRoot, "internal", "runtime")) + "' && go test ./..."}
	if !slices.Equal(executionPlan.TestCommands, wantTests) {
		t.Fatalf("unexpected testCommands: got %v want %v", executionPlan.TestCommands, wantTests)
	}
}

func mapsEqual(left, right map[string]any) bool {
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && string(leftJSON) == string(rightJSON)
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
		filepath.Join("internal", "runtime"),
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
    - internal/runtime/runner.go
  validation_roots:
    - ../outside
  validation_targets:
    - ` + filepath.ToSlash(filepath.Join(outsideRoot, "internal", "evil_test.go")) + `
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

func TestBootstrapRejectsValidationRootsThatAreFiles(t *testing.T) {
	projectRoot := t.TempDir()
	bootstrapper, err := NewBootstrapper(projectRoot)
	if err != nil {
		t.Fatalf("NewBootstrapper returned error: %v", err)
	}

	for _, relPath := range []string{
		filepath.Join(".harness", "requests"),
		filepath.Join("internal", "runtime"),
	} {
		if err := os.MkdirAll(filepath.Join(projectRoot, relPath), 0o755); err != nil {
			t.Fatalf("failed to create %q: %v", relPath, err)
		}
	}

	fileRoot := filepath.Join(projectRoot, "internal", "runtime", "runner.go")
	if err := os.WriteFile(fileRoot, []byte("package runtime\n"), 0o644); err != nil {
		t.Fatalf("failed to write file-based validation root: %v", err)
	}

	requestPath := filepath.Join(projectRoot, ".harness", "requests", "request.yaml")
	requestBody := `task_type: bug_fix
goal: reject file paths in validation_roots
context:
  suspected_files:
    - internal/runtime/runner.go
  validation_roots:
    - ` + filepath.ToSlash(fileRoot) + `
  validation_targets:
    - ` + filepath.ToSlash(filepath.Join(projectRoot, "internal", "runtime", "runner_test.go")) + `
constraints: []
definition_of_done:
  - reject file validation roots
risk_tolerance: low
validation_profile: standard
`
	if err := os.WriteFile(requestPath, []byte(requestBody), 0o644); err != nil {
		t.Fatalf("failed to write request fixture: %v", err)
	}

	_, err = bootstrapper.Bootstrap(requestPath, "bootstrap-rejects-file-validation-root")
	if err == nil {
		t.Fatalf("expected Bootstrap to reject file-based validation_roots")
	}
	if !strings.Contains(err.Error(), "context.validation_roots") {
		t.Fatalf("expected validation_roots error context, got %v", err)
	}
	if !strings.Contains(err.Error(), "directory") {
		t.Fatalf("expected directory validation error, got %v", err)
	}
}

func TestBootstrapRejectsValidationTargetsThatAreDirectories(t *testing.T) {
	projectRoot := t.TempDir()
	bootstrapper, err := NewBootstrapper(projectRoot)
	if err != nil {
		t.Fatalf("NewBootstrapper returned error: %v", err)
	}

	for _, relPath := range []string{
		filepath.Join(".harness", "requests"),
		filepath.Join("internal", "runtime"),
	} {
		if err := os.MkdirAll(filepath.Join(projectRoot, relPath), 0o755); err != nil {
			t.Fatalf("failed to create %q: %v", relPath, err)
		}
	}

	directoryTarget := filepath.Join(projectRoot, "internal", "runtime")
	requestPath := filepath.Join(projectRoot, ".harness", "requests", "request.yaml")
	requestBody := `task_type: bug_fix
goal: reject directory paths in validation_targets
context:
  suspected_files:
    - internal/runtime/runner.go
  validation_roots:
    - ` + filepath.ToSlash(filepath.Join(projectRoot, "internal")) + `
  validation_targets:
    - ` + filepath.ToSlash(directoryTarget) + `
constraints: []
definition_of_done:
  - reject directory validation targets
risk_tolerance: low
validation_profile: standard
`
	if err := os.WriteFile(requestPath, []byte(requestBody), 0o644); err != nil {
		t.Fatalf("failed to write request fixture: %v", err)
	}

	_, err = bootstrapper.Bootstrap(requestPath, "bootstrap-rejects-directory-validation-target")
	if err == nil {
		t.Fatalf("expected Bootstrap to reject directory-based validation_targets")
	}
	if !strings.Contains(err.Error(), "context.validation_targets") {
		t.Fatalf("expected validation_targets error context, got %v", err)
	}
	if !strings.Contains(err.Error(), "file") {
		t.Fatalf("expected file validation error, got %v", err)
	}
}

func TestBootstrapReturnsErrorForMalformedSupervisorConfig(t *testing.T) {
	projectRoot := t.TempDir()
	bootstrapper, err := NewBootstrapper(projectRoot)
	if err != nil {
		t.Fatalf("NewBootstrapper returned error: %v", err)
	}

	for _, relPath := range []string{
		filepath.Join(".harness", "requests"),
		filepath.Join(".harness", "supervisor"),
		filepath.Join("internal", "runtime"),
	} {
		if err := os.MkdirAll(filepath.Join(projectRoot, relPath), 0o755); err != nil {
			t.Fatalf("failed to create %q: %v", relPath, err)
		}
	}

	requestPath := filepath.Join(projectRoot, ".harness", "requests", "request.yaml")
	requestBody := `task_type: bug_fix
goal: surface malformed supervisor config as a normal error
context:
  suspected_files:
    - internal/runtime/runner.go
constraints: []
definition_of_done:
  - report config errors without crashing
risk_tolerance: low
validation_profile: standard
`
	if err := os.WriteFile(requestPath, []byte(requestBody), 0o644); err != nil {
		t.Fatalf("failed to write request fixture: %v", err)
	}

	malformedExecutionPolicy := `artifacts:
  root: 123
format:
  command: gofmt -w {files}
analyze:
  package_command: go build ./...
  workspace_fallback: go build ./...
  smoke_command: go build ./...
tests:
  package_command: go test ./...
  workspace_fallback: go test ./...
  smoke_command: go test ./...
runtime:
  create_placeholders: true
  create_actor_briefs: true
  persist_json_snapshots: true
`
	if err := os.WriteFile(
		filepath.Join(projectRoot, ".harness", "supervisor", "execution_policy.yaml"),
		[]byte(malformedExecutionPolicy),
		0o644,
	); err != nil {
		t.Fatalf("failed to write malformed execution_policy.yaml: %v", err)
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("Bootstrap panicked for malformed supervisor config: %v", recovered)
		}
	}()

	_, err = bootstrapper.Bootstrap(requestPath, "bootstrap-malformed-supervisor-config")
	if err == nil {
		t.Fatalf("expected Bootstrap to return an error for malformed supervisor config")
	}
	if !strings.Contains(err.Error(), "execution policy") || !strings.Contains(err.Error(), "root") {
		t.Fatalf("expected execution policy error mentioning root, got %v", err)
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
