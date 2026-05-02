from __future__ import annotations

import json
from collections.abc import Callable
from pathlib import Path

from rail.api import start_task
from rail.artifacts import bind_effective_policy
from rail.artifacts.run_attempts import allocate_run_attempt
from rail.actor_runtime.codex_vault import CodexVaultActorRuntime
from rail.actor_runtime.runtime import ActorResult, ActorRuntime, build_invocation
from rail.live_smoke.classification import classify_actor_result
from rail.live_smoke.contracts import V1_LIVE_SMOKE_ACTORS, evaluate_behavior_smoke
from rail.live_smoke.fixtures import copy_fixture_target
from rail.live_smoke.models import (
    LiveSmokeActor,
    LiveSmokeReport,
    LiveSmokeVerdict,
    OwningSurface,
    SymptomClass,
)
from rail.policy import load_effective_policy

RuntimeFactory = Callable[[Path], ActorRuntime]

_REPORT_FILE_NAME = "live_smoke_report.json"
_UNAVAILABLE_FIXTURE_DIGEST = "sha256:fixture-unavailable"
_PLANNER_PRIOR_OUTPUT = {
    "summary": "Inspect the packaged live smoke fixture and produce a safe implementation plan.",
    "substeps": [
        "Read the fixture README and docs.",
        "Identify the minimal target files needed for the requested behavior.",
    ],
    "risks": [],
    "acceptance_criteria_refined": [
        "The actor returns schema-valid output for the live smoke fixture.",
    ],
}


class LiveSmokeRunner:
    def __init__(
        self,
        *,
        report_root: Path,
        runtime_factory: RuntimeFactory | None = None,
    ) -> None:
        self.report_root = report_root
        self.runtime_factory = runtime_factory or _default_runtime_factory

    def run_all(self) -> list[LiveSmokeReport]:
        return [self.run_actor(actor) for actor in V1_LIVE_SMOKE_ACTORS]

    def run_actor(self, actor: LiveSmokeActor) -> LiveSmokeReport:
        report_dir = self.report_root / actor.value
        target_root = report_dir / "target"
        try:
            copied_fixture = copy_fixture_target(target_root, report_root=self.report_root)
            policy = load_effective_policy(copied_fixture.target_root)
            handle = bind_effective_policy(
                start_task(_request_draft(actor, copied_fixture.target_root)),
                policy,
            )
            invocation = build_invocation(
                handle,
                actor.value,
                attempt_ref=allocate_run_attempt(handle.artifact_dir),
                prior_outputs=_prior_outputs_for(actor),
            )
        except Exception:
            report = _failure_report(
                actor=actor,
                report_dir=report_dir,
                fixture_digest=_UNAVAILABLE_FIXTURE_DIGEST,
                symptom_class=SymptomClass.FIXTURE_PREP_FAILURE,
                owning_surface=OwningSurface.FIXTURE,
            )
            _write_report(report)
            return report

        try:
            result = self.runtime_factory(copied_fixture.target_root).run(invocation)
        except Exception:
            report = _failure_report(
                actor=actor,
                report_dir=report_dir,
                fixture_digest=copied_fixture.fixture_digest,
                symptom_class=SymptomClass.PROVIDER_TRANSIENT_FAILURE,
                owning_surface=OwningSurface.PROVIDER,
                artifact_id=handle.artifact_id,
                artifact_dir=handle.artifact_dir,
            )
            _write_report(report)
            return report

        evidence_error = _evidence_error(handle.artifact_dir, result)
        if evidence_error is not None:
            report = _failure_report(
                actor=actor,
                report_dir=report_dir,
                fixture_digest=copied_fixture.fixture_digest,
                symptom_class=SymptomClass.EVIDENCE_WRITER_FAILURE,
                owning_surface=OwningSurface.PROVIDER,
                artifact_id=handle.artifact_id,
                artifact_dir=handle.artifact_dir,
            )
            _write_report(report)
            return report

        behavior_error = None
        if result.status == "succeeded":
            behavior_error = evaluate_behavior_smoke(actor, result.structured_output)
        classification = classify_actor_result(
            actor,
            result,
            behavior_error=behavior_error,
        )
        report = LiveSmokeReport(
            actor=actor,
            verdict=_verdict_for(classification.symptom_class),
            symptom_class=classification.symptom_class,
            owning_surface=classification.owning_surface,
            artifact_id=handle.artifact_id,
            artifact_dir=handle.artifact_dir,
            report_dir=report_dir,
            fixture_digest=copied_fixture.fixture_digest,
            evidence_refs=[
                result.events_ref.as_posix(),
                result.runtime_evidence_ref.as_posix(),
            ],
            repair_proposal=classification.repair_proposal,
        )
        _write_report(report)
        return report


def _default_runtime_factory(target_root: Path) -> ActorRuntime:
    return CodexVaultActorRuntime(
        project_root=Path("."),
        policy=load_effective_policy(target_root),
    )


def _request_draft(actor: LiveSmokeActor, target_root: Path) -> dict[str, object]:
    return {
        "project_root": target_root.resolve(strict=True).as_posix(),
        "task_type": "test_repair",
        "goal": f"Run the Rail {actor.value} live smoke against the packaged fixture target.",
        "context": {
            "feature": "actor live smoke",
            "validation_roots": ["tests"],
            "validation_targets": ["tests/test_service.py"],
        },
        "constraints": [
            "Do not mutate target files directly.",
            "Return only schema-valid structured actor output.",
        ],
        "definition_of_done": [
            "The actor completes with schema-valid output.",
            "The live smoke behavior check accepts the output.",
        ],
        "validation_profile": "smoke",
    }


def _prior_outputs_for(actor: LiveSmokeActor) -> dict[str, dict[str, object]]:
    if actor == LiveSmokeActor.CONTEXT_BUILDER:
        return {LiveSmokeActor.PLANNER.value: dict(_PLANNER_PRIOR_OUTPUT)}
    return {}


def _verdict_for(symptom_class: object | None) -> LiveSmokeVerdict:
    if symptom_class is None:
        return LiveSmokeVerdict.PASSED
    return LiveSmokeVerdict.FAILED


def _evidence_error(artifact_dir: Path, result: ActorResult) -> str | None:
    for evidence_ref in (result.events_ref, result.runtime_evidence_ref):
        if evidence_ref.is_absolute():
            return "evidence refs must be relative to artifact_dir"
        if ".." in evidence_ref.parts:
            return "evidence refs must not contain traversal"
        if len(evidence_ref.parts) < 2 or evidence_ref.parts[0] != "runs":
            return "evidence refs must be attempt-scoped under runs/"
        if not evidence_ref.parts[1].startswith("attempt-"):
            return "evidence refs must be attempt-scoped under runs/attempt-*"

        evidence_path = artifact_dir / evidence_ref
        try:
            resolved = evidence_path.resolve(strict=True)
            artifact_root = artifact_dir.resolve(strict=True)
        except OSError:
            return "evidence refs must resolve to existing files"
        if artifact_root not in (resolved, *resolved.parents):
            return "evidence refs must resolve inside artifact_dir"
        if evidence_path.is_symlink():
            return "evidence refs must not point at symlinks"
        if not evidence_path.is_file():
            return "evidence refs must resolve to files"
    return None


def _failure_report(
    *,
    actor: LiveSmokeActor,
    report_dir: Path,
    fixture_digest: str,
    symptom_class: SymptomClass,
    owning_surface: OwningSurface,
    artifact_id: str | None = None,
    artifact_dir: Path | None = None,
) -> LiveSmokeReport:
    return LiveSmokeReport(
        actor=actor,
        verdict=LiveSmokeVerdict.FAILED,
        symptom_class=symptom_class,
        owning_surface=owning_surface,
        artifact_id=artifact_id,
        artifact_dir=artifact_dir,
        report_dir=report_dir,
        fixture_digest=fixture_digest,
        evidence_refs=[],
        repair_proposal=None,
    )


def _write_report(report: LiveSmokeReport) -> None:
    report.report_dir.mkdir(parents=True, exist_ok=True)
    payload = report.model_dump(mode="json")
    (report.report_dir / _REPORT_FILE_NAME).write_text(
        json.dumps(payload, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )
