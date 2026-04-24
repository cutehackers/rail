package runtime

import "testing"

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
