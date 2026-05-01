from __future__ import annotations

from pathlib import Path

from rail.actor_runtime.vault_audit import audit_vault_materialization
from rail.actor_runtime.vault_env import VaultEnvironment


def test_vault_audit_allows_codex_cli_bootstrap_material(tmp_path):
    artifact_dir = tmp_path / "artifact"
    env = _vault_environment(artifact_dir)
    env.codex_home.mkdir(parents=True)
    (env.codex_home / "auth.json").write_text("{}", encoding="utf-8")
    (env.codex_home / "installation_id").write_text("installation\n", encoding="utf-8")
    (env.codex_home / "models_cache.json").write_text("{}", encoding="utf-8")
    (env.codex_home / "config.toml").write_text(
        '\n'.join(
            [
                '[plugins."gmail@openai-curated"]',
                "enabled = true",
                "",
                '[plugins."github@openai-curated"]',
                "enabled = true",
            ]
        ),
        encoding="utf-8",
    )
    for directory in ("cache/codex_apps_tools", "plugins/cache/openai-curated/github/hash", "shell_snapshots", "tmp", "memories"):
        (env.codex_home / directory).mkdir(parents=True)
    (env.codex_home / ".tmp" / "plugins" / ".git").mkdir(parents=True)
    system_skills = env.codex_home / "skills" / ".system"
    system_skills.mkdir(parents=True)
    (system_skills / ".codex-system-skills.marker").write_text("22c0ca9bd55ca4ff", encoding="utf-8")
    for name in ("imagegen", "openai-docs", "plugin-creator", "skill-creator", "skill-installer"):
        (system_skills / name).mkdir()

    assert audit_vault_materialization(env, artifact_dir=artifact_dir) is None


def test_vault_audit_rejects_user_skill_materialization(tmp_path):
    artifact_dir = tmp_path / "artifact"
    env = _vault_environment(artifact_dir)
    env.codex_home.mkdir(parents=True)
    (env.codex_home / "auth.json").write_text("{}", encoding="utf-8")
    user_skill = env.codex_home / "skills" / "rail"
    user_skill.mkdir(parents=True)
    (user_skill / "SKILL.md").write_text("# Rail\n", encoding="utf-8")

    violation = audit_vault_materialization(env, artifact_dir=artifact_dir)

    assert violation == "skill materialization is not allowed"


def test_vault_audit_rejects_unmarked_system_skills(tmp_path):
    artifact_dir = tmp_path / "artifact"
    env = _vault_environment(artifact_dir)
    env.codex_home.mkdir(parents=True)
    (env.codex_home / "auth.json").write_text("{}", encoding="utf-8")
    (env.codex_home / "skills" / ".system" / "openai-docs").mkdir(parents=True)

    violation = audit_vault_materialization(env, artifact_dir=artifact_dir)

    assert violation == "skill materialization is not allowed"


def test_vault_audit_rejects_plugin_materialization_outside_cache(tmp_path):
    artifact_dir = tmp_path / "artifact"
    env = _vault_environment(artifact_dir)
    env.codex_home.mkdir(parents=True)
    (env.codex_home / "auth.json").write_text("{}", encoding="utf-8")
    (env.codex_home / "plugins" / "custom-plugin").mkdir(parents=True)

    violation = audit_vault_materialization(env, artifact_dir=artifact_dir)

    assert violation == "plugin materialization is not allowed"


def test_vault_audit_rejects_unexpected_config_toml(tmp_path):
    artifact_dir = tmp_path / "artifact"
    env = _vault_environment(artifact_dir)
    env.codex_home.mkdir(parents=True)
    (env.codex_home / "auth.json").write_text("{}", encoding="utf-8")
    (env.codex_home / "config.toml").write_text("model = 'user'\n", encoding="utf-8")

    violation = audit_vault_materialization(env, artifact_dir=artifact_dir)

    assert violation == "unexpected config inheritance is not allowed"


def test_vault_audit_rejects_symlink_inside_allowed_material(tmp_path):
    artifact_dir = tmp_path / "artifact"
    env = _vault_environment(artifact_dir)
    env.codex_home.mkdir(parents=True)
    (env.codex_home / "auth.json").write_text("{}", encoding="utf-8")
    cache_dir = env.codex_home / "plugins" / "cache"
    cache_dir.mkdir(parents=True)
    outside = tmp_path / "outside"
    outside.mkdir()
    (cache_dir / "escape").symlink_to(outside, target_is_directory=True)

    violation = audit_vault_materialization(env, artifact_dir=artifact_dir)

    assert violation == "unsafe vault material"


def _vault_environment(artifact_dir: Path) -> VaultEnvironment:
    codex_home = artifact_dir / "actor_runtime" / "codex_home"
    temp_dir = artifact_dir / "actor_runtime" / "tmp"
    evidence_dir = artifact_dir / "actor_runtime" / "evidence"
    temp_dir.mkdir(parents=True)
    evidence_dir.mkdir(parents=True)
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
