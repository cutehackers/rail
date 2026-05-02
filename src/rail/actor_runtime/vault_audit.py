from __future__ import annotations

from pathlib import Path
from typing import Literal

from pydantic import BaseModel, ConfigDict
from rail.actor_runtime.codex_bootstrap_profile import bootstrap_profile_violation
from rail.actor_runtime.vault_env import VaultEnvironment
from rail.workspace.isolation import is_hardlink

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
_TRUSTED_PROCESS_PATH = "/usr/bin:/bin"
_CAPABILITY_IDENTITY_KEYS = ("type", "kind", "tool", "name")
_MCP_IDENTITY_KEYS = ("type", "kind", "tool")
_BEHAVIOR_SIGNAL_KEYS = ("type", "kind", "category", "message")
_CAPABILITY_EVENT_TYPES = {
    "tool_call",
    "tool_invocation",
    "function_call",
    "skill_invocation",
    "skill_execution",
    "mcp_call",
    "hook_execution",
    "rule_applied",
    "config_loaded",
}
_PASSIVE_EVENT_CATEGORIES = {"plugin_cache", "skill_registry", "metadata", "discovery"}
_TOOL_EVENT_TYPES = {
    "tool_call",
    "tool_invocation",
    "function_call",
    "command_call",
    "command_execution",
    "exec_command",
    "exec_command_begin",
    "mcp_call",
    "mcp_invocation",
    "mcp_tool_call",
}
_SHELL_COMMAND_EVENT_TYPES = {"shell", "command_call", "command_execution", "exec_command", "exec_command_begin"}
_CAPABILITY_EXECUTION_TYPES = {"capability_call", "capability_execution", "capability_invocation"}
_BEHAVIOR_AFFECTING_SOURCES = {"user", "parent", "target", "unknown"}
_PASSIVE_EVENT_TERMS = {"plugin_cache", "skill_registry", "metadata", "discovery", "actor-local config inspected"}
_HOOK_RULE_CONFIG_EVENT_TYPES = {
    "hook_execution",
    "hook_applied",
    "rule_applied",
    "rule_execution",
    "config_loaded",
    "config_applied",
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
            if child.name in _ALLOWED_AUTH_MATERIAL:
                if child.is_symlink() or not child.is_file() or is_hardlink(child):
                    return _violation(
                        code="unsafe_vault_material",
                        reason="unsafe vault material",
                        audit_layer="materialization",
                        path=child,
                        artifact_dir=artifact_dir,
                    )
                continue
            materialization_violation = bootstrap_profile_violation(child, codex_home=vault_environment.codex_home)
            if materialization_violation is not None:
                return _with_artifact_path_ref(
                    materialization_violation,
                    codex_home=vault_environment.codex_home,
                    artifact_dir=artifact_dir,
                )
    return None


def audit_codex_event_capabilities(events: list[dict[str, object]]) -> VaultAuditViolation | None:
    for event in events:
        for mapping in _event_dicts(event):
            event_type = _event_token(mapping, "type")
            event_kind = _event_token(mapping, "kind")
            event_source = _event_source(mapping)
            identity = _event_identity_text(mapping)
            mcp_identity = _event_mcp_identity_text(mapping)
            behavior_signal = _event_behavior_signal_text(mapping)
            if _is_hook_rule_config_capability_event(event_type=event_type, event_kind=event_kind, behavior_signal=behavior_signal):
                return _hook_rule_config_violation(behavior_signal)
            if _is_passive_discovery_event(mapping, event_type=event_type, event_kind=event_kind):
                continue
            if _is_mcp_capability_event(mapping, event_type=event_type, event_kind=event_kind, identity=mcp_identity):
                return _violation(code="mcp_capability_used", reason="MCP invocation is not allowed", audit_layer="capability")
            if _is_plugin_capability_event(mapping, event_type=event_type, event_kind=event_kind, identity=identity):
                return _violation(code="plugin_capability_used", reason="plugin capability use is not allowed", audit_layer="capability")
            if _is_skill_capability_event(
                mapping,
                event_type=event_type,
                event_kind=event_kind,
                event_source=event_source,
                identity=identity,
            ):
                return _violation(code="skill_capability_used", reason="skill invocation is not allowed", audit_layer="capability")
    return None


def _is_passive_discovery_event(mapping: dict[str, object], *, event_type: str, event_kind: str) -> bool:
    category = _event_token(mapping, "category")
    message = _event_token(mapping, "message")
    if _is_tool_or_capability_call(mapping, event_type=event_type, event_kind=event_kind):
        return False
    passive_text = f"{category} {message}"
    return any(term in passive_text for term in _PASSIVE_EVENT_TERMS) or category in _PASSIVE_EVENT_CATEGORIES


def _is_mcp_capability_event(
    mapping: dict[str, object],
    *,
    event_type: str,
    event_kind: str,
    identity: str,
) -> bool:
    return "mcp" in identity and _is_tool_or_capability_call(mapping, event_type=event_type, event_kind=event_kind)


def _is_plugin_capability_event(
    mapping: dict[str, object],
    *,
    event_type: str,
    event_kind: str,
    identity: str,
) -> bool:
    if not _is_tool_or_capability_call(mapping, event_type=event_type, event_kind=event_kind):
        return False
    if _is_shell_command_event(mapping, event_type=event_type, event_kind=event_kind):
        return False
    return "plugin" in identity or _is_non_shell_tool_call(mapping, event_type=event_type, event_kind=event_kind)


def _is_skill_capability_event(
    mapping: dict[str, object],
    *,
    event_type: str,
    event_kind: str,
    event_source: str,
    identity: str,
) -> bool:
    if event_type in {"skill_invocation", "skill_execution"} or event_kind in {"skill_invocation", "skill_execution"}:
        return True
    if _is_behavior_affecting_capability_call(mapping, event_type=event_type, event_kind=event_kind) and event_source in _BEHAVIOR_AFFECTING_SOURCES:
        return True
    if "skill" not in identity:
        return False
    return _is_tool_or_capability_call(mapping, event_type=event_type, event_kind=event_kind) and event_source in _BEHAVIOR_AFFECTING_SOURCES


def _is_hook_rule_config_capability_event(*, event_type: str, event_kind: str, behavior_signal: str) -> bool:
    if event_type in _HOOK_RULE_CONFIG_EVENT_TYPES or event_kind in _HOOK_RULE_CONFIG_EVENT_TYPES:
        return True
    behavior_change_terms = ("applied", "executed", "loaded")
    return any(term in behavior_signal for term in ("hook", "rule", "config")) and any(term in behavior_signal for term in behavior_change_terms)


def _hook_rule_config_violation(value: str) -> VaultAuditViolation:
    if "hook" in value:
        return _violation(code="hook_capability_used", reason="hook materialization is not allowed", audit_layer="capability")
    if "rule" in value:
        return _violation(code="rule_capability_used", reason="user rule materialization is not allowed", audit_layer="capability")
    return _violation(code="inherited_config_applied", reason="unexpected config inheritance is not allowed", audit_layer="capability")


def _is_tool_or_capability_call(mapping: dict[str, object], *, event_type: str, event_kind: str) -> bool:
    return (
        event_type in _CAPABILITY_EVENT_TYPES
        or event_kind in _CAPABILITY_EVENT_TYPES
        or event_type in _TOOL_EVENT_TYPES
        or event_kind in _TOOL_EVENT_TYPES
        or event_type in _CAPABILITY_EXECUTION_TYPES
        or event_kind in _CAPABILITY_EXECUTION_TYPES
        or "tool" in mapping
    )


def _is_behavior_affecting_capability_call(mapping: dict[str, object], *, event_type: str, event_kind: str) -> bool:
    if _is_shell_command_event(mapping, event_type=event_type, event_kind=event_kind):
        return False
    if _is_explicit_capability_event(event_type=event_type, event_kind=event_kind):
        return True
    return _is_tool_or_capability_call(mapping, event_type=event_type, event_kind=event_kind)


def _is_explicit_capability_event(*, event_type: str, event_kind: str) -> bool:
    return (
        event_type in _CAPABILITY_EVENT_TYPES
        or event_kind in _CAPABILITY_EVENT_TYPES
        or event_type in _CAPABILITY_EXECUTION_TYPES
        or event_kind in _CAPABILITY_EXECUTION_TYPES
    )


def _is_shell_command_event(mapping: dict[str, object], *, event_type: str, event_kind: str) -> bool:
    if event_type in _SHELL_COMMAND_EVENT_TYPES or event_kind in _SHELL_COMMAND_EVENT_TYPES:
        return True
    identity = _event_identity_text(mapping)
    return (
        "shell" in identity
        and (event_type in _TOOL_EVENT_TYPES or event_kind in _TOOL_EVENT_TYPES)
        and ("tool" in mapping or "command" in mapping)
    )


def _is_non_shell_tool_call(mapping: dict[str, object], *, event_type: str, event_kind: str) -> bool:
    return event_type in _TOOL_EVENT_TYPES or event_kind in _TOOL_EVENT_TYPES or "tool" in mapping


def _event_token(mapping: dict[str, object], key: str) -> str:
    value = mapping.get(key)
    return value.lower() if isinstance(value, str) else ""


def _event_source(mapping: dict[str, object]) -> str:
    return _event_token(mapping, "source") or "unknown"


def _event_identity_text(mapping: dict[str, object]) -> str:
    return " ".join(str(mapping.get(key)).lower() for key in _CAPABILITY_IDENTITY_KEYS if mapping.get(key) is not None)


def _event_mcp_identity_text(mapping: dict[str, object]) -> str:
    return " ".join(str(mapping.get(key)).lower() for key in _MCP_IDENTITY_KEYS if mapping.get(key) is not None)


def _event_behavior_signal_text(mapping: dict[str, object]) -> str:
    return " ".join(str(mapping.get(key)).lower() for key in _BEHAVIOR_SIGNAL_KEYS if mapping.get(key) is not None)


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


def _with_artifact_path_ref(
    violation: VaultAuditViolation,
    *,
    codex_home: Path,
    artifact_dir: Path,
) -> VaultAuditViolation:
    if violation.path_ref is None:
        return violation
    try:
        codex_home_ref = codex_home.relative_to(artifact_dir).as_posix()
    except ValueError:
        return violation.model_copy(update={"path_ref": None})
    return violation.model_copy(update={"path_ref": f"{codex_home_ref}/{violation.path_ref}"})


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
