from __future__ import annotations

import json
from pathlib import Path

import rail
import pytest

from rail.actor_runtime import codex_vault
from rail.actor_runtime.codex_vault import CodexVaultActorRuntime
from rail.actor_runtime.runtime import build_invocation
from rail.actor_runtime.vault_env import materialize_vault_environment
from rail.policy import load_effective_policy


def test_materialize_vault_env_uses_artifact_local_codex_home(tmp_path):
    artifact_dir = tmp_path / "artifact"
    auth_home = tmp_path / "auth"
    auth_home.mkdir()
    (auth_home / "auth.json").write_text("{}", encoding="utf-8")

    env = materialize_vault_environment(
        artifact_dir=artifact_dir,
        auth_home=auth_home,
        base_environ={"HOME": "/should/not/leak"},
    )

    assert env.codex_home == artifact_dir / "actor_runtime" / "codex_home"
    assert env.evidence_dir == artifact_dir / "actor_runtime" / "evidence"
    assert env.environ["CODEX_HOME"] == str(env.codex_home)
    assert env.environ["HOME"] == str(env.codex_home)
    assert env.codex_home.is_dir()
    assert env.evidence_dir.is_dir()
    assert env.codex_home.stat().st_mode & 0o777 == 0o700


def test_materialize_vault_env_does_not_copy_user_codex_surfaces_from_environment(tmp_path):
    artifact_dir = tmp_path / "artifact"
    auth_home = tmp_path / "auth"
    auth_home.mkdir()
    (auth_home / "auth.json").write_text("{}", encoding="utf-8")
    user_codex_home = tmp_path / "user-codex"
    for name in ("skills", "plugins", "mcp", "hooks", "rules"):
        (user_codex_home / name).mkdir(parents=True)
    (user_codex_home / "config.toml").write_text("model = 'user'\n", encoding="utf-8")

    env = materialize_vault_environment(
        artifact_dir=artifact_dir,
        auth_home=auth_home,
        base_environ={"CODEX_HOME": str(user_codex_home)},
    )

    for name in ("skills", "plugins", "mcp", "hooks", "rules", "config.toml"):
        assert not (env.codex_home / name).exists()


def test_materialize_vault_env_copies_only_auth_json(tmp_path):
    artifact_dir = tmp_path / "artifact"
    auth_home = tmp_path / "auth"
    auth_home.mkdir()
    (auth_home / "auth.json").write_text('{"tokens":[]}', encoding="utf-8")

    env = materialize_vault_environment(
        artifact_dir=artifact_dir,
        auth_home=auth_home,
        base_environ={"PATH": "/usr/bin"},
    )

    assert (env.codex_home / "auth.json").read_text(encoding="utf-8") == '{"tokens":[]}'
    assert sorted(path.name for path in env.codex_home.iterdir()) == ["auth.json"]
    assert env.copied_auth_material == ["auth.json"]
    assert (env.codex_home / "auth.json").stat().st_mode & 0o777 == 0o600


def test_materialize_vault_env_uses_actor_scoped_home_when_actor_is_provided(tmp_path):
    artifact_dir = tmp_path / "artifact"
    auth_home = tmp_path / "auth"
    auth_home.mkdir()
    (auth_home / "auth.json").write_text("{}", encoding="utf-8")

    planner_env = materialize_vault_environment(
        artifact_dir=artifact_dir,
        auth_home=auth_home,
        base_environ={},
        actor="planner",
    )
    context_env = materialize_vault_environment(
        artifact_dir=artifact_dir,
        auth_home=auth_home,
        base_environ={},
        actor="context_builder",
    )

    assert planner_env.codex_home == artifact_dir / "actor_runtime" / "actors" / "planner" / "codex_home"
    assert context_env.codex_home == artifact_dir / "actor_runtime" / "actors" / "context_builder" / "codex_home"
    assert (planner_env.codex_home / "auth.json").is_file()
    assert (context_env.codex_home / "auth.json").is_file()


def test_materialize_vault_env_rejects_preexisting_codex_surfaces(tmp_path):
    artifact_dir = tmp_path / "artifact"
    codex_home = artifact_dir / "actor_runtime" / "codex_home"
    (codex_home / "skills").mkdir(parents=True)
    auth_home = tmp_path / "auth"
    auth_home.mkdir()
    (auth_home / "auth.json").write_text("{}", encoding="utf-8")

    with pytest.raises(ValueError, match="unexpected vault material"):
        materialize_vault_environment(
            artifact_dir=artifact_dir,
            auth_home=auth_home,
            base_environ={},
        )


def test_materialize_vault_env_rejects_symlinked_actor_runtime_parent(tmp_path):
    artifact_dir = tmp_path / "artifact"
    artifact_dir.mkdir()
    outside_runtime = tmp_path / "outside-runtime"
    outside_runtime.mkdir()
    (artifact_dir / "actor_runtime").symlink_to(outside_runtime, target_is_directory=True)
    auth_home = tmp_path / "auth"
    auth_home.mkdir()
    (auth_home / "auth.json").write_text("{}", encoding="utf-8")

    with pytest.raises(ValueError, match="unsafe vault material"):
        materialize_vault_environment(
            artifact_dir=artifact_dir,
            auth_home=auth_home,
            base_environ={},
        )


def test_materialize_vault_env_rejects_preexisting_auth_destination_symlink(tmp_path):
    artifact_dir = tmp_path / "artifact"
    codex_home = artifact_dir / "actor_runtime" / "codex_home"
    codex_home.mkdir(parents=True)
    outside = tmp_path / "outside-auth.json"
    outside.write_text("outside", encoding="utf-8")
    (codex_home / "auth.json").symlink_to(outside)
    auth_home = tmp_path / "auth"
    auth_home.mkdir()
    (auth_home / "auth.json").write_text("{}", encoding="utf-8")

    with pytest.raises(ValueError, match="unsafe vault material"):
        materialize_vault_environment(
            artifact_dir=artifact_dir,
            auth_home=auth_home,
            base_environ={},
        )


def test_materialize_vault_env_rejects_symlinked_evidence_dir(tmp_path):
    artifact_dir = tmp_path / "artifact"
    evidence_dir = artifact_dir / "actor_runtime" / "evidence"
    evidence_dir.parent.mkdir(parents=True)
    outside_evidence = tmp_path / "outside-evidence"
    outside_evidence.mkdir()
    evidence_dir.symlink_to(outside_evidence, target_is_directory=True)
    auth_home = tmp_path / "auth"
    auth_home.mkdir()
    (auth_home / "auth.json").write_text("{}", encoding="utf-8")

    with pytest.raises(ValueError, match="unsafe vault material"):
        materialize_vault_environment(
            artifact_dir=artifact_dir,
            auth_home=auth_home,
            base_environ={},
        )


def test_materialize_vault_env_rejects_config_toml_in_auth_home(tmp_path):
    artifact_dir = tmp_path / "artifact"
    auth_home = tmp_path / "auth"
    auth_home.mkdir()
    (auth_home / "auth.json").write_text("{}", encoding="utf-8")
    (auth_home / "config.toml").write_text("model = 'user'\n", encoding="utf-8")

    with pytest.raises(ValueError, match="unknown auth material"):
        materialize_vault_environment(
            artifact_dir=artifact_dir,
            auth_home=auth_home,
            base_environ={},
        )


def test_materialize_vault_env_rejects_unknown_auth_material(tmp_path):
    artifact_dir = tmp_path / "artifact"
    auth_home = tmp_path / "auth"
    auth_home.mkdir()
    (auth_home / "auth.json").write_text("{}", encoding="utf-8")
    (auth_home / "plugins").mkdir()

    with pytest.raises(ValueError, match="unknown auth material"):
        materialize_vault_environment(
            artifact_dir=artifact_dir,
            auth_home=auth_home,
            base_environ={},
        )


def test_materialize_vault_env_uses_artifact_local_temp_and_does_not_leak_secret_or_user_paths(tmp_path):
    artifact_dir = tmp_path / "artifact"
    auth_home = tmp_path / "auth"
    auth_home.mkdir()
    (auth_home / "auth.json").write_text("{}", encoding="utf-8")

    env = materialize_vault_environment(
        artifact_dir=artifact_dir,
        auth_home=auth_home,
        base_environ={
            "PATH": "/usr/bin",
            "TMPDIR": "/tmp/rail",
            "OPENAI_API_KEY": "sk-secret",
            "CODEX_HOME": "/user/codex",
            "HOME": "/user/home",
        },
    )

    assert env.environ == {
        "PATH": "/usr/bin",
        "TMPDIR": str(env.temp_dir),
        "TMP": str(env.temp_dir),
        "TEMP": str(env.temp_dir),
        "HOME": str(env.codex_home),
        "CODEX_HOME": str(env.codex_home),
    }
    assert env.temp_dir == artifact_dir / "actor_runtime" / "tmp"


def test_runtime_materialization_failure_blocks_without_crashing(tmp_path):
    target = _target_repo(tmp_path)
    handle = rail.start_task(_draft(target))

    def fail_materialization(**_kwargs):
        raise PermissionError("cannot create vault")

    runtime = CodexVaultActorRuntime(
        project_root=Path("."),
        policy=load_effective_policy(tmp_path),
        command_resolver=lambda: tmp_path / "bin" / "codex",
        command_trust_checker=lambda _path, _target_root, _artifact_dir: None,
        runner=_ready_codex_runner(),
        environment_materializer=fail_materialization,
    )

    result = runtime.run(build_invocation(handle, "planner"))

    assert result.status == "interrupted"
    assert result.blocked_category == "environment"
    assert "environment materialization failed" in result.structured_output["error"]
    assert "cannot create vault" not in result.structured_output["error"]


def test_runtime_materialization_failure_does_not_leak_host_paths(tmp_path):
    target = _target_repo(tmp_path)
    handle = rail.start_task(_draft(target))
    private_path = tmp_path / "private-auth" / "auth.json"

    def fail_materialization(**_kwargs):
        raise PermissionError(f"permission denied: {private_path}")

    runtime = CodexVaultActorRuntime(
        project_root=Path("."),
        policy=load_effective_policy(tmp_path),
        command_resolver=lambda: tmp_path / "bin" / "codex",
        command_trust_checker=lambda _path, _target_root, _artifact_dir: None,
        runner=_ready_codex_runner(),
        environment_materializer=fail_materialization,
    )

    result = runtime.run(build_invocation(handle, "planner"))

    evidence = json.loads((handle.artifact_dir / result.runtime_evidence_ref).read_text(encoding="utf-8"))
    serialized = json.dumps(evidence)
    assert str(private_path) not in serialized
    assert str(private_path) not in result.structured_output["error"]
    assert evidence["error_type"] == "PermissionError"


def test_runtime_evidence_uses_relative_vault_refs(tmp_path, monkeypatch):
    target = _target_repo(tmp_path)
    handle = rail.start_task(_draft(target))
    rail_home = tmp_path / "rail-home"
    auth_home = rail_home / "codex"
    auth_home.mkdir(parents=True)
    auth_file = auth_home / "auth.json"
    auth_file.write_text("{}", encoding="utf-8")
    auth_file.chmod(0o600)
    monkeypatch.setenv("RAIL_HOME", str(rail_home))

    runtime = CodexVaultActorRuntime(
        project_root=Path("."),
        policy=load_effective_policy(tmp_path),
        command_resolver=lambda: tmp_path / "bin" / "codex",
        command_trust_checker=lambda _path, _target_root, _artifact_dir: None,
        runner=_ready_codex_runner(),
    )

    result = runtime.run(build_invocation(handle, "planner"))

    evidence = json.loads((handle.artifact_dir / result.runtime_evidence_ref).read_text(encoding="utf-8"))
    assert evidence["vault_codex_home_ref"] == "actor_runtime/actors/planner/codex_home"
    assert evidence["vault_evidence_dir_ref"] == "actor_runtime/actors/planner/evidence"
    serialized = json.dumps(evidence)
    assert str(handle.artifact_dir) not in serialized
    assert str(rail_home) not in serialized


def _target_repo(tmp_path):
    target = tmp_path / "target"
    target.mkdir()
    return target


def _draft(target) -> dict[str, object]:
    return {
        "project_root": str(target),
        "task_type": "bug_fix",
        "goal": "Materialize Codex Vault environment.",
        "definition_of_done": ["Environment materializes safely."],
    }


def _ready_codex_runner():
    def runner(command: list[str]) -> codex_vault.CodexCommandRunResult:
        if command[1:] == ["--version"]:
            return codex_vault.CodexCommandRunResult(stdout="codex-cli 0.124.0", stderr="", returncode=0)
        if command[1:] == ["exec", "--help"]:
            return codex_vault.CodexCommandRunResult(
                stdout="\n".join(codex_vault.CODEX_EXEC_REQUIRED_HELP_FLAGS),
                stderr="",
                returncode=0,
            )
        raise AssertionError(f"unexpected command: {command}")

    return runner
