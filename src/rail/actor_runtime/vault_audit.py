from __future__ import annotations

import tomllib
from pathlib import Path

from rail.actor_runtime.vault_env import VaultEnvironment

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


def audit_vault_materialization(vault_environment: VaultEnvironment, *, artifact_dir: Path) -> str | None:
    artifact_root = artifact_dir.resolve(strict=False)
    for path in (vault_environment.codex_home, vault_environment.evidence_dir, vault_environment.temp_dir):
        resolved = path.resolve(strict=False)
        if not _is_relative_to(resolved, artifact_root):
            return "vault materialization escaped artifact directory"
        if path.is_symlink():
            return "unsafe vault material"

    unexpected_env = sorted(set(vault_environment.environ) - _ALLOWED_ENV_KEYS)
    if unexpected_env:
        return "unexpected config inheritance is not allowed"
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
            return "unexpected config inheritance is not allowed"

    unexpected_auth = sorted(set(vault_environment.copied_auth_material) - _ALLOWED_AUTH_MATERIAL)
    if unexpected_auth:
        return "auth material outside the allowlist is not allowed"

    if vault_environment.codex_home.exists():
        for child in vault_environment.codex_home.iterdir():
            materialization_violation = _codex_home_entry_violation(child)
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


def _codex_home_entry_violation(path: Path) -> str | None:
    forbidden_violation = _FORBIDDEN_CODEX_HOME_ENTRIES.get(path.name)
    if forbidden_violation is not None:
        return forbidden_violation
    if path.name in _ALLOWED_AUTH_MATERIAL:
        if path.is_symlink() or not path.is_file():
            return "unsafe vault material"
        return None
    if path.name in _ALLOWED_OPERATIONAL_DIRS:
        if path.is_symlink() or not path.is_dir() or _contains_symlink(path):
            return "unsafe vault material"
        return None
    if path.name in _ALLOWED_OPERATIONAL_FILES:
        if path.is_symlink() or not path.is_file():
            return "unsafe vault material"
        return None
    if path.name == "config.toml":
        return _config_toml_violation(path)
    if path.name == "skills":
        return _skills_materialization_violation(path)
    if path.name == "plugins":
        return _plugins_materialization_violation(path)
    return "auth material outside the allowlist is not allowed"


def _config_toml_violation(path: Path) -> str | None:
    if path.is_symlink() or not path.is_file():
        return "unsafe vault material"
    try:
        config = tomllib.loads(path.read_text(encoding="utf-8"))
    except (OSError, tomllib.TOMLDecodeError, UnicodeDecodeError):
        return "unexpected config inheritance is not allowed"
    if set(config) != {"plugins"}:
        return "unexpected config inheritance is not allowed"
    plugins = config.get("plugins")
    if not isinstance(plugins, dict) or not plugins:
        return "unexpected config inheritance is not allowed"
    for name, settings in plugins.items():
        if not isinstance(name, str) or not name.endswith("@openai-curated"):
            return "unexpected config inheritance is not allowed"
        if settings != {"enabled": True}:
            return "unexpected config inheritance is not allowed"
    return None


def _skills_materialization_violation(path: Path) -> str | None:
    if path.is_symlink() or not path.is_dir() or _contains_symlink(path):
        return "unsafe vault material"
    children = {child.name for child in path.iterdir()}
    if children != {".system"}:
        return "skill materialization is not allowed"
    system_skills = path / ".system"
    marker = system_skills / ".codex-system-skills.marker"
    if not marker.is_file() or marker.is_symlink():
        return "skill materialization is not allowed"
    system_children = {child.name for child in system_skills.iterdir()}
    allowed_children = _ALLOWED_CODEX_SYSTEM_SKILLS | {".codex-system-skills.marker"}
    if not system_children <= allowed_children:
        return "skill materialization is not allowed"
    return None


def _plugins_materialization_violation(path: Path) -> str | None:
    if path.is_symlink() or not path.is_dir() or _contains_symlink(path):
        return "unsafe vault material"
    children = {child.name for child in path.iterdir()}
    if not children <= {"cache"}:
        return "plugin materialization is not allowed"
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
