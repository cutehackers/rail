package request

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestComposeRequestNormalizesDraftFromStdin(t *testing.T) {
	draft, err := DecodeDraft(strings.NewReader(`{
		"request_version": " ",
		"project_root": "./target-app",
		"task_type": " bug_fix ",
		"goal": "  Fix the loading indicator  ",
		"context": [" profile screen ", "", " retry path "],
		"constraints": [" no new deps ", " "],
		"definition_of_done": [" spinner clears ", " regression test added "]
	}`))
	if err != nil {
		t.Fatalf("DecodeDraft returned error: %v", err)
	}

	normalized, err := NormalizeDraft(draft, "/workspace/rail")
	if err != nil {
		t.Fatalf("NormalizeDraft returned error: %v", err)
	}

	if normalized.RequestVersion != "1" {
		t.Fatalf("expected default request version, got %q", normalized.RequestVersion)
	}

	wantRoot := filepath.Clean("/workspace/rail/target-app")
	if normalized.ProjectRoot != wantRoot {
		t.Fatalf("expected project root %q, got %q", wantRoot, normalized.ProjectRoot)
	}

	if normalized.TaskType != "bug_fix" {
		t.Fatalf("expected normalized task type, got %q", normalized.TaskType)
	}

	if normalized.Goal != "Fix the loading indicator" {
		t.Fatalf("expected trimmed goal, got %q", normalized.Goal)
	}

	wantContext := []string{"profile screen", "retry path"}
	if strings.Join(normalized.Context, "|") != strings.Join(wantContext, "|") {
		t.Fatalf("unexpected context: want %v got %v", wantContext, normalized.Context)
	}

	wantConstraints := []string{"no new deps"}
	if strings.Join(normalized.Constraints, "|") != strings.Join(wantConstraints, "|") {
		t.Fatalf("unexpected constraints: want %v got %v", wantConstraints, normalized.Constraints)
	}

	wantDoD := []string{"spinner clears", "regression test added"}
	if strings.Join(normalized.DefinitionOfDone, "|") != strings.Join(wantDoD, "|") {
		t.Fatalf("unexpected definition_of_done: want %v got %v", wantDoD, normalized.DefinitionOfDone)
	}

	if normalized.RiskTolerance != "low" {
		t.Fatalf("expected default low risk tolerance, got %q", normalized.RiskTolerance)
	}
}

func TestComposeRequestFillsDefaultRiskToleranceByTaskType(t *testing.T) {
	normalized, err := NormalizeDraft(Draft{
		TaskType: "safe_refactor",
		Goal:     "Split build logic into sections",
	}, "/workspace/rail")
	if err != nil {
		t.Fatalf("NormalizeDraft returned error: %v", err)
	}

	if normalized.RiskTolerance != "medium" {
		t.Fatalf("expected medium risk tolerance for safe_refactor, got %q", normalized.RiskTolerance)
	}
}

func TestComposeRequestRejectsMissingGoal(t *testing.T) {
	_, err := NormalizeDraft(Draft{
		TaskType: "bug_fix",
	}, "/workspace/rail")
	if err == nil {
		t.Fatal("expected missing goal to return an error")
	}

	if !strings.Contains(err.Error(), "goal is required") {
		t.Fatalf("expected missing goal error, got %v", err)
	}
}
