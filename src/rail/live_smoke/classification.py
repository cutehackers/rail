from __future__ import annotations

from pydantic import BaseModel, ConfigDict

from rail.actor_runtime.runtime import ActorResult
from rail.live_smoke.models import (
    LiveSmokeActor,
    OwningSurface,
    RepairProposal,
    SymptomClass,
)

_SHELL_EXECUTABLE_NOT_ALLOWED = "shell executable is not allowed"
_VALIDATION_ERROR_MARKER = "validation"


class LiveSmokeClassification(BaseModel):
    model_config = ConfigDict(extra="forbid")

    symptom_class: SymptomClass | None = None
    owning_surface: OwningSurface | None = None
    repair_proposal: RepairProposal | None = None


def classify_actor_result(
    actor: LiveSmokeActor,
    result: ActorResult,
    *,
    behavior_error: str | None = None,
) -> LiveSmokeClassification:
    error_text = _extract_error_text(result)

    if result.blocked_category == "policy":
        owning_surface = _policy_owning_surface(error_text)
        return LiveSmokeClassification(
            symptom_class=SymptomClass.POLICY_VIOLATION,
            owning_surface=owning_surface,
            repair_proposal=_policy_repair_proposal(actor, owning_surface, error_text),
        )

    if result.blocked_category == "environment":
        return LiveSmokeClassification(
            symptom_class=SymptomClass.READINESS_FAILURE,
            owning_surface=OwningSurface.OPERATOR_ENVIRONMENT,
        )

    if result.blocked_category == "runtime":
        return LiveSmokeClassification(
            symptom_class=_runtime_symptom_class(error_text),
            owning_surface=OwningSurface.ACTOR_PROMPT,
        )

    if behavior_error is not None:
        return LiveSmokeClassification(
            symptom_class=SymptomClass.BEHAVIOR_SMOKE_FAILURE,
            owning_surface=OwningSurface.ACTOR_PROMPT,
        )

    return LiveSmokeClassification()


def _extract_error_text(result: ActorResult) -> str:
    error = result.structured_output.get("error")
    if isinstance(error, str):
        return error
    return ""


def _policy_owning_surface(error_text: str) -> OwningSurface:
    if _SHELL_EXECUTABLE_NOT_ALLOWED in error_text:
        return OwningSurface.RUNTIME_CONTRACT
    return OwningSurface.RUNTIME_CONTRACT


def _policy_repair_proposal(
    actor: LiveSmokeActor,
    owning_surface: OwningSurface,
    error_text: str,
) -> RepairProposal | None:
    if _SHELL_EXECUTABLE_NOT_ALLOWED not in error_text:
        return None

    return RepairProposal(
        owning_surface=owning_surface,
        file_paths=[
            "src/rail/actor_runtime/runtime.py",
            f".harness/actors/{actor.value}.md",
        ],
        summary="Align live smoke actor instructions with runtime shell policy.",
        preserves_fail_closed_policy=True,
    )


def _runtime_symptom_class(error_text: str) -> SymptomClass:
    if _VALIDATION_ERROR_MARKER in error_text.lower():
        return SymptomClass.SCHEMA_MISMATCH
    return SymptomClass.UNKNOWN_FAILURE
