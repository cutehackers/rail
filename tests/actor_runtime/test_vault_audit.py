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

    assert violation is not None
    assert violation.code == "user_skill_materialized"
    assert violation.audit_layer == "provenance"
    assert violation.reason == "user-controlled skill materialized in actor-local CODEX_HOME"
    assert violation.path_ref == "actor_runtime/codex_home/skills/rail"


def test_vault_audit_reports_unknown_auth_material_code(tmp_path):
    artifact_dir = tmp_path / "artifact"
    env = _vault_environment(artifact_dir).model_copy(update={"copied_auth_material": ["auth.json", "session.db"]})
    env.codex_home.mkdir(parents=True)
    (env.codex_home / "auth.json").write_text("{}", encoding="utf-8")

    violation = audit_vault_materialization(env, artifact_dir=artifact_dir)

    assert violation is not None
    assert violation.code == "unknown_auth_material"
    assert violation.audit_layer == "materialization"
    assert violation.reason == "auth material outside the allowlist is not allowed"
    assert violation.path_ref is None


def test_vault_audit_rejects_unmarked_system_skills(tmp_path):
    artifact_dir = tmp_path / "artifact"
    env = _vault_environment(artifact_dir)
    env.codex_home.mkdir(parents=True)
    (env.codex_home / "auth.json").write_text("{}", encoding="utf-8")
    (env.codex_home / "skills" / ".system" / "openai-docs").mkdir(parents=True)

    violation = audit_vault_materialization(env, artifact_dir=artifact_dir)

    assert violation is not None
    assert violation.code == "bootstrap_profile_mismatch"
    assert violation.audit_layer == "bootstrap"
    assert violation.reason == "skill materialization is not allowed"
    assert violation.path_ref == "actor_runtime/codex_home/skills"


def test_vault_audit_rejects_plugin_materialization_outside_cache(tmp_path):
    artifact_dir = tmp_path / "artifact"
    env = _vault_environment(artifact_dir)
    env.codex_home.mkdir(parents=True)
    (env.codex_home / "auth.json").write_text("{}", encoding="utf-8")
    (env.codex_home / "plugins" / "custom-plugin").mkdir(parents=True)

    violation = audit_vault_materialization(env, artifact_dir=artifact_dir)

    assert violation is not None
    assert violation.code == "user_plugin_materialized"
    assert violation.audit_layer == "provenance"
    assert violation.reason == "plugin materialization is not allowed"
    assert violation.path_ref == "actor_runtime/codex_home/plugins/custom-plugin"


def test_vault_audit_rejects_unexpected_config_toml(tmp_path):
    artifact_dir = tmp_path / "artifact"
    env = _vault_environment(artifact_dir)
    env.codex_home.mkdir(parents=True)
    (env.codex_home / "auth.json").write_text("{}", encoding="utf-8")
    (env.codex_home / "config.toml").write_text("model = 'user'\n", encoding="utf-8")

    violation = audit_vault_materialization(env, artifact_dir=artifact_dir)

    assert violation is not None
    assert violation.code == "inherited_config_applied"
    assert violation.audit_layer == "provenance"
    assert violation.reason == "unexpected config inheritance is not allowed"
    assert violation.path_ref == "actor_runtime/codex_home/config.toml"


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

    assert violation is not None
    assert violation.code == "unsafe_vault_material"
    assert violation.audit_layer == "materialization"
    assert violation.reason == "unsafe vault material"
    assert violation.path_ref == "actor_runtime/codex_home/plugins"


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
