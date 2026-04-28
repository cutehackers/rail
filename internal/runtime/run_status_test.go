package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunStatusForLegacyBackendPolicyBlockInfersBlockedActor(t *testing.T) {
	artifactDirectory := t.TempDir()
	runsDirectory := filepath.Join(artifactDirectory, "runs")
	if err := os.MkdirAll(runsDirectory, 0o755); err != nil {
		t.Fatalf("create runs directory: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(artifactDirectory, "execution_report.yaml"),
		[]byte("failure_details:\n  - backend_policy_violation: unexpected_skill_injection in "+filepath.Join(runsDirectory, "01_planner-events.jsonl")+"\n"),
		0o644,
	); err != nil {
		t.Fatalf("write execution report: %v", err)
	}

	status := runStatusForCompletedState(artifactDirectory, State{
		Status:          "blocked_environment",
		CurrentActor:    nil,
		CompletedActors: []string{},
		LastReasonCodes: []string{"backend_policy_violation"},
		ActionHistory:   []string{"block_environment"},
	}, "")

	if status.Phase != "backend_policy" {
		t.Fatalf("expected backend_policy phase for legacy backend block, got %#v", status)
	}
	if status.CurrentActor != "planner" {
		t.Fatalf("expected blocked actor to be inferred from events path, got %#v", status)
	}
	if status.LastSuccessfulActor != "" {
		t.Fatalf("expected no successful actor for legacy pre-planner block, got %#v", status)
	}
	if status.InterruptionKind != "backend_policy_violation" {
		t.Fatalf("expected backend policy interruption, got %#v", status)
	}
}

func TestRunStatusForLegacyBackendPolicyBlockInfersRetryVisitActor(t *testing.T) {
	artifactDirectory := t.TempDir()
	runsDirectory := filepath.Join(artifactDirectory, "runs")
	if err := os.MkdirAll(runsDirectory, 0o755); err != nil {
		t.Fatalf("create runs directory: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(artifactDirectory, "terminal_summary.md"),
		[]byte("backend_policy_violation: unexpected_skill_injection in "+filepath.Join(runsDirectory, "04_generator-visit-02-events.jsonl")+"\n"),
		0o644,
	); err != nil {
		t.Fatalf("write terminal summary: %v", err)
	}

	status := runStatusForCompletedState(artifactDirectory, State{
		Status:          "blocked_environment",
		CompletedActors: []string{"planner", "context_builder", "critic", "generator", "executor", "evaluator"},
		LastReasonCodes: []string{"backend_policy_violation"},
		ActionHistory:   []string{"block_environment"},
	}, "")

	if status.CurrentActor != "generator" {
		t.Fatalf("expected blocked actor to be inferred from retry visit events path, got %#v", status)
	}
	if status.LastSuccessfulActor != "evaluator" {
		t.Fatalf("expected retry visit block to preserve evaluator as last successful actor, got %#v", status)
	}
}

func TestRunStatusForBackendPolicyBlockUsesLatestActorVisit(t *testing.T) {
	status := runStatusForCompletedState(t.TempDir(), State{
		Status:          "blocked_environment",
		BlockedActor:    stringPtr("generator"),
		CompletedActors: []string{"planner", "context_builder", "critic", "generator", "executor", "evaluator", "generator"},
		LastReasonCodes: []string{"backend_policy_violation"},
		ActionHistory:   []string{"route_generator_revision", "block_environment"},
	}, "backend_policy_violation: unexpected_skill_injection in runs/04_generator-visit-02-events.jsonl")

	if status.CurrentActor != "generator" {
		t.Fatalf("expected generator as blocked actor, got %#v", status)
	}
	if status.LastSuccessfulActor != "evaluator" {
		t.Fatalf("expected latest generator visit to report evaluator as previous successful actor, got %#v", status)
	}
}
