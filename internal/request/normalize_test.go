package request

import (
	"strings"
	"testing"
)

func TestComposeRequestNormalizesDraftIntoCanonicalRequestShape(t *testing.T) {
	draft, err := DecodeDraft(strings.NewReader(`{
		"request_version": " ",
		"project_root": "/tmp/target-app",
		"task_type": " bug_fix ",
		"goal": "  Fix the loading indicator  ",
		"context": {
			"feature": " profile ",
			"suspected_files": [" internal/profile/service.go ", ""],
			"related_files": [" cmd/rail/main.go "],
			"validation_roots": [" internal/profile "],
			"validation_targets": [" internal/profile/service_test.go "]
		},
		"constraints": [" no new deps ", " "],
		"definition_of_done": [" spinner clears ", " regression test added "]
	}`))
	if err != nil {
		t.Fatalf("DecodeDraft returned error: %v", err)
	}

	materialized, err := NormalizeDraft(draft)
	if err != nil {
		t.Fatalf("NormalizeDraft returned error: %v", err)
	}

	if materialized.ProjectRoot != "/tmp/target-app" {
		t.Fatalf("expected explicit project root, got %q", materialized.ProjectRoot)
	}

	request := materialized.Request
	if request.TaskType != "bug_fix" {
		t.Fatalf("expected normalized task type, got %q", request.TaskType)
	}
	if request.Goal != "Fix the loading indicator" {
		t.Fatalf("expected trimmed goal, got %q", request.Goal)
	}
	if request.Context.Feature != "profile" {
		t.Fatalf("expected trimmed feature, got %q", request.Context.Feature)
	}
	if got, want := strings.Join(request.Context.SuspectedFiles, "|"), "internal/profile/service.go"; got != want {
		t.Fatalf("unexpected suspected_files: want %q got %q", want, got)
	}
	if got, want := strings.Join(request.Context.RelatedFiles, "|"), "cmd/rail/main.go"; got != want {
		t.Fatalf("unexpected related_files: want %q got %q", want, got)
	}
	if got, want := strings.Join(request.Context.ValidationRoots, "|"), "internal/profile"; got != want {
		t.Fatalf("unexpected validation_roots: want %q got %q", want, got)
	}
	if got, want := strings.Join(request.Context.ValidationTargets, "|"), "internal/profile/service_test.go"; got != want {
		t.Fatalf("unexpected validation_targets: want %q got %q", want, got)
	}
	if request.Priority != "medium" {
		t.Fatalf("expected default priority medium, got %q", request.Priority)
	}
	if request.ValidationProfile != "standard" {
		t.Fatalf("expected default validation_profile standard, got %q", request.ValidationProfile)
	}
	if request.RiskTolerance != "low" {
		t.Fatalf("expected default low risk tolerance, got %q", request.RiskTolerance)
	}
}

func TestComposeRequestFillsDefaultRiskToleranceByTaskType(t *testing.T) {
	materialized, err := NormalizeDraft(Draft{
		ProjectRoot: "/tmp/target-app",
		TaskType:    "safe_refactor",
		Goal:        "Split build logic into sections",
	})
	if err != nil {
		t.Fatalf("NormalizeDraft returned error: %v", err)
	}

	if materialized.Request.RiskTolerance != "medium" {
		t.Fatalf("expected medium risk tolerance for safe_refactor, got %q", materialized.Request.RiskTolerance)
	}
}

func TestComposeRequestNormalizesExplicitSmokeValidationProfile(t *testing.T) {
	materialized, err := NormalizeDraft(Draft{
		ProjectRoot:       "/tmp/target-app",
		TaskType:          "bug_fix",
		Goal:              "Verify smoke routing",
		ValidationProfile: "smoke",
	})
	if err != nil {
		t.Fatalf("NormalizeDraft returned error: %v", err)
	}

	if materialized.Request.ValidationProfile != "smoke" {
		t.Fatalf("expected smoke validation_profile, got %q", materialized.Request.ValidationProfile)
	}
}

func TestComposeRequestNormalizesRealValidationProfileAlias(t *testing.T) {
	materialized, err := NormalizeDraft(Draft{
		ProjectRoot:       "/tmp/target-app",
		TaskType:          "bug_fix",
		Goal:              "Verify real routing",
		ValidationProfile: "real",
	})
	if err != nil {
		t.Fatalf("NormalizeDraft returned error: %v", err)
	}

	if materialized.Request.ValidationProfile != "standard" {
		t.Fatalf("expected real alias to normalize to standard, got %q", materialized.Request.ValidationProfile)
	}
}

func TestComposeRequestRejectsMissingGoal(t *testing.T) {
	_, err := NormalizeDraft(Draft{
		ProjectRoot: "/tmp/target-app",
		TaskType:    "bug_fix",
	})
	if err == nil {
		t.Fatal("expected missing goal to return an error")
	}
	if !strings.Contains(err.Error(), "goal is required") {
		t.Fatalf("expected missing goal error, got %v", err)
	}
}

func TestComposeRequestRejectsMissingProjectRoot(t *testing.T) {
	_, err := NormalizeDraft(Draft{
		TaskType: "bug_fix",
		Goal:     "Fix the bug",
	})
	if err == nil {
		t.Fatal("expected missing project_root to return an error")
	}
	if !strings.Contains(err.Error(), "project_root is required") {
		t.Fatalf("expected missing project_root error, got %v", err)
	}
}

func TestComposeRequestRejectsRelativeProjectRoot(t *testing.T) {
	_, err := NormalizeDraft(Draft{
		RequestVersion: "1",
		ProjectRoot:    "./target-app",
		TaskType:       "bug_fix",
		Goal:           "Fix the bug",
	})
	if err == nil {
		t.Fatal("expected relative project_root to return an error")
	}
	if !strings.Contains(err.Error(), "project_root must be an absolute path") {
		t.Fatalf("expected absolute-path error, got %v", err)
	}
}

func TestComposeRequestRejectsInvalidRiskTolerance(t *testing.T) {
	_, err := NormalizeDraft(Draft{
		ProjectRoot:   "/tmp/target-app",
		TaskType:      "bug_fix",
		Goal:          "Fix the bug",
		RiskTolerance: "aggressive",
	})
	if err == nil {
		t.Fatal("expected invalid risk_tolerance to return an error")
	}
	if !strings.Contains(err.Error(), "unsupported risk_tolerance") {
		t.Fatalf("expected invalid risk_tolerance error, got %v", err)
	}
}

func TestComposeRequestRejectsUnsupportedValidationProfile(t *testing.T) {
	_, err := NormalizeDraft(Draft{
		ProjectRoot:       "/tmp/target-app",
		TaskType:          "bug_fix",
		Goal:              "Fix the bug",
		ValidationProfile: "fast",
	})
	if err == nil {
		t.Fatal("expected invalid validation_profile to return an error")
	}
	if !strings.Contains(err.Error(), "unsupported validation_profile") {
		t.Fatalf("expected invalid validation_profile error, got %v", err)
	}
}

func TestComposeRequestRejectsUnknownDraftFields(t *testing.T) {
	_, err := DecodeDraft(strings.NewReader(`{
		"project_root": "/tmp/target-app",
		"task_type": "bug_fix",
		"goal": "Fix the bug",
		"unknown_field": true
	}`))
	if err == nil {
		t.Fatal("expected unknown field rejection")
	}
	if !strings.Contains(err.Error(), "field unknown_field not found") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestComposeRequestRejectsMultipleDraftDocuments(t *testing.T) {
	_, err := DecodeDraft(strings.NewReader(`---
project_root: /tmp/target-app
task_type: bug_fix
goal: Fix the bug
---
project_root: /tmp/another-target
task_type: bug_fix
goal: Second draft
`))
	if err == nil {
		t.Fatal("expected multi-document draft rejection")
	}
	if !strings.Contains(err.Error(), "multiple draft documents are not allowed") {
		t.Fatalf("expected multi-document error, got %v", err)
	}
}

func TestComposeRequestRejectsUnsupportedRequestVersion(t *testing.T) {
	_, err := NormalizeDraft(Draft{
		RequestVersion: "2",
		ProjectRoot:    "/tmp/target-app",
		TaskType:       "bug_fix",
		Goal:           "Fix the bug",
	})
	if err == nil {
		t.Fatal("expected unsupported request_version to return an error")
	}
	if !strings.Contains(err.Error(), "unsupported request_version") {
		t.Fatalf("expected request_version error, got %v", err)
	}
}
