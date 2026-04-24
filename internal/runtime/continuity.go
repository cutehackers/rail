package runtime

import (
	"fmt"
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
