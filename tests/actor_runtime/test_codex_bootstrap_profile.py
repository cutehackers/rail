from __future__ import annotations

import os

from rail.actor_runtime.codex_bootstrap_profile import bootstrap_profile_violation


def test_bootstrap_profile_allows_passive_codex_material(tmp_path):
    codex_home = tmp_path / "codex_home"
    codex_home.mkdir()
    for directory in (
        "cache/codex_apps_tools",
        "plugins/cache/openai-curated/github/hash",
        "shell_snapshots",
        "tmp",
        "memories",
    ):
        (codex_home / directory).mkdir(parents=True)
    (codex_home / ".tmp" / "plugins" / ".git").mkdir(parents=True)
    (codex_home / "installation_id").write_text("id\n", encoding="utf-8")
    (codex_home / "models_cache.json").write_text("{}", encoding="utf-8")
    (codex_home / "config.toml").write_text('[plugins."github@openai-curated"]\nenabled = true\n', encoding="utf-8")
    system_skills = codex_home / "skills" / ".system"
    system_skills.mkdir(parents=True)
    (system_skills / ".codex-system-skills.marker").write_text("marker\n", encoding="utf-8")
    (system_skills / "openai-docs").mkdir()

    violations = [bootstrap_profile_violation(path, codex_home=codex_home) for path in codex_home.iterdir()]

    assert all(violation is None for violation in violations)


def test_bootstrap_profile_rejects_custom_plugin_material(tmp_path):
    codex_home = tmp_path / "codex_home"
    custom_plugin = codex_home / "plugins" / "custom"
    custom_plugin.mkdir(parents=True)

    violation = bootstrap_profile_violation(codex_home / "plugins", codex_home=codex_home)

    assert violation is not None
    assert violation.code == "user_plugin_materialized"
    assert violation.audit_layer == "provenance"


def test_bootstrap_profile_rejects_plugin_cache_file_material(tmp_path):
    codex_home = tmp_path / "codex_home"
    plugins = codex_home / "plugins"
    plugins.mkdir(parents=True)
    (plugins / "cache").write_text("not a cache directory\n", encoding="utf-8")

    violation = bootstrap_profile_violation(codex_home / "plugins", codex_home=codex_home)

    assert violation is not None
    assert violation.code == "unsafe_vault_material"
    assert violation.audit_layer == "materialization"


def test_bootstrap_profile_rejects_hardlinked_allowed_file_material(tmp_path):
    codex_home = tmp_path / "codex_home"
    codex_home.mkdir()
    outside = tmp_path / "outside-models-cache.json"
    outside.write_text("{}", encoding="utf-8")
    os.link(outside, codex_home / "models_cache.json")

    violation = bootstrap_profile_violation(codex_home / "models_cache.json", codex_home=codex_home)

    assert violation is not None
    assert violation.code == "unsafe_vault_material"
    assert violation.audit_layer == "materialization"


def test_bootstrap_profile_rejects_system_skill_file_material(tmp_path):
    codex_home = tmp_path / "codex_home"
    system_skills = codex_home / "skills" / ".system"
    system_skills.mkdir(parents=True)
    (system_skills / ".codex-system-skills.marker").write_text("marker\n", encoding="utf-8")
    (system_skills / "openai-docs").write_text("# OpenAI docs\n", encoding="utf-8")

    violation = bootstrap_profile_violation(codex_home / "skills", codex_home=codex_home)

    assert violation is not None
    assert violation.code == "user_skill_materialized"
    assert violation.audit_layer == "provenance"
