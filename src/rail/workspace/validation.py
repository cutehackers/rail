from __future__ import annotations

from pathlib import Path
from typing import Literal

import yaml
from pydantic import BaseModel, ConfigDict

from rail.auth.redaction import redact_secrets

ValidationSource = Literal["request", "policy", "actor"]


class ValidationEvidence(BaseModel):
    model_config = ConfigDict(extra="forbid")

    command: str
    exit_code: int
    status: Literal["pass", "fail"]
    source: ValidationSource
    stdout_ref: str
    stderr_ref: str
    patch_digest: str
    tree_digest: str
    request_digest: str | None = None
    effective_policy_digest: str | None = None
    actor_invocation_digest: str | None = None
    credential_mode: str
    network_mode: str
    sandbox_ref: str
    mutation_status: Literal["clean", "mutated"]
    duration_ms: int = 0
    ref: Path


def record_validation_evidence(
    artifact_dir: Path,
    *,
    command: str,
    exit_code: int,
    source: ValidationSource,
    patch_digest: str,
    tree_digest: str,
    request_digest: str | None = None,
    effective_policy_digest: str | None = None,
    actor_invocation_digest: str | None = None,
    stdout: str = "",
    stderr: str = "",
    mutation_status: Literal["clean", "mutated"] = "clean",
    duration_ms: int = 0,
    credential_mode: str = "minimum",
    network_mode: str = "disabled",
    sandbox_ref: str = "sandbox",
) -> ValidationEvidence:
    validation_dir = artifact_dir / "validation"
    validation_dir.mkdir(parents=True, exist_ok=True)
    stdout_ref = "validation/stdout.txt"
    stderr_ref = "validation/stderr.txt"
    evidence_ref = Path("validation/evidence.yaml")
    (artifact_dir / stdout_ref).write_text(redact_secrets(stdout), encoding="utf-8")
    (artifact_dir / stderr_ref).write_text(redact_secrets(stderr), encoding="utf-8")
    evidence = ValidationEvidence(
        command=command,
        exit_code=exit_code,
        status="pass" if exit_code == 0 else "fail",
        source=source,
        stdout_ref=stdout_ref,
        stderr_ref=stderr_ref,
        patch_digest=patch_digest,
        tree_digest=tree_digest,
        request_digest=request_digest,
        effective_policy_digest=effective_policy_digest,
        actor_invocation_digest=actor_invocation_digest,
        credential_mode=credential_mode,
        network_mode=network_mode,
        sandbox_ref=sandbox_ref,
        mutation_status=mutation_status,
        duration_ms=duration_ms,
        ref=evidence_ref,
    )
    (artifact_dir / evidence_ref).write_text(yaml.safe_dump(evidence.model_dump(mode="json"), sort_keys=True), encoding="utf-8")
    return evidence


def load_validation_evidence(artifact_dir: Path, ref: Path) -> ValidationEvidence:
    return ValidationEvidence.model_validate(yaml.safe_load((artifact_dir / ref).read_text(encoding="utf-8")))
