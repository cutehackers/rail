package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"rail/internal/contracts"
)

type Router struct {
	projectRoot string
	validator   *contracts.Validator
}

func NewRouter(projectRoot string) (*Router, error) {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve project root: %w", err)
	}
	validator, err := contracts.NewValidator(root)
	if err != nil {
		return nil, err
	}
	return &Router{projectRoot: root, validator: validator}, nil
}

func (r *Router) RouteEvaluation(artifactPath string) (string, error) {
	artifactDirectory, err := resolveArtifactDirectory(artifactPath)
	if err != nil {
		return "", err
	}

	workflow, err := readResolvedWorkflow(filepath.Join(artifactDirectory, "resolved_workflow.json"))
	if err != nil {
		return "", err
	}
	state, err := readState(filepath.Join(artifactDirectory, "state.json"))
	if err != nil {
		return "", err
	}
	if state.CurrentActor == nil || *state.CurrentActor != "evaluator" || shouldTerminate(state) {
		return fmt.Sprintf(
			"Harness evaluation routing skipped for %s (currentActor=%s, status=%s)",
			artifactDirectory,
			actorLabel(state.CurrentActor),
			state.Status,
		), nil
	}

	evaluationPath := filepath.Join(artifactDirectory, "evaluation_result.yaml")
	evaluationMap, err := r.validator.ValidateArtifactFile(evaluationPath, "evaluation_result")
	if err != nil {
		return "", err
	}

	nextState, err := advanceState(state, workflow, evaluationMap)
	if err != nil {
		return "", err
	}
	if err := writeJSON(filepath.Join(artifactDirectory, "state.json"), nextState); err != nil {
		return "", err
	}
	if err := appendSupervisorDecisionTrace(filepath.Join(artifactDirectory, "supervisor_trace.md"), nextState); err != nil {
		return "", err
	}
	if shouldTerminate(nextState) {
		if err := writeTerminalSummary(filepath.Join(artifactDirectory, "terminal_summary.md"), nextState); err != nil {
			return "", err
		}
	}

	return formatExecutionSummary(artifactDirectory, nextState, "Harness evaluation routed"), nil
}

func resolveArtifactDirectory(artifactPath string) (string, error) {
	if strings.TrimSpace(artifactPath) == "" {
		return "", fmt.Errorf("artifact path is required")
	}
	resolved, err := filepath.Abs(artifactPath)
	if err != nil {
		return "", fmt.Errorf("resolve artifact path: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("stat artifact path: %w", err)
	}
	if info.IsDir() {
		return resolved, nil
	}
	if filepath.Base(resolved) == "evaluation_result.yaml" {
		return filepath.Dir(resolved), nil
	}
	return "", fmt.Errorf("artifact path must be a directory or evaluation_result.yaml: %s", artifactPath)
}

func readResolvedWorkflow(path string) (ResolvedWorkflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ResolvedWorkflow{}, fmt.Errorf("read resolved workflow: %w", err)
	}
	var workflow ResolvedWorkflow
	if err := json.Unmarshal(data, &workflow); err != nil {
		return ResolvedWorkflow{}, fmt.Errorf("decode resolved workflow: %w", err)
	}
	return workflow, nil
}

func readState(path string) (State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return State{}, fmt.Errorf("read state: %w", err)
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, fmt.Errorf("decode state: %w", err)
	}
	if state.LastReasonCodes == nil {
		state.LastReasonCodes = []string{}
	}
	if state.ActionHistory == nil {
		state.ActionHistory = []string{}
	}
	if state.LastInterventionTriggerReasonCodes == nil {
		state.LastInterventionTriggerReasonCodes = []string{}
	}
	if state.CompletedActors == nil {
		state.CompletedActors = []string{}
	}
	return state, nil
}

func advanceState(state State, workflow ResolvedWorkflow, evaluation map[string]any) (State, error) {
	nextState := state
	nextState.CompletedActors = append(append([]string{}, state.CompletedActors...), "evaluator")

	decision, err := readString(evaluation, "decision")
	if err != nil {
		return State{}, err
	}
	reasonCodes, err := readOptionalStringList(evaluation, "reason_codes")
	if err != nil {
		return State{}, err
	}
	nextAction, err := readOptionalString(evaluation, "next_action")
	if err != nil {
		return State{}, err
	}
	reasonCategory := primaryReasonCategory(reasonCodes)

	nextState.LastDecision = &decision
	nextState.LastReasonCodes = reasonCodes

	switch decision {
	case "pass":
		nextState.Status = "passed"
		nextState.CurrentActor = nil
		nextState.ActionHistory = append(nextState.ActionHistory, "pass")
		return nextState, nil
	case "reject":
		nextState.Status = "rejected"
		nextState.CurrentActor = nil
		nextState.ActionHistory = append(nextState.ActionHistory, "reject")
		return nextState, nil
	}

	action := routeFromEvaluationResult(decision, reasonCodes, nextAction)
	if action == "" {
		return State{}, fmt.Errorf("evaluation_result.yaml revise decision requires either a supported `next_action` or routeable `reason_codes`")
	}

	nextState.LastInterventionTriggerReasonCodes = append([]string{}, reasonCodes...)
	nextState.LastInterventionTriggerCategory = stringPtr(reasonCategory)

	switch action {
	case "rebuild_context":
		nextState.ContextRebuildsRemaining--
		if nextState.ContextRebuildsRemaining < 0 || !contains(workflow.Actors, "context_builder") {
			nextState.Status = "evolution_exhausted"
			nextState.CurrentActor = nil
			return nextState, nil
		}
		nextState.Status = "rebuilding_context"
		nextState.CurrentActor = stringPtr("context_builder")
		nextState.ActionHistory = append(nextState.ActionHistory, action)
		nextState.PendingContextRefreshTrigger = stringPtr(contextRefreshTrigger(reasonCodes, nextAction))
		nextState.PendingContextRefreshReasonFamily = stringPtr(reasonCategory)
	case "tighten_validation":
		nextState.ValidationTighteningsRemaining--
		if nextState.ValidationTighteningsRemaining < 0 || !contains(workflow.Actors, "executor") {
			nextState.Status = "evolution_exhausted"
			nextState.CurrentActor = nil
			return nextState, nil
		}
		nextState.Status = "tightening_validation"
		nextState.CurrentActor = stringPtr("executor")
		nextState.ActionHistory = append(nextState.ActionHistory, action)
	case "split_task":
		nextState.Status = "split_required"
		nextState.CurrentActor = nil
		nextState.ActionHistory = append(nextState.ActionHistory, action)
	case "block_environment":
		nextState.Status = "blocked_environment"
		nextState.CurrentActor = nil
		nextState.ActionHistory = append(nextState.ActionHistory, action)
	default:
		nextState.GeneratorRetriesRemaining--
		if nextState.GeneratorRetriesRemaining < 0 || !contains(workflow.Actors, "generator") {
			nextState.Status = "revise_exhausted"
			nextState.CurrentActor = nil
			return nextState, nil
		}
		nextState.Status = "revising"
		nextState.CurrentActor = stringPtr("generator")
		nextState.ActionHistory = append(nextState.ActionHistory, "revise_generator")
	}

	return nextState, nil
}

func shouldTerminate(state State) bool {
	return state.CurrentActor == nil ||
		state.Status == "passed" ||
		state.Status == "rejected" ||
		state.Status == "revise_exhausted" ||
		state.Status == "evolution_exhausted" ||
		state.Status == "blocked_environment" ||
		state.Status == "split_required"
}

func routeFromEvaluationResult(decision string, reasonCodes []string, nextAction *string) string {
	if decision == "pass" {
		return "pass"
	}
	if hasEnvironmentFailure(reasonCodes) {
		return "block_environment"
	}
	if hasScopeFailure(reasonCodes) {
		return "split_task"
	}
	if hasContextFailure(reasonCodes) {
		return "rebuild_context"
	}
	if hasValidationScopeFailure(reasonCodes) {
		return "tighten_validation"
	}
	if hasValidationEvidenceFailure(reasonCodes) ||
		hasValidationRequirementFailure(reasonCodes) ||
		hasRequirementsCoverageFailure(reasonCodes) ||
		hasRequirementsBehaviorFailure(reasonCodes) ||
		hasValidationFailure(reasonCodes) ||
		hasRequirementsFailure(reasonCodes) ||
		hasImplementationFailure(reasonCodes) ||
		hasArchitectureFailure(reasonCodes) {
		return "revise_generator"
	}
	if nextAction == nil {
		return ""
	}
	return *nextAction
}

func hasEnvironmentFailure(reasonCodes []string) bool {
	for _, code := range reasonCodes {
		if strings.HasPrefix(code, "environment_") ||
			strings.Contains(code, "permission_error") ||
			strings.Contains(code, "sandbox") ||
			strings.Contains(code, "tooling_unavailable") ||
			strings.Contains(code, "sdk_cache") {
			return true
		}
	}
	return false
}

func hasScopeFailure(reasonCodes []string) bool {
	for _, code := range reasonCodes {
		if strings.HasPrefix(code, "scope_") {
			return true
		}
	}
	return false
}

func hasContextFailure(reasonCodes []string) bool {
	for _, code := range reasonCodes {
		if strings.HasPrefix(code, "context_") {
			return true
		}
	}
	return false
}

func hasValidationScopeFailure(reasonCodes []string) bool {
	for _, code := range reasonCodes {
		if strings.HasPrefix(code, "validation_scope_") ||
			strings.HasPrefix(code, "validation_target_") ||
			strings.HasPrefix(code, "validation_mismatch_") {
			return true
		}
	}
	return false
}

func hasValidationEvidenceFailure(reasonCodes []string) bool {
	return hasPrefixedReason(reasonCodes, "validation_evidence_")
}

func hasValidationRequirementFailure(reasonCodes []string) bool {
	return hasPrefixedReason(reasonCodes, "validation_requirement_")
}

func hasRequirementsCoverageFailure(reasonCodes []string) bool {
	return hasPrefixedReason(reasonCodes, "requirements_coverage_")
}

func hasRequirementsBehaviorFailure(reasonCodes []string) bool {
	return hasPrefixedReason(reasonCodes, "requirements_behavior_")
}

func hasValidationFailure(reasonCodes []string) bool {
	for _, code := range reasonCodes {
		if strings.HasPrefix(code, "validation_") &&
			!strings.HasPrefix(code, "validation_scope_") &&
			!strings.HasPrefix(code, "validation_target_") &&
			!strings.HasPrefix(code, "validation_mismatch_") &&
			!strings.HasPrefix(code, "validation_evidence_") &&
			!strings.HasPrefix(code, "validation_requirement_") {
			return true
		}
	}
	return false
}

func hasRequirementsFailure(reasonCodes []string) bool {
	for _, code := range reasonCodes {
		if strings.HasPrefix(code, "requirements_") &&
			!strings.HasPrefix(code, "requirements_coverage_") &&
			!strings.HasPrefix(code, "requirements_behavior_") {
			return true
		}
	}
	return false
}

func hasImplementationFailure(reasonCodes []string) bool {
	return hasPrefixedReason(reasonCodes, "implementation_")
}

func hasArchitectureFailure(reasonCodes []string) bool {
	return hasPrefixedReason(reasonCodes, "architecture_")
}

func hasPrefixedReason(reasonCodes []string, prefix string) bool {
	for _, code := range reasonCodes {
		if strings.HasPrefix(code, prefix) {
			return true
		}
	}
	return false
}

func contextRefreshTrigger(reasonCodes []string, nextAction *string) string {
	if hasContextFailure(reasonCodes) {
		return "reason_codes"
	}
	if nextAction != nil && *nextAction == "rebuild_context" {
		return "next_action"
	}
	return "unknown"
}

func primaryReasonCategory(reasonCodes []string) string {
	for _, code := range reasonCodes {
		switch {
		case strings.HasPrefix(code, "environment_"),
			strings.Contains(code, "permission_error"),
			strings.Contains(code, "sandbox"),
			strings.Contains(code, "tooling_unavailable"),
			strings.Contains(code, "sdk_cache"):
			return "environment"
		case strings.HasPrefix(code, "validation_"):
			return "validation"
		case strings.HasPrefix(code, "requirements_"):
			return "requirements"
		case strings.HasPrefix(code, "context_"):
			return "context"
		case strings.HasPrefix(code, "implementation_"):
			return "implementation"
		case strings.HasPrefix(code, "scope_"):
			return "scope"
		case strings.HasPrefix(code, "architecture_"):
			return "architecture"
		}
	}
	if len(reasonCodes) == 0 {
		return "none"
	}
	return "mixed"
}

func formatExecutionSummary(artifactDirectory string, state State, prefix string) string {
	action := "none"
	if len(state.ActionHistory) > 0 {
		action = state.ActionHistory[len(state.ActionHistory)-1]
	}
	reasons := "none"
	if len(state.LastReasonCodes) > 0 {
		reasons = strings.Join(state.LastReasonCodes, ", ")
	}
	outcome := "updated"
	switch state.Status {
	case "passed":
		outcome = "passed cleanly"
	case "awaiting_integrator":
		outcome = "awaiting explicit post-pass integration"
	case "blocked_environment":
		outcome = "blocked by environment"
	case "split_required":
		outcome = "requires task split"
	case "evolution_exhausted":
		outcome = "stopped after exhausted evolution budget"
	case "revise_exhausted":
		outcome = "stopped after exhausted revision budget"
	case "rejected":
		outcome = "rejected by evaluator"
	}
	return fmt.Sprintf(
		"%s at %s (%s, status=%s, currentActor=%s, action=%s, reasons=%s)",
		prefix,
		artifactDirectory,
		outcome,
		state.Status,
		actorLabel(state.CurrentActor),
		action,
		reasons,
	)
}

func appendSupervisorDecisionTrace(path string, state State) error {
	var builder strings.Builder
	if _, err := os.Stat(path); os.IsNotExist(err) {
		builder.WriteString("# Supervisor Decision Trace\n\n")
	}
	builder.WriteString(fmt.Sprintf("status=%s\n", state.Status))
	builder.WriteString(fmt.Sprintf("action=%s\n", lastAction(state.ActionHistory)))
	builder.WriteString(fmt.Sprintf("reason_codes=%s\n", strings.Join(state.LastReasonCodes, ", ")))
	builder.WriteString(fmt.Sprintf("selected_action=%s\n", lastAction(state.ActionHistory)))
	builder.WriteString("\n")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open supervisor trace: %w", err)
	}
	defer file.Close()
	if _, err := file.WriteString(builder.String()); err != nil {
		return fmt.Errorf("write supervisor trace: %w", err)
	}
	return nil
}

func writeTerminalSummary(path string, state State) error {
	var summary string
	switch state.Status {
	case "split_required":
		summary = "The request should be decomposed before continuing."
	case "blocked_environment":
		summary = "The workflow is blocked by environment issues."
	default:
		summary = fmt.Sprintf("Terminal status: %s", state.Status)
	}
	return os.WriteFile(path, []byte(summary+"\n"), 0o644)
}

func readOptionalString(source map[string]any, key string) (*string, error) {
	value, ok := source[key]
	if !ok || value == nil {
		return nil, nil
	}
	text, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("expected `%s` to be a string", key)
	}
	return &text, nil
}

func readOptionalStringList(source map[string]any, key string) ([]string, error) {
	value, ok := source[key]
	if !ok || value == nil {
		return []string{}, nil
	}
	list, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("expected `%s` to be a list", key)
	}
	result := make([]string, 0, len(list))
	for _, entry := range list {
		text, ok := entry.(string)
		if !ok {
			return nil, fmt.Errorf("expected `%s` entries to be strings", key)
		}
		result = append(result, text)
	}
	return result, nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func stringPtr(value string) *string {
	return &value
}

func actorLabel(actor *string) string {
	if actor == nil {
		return "none"
	}
	return *actor
}

func lastAction(actions []string) string {
	if len(actions) == 0 {
		return "none"
	}
	return actions[len(actions)-1]
}
