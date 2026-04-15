package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"rail/internal/contracts"

	"gopkg.in/yaml.v3"
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
	artifactDirectory, err := r.resolveArtifactDirectory(artifactPath)
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
	if err := appendSupervisorDecisionTrace(artifactDirectory, nextState); err != nil {
		return "", err
	}
	if shouldTerminate(nextState) {
		if err := r.writeTerminalSummary(artifactDirectory, nextState); err != nil {
			return "", err
		}
	}

	return formatExecutionSummary(artifactDirectory, nextState, "Harness evaluation routed"), nil
}

func (r *Router) resolveArtifactDirectory(artifactPath string) (string, error) {
	if strings.TrimSpace(artifactPath) == "" {
		return "", fmt.Errorf("artifact path is required")
	}
	resolved, err := contracts.ResolvePathWithinRoot(r.projectRoot, artifactPath)
	if err != nil {
		return "", err
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
		pendingTrigger := state.PendingContextRefreshTrigger
		if pendingTrigger == nil {
			pendingTrigger = stringPtr(contextRefreshTrigger(reasonCodes, nextAction))
		}
		pendingReasonFamily := state.PendingContextRefreshReasonFamily
		if pendingReasonFamily == nil {
			pendingReasonFamily = stringPtr(reasonCategory)
		}
		nextState.Status = "rebuilding_context"
		nextState.CurrentActor = stringPtr("context_builder")
		nextState.ActionHistory = append(nextState.ActionHistory, action)
		nextState.PendingContextRefreshTrigger = pendingTrigger
		nextState.PendingContextRefreshReasonFamily = pendingReasonFamily
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

func appendSupervisorDecisionTrace(artifactDirectory string, state State) error {
	tracePath := filepath.Join(artifactDirectory, "supervisor_trace.md")
	evaluationPath := filepath.Join(artifactDirectory, "evaluation_result.yaml")
	evaluationMap := map[string]any{}
	if _, err := os.Stat(evaluationPath); err == nil {
		value, readErr := readYAMLMap(evaluationPath)
		if readErr != nil {
			return readErr
		}
		evaluationMap = value
	}
	decision := readOptionalStringValue(evaluationMap, "decision", derefString(state.LastDecision, ""))
	qualityConfidence := readOptionalStringValue(evaluationMap, "quality_confidence", "unknown")
	action := lastAction(state.ActionHistory)
	iteration := countActor(state.CompletedActors, "evaluator")
	category := primaryReasonCategory(state.LastReasonCodes)
	triggerCategory := category
	if state.LastInterventionTriggerCategory != nil {
		triggerCategory = *state.LastInterventionTriggerCategory
	}
	triggerReasonCodes := state.LastInterventionTriggerReasonCodes
	if len(triggerReasonCodes) == 0 {
		triggerReasonCodes = state.LastReasonCodes
	}
	executedInterventionCount := executedInterventionCount(state)

	var builder strings.Builder
	if _, err := os.Stat(tracePath); os.IsNotExist(err) {
		builder.WriteString("# Supervisor Decision Trace\n\n")
		builder.WriteString("Reason code taxonomy:\n")
		builder.WriteString("- `environment_*`: tooling, sandbox, SDK cache, permissions, or external setup failures\n")
		builder.WriteString("- `validation_scope_*` / `validation_target_*` / `validation_mismatch_*`: validation scope or target selection problems\n")
		builder.WriteString("- `validation_evidence_*`: validation evidence is missing, incomplete, or weak\n")
		builder.WriteString("- `validation_requirement_*`: validation exposed a concrete unmet requirement\n")
		builder.WriteString("- `requirements_coverage_*` / `requirements_behavior_*`: required coverage or behavior is still missing\n")
		builder.WriteString("- `context_*`: insufficient repository context or missing grounding\n")
		builder.WriteString("- `implementation_*`: code or patch quality gaps\n")
		builder.WriteString("- `scope_*`: blast radius or task-boundary findings\n")
		builder.WriteString("- `architecture_*`: design or layering violations\n\n")
		builder.WriteString("Routing rule:\n")
		builder.WriteString("- runtime treats `reason_codes` as authoritative; `next_action` is used only when the reason-code taxonomy does not resolve a supervisor action\n\n")
	}
	builder.WriteString(fmt.Sprintf("## Iteration %d\n\n", iteration))
	builder.WriteString(fmt.Sprintf("- decision: `%s`\n", fallbackString(decision, "unknown")))
	builder.WriteString(fmt.Sprintf("- selected_action: `%s`\n", fallbackString(action, "unknown")))
	builder.WriteString(fmt.Sprintf("- routing_status_after_iteration: `%s`\n", state.Status))
	builder.WriteString(fmt.Sprintf("- primary_reason_category: `%s`\n", category))
	builder.WriteString(fmt.Sprintf("- reason_codes: `%s`\n", joinOrNone(state.LastReasonCodes)))
	builder.WriteString(fmt.Sprintf("- quality_confidence: `%s`\n", qualityConfidence))
	builder.WriteString(fmt.Sprintf("- context_refresh: `count=%d, last_trigger=%s, last_reason_family=%s`\n", state.ContextRefreshCount, derefString(state.LastContextRefreshTrigger, "none"), derefString(state.LastContextRefreshReasonFamily, "none")))
	builder.WriteString(fmt.Sprintf("- executed_intervention_count: `%d`\n", executedInterventionCount))
	builder.WriteString(fmt.Sprintf("- guardrail_cost: `generator_revisions_used=%d, context_rebuilds_used=%d, validation_tightenings_used=%d`\n", state.GeneratorRevisionsUsed, state.ContextRefreshCount, state.ValidationTighteningsUsed))
	builder.WriteString(fmt.Sprintf("- guardrail_value: `trigger=%s, outcome=%s`\n", terminalRiskTriggerLabel(triggerReasonCodes, triggerCategory), guardrailValueOutcome(state.Status, qualityConfidence, executedInterventionCount)))
	if shouldTerminate(state) {
		builder.WriteString(fmt.Sprintf("- terminal_status: `%s`\n", state.Status))
	} else {
		builder.WriteString(fmt.Sprintf("- non_terminal_routing_state: `%s`\n", state.Status))
	}
	builder.WriteString(fmt.Sprintf("- budgets_remaining: `generator=%d, context=%d, validation=%d`\n\n", state.GeneratorRetriesRemaining, state.ContextRebuildsRemaining, state.ValidationTighteningsRemaining))

	file, err := os.OpenFile(tracePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open supervisor trace: %w", err)
	}
	defer file.Close()
	if _, err := file.WriteString(builder.String()); err != nil {
		return fmt.Errorf("write supervisor trace: %w", err)
	}
	return nil
}

func (r *Router) writeTerminalSummary(artifactDirectory string, state State) error {
	summaryPath := filepath.Join(artifactDirectory, "terminal_summary.md")
	evaluationPath := filepath.Join(artifactDirectory, "evaluation_result.yaml")
	executionPath := filepath.Join(artifactDirectory, "execution_report.yaml")
	evaluationMap := map[string]any{}
	if _, err := os.Stat(evaluationPath); err == nil {
		value, readErr := readYAMLMap(evaluationPath)
		if readErr != nil {
			return readErr
		}
		evaluationMap = value
	}
	if _, err := os.Stat(executionPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("terminal routing requires execution_report.yaml: %s", executionPath)
		}
		return fmt.Errorf("stat execution_report.yaml: %w", err)
	}
	executionMap, err := readYAMLMap(executionPath)
	if err != nil {
		return err
	}
	findings, err := readOptionalStringList(evaluationMap, "findings")
	if err != nil {
		return err
	}
	failureDetails, err := readOptionalStringList(executionMap, "failure_details")
	if err != nil {
		return err
	}
	logs, err := readOptionalStringList(executionMap, "logs")
	if err != nil {
		return err
	}
	qualityConfidence := readOptionalStringValue(evaluationMap, "quality_confidence", "unknown")
	action := lastAction(state.ActionHistory)
	if action == "" {
		action = "none"
	}
	decision := readOptionalStringValue(evaluationMap, "decision", derefString(state.LastDecision, "unknown"))
	reasonCategory := primaryReasonCategory(state.LastReasonCodes)
	triggerCategory := reasonCategory
	if state.LastInterventionTriggerCategory != nil {
		triggerCategory = *state.LastInterventionTriggerCategory
	}
	triggerReasonCodes := state.LastInterventionTriggerReasonCodes
	if len(triggerReasonCodes) == 0 {
		triggerReasonCodes = state.LastReasonCodes
	}
	executedInterventionCount := executedInterventionCount(state)

	var builder strings.Builder
	builder.WriteString("# Terminal Outcome\n\n")
	builder.WriteString(fmt.Sprintf("- status: `%s`\n", state.Status))
	builder.WriteString(fmt.Sprintf("- action: `%s`\n", action))
	builder.WriteString(fmt.Sprintf("- decision: `%s`\n", decision))
	builder.WriteString(fmt.Sprintf("- reason_category: `%s`\n", reasonCategory))
	builder.WriteString(fmt.Sprintf("- reason_codes: `%s`\n", joinOrNone(state.LastReasonCodes)))
	builder.WriteString(fmt.Sprintf("- quality_confidence: `%s`\n", qualityConfidence))
	builder.WriteString(fmt.Sprintf("- context_refresh: `count=%d, last_trigger=%s, last_reason_family=%s`\n\n", state.ContextRefreshCount, derefString(state.LastContextRefreshTrigger, "none"), derefString(state.LastContextRefreshReasonFamily, "none")))

	builder.WriteString("## Summary\n\n")
	builder.WriteString(terminalOutcomeSummary(state.Status))
	builder.WriteString("\n\n## Recommended Next Step\n\n")
	builder.WriteString("- " + terminalOutcomeNextStep(state.Status) + "\n\n")

	builder.WriteString("## Guardrail Cost\n\n")
	builder.WriteString(fmt.Sprintf("- executed_intervention_count: `%d`\n", executedInterventionCount))
	builder.WriteString(fmt.Sprintf("- generator_revisions_used: `%d`\n", state.GeneratorRevisionsUsed))
	builder.WriteString(fmt.Sprintf("- context_rebuilds_used: `%d`\n", state.ContextRefreshCount))
	builder.WriteString(fmt.Sprintf("- validation_tightenings_used: `%d`\n", state.ValidationTighteningsUsed))
	builder.WriteString(fmt.Sprintf("- terminal_status: `%s`\n\n", state.Status))

	builder.WriteString("## Guardrail Value\n\n")
	builder.WriteString(fmt.Sprintf("- trigger_failure_or_risk: `%s`\n", terminalRiskTriggerLabel(triggerReasonCodes, triggerCategory)))
	builder.WriteString(fmt.Sprintf("- last_intervention: `%s`\n", lastGuardrailIntervention(state.ActionHistory)))
	builder.WriteString(fmt.Sprintf("- final_quality_confidence: `%s`\n", qualityConfidence))
	builder.WriteString(fmt.Sprintf("- outcome: `%s`\n\n", guardrailValueOutcome(state.Status, qualityConfidence, executedInterventionCount)))

	if len(findings) > 0 {
		builder.WriteString("## Evaluator Findings\n")
		for _, finding := range findings {
			builder.WriteString("- " + finding + "\n")
		}
		builder.WriteString("\n")
	}
	if len(failureDetails) > 0 {
		builder.WriteString("## Failure Details\n")
		for _, detail := range failureDetails {
			builder.WriteString("- " + detail + "\n")
		}
		builder.WriteString("\n")
	}
	if len(logs) > 0 {
		builder.WriteString("## Command Logs\n")
		limit := min(len(logs), 10)
		for _, logLine := range logs[:limit] {
			builder.WriteString("- " + logLine + "\n")
		}
		if len(logs) > 10 {
			builder.WriteString(fmt.Sprintf("- ... (%d more)\n", len(logs)-10))
		}
		builder.WriteString("\n")
	}

	enrichedExecutionMap := map[string]any{}
	for key, value := range executionMap {
		enrichedExecutionMap[key] = value
	}
	enrichedExecutionMap["executed_intervention_count"] = executedInterventionCount
	enrichedExecutionMap["context_refresh"] = map[string]any{
		"count":              state.ContextRefreshCount,
		"last_trigger":       state.LastContextRefreshTrigger,
		"last_reason_family": state.LastContextRefreshReasonFamily,
	}
	enrichedExecutionMap["guardrail_cost"] = map[string]any{
		"generator_revisions_used":    state.GeneratorRevisionsUsed,
		"context_rebuilds_used":       state.ContextRefreshCount,
		"validation_tightenings_used": state.ValidationTighteningsUsed,
	}
	enrichedExecutionMap["guardrail_value"] = map[string]any{
		"trigger_failure_or_risk": terminalRiskTriggerLabel(triggerReasonCodes, triggerCategory),
		"trigger_reason_codes":    triggerReasonCodes,
		"trigger_reason_category": triggerCategory,
		"last_intervention":       lastGuardrailIntervention(state.ActionHistory),
		"quality_confidence":      qualityConfidence,
		"outcome":                 guardrailValueOutcome(state.Status, qualityConfidence, executedInterventionCount),
	}
	enrichedExecutionMap["terminal_status"] = state.Status
	enrichedExecutionMap["approved_memory_consideration"] = ensureApprovedMemoryConsideration(executionMap)
	if err := writeYAML(executionPath, enrichedExecutionMap); err != nil {
		return err
	}
	if _, err := r.validator.ValidateArtifactFile(executionPath, "execution_report"); err != nil {
		return err
	}

	return os.WriteFile(summaryPath, []byte(builder.String()), 0o644)
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

func readYAMLMap(path string) (map[string]any, error) {
	value, err := contracts.ReadYAMLFile(path)
	if err != nil {
		return nil, err
	}
	return contracts.AsMap(value, path)
}

func countActor(completedActors []string, actor string) int {
	count := 0
	for _, completedActor := range completedActors {
		if completedActor == actor {
			count++
		}
	}
	return count
}

func executedInterventionCount(state State) int {
	return state.GeneratorRevisionsUsed + state.ContextRefreshCount + state.ValidationTighteningsUsed
}

func terminalRiskTriggerLabel(reasonCodes []string, reasonCategory string) string {
	if len(reasonCodes) == 0 {
		if reasonCategory == "none" {
			return "none_active"
		}
		return reasonCategory + " :: resolved_before_terminal"
	}
	return reasonCategory + " :: " + strings.Join(reasonCodes, ", ")
}

func lastGuardrailIntervention(actionHistory []string) string {
	for index := len(actionHistory) - 1; index >= 0; index-- {
		action := actionHistory[index]
		switch action {
		case "revise_generator", "rebuild_context", "tighten_validation", "split_task", "block_environment":
			return action
		}
	}
	return "none"
}

func guardrailValueOutcome(status, qualityConfidence string, executedInterventionCount int) string {
	if status == "passed" {
		if executedInterventionCount == 0 {
			return "pass_without_guardrail_intervention"
		}
		switch qualityConfidence {
		case "high":
			return "improved_confidence"
		case "medium":
			return "accepted_after_intervention"
		case "low":
			return "accepted_with_low_confidence"
		default:
			return "accepted_after_intervention"
		}
	}
	switch status {
	case "blocked_environment", "split_required", "evolution_exhausted", "revise_exhausted", "rejected":
		return "bounded_refusal"
	default:
		return "needs_review"
	}
}

func terminalOutcomeSummary(status string) string {
	switch status {
	case "passed":
		return "The supervisor accepted the run and no further evolution step is required."
	case "blocked_environment":
		return "The supervisor was blocked by environment or tooling issues that prevented credible validation. More code changes would not have fixed this run."
	case "split_required":
		return "The supervisor stopped because the request is too broad or crosses task boundaries and should be decomposed before continuing."
	case "evolution_exhausted":
		return "The supervisor stopped because it ran out of bounded evolution actions without finding a credible path forward."
	case "revise_exhausted":
		return "The supervisor stopped because implementation retries were exhausted without closing the gap."
	case "rejected":
		return "The evaluator rejected the run because the result violated constraints or carried unacceptable risk."
	default:
		return "The harness recorded a terminal state, but this outcome needs manual review."
	}
}

func terminalOutcomeNextStep(status string) string {
	switch status {
	case "passed":
		return "Proceed to integration or handoff using the completed artifacts."
	case "blocked_environment":
		return "Fix the environment or tooling issue first, then rerun the same request rather than revising the generator output."
	case "split_required":
		return "Rewrite the request into smaller follow-up tasks with tighter scope and rerun them separately."
	case "evolution_exhausted":
		return "Inspect the supervisor trace, refine the request or context, and restart with a clearer validation strategy."
	case "revise_exhausted":
		return "Review the implementation findings, update the plan or context, and restart with a more credible generator target."
	case "rejected":
		return "Address the rejection reason before rerunning. Do not continue with the same artifact chain."
	default:
		return "Inspect the evaluator result and supervisor trace before deciding the next action."
	}
}

func joinOrNone(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return strings.Join(values, ", ")
}

func fallbackString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func derefString(value *string, fallback string) string {
	if value == nil {
		return fallback
	}
	return fallbackString(*value, fallback)
}

func readOptionalStringValue(source map[string]any, key, fallback string) string {
	value, ok := source[key]
	if !ok || value == nil {
		return fallback
	}
	text, ok := value.(string)
	if !ok {
		return fallback
	}
	return fallbackString(text, fallback)
}

func ensureApprovedMemoryConsideration(source map[string]any) map[string]any {
	existing, ok := source["approved_memory_consideration"].(map[string]any)
	if !ok {
		existing = map[string]any{}
	}
	return map[string]any{
		"considered_ref":                     readOptionalStringValue(existing, "considered_ref", ""),
		"lookup_key":                         readOptionalStringValue(existing, "lookup_key", ""),
		"task_family_source":                 readOptionalStringValue(existing, "task_family_source", ""),
		"disposition":                        readOptionalStringValue(existing, "disposition", "drop"),
		"reasons":                            fallbackStringList(existing, "reasons"),
		"originating_candidate_refs":         fallbackStringList(existing, "originating_candidate_refs"),
		"current_state_refresh_ref":          readOptionalStringValue(existing, "current_state_refresh_ref", ""),
		"current_state_refresh_generated_at": fallbackNullableString(existing, "current_state_refresh_generated_at"),
	}
}

func fallbackStringList(source map[string]any, key string) []string {
	values, err := readOptionalStringList(source, key)
	if err != nil {
		return []string{}
	}
	return values
}

func fallbackNullableString(source map[string]any, key string) any {
	value, ok := source[key]
	if !ok || value == nil {
		return nil
	}
	switch typed := value.(type) {
	case string:
		return stringScalarNode(typed)
	case time.Time:
		return stringScalarNode(typed.Format(time.RFC3339Nano))
	default:
		return nil
	}
}

func stringScalarNode(value string) *yaml.Node {
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: value,
	}
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
