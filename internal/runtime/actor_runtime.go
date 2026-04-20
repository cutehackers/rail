package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func runStructuredCodexCommand(
	actorName string,
	workingDirectory string,
	prompt string,
	logPath string,
	schemaPath string,
	timeout time.Duration,
) (map[string]any, error) {
	model := strings.TrimSpace(os.Getenv("RAIL_ACTOR_MODEL"))
	if model == "" {
		model = "gpt-5.4-mini"
	}
	reasoningEffort := strings.TrimSpace(os.Getenv("RAIL_ACTOR_REASONING_EFFORT"))
	if reasoningEffort == "" {
		reasoningEffort = "low"
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"codex",
		"exec",
		"-m",
		model,
		"--cd",
		workingDirectory,
		"--ephemeral",
		"--color",
		"never",
		"-s",
		"danger-full-access",
		"--skip-git-repo-check",
		"-c",
		fmt.Sprintf(`model_reasoning_effort="%s"`, reasoningEffort),
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
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("timed out while executing actor `%s`", actorName)
	}
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
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"decision", "scores", "findings", "reason_codes", "quality_confidence", "next_action"},
			"properties": map[string]any{
				"decision": map[string]any{
					"type": "string",
					"enum": []string{"pass", "revise", "reject"},
				},
				"scores": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"requirements", "architecture", "regression_risk"},
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
					"anyOf": []map[string]any{
						{
							"type": "string",
							"enum": []string{"revise_generator", "rebuild_context", "tighten_validation", "split_task", "block_environment"},
						},
						{
							"type": "null",
						},
					},
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

func boundedNumberSchema() map[string]any {
	return map[string]any{
		"type":    "number",
		"minimum": 0,
		"maximum": 1,
	}
}
