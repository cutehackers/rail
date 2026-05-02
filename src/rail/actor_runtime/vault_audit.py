from __future__ import annotations

import tomllib
from pathlib import Path
from typing import Literal

from pydantic import BaseModel, ConfigDict
from rail.actor_runtime.vault_env import VaultEnvironment

VaultAuditLayer = Literal["materialization", "bootstrap", "provenance", "capability"]


class VaultAuditViolation(BaseModel):
    model_config = ConfigDict(extra="forbid")

    category: Literal["policy"] = "policy"
    code: str
    reason: str
    audit_layer: VaultAuditLayer
    path_ref: str | None = None


_ALLOWED_ENV_KEYS = {"PATH", "HOME", "CODEX_HOME", "TMPDIR", "TMP", "TEMP"}
_ALLOWED_AUTH_MATERIAL = {"auth.json"}
_ALLOWED_OPERATIONAL_DIRS = {"cache", "log", "memories", "shell_snapshots", "tmp", ".tmp"}
_ALLOWED_OPERATIONAL_FILES = {"installation_id", "models_cache.json"}
_ALLOWED_CODEX_SYSTEM_SKILLS = {"imagegen", "openai-docs", "plugin-creator", "skill-creator", "skill-installer"}
_TRUSTED_PROCESS_PATH = "/usr/bin:/bin"
_EVENT_AUDIT_KEYS = ("type", "event", "kind", "tool", "name", "server", "source", "path", "category")
_FORBIDDEN_CODEX_HOME_ENTRIES = {
    "mcp": "MCP config materialization is not allowed",
    "hooks": "hook materialization is not allowed",
    "rules": "user rule materialization is not allowed",
    "config.json": "unexpected config inheritance is not allowed",
    "settings.json": "unexpected config inheritance is not allowed",
}


def audit_vault_materialization(vault_environment: VaultEnvironment, *, artifact_dir: Path) -> VaultAuditViolation | None:
    artifact_root = artifact_dir.resolve(strict=False)
    for path in (vault_environment.codex_home, vault_environment.evidence_dir, vault_environment.temp_dir):
        resolved = path.resolve(strict=False)
        if not _is_relative_to(resolved, artifact_root):
            return _violation(
                code="unsafe_vault_material",
                reason="vault materialization escaped artifact directory",
                audit_layer="materialization",
                path=path,
                artifact_dir=artifact_dir,
            )
        if path.is_symlink():
            return _violation(
                code="unsafe_vault_material",
                reason="unsafe vault material",
                audit_layer="materialization",
                path=path,
                artifact_dir=artifact_dir,
            )

    unexpected_env = sorted(set(vault_environment.environ) - _ALLOWED_ENV_KEYS)
    if unexpected_env:
        return _violation(
            code="inherited_config_applied",
            reason="unexpected config inheritance is not allowed",
            audit_layer="provenance",
        )
    expected_env = {
        "PATH": _TRUSTED_PROCESS_PATH,
        "HOME": vault_environment.codex_home.as_posix(),
        "CODEX_HOME": vault_environment.codex_home.as_posix(),
        "TMPDIR": vault_environment.temp_dir.as_posix(),
        "TMP": vault_environment.temp_dir.as_posix(),
        "TEMP": vault_environment.temp_dir.as_posix(),
    }
    for key, value in expected_env.items():
        if vault_environment.environ.get(key) != value:
            return _violation(
                code="inherited_config_applied",
                reason="unexpected config inheritance is not allowed",
                audit_layer="provenance",
            )

    unexpected_auth = sorted(set(vault_environment.copied_auth_material) - _ALLOWED_AUTH_MATERIAL)
    if unexpected_auth:
        return _violation(
            code="unknown_auth_material",
            reason="auth material outside the allowlist is not allowed",
            audit_layer="materialization",
        )

    if vault_environment.codex_home.exists():
        for child in vault_environment.codex_home.iterdir():
            materialization_violation = _codex_home_entry_violation(child, artifact_dir=artifact_dir)
            if materialization_violation is not None:
                return materialization_violation
    return None


def audit_codex_event_contamination(events: list[dict[str, object]]) -> str | None:
    for event in events:
        for mapping in _event_dicts(event):
            lowered = " ".join(str(mapping.get(key)).lower() for key in _EVENT_AUDIT_KEYS if mapping.get(key) is not None)
            if "mcp" in lowered:
                return "MCP invocation is not allowed"
            if "plugin" in lowered:
                return "plugin invocation is not allowed"
            if "skill" in lowered:
                return "skill invocation is not allowed"
            if "hook" in lowered:
                return "hook materialization is not allowed"
            if "rule" in lowered:
                return "user rule materialization is not allowed"
            if "config" in lowered:
                return "unexpected config inheritance is not allowed"
    return None


def _violation(
    *,
    code: str,
    reason: str,
    audit_layer: VaultAuditLayer,
    path: Path | None = None,
    artifact_dir: Path | None = None,
) -> VaultAuditViolation:
    path_ref = None
    if path is not None and artifact_dir is not None:
        try:
            path_ref = path.relative_to(artifact_dir).as_posix()
        except ValueError:
            path_ref = None
    return VaultAuditViolation(code=code, reason=reason, audit_layer=audit_layer, path_ref=path_ref)


def _codex_home_entry_violation(path: Path, *, artifact_dir: Path) -> VaultAuditViolation | None:
    forbidden_violation = _FORBIDDEN_CODEX_HOME_ENTRIES.get(path.name)
    if forbidden_violation is not None:
        code = {
            "mcp": "mcp_config_materialized",
            "hooks": "hook_materialized",
            "rules": "user_rule_materialized",
            "config.json": "inherited_config_applied",
            "settings.json": "inherited_config_applied",
        }[path.name]
        return _violation(
            code=code,
            reason=forbidden_violation,
            audit_layer="provenance",
            path=path,
            artifact_dir=artifact_dir,
        )
    if path.name in _ALLOWED_AUTH_MATERIAL:
        if path.is_symlink() or not path.is_file():
            return _violation(
                code="unsafe_vault_material",
                reason="unsafe vault material",
                audit_layer="materialization",
                path=path,
                artifact_dir=artifact_dir,
            )
        return None
    if path.name in _ALLOWED_OPERATIONAL_DIRS:
        if path.is_symlink() or not path.is_dir() or _contains_symlink(path):
            return _violation(
                code="unsafe_vault_material",
                reason="unsafe vault material",
                audit_layer="materialization",
                path=path,
                artifact_dir=artifact_dir,
            )
        return None
    if path.name in _ALLOWED_OPERATIONAL_FILES:
        if path.is_symlink() or not path.is_file():
            return _violation(
                code="unsafe_vault_material",
                reason="unsafe vault material",
                audit_layer="materialization",
                path=path,
                artifact_dir=artifact_dir,
            )
        return None
    if path.name == "config.toml":
        return _config_toml_violation(path, artifact_dir=artifact_dir)
    if path.name == "skills":
        return _skills_materialization_violation(path, artifact_dir=artifact_dir)
    if path.name == "plugins":
        return _plugins_materialization_violation(path, artifact_dir=artifact_dir)
    return _violation(
        code="unknown_auth_material",
        reason="auth material outside the allowlist is not allowed",
        audit_layer="materialization",
        path=path,
        artifact_dir=artifact_dir,
    )


def _config_toml_violation(path: Path, *, artifact_dir: Path) -> VaultAuditViolation | None:
    if path.is_symlink() or not path.is_file():
        return _violation(
            code="unsafe_vault_material",
            reason="unsafe vault material",
            audit_layer="materialization",
            path=path,
            artifact_dir=artifact_dir,
        )
    try:
        config = tomllib.loads(path.read_text(encoding="utf-8"))
    except (OSError, tomllib.TOMLDecodeError, UnicodeDecodeError):
        return _inherited_config_violation(path, artifact_dir=artifact_dir)
    if set(config) != {"plugins"}:
        return _inherited_config_violation(path, artifact_dir=artifact_dir)
    plugins = config.get("plugins")
    if not isinstance(plugins, dict) or not plugins:
        return _inherited_config_violation(path, artifact_dir=artifact_dir)
    for name, settings in plugins.items():
        if not isinstance(name, str) or not name.endswith("@openai-curated"):
            return _inherited_config_violation(path, artifact_dir=artifact_dir)
        if settings != {"enabled": True}:
            return _inherited_config_violation(path, artifact_dir=artifact_dir)
    return None


def _inherited_config_violation(path: Path, *, artifact_dir: Path) -> VaultAuditViolation:
    return _violation(
        code="inherited_config_applied",
        reason="unexpected config inheritance is not allowed",
        audit_layer="provenance",
        path=path,
        artifact_dir=artifact_dir,
    )


def _skills_materialization_violation(path: Path, *, artifact_dir: Path) -> VaultAuditViolation | None:
    if path.is_symlink() or not path.is_dir() or _contains_symlink(path):
        return _violation(
            code="unsafe_vault_material",
            reason="unsafe vault material",
            audit_layer="materialization",
            path=path,
            artifact_dir=artifact_dir,
        )
    children = {child.name for child in path.iterdir()}
    if children != {".system"}:
        user_skill = next((child for child in path.iterdir() if child.name != ".system"), path)
        return _violation(
            code="user_skill_materialized",
            reason="user-controlled skill materialized in actor-local CODEX_HOME",
            audit_layer="provenance",
            path=user_skill,
            artifact_dir=artifact_dir,
        )
    system_skills = path / ".system"
    marker = system_skills / ".codex-system-skills.marker"
    if not marker.is_file() or marker.is_symlink():
        return _violation(
            code="bootstrap_profile_mismatch",
            reason="skill materialization is not allowed",
            audit_layer="bootstrap",
            path=path,
            artifact_dir=artifact_dir,
        )
    system_children = {child.name for child in system_skills.iterdir()}
    allowed_children = _ALLOWED_CODEX_SYSTEM_SKILLS | {".codex-system-skills.marker"}
    if not system_children <= allowed_children:
        return _violation(
            code="bootstrap_profile_mismatch",
            reason="skill materialization is not allowed",
            audit_layer="bootstrap",
            path=path,
            artifact_dir=artifact_dir,
        )
    return None


def _plugins_materialization_violation(path: Path, *, artifact_dir: Path) -> VaultAuditViolation | None:
    if path.is_symlink() or not path.is_dir() or _contains_symlink(path):
        return _violation(
            code="unsafe_vault_material",
            reason="unsafe vault material",
            audit_layer="materialization",
            path=path,
            artifact_dir=artifact_dir,
        )
    children = {child.name for child in path.iterdir()}
    if not children <= {"cache"}:
        user_plugin = next((child for child in path.iterdir() if child.name != "cache"), path)
        return _violation(
            code="user_plugin_materialized",
            reason="plugin materialization is not allowed",
            audit_layer="provenance",
            path=user_plugin,
            artifact_dir=artifact_dir,
        )
    return None


def _contains_symlink(path: Path) -> bool:
    return any(child.is_symlink() for child in path.rglob("*"))


def _event_dicts(event: dict[str, object]) -> list[dict[str, object]]:
    dicts: list[dict[str, object]] = []
    _append_event_dicts(event, dicts)
    return dicts


def _append_event_dicts(event: dict[str, object], dicts: list[dict[str, object]]) -> None:
    dicts.append(event)
    for key in ("item", "message", "msg"):
        child = event.get(key)
        if isinstance(child, dict):
            _append_event_dicts(child, dicts)
        elif isinstance(child, list):
            for item in child:
                if isinstance(item, dict):
                    _append_event_dicts(item, dicts)
    content = event.get("content")
    if isinstance(content, dict):
        _append_event_dicts(content, dicts)
    elif isinstance(content, list):
        for item in content:
            if isinstance(item, dict):
                _append_event_dicts(item, dicts)


def _is_relative_to(path: Path, parent: Path) -> bool:
    try:
        path.relative_to(parent)
    except ValueError:
        return False
    return True
