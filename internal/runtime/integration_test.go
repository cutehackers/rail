package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rail/internal/contracts"
)

func TestRunnerIntegrateRejectsNonPassArtifacts(t *testing.T) {
	repoRoot := testRepoRoot(t)
	runner, err := NewRunner(repoRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}

	_, err = runner.Integrate(filepath.Join(repoRoot, "test", "fixtures", "standard_route", "split_task"), "")
	if err == nil {
		t.Fatalf("expected integrate to reject non-pass artifacts")
	}
	if !strings.Contains(err.Error(), "integrator requires evaluator decision `pass`") {
		t.Fatalf("unexpected integrate error: %v", err)
	}
}

func TestRunnerIntegrateNormalizesBlockedIntegratorOutput(t *testing.T) {
	repoRoot := testRepoRoot(t)
	projectRoot := filepath.Join(repoRoot, "examples", "smoke-target")
	runner, err := NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}

	taskID := "runtime-integrate-blocked-test"
	artifactPath := filepath.Join(projectRoot, ".harness", "artifacts", taskID)
	_ = os.RemoveAll(artifactPath)
	t.Cleanup(func() {
		_ = os.RemoveAll(artifactPath)
	})

	if _, err := runner.Run(filepath.Join(projectRoot, ".harness", "requests", "valid_request.yaml"), taskID); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if _, err := runner.Execute(artifactPath); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	fakeBin := t.TempDir()
	fakeCodex := filepath.Join(fakeBin, "codex")
	if err := os.WriteFile(fakeCodex, []byte(`#!/usr/bin/env python3
import json
import os
import sys

output_path = None
for index, value in enumerate(sys.argv):
    if value == "--output-last-message" and index + 1 < len(sys.argv):
        output_path = sys.argv[index + 1]
        break

os.makedirs(os.path.dirname(output_path), exist_ok=True)
with open(output_path, "w", encoding="utf-8") as handle:
    json.dump({
        "summary": "Blocked handoff.",
        "files_changed": [],
        "validation": [],
        "risks": [],
        "follow_up": [],
        "evidence_quality": "adequate",
        "release_readiness": "conditional",
        "blocking_issues": ["Manual approval is still missing."]
    }, handle)
`), 0o755); err != nil {
		t.Fatalf("failed to write fake codex: %v", err)
	}

	originalPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", fakeBin+string(os.PathListSeparator)+originalPath); err != nil {
		t.Fatalf("failed to set PATH: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Setenv("PATH", originalPath)
	})

	if _, err := runner.Integrate(artifactPath, projectRoot); err != nil {
		t.Fatalf("Integrate returned error: %v", err)
	}

	value, err := contracts.ReadYAMLFile(filepath.Join(artifactPath, "integration_result.yaml"))
	if err != nil {
		t.Fatalf("failed to read integration_result.yaml: %v", err)
	}
	result, err := contracts.AsMap(value, "integration_result")
	if err != nil {
		t.Fatalf("failed to decode integration_result.yaml: %v", err)
	}
	if got := result["release_readiness"]; got != "blocked" {
		t.Fatalf("unexpected release_readiness: got %v want blocked", got)
	}
	blockingIssues := stringList(result["blocking_issues"])
	if len(blockingIssues) != 1 || blockingIssues[0] != "Manual approval is still missing." {
		t.Fatalf("unexpected blocking_issues: %v", blockingIssues)
	}
}
