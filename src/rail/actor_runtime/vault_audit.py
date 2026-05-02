from __future__ import annotations

from pathlib import Path
from typing import Literal

from pydantic import BaseModel, ConfigDict
from rail.actor_runtime.codex_bootstrap_profile import bootstrap_profile_violation
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
_TRUSTED_PROCESS_PATH = "/usr/bin:/bin"
_EVENT_AUDIT_KEYS = ("type", "event", "kind", "tool", "name", "server", "source", "path", "category")


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
                if child.is_symlink() or not child.is_file():
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
