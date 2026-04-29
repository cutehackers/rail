from __future__ import annotations

from pathlib import Path

import yaml

from rail.policy.schema import ActorRuntimePolicyV2
from rail.policy.validate import narrow_policy

_DEFAULT_POLICY_PATH = Path("assets/defaults/supervisor/actor_runtime.yaml")
_TARGET_POLICY_PATH = Path(".harness/supervisor/actor_runtime.yaml")


def load_effective_policy(project_root: Path) -> ActorRuntimePolicyV2:
    base = _load_policy(_DEFAULT_POLICY_PATH)
    target_policy_path = project_root / _TARGET_POLICY_PATH
    if not target_policy_path.is_file():
        return base
    return narrow_policy(base, _load_policy(target_policy_path))


def _load_policy(path: Path) -> ActorRuntimePolicyV2:
    with path.open(encoding="utf-8") as stream:
        payload = yaml.safe_load(stream) or {}
    return ActorRuntimePolicyV2.model_validate(payload)
