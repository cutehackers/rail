from __future__ import annotations

import shlex
import subprocess
import time
from pathlib import Path
from typing import Literal

from pydantic import BaseModel, ConfigDict

from rail.workspace.isolation import tree_digest
from rail.workspace.validation import ValidationEvidence, record_validation_evidence

ValidationCommandSource = Literal["request", "policy", "actor"]


class ValidationCommand(BaseModel):
    model_config = ConfigDict(extra="forbid")

    argv: list[str]
    source: ValidationCommandSource
    timeout_seconds: int = 30


def run_validation_command(
    *,
    artifact_dir: Path,
    target_root: Path,
    command: ValidationCommand,
    patch_digest: str,
    request_digest: str,
    effective_policy_digest: str,
    actor_invocation_digest: str,
) -> ValidationEvidence:
    if command.source not in {"request", "policy"}:
        raise ValueError("validation command source must be request or policy")
    if not command.argv:
        raise ValueError("validation command argv must not be empty")

    before_digest = tree_digest(target_root)
    started = time.monotonic()
    try:
        completed = subprocess.run(
            command.argv,
            cwd=target_root,
            timeout=command.timeout_seconds,
            capture_output=True,
            text=True,
            check=False,
        )
        exit_code = completed.returncode
        stdout = completed.stdout
        stderr = completed.stderr
    except subprocess.TimeoutExpired as exc:
        exit_code = 124
        stdout = _decode_timeout_output(exc.stdout)
        stderr = _decode_timeout_output(exc.stderr)
        stderr = f"{stderr}\nvalidation command timed out after {command.timeout_seconds}s".strip()

    duration_ms = int((time.monotonic() - started) * 1000)
    after_digest = tree_digest(target_root)
    mutation_status: Literal["clean", "mutated"] = "clean" if after_digest == before_digest else "mutated"

    return record_validation_evidence(
        artifact_dir,
        command=shlex.join(command.argv),
        exit_code=exit_code,
        source=command.source,
        patch_digest=patch_digest,
        tree_digest=after_digest,
        request_digest=request_digest,
        effective_policy_digest=effective_policy_digest,
        actor_invocation_digest=actor_invocation_digest,
        stdout=stdout,
        stderr=stderr,
        mutation_status=mutation_status,
        duration_ms=duration_ms,
    )


def _decode_timeout_output(output: str | bytes | None) -> str:
    if output is None:
        return ""
    if isinstance(output, bytes):
        return output.decode("utf-8", errors="replace")
    return output
