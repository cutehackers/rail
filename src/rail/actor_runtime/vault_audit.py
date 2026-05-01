from __future__ import annotations

from pathlib import Path

from rail.actor_runtime.vault_env import VaultEnvironment

_ALLOWED_ENV_KEYS = {"PATH", "HOME", "CODEX_HOME", "TMPDIR", "TMP", "TEMP"}
_ALLOWED_AUTH_MATERIAL = {"auth.json"}
_ALLOWED_OPERATIONAL_DIRS = {"log", "memories", "tmp"}
_TRUSTED_PROCESS_PATH = "/usr/bin:/bin"
_EVENT_AUDIT_KEYS = ("type", "event", "kind", "tool", "name", "server", "source", "path", "category")
_FORBIDDEN_CODEX_HOME_ENTRIES = {
    "skills": "skill materialization is not allowed",
    "plugins": "plugin materialization is not allowed",
    "mcp": "MCP config materialization is not allowed",
    "hooks": "hook materialization is not allowed",
    "rules": "user rule materialization is not allowed",
    "config.toml": "unexpected config inheritance is not allowed",
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
            materialization_violation = _forbidden_codex_home_entry_violation(child.name)
            if materialization_violation is not None:
                return materialization_violation
            if child.name in _ALLOWED_OPERATIONAL_DIRS:
                if child.is_symlink() or not child.is_dir():
                    return "unsafe vault material"
                continue
            if child.name not in _ALLOWED_AUTH_MATERIAL:
                return "auth material outside the allowlist is not allowed"
            if child.is_symlink():
                return "unsafe vault material"
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


def _forbidden_codex_home_entry_violation(name: str) -> str | None:
    return _FORBIDDEN_CODEX_HOME_ENTRIES.get(name)


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
