from __future__ import annotations

import pytest
import yaml

import rail
from rail.artifacts.handle import load_handle_file
from rail.artifacts.store import bind_effective_policy
from rail.policy import load_effective_policy


def test_start_task_persists_initial_handle(tmp_path):
    target = tmp_path / "target"
    target.mkdir()

    handle = rail.start_task(_draft(target, "Persist initial handle."))

    assert (handle.artifact_dir / "handle.yaml").is_file()
    reloaded = load_handle_file(handle.artifact_dir / "handle.yaml")
    assert reloaded.artifact_id == handle.artifact_id
    assert reloaded.effective_policy_digest is None


def test_start_task_persists_reloadable_bound_handle(tmp_path):
    target = tmp_path / "target"
    target.mkdir()
    handle = rail.start_task(_draft(target, "Persist bound handle."))
    bound = bind_effective_policy(handle, load_effective_policy(target))

    reloaded = load_handle_file(bound.artifact_dir / "handle.yaml")

    assert reloaded.artifact_id == bound.artifact_id
    assert reloaded.artifact_dir == bound.artifact_dir
    assert reloaded.project_root == bound.project_root
    assert reloaded.request_snapshot_digest == bound.request_snapshot_digest
    assert reloaded.effective_policy_digest == bound.effective_policy_digest


def test_load_handle_rejects_forged_artifact_id(tmp_path):
    target = tmp_path / "target"
    target.mkdir()
    handle = rail.start_task(_draft(target, "Reject forged handle."))
    path = handle.artifact_dir / "handle.yaml"
    payload = yaml.safe_load(path.read_text(encoding="utf-8"))
    payload["artifact_id"] = "rail-forged"
    path.write_text(yaml.safe_dump(payload), encoding="utf-8")

    with pytest.raises(ValueError, match="artifact_id"):
        rail.load_handle(path)


def test_loaded_handle_projects_status_without_allocating_new_artifact(tmp_path):
    target = tmp_path / "target"
    target.mkdir()
    handle = rail.start_task(_draft(target, "Project loaded handle."))
    before = set(handle.artifact_dir.parent.iterdir())

    loaded = rail.load_handle(handle.artifact_dir / "handle.yaml")
    status = rail.status(loaded)
    result = rail.result(loaded)
    after = set(handle.artifact_dir.parent.iterdir())

    assert loaded.artifact_id == handle.artifact_id
    assert status.current_actor is None
    assert result.outcome == "unknown"
    assert before == after


def _draft(target, goal: str) -> dict[str, object]:
    return {
        "project_root": str(target),
        "task_type": "bug_fix",
        "goal": goal,
        "definition_of_done": ["Handle reloads."],
    }
