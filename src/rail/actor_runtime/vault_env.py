from __future__ import annotations

import shutil
import stat
from collections.abc import Mapping
from pathlib import Path

from pydantic import BaseModel, ConfigDict

from rail.auth.credentials import validate_codex_auth_material

_PROCESS_ENV_ALLOWLIST = {"PATH"}
_CODEX_AUTH_COPY_ALLOWLIST = {"auth.json"}


class VaultEnvironment(BaseModel):
    model_config = ConfigDict(extra="forbid")

    codex_home: Path
    evidence_dir: Path
    temp_dir: Path
    environ: dict[str, str]
    copied_auth_material: list[str]


def materialize_vault_environment(
    *,
    artifact_dir: Path,
    auth_home: Path,
    base_environ: Mapping[str, str],
) -> VaultEnvironment:
    actor_runtime_dir = artifact_dir / "actor_runtime"
    codex_home = actor_runtime_dir / "codex_home"
    evidence_dir = actor_runtime_dir / "evidence"
    temp_dir = actor_runtime_dir / "tmp"

    accepted_auth_material = validate_codex_auth_material(auth_home)
    unexpected_material = sorted(path.name for path in accepted_auth_material if path.name not in _CODEX_AUTH_COPY_ALLOWLIST)
    if unexpected_material:
        raise ValueError("unknown auth material")

    _prepare_actor_runtime_dir(actor_runtime_dir)
    _prepare_empty_directory(codex_home, mode=0o700)
    _prepare_empty_directory(evidence_dir, mode=0o700)
    _prepare_empty_directory(temp_dir, mode=0o700)

    copied_auth_material: list[str] = []
    for source in accepted_auth_material:
        destination = codex_home / source.name
        if destination.exists() or destination.is_symlink():
            raise ValueError("unsafe vault material")
        shutil.copyfile(source, destination)
        destination.chmod(0o600)
        copied_auth_material.append(source.name)

    environ = _scrub_vault_environment(base_environ, codex_home=codex_home, temp_dir=temp_dir)
    return VaultEnvironment(
        codex_home=codex_home,
        evidence_dir=evidence_dir,
        temp_dir=temp_dir,
        environ=environ,
        copied_auth_material=sorted(copied_auth_material),
    )


def _prepare_empty_directory(path: Path, *, mode: int) -> None:
    if path.is_symlink():
        raise ValueError("unsafe vault material")
    if path.exists():
        if not path.is_dir():
            raise ValueError("unsafe vault material")
        children = list(path.iterdir())
        if children:
            if any(child.is_symlink() for child in children):
                raise ValueError("unsafe vault material")
            raise ValueError("unexpected vault material")
    else:
        path.mkdir(mode=mode, parents=True)
    path.chmod(mode)
    if path.stat().st_mode & (stat.S_IWGRP | stat.S_IWOTH):
        raise ValueError("unsafe vault material permissions")


def _prepare_actor_runtime_dir(path: Path) -> None:
    if path.is_symlink():
        raise ValueError("unsafe vault material")
    if path.exists() and not path.is_dir():
        raise ValueError("unsafe vault material")
    path.mkdir(parents=True, exist_ok=True)
    if path.is_symlink():
        raise ValueError("unsafe vault material")


def _scrub_vault_environment(base_environ: Mapping[str, str], *, codex_home: Path, temp_dir: Path) -> dict[str, str]:
    environ = {name: value for name, value in base_environ.items() if name in _PROCESS_ENV_ALLOWLIST and value}
    environ["HOME"] = str(codex_home)
    environ["CODEX_HOME"] = str(codex_home)
    environ["TMPDIR"] = str(temp_dir)
    environ["TMP"] = str(temp_dir)
    environ["TEMP"] = str(temp_dir)
    return environ
