from __future__ import annotations

import pytest
from pydantic import ValidationError

from rail.actor_runtime.schemas import ACTOR_OUTPUT_MODELS
from rail.live_smoke.models import LiveSmokeActor
from rail.live_smoke.seeds import (
    build_live_smoke_seed,
    canonical_prior_outputs_for,
    seed_payload_digest,
)


def test_canonical_prior_outputs_are_schema_valid_for_every_actor() -> None:
    for actor in LiveSmokeActor:
        prior_outputs = canonical_prior_outputs_for(actor)
        for upstream_actor, output in prior_outputs.items():
            ACTOR_OUTPUT_MODELS[upstream_actor].model_validate(output)


def test_context_builder_seed_uses_schema_valid_planner_output() -> None:
    prior_outputs = canonical_prior_outputs_for(LiveSmokeActor.CONTEXT_BUILDER)

    assert set(prior_outputs) == {"planner"}
    assert prior_outputs["planner"]["likely_files"] == ["app/service.py", "tests/test_service.py"]


def test_generator_seed_records_expected_patch_path_and_digest() -> None:
    prior_outputs = canonical_prior_outputs_for(LiveSmokeActor.GENERATOR)
    seed = build_live_smoke_seed(
        LiveSmokeActor.GENERATOR,
        fixture_digest="sha256:fixture",
        prior_outputs=prior_outputs,
    )

    assert seed.synthetic is True
    assert seed.expected_patch_paths == ["app/service.py"]
    assert set(seed.upstream_output_digests) == {"planner", "context_builder", "critic"}
    assert seed.seed_digest == seed_payload_digest(seed)


def test_evaluator_seed_records_synthetic_validation_digest() -> None:
    seed = build_live_smoke_seed(
        LiveSmokeActor.EVALUATOR,
        fixture_digest="sha256:fixture",
        prior_outputs=canonical_prior_outputs_for(LiveSmokeActor.EVALUATOR),
    )

    assert seed.validation_evidence_digest == "sha256:live-smoke-synthetic-validation"
    assert set(seed.upstream_output_digests) == {
        "planner",
        "context_builder",
        "critic",
        "generator",
        "executor",
    }


def test_seed_rejects_tampered_digest() -> None:
    with pytest.raises(ValidationError):
        build_live_smoke_seed(
            LiveSmokeActor.CRITIC,
            fixture_digest="sha256:fixture",
            prior_outputs=canonical_prior_outputs_for(LiveSmokeActor.CRITIC),
        ).model_copy(update={"seed_digest": "sha256:wrong"}, deep=True).model_validate(
            {
                "schema_version": "1",
                "actor": "critic",
                "fixture_digest": "sha256:fixture",
                "synthetic": True,
                "upstream_output_digests": {},
                "validation_evidence_digest": None,
                "expected_patch_paths": [],
                "seed_digest": "sha256:wrong",
            }
        )
