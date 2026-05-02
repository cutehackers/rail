from __future__ import annotations

import os

import pytest

from rail.live_smoke.contracts import V1_LIVE_SMOKE_ACTORS
from rail.live_smoke.models import LiveSmokeActor, LiveSmokeVerdict
from rail.live_smoke.runner import LiveSmokeRunner

pytestmark = pytest.mark.skipif(
    os.environ.get("RAIL_CODEX_VAULT_LIVE_SMOKE") != "1",
    reason="codex_vault live smoke is opt-in",
)


@pytest.mark.parametrize("actor", V1_LIVE_SMOKE_ACTORS)
def test_optional_codex_vault_live_smoke_runs_v1_actor_without_openai_api_key(
    tmp_path,
    monkeypatch,
    actor: LiveSmokeActor,
) -> None:
    monkeypatch.delenv("OPENAI_API_KEY", raising=False)
    runner = LiveSmokeRunner(report_root=tmp_path / "reports")

    report = runner.run_actor(actor)

    assert report.verdict == LiveSmokeVerdict.PASSED, (
        f"actor={report.actor.value} symptom={report.symptom_class} "
        f"surface={report.owning_surface} report={report.report_dir}"
    )
