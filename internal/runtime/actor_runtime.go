package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func runStructuredCodexCommand(
	actorName string,
	profile ActorProfile,
	workingDirectory string,
	prompt string,
	logPath string,
	schemaPath string,
) (map[string]any, error) {
	if strings.TrimSpace(profile.Model) == "" {
		return nil, fmt.Errorf("missing actor profile model for structured actor %q", actorName)
	}
	if _, ok := supportedActorReasoningEfforts[strings.TrimSpace(profile.Reasoning)]; !ok {
		return nil, fmt.Errorf("unsupported actor profile reasoning %q for structured actor %q", profile.Reasoning, actorName)
	}

	cmd := exec.Command(
		"codex",
		"exec",
		"-m",
		profile.Model,
		"--cd",
		workingDirectory,
		"--ephemeral",
		"--color",
		"never",
		"-s",
		"danger-full-access",
		"--skip-git-repo-check",
		"-c",
		fmt.Sprintf(`model_reasoning_effort="%s"`, profile.Reasoning),
		"-c",
		`approval_policy="never"`,
		"--output-schema",
		schemaPath,
		"--output-last-message",
		logPath,
		prompt,
	)
	cmd.Dir = workingDirectory
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("actor `%s` failed: %s", actorName, strings.TrimSpace(string(output)))
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		return nil, fmt.Errorf("read %s output: %w", actorName, err)
	}
	var response map[string]any
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("decode %s structured response: %w", actorName, err)
	}
	return response, nil
}

func materializeActorOutputSchema(runsDirectory string, actorIndex int, actorName string, outputName string) (string, error) {
	schema, err := actorOutputJSONSchema(outputName)
	if err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal %s output schema: %w", actorName, err)
	}
	targetPath := filepath.Join(runsDirectory, fmt.Sprintf("%02d_%s-output-schema.json", actorIndex+1, actorName))
	if err := os.WriteFile(targetPath, data, 0o644); err != nil {
		return "", fmt.Errorf("write %s output schema: %w", actorName, err)
	}
	return targetPath, nil
}

func actorOutputJSONSchema(outputName string) (map[string]any, error) {
	switch outputName {
	case "plan":
		return map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"summary", "likely_files", "assumptions", "substeps", "risks", "acceptance_criteria_refined"},
			"properties": map[string]any{
				"summary":                     map[string]any{"type": "string"},
				"likely_files":                stringArraySchema(),
				"assumptions":                 stringArraySchema(),
				"substeps":                    stringArraySchema(),
				"risks":                       stringArraySchema(),
				"acceptance_criteria_refined": stringArraySchema(),
			},
		}, nil
	case "context_pack":
		return map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"relevant_files", "repo_patterns", "test_patterns", "forbidden_changes", "implementation_hints"},
			"properties": map[string]any{
				"relevant_files": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type":                 "object",
						"additionalProperties": false,
						"required":             []string{"path", "why"},
						"properties": map[string]any{
							"path": map[string]any{"type": "string"},
							"why":  map[string]any{"type": "string"},
						},
					},
				},
				"repo_patterns":        stringArraySchema(),
				"test_patterns":        stringArraySchema(),
				"forbidden_changes":    stringArraySchema(),
				"implementation_hints": stringArraySchema(),
			},
		}, nil
	case "critic_report":
		return map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required": []string{
				"priority_focus",
				"missing_requirements",
				"risk_hypotheses",
				"validation_expectations",
				"generator_guardrails",
				"blocked_assumptions",
			},
			"properties": map[string]any{
				"priority_focus":          boundedStringArraySchema(6),
				"missing_requirements":    boundedStringArraySchema(8),
				"risk_hypotheses":         boundedStringArraySchema(8),
				"validation_expectations": boundedStringArraySchema(8),
				"generator_guardrails":    boundedStringArraySchema(8),
				"blocked_assumptions":     boundedStringArraySchema(8),
			},
		}, nil
	case "implementation_result":
		return map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"changed_files", "patch_summary", "tests_added_or_updated", "known_limits"},
			"properties": map[string]any{
				"changed_files":          stringArraySchema(),
				"patch_summary":          stringArraySchema(),
				"tests_added_or_updated": stringArraySchema(),
				"known_limits":           stringArraySchema(),
			},
		}, nil
	case "evaluation_result":
		return map[string]any{
			"type":     "object",
			"required": []string{"decision", "findings", "reason_codes", "quality_confidence"},
			"properties": map[string]any{
				"decision": map[string]any{
					"type": "string",
					"enum": []string{"pass", "revise", "reject"},
				},
				"scores": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"requirements":    boundedNumberSchema(),
						"architecture":    boundedNumberSchema(),
						"regression_risk": boundedNumberSchema(),
					},
				},
				"findings":     stringArraySchema(),
				"reason_codes": stringArraySchema(),
				"quality_confidence": map[string]any{
					"type": "string",
					"enum": []string{"high", "medium", "low"},
				},
				"next_action": map[string]any{
					"type": "string",
					"enum": []string{"revise_generator", "rebuild_context", "tighten_validation", "split_task", "block_environment"},
				},
			},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported actor output schema: %s", outputName)
	}
}

func stringArraySchema() map[string]any {
	return map[string]any{
		"type": "array",
		"items": map[string]any{
			"type": "string",
		},
	}
}

func boundedStringArraySchema(maxItems int) map[string]any {
	schema := stringArraySchema()
	schema["maxItems"] = maxItems
	schema["items"] = map[string]any{
		"type":      "string",
		"maxLength": 240,
	}
	return schema
}

func boundedNumberSchema() map[string]any {
	return map[string]any{
		"type":    "number",
		"minimum": 0,
		"maximum": 1,
	}
}
