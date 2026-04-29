from __future__ import annotations

import inspect
from pathlib import Path

import rail


def test_fresh_goal_without_handle_allocates_new_artifact(tmp_path):
    target = _target_repo(tmp_path)

    first = rail.start_task(_draft(target))
    (first.artifact_dir / "run_status.yaml").write_text("status: blocked\n", encoding="utf-8")
    second = rail.start_task(_draft(target))

    assert first.artifact_id != second.artifact_id
    assert first.artifact_dir != second.artifact_dir


def test_known_handle_selects_existing_artifact_flow(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))

    decision = rail.decide_task_identity("continue the previous task", known_handle=handle)

    assert decision.flow == "existing_artifact"
    assert decision.handle == handle
    assert decision.requires_clarification is False


def test_resume_like_intent_without_handle_requires_clarification():
    decision = rail.decide_task_identity("resume the previous Rail task")

    assert decision.flow == "clarification_needed"
    assert decision.requires_clarification is True


def test_request_file_path_is_not_run_identity(tmp_path):
    target = _target_repo(tmp_path)
    request_file = target / ".harness" / "requests" / "request.yaml"
    request_file.parent.mkdir(parents=True)
    request_file.write_text("task_type: bug_fix\n", encoding="utf-8")

    decision = rail.decide_task_identity(f"resume {request_file}")

    assert decision.flow == "clarification_needed"
    assert decision.handle is None


def test_fresh_intent_without_handle_selects_fresh_task_flow():
    decision = rail.decide_task_identity("build a new feature")

    assert decision.flow == "fresh_task"
    assert decision.requires_clarification is False


def test_start_task_does_not_accept_user_supplied_task_id():
    signature = inspect.signature(rail.start_task)

    assert "task_id" not in signature.parameters
    assert "artifact_id" not in signature.parameters


def _target_repo(tmp_path: Path) -> Path:
    target = tmp_path / "target-repo"
    target.mkdir(parents=True, exist_ok=True)
    return target


def _draft(target: Path) -> dict[str, object]:
    return {
        "project_root": str(target),
        "task_type": "feature_addition",
        "goal": "Build a fresh workflow.",
        "definition_of_done": ["A new artifact is allocated."],
    }
