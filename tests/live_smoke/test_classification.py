from __future__ import annotations

from pathlib import Path

from rail.actor_runtime.runtime import ActorResult
from rail.live_smoke.classification import classify_actor_result
from rail.live_smoke.models import LiveSmokeActor, OwningSurface, SymptomClass


def test_classifies_grep_policy_violation_as_prompt_or_contract_gap() -> None:
    result = ActorResult(
        status="interrupted",
        structured_output={"error": "shell executable is not allowed: grep"},
        events_ref=Path("runs/attempt-0001/context_builder.events.jsonl"),
        runtime_evidence_ref=Path("runs/attempt-0001/context_builder.runtime_evidence.json"),
        blocked_category="policy",
    )

    classification = classify_actor_result(
        LiveSmokeActor.CONTEXT_BUILDER,
        result,
        behavior_error=None,
    )

    assert classification.symptom_class == SymptomClass.POLICY_VIOLATION
    assert classification.owning_surface == OwningSurface.RUNTIME_CONTRACT
    assert classification.repair_proposal is not None
    assert classification.repair_proposal.preserves_fail_closed_policy is True


def test_classifies_behavior_error_without_repair_when_output_is_missing() -> None:
    result = ActorResult(
        status="succeeded",
        structured_output={"relevant_files": []},
        events_ref=Path("runs/attempt-0001/context_builder.events.jsonl"),
        runtime_evidence_ref=Path("runs/attempt-0001/context_builder.runtime_evidence.json"),
    )

    classification = classify_actor_result(
        LiveSmokeActor.CONTEXT_BUILDER,
        result,
        behavior_error="context_builder output must include non-empty relevant_files",
    )

    assert classification.symptom_class == SymptomClass.BEHAVIOR_SMOKE_FAILURE
    assert classification.owning_surface == OwningSurface.ACTOR_PROMPT
