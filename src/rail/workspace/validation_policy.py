from __future__ import annotations

import shlex
from pathlib import Path

import yaml

from rail.workspace.validation_runner import ValidationCommand


def load_policy_validation_commands(target_root: Path) -> list[ValidationCommand]:
    policy_path = target_root / ".harness" / "supervisor" / "execution_policy.yaml"
    if not policy_path.is_file():
        raise ValueError("validation commands are not configured")
    payload = yaml.safe_load(policy_path.read_text(encoding="utf-8")) or {}
    if not isinstance(payload, dict):
        raise ValueError("validation policy must be a mapping")
    validation = payload.get("validation")
    if not isinstance(validation, dict):
        raise ValueError("validation commands are not configured")
    commands = validation.get("commands")
    if not isinstance(commands, list) or not commands:
        raise ValueError("validation commands are not configured")
    return [_command_from_policy(item) for item in commands]


def _command_from_policy(item: object) -> ValidationCommand:
    if isinstance(item, str):
        argv = shlex.split(item)
    elif isinstance(item, list):
        argv = [str(part) for part in item]
    else:
        raise ValueError("validation command must be a string or argv list")
    return ValidationCommand(argv=argv, source="policy")
