from __future__ import annotations

import json
from pathlib import Path

import pytest
import yaml

import rail
from rail.actor_runtime import codex_vault
from rail.actor_runtime.codex_vault import CodexVaultActorRuntime
from rail.artifacts.projection import project_result
from rail.policy import load_effective_policy
from tests.actor_runtime_test_fixtures import scripted_agents_runtime


def test_result_projection_reads_artifacts_only(tmp_path):
    target = _target_repo(tmp_path)
    handle = rail.start_task(_draft(target))
    rail.supervise(handle, runtime=scripted_agents_runtime(target))

    projection = project_result(handle)

    assert projection.outcome == "pass"
    assert projection.outcome_label == "pass"
    assert projection.blocked_category is None
    assert projection.current_phase == "terminal"
    assert projection.terminal_decision == "pass"
    assert projection.reason
    assert projection.evidence_refs
    assert "validation/evidence.yaml" in projection.evidence_refs
    assert "src/rail/api.py" in projection.changed_files
    assert projection.residual_risk == "low"
    assert projection.next_step == "complete"


def test_status_and_result_api_project_from_handle(tmp_path):
    target = _target_repo(tmp_path)
    handle = rail.start_task(_draft(target))
    rail.supervise(handle, runtime=scripted_agents_runtime(target))

    status = rail.status(handle)
    result = rail.result(handle)

    assert status.current_phase == "terminal"
    assert status.current_actor == "evaluator"
    assert status.next_step == "complete"
    assert result.outcome == "pass"


@pytest.mark.parametrize("blocked_category", ["runtime", "validation", "policy", "environment"])
def test_result_projection_projects_blocked_categories_from_artifacts_only(tmp_path, blocked_category):
    target = _target_repo(tmp_path)
    handle = rail.start_task(_draft(target))
    (handle.artifact_dir / "run_status.yaml").write_text(
        yaml.safe_dump(
            {
                "schema_version": "1",
                "artifact_id": handle.artifact_id,
                "status": "blocked",
                "outcome": "blocked",
                "current_actor": "evaluator",
                "blocked_category": blocked_category,
                "reason": f"{blocked_category} blocked reason",
                "visited": ["planner"],
            },
            sort_keys=True,
        ),
        encoding="utf-8",
    )

    projection = rail.result(handle)

    assert projection.outcome == "blocked"
    assert projection.outcome_label == f"{blocked_category}_blocked"
    assert projection.current_phase == "terminal"
    assert projection.terminal_decision == "blocked"
    assert projection.blocked_category == blocked_category
    assert projection.reason == f"{blocked_category} blocked reason"
    assert projection.next_step


def test_result_projection_projects_missing_codex_command_as_environment_blocked(tmp_path):
    target = _target_repo(tmp_path)
    handle = rail.start_task(_draft(target))
    runtime = CodexVaultActorRuntime(
        project_root=Path("."),
        policy=load_effective_policy(tmp_path),
        command_resolver=lambda: None,
    )

    rail.supervise(handle, runtime=runtime)

    projection = rail.result(handle)
    assert projection.outcome == "blocked"
    assert projection.outcome_label == "environment_blocked"
    assert projection.blocked_category == "environment"
    assert "Codex command" in projection.reason


def test_result_projection_projects_contamination_as_policy_blocked(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    runtime = CodexVaultActorRuntime(
        project_root=Path("."),
        policy=load_effective_policy(tmp_path),
        command_resolver=lambda: _fake_codex_command(tmp_path),
        command_trust_checker=lambda _path, _target_root, _artifact_dir: None,
        runner=ContaminatedCodexRunner(),
        environment_materializer=_fake_vault_environment,
    )

    rail.supervise(handle, runtime=runtime)

    projection = rail.result(handle)
    assert projection.outcome == "blocked"
    assert projection.outcome_label == "policy_blocked"
    assert projection.blocked_category == "policy"
    assert "skill" in projection.reason


def test_result_projection_redacts_secret_reason_from_artifacts(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))
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

    projection = rail.result(handle)

    assert "sk-secret-value" not in projection.reason
    assert "[REDACTED]" in projection.reason


def _target_repo(tmp_path: Path) -> Path:
    target = tmp_path / "target-repo"
    target.mkdir(parents=True, exist_ok=True)
    return target


def _draft(target: Path) -> dict[str, object]:
    return {
        "project_root": str(target),
        "task_type": "bug_fix",
        "goal": "Project result.",
        "definition_of_done": ["Projection reads artifacts."],
    }


class ContaminatedCodexRunner:
    def __call__(self, command: list[str], *, stdin: str | None = None, environ=None, timeout=None):
        if command[1:] == ["--version"]:
            return codex_vault.CodexCommandRunResult(stdout="codex-cli 0.124.0", stderr="", returncode=0)
        if command[1:] == ["exec", "--help"]:
            return codex_vault.CodexCommandRunResult(
                stdout="\n".join(codex_vault.CODEX_EXEC_REQUIRED_HELP_FLAGS),
                stderr="",
                returncode=0,
            )
        if command[1:3] == ["exec", "--json"]:
            events = [
                {"type": "skill_invocation", "name": "Rail"},
                {
                    "type": "final_output",
                    "output": {
                        "summary": "Plan",
                        "likely_files": [],
                        "substeps": [],
                        "risks": [],
                        "acceptance_criteria_refined": [],
                    },
                },
            ]
            return codex_vault.CodexCommandRunResult(
                stdout="\n".join(json.dumps(event, sort_keys=True) for event in events),
                stderr="",
                returncode=0,
            )
        raise AssertionError(f"unexpected command: {command}")


def _fake_codex_command(tmp_path: Path) -> Path:
    command = tmp_path / "trusted-bin" / "codex"
    command.parent.mkdir(exist_ok=True)
    command.write_text("#!/bin/sh\nexit 0\n", encoding="utf-8")
    command.chmod(0o755)
    return command


def _fake_vault_environment(*, artifact_dir: Path, auth_home: Path, base_environ, actor: str | None = None):
    from rail.actor_runtime.vault_env import VaultEnvironment

    codex_home = artifact_dir / "actor_runtime" / "actors" / (actor or "actor") / "invocation" / "codex_home"
    evidence_dir = codex_home.parent / "evidence"
    temp_dir = codex_home.parent / "tmp"
    for path in (codex_home, evidence_dir, temp_dir):
        path.mkdir(parents=True, exist_ok=True)
    (codex_home / "auth.json").write_text("{}", encoding="utf-8")
    return VaultEnvironment(
        codex_home=codex_home,
        evidence_dir=evidence_dir,
        temp_dir=temp_dir,
        environ={
            "PATH": "/usr/bin:/bin",
            "HOME": str(codex_home),
            "CODEX_HOME": str(codex_home),
            "TMPDIR": str(temp_dir),
            "TMP": str(temp_dir),
            "TEMP": str(temp_dir),
        },
        copied_auth_material=["auth.json"],
    )
