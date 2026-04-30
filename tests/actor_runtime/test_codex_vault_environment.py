from __future__ import annotations

import pytest

from rail.actor_runtime.vault_env import materialize_vault_environment


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


def test_materialize_vault_env_does_not_leak_secret_or_user_paths(tmp_path):
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
        "TMPDIR": "/tmp/rail",
        "HOME": str(env.codex_home),
        "CODEX_HOME": str(env.codex_home),
    }
