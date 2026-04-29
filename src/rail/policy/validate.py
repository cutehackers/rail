from __future__ import annotations

import hashlib
import json
from typing import Any

from rail.policy.schema import ActorRuntimePolicyV2

_MUTATION_ORDER = {"read_only": 0, "patch_bundle": 1, "direct": 2}
_NETWORK_ORDER = {"disabled": 0, "restricted": 1, "enabled": 2}
_APPROVAL_ORDER = {"never": 0, "on_request": 1, "always": 2}


def narrow_policy(base: ActorRuntimePolicyV2, overlay: ActorRuntimePolicyV2) -> ActorRuntimePolicyV2:
    if overlay.runtime.provider != base.runtime.provider:
        raise ValueError("runtime.provider cannot change")
    _reject_string_change("runtime.model", base.runtime.model, overlay.runtime.model)
    _reject_greater("runtime.timeout_seconds", base.runtime.timeout_seconds, overlay.runtime.timeout_seconds)
    _reject_greater("actor_runtime.max_actor_turns", base.actor_runtime.max_actor_turns, overlay.actor_runtime.max_actor_turns)
    if overlay.actor_runtime.direct_target_mutation:
        raise ValueError("direct_target_mutation is not allowed")
    _reject_order_broaden(
        "workspace.mutation_mode", _MUTATION_ORDER, base.workspace.mutation_mode, overlay.workspace.mutation_mode
    )
    _reject_order_broaden("workspace.network_mode", _NETWORK_ORDER, base.workspace.network_mode, overlay.workspace.network_mode)
    _reject_string_change("workspace.sandbox_mode", base.workspace.sandbox_mode, overlay.workspace.sandbox_mode)
    _narrow_shell_tool(base, overlay)
    _narrow_filesystem_tool(base, overlay)
    _narrow_simple_tool("network", base.tools.network.enabled, overlay.tools.network.enabled)
    _reject_allowlist_broaden("network.allowlist", base.tools.network.allowlist, overlay.tools.network.allowlist)
    _narrow_simple_tool("mcp", base.tools.mcp.enabled, overlay.tools.mcp.enabled)
    _reject_allowlist_broaden("mcp.allowlist", base.tools.mcp.allowlist, overlay.tools.mcp.allowlist)
    _reject_bool_broaden("capabilities.patch_apply", base.capabilities.patch_apply, overlay.capabilities.patch_apply)
    _reject_bool_broaden("capabilities.validation", base.capabilities.validation, overlay.capabilities.validation)
    _reject_bool_broaden("capabilities.binary_files", base.capabilities.binary_files, overlay.capabilities.binary_files)
    _reject_order_broaden("approval_policy.mode", _APPROVAL_ORDER, base.approval_policy.mode, overlay.approval_policy.mode)
    return overlay


def digest_policy(policy: ActorRuntimePolicyV2) -> str:
    payload = json.dumps(policy.model_dump(mode="json"), sort_keys=True, separators=(",", ":"))
    return "sha256:" + hashlib.sha256(payload.encode("utf-8")).hexdigest()


def _narrow_shell_tool(base: ActorRuntimePolicyV2, overlay: ActorRuntimePolicyV2) -> None:
    _narrow_simple_tool("shell", base.tools.shell.enabled, overlay.tools.shell.enabled)
    _reject_allowlist_broaden("shell.allowlist", base.tools.shell.allowlist, overlay.tools.shell.allowlist)
    _reject_greater("shell.timeout_seconds", base.tools.shell.timeout_seconds, overlay.tools.shell.timeout_seconds)
    _reject_greater("shell.max_output_bytes", base.tools.shell.max_output_bytes, overlay.tools.shell.max_output_bytes)


def _narrow_filesystem_tool(base: ActorRuntimePolicyV2, overlay: ActorRuntimePolicyV2) -> None:
    _narrow_simple_tool("filesystem", base.tools.filesystem.enabled, overlay.tools.filesystem.enabled)
    _reject_allowlist_broaden("filesystem.allowlist", base.tools.filesystem.allowlist, overlay.tools.filesystem.allowlist)
    _reject_greater("filesystem.max_file_bytes", base.tools.filesystem.max_file_bytes, overlay.tools.filesystem.max_file_bytes)


def _narrow_simple_tool(name: str, base_enabled: bool, overlay_enabled: bool) -> None:
    _reject_bool_broaden(f"{name}.enabled", base_enabled, overlay_enabled)


def _reject_bool_broaden(field: str, base: bool, overlay: bool) -> None:
    if overlay and not base:
        raise ValueError(f"{field} cannot broaden policy")


def _reject_string_change(field: str, base: Any, overlay: Any) -> None:
    if overlay != base:
        raise ValueError(f"{field} cannot broaden or change policy")


def _reject_greater(field: str, base: int, overlay: int) -> None:
    if overlay > base:
        raise ValueError(f"{field} cannot exceed default policy")


def _reject_allowlist_broaden(field: str, base: list[str], overlay: list[str]) -> None:
    if not set(overlay).issubset(set(base)):
        raise ValueError(f"{field} cannot broaden policy")


def _reject_order_broaden(field: str, order: dict[str, int], base: str, overlay: str) -> None:
    if order[overlay] > order[base]:
        raise ValueError(f"{field} cannot broaden policy")
