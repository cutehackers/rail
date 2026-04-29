from __future__ import annotations

from pathlib import Path
from typing import Literal

from pydantic import BaseModel, ConfigDict

CredentialCategory = Literal["operator_env", "operator_keychain", "ci_secret", "target_env", "target_file", "local_file"]

_ALLOWED_CATEGORIES = {"operator_env", "operator_keychain", "ci_secret"}


class CredentialSource(BaseModel):
    model_config = ConfigDict(extra="forbid")

    category: CredentialCategory
    name: str
    value: str | None = None
    path: Path | None = None


def validate_credential_source(source: CredentialSource, project_root: Path) -> CredentialSource:
    if source.category not in _ALLOWED_CATEGORIES:
        raise ValueError(f"credential source category is not allowed: {source.category}")

    if source.path is not None and _is_inside(source.path, project_root):
        raise ValueError("target-local credential paths are not allowed")

    return source


def build_actor_environment(sources: list[CredentialSource], project_root: Path) -> dict[str, str]:
    env: dict[str, str] = {}
    for source in sources:
        validate_credential_source(source, project_root)
        if source.value is not None:
            env[source.name] = source.value
    return env


def _is_inside(path: Path, parent: Path) -> bool:
    try:
        path.resolve(strict=False).relative_to(parent.resolve(strict=False))
    except ValueError:
        return False
    return True
