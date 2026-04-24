package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

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

	workflow, err := readWorkflow(filepath.Join(artifactDirectory, "workflow.json"))
	if err != nil {
		return "", err
	}
	state, err := readState(filepath.Join(artifactDirectory, "state.json"))
	if err != nil {
		return "", err
	}
	state = normalizeEvaluatorEntryState(state)
	if err := r.validateRequiredCriticReport(artifactDirectory, workflow); err != nil {
		return "", err
	}
	activeEvaluatorRouting := state.CurrentActor != nil && *state.CurrentActor == "evaluator" && !shouldTerminate(state)
	if refreshed, err := r.refreshPersistedOutputsIfNeeded(artifactDirectory, workflow, state, activeEvaluatorRouting); err != nil {
		return "", err
	} else if refreshed && !activeEvaluatorRouting {
		return formatExecutionSummary(artifactDirectory, state, "Harness evaluation refreshed persisted outputs"), nil
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
	if err := updateContinuityAfterEvaluation(artifactDirectory, nextState); err != nil {
		return "", err
	}
	if err := appendSupervisorDecisionTrace(artifactDirectory, nextState); err != nil {
		return "", err
	}
	if err := r.writeEnrichedExecutionReport(artifactDirectory, workflow, nextState, evaluationMap); err != nil {
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

func readWorkflow(path string) (Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Workflow{}, fmt.Errorf("read workflow: %w", err)
	}
	var workflow Workflow
	if err := json.Unmarshal(data, &workflow); err != nil {
		return Workflow{}, fmt.Errorf("decode workflow: %w", err)
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
	if state.QualityTrajectory == nil {
		state.QualityTrajectory = []QualityIteration{}
	}
	if state.ActorProfilesUsed == nil {
		state.ActorProfilesUsed = []ActorProfileUsed{}
	}
	if state.CompletedActors == nil {
		state.CompletedActors = []string{}
	}
	return state, nil
}

func (r *Router) validateRequiredCriticReport(artifactDirectory string, workflow Workflow) error {
	if !workflowRequiresCriticReport(workflow) {
		return nil
	}
	if _, err := r.validator.ValidateArtifactFile(filepath.Join(artifactDirectory, "critic_report.yaml"), "critic_report"); err != nil {
		return err
	}
	return nil
}

func (r *Router) refreshPersistedOutputsIfNeeded(artifactDirectory string, workflow Workflow, state State, activeEvaluatorRouting bool) (bool, error) {
	refreshed := false

	tracePath := filepath.Join(artifactDirectory, "supervisor_trace.md")
	if _, err := os.Stat(tracePath); err == nil {
		// already present
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("stat supervisor_trace.md: %w", err)
	} else if len(state.ActionHistory) > 0 && !activeEvaluatorRouting {
		if err := appendSupervisorDecisionTrace(artifactDirectory, state); err != nil {
			return false, err
		}
		refreshed = true
	}

	if len(state.ActionHistory) > 0 && !activeEvaluatorRouting {
		evaluationMap, err := r.validator.ValidateArtifactFile(filepath.Join(artifactDirectory, "evaluation_result.yaml"), "evaluation_result")
		if err != nil {
			return false, err
		}
		if err := r.writeEnrichedExecutionReport(artifactDirectory, workflow, state, evaluationMap); err != nil {
			return false, err
		}
		refreshed = true
	}

	if !shouldTerminate(state) {
		return refreshed, nil
	}

	summaryPath := filepath.Join(artifactDirectory, "terminal_summary.md")
	if _, err := os.Stat(summaryPath); err == nil {
		return refreshed, nil
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("stat terminal_summary.md: %w", err)
	}

	if err := r.writeTerminalSummary(artifactDirectory, state); err != nil {
		return false, err
	}
	return true, nil
}

func normalizeEvaluatorEntryState(state State) State {
	if state.CurrentActor == nil || *state.CurrentActor != "evaluator" {
		return state
	}

	nextState := state
	switch state.Status {
	case "revising":
		nextState.GeneratorRevisionsUsed++
	case "tightening_validation":
		nextState.ValidationTighteningsUsed++
	case "rebuilding_context":
		nextState.ContextRefreshCount++
		nextState.LastContextRefreshTrigger = state.PendingContextRefreshTrigger
		nextState.LastContextRefreshReasonFamily = state.PendingContextRefreshReasonFamily
		nextState.PendingContextRefreshTrigger = nil
		nextState.PendingContextRefreshReasonFamily = nil
	}
	return nextState
}

func advanceState(state State, workflow Workflow, evaluation map[string]any) (State, error) {
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
	qualityConfidence := normalizeQualityConfidence(readOptionalStringValue(evaluation, "quality_confidence", "low"))
	reasonCategory := primaryReasonCategory(reasonCodes)

	nextState.LastDecision = &decision
	nextState.LastReasonCodes = reasonCodes
	action := "pass"

	switch decision {
	case "pass":
		nextState.Status = "passed"
		nextState.CurrentActor = nil
		nextState.ActionHistory = append(nextState.ActionHistory, action)
	case "reject":
		action = "reject"
		nextState.Status = "rejected"
		nextState.CurrentActor = nil
		nextState.ActionHistory = append(nextState.ActionHistory, "reject")
	default:
		action = routeFromEvaluationResult(decision, reasonCodes, nextAction)
		if action == "" {
			return State{}, fmt.Errorf("evaluation_result.yaml revise decision requires either a supported `next_action` or routeable `reason_codes`")
		}

		nextState.LastInterventionTriggerReasonCodes = append([]string{}, reasonCodes...)
		nextState.LastInterventionTriggerCategory = stringPtr(reasonCategory)

		switch action {
		case "rebuild_context":
			nextState.ActionHistory = append(nextState.ActionHistory, action)
			nextState.ContextRebuildsRemaining--
			if nextState.ContextRebuildsRemaining < 0 || !contains(workflow.Actors, "context_builder") {
				nextState.Status = "evolution_exhausted"
				nextState.CurrentActor = nil
			} else {
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
				nextState.PendingContextRefreshTrigger = pendingTrigger
				nextState.PendingContextRefreshReasonFamily = pendingReasonFamily
			}
		case "tighten_validation":
			nextState.ActionHistory = append(nextState.ActionHistory, action)
			nextState.ValidationTighteningsRemaining--
			if nextState.ValidationTighteningsRemaining < 0 || !contains(workflow.Actors, "executor") {
				nextState.Status = "evolution_exhausted"
				nextState.CurrentActor = nil
			} else {
				nextState.Status = "tightening_validation"
				nextState.CurrentActor = stringPtr("executor")
			}
		case "split_task":
			nextState.Status = "split_required"
			nextState.CurrentActor = nil
			nextState.ActionHistory = append(nextState.ActionHistory, action)
		case "block_environment":
			nextState.Status = "blocked_environment"
			nextState.CurrentActor = nil
			nextState.ActionHistory = append(nextState.ActionHistory, action)
		default:
			nextState.ActionHistory = append(nextState.ActionHistory, "revise_generator")
			nextState.GeneratorRetriesRemaining--
			if nextState.GeneratorRetriesRemaining < 0 || !contains(workflow.Actors, "generator") {
				nextState.Status = "revise_exhausted"
				nextState.CurrentActor = nil
			} else {
				nextState.Status = "revising"
				nextState.CurrentActor = stringPtr("generator")
				action = "revise_generator"
			}
		}
	}

	nextState.QualityTrajectory = append(
		nextState.QualityTrajectory,
		buildQualityIteration(nextState, decision, action, qualityConfidence, reasonCodes, reasonCategory),
	)
	return nextState, nil
}

func buildQualityIteration(
	state State,
	decision string,
	action string,
	qualityConfidence string,
	reasonCodes []string,
	reasonCategory string,
) QualityIteration {
	return QualityIteration{
		Iteration:             len(state.QualityTrajectory) + 1,
		Actor:                 "evaluator",
		Decision:              decision,
		Action:                action,
		QualityConfidence:     qualityConfidence,
		ReasonCodes:           append([]string{}, reasonCodes...),
		TriggerCategory:       reasonCategory,
		Status:                state.Status,
		ExecutorInterventions: state.ContextRefreshCount + state.GeneratorRevisionsUsed + state.ValidationTighteningsUsed,
		ContextRebuilds:       state.ContextRefreshCount,
		GeneratorRevisions:    state.GeneratorRevisionsUsed,
		ValidationTightenings: state.ValidationTighteningsUsed,
		TotalInterventions:    executedInterventionCount(state),
	}
}

type qualityTrajectorySummary struct {
	Summary            string
	ImprovedIterations int
	DeclinedIterations int
}

func summarizeQualityTrajectory(iterations []QualityIteration) qualityTrajectorySummary {
	if len(iterations) == 0 {
		return qualityTrajectorySummary{Summary: "no trajectory available"}
	}
	improved := 0
	declined := 0
	prev := confidenceRank(iterations[0].QualityConfidence)
	for index := 1; index < len(iterations); index++ {
		curr := confidenceRank(iterations[index].QualityConfidence)
		if curr > prev {
			improved++
		}
		if curr < prev {
			declined++
		}
		prev = curr
	}
	if improved > declined {
		return qualityTrajectorySummary{
			Summary:            "overall_quality_trajectory_improved",
			ImprovedIterations: improved,
			DeclinedIterations: declined,
		}
	}
	if declined > improved {
		return qualityTrajectorySummary{
			Summary:            "overall_quality_trajectory_declined",
			ImprovedIterations: improved,
			DeclinedIterations: declined,
		}
	}
	return qualityTrajectorySummary{
		Summary:            "overall_quality_trajectory_stable",
		ImprovedIterations: improved,
		DeclinedIterations: declined,
	}
}

func qualityTrend(iterations []QualityIteration) string {
	if len(iterations) == 0 {
		return "insufficient_data"
	}
	if len(iterations) == 1 {
		return "single_observation"
	}
	current := confidenceRank(iterations[len(iterations)-1].QualityConfidence)
	previous := confidenceRank(iterations[len(iterations)-2].QualityConfidence)
	switch {
	case current > previous:
		return "improved"
	case current < previous:
		return "declined"
	default:
		return "stable"
	}
}

func confidenceRank(value string) int {
	switch normalizeQualityConfidence(value) {
	case "critical", "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func normalizeQualityConfidence(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "high":
		return "high"
	case "critical":
		return "high"
	case "medium":
		return "medium"
	case "low":
		return "low"
	default:
		return "low"
	}
}

func actorTraversalPath(state State) []string {
	path := append([]string{}, state.CompletedActors...)
	if state.CurrentActor != nil && *state.CurrentActor != "" {
		path = append(path, *state.CurrentActor)
	}
	return path
}

func actorVisitCountsSummary(path []string) string {
	counts := actorVisitCounts(path)
	if len(counts) == 0 {
		return "none"
	}
	keys := make([]string, 0, len(counts))
	for actor := range counts {
		keys = append(keys, actor)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, actor := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", actor, counts[actor]))
	}
	return strings.Join(parts, ", ")
}

func actorVisitCounts(path []string) map[string]int {
	counts := map[string]int{}
	for _, actor := range path {
		if actor == "" {
			continue
		}
		counts[actor]++
	}
	return counts
}

func actorTraversalSegments(path []string) []string {
	if len(path) <= 1 {
		return nil
	}
	segments := make([]string, 0, len(path)-1)
	for index := 1; index < len(path); index++ {
		from := path[index-1]
		to := path[index]
		if from == "" || to == "" {
			continue
		}
		segments = append(segments, from+"->"+to)
	}
	return segments
}

func actorGraphLine(path []string) string {
	if len(path) == 0 {
		return "none"
	}
	return strings.Join(path, " -> ")
}

func buildActorGraphReport(state State) map[string]any {
	path := actorTraversalPath(state)
	counts := actorVisitCounts(path)
	segments := actorTraversalSegments(path)
	return map[string]any{
		"path":        path,
		"total_steps": len(path),
		"segments":    segments,
		"visit_counts": func() map[string]int {
			output := map[string]int{}
			for key, value := range counts {
				output[key] = value
			}
			return output
		}(),
		"current_actor": state.CurrentActor,
	}
}

func buildQualityTrajectoryReport(trajectory []QualityIteration) []map[string]any {
	reports := make([]map[string]any, 0, len(trajectory))
	for _, entry := range trajectory {
		reports = append(reports, map[string]any{
			"iteration":              entry.Iteration,
			"actor":                  entry.Actor,
			"decision":               entry.Decision,
			"action":                 entry.Action,
			"quality_confidence":     entry.QualityConfidence,
			"reason_codes":           entry.ReasonCodes,
			"trigger_category":       entry.TriggerCategory,
			"status":                 entry.Status,
			"executor_interventions": entry.ExecutorInterventions,
			"context_rebuilds":       entry.ContextRebuilds,
			"generator_revisions":    entry.GeneratorRevisions,
			"validation_tightenings": entry.ValidationTightenings,
			"total_interventions":    entry.TotalInterventions,
		})
	}
	return reports
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
	qualityConfidence := normalizeQualityConfidence(readOptionalStringValue(evaluationMap, "quality_confidence", "low"))
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
	builder.WriteString("## Actor Graph and Traversal\n\n")
	actorPath := actorTraversalPath(state)
	actorSegments := joinOrNone(actorTraversalSegments(actorPath))
	builder.WriteString(fmt.Sprintf("- traversal_count: `%d`\n", len(actorPath)))
	builder.WriteString(fmt.Sprintf("- actor_graph: `%s`\n", actorGraphLine(actorPath)))
	builder.WriteString(fmt.Sprintf("- actor_visit_counts: `%s`\n", actorVisitCountsSummary(actorPath)))
	builder.WriteString(fmt.Sprintf("- actor_segments: `%s`\n", actorSegments))
	if len(state.QualityTrajectory) == 0 {
		builder.WriteString("- quality_trajectory: `none`\n\n")
	} else {
		builder.WriteString(fmt.Sprintf("- quality_trajectory_summary: `%s`\n", summarizeQualityTrajectory(state.QualityTrajectory).Summary))
		builder.WriteString(fmt.Sprintf("- quality_trajectory: `%s`\n", qualityTrend(state.QualityTrajectory)))
		builder.WriteString("## Quality Trajectory History\n\n")
		for _, entry := range state.QualityTrajectory {
			builder.WriteString(fmt.Sprintf(
				"- iteration %d: actor=%s decision=%s action=%s quality=%s reasons=[%s]\n",
				entry.Iteration,
				entry.Actor,
				entry.Decision,
				entry.Action,
				entry.QualityConfidence,
				joinOrNone(entry.ReasonCodes),
			))
		}
		builder.WriteString("\n")
	}
	builder.WriteString(fmt.Sprintf("- quality_trajectory_count: `%d`\n", len(state.QualityTrajectory)))
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
	workflow, err := readWorkflow(filepath.Join(artifactDirectory, "workflow.json"))
	if err != nil {
		return err
	}
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
	qualityConfidence := normalizeQualityConfidence(readOptionalStringValue(evaluationMap, "quality_confidence", "low"))
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

	actorPath := actorTraversalPath(state)
	actorSegments := joinOrNone(actorTraversalSegments(actorPath))
	builder.WriteString("## Actor Graph & Traversal\n\n")
	builder.WriteString(fmt.Sprintf("- traversal_count: `%d`\n", len(actorPath)))
	builder.WriteString(fmt.Sprintf("- actor_graph: `%s`\n", actorGraphLine(actorPath)))
	builder.WriteString(fmt.Sprintf("- actor_visit_counts: `%s`\n", actorVisitCountsSummary(actorPath)))
	builder.WriteString(fmt.Sprintf("- actor_segments: `%s`\n\n", actorSegments))

	builder.WriteString("## Quality Improvement Trajectory\n\n")
	if len(state.QualityTrajectory) == 0 {
		builder.WriteString("- no quality trajectory records captured yet\n\n")
	} else {
		improvementSummary := summarizeQualityTrajectory(state.QualityTrajectory)
		builder.WriteString(fmt.Sprintf("- summary: `%s`\n", improvementSummary.Summary))
		builder.WriteString(fmt.Sprintf("- trend: `%s`\n", qualityTrend(state.QualityTrajectory)))
		builder.WriteString(fmt.Sprintf("- iterations: `%d`\n\n", len(state.QualityTrajectory)))
		for _, entry := range state.QualityTrajectory {
			builder.WriteString(fmt.Sprintf(
				"- iteration %d: actor=%s decision=%s action=%s quality=%s reasons=[%s]\n",
				entry.Iteration,
				entry.Actor,
				entry.Decision,
				entry.Action,
				entry.QualityConfidence,
				joinOrNone(entry.ReasonCodes),
			))
		}
		builder.WriteString("\n")
	}

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
	improvementSummary := summarizeQualityTrajectory(state.QualityTrajectory)
	actorProfilesUsed, err := buildActorProfilesUsedReport(workflow, state)
	if err != nil {
		return err
	}
	criticFindingsApplied, criticToEvaluatorDelta, err := buildCriticReporting(artifactDirectory, workflow, evaluationMap, executionMap)
	if err != nil {
		return err
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
	enrichedExecutionMap["actor_graph"] = buildActorGraphReport(state)
	enrichedExecutionMap["actor_profiles_used"] = actorProfilesUsed
	enrichedExecutionMap["critic_findings_applied"] = criticFindingsApplied
	enrichedExecutionMap["critic_to_evaluator_delta"] = criticToEvaluatorDelta
	enrichedExecutionMap["quality_trajectory"] = buildQualityTrajectoryReport(state.QualityTrajectory)
	enrichedExecutionMap["quality_improvement_summary"] = map[string]any{
		"trend":               qualityTrend(state.QualityTrajectory),
		"summary":             improvementSummary.Summary,
		"improved_iterations": improvementSummary.ImprovedIterations,
		"declined_iterations": improvementSummary.DeclinedIterations,
		"iteration_count":     len(state.QualityTrajectory),
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

func (r *Router) writeEnrichedExecutionReport(artifactDirectory string, workflow Workflow, state State, evaluationMap map[string]any) error {
	executionPath := filepath.Join(artifactDirectory, "execution_report.yaml")
	executionMap, err := readYAMLMap(executionPath)
	if err != nil {
		if !shouldTerminate(state) || terminalSummaryPresent(artifactDirectory) {
			executionMap = recoveredExecutionReportBase(state)
		} else {
			return err
		}
	}
	failureDetails, err := readOptionalStringList(executionMap, "failure_details")
	if err != nil {
		return err
	}
	qualityConfidence := normalizeQualityConfidence(readOptionalStringValue(evaluationMap, "quality_confidence", "low"))
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

	enrichedExecutionMap := map[string]any{}
	for key, value := range executionMap {
		enrichedExecutionMap[key] = value
	}
	improvementSummary := summarizeQualityTrajectory(state.QualityTrajectory)
	actorProfilesUsed, err := buildActorProfilesUsedReport(workflow, state)
	if err != nil {
		return err
	}
	criticFindingsApplied, criticToEvaluatorDelta, err := buildCriticReporting(artifactDirectory, workflow, evaluationMap, executionMap)
	if err != nil {
		return err
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
	enrichedExecutionMap["actor_graph"] = buildActorGraphReport(state)
	enrichedExecutionMap["actor_profiles_used"] = actorProfilesUsed
	enrichedExecutionMap["critic_findings_applied"] = criticFindingsApplied
	enrichedExecutionMap["critic_to_evaluator_delta"] = criticToEvaluatorDelta
	enrichedExecutionMap["quality_trajectory"] = buildQualityTrajectoryReport(state.QualityTrajectory)
	enrichedExecutionMap["quality_improvement_summary"] = map[string]any{
		"trend":               qualityTrend(state.QualityTrajectory),
		"summary":             improvementSummary.Summary,
		"improved_iterations": improvementSummary.ImprovedIterations,
		"declined_iterations": improvementSummary.DeclinedIterations,
		"iteration_count":     len(state.QualityTrajectory),
	}
	enrichedExecutionMap["terminal_status"] = state.Status
	enrichedExecutionMap["approved_memory_consideration"] = ensureApprovedMemoryConsideration(executionMap)
	if err := writeYAML(executionPath, enrichedExecutionMap); err != nil {
		return err
	}
	if _, err := r.validator.ValidateArtifactFile(executionPath, "execution_report"); err != nil {
		return err
	}
	_ = failureDetails
	return nil
}

func recoveredExecutionReportBase(state State) map[string]any {
	return map[string]any{
		"format":  "fail",
		"analyze": "fail",
		"tests": map[string]any{
			"total":  0,
			"passed": 0,
			"failed": 0,
		},
		"failure_details": []any{
			fmt.Sprintf("execution_report.yaml was missing during route-evaluation refresh for status %s", state.Status),
		},
		"logs": []any{},
	}
}

func terminalSummaryPresent(artifactDirectory string) bool {
	return !persistedOutputMissing(filepath.Join(artifactDirectory, "terminal_summary.md"))
}

func buildActorProfilesUsedReport(workflow Workflow, state State) ([]map[string]any, error) {
	if err := validateActorProfilesSnapshot(workflow, state.ActorProfilesUsed); err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(state.ActorProfilesUsed))
	for _, profile := range state.ActorProfilesUsed {
		items = append(items, map[string]any{
			"actor":     profile.Actor,
			"model":     profile.Model,
			"reasoning": profile.Reasoning,
		})
	}
	return items, nil
}

func validateActorProfilesSnapshot(workflow Workflow, snapshot []ActorProfileUsed) error {
	requiredActors := profiledActorsForWorkflow(workflow)
	if len(snapshot) == 0 {
		return fmt.Errorf("state is missing persisted actorProfilesUsed snapshot")
	}
	requiredSet := make(map[string]struct{}, len(requiredActors))
	for _, actor := range requiredActors {
		requiredSet[actor] = struct{}{}
	}
	seen := make(map[string]struct{}, len(snapshot))
	for _, profile := range snapshot {
		actorName := strings.TrimSpace(profile.Actor)
		modelName := strings.TrimSpace(profile.Model)
		reasoning := strings.TrimSpace(profile.Reasoning)
		if actorName == "" || modelName == "" || reasoning == "" {
			return fmt.Errorf("actorProfilesUsed contains incomplete entry for actor %q", profile.Actor)
		}
		if _, ok := supportedActorReasoningEfforts[reasoning]; !ok {
			return fmt.Errorf("actorProfilesUsed contains unsupported reasoning %q for actor %q", profile.Reasoning, profile.Actor)
		}
		if actorName != profile.Actor || modelName != profile.Model || reasoning != profile.Reasoning {
			return fmt.Errorf("actorProfilesUsed contains non-normalized values for actor %q", profile.Actor)
		}
		if _, ok := requiredSet[actorName]; !ok {
			return fmt.Errorf("actorProfilesUsed contains unexpected actor %q", profile.Actor)
		}
		if _, ok := seen[actorName]; ok {
			return fmt.Errorf("actorProfilesUsed contains duplicate actor %q", profile.Actor)
		}
		seen[actorName] = struct{}{}
	}
	if len(seen) != len(requiredSet) {
		missing := []string{}
		for _, actor := range requiredActors {
			if _, ok := seen[actor]; !ok {
				missing = append(missing, actor)
			}
		}
		sort.Strings(missing)
		return fmt.Errorf("actorProfilesUsed is missing required actors: %s", strings.Join(missing, ", "))
	}
	return nil
}

func workflowRequiresCriticReport(workflow Workflow) bool {
	return contains(workflow.Actors, "critic")
}

type criticFindingRecord struct {
	Category string
	Finding  string
	Status   string
	Matched  []string
}

func buildCriticReporting(
	artifactDirectory string,
	workflow Workflow,
	evaluationMap map[string]any,
	executionMap map[string]any,
) (map[string]any, map[string]any, error) {
	criticFindings, err := loadCriticFindings(filepath.Join(artifactDirectory, "critic_report.yaml"), workflowRequiresCriticReport(workflow))
	if err != nil {
		return nil, nil, err
	}

	evaluatorFindings, err := readOptionalStringList(evaluationMap, "findings")
	if err != nil {
		return nil, nil, err
	}
	evaluatorReasonCodes, err := readOptionalStringList(evaluationMap, "reason_codes")
	if err != nil {
		return nil, nil, err
	}
	failureDetails, err := readOptionalStringList(executionMap, "failure_details")
	if err != nil {
		return nil, nil, err
	}

	reasonEvidence := make([]criticEvidence, 0, len(evaluatorReasonCodes))
	for _, code := range evaluatorReasonCodes {
		reasonEvidence = append(reasonEvidence, criticEvidence{Value: code, Kind: "reason_code"})
	}
	textEvidence := make([]criticEvidence, 0, len(evaluatorFindings)+len(failureDetails))
	for _, finding := range evaluatorFindings {
		textEvidence = append(textEvidence, criticEvidence{Value: finding, Kind: "evaluator_finding"})
	}
	for _, detail := range failureDetails {
		textEvidence = append(textEvidence, criticEvidence{Value: detail, Kind: "failure_detail"})
	}
	usedReasonEvidence := map[int]struct{}{}
	usedTextEvidence := map[int]struct{}{}

	resolvedCount := 0
	confirmedCount := 0
	unmetCount := 0
	confirmedCategories := map[string]struct{}{}
	appliedFindings := make([]map[string]any, 0, len(criticFindings))
	for _, finding := range criticFindings {
		matchedReasonCodes, confirmed := matchReasonEvidence(finding, reasonEvidence, usedReasonEvidence)
		textResolved := false
		if !confirmed {
			confirmed, textResolved = matchTextEvidence(finding, textEvidence, usedTextEvidence)
		}
		status := criticFindingStatus(confirmed, textResolved)
		switch status {
		case "resolved":
			resolvedCount++
		case "confirmed":
			confirmedCount++
			confirmedCategories[finding.Category] = struct{}{}
		default:
			unmetCount++
		}
		appliedFindings = append(appliedFindings, map[string]any{
			"category":             finding.Category,
			"finding":              finding.Finding,
			"status":               status,
			"matched_reason_codes": matchedReasonCodes,
		})
	}

	categories := make([]string, 0, len(confirmedCategories))
	for category := range confirmedCategories {
		categories = append(categories, category)
	}
	sort.Strings(categories)

	totalFindings := len(criticFindings)
	summary := "no critic findings available"
	if totalFindings > 0 {
		summary = fmt.Sprintf(
			"%d critic findings tracked: %d confirmed by evaluator, %d resolved before terminal evaluation, %d unmet at terminal state",
			totalFindings,
			confirmedCount,
			resolvedCount,
			unmetCount,
		)
	}

	return map[string]any{
			"total_findings":  totalFindings,
			"resolved_count":  resolvedCount,
			"confirmed_count": confirmedCount,
			"unmet_count":     unmetCount,
			"findings":        appliedFindings,
		}, map[string]any{
			"summary":                summary,
			"confirmed_count":        confirmedCount,
			"resolved_count":         resolvedCount,
			"unmet_count":            unmetCount,
			"evaluator_reason_codes": evaluatorReasonCodes,
			"confirmed_categories":   categories,
		}, nil
}

type criticEvidence struct {
	Value string
	Kind  string
}

func loadCriticFindings(path string, required bool) ([]criticFindingRecord, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			if required {
				return nil, fmt.Errorf("workflow requires critic_report but artifact is missing: %s", path)
			}
			return []criticFindingRecord{}, nil
		}
		return nil, fmt.Errorf("stat critic_report.yaml: %w", err)
	}

	report, err := readYAMLMap(path)
	if err != nil {
		return nil, err
	}

	categories := []string{
		"priority_focus",
		"missing_requirements",
		"risk_hypotheses",
		"validation_expectations",
		"generator_guardrails",
		"blocked_assumptions",
	}
	findings := make([]criticFindingRecord, 0)
	for _, category := range categories {
		values, err := readOptionalStringOrList(report, category)
		if err != nil {
			return nil, err
		}
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			findings = append(findings, criticFindingRecord{
				Category: category,
				Finding:  value,
			})
		}
	}
	return findings, nil
}

func readOptionalStringOrList(source map[string]any, key string) ([]string, error) {
	value, ok := source[key]
	if !ok || value == nil {
		return []string{}, nil
	}
	switch typed := value.(type) {
	case string:
		return []string{typed}, nil
	case []any:
		result := make([]string, 0, len(typed))
		for _, entry := range typed {
			text, ok := entry.(string)
			if !ok {
				return nil, fmt.Errorf("expected `%s` entries to be strings", key)
			}
			result = append(result, text)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("expected `%s` to be a string or list", key)
	}
}

func matchReasonEvidence(finding criticFindingRecord, evidence []criticEvidence, used map[int]struct{}) ([]string, bool) {
	bestIndex := -1
	bestScore := 0
	for index, item := range evidence {
		if _, ok := used[index]; ok {
			continue
		}
		score := scoreCriticReasonMatch(finding, item.Value)
		if score > bestScore {
			bestScore = score
			bestIndex = index
		}
	}
	if bestIndex == -1 || bestScore < 3 {
		return []string{}, false
	}
	used[bestIndex] = struct{}{}
	return []string{evidence[bestIndex].Value}, true
}

func matchTextEvidence(finding criticFindingRecord, evidence []criticEvidence, used map[int]struct{}) (bool, bool) {
	bestIndex := -1
	bestScore := 0
	for index, item := range evidence {
		if _, ok := used[index]; ok {
			continue
		}
		score := scoreCriticTextMatch(finding.Finding, item.Value)
		if score > bestScore {
			bestScore = score
			bestIndex = index
		}
	}
	if bestIndex == -1 || bestScore < 4 {
		return false, false
	}
	used[bestIndex] = struct{}{}
	return true, evidence[bestIndex].Kind == "evaluator_finding" && containsResolvedLanguage(evidence[bestIndex].Value)
}

func criticFindingStatus(confirmed bool, resolved bool) string {
	if resolved {
		return "resolved"
	}
	if confirmed {
		return "confirmed"
	}
	return "unmet"
}

func scoreCriticReasonMatch(finding criticFindingRecord, reasonCode string) int {
	score := 0
	normalizedCode := normalizeCriticEvidence(reasonCode)
	normalizedFinding := normalizeCriticEvidence(finding.Finding)
	if normalizedFinding != "" && strings.Contains(normalizedFinding, normalizedCode) {
		score += 4
	}
	if categoryPrefixMatchesFinding(finding.Category, reasonCode) {
		score++
	}
	score += tokenOverlapCount(meaningfulTokens(finding.Finding), meaningfulTokens(reasonCode))
	return score
}

func categoryPrefixMatchesFinding(category string, reasonCode string) bool {
	switch category {
	case "missing_requirements":
		return strings.HasPrefix(reasonCode, "requirements_")
	case "risk_hypotheses":
		return strings.HasPrefix(reasonCode, "architecture_") ||
			strings.HasPrefix(reasonCode, "context_") ||
			strings.HasPrefix(reasonCode, "environment_") ||
			strings.HasPrefix(reasonCode, "implementation_") ||
			strings.HasPrefix(reasonCode, "scope_") ||
			strings.HasPrefix(reasonCode, "validation_")
	case "validation_expectations":
		return strings.HasPrefix(reasonCode, "validation_")
	case "generator_guardrails":
		return strings.HasPrefix(reasonCode, "architecture_") ||
			strings.HasPrefix(reasonCode, "implementation_") ||
			strings.HasPrefix(reasonCode, "validation_")
	case "blocked_assumptions":
		return strings.HasPrefix(reasonCode, "context_") ||
			strings.HasPrefix(reasonCode, "environment_") ||
			strings.HasPrefix(reasonCode, "scope_")
	default:
		return false
	}
}

func scoreCriticTextMatch(finding string, evidence string) int {
	score := 0
	normalizedFinding := normalizeCriticEvidence(finding)
	normalizedEvidence := normalizeCriticEvidence(evidence)
	if normalizedFinding != "" && normalizedEvidence != "" &&
		(strings.Contains(normalizedEvidence, normalizedFinding) || strings.Contains(normalizedFinding, normalizedEvidence)) {
		score += 5
	}
	score += tokenOverlapCount(meaningfulTokens(finding), meaningfulTokens(evidence))
	return score
}

func containsResolvedLanguage(value string) bool {
	normalized := normalizeCriticEvidence(value)
	for _, fragment := range []string{
		"not resolved",
		"not addressed",
		"not satisfied",
		"not covered",
		"not fixed",
		"no longer unresolved",
		"still unresolved",
		"still not covered",
		"still not resolved",
		"unresolved",
	} {
		if strings.Contains(normalized, fragment) {
			return false
		}
	}
	for _, fragment := range []string{"resolved", "addressed", "satisfied", "covered", "fixed", "no longer"} {
		if strings.Contains(normalized, fragment) {
			return true
		}
	}
	return false
}

func normalizeCriticEvidence(value string) string {
	var builder strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			builder.WriteRune(r)
		default:
			builder.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(builder.String()), " ")
}

func meaningfulTokens(value string) []string {
	fields := strings.Fields(normalizeCriticEvidence(value))
	if len(fields) == 0 {
		return nil
	}
	result := make([]string, 0, len(fields))
	for _, field := range fields {
		if len(field) <= 2 {
			continue
		}
		result = append(result, field)
	}
	return result
}

func tokenOverlapCount(left []string, right []string) int {
	if len(left) == 0 || len(right) == 0 {
		return 0
	}
	rightSet := make(map[string]struct{}, len(right))
	for _, token := range right {
		rightSet[token] = struct{}{}
	}
	count := 0
	seen := map[string]struct{}{}
	for _, token := range left {
		if _, ok := seen[token]; ok {
			continue
		}
		if _, ok := rightSet[token]; ok {
			count++
			seen[token] = struct{}{}
		}
	}
	return count
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
