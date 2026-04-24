package reporting

import (
	"fmt"
	"os"
	"strings"
)

type TerminalOutcome struct {
	Status                    string
	PolicyViolations          []string
	MissingValidationEvidence []string
}

func ReadTerminalSummary(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read terminal summary %s: %w", path, err)
	}
	return string(data), nil
}

func WriteTerminalSummary(path, summary string) error {
	if err := os.WriteFile(path, []byte(summary), 0o644); err != nil {
		return fmt.Errorf("write terminal summary %s: %w", path, err)
	}
	return nil
}

func BuildReportingLimits(outcome TerminalOutcome) string {
	if len(outcome.PolicyViolations) == 0 && len(outcome.MissingValidationEvidence) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("## Reporting Limits\n\n")
	if len(outcome.PolicyViolations) > 0 {
		builder.WriteString("- Final answer must not claim successful implementation because policy violations were detected.\n")
		for _, violation := range outcome.PolicyViolations {
			trimmed := strings.TrimSpace(violation)
			if trimmed == "" {
				continue
			}
			builder.WriteString("- policy violation: `")
			builder.WriteString(trimmed)
			builder.WriteString("`\n")
		}
	}
	if len(outcome.MissingValidationEvidence) > 0 {
		builder.WriteString("- Final answer must not claim validation passed without concrete evidence.\n")
		for _, missing := range outcome.MissingValidationEvidence {
			trimmed := strings.TrimSpace(missing)
			if trimmed == "" {
				continue
			}
			builder.WriteString("- missing evidence: `")
			builder.WriteString(trimmed)
			builder.WriteString("`\n")
		}
	}
	builder.WriteString("- Required final-answer contract: include outcome, changed scope, validation evidence, residual risks, and next step if blocked.\n\n")
	return builder.String()
}
