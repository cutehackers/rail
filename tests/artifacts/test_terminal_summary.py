from __future__ import annotations

from pathlib import Path

import yaml

import rail
from rail.artifacts.terminal_summary import project_terminal_summary
from tests.actor_runtime_test_fixtures import scripted_agents_runtime


def test_terminal_summary_explains_blocked_environment_readiness(tmp_path, monkeypatch):
    monkeypatch.delenv("OPENAI_API_KEY", raising=False)
    monkeypatch.delenv("RAIL_ACTOR_RUNTIME_LIVE", raising=False)
    handle = rail.start_task(_draft(_target_repo(tmp_path), "Explain blocked state."))

    rail.supervise(handle)

    summary = project_terminal_summary(handle)

    assert summary.outcome == "blocked"
    assert summary.blocked_category == "environment"
    assert "auth" in summary.reason.lower() or "codex" in summary.reason.lower()
    assert "environment" in summary.next_step
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


def test_terminal_summary_redacts_secret_reason_from_artifacts(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path), "Explain secret blocked state."))
    (handle.artifact_dir / "run_status.yaml").write_text(
        yaml.safe_dump(
            {
                "schema_version": "1",
                "artifact_id": handle.artifact_id,
                "status": "blocked",
                "outcome": "blocked",
                "current_actor": "planner",
                "blocked_category": "environment",
                "reason": "OPENAI_API_KEY=sk-secret-value",
                "visited": ["planner"],
            },
            sort_keys=True,
        ),
        encoding="utf-8",
    )

    summary = project_terminal_summary(handle)

    assert "sk-secret-value" not in summary.reason
    assert "[REDACTED]" in summary.reason


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
