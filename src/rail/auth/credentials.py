from __future__ import annotations

import os
import stat
from collections.abc import Mapping
from pathlib import Path
from typing import Literal

from pydantic import BaseModel, ConfigDict

CredentialCategory = Literal["operator_env", "operator_keychain", "ci_secret", "target_env", "target_file", "local_file"]

_ALLOWED_CATEGORIES = {"operator_env", "operator_keychain", "ci_secret"}
_CODEX_AUTH_ALLOWLIST = {"auth.json"}


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


def validate_sdk_credential_format(source: CredentialSource) -> CredentialSource:
    if source.name == "OPENAI_API_KEY" and source.value is not None and not source.value.startswith("sk-"):
        raise ValueError("operator SDK invalid credential is configured")
    return source


def build_actor_environment(sources: list[CredentialSource], project_root: Path) -> dict[str, str]:
    env: dict[str, str] = {}
    for source in sources:
        validate_credential_source(source, project_root)
        if source.value is not None:
            env[source.name] = source.value
    return env


def discover_sdk_credential_sources(environ: Mapping[str, str] | None = None) -> list[CredentialSource]:
    environ = os.environ if environ is None else environ
    value = environ.get("OPENAI_API_KEY", "").strip()
    if not value:
        return []
    return [CredentialSource(category="operator_env", name="OPENAI_API_KEY", value=value)]


def codex_auth_home(*, environ: Mapping[str, str]) -> Path:
    root = Path(environ.get("RAIL_HOME", "") or Path.home() / ".rail")
    return root / "codex"


def validate_codex_auth_material(auth_home: Path) -> list[Path]:
    if auth_home.exists():
        if auth_home.stat().st_mode & (stat.S_IWGRP | stat.S_IWOTH):
            raise ValueError("unsafe auth home permissions")
        material = list(auth_home.iterdir())
    else:
        material = []

    unknown = sorted(path.name for path in material if path.name not in _CODEX_AUTH_ALLOWLIST)
    if unknown:
        raise ValueError("unknown auth material")

    auth_file = auth_home / "auth.json"
    if not auth_file.is_file():
        raise ValueError("missing auth.json")
    if auth_file.stat().st_mode & (stat.S_IWGRP | stat.S_IWOTH):
        raise ValueError("unsafe auth material permissions")
    return [auth_file]


def _is_inside(path: Path, parent: Path) -> bool:
    try:
        path.resolve(strict=False).relative_to(parent.resolve(strict=False))
    except ValueError:
        return False
    return True
