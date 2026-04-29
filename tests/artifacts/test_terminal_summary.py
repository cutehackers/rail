from __future__ import annotations

from pathlib import Path

import rail
from rail.artifacts.terminal_summary import project_terminal_summary
from tests.actor_runtime_test_fixtures import scripted_agents_runtime


def test_terminal_summary_explains_blocked_runtime(tmp_path, monkeypatch):
    monkeypatch.delenv("OPENAI_API_KEY", raising=False)
    monkeypatch.delenv("RAIL_ACTOR_RUNTIME_LIVE", raising=False)
    handle = rail.start_task(_draft(_target_repo(tmp_path), "Explain blocked state."))

    rail.supervise(handle)

    summary = project_terminal_summary(handle)

    assert summary.outcome == "blocked"
    assert summary.blocked_category == "runtime"
    assert "runtime" in summary.reason.lower() or "actor" in summary.reason.lower()
    assert summary.next_step
    assert (handle.artifact_dir / "terminal_summary.yaml").is_file()


def test_terminal_summary_projects_passed_artifact(tmp_path):
    target = _target_repo(tmp_path)
    handle = rail.start_task(_draft(target, "Explain pass state."))

    rail.supervise(handle, runtime=scripted_agents_runtime(target, patch_path="app.txt", patch_content="new\n"))

    summary = project_terminal_summary(handle)

    assert summary.outcome == "pass"
    assert summary.blocked_category is None
    assert summary.evidence_refs
    assert summary.next_step == "complete"


def _target_repo(tmp_path: Path) -> Path:
    target = tmp_path / "target"
    target.mkdir()
    return target


def _draft(target: Path, goal: str) -> dict[str, object]:
    return {
        "project_root": str(target),
        "task_type": "bug_fix",
        "goal": goal,
        "definition_of_done": ["Summary explains the state."],
    }
