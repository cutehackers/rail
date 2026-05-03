from __future__ import annotations

from typing import Literal

from pydantic import BaseModel, ConfigDict, Field, model_validator

from rail.actor_runtime.schemas import ACTOR_OUTPUT_MODELS
from rail.artifacts.digests import digest_payload
from rail.live_smoke.models import LiveSmokeActor

LIVE_SMOKE_SEED_SCHEMA_VERSION = "1"
_SYNTHETIC_VALIDATION_EVIDENCE_DIGEST = "sha256:live-smoke-synthetic-validation"


class LiveSmokeSeed(BaseModel):
    model_config = ConfigDict(extra="forbid")

    schema_version: Literal["1"] = "1"
    actor: LiveSmokeActor
    fixture_digest: str
    synthetic: Literal[True] = True
    upstream_output_digests: dict[str, str] = Field(default_factory=dict)
    validation_evidence_digest: str | None = None
    expected_patch_paths: list[str] = Field(default_factory=list)
    seed_digest: str

    @model_validator(mode="after")
    def validate_seed_digest(self) -> LiveSmokeSeed:
        expected_digest = seed_payload_digest(self)
        if self.seed_digest != expected_digest:
            raise ValueError("seed_digest does not match seed payload")
        return self


def build_live_smoke_seed(
    actor: LiveSmokeActor,
    *,
    fixture_digest: str,
    prior_outputs: dict[str, dict[str, object]],
) -> LiveSmokeSeed:
    seed_payload = {
        "schema_version": LIVE_SMOKE_SEED_SCHEMA_VERSION,
        "actor": actor.value,
        "fixture_digest": fixture_digest,
        "synthetic": True,
        "upstream_output_digests": _upstream_output_digests(prior_outputs),
        "validation_evidence_digest": _validation_evidence_digest_for(actor),
        "expected_patch_paths": _expected_patch_paths_for(actor),
    }
    return LiveSmokeSeed.model_validate(seed_payload | {"seed_digest": digest_payload(seed_payload)})


def canonical_prior_outputs_for(actor: LiveSmokeActor) -> dict[str, dict[str, object]]:
    outputs: dict[str, dict[str, object]] = {}
    for upstream_actor in _upstream_actors_for(actor):
        output = _canonical_output_for(upstream_actor)
        outputs[upstream_actor.value] = ACTOR_OUTPUT_MODELS[upstream_actor.value].model_validate(output).model_dump(
            mode="json"
        )
    return outputs


def seed_payload_digest(seed: LiveSmokeSeed) -> str:
    payload = seed.model_dump(mode="json", exclude={"seed_digest"})
    return digest_payload(payload)


def _upstream_actors_for(actor: LiveSmokeActor) -> tuple[LiveSmokeActor, ...]:
    if actor == LiveSmokeActor.PLANNER:
        return ()
    if actor == LiveSmokeActor.CONTEXT_BUILDER:
        return (LiveSmokeActor.PLANNER,)
    if actor == LiveSmokeActor.CRITIC:
        return (LiveSmokeActor.PLANNER, LiveSmokeActor.CONTEXT_BUILDER)
    if actor == LiveSmokeActor.GENERATOR:
        return (LiveSmokeActor.PLANNER, LiveSmokeActor.CONTEXT_BUILDER, LiveSmokeActor.CRITIC)
    if actor == LiveSmokeActor.EXECUTOR:
        return (
            LiveSmokeActor.PLANNER,
            LiveSmokeActor.CONTEXT_BUILDER,
            LiveSmokeActor.CRITIC,
            LiveSmokeActor.GENERATOR,
        )
    if actor == LiveSmokeActor.EVALUATOR:
        return (
            LiveSmokeActor.PLANNER,
            LiveSmokeActor.CONTEXT_BUILDER,
            LiveSmokeActor.CRITIC,
            LiveSmokeActor.GENERATOR,
            LiveSmokeActor.EXECUTOR,
        )
    return ()


def _canonical_output_for(actor: LiveSmokeActor) -> dict[str, object]:
    if actor == LiveSmokeActor.PLANNER:
        return {
            "summary": "Inspect the packaged live smoke fixture and make one bounded service change.",
            "likely_files": ["app/service.py", "tests/test_service.py"],
            "assumptions": [],
            "substeps": [
                "Read the fixture README and docs.",
                "Inspect app/service.py and tests/test_service.py.",
                "Return schema-valid actor output for the live smoke boundary.",
            ],
            "risks": ["Synthetic seed must not be mistaken for real supervisor evidence."],
            "acceptance_criteria_refined": [
                "The actor output is schema-valid.",
                "The actor stays within the fixture target and Rail policy boundary.",
            ],
        }
    if actor == LiveSmokeActor.CONTEXT_BUILDER:
        return {
            "relevant_files": [
                {"path": "README.md", "why": "Explains the live smoke fixture target."},
                {"path": "app/service.py", "why": "Contains the small service function under change."},
                {"path": "tests/test_service.py", "why": "Contains the focused validation target."},
            ],
            "repo_patterns": [
                "Source code lives under app/.",
                "Tests live under tests/ and use direct pytest assertions.",
            ],
            "test_patterns": ["Focused tests import from app.service and assert exact string output."],
            "forbidden_changes": [
                "Do not inspect parent directories or host paths.",
                "Do not mutate target files directly.",
            ],
            "implementation_hints": [
                "Keep changes limited to app/service.py and tests/test_service.py when a patch is required.",
                "Return a patch bundle instead of direct target mutation.",
            ],
        }
    if actor == LiveSmokeActor.CRITIC:
        return {
            "priority_focus": ["Patch bundle boundary", "Fixture-scoped validation"],
            "missing_requirements": [],
            "risk_hypotheses": ["Generator may omit the required patch bundle."],
            "validation_expectations": ["Focused service test remains the validation target."],
            "generator_guardrails": [
                "Do not mutate target files directly.",
                "Patch paths must stay relative to the fixture target.",
            ],
            "blocked_assumptions": [],
        }
    if actor == LiveSmokeActor.GENERATOR:
        return {
            "changed_files": [],
            "patch_summary": ["Synthetic read-only seed; no patch has been applied."],
            "tests_added_or_updated": [],
            "known_limits": ["Executor live smoke receives an unapplied read-only generator seed."],
        }
    if actor == LiveSmokeActor.EXECUTOR:
        return {
            "format": "pass",
            "analyze": "pass",
            "tests": {"total": 1, "passed": 1, "failed": 0},
            "failure_details": [],
            "logs": ["Synthetic validation evidence: tests/test_service.py passed for the fixture."],
        }
    raise ValueError(f"unsupported canonical output actor: {actor.value}")


def _upstream_output_digests(prior_outputs: dict[str, dict[str, object]]) -> dict[str, str]:
    return {
        actor: digest_payload(output)
        for actor, output in sorted(prior_outputs.items())
    }


def _validation_evidence_digest_for(actor: LiveSmokeActor) -> str | None:
    if actor == LiveSmokeActor.EVALUATOR:
        return _SYNTHETIC_VALIDATION_EVIDENCE_DIGEST
    return None


def _expected_patch_paths_for(actor: LiveSmokeActor) -> list[str]:
    if actor == LiveSmokeActor.GENERATOR:
        return ["app/service.py"]
    return []
