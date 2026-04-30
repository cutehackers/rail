from __future__ import annotations

import shutil
from collections.abc import Mapping
from pathlib import Path

from pydantic import BaseModel, ConfigDict

from rail.auth.credentials import validate_codex_auth_material

_PROCESS_ENV_ALLOWLIST = {"PATH", "TMPDIR", "TMP", "TEMP"}
_CODEX_AUTH_COPY_ALLOWLIST = {"auth.json"}


class VaultEnvironment(BaseModel):
    model_config = ConfigDict(extra="forbid")

    codex_home: Path
    evidence_dir: Path
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

    accepted_auth_material = validate_codex_auth_material(auth_home)
    unexpected_material = sorted(path.name for path in accepted_auth_material if path.name not in _CODEX_AUTH_COPY_ALLOWLIST)
    if unexpected_material:
        raise ValueError("unknown auth material")

    codex_home.mkdir(parents=True, exist_ok=True)
    evidence_dir.mkdir(parents=True, exist_ok=True)

    copied_auth_material: list[str] = []
    for source in accepted_auth_material:
        destination = codex_home / source.name
        shutil.copy2(source, destination)
        copied_auth_material.append(source.name)

    environ = _scrub_vault_environment(base_environ, codex_home=codex_home)
    return VaultEnvironment(
        codex_home=codex_home,
        evidence_dir=evidence_dir,
        environ=environ,
        copied_auth_material=sorted(copied_auth_material),
    )


def _scrub_vault_environment(base_environ: Mapping[str, str], *, codex_home: Path) -> dict[str, str]:
    environ = {name: value for name, value in base_environ.items() if name in _PROCESS_ENV_ALLOWLIST and value}
    environ["HOME"] = str(codex_home)
    environ["CODEX_HOME"] = str(codex_home)
    return environ
