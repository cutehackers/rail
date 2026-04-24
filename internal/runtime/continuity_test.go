package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitialNextActionTargetsFirstActor(t *testing.T) {
	next := initialNextAction(Workflow{
		Actors: []string{"planner", "context_builder"},
	})
	if next["actor"] != "planner" {
		t.Fatalf("unexpected actor: got %#v want planner", next["actor"])
	}
	if next["reason"] != "bootstrap_initialized" {
		t.Fatalf("unexpected reason: got %#v want bootstrap_initialized", next["reason"])
	}
}

func TestInitialFinalAnswerContractBlocksUnsupportedClaims(t *testing.T) {
	contract := initialFinalAnswerContract()
	claims, ok := contract["forbidden_claims"].([]string)
	if !ok {
		t.Fatalf("expected forbidden_claims to be []string, got %T", contract["forbidden_claims"])
	}
	for _, claim := range claims {
		if claim == "claim_tests_passed_without_evidence" {
			return
		}
	}
	t.Fatalf("expected final answer contract to block unverified test claims, got %v", claims)
}

func TestBuildNextActionAfterActorTargetsNextActor(t *testing.T) {
	nextActor := "context_builder"
	next := buildNextActionAfterActor("planner", &nextActor)

	if next["actor"] != "context_builder" {
		t.Fatalf("unexpected actor: got %#v want context_builder", next["actor"])
	}
	if next["reason"] != "actor_completed" {
		t.Fatalf("unexpected reason: got %#v want actor_completed", next["reason"])
	}
	evidence, ok := next["evidence_to_read"].([]string)
	if !ok {
		t.Fatalf("expected evidence_to_read to be []string, got %T", next["evidence_to_read"])
	}
	if !contains(evidence, "plan.yaml") {
		t.Fatalf("expected next action to include plan.yaml evidence, got %v", evidence)
	}
}

func TestBuildNextActionAfterEvaluationTargetsRevisionActor(t *testing.T) {
	decision := "revise"
	next := buildNextActionAfterEvaluation(State{
		Status:          "revising",
		CurrentActor:    stringPtr("generator"),
		LastDecision:    &decision,
		LastReasonCodes: []string{"implementation_patch_invalid"},
		ActionHistory:   []string{"revise_generator"},
	}, "generator")

	if next["actor"] != "generator" {
		t.Fatalf("unexpected actor: got %#v want generator", next["actor"])
	}
	if next["reason"] != "evaluator_requested_revision" {
		t.Fatalf("unexpected reason: got %#v want evaluator_requested_revision", next["reason"])
	}
	evidence, ok := next["evidence_to_read"].([]string)
	if !ok {
		t.Fatalf("expected evidence_to_read to be []string, got %T", next["evidence_to_read"])
	}
	if !contains(evidence, "evaluation_result.yaml") {
		t.Fatalf("expected next action to include evaluation_result.yaml evidence, got %v", evidence)
	}
}

func TestAppendWorkLedgerEntryAddsReadableSection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "work_ledger.md")
	if err := os.WriteFile(path, []byte("# Work Ledger\n"), 0o644); err != nil {
		t.Fatalf("failed to seed work ledger: %v", err)
	}

	if err := appendWorkLedgerEntry(path, "Actor completed: planner", []string{
		"status: in_progress",
		"next actor: context_builder",
	}); err != nil {
		t.Fatalf("appendWorkLedgerEntry returned error: %v", err)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read work ledger: %v", err)
	}
	for _, fragment := range []string{
		"## Actor completed: planner",
		"- status: in_progress",
		"- next actor: context_builder",
	} {
		if !strings.Contains(string(body), fragment) {
			t.Fatalf("expected work ledger to contain %q, got:\n%s", fragment, string(body))
		}
	}
}
