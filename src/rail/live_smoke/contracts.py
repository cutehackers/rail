from __future__ import annotations

from collections.abc import Mapping

from rail.live_smoke.models import LiveSmokeActor

V1_LIVE_SMOKE_ACTORS = (
    LiveSmokeActor.PLANNER,
    LiveSmokeActor.CONTEXT_BUILDER,
)

_PLANNER_REQUIRED_FIELDS = (
    "summary",
    "substeps",
    "risks",
    "acceptance_criteria_refined",
)
_CONTEXT_BUILDER_REQUIRED_FIELDS = (
    "relevant_files",
    "repo_patterns",
    "test_patterns",
    "forbidden_changes",
    "implementation_hints",
)
_CONTEXT_BUILDER_NON_EMPTY_FIELDS = (
    "relevant_files",
    "repo_patterns",
    "forbidden_changes",
    "implementation_hints",
)


def evaluate_behavior_smoke(
    actor: LiveSmokeActor,
    output: Mapping[str, object],
) -> str | None:
    if actor == LiveSmokeActor.PLANNER:
        return _evaluate_required_fields(
            actor=actor,
            output=output,
            required_fields=_PLANNER_REQUIRED_FIELDS,
        )

    if actor == LiveSmokeActor.CONTEXT_BUILDER:
        missing_field_error = _evaluate_required_fields(
            actor=actor,
            output=output,
            required_fields=_CONTEXT_BUILDER_REQUIRED_FIELDS,
        )
        if missing_field_error is not None:
            return missing_field_error

        for field_name in _CONTEXT_BUILDER_NON_EMPTY_FIELDS:
            value = output[field_name]
            if not isinstance(value, list) or not value:
                return f"{actor.value} output must include non-empty {field_name}"

    return None


def _evaluate_required_fields(
    *,
    actor: LiveSmokeActor,
    output: Mapping[str, object],
    required_fields: tuple[str, ...],
) -> str | None:
    for field_name in required_fields:
        if field_name not in output:
            return f"{actor.value} output must include {field_name}"
    return None
