from __future__ import annotations

import hashlib
import os
import shlex
import subprocess
import time
from collections.abc import Mapping
from pathlib import Path
from typing import Literal

from pydantic import BaseModel, ConfigDict

from rail.workspace.isolation import tree_digest
from rail.workspace.validation import ValidationEvidence, record_validation_evidence

ValidationCommandSource = Literal["request", "policy", "actor"]
_VALIDATION_ENV_ALLOWLIST = {"PATH", "HOME", "TMPDIR", "TEMP", "TMP", "LANG", "LC_ALL", "PYTHONPATH", "VIRTUAL_ENV", "UV_CACHE_DIR"}


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
    return run_validation_commands(
        artifact_dir=artifact_dir,
        target_root=target_root,
        commands=[command],
        patch_digest=patch_digest,
        request_digest=request_digest,
        effective_policy_digest=effective_policy_digest,
        actor_invocation_digest=actor_invocation_digest,
    )


def run_validation_commands(
    *,
    artifact_dir: Path,
    target_root: Path,
    commands: list[ValidationCommand],
    patch_digest: str,
    request_digest: str,
    effective_policy_digest: str,
    actor_invocation_digest: str,
) -> ValidationEvidence:
    if not commands:
        raise ValueError("validation commands are not configured")
    for command in commands:
        _validate_command(command)

    before_digest = tree_digest(target_root)
    before_protected_digest = _protected_digest(target_root / ".harness")
    started = time.monotonic()
    stdout_parts: list[str] = []
    stderr_parts: list[str] = []
    exit_code = 0

    for command in commands:
        result = _run_one(command, target_root)
        stdout_parts.append(result.stdout)
        stderr_parts.append(result.stderr)
        exit_code = result.exit_code
        if exit_code != 0:
            break

    duration_ms = int((time.monotonic() - started) * 1000)
    after_digest = tree_digest(target_root)
    after_protected_digest = _protected_digest(target_root / ".harness")
    mutation_status: Literal["clean", "mutated"] = (
        "clean" if after_digest == before_digest and after_protected_digest == before_protected_digest else "mutated"
    )

    return record_validation_evidence(
        artifact_dir,
        command=" && ".join(shlex.join(command.argv) for command in commands),
        exit_code=exit_code,
        source=commands[0].source,
        patch_digest=patch_digest,
        tree_digest=after_digest,
        request_digest=request_digest,
        effective_policy_digest=effective_policy_digest,
        actor_invocation_digest=actor_invocation_digest,
        stdout="\n".join(part for part in stdout_parts if part),
        stderr="\n".join(part for part in stderr_parts if part),
        mutation_status=mutation_status,
        duration_ms=duration_ms,
        credential_mode="scrubbed",
        network_mode="inherited",
        sandbox_ref="target-root-subprocess",
    )


class _CommandResult(BaseModel):
    model_config = ConfigDict(extra="forbid")

    exit_code: int
    stdout: str
    stderr: str


def _validate_command(command: ValidationCommand) -> None:
    if command.source not in {"request", "policy"}:
        raise ValueError("validation command source must be request or policy")
    if not command.argv:
        raise ValueError("validation command argv must not be empty")


def _run_one(command: ValidationCommand, target_root: Path) -> _CommandResult:
    try:
        completed = subprocess.run(
            command.argv,
            cwd=target_root,
            timeout=command.timeout_seconds,
            capture_output=True,
            text=True,
            check=False,
            env=_validation_environment(os.environ),
        )
        return _CommandResult(exit_code=completed.returncode, stdout=completed.stdout, stderr=completed.stderr)
    except subprocess.TimeoutExpired as exc:
        stdout = _decode_timeout_output(exc.stdout)
        stderr = _decode_timeout_output(exc.stderr)
        stderr = f"{stderr}\nvalidation command timed out after {command.timeout_seconds}s".strip()
        return _CommandResult(exit_code=124, stdout=stdout, stderr=stderr)
    except OSError as exc:
        return _CommandResult(exit_code=127, stdout="", stderr=str(exc))


def _decode_timeout_output(output: str | bytes | None) -> str:
    if output is None:
        return ""
    if isinstance(output, bytes):
        return output.decode("utf-8", errors="replace")
    return output


def _validation_environment(environ: Mapping[str, str]) -> dict[str, str]:
    return {key: value for key, value in environ.items() if key in _VALIDATION_ENV_ALLOWLIST}


def _protected_digest(root: Path) -> str:
    digest = hashlib.sha256()
    if not root.exists():
        return "sha256:" + digest.hexdigest()
    for path in sorted(root.rglob("*")):
        if not path.is_file() or ".git" in path.parts:
            continue
        relative = path.relative_to(root).as_posix()
        digest.update(relative.encode("utf-8"))
        digest.update(path.read_bytes())
    return "sha256:" + digest.hexdigest()
