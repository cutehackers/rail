from __future__ import annotations

import tomllib
from pathlib import Path
from typing import TYPE_CHECKING

from rail.workspace.isolation import is_hardlink

if TYPE_CHECKING:
    from rail.actor_runtime.vault_audit import VaultAuditLayer, VaultAuditViolation

ALLOWED_OPERATIONAL_DIRS = {"cache", "log", "memories", "shell_snapshots", "tmp", ".tmp"}
ALLOWED_OPERATIONAL_FILES = {"installation_id", "models_cache.json"}
ALLOWED_CODEX_SYSTEM_SKILLS = {"imagegen", "openai-docs", "plugin-creator", "skill-creator", "skill-installer"}
FORBIDDEN_CODEX_HOME_ENTRIES = {
    "mcp": ("mcp_config_materialized", "MCP config materialization is not allowed"),
    "hooks": ("hook_materialized", "hook materialization is not allowed"),
    "rules": ("user_rule_materialized", "user rule materialization is not allowed"),
    "config.json": ("inherited_config_applied", "unexpected config inheritance is not allowed"),
    "settings.json": ("inherited_config_applied", "unexpected config inheritance is not allowed"),
}


def bootstrap_profile_violation(path: Path, *, codex_home: Path) -> VaultAuditViolation | None:
    forbidden_violation = FORBIDDEN_CODEX_HOME_ENTRIES.get(path.name)
    if forbidden_violation is not None:
        code, reason = forbidden_violation
        return _violation(code=code, reason=reason, audit_layer="provenance", path=path, codex_home=codex_home)
    if path.name in ALLOWED_OPERATIONAL_DIRS:
        if path.is_symlink() or not path.is_dir() or _contains_unsafe_link(path):
            return _unsafe_vault_material_violation(path, codex_home=codex_home)
        return None
    if path.name in ALLOWED_OPERATIONAL_FILES:
        if _is_unsafe_file(path):
            return _unsafe_vault_material_violation(path, codex_home=codex_home)
        return None
    if path.name == "config.toml":
        return _config_toml_violation(path, codex_home=codex_home)
    if path.name == "skills":
        return _skills_materialization_violation(path, codex_home=codex_home)
    if path.name == "plugins":
        return _plugins_materialization_violation(path, codex_home=codex_home)
    return _violation(
        code="bootstrap_profile_mismatch",
        reason="Codex bootstrap material is outside the passive profile",
        audit_layer="bootstrap",
        path=path,
        codex_home=codex_home,
    )


def _config_toml_violation(path: Path, *, codex_home: Path) -> VaultAuditViolation | None:
    if _is_unsafe_file(path):
        return _unsafe_vault_material_violation(path, codex_home=codex_home)
    try:
        config = tomllib.loads(path.read_text(encoding="utf-8"))
    except (OSError, tomllib.TOMLDecodeError, UnicodeDecodeError):
        return _inherited_config_violation(path, codex_home=codex_home)
    if set(config) != {"plugins"}:
        return _inherited_config_violation(path, codex_home=codex_home)
    plugins = config.get("plugins")
    if not isinstance(plugins, dict) or not plugins:
        return _inherited_config_violation(path, codex_home=codex_home)
    for name, settings in plugins.items():
        if not isinstance(name, str) or not name.endswith("@openai-curated"):
            return _inherited_config_violation(path, codex_home=codex_home)
        if settings != {"enabled": True}:
            return _inherited_config_violation(path, codex_home=codex_home)
    return None


def _skills_materialization_violation(path: Path, *, codex_home: Path) -> VaultAuditViolation | None:
    if path.is_symlink() or not path.is_dir() or _contains_unsafe_link(path):
        return _unsafe_vault_material_violation(path, codex_home=codex_home)
    children = {child.name for child in path.iterdir()}
    if children != {".system"}:
        user_skill = next((child for child in path.iterdir() if child.name != ".system"), path)
        return _violation(
            code="user_skill_materialized",
            reason="user-controlled skill materialized in actor-local CODEX_HOME",
            audit_layer="provenance",
            path=user_skill,
            codex_home=codex_home,
        )
    system_skills = path / ".system"
    marker = system_skills / ".codex-system-skills.marker"
    if _is_unsafe_file(marker):
        return _skill_profile_mismatch(path, codex_home=codex_home)
    system_children = {child.name for child in system_skills.iterdir()}
    allowed_children = ALLOWED_CODEX_SYSTEM_SKILLS | {".codex-system-skills.marker"}
    if not system_children <= allowed_children:
        return _skill_profile_mismatch(path, codex_home=codex_home)
    for child in system_skills.iterdir():
        if child.name == ".codex-system-skills.marker":
            continue
        if child.is_symlink() or not child.is_dir() or _contains_unsafe_link(child):
            return _violation(
                code="user_skill_materialized",
                reason="user-controlled skill materialized in actor-local CODEX_HOME",
                audit_layer="provenance",
                path=child,
                codex_home=codex_home,
            )
    return None


def _plugins_materialization_violation(path: Path, *, codex_home: Path) -> VaultAuditViolation | None:
    if path.is_symlink() or not path.is_dir() or _contains_unsafe_link(path):
        return _unsafe_vault_material_violation(path, codex_home=codex_home)
    children = {child.name for child in path.iterdir()}
    if not children <= {"cache"}:
        user_plugin = next((child for child in path.iterdir() if child.name != "cache"), path)
        return _violation(
            code="user_plugin_materialized",
            reason="plugin materialization is not allowed",
            audit_layer="provenance",
            path=user_plugin,
            codex_home=codex_home,
        )
    plugin_cache = path / "cache"
    if plugin_cache.exists() and (plugin_cache.is_symlink() or not plugin_cache.is_dir() or _contains_unsafe_link(plugin_cache)):
        return _unsafe_vault_material_violation(plugin_cache, codex_home=codex_home)
    return None


def _unsafe_vault_material_violation(path: Path, *, codex_home: Path) -> VaultAuditViolation:
    return _violation(
        code="unsafe_vault_material",
        reason="unsafe vault material",
        audit_layer="materialization",
        path=path,
        codex_home=codex_home,
    )


def _inherited_config_violation(path: Path, *, codex_home: Path) -> VaultAuditViolation:
    return _violation(
        code="inherited_config_applied",
        reason="unexpected config inheritance is not allowed",
        audit_layer="provenance",
        path=path,
        codex_home=codex_home,
    )


def _skill_profile_mismatch(path: Path, *, codex_home: Path) -> VaultAuditViolation:
    return _violation(
        code="bootstrap_profile_mismatch",
        reason="skill materialization is not allowed",
        audit_layer="bootstrap",
        path=path,
        codex_home=codex_home,
    )


def _violation(
    *,
    code: str,
    reason: str,
    audit_layer: VaultAuditLayer,
    path: Path,
    codex_home: Path,
) -> VaultAuditViolation:
    from rail.actor_runtime.vault_audit import VaultAuditViolation

    path_ref = None
    try:
        path_ref = path.relative_to(codex_home).as_posix()
    except ValueError:
        path_ref = None
    return VaultAuditViolation(code=code, reason=reason, audit_layer=audit_layer, path_ref=path_ref)


def _is_unsafe_file(path: Path) -> bool:
    return path.is_symlink() or not path.is_file() or is_hardlink(path)


def _contains_unsafe_link(path: Path) -> bool:
    return any(child.is_symlink() or (child.is_file() and is_hardlink(child)) for child in path.rglob("*"))
