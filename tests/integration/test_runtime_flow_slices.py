from __future__ import annotations

from pathlib import Path

import rail
from tests.integration_flow_fixtures import run_runtime_flow_slices


def test_runtime_flow_slices_wire_actor_patch_validation_evaluator_and_projection(tmp_path):
    target = _target_repo(tmp_path)
    (target / "app.txt").write_text("old\n", encoding="utf-8")
    handle = rail.start_task(_draft(target))

    report = run_runtime_flow_slices(handle, target_root=target, relative_path="app.txt", content="new\n")

    assert report.actor_evidence_persisted is True
    assert report.patch_bundle_ref == "inline"
    assert report.patch_applied is True
    assert report.validation_status == "pass"
    assert report.evaluator_outcome == "pass"
    assert report.result_outcome == "pass"
    assert report.status_phase == "terminal"
    assert (target / "app.txt").read_text(encoding="utf-8") == "new\n"


def _target_repo(tmp_path: Path) -> Path:
    target = tmp_path / "target-repo"
    target.mkdir()
    policy = target / ".harness" / "supervisor" / "execution_policy.yaml"
    policy.parent.mkdir(parents=True, exist_ok=True)
    policy.write_text(
        "version: 2\nvalidation:\n  commands:\n    - python -c \"import pathlib; assert pathlib.Path('app.txt').exists()\"\n",
        encoding="utf-8",
    )
    return target


def _draft(target: Path) -> dict[str, object]:
    return {
        "project_root": str(target),
        "task_type": "bug_fix",
        "goal": "Wire runtime slices.",
        "definition_of_done": ["Runtime slices pass."],
    }
