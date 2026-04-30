from __future__ import annotations

import re
import shutil
import stat
import subprocess
from collections.abc import Callable
from pathlib import Path
from typing import Literal

from pydantic import BaseModel, ConfigDict

from rail.actor_runtime.evidence import write_runtime_evidence
from rail.actor_runtime.events import normalize_sdk_event
from rail.actor_runtime.runtime import ActorInvocation, ActorResult
from rail.policy.schema import ActorRuntimePolicyV2

CODEX_COMMAND_NAME = "codex"
CODEX_MINIMUM_SUPPORTED_VERSION = (0, 124, 0)
CODEX_VERSION_PATTERN = re.compile(r"^codex-cli (?P<major>0|[1-9]\d*)\.(?P<minor>0|[1-9]\d*)\.(?P<patch>0|[1-9]\d*)$")
CODEX_EXEC_REQUIRED_HELP_FLAGS = (
    "--json",
    "--output-schema",
    "--ignore-user-config",
    "--ignore-rules",
    "--ephemeral",
    "--sandbox",
    "--cd",
)
_FORBIDDEN_CODEX_EXEC_FLAG = "--dangerously-bypass-approvals-and-sandbox"
_TRUSTED_UNRESOLVED_COMMAND_ROOTS = (Path("/opt/homebrew/bin"), Path("/usr/local/bin"), Path("/usr/bin"))
_TRUSTED_RESOLVED_COMMAND_ROOTS = (
    Path("/opt/homebrew/Caskroom/codex"),
    Path("/opt/homebrew/bin"),
    Path("/usr/local/bin"),
    Path("/usr/bin"),
)
_UNTRUSTED_TEMP_ROOTS = (Path("/tmp"), Path("/var/tmp"))

CodexCommandResolver = Callable[[], Path | None]
CodexCommandTrustChecker = Callable[[Path, Path, Path | None], str | None]
CodexCommandRunner = Callable[[list[str]], "CodexCommandRunResult"]


class CodexCommandReadiness(BaseModel):
    model_config = ConfigDict(extra="forbid")

    ready: bool
    reason: str
    command_path: Path | None = None
    codex_version: str | None = None
    blocked_category: Literal["environment"] | None = None


class CodexCommandRunResult(BaseModel):
    model_config = ConfigDict(extra="forbid")

    stdout: str
    stderr: str
    returncode: int


class CodexVaultActorRuntime:
    def __init__(
        self,
        *,
        project_root: Path,
        policy: ActorRuntimePolicyV2,
        command_resolver: CodexCommandResolver | None = None,
        command_trust_checker: CodexCommandTrustChecker | None = None,
        runner: CodexCommandRunner | None = None,
    ) -> None:
        self.project_root = project_root
        self.policy = policy
        self.command_resolver = command_resolver or resolve_codex_command
        self.command_trust_checker = command_trust_checker or check_trusted_codex_command
        self.runner = runner or run_codex_command
        self._readiness_cache: CodexCommandReadiness | None = None

    def readiness(self) -> CodexCommandReadiness:
        if self._readiness_cache is not None:
            return self._readiness_cache

        command_path = self.command_resolver()
        if command_path is None:
            return self._cache_readiness(
                CodexCommandReadiness(
                    ready=False,
                    reason="Codex command was not found on PATH",
                    blocked_category="environment",
                )
            )

        trust_failure = self.command_trust_checker(command_path, self.project_root, None)
        if trust_failure is not None:
            return self._cache_readiness(
                CodexCommandReadiness(
                    ready=False,
                    reason=trust_failure,
                    blocked_category="environment",
                )
            )

        version_result = self._run_readiness_command([command_path.as_posix(), "--version"])
        if version_result.returncode < 0:
            return self._cache_readiness(
                CodexCommandReadiness(
                    ready=False,
                    reason="Codex command version check failed",
                    command_path=command_path,
                    blocked_category="environment",
                )
            )
        version_stdout = version_result.stdout.strip()
        if version_result.returncode != 0:
            return self._cache_readiness(
                CodexCommandReadiness(
                    ready=False,
                    reason="Codex command version check failed",
                    command_path=command_path,
                    codex_version=version_stdout or None,
                    blocked_category="environment",
                )
            )
        version = _parse_codex_version(version_stdout)
        if version is None:
            return self._cache_readiness(
                CodexCommandReadiness(
                    ready=False,
                    reason='Codex command version must match "codex-cli MAJOR.MINOR.PATCH"',
                    command_path=command_path,
                    codex_version=version_stdout or None,
                    blocked_category="environment",
                )
            )
        if version < CODEX_MINIMUM_SUPPORTED_VERSION:
            return self._cache_readiness(
                CodexCommandReadiness(
                    ready=False,
                    reason="Codex command is below the minimum supported version 0.124.0",
                    command_path=command_path,
                    codex_version=version_stdout,
                    blocked_category="environment",
                )
            )

        help_result = self._run_readiness_command([command_path.as_posix(), "exec", "--help"])
        if help_result.returncode < 0:
            return self._cache_readiness(
                CodexCommandReadiness(
                    ready=False,
                    reason="Codex command exec help check failed",
                    command_path=command_path,
                    codex_version=version_stdout,
                    blocked_category="environment",
                )
            )
        help_output = f"{help_result.stdout}\n{help_result.stderr}"
        if help_result.returncode != 0:
            return self._cache_readiness(
                CodexCommandReadiness(
                    ready=False,
                    reason="Codex command exec help check failed",
                    command_path=command_path,
                    codex_version=version_stdout,
                    blocked_category="environment",
                )
            )
        missing_flags = [flag for flag in CODEX_EXEC_REQUIRED_HELP_FLAGS if flag not in help_output]
        if missing_flags:
            return self._cache_readiness(
                CodexCommandReadiness(
                    ready=False,
                    reason=f"Codex command exec help is missing required flags: {', '.join(missing_flags)}",
                    command_path=command_path,
                    codex_version=version_stdout,
                    blocked_category="environment",
                )
            )

        return self._cache_readiness(
            CodexCommandReadiness(
                ready=True,
                reason="Codex command is ready",
                command_path=command_path,
                codex_version=version_stdout,
            )
        )

    def _cache_readiness(self, readiness: CodexCommandReadiness) -> CodexCommandReadiness:
        self._readiness_cache = readiness
        return readiness

    def _run_readiness_command(self, command: list[str]) -> CodexCommandRunResult:
        try:
            return self.runner(command)
        except (OSError, subprocess.SubprocessError) as exc:
            return CodexCommandRunResult(stdout="", stderr=type(exc).__name__, returncode=-1)

    def run(self, invocation: ActorInvocation) -> ActorResult:
        readiness = self.readiness()
        if not readiness.ready:
            command_path_status = _command_path_status(readiness)
            events_ref, evidence_ref = write_runtime_evidence(
                invocation.artifact_dir,
                invocation.actor,
                normalize_sdk_event(
                    {
                        "status": "interrupted",
                        "actor": invocation.actor,
                        "error": readiness.reason,
                        "blocked_category": readiness.blocked_category,
                        "runtime_provider": self.policy.runtime.provider,
                        "runtime_project_root": self.project_root.as_posix(),
                        "target_root": invocation.target_root.as_posix(),
                        "command_path_status": command_path_status,
                        "codex_version": readiness.codex_version,
                    }
                ),
            )
            return ActorResult(
                status="interrupted",
                structured_output={"error": readiness.reason},
                events_ref=events_ref,
                runtime_evidence_ref=evidence_ref,
                blocked_category=readiness.blocked_category,
            )
        reason = "Codex Vault Actor Runtime environment readiness is not implemented; credential and command checks are pending"
        events_ref, evidence_ref = write_runtime_evidence(
            invocation.artifact_dir,
            invocation.actor,
            normalize_sdk_event(
                {
                    "status": "interrupted",
                    "actor": invocation.actor,
                    "error": reason,
                    "blocked_category": "environment",
                    "runtime_provider": self.policy.runtime.provider,
                    "runtime_project_root": self.project_root.as_posix(),
                    "target_root": invocation.target_root.as_posix(),
                }
            ),
        )
        return ActorResult(
            status="interrupted",
            structured_output={"error": reason},
            events_ref=events_ref,
            runtime_evidence_ref=evidence_ref,
            blocked_category="environment",
        )


def resolve_codex_command() -> Path | None:
    command = shutil.which(CODEX_COMMAND_NAME)
    if command is None:
        return None
    return Path(command)


def run_codex_command(command: list[str]) -> CodexCommandRunResult:
    completed = subprocess.run(command, capture_output=True, text=True, timeout=15, check=False)
    return CodexCommandRunResult(
        stdout=completed.stdout,
        stderr=completed.stderr,
        returncode=completed.returncode,
    )


def build_required_codex_exec_args(*, output_schema: Path, sandbox: Path) -> list[str]:
    args = [
        "exec",
        "--json",
        "--output-schema",
        output_schema.as_posix(),
        "--ignore-user-config",
        "--ignore-rules",
        "--ephemeral",
        "--sandbox",
        "read-only",
        "--cd",
        sandbox.as_posix(),
        "-c",
        "shell_environment_policy.inherit=none",
    ]
    if _FORBIDDEN_CODEX_EXEC_FLAG in args:
        raise ValueError("forbidden Codex execution flag configured")
    return args


def check_trusted_codex_command(command_path: Path, _target_root: Path, _artifact_dir: Path | None) -> str | None:
    if not command_path.is_absolute():
        return "Codex command path must be absolute"
    if not _path_is_under_any(command_path, _TRUSTED_UNRESOLVED_COMMAND_ROOTS):
        return "Codex command path must be under a trusted install directory"
    if _path_is_under_any(command_path, _UNTRUSTED_TEMP_ROOTS):
        return "Codex command path must not live under a temporary directory"
    if not command_path.exists():
        return "Codex command path does not exist"

    resolved_path = command_path.resolve()
    if not resolved_path.exists():
        return "Codex command resolved path does not exist"
    if not _path_is_under_any(resolved_path, _TRUSTED_RESOLVED_COMMAND_ROOTS):
        return "Codex command resolved path must be under a trusted install directory"
    if _path_is_under_any(resolved_path, _UNTRUSTED_TEMP_ROOTS):
        return "Codex command resolved path must not live under a temporary directory"
    return _check_codex_command_permissions(command_path, resolved_path)


def _check_codex_command_permissions(command_path: Path, resolved_path: Path) -> str | None:
    for label, path in (("Codex command path", command_path), ("Codex command resolved path", resolved_path)):
        mode = path.stat().st_mode
        if mode & (stat.S_IWGRP | stat.S_IWOTH):
            return f"{label} must not be group-writable or world-writable"
    return None


def _parse_codex_version(value: str) -> tuple[int, int, int] | None:
    match = CODEX_VERSION_PATTERN.fullmatch(value)
    if match is None:
        return None
    return (int(match["major"]), int(match["minor"]), int(match["patch"]))


def _path_is_under_any(path: Path, roots: tuple[Path, ...]) -> bool:
    return any(path == root or path.is_relative_to(root) for root in roots)


def _command_path_status(readiness: CodexCommandReadiness) -> str:
    if readiness.command_path is not None:
        return "trusted"
    if "not found" in readiness.reason:
        return "missing"
    return "untrusted"
