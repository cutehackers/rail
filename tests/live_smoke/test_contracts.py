from __future__ import annotations

from pathlib import Path

import pytest
from pydantic import ValidationError

from rail.live_smoke.models import (
    LiveSmokeActor,
    LiveSmokeReport,
    LiveSmokeVerdict,
    OwningSurface,
    RepairProposal,
    SymptomClass,
)


def test_live_smoke_report_rejects_unknown_fields(tmp_path: Path) -> None:
    with pytest.raises(ValidationError):
        LiveSmokeReport(
            actor=LiveSmokeActor.PLANNER,
            verdict=LiveSmokeVerdict.PASSED,
            symptom_class=None,
            owning_surface=None,
            report_dir=tmp_path,
            fixture_digest="sha256:abc",
            evidence_refs=[],
            repair_proposal=None,
            unexpected=True,
        )


def test_repair_proposal_records_safe_rail_owned_surface() -> None:
    proposal = RepairProposal(
        owning_surface=OwningSurface.ACTOR_PROMPT,
        file_paths=[".harness/actors/context_builder.md"],
        summary="Forbid grep fallback in context collection.",
        preserves_fail_closed_policy=True,
    )

    assert proposal.owning_surface == OwningSurface.ACTOR_PROMPT
    assert proposal.preserves_fail_closed_policy is True


def test_symptom_classes_include_non_actor_environment_failures() -> None:
    assert SymptomClass.READINESS_FAILURE.value == "readiness_failure"
    assert SymptomClass.PROVIDER_TRANSIENT_FAILURE.value == "provider_transient_failure"
    assert SymptomClass.EVIDENCE_WRITER_FAILURE.value == "evidence_writer_failure"
