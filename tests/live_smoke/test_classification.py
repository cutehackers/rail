from __future__ import annotations

from pathlib import Path

import pytest
from pydantic import ValidationError

from rail.actor_runtime.runtime import ActorResult
from rail.live_smoke.classification import (
    LiveSmokeClassification,
    classify_actor_result,
)
from rail.live_smoke.models import LiveSmokeActor, OwningSurface, SymptomClass


def _actor_result(
    *,
    status: str = "succeeded",
    structured_output: dict[str, object] | None = None,
    blocked_category: str | None = None,
) -> ActorResult:
    return ActorResult(
        status=status,
        structured_output=structured_output or {},
        events_ref=Path("runs/attempt-0001/context_builder.events.jsonl"),
        runtime_evidence_ref=Path("runs/attempt-0001/context_builder.runtime_evidence.json"),
        blocked_category=blocked_category,
    )


def test_live_smoke_classification_rejects_unknown_fields() -> None:
    with pytest.raises(ValidationError):
        LiveSmokeClassification(unexpected=True)


def test_classifies_grep_policy_violation_as_prompt_or_contract_gap() -> None:
    result = _actor_result(
        status="interrupted",
        structured_output={"error": "shell executable is not allowed: grep"},
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


def test_classifies_environment_block_as_readiness_failure() -> None:
    result = _actor_result(
        status="interrupted",
        structured_output={"error": "missing provider credentials"},
        blocked_category="environment",
    )

    classification = classify_actor_result(
        LiveSmokeActor.CONTEXT_BUILDER,
        result,
        behavior_error=None,
    )

    assert classification.symptom_class == SymptomClass.READINESS_FAILURE
    assert classification.owning_surface == OwningSurface.OPERATOR_ENVIRONMENT
    assert classification.repair_proposal is None


def test_classifies_runtime_validation_error_as_schema_mismatch() -> None:
    result = _actor_result(
        status="failed",
        structured_output={"error": "validation failed for structured output"},
        blocked_category="runtime",
    )

    classification = classify_actor_result(
        LiveSmokeActor.CONTEXT_BUILDER,
        result,
        behavior_error=None,
    )

    assert classification.symptom_class == SymptomClass.SCHEMA_MISMATCH
    assert classification.owning_surface == OwningSurface.ACTOR_PROMPT
    assert classification.repair_proposal is None


def test_classifies_behavior_error_without_repair_when_output_is_missing() -> None:
    result = _actor_result(
        structured_output={"relevant_files": []},
    )

    classification = classify_actor_result(
        LiveSmokeActor.CONTEXT_BUILDER,
        result,
        behavior_error="context_builder output must include non-empty relevant_files",
    )

    assert classification.symptom_class == SymptomClass.BEHAVIOR_SMOKE_FAILURE
    assert classification.owning_surface == OwningSurface.ACTOR_PROMPT


def test_classifies_succeeded_result_without_behavior_error_as_no_symptom() -> None:
    result = _actor_result(
        structured_output={
            "relevant_files": [{"path": "README.md", "why": "entry point"}],
        },
    )

    classification = classify_actor_result(
        LiveSmokeActor.CONTEXT_BUILDER,
        result,
        behavior_error=None,
    )

    assert classification.symptom_class is None
    assert classification.owning_surface is None
    assert classification.repair_proposal is None


def test_classifies_unblocked_failed_result_as_unknown_failure() -> None:
    result = _actor_result(
        status="failed",
        structured_output={"error": "actor failed without blocked category"},
    )

    classification = classify_actor_result(
        LiveSmokeActor.CONTEXT_BUILDER,
        result,
        behavior_error=None,
    )

    assert classification.symptom_class == SymptomClass.UNKNOWN_FAILURE
    assert classification.owning_surface == OwningSurface.UNKNOWN
    assert classification.repair_proposal is None
