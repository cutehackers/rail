from __future__ import annotations

import os
import stat
from pathlib import Path

import yaml

from rail.policy.schema import ActorRuntimePolicyV2
from rail.policy.validate import narrow_policy
from rail.resources import load_default_asset_yaml

_TARGET_POLICY_PATH = Path(".harness/supervisor/actor_runtime.yaml")
_OPERATOR_POLICY_ENV = "RAIL_OPERATOR_ACTOR_RUNTIME_POLICY"


def load_effective_policy(project_root: Path) -> ActorRuntimePolicyV2:
    base = _load_base_policy(project_root)
    target_policy_path = project_root / _TARGET_POLICY_PATH
    if not target_policy_path.is_file():
        return base
    return narrow_policy(base, _load_policy(target_policy_path))


def _load_base_policy(project_root: Path) -> ActorRuntimePolicyV2:
    operator_policy = os.environ.get(_OPERATOR_POLICY_ENV)
    if operator_policy is None:
        return ActorRuntimePolicyV2.model_validate(load_default_asset_yaml("defaults/supervisor/actor_runtime.yaml"))
    path = Path(operator_policy)
    _validate_operator_policy_path(path, project_root)
    return _load_policy(path)


def _load_policy(path: Path) -> ActorRuntimePolicyV2:
    with path.open(encoding="utf-8") as stream:
        payload = yaml.safe_load(stream) or {}
    return ActorRuntimePolicyV2.model_validate(payload)


def _validate_operator_policy_path(path: Path, project_root: Path) -> None:
    if not path.is_absolute():
        raise ValueError(f"{_OPERATOR_POLICY_ENV} must be an absolute path")
    if not path.exists():
        raise ValueError(f"{_OPERATOR_POLICY_ENV} must exist")
    if not path.is_file():
        raise ValueError(f"{_OPERATOR_POLICY_ENV} must point to a file")
    if path.is_symlink():
        raise ValueError(f"{_OPERATOR_POLICY_ENV} must not be symlinked")
    try:
        path.resolve(strict=True).relative_to(project_root.resolve(strict=False))
    except ValueError:
        pass
    else:
        raise ValueError(f"{_OPERATOR_POLICY_ENV} must not be inside the target repository")
    mode = path.stat().st_mode
    if mode & (stat.S_IWGRP | stat.S_IWOTH):
        raise ValueError(f"{_OPERATOR_POLICY_ENV} must not be group-writable or world-writable")
