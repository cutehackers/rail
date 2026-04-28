package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	workLedgerFileName          = "work_ledger.md"
	nextActionFileName          = "next_action.yaml"
	evidenceFileName            = "evidence.yaml"
	finalAnswerContractFileName = "final_answer_contract.yaml"
)

func initialWorkLedger(workflow Workflow) string {
	var builder strings.Builder
	builder.WriteString("# Work Ledger\n\n")
	builder.WriteString(fmt.Sprintf("- Task ID: `%s`\n", workflow.TaskID))
	builder.WriteString(fmt.Sprintf("- Task type: `%s`\n", workflow.TaskType))
	builder.WriteString(fmt.Sprintf("- Target project root: `%s`\n", workflow.ProjectRoot))
	builder.WriteString("- Status: `initialized`\n\n")
	builder.WriteString("## Notes\n")
	builder.WriteString("- Bootstrap created the initial workflow artifacts.\n")
	builder.WriteString("- Actors must use artifact evidence rather than conversation memory.\n")
	return builder.String()
}

func initialNextAction(workflow Workflow) map[string]any {
	var actor any
	if len(workflow.Actors) > 0 {
		actor = workflow.Actors[0]
	}
	return map[string]any{
		"actor":              actor,
		"reason":             "bootstrap_initialized",
		"must_do":            []string{"Read the actor brief and required contract inputs before producing output."},
		"must_not_do":        []string{"Do not write artifact files directly unless Rail explicitly owns that step."},
		"evidence_to_read":   []string{"request.yaml", "workflow.json", "execution_plan.json"},
		"blocking_questions": []string{},
	}
}

func initialEvidence() map[string]any {
	return map[string]any{
		"changed_files": []string{},
		"validation": map[string]any{
			"analyze": map[string]any{
				"command": "",
				"status":  "not_run",
			},
			"tests": map[string]any{
				"command": "",
				"status":  "not_run",
			},
		},
		"known_limits": []string{"No actor execution has run yet."},
	}
}

func initialFinalAnswerContract() map[string]any {
	return map[string]any{
		"required_sections": []string{
			"outcome",
			"changed_scope",
			"validation_evidence",
			"residual_risks",
			"next_step_if_blocked",
		},
		"forbidden_claims": []string{
			"claim_tests_passed_without_evidence",
			"claim_feature_complete_when_only_bootstrapped",
			"hide_policy_violation",
		},
	}
}

func buildNextActionAfterActor(actorName string, nextActor *string) map[string]any {
	var actor any
	if nextActor != nil {
		actor = *nextActor
	}
	return map[string]any{
		"actor":              actor,
		"reason":             "actor_completed",
		"must_do":            []string{"Read newly available artifact evidence before continuing."},
		"must_not_do":        []string{"Do not skip required prerequisite artifacts."},
		"evidence_to_read":   []string{artifactFileName(canonicalOutputForActor(actorName))},
		"blocking_questions": []string{},
	}
}

func buildNextActionAfterEvaluation(state State, fallbackActor string) map[string]any {
	actor, reason := nextActorAndReasonAfterEvaluation(state, fallbackActor)
	var actorValue any
	if actor != nil {
		actorValue = *actor
	}
	evidence := []string{"evaluation_result.yaml", "execution_report.yaml", "supervisor_trace.md"}
	if actor != nil {
		evidence = append(evidence, artifactFileName(canonicalOutputForActor(*actor)))
	}
	return map[string]any{
		"actor":              actorValue,
		"reason":             reason,
		"must_do":            nextActionMustDo(reason),
		"must_not_do":        []string{"Do not continue from conversation memory alone; read the listed Rail artifacts first."},
		"evidence_to_read":   evidence,
		"blocking_questions": []string{},
	}
}

func buildNextActionForBlockedActorRetry(actorName string) map[string]any {
	return map[string]any{
		"actor":              actorName,
		"reason":             "blocked_actor_retry",
		"must_do":            []string{"Rerun the blocked actor with the current sealed runtime before reporting a terminal block."},
		"must_not_do":        []string{"Do not use stale run_status.yaml contents to choose a different actor."},
		"evidence_to_read":   []string{"state.json", runStatusFileName, workLedgerFileName, "runs/"},
		"blocking_questions": []string{},
	}
}

func nextActorAndReasonAfterEvaluation(state State, fallbackActor string) (*string, string) {
	switch state.Status {
	case "passed":
		return nil, "terminal_pass"
	case "rejected", "revise_exhausted", "evolution_exhausted", "split_required":
		return nil, "terminal_reject"
	case "blocked_environment":
		return nil, "environment_blocked"
	}

	action := lastAction(state.ActionHistory)
	switch action {
	case "rebuild_context":
		return stringPtr("context_builder"), "context_rebuild_requested"
	case "tighten_validation":
		return stringPtr("executor"), "validation_tighten_requested"
	case "revise_generator":
		return stringPtr("generator"), "evaluator_requested_revision"
	}

	if state.LastDecision != nil && *state.LastDecision == "revise" {
		if state.CurrentActor != nil {
			return stringPtr(*state.CurrentActor), "evaluator_requested_revision"
		}
		return stringPtr(fallbackActor), "evaluator_requested_revision"
	}
	if state.CurrentActor != nil {
		return stringPtr(*state.CurrentActor), "evaluator_requested_action"
	}
	return nil, "terminal_reject"
}

func nextActionMustDo(reason string) []string {
	switch reason {
	case "terminal_pass", "terminal_reject", "environment_blocked":
		return []string{"Read the terminal summary and final answer contract before reporting to the user."}
	default:
		return []string{"Read evaluator feedback, execution evidence, and the required actor brief before continuing."}
	}
}

func appendWorkLedgerEntry(path string, heading string, lines []string) error {
	if strings.TrimSpace(heading) == "" {
		return fmt.Errorf("work ledger heading is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create work ledger directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open work ledger: %w", err)
	}
	defer file.Close()

	var builder strings.Builder
	builder.WriteString("\n## ")
	builder.WriteString(strings.TrimSpace(heading))
	builder.WriteString("\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		builder.WriteString("- ")
		builder.WriteString(trimmed)
		builder.WriteString("\n")
	}
	if _, err := file.WriteString(builder.String()); err != nil {
		return fmt.Errorf("append work ledger: %w", err)
	}
	return nil
}

func updateContinuityAfterActor(artifactDirectory string, actorName string, state State) error {
	if err := writeYAML(
		filepath.Join(artifactDirectory, nextActionFileName),
		buildNextActionAfterActor(actorName, state.CurrentActor),
	); err != nil {
		return err
	}
	if err := appendWorkLedgerEntry(
		filepath.Join(artifactDirectory, workLedgerFileName),
		"Actor completed: "+actorName,
		[]string{
			"status: " + state.Status,
			"next actor: " + actorLabel(state.CurrentActor),
			"evidence: " + artifactFileName(canonicalOutputForActor(actorName)),
		},
	); err != nil {
		return err
	}
	return writeRunStatus(artifactDirectory, runStatusAfterActor(artifactDirectory, actorName, state))
}

func updateContinuityAfterEvaluation(artifactDirectory string, state State) error {
	if err := writeYAML(
		filepath.Join(artifactDirectory, nextActionFileName),
		buildNextActionAfterEvaluation(state, "generator"),
	); err != nil {
		return err
	}
	if err := appendWorkLedgerEntry(
		filepath.Join(artifactDirectory, workLedgerFileName),
		"Evaluator routed",
		[]string{
			"status: " + state.Status,
			"decision: " + derefString(state.LastDecision, "unknown"),
			"action: " + fallbackString(lastAction(state.ActionHistory), "unknown"),
			"next actor: " + actorLabel(state.CurrentActor),
			"reason codes: " + joinOrNone(state.LastReasonCodes),
		},
	); err != nil {
		return err
	}
	return nil
}
