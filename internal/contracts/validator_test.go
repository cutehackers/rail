package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateRequestRejectsPathOutsideProjectRoot(t *testing.T) {
	projectRoot := t.TempDir()
	validator, err := NewValidator(projectRoot)
	if err != nil {
		t.Fatalf("NewValidator returned error: %v", err)
	}

	outsideDir := t.TempDir()
	requestPath := filepath.Join(outsideDir, "request.yaml")
	requestBody, err := os.ReadFile(filepath.Join(testRepoRootFromContracts(t), "test", "fixtures", "valid_request.yaml"))
	if err != nil {
		t.Fatalf("failed to read fixture request: %v", err)
	}
	if err := os.WriteFile(requestPath, requestBody, 0o644); err != nil {
		t.Fatalf("failed to write request fixture: %v", err)
	}

	_, err = validator.ValidateRequestFile(requestPath)
	if err == nil {
		t.Fatalf("expected ValidateRequestFile to reject a request outside %q", projectRoot)
	}
	if !strings.Contains(err.Error(), "project root") {
		t.Fatalf("expected project-root confinement error, got %v", err)
	}
}

func TestValidateRequestRejectsCommandLikeValidationTargets(t *testing.T) {
	projectRoot := t.TempDir()
	requestDir := filepath.Join(projectRoot, ".harness", "requests")
	if err := os.MkdirAll(requestDir, 0o755); err != nil {
		t.Fatalf("failed to create request directory: %v", err)
	}
	requestPath := filepath.Join(requestDir, "request.yaml")
	requestBody := `task_type: test_repair
goal: reject command-like validation targets
context:
  validation_targets:
    - go test ./...
constraints: []
definition_of_done:
  - reject command-like validation target
priority: medium
risk_tolerance: low
validation_profile: standard
`
	if err := os.WriteFile(requestPath, []byte(requestBody), 0o644); err != nil {
		t.Fatalf("failed to write request: %v", err)
	}
	validator, err := NewValidator(projectRoot)
	if err != nil {
		t.Fatalf("NewValidator returned error: %v", err)
	}

	_, err = validator.ValidateRequestFile(requestPath)
	if err == nil {
		t.Fatalf("expected ValidateRequestFile to reject command-like validation_targets")
	}
	if !strings.Contains(err.Error(), "validation_targets") {
		t.Fatalf("expected validation_targets error, got %v", err)
	}
}

func TestResolvePathWithinRootCanonicalizesSymlinkedPaths(t *testing.T) {
	projectRoot := t.TempDir()
	requestDir := filepath.Join(projectRoot, ".harness", "requests")
	if err := os.MkdirAll(requestDir, 0o755); err != nil {
		t.Fatalf("failed to create request directory: %v", err)
	}

	symlinkParent := t.TempDir()
	symlinkRoot := filepath.Join(symlinkParent, "workspace")
	if err := os.Symlink(projectRoot, symlinkRoot); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	symlinkPath := filepath.Join(symlinkRoot, ".harness", "requests", "request.yaml")
	resolved, err := ResolvePathWithinRoot(projectRoot, symlinkPath)
	if err != nil {
		t.Fatalf("ResolvePathWithinRoot returned error: %v", err)
	}

	canonicalRoot, err := filepath.EvalSymlinks(projectRoot)
	if err != nil {
		t.Fatalf("failed to canonicalize project root: %v", err)
	}
	want := filepath.Join(canonicalRoot, ".harness", "requests", "request.yaml")
	if resolved != want {
		t.Fatalf("unexpected resolved path: got %q want %q", resolved, want)
	}
}

func TestValidateArtifactFileSupportsLearningAndIntegrationSchemas(t *testing.T) {
	projectRoot := t.TempDir()
	if err := copyDirectory(
		filepath.Join(testRepoRootFromContracts(t), ".harness", "templates"),
		filepath.Join(projectRoot, ".harness", "templates"),
	); err != nil {
		t.Fatalf("failed to copy templates: %v", err)
	}

	validator, err := NewValidator(projectRoot)
	if err != nil {
		t.Fatalf("NewValidator returned error: %v", err)
	}

	tests := []struct {
		name       string
		filePath   string
		schemaName string
		content    string
	}{
		{
			name:       "critic_report",
			filePath:   filepath.Join(".harness", "artifacts", "sample", "critic_report.yaml"),
			schemaName: "critic_report",
			content: `priority_focus: []
missing_requirements: []
risk_hypotheses: []
validation_expectations: []
generator_guardrails: []
blocked_assumptions: []
`,
		},
		{
			name:       "integration_result",
			filePath:   filepath.Join(".harness", "artifacts", "sample", "integration_result.yaml"),
			schemaName: "integration_result",
			content: `summary: handoff
files_changed: []
validation: []
risks: []
follow_up: []
evidence_quality: draft
release_readiness: ready
blocking_issues: []
`,
		},
		{
			name:       "approved_family_memory",
			filePath:   filepath.Join(".harness", "learning", "approved", "feature_addition.yaml"),
			schemaName: "approved_family_memory",
			content: `task_family: feature_addition
task_family_source: task_type
approved_observation: reusable guidance
applicability_conditions: []
evidence_basis: []
guardrail_note: baseline guardrail note
request_compatibility:
  required_context_features: ["feature_addition"]
  goal_must_include_any: ["feature"]
  goal_must_exclude_any: []
  constraint_must_include_any: []
  constraint_must_exclude_any: []
repository_compatibility:
  required_paths_exist: ["cmd/rail/main.go"]
  required_paths_absent: []
latest_family_evidence_expectations:
  lookup_key: feature_addition::task_type
  baseline_approved_memory_ref: .harness/learning/approved/feature_addition.yaml
  required_latest_success_ref: ~
  required_latest_failure_ref: ~
  forbid_any_latest_failure: true
freshness_marker:
  contract_version: 1
  policy_version: 1
  memory_schema_version: 3
  repository_assumptions_ref: .harness/learning/approved/feature_addition.yaml
  repository_state_ref: .harness/learning/approved/feature_addition.yaml
  refreshed_at: "2026-04-16T00:00:00Z"
  freshness_sequence: 1
disposition_history: []
originating_candidate_refs: []
reviewed_user_outcome_feedback_refs: []
`,
		},
		{
			name:       "family_evidence_index",
			filePath:   filepath.Join(".harness", "learning", "family_evidence_index.yaml"),
			schemaName: "family_evidence_index",
			content: `latest_approved_memory_refs_by_family: {}
latest_confirmed_success_refs_by_family: {}
latest_failure_refs_by_family: {}
latest_review_decision_refs_by_family: {}
latest_provisional_candidate_dispositions_by_family: {}
index_generated_at: derived:empty
index_sequence: 0
`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			filePath := filepath.Join(projectRoot, tc.filePath)
			if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
				t.Fatalf("failed to create %s: %v", filePath, err)
			}
			if err := os.WriteFile(filePath, []byte(tc.content), 0o644); err != nil {
				t.Fatalf("failed to write %s: %v", filePath, err)
			}
			if _, err := validator.ValidateArtifactFile(tc.filePath, tc.schemaName); err != nil {
				t.Fatalf("ValidateArtifactFile returned error: %v", err)
			}
		})
	}
}

func TestValidateArtifactFileRejectsUnknownSchemaName(t *testing.T) {
	projectRoot := t.TempDir()
	if err := copyDirectory(
		filepath.Join(testRepoRootFromContracts(t), ".harness", "templates"),
		filepath.Join(projectRoot, ".harness", "templates"),
	); err != nil {
		t.Fatalf("failed to copy templates: %v", err)
	}

	validator, err := NewValidator(projectRoot)
	if err != nil {
		t.Fatalf("NewValidator returned error: %v", err)
	}

	filePath := filepath.Join(projectRoot, ".harness", "artifacts", "sample", "integration_result.yaml")
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatalf("failed to create artifact directory: %v", err)
	}
	if err := os.WriteFile(filePath, []byte("summary: test\nfiles_changed: []\nvalidation: []\nrisks: []\nfollow_up: []\nevidence_quality: draft\nrelease_readiness: ready\nblocking_issues: []\n"), 0o644); err != nil {
		t.Fatalf("failed to write artifact fixture: %v", err)
	}

	if _, err := validator.ValidateArtifactFile(filepath.Join(".harness", "artifacts", "sample", "integration_result.yaml"), "does_not_exist"); err == nil {
		t.Fatalf("expected unknown schema error")
	}
}

func TestValidateArtifactFileRejectsInvalidCriticReport(t *testing.T) {
	projectRoot := t.TempDir()
	if err := copyDirectory(
		filepath.Join(testRepoRootFromContracts(t), ".harness", "templates"),
		filepath.Join(projectRoot, ".harness", "templates"),
	); err != nil {
		t.Fatalf("failed to copy templates: %v", err)
	}

	validator, err := NewValidator(projectRoot)
	if err != nil {
		t.Fatalf("NewValidator returned error: %v", err)
	}

	tests := []struct {
		name    string
		content string
	}{
		{
			name: "missing_required_field",
			content: `priority_focus: []
missing_requirements: []
risk_hypotheses: []
validation_expectations: []
generator_guardrails: []
`,
		},
		{
			name: "extra_property",
			content: `priority_focus: []
missing_requirements: []
risk_hypotheses: []
validation_expectations: []
generator_guardrails: []
blocked_assumptions: []
surprise_field: true
`,
		},
		{
			name: "too_many_items",
			content: `priority_focus:
  - one
  - two
  - three
  - four
  - five
  - six
  - seven
missing_requirements: []
risk_hypotheses: []
validation_expectations: []
generator_guardrails: []
blocked_assumptions: []
`,
		},
		{
			name: "item_exceeds_max_length",
			content: `priority_focus:
  - ` + strings.Repeat("x", 241) + `
missing_requirements: []
risk_hypotheses: []
validation_expectations: []
generator_guardrails: []
blocked_assumptions: []
`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			filePath := filepath.Join(projectRoot, ".harness", "artifacts", "sample", "critic_report.yaml")
			if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
				t.Fatalf("failed to create artifact directory: %v", err)
			}
			if err := os.WriteFile(filePath, []byte(tc.content), 0o644); err != nil {
				t.Fatalf("failed to write critic_report fixture: %v", err)
			}

			if _, err := validator.ValidateArtifactFile(filepath.Join(".harness", "artifacts", "sample", "critic_report.yaml"), "critic_report"); err == nil {
				t.Fatalf("expected invalid critic_report to fail validation")
			}
		})
	}
}

func TestValidateArtifactFileRejectsInvalidCriticReportUsingEmbeddedDefaults(t *testing.T) {
	projectRoot := t.TempDir()

	validator, err := NewValidator(projectRoot)
	if err != nil {
		t.Fatalf("NewValidator returned error: %v", err)
	}

	filePath := filepath.Join(projectRoot, ".harness", "artifacts", "sample", "critic_report.yaml")
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatalf("failed to create artifact directory: %v", err)
	}
	if err := os.WriteFile(filePath, []byte(`priority_focus:
  - one
  - two
  - three
  - four
  - five
  - six
  - seven
missing_requirements: []
risk_hypotheses: []
validation_expectations: []
generator_guardrails: []
blocked_assumptions: []
`), 0o644); err != nil {
		t.Fatalf("failed to write critic_report fixture: %v", err)
	}

	if _, err := validator.ValidateArtifactFile(filepath.Join(".harness", "artifacts", "sample", "critic_report.yaml"), "critic_report"); err == nil {
		t.Fatalf("expected invalid critic_report to fail validation using embedded defaults")
	}
}

func TestValidateArtifactFileAcceptsUnicodeCriticReportAtMaxLength(t *testing.T) {
	projectRoot := t.TempDir()
	if err := copyDirectory(
		filepath.Join(testRepoRootFromContracts(t), ".harness", "templates"),
		filepath.Join(projectRoot, ".harness", "templates"),
	); err != nil {
		t.Fatalf("failed to copy templates: %v", err)
	}

	validator, err := NewValidator(projectRoot)
	if err != nil {
		t.Fatalf("NewValidator returned error: %v", err)
	}

	tests := []struct {
		name      string
		value     string
		wantError bool
	}{
		{
			name:      "at_limit",
			value:     strings.Repeat("한", 240),
			wantError: false,
		},
		{
			name:      "over_limit",
			value:     strings.Repeat("한", 241),
			wantError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			filePath := filepath.Join(projectRoot, ".harness", "artifacts", "sample", "critic_report.yaml")
			if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
				t.Fatalf("failed to create artifact directory: %v", err)
			}
			content := `priority_focus:
  - ` + tc.value + `
missing_requirements: []
risk_hypotheses: []
validation_expectations: []
generator_guardrails: []
blocked_assumptions: []
`
			if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
				t.Fatalf("failed to write critic_report fixture: %v", err)
			}

			_, err := validator.ValidateArtifactFile(filepath.Join(".harness", "artifacts", "sample", "critic_report.yaml"), "critic_report")
			if tc.wantError && err == nil {
				t.Fatalf("expected unicode critic_report to fail validation")
			}
			if !tc.wantError && err != nil {
				t.Fatalf("expected unicode critic_report to pass validation: %v", err)
			}
		})
	}
}

func testRepoRootFromContracts(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("failed to resolve repo root: %v", err)
	}
	return root
}

func copyDirectory(sourcePath, destinationPath string) error {
	if err := os.MkdirAll(destinationPath, 0o755); err != nil {
		return err
	}
	return filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(sourcePath, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destinationPath, relative)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}
