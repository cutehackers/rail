package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type ActorCommandSpec struct {
	ActorName        string
	Profile          ActorProfile
	WorkingDirectory string
	Prompt           string
	LastMessagePath  string
	SchemaPath       string
	EventsPath       string
}

func buildCodexCLIArgs(backend ActorBackendConfig, spec ActorCommandSpec) []string {
	args := []string{
		backend.Subcommand,
		"-m",
		spec.Profile.Model,
		"--cd",
		spec.WorkingDirectory,
	}
	if backend.Ephemeral {
		args = append(args, "--ephemeral")
	}
	args = append(args,
		"--color",
		"never",
		"-s",
		backend.Sandbox,
	)
	if backend.SkipGitRepoCheck {
		args = append(args, "--skip-git-repo-check")
	}
	args = append(args,
		"-c",
		fmt.Sprintf(`model_reasoning_effort="%s"`, spec.Profile.Reasoning),
		"-c",
		fmt.Sprintf(`approval_policy="%s"`, backend.ApprovalPolicy),
		"--output-schema",
		spec.SchemaPath,
		"--output-last-message",
		spec.LastMessagePath,
	)
	if backend.CaptureJSONEvents {
		args = append(args, "--json")
	}
	args = append(args, spec.Prompt)
	return args
}

func defaultCodexCLIBackend() ActorBackendConfig {
	return ActorBackendConfig{
		Command:           "codex",
		Subcommand:        "exec",
		Sandbox:           "workspace-write",
		ApprovalPolicy:    "never",
		SessionMode:       "per_actor",
		Ephemeral:         true,
		CaptureJSONEvents: false,
		SkipGitRepoCheck:  true,
	}
}

// runCommand executes the current actor command backend using only the
// repository-resolved actor profile passed by the caller. Environment overrides
// are intentionally unsupported; profile selection belongs in actor_profiles.yaml.
func runCommand(backend ActorBackendConfig, spec ActorCommandSpec) (map[string]any, error) {
	profile, err := normalizeActorProfile(spec.ActorName, spec.Profile)
	if err != nil {
		return nil, err
	}
	spec.Profile = profile
	if backend.CaptureJSONEvents && strings.TrimSpace(spec.EventsPath) == "" {
		return nil, fmt.Errorf("capture JSON events requires events path")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, backend.Command, buildCodexCLIArgs(backend, spec)...)
	cmd.Dir = spec.WorkingDirectory

	output := &synchronizedBuffer{}
	watchdog := newActorWatchdog(spec.ActorName, defaultActorWatchdogConfig)
	progressWriter := watchdog.ProgressWriter()
	stdoutWriters := []io.Writer{output, progressWriter}
	var eventsFile *os.File
	if backend.CaptureJSONEvents {
		eventsFile, err = os.Create(spec.EventsPath)
		if err != nil {
			return nil, fmt.Errorf("create JSON events log for %s: %w", spec.ActorName, err)
		}
		defer eventsFile.Close()
		stdoutWriters = append(stdoutWriters, eventsFile)
	}
	cmd.Stdout = io.MultiWriter(stdoutWriters...)
	cmd.Stderr = io.MultiWriter(output, progressWriter)

	watchdog.Start(cancel)
	err = cmd.Run()
	watchdog.Stop()
	if expiration, expired := watchdog.Expiration(); expired {
		return nil, fmt.Errorf("actor `%s` failed: actor_watchdog_expired: no command progress observed for %s", expiration.ActorName, expiration.QuietWindow)
	}
	if err != nil {
		return nil, fmt.Errorf("actor `%s` failed: %s", spec.ActorName, strings.TrimSpace(output.String()))
	}

	data, err := os.ReadFile(spec.LastMessagePath)
	if err != nil {
		return nil, fmt.Errorf("read %s output: %w", spec.ActorName, err)
	}
	var response map[string]any
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("decode %s actor response: %w", spec.ActorName, err)
	}
	return response, nil
}

type synchronizedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *synchronizedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(p)
}

func (b *synchronizedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
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
