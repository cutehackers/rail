package runtime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"rail/internal/assets"

	"gopkg.in/yaml.v3"
)

func TestRunCommandStopsWhenActorWatchdogSeesNoProgress(t *testing.T) {
	workingDirectory := t.TempDir()
	fakeBin := t.TempDir()
	logPath := filepath.Join(workingDirectory, "response.json")
	schemaPath := filepath.Join(workingDirectory, "schema.json")
	fakeCodexPath := filepath.Join(fakeBin, "codex")

	script := `#!/usr/bin/env python3
import time

time.sleep(0.5)
`
	if err := os.WriteFile(fakeCodexPath, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake codex: %v", err)
	}

	defaultActorWatchdogConfig = ActorWatchdogConfig{
		QuietWindow: 25 * time.Millisecond,
		CheckEvery:  5 * time.Millisecond,
	}
	t.Cleanup(func() {
		defaultActorWatchdogConfig = productionActorWatchdogConfig()
	})
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	_, err := runCommand(
		"planner",
		ActorProfile{Model: "gpt-5.4-mini", Reasoning: "high"},
		workingDirectory,
		"prompt",
		logPath,
		schemaPath,
	)
	if err == nil {
		t.Fatalf("expected runCommand to fail when actor watchdog sees no progress")
	}
	if !strings.Contains(err.Error(), "actor_watchdog_expired") {
		t.Fatalf("expected actor_watchdog_expired error, got %v", err)
	}
}

func TestRunCommandKeepsRunningWhenActorWatchdogSeesProgress(t *testing.T) {
	workingDirectory := t.TempDir()
	fakeBin := t.TempDir()
	logPath := filepath.Join(workingDirectory, "response.json")
	schemaPath := filepath.Join(workingDirectory, "schema.json")
	fakeCodexPath := filepath.Join(fakeBin, "codex")

	script := `#!/usr/bin/env bash
set -euo pipefail

output_path=""
while (($# > 0)); do
  if [[ "$1" == "--output-last-message" ]]; then
    shift
    output_path="$1"
  fi
  shift || true
done

for index in 1 2 3 4 5 6; do
  printf 'progress %s\n' "$index"
  sleep 0.2
done

printf '{"summary":"ok"}' >"$output_path"
`
	if err := os.WriteFile(fakeCodexPath, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake codex: %v", err)
	}

	defaultActorWatchdogConfig = ActorWatchdogConfig{
		QuietWindow: time.Second,
		CheckEvery:  25 * time.Millisecond,
	}
	t.Cleanup(func() {
		defaultActorWatchdogConfig = productionActorWatchdogConfig()
	})
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	response, err := runCommand(
		"planner",
		ActorProfile{Model: "gpt-5.4-mini", Reasoning: "high"},
		workingDirectory,
		"prompt",
		logPath,
		schemaPath,
	)
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}
	if got, want := response["summary"], "ok"; got != want {
		t.Fatalf("unexpected actor response summary: got %#v want %#v", got, want)
	}
}

func TestRunCommandUsesNormalizedActorProfileWithoutEnvFallbacks(t *testing.T) {
	workingDirectory := t.TempDir()
	fakeBin := t.TempDir()
	invocationPath := filepath.Join(workingDirectory, "invocation.json")
	logPath := filepath.Join(workingDirectory, "response.json")
	schemaPath := filepath.Join(workingDirectory, "schema.json")
	fakeCodexPath := filepath.Join(fakeBin, "codex")

	script := `#!/usr/bin/env python3
import json
import os
import re
import sys

invocation_path = os.environ["RAIL_TEST_INVOCATION_PATH"]
output_path = None
model = ""
reasoning = ""

for index, value in enumerate(sys.argv):
    if value == "--output-last-message" and index + 1 < len(sys.argv):
        output_path = sys.argv[index + 1]
    if value == "-m" and index + 1 < len(sys.argv):
        model = sys.argv[index + 1]
    if value == "-c" and index + 1 < len(sys.argv):
        match = re.match(r'model_reasoning_effort="([^"]+)"', sys.argv[index + 1])
        if match:
            reasoning = match.group(1)

with open(invocation_path, "w", encoding="utf-8") as handle:
    json.dump({"model": model, "reasoning": reasoning}, handle)

with open(output_path, "w", encoding="utf-8") as handle:
    json.dump({"summary": "ok"}, handle)
`
	if err := os.WriteFile(fakeCodexPath, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake codex: %v", err)
	}

	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("RAIL_ACTOR_MODEL", "wrong-model")
	t.Setenv("RAIL_ACTOR_REASONING_EFFORT", "low")
	t.Setenv("RAIL_TEST_INVOCATION_PATH", invocationPath)

	response, err := runCommand(
		"planner",
		ActorProfile{Model: " gpt-5.4-mini ", Reasoning: " high "},
		workingDirectory,
		"prompt",
		logPath,
		schemaPath,
	)
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}
	if got, want := response["summary"], "ok"; got != want {
		t.Fatalf("unexpected actor response summary: got %#v want %#v", got, want)
	}

	data, err := os.ReadFile(invocationPath)
	if err != nil {
		t.Fatalf("failed to read invocation log: %v", err)
	}

	var invocation struct {
		Model     string `json:"model"`
		Reasoning string `json:"reasoning"`
	}
	if err := json.Unmarshal(data, &invocation); err != nil {
		t.Fatalf("failed to decode invocation log: %v", err)
	}
	if invocation.Model != "gpt-5.4-mini" {
		t.Fatalf("expected runCommand to use normalized profile model, got %q", invocation.Model)
	}
	if invocation.Reasoning != "high" {
		t.Fatalf("expected runCommand to use normalized profile reasoning, got %q", invocation.Reasoning)
	}
}

func TestActorOutputJSONSchemaPlanRequiresAssumptions(t *testing.T) {
	schema, err := actorOutputJSONSchema("plan")
	if err != nil {
		t.Fatalf("actorOutputJSONSchema returned error: %v", err)
	}

	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected properties map, got %T", schema["properties"])
	}
	if _, ok := properties["assumptions"]; !ok {
		t.Fatalf("expected plan schema to expose assumptions property")
	}

	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatalf("expected required array, got %T", schema["required"])
	}
	requiredSet := map[string]struct{}{}
	for _, name := range required {
		requiredSet[name] = struct{}{}
	}
	for name := range properties {
		if _, ok := requiredSet[name]; !ok {
			t.Fatalf("expected required list to include property %q", name)
		}
	}
}

func TestActorOutputJSONSchemaCriticReport(t *testing.T) {
	schema, err := actorOutputJSONSchema("critic_report")
	if err != nil {
		t.Fatalf("actorOutputJSONSchema returned error: %v", err)
	}

	if got, ok := schema["additionalProperties"].(bool); !ok || got {
		t.Fatalf("expected critic_report schema to be closed, got %v", schema["additionalProperties"])
	}

	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected properties map, got %T", schema["properties"])
	}

	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatalf("expected required array, got %T", schema["required"])
	}
	requiredSet := map[string]struct{}{}
	for _, name := range required {
		requiredSet[name] = struct{}{}
	}

	expectedFields := []string{
		"priority_focus",
		"missing_requirements",
		"risk_hypotheses",
		"validation_expectations",
		"generator_guardrails",
		"blocked_assumptions",
	}
	for _, field := range expectedFields {
		if _, ok := properties[field]; !ok {
			t.Fatalf("expected critic_report schema to expose %q", field)
		}
		if _, ok := requiredSet[field]; !ok {
			t.Fatalf("expected critic_report schema to require %q", field)
		}
	}
	if len(required) != len(expectedFields) {
		t.Fatalf("unexpected required field count: got %d want %d", len(required), len(expectedFields))
	}

	maxItemsByField := map[string]int{
		"priority_focus":          6,
		"missing_requirements":    8,
		"risk_hypotheses":         8,
		"validation_expectations": 8,
		"generator_guardrails":    8,
		"blocked_assumptions":     8,
	}
	for field, wantMaxItems := range maxItemsByField {
		property, ok := properties[field].(map[string]any)
		if !ok {
			t.Fatalf("expected %q property schema to be a map, got %T", field, properties[field])
		}
		if got, ok := property["type"].(string); !ok || got != "array" {
			t.Fatalf("expected %q to be an array schema, got %v", field, property["type"])
		}
		if got, ok := property["maxItems"].(int); !ok || got != wantMaxItems {
			t.Fatalf("unexpected maxItems for %q: got %v want %v", field, property["maxItems"], wantMaxItems)
		}
		items, ok := property["items"].(map[string]any)
		if !ok {
			t.Fatalf("expected %q items schema to be a map, got %T", field, property["items"])
		}
		if got, ok := items["type"].(string); !ok || got != "string" {
			t.Fatalf("expected %q items to be strings, got %v", field, items["type"])
		}
		if got, ok := items["maxLength"].(int); !ok || got != 240 {
			t.Fatalf("unexpected item maxLength for %q: got %v want %d", field, items["maxLength"], 240)
		}
	}
}

func TestActorOutputJSONSchemaCriticReportMatchesTemplate(t *testing.T) {
	schema, err := actorOutputJSONSchema("critic_report")
	if err != nil {
		t.Fatalf("actorOutputJSONSchema returned error: %v", err)
	}

	repoRoot := testRepoRoot(t)
	repoTemplate := loadCriticReportSchemaTemplate(t, repoRoot)
	embeddedTemplate := loadCriticReportSchemaTemplate(t, t.TempDir())
	assertNormalizedSchemaEqual(t, normalizeSchemaForParity(schema), normalizeSchemaForParity(repoTemplate), "runtime schema", "repo template")
	assertNormalizedSchemaEqual(t, normalizeSchemaForParity(repoTemplate), normalizeSchemaForParity(embeddedTemplate), "repo template", "embedded default template")
	assertNormalizedSchemaEqual(t, normalizeSchemaForParity(schema), normalizeSchemaForParity(embeddedTemplate), "runtime schema", "embedded default template")
}

func TestActorOutputJSONSchemaEvaluationResultMatchesTemplate(t *testing.T) {
	schema, err := actorOutputJSONSchema("evaluation_result")
	if err != nil {
		t.Fatalf("actorOutputJSONSchema returned error: %v", err)
	}

	repoRoot := testRepoRoot(t)
	repoTemplate := loadSchemaTemplate(t, repoRoot, ".harness/templates/evaluation_result.schema.yaml")
	embeddedTemplate := loadSchemaTemplate(t, t.TempDir(), ".harness/templates/evaluation_result.schema.yaml")
	assertNormalizedSchemaEqual(t, normalizeSchemaForParity(schema), normalizeSchemaForParity(repoTemplate), "runtime schema", "repo template")
	assertNormalizedSchemaEqual(t, normalizeSchemaForParity(repoTemplate), normalizeSchemaForParity(embeddedTemplate), "repo template", "embedded default template")
	assertNormalizedSchemaEqual(t, normalizeSchemaForParity(schema), normalizeSchemaForParity(embeddedTemplate), "runtime schema", "embedded default template")
}

func TestNormalizeActorResponseDropsNullNextActionForTerminalEvaluation(t *testing.T) {
	normalized := normalizeActorResponse("evaluation_result", map[string]any{
		"decision":           "pass",
		"next_action":        nil,
		"quality_confidence": "high",
	})

	if _, exists := normalized["next_action"]; exists {
		t.Fatalf("expected next_action to be removed for terminal evaluation decisions")
	}
}

func loadCriticReportSchemaTemplate(t *testing.T, projectRoot string) map[string]any {
	t.Helper()

	return loadSchemaTemplate(t, projectRoot, ".harness/templates/critic_report.schema.yaml")
}

func loadSchemaTemplate(t *testing.T, projectRoot string, relativePath string) map[string]any {
	t.Helper()

	data, _, err := assets.Resolve(projectRoot, relativePath)
	if err != nil {
		t.Fatalf("failed to load schema template %s: %v", relativePath, err)
	}

	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to decode schema template %s: %v", relativePath, err)
	}

	template, ok := normalizeSchemaTestValue(raw).(map[string]any)
	if !ok {
		t.Fatalf("expected template schema to decode to a map, got %T", raw)
	}
	return template
}

func assertNormalizedSchemaEqual(t *testing.T, actual map[string]any, expected map[string]any, actualLabel string, expectedLabel string) {
	t.Helper()

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("schema mismatch for %s vs %s:\nactual=%#v\nexpected=%#v", actualLabel, expectedLabel, actual, expected)
	}
}

func normalizeSchemaTestValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		normalized := make(map[string]any, len(typed))
		for key, nested := range typed {
			normalized[key] = normalizeSchemaTestValue(nested)
		}
		return normalized
	case map[any]any:
		normalized := make(map[string]any, len(typed))
		for key, nested := range typed {
			normalized[key.(string)] = normalizeSchemaTestValue(nested)
		}
		return normalized
	case []any:
		normalized := make([]any, len(typed))
		for index, nested := range typed {
			normalized[index] = normalizeSchemaTestValue(nested)
		}
		return normalized
	default:
		return value
	}
}

func normalizeSchemaForParity(value any) map[string]any {
	normalized, ok := normalizeSchemaParityValue(value).(map[string]any)
	if !ok {
		return nil
	}
	return normalized
}

func normalizeSchemaParityValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		normalized := make(map[string]any, len(typed))
		for key, nested := range typed {
			normalized[key] = normalizeSchemaParityValue(nested)
		}
		return normalized
	case []string:
		normalized := append([]string{}, typed...)
		sort.Strings(normalized)
		items := make([]any, len(normalized))
		for i, item := range normalized {
			items[i] = item
		}
		return items
	case []any:
		normalized := make([]any, len(typed))
		for i, nested := range typed {
			normalized[i] = normalizeSchemaParityValue(nested)
		}
		if allStrings(normalized) {
			sort.Slice(normalized, func(i, j int) bool {
				return normalized[i].(string) < normalized[j].(string)
			})
		}
		return normalized
	case int:
		return float64(typed)
	default:
		return value
	}
}

func allStrings(values []any) bool {
	for _, value := range values {
		if _, ok := value.(string); !ok {
			return false
		}
	}
	return true
}
