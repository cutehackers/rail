from __future__ import annotations

from collections.abc import Mapping
from pathlib import Path

import yaml

from rail.live_smoke.models import LiveSmokeActor
from rail.live_smoke.seeds import LiveSmokeSeed
from rail.workspace.isolation import tree_digest
from rail.workspace.patch_bundle import PatchBundle, validate_patch_bundle

V1_LIVE_SMOKE_ACTORS = (
    LiveSmokeActor.PLANNER,
    LiveSmokeActor.CONTEXT_BUILDER,
)
LIVE_SMOKE_ACTORS = (
    LiveSmokeActor.PLANNER,
    LiveSmokeActor.CONTEXT_BUILDER,
    LiveSmokeActor.CRITIC,
    LiveSmokeActor.GENERATOR,
    LiveSmokeActor.EXECUTOR,
    LiveSmokeActor.EVALUATOR,
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
_CRITIC_REQUIRED_FIELDS = (
    "priority_focus",
    "missing_requirements",
    "risk_hypotheses",
    "validation_expectations",
    "generator_guardrails",
    "blocked_assumptions",
)
_CRITIC_NON_EMPTY_FIELDS = (
    "priority_focus",
    "validation_expectations",
    "generator_guardrails",
)
_GENERATOR_REQUIRED_FIELDS = (
    "changed_files",
    "patch_summary",
    "tests_added_or_updated",
    "known_limits",
)
_EXECUTOR_REQUIRED_FIELDS = (
    "format",
    "analyze",
    "tests",
    "failure_details",
    "logs",
)
_EVALUATOR_REQUIRED_FIELDS = (
    "decision",
    "evaluated_input_digest",
    "findings",
    "reason_codes",
    "quality_confidence",
)


def evaluate_behavior_smoke(
    actor: LiveSmokeActor,
    output: Mapping[str, object],
    *,
    seed: LiveSmokeSeed | None = None,
    target_root: Path | None = None,
    artifact_dir: Path | None = None,
    invocation_input: Mapping[str, object] | None = None,
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

    if actor == LiveSmokeActor.CRITIC:
        return _evaluate_non_empty_fields(
            actor=actor,
            output=output,
            required_fields=_CRITIC_REQUIRED_FIELDS,
            non_empty_fields=_CRITIC_NON_EMPTY_FIELDS,
        )

    if actor == LiveSmokeActor.GENERATOR:
        missing_field_error = _evaluate_required_fields(
            actor=actor,
            output=output,
            required_fields=_GENERATOR_REQUIRED_FIELDS,
        )
        if missing_field_error is not None:
            return missing_field_error
        return _evaluate_generator_patch_bundle(
            output,
            seed=seed,
            target_root=target_root,
            artifact_dir=artifact_dir,
        )

    if actor == LiveSmokeActor.EXECUTOR:
        missing_field_error = _evaluate_required_fields(
            actor=actor,
            output=output,
            required_fields=_EXECUTOR_REQUIRED_FIELDS,
        )
        if missing_field_error is not None:
            return missing_field_error
        return _evaluate_executor_report(output)

    if actor == LiveSmokeActor.EVALUATOR:
        missing_field_error = _evaluate_required_fields(
            actor=actor,
            output=output,
            required_fields=_EVALUATOR_REQUIRED_FIELDS,
        )
        if missing_field_error is not None:
            return missing_field_error
        return _evaluate_evaluator_result(output, invocation_input=invocation_input)

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


def _evaluate_non_empty_fields(
    *,
    actor: LiveSmokeActor,
    output: Mapping[str, object],
    required_fields: tuple[str, ...],
    non_empty_fields: tuple[str, ...],
) -> str | None:
    missing_field_error = _evaluate_required_fields(
        actor=actor,
        output=output,
        required_fields=required_fields,
    )
    if missing_field_error is not None:
        return missing_field_error

    for field_name in non_empty_fields:
        value = output[field_name]
        if not isinstance(value, list) or not value:
            return f"{actor.value} output must include non-empty {field_name}"
    return None


def _evaluate_generator_patch_bundle(
    output: Mapping[str, object],
    *,
    seed: LiveSmokeSeed | None,
    target_root: Path | None,
    artifact_dir: Path | None,
) -> str | None:
    if seed is None or not seed.expected_patch_paths:
        return None

    bundle, error = _patch_bundle_from_output(output, artifact_dir=artifact_dir)
    if error is not None:
        return error
    if bundle is None:
        return "generator output must include patch_bundle for seeded patch smoke"

    try:
        validate_patch_bundle(bundle)
    except ValueError as exc:
        return f"generator patch_bundle is invalid: {exc}"

    if bundle.base_tree_digest != seed.fixture_digest:
        return "generator patch_bundle base_tree_digest must match live smoke fixture_digest"

    operation_paths = {operation.path for operation in bundle.operations}
    for expected_path in seed.expected_patch_paths:
        if expected_path not in operation_paths:
            return f"generator patch_bundle must include expected path {expected_path}"

    if target_root is not None and tree_digest(target_root) != seed.fixture_digest:
        return "generator live smoke target changed directly"

    return None


def _patch_bundle_from_output(
    output: Mapping[str, object],
    *,
    artifact_dir: Path | None,
) -> tuple[PatchBundle | None, str | None]:
    inline_bundle = output.get("patch_bundle")
    if isinstance(inline_bundle, dict):
        return PatchBundle.model_validate(inline_bundle), None

    patch_ref = output.get("patch_bundle_ref")
    if patch_ref in (None, ""):
        return None, None
    if not isinstance(patch_ref, str):
        return None, "generator patch_bundle_ref must be a string"
    if artifact_dir is None:
        return None, "generator patch_bundle_ref requires artifact_dir"

    patch_path = Path(patch_ref)
    if patch_path.is_absolute() or ".." in patch_path.parts:
        return None, "generator patch_bundle_ref must stay inside artifact"
    resolved_patch_path = (artifact_dir / patch_path).resolve(strict=False)
    resolved_artifact_dir = artifact_dir.resolve(strict=False)
    if resolved_artifact_dir not in (resolved_patch_path, *resolved_patch_path.parents):
        return None, "generator patch_bundle_ref escapes artifact"
    if resolved_patch_path.is_symlink() or not resolved_patch_path.is_file():
        return None, "generator patch_bundle_ref is missing or unsafe"

    payload = yaml.safe_load(resolved_patch_path.read_text(encoding="utf-8"))
    if not isinstance(payload, dict):
        return None, "generator patch_bundle must be a mapping"
    return PatchBundle.model_validate(payload), None


def _evaluate_executor_report(output: Mapping[str, object]) -> str | None:
    tests = output["tests"]
    if not isinstance(tests, Mapping):
        return "executor output tests must be a mapping"
    for field_name in ("total", "passed", "failed"):
        if not isinstance(tests.get(field_name), int):
            return f"executor output tests.{field_name} must be an integer"
    if tests["total"] != tests["passed"] + tests["failed"]:
        return "executor output test counts must be internally consistent"

    logs = output["logs"]
    if not isinstance(logs, list) or not logs:
        return "executor output must include non-empty logs"

    has_failure = output["format"] == "fail" or output["analyze"] == "fail" or tests["failed"] > 0
    if has_failure:
        failure_details = output["failure_details"]
        if not isinstance(failure_details, list) or not failure_details:
            return "executor failing output must include failure_details"
        if not any(isinstance(detail, str) and detail.startswith("class=") for detail in failure_details):
            return "executor failing output must include machine-readable class= failure detail"

    return None


def _evaluate_evaluator_result(
    output: Mapping[str, object],
    *,
    invocation_input: Mapping[str, object] | None,
) -> str | None:
    if invocation_input is None:
        return "evaluator smoke requires invocation input"

    expected_digest = invocation_input.get("evaluator_input_digest")
    if not isinstance(expected_digest, str) or not expected_digest:
        return "evaluator smoke requires bound evaluator_input_digest"
    if output["evaluated_input_digest"] != expected_digest:
        return "evaluator output must echo evaluator_input_digest"

    decision = output["decision"]
    has_next_action = output.get("next_action") is not None
    if decision == "revise" and not has_next_action:
        return "evaluator revise output must include next_action"
    if decision in {"pass", "reject"} and has_next_action:
        return "evaluator pass/reject output must omit next_action"

    return None
