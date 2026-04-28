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
	resolvedFakeCodex, err := filepath.EvalSymlinks(fakeCodex)
	if err != nil {
		t.Fatalf("failed to resolve fake codex: %v", err)
	}
	if err := os.WriteFile(filepath.Join(fakeBin, ".rail-internal-test-codex"), []byte(filepath.Clean(resolvedFakeCodex)+"\n"), 0o600); err != nil {
		t.Fatalf("failed to write fake codex marker: %v", err)
	}

	originalPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", fakeBin+string(os.PathListSeparator)+originalPath); err != nil {
		t.Fatalf("failed to set PATH: %v", err)
	}
	t.Setenv("RAIL_INTERNAL_TEST_ALLOW_UNTRUSTED_CODEX_PATH", "rail-internal-tests-only")
	t.Setenv("RAIL_INTERNAL_TEST_CODEX_PATH", resolvedFakeCodex)
	t.Cleanup(func() {
		_ = os.Setenv("PATH", originalPath)
	})
	t.Setenv("RAIL_CODEX_AUTH_HOME", testRailCodexAuthHome(t))

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

func TestIntegrateRejectsUnexpectedSkillInjection(t *testing.T) {
	projectRoot, requestPath := prepareRealProject(t)
	installFakeCodexForRealMode(t, projectRoot)
	t.Setenv("RAIL_TEST_CODEX_VIOLATION_ACTOR", "integrator")

	runner, err := NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}

	artifactPath, err := runner.Run(requestPath, "go-real-integrator-skill-injection")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if _, err := runner.Execute(artifactPath); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	_, err = runner.Integrate(artifactPath, projectRoot)
	if err == nil {
		t.Fatalf("expected Integrate to reject unexpected skill injection")
	}
	for _, fragment := range []string{"backend_policy_violation", "unexpected_skill_injection", "integrator-events.jsonl"} {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("expected integrate error to contain %q, got %v", fragment, err)
		}
	}
	if _, statErr := os.Stat(filepath.Join(artifactPath, "integration_result.yaml")); !os.IsNotExist(statErr) {
		t.Fatalf("expected integration_result.yaml not to be written after policy violation, stat err=%v", statErr)
	}

	runStatus, err := ReadRunStatus(artifactPath)
	if err != nil {
		t.Fatalf("expected run_status.yaml to be readable after integrator policy violation: %v", err)
	}
	if runStatus.Status != "interrupted" || runStatus.Phase != "backend_policy" || runStatus.CurrentActor != "integrator" {
		t.Fatalf("expected integrator policy violation to overwrite pass status, got %#v", runStatus)
	}
	if runStatus.LastSuccessfulActor != "evaluator" {
		t.Fatalf("expected evaluator to remain last successful actor, got %#v", runStatus)
	}
	if runStatus.InterruptionKind != "backend_policy_violation" {
		t.Fatalf("expected backend policy interruption, got %#v", runStatus)
	}
	for _, fragment := range []string{"backend_policy_violation", "unexpected_skill_injection", "integrator-events.jsonl"} {
		if !strings.Contains(runStatus.Message, fragment) {
			t.Fatalf("expected run status message to contain %q, got %#v", fragment, runStatus)
		}
	}

	result, err := ProjectHarnessResultForArtifact(projectRoot, artifactPath)
	if err != nil {
		t.Fatalf("expected harness result projection after integrator policy violation: %v", err)
	}
	if result.Status != "interrupted" || result.Phase != "backend_policy" || result.CurrentActor != "integrator" {
		t.Fatalf("expected result projection to show integrator interruption, got %#v", result)
	}
	if result.InterruptionKind != "backend_policy_violation" {
		t.Fatalf("expected result projection backend policy interruption, got %#v", result)
	}
	if result.Terminal {
		t.Fatalf("expected integrator interruption not to reuse stale terminal pass summary, got %#v", result)
	}
}

func TestIntegrateFailsForInvalidIntegratorProfile(t *testing.T) {
	projectRoot, requestPath := prepareRealProject(t)
	_ = installFakeCodexForRealMode(t, projectRoot)

	runner, err := NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}

	artifactPath, err := runner.Run(requestPath, "go-real-integrator-invalid-profile")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if _, err := runner.Execute(artifactPath); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	profilesPath := filepath.Join(projectRoot, ".harness", "supervisor", "actor_profiles.yaml")
	invalidProfiles := `version: 1
actors:
  planner: { model: gpt-5.4, reasoning: high }
  context_builder: { model: gpt-5.4-mini, reasoning: high }
  critic: { model: gpt-5.4, reasoning: high }
  generator: { model: gpt-5.4, reasoning: high }
  evaluator: { model: gpt-5.4, reasoning: high }
  integrator: { model: gpt-5.4, reasoning: critical }
`
	if err := os.WriteFile(profilesPath, []byte(invalidProfiles), 0o644); err != nil {
		t.Fatalf("failed to write invalid actor profiles: %v", err)
	}

	_, err = runner.Integrate(artifactPath, projectRoot)
	if err == nil {
		t.Fatalf("expected Integrate to fail for invalid integrator profile")
	}
	if !strings.Contains(err.Error(), "unsupported reasoning") {
		t.Fatalf("expected invalid integrator profile error, got %v", err)
	}
}

func TestIntegrateUsesOverrideRootActorProfiles(t *testing.T) {
	projectRoot, requestPath := prepareRealProject(t)
	_ = installFakeCodexForRealMode(t, projectRoot)

	runner, err := NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}

	artifactPath, err := runner.Run(requestPath, "go-real-integrator-override-profile")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if _, err := runner.Execute(artifactPath); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	overrideRoot := t.TempDir()
	for _, relPath := range []string{
		filepath.Join(".harness", "supervisor"),
		"feature",
	} {
		if err := os.MkdirAll(filepath.Join(overrideRoot, relPath), 0o755); err != nil {
			t.Fatalf("failed to create %q: %v", relPath, err)
		}
	}
	overrideProfiles := `version: 1
actors:
  planner: { model: gpt-5.4, reasoning: high }
  context_builder: { model: gpt-5.4-mini, reasoning: high }
  critic: { model: gpt-5.4, reasoning: high }
  generator: { model: gpt-5.4, reasoning: high }
  evaluator: { model: gpt-5.4, reasoning: high }
  integrator: { model: gpt-5.4-override-integrator, reasoning: xhigh }
`
	if err := os.WriteFile(filepath.Join(overrideRoot, ".harness", "supervisor", "actor_profiles.yaml"), []byte(overrideProfiles), 0o644); err != nil {
		t.Fatalf("failed to write override actor profiles: %v", err)
	}
	if err := os.WriteFile(filepath.Join(overrideRoot, "feature", "ready.go"), []byte("package feature\n\nfunc Ready() bool { return true }\n"), 0o644); err != nil {
		t.Fatalf("failed to write override ready.go: %v", err)
	}

	_, err = runner.Integrate(artifactPath, overrideRoot)
	if err != nil {
		t.Fatalf("Integrate returned error: %v", err)
	}

	actorLog, err := os.ReadFile(filepath.Join(overrideRoot, ".actor-log"))
	if err != nil {
		t.Fatalf("failed to read fake codex actor log: %v", err)
	}
	if !strings.Contains(string(actorLog), "integrator|gpt-5.4-override-integrator|xhigh") {
		t.Fatalf("expected integrator to use override-root actor profile, got:\n%s", string(actorLog))
	}
}

func TestIntegrateUsesWorkflowProjectRootActorProfiles(t *testing.T) {
	targetRoot, requestPath := prepareRealProject(t)
	_ = installFakeCodexForRealMode(t, targetRoot)

	targetProfiles := `version: 1
actors:
  planner: { model: gpt-5.4, reasoning: high }
  context_builder: { model: gpt-5.4-mini, reasoning: high }
  critic: { model: gpt-5.4, reasoning: high }
  generator: { model: gpt-5.4, reasoning: high }
  evaluator: { model: gpt-5.4, reasoning: high }
  integrator: { model: gpt-5.4-workflow-integrator, reasoning: xhigh }
`
	if err := os.WriteFile(filepath.Join(targetRoot, ".harness", "supervisor", "actor_profiles.yaml"), []byte(targetProfiles), 0o644); err != nil {
		t.Fatalf("failed to write target actor profiles: %v", err)
	}

	targetRunner, err := NewRunner(targetRoot)
	if err != nil {
		t.Fatalf("NewRunner(targetRoot) returned error: %v", err)
	}
	artifactPath, err := targetRunner.Run(requestPath, "go-real-integrator-cross-root")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if _, err := targetRunner.Execute(artifactPath); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	controlRoot := t.TempDir()
	for _, relPath := range []string{filepath.Join(".harness", "supervisor"), filepath.Join(".harness", "artifacts")} {
		if err := os.MkdirAll(filepath.Join(controlRoot, relPath), 0o755); err != nil {
			t.Fatalf("failed to create %q: %v", relPath, err)
		}
	}
	controlProfiles := `version: 1
actors:
  planner: { model: bad-control-planner, reasoning: critical }
`
	if err := os.WriteFile(filepath.Join(controlRoot, ".harness", "supervisor", "actor_profiles.yaml"), []byte(controlProfiles), 0o644); err != nil {
		t.Fatalf("failed to write control-root actor profiles: %v", err)
	}
	controlArtifactPath := filepath.Join(controlRoot, ".harness", "artifacts", "go-real-integrator-cross-root")
	if err := copyDirectory(artifactPath, controlArtifactPath); err != nil {
		t.Fatalf("failed to copy artifact into control root: %v", err)
	}

	controlRunner, err := NewRunner(controlRoot)
	if err != nil {
		t.Fatalf("NewRunner(controlRoot) returned error: %v", err)
	}

	_, err = controlRunner.Integrate(filepath.ToSlash(filepath.Join(".harness", "artifacts", "go-real-integrator-cross-root")), "")
	if err != nil {
		t.Fatalf("Integrate returned error: %v", err)
	}

	actorLog, err := os.ReadFile(filepath.Join(targetRoot, ".actor-log"))
	if err != nil {
		t.Fatalf("failed to read fake codex actor log: %v", err)
	}
	if !strings.Contains(string(actorLog), "integrator|gpt-5.4-workflow-integrator|xhigh") {
		t.Fatalf("expected integrator to use workflow project-root actor profile, got:\n%s", string(actorLog))
	}
}
