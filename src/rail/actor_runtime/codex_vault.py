from __future__ import annotations

import json
import os
import re
import shlex
import shutil
import stat
import subprocess
from collections.abc import Callable
from pathlib import Path
from typing import Any, Literal

from pydantic import BaseModel, ConfigDict
from pydantic import ValidationError

from rail.artifacts.digests import digest_file
from rail.actor_runtime.evidence import write_runtime_evidence
from rail.actor_runtime.events import normalize_sdk_event
from rail.actor_runtime.output_schema import compile_codex_output_schema
from rail.actor_runtime.prompts import load_actor_catalog
from rail.actor_runtime.runtime import ActorInvocation, ActorResult
from rail.actor_runtime.vault_audit import audit_codex_event_capabilities, audit_vault_materialization
from rail.actor_runtime.vault_env import VaultEnvironment, materialize_vault_environment
from rail.auth.credentials import codex_auth_home
from rail.auth.redaction import redact_secrets
from rail.policy.schema import ActorRuntimePolicyV2
from rail.workspace.isolation import target_mutation_digest, tree_digest
from rail.workspace.sandbox import create_sandbox

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
    "--model",
    "--skip-git-repo-check",
)
_FORBIDDEN_CODEX_EXEC_FLAG = "--dangerously-bypass-approvals-and-sandbox"
_TRUSTED_UNRESOLVED_COMMAND_ROOTS = (Path("/opt/homebrew/bin"), Path("/usr/local/bin"), Path("/usr/bin"))
_TRUSTED_RESOLVED_COMMAND_ROOTS = (
    Path("/opt/homebrew/Caskroom/codex"),
    Path("/opt/homebrew/bin"),
    Path("/usr/local/bin"),
    Path("/usr/bin"),
)
_TRUSTED_SYSTEM_BINARY_ROOTS = (Path("/bin"), Path("/usr/bin"))
_UNTRUSTED_TEMP_ROOTS = (Path("/tmp"), Path("/var/tmp"))
_READ_ONLY_SHELL_EXECUTABLES = {"pwd", "ls", "find", "rg", "sed", "cat", "wc", "head", "tail", "stat", "test"}
_TRUSTED_SHELL_WRAPPER_PATHS = {Path("/bin/zsh"), Path("/bin/sh"), Path("/usr/bin/zsh"), Path("/usr/bin/sh")}
_ACTOR_MESSAGE_ITEM_TYPES = {"agent_message", "assistant_message"}
_SHELL_OPERATOR_PATTERN = re.compile(r"(\|\||&&|[|<>;&`{}\n\r])|\$\(")
_SHELL_VARIABLE_PATTERN = re.compile(r"\$(?:[A-Za-z_][A-Za-z0-9_]*|\{[^}]+\})")

CodexCommandResolver = Callable[[], Path | None]
CodexCommandTrustChecker = Callable[[Path, Path, Path | None], str | None]
CodexCommandRunner = Callable[..., "CodexCommandRunResult"]
VaultEnvironmentMaterializer = Callable[..., VaultEnvironment]


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


class ParsedCodexEvents(BaseModel):
    model_config = ConfigDict(extra="forbid")

    raw_events: list[dict[str, object]]
    normalized_events: list[dict[str, object]]
    final_output: dict[str, object] | None = None


class CodexEventParseError(ValueError):
    def __init__(self, message: str, parsed: ParsedCodexEvents) -> None:
        super().__init__(message)
        self.parsed = parsed


class CodexVaultActorRuntime:
    def __init__(
        self,
        *,
        project_root: Path,
        policy: ActorRuntimePolicyV2,
        command_resolver: CodexCommandResolver | None = None,
        command_trust_checker: CodexCommandTrustChecker | None = None,
        runner: CodexCommandRunner | None = None,
        environment_materializer: VaultEnvironmentMaterializer | None = None,
    ) -> None:
        self.project_root = project_root
        self.policy = policy
        self.command_resolver = command_resolver or resolve_codex_command
        self.command_trust_checker = command_trust_checker or check_trusted_codex_command
        self.runner = runner or run_codex_command
        self.environment_materializer = environment_materializer or materialize_vault_environment
        self.catalog = load_actor_catalog(project_root)
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
                invocation.attempt_ref,
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
        try:
            vault_environment = self.environment_materializer(
                artifact_dir=invocation.artifact_dir,
                auth_home=codex_auth_home(environ=os.environ),
                base_environ=os.environ,
                actor=invocation.actor,
            )
        except (OSError, ValueError) as exc:
            reason = "Codex Vault Actor Runtime environment materialization failed"
            events_ref, evidence_ref = write_runtime_evidence(
                invocation.artifact_dir,
                invocation.attempt_ref,
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
                        "error_type": type(exc).__name__,
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

        materialization_violation = audit_vault_materialization(vault_environment, artifact_dir=invocation.artifact_dir)
        if materialization_violation is not None:
            reason = materialization_violation.reason
            events_ref, evidence_ref = write_runtime_evidence(
                invocation.artifact_dir,
                invocation.attempt_ref,
                invocation.actor,
                self._evidence_payload(
                    invocation,
                    readiness=readiness,
                    vault_environment=vault_environment,
                    status="interrupted",
                    blocked_category="policy",
                    error=reason,
                    extra={"policy_violation": materialization_violation.model_dump()},
                ),
            )
            return ActorResult(
                status="interrupted",
                structured_output={"error": reason},
                events_ref=events_ref,
                runtime_evidence_ref=evidence_ref,
                blocked_category="policy",
            )

        target_pre_run_tree_digest = target_mutation_digest(invocation.target_root)
        try:
            sandbox = create_sandbox(invocation.target_root)
        except (OSError, ValueError) as exc:
            reason = "Codex Vault Actor Runtime sandbox creation failed"
            events_ref, evidence_ref = write_runtime_evidence(
                invocation.artifact_dir,
                invocation.attempt_ref,
                invocation.actor,
                self._evidence_payload(
                    invocation,
                    readiness=readiness,
                    vault_environment=vault_environment,
                    status="interrupted",
                    blocked_category="environment",
                    error=reason,
                    extra={"error_type": type(exc).__name__},
                ),
            )
            return ActorResult(
                status="interrupted",
                structured_output={"error": reason},
                events_ref=events_ref,
                runtime_evidence_ref=evidence_ref,
                blocked_category="environment",
            )

        command_path = readiness.command_path
        if command_path is None:
            reason = "Codex command was not available after readiness"
            events_ref, evidence_ref = write_runtime_evidence(
                invocation.artifact_dir,
                invocation.attempt_ref,
                invocation.actor,
                self._evidence_payload(
                    invocation,
                    readiness=readiness,
                    vault_environment=vault_environment,
                    status="interrupted",
                    blocked_category="environment",
                    error=reason,
                    extra={"sandbox_root_ref": sandbox.sandbox_root.as_posix()},
                ),
            )
            return ActorResult(
                status="interrupted",
                structured_output={"error": reason},
                events_ref=events_ref,
                runtime_evidence_ref=evidence_ref,
                blocked_category="environment",
            )

        invocation_path_failure = _check_invocation_command_path(
            command_path,
            artifact_dir=invocation.artifact_dir,
            target_root=invocation.target_root,
            sandbox_root=sandbox.sandbox_root,
        )
        if invocation_path_failure is not None:
            events_ref, evidence_ref = write_runtime_evidence(
                invocation.artifact_dir,
                invocation.attempt_ref,
                invocation.actor,
                self._evidence_payload(
                    invocation,
                    readiness=readiness,
                    vault_environment=vault_environment,
                    status="interrupted",
                    blocked_category="policy",
                    error=invocation_path_failure,
                    extra={
                        "sandbox_root_ref": sandbox.sandbox_root.as_posix(),
                        "target_pre_run_tree_digest": target_pre_run_tree_digest,
                        "sandbox_base_tree_digest": tree_digest(sandbox.sandbox_root),
                        "policy_violation": {"reason": invocation_path_failure},
                    },
                ),
            )
            return ActorResult(
                status="interrupted",
                structured_output={"error": invocation_path_failure},
                events_ref=events_ref,
                runtime_evidence_ref=evidence_ref,
                blocked_category="policy",
            )

        entry = self.catalog[invocation.actor]
        output_schema_ref, output_schema_path, output_schema_digest = _materialize_output_schema(invocation, entry.schema_source)
        prompt = (
            f"{entry.prompt}\n\n"
            f"Invocation: {invocation.prompt}\n\n"
            f"Policy digest: {invocation.policy_digest}\n\n"
            f"Actor input JSON:\n{json.dumps(invocation.input, sort_keys=True, ensure_ascii=False)}"
        )
        command = [
            command_path.as_posix(),
            *build_required_codex_exec_args(
                output_schema=output_schema_path,
                sandbox=sandbox.sandbox_root,
                model=self.policy.runtime.model,
            ),
        ]
        raw_events: list[dict[str, object]] = []
        normalized_events: list[dict[str, object]] = []
        final_output: dict[str, object] | None = None
        command_result: CodexCommandRunResult | None = None
        try:
            command_result = self.runner(
                command,
                stdin=prompt,
                environ=vault_environment.environ,
                timeout=self.policy.runtime.timeout_seconds,
            )
            parsed = _parse_codex_json_events(command_result.stdout)
            raw_events = parsed.raw_events
            normalized_events = parsed.normalized_events
            final_output = parsed.final_output
        except Exception as exc:
            if isinstance(exc, CodexEventParseError):
                raw_events = exc.parsed.raw_events
                normalized_events = exc.parsed.normalized_events
            message = str(redact_secrets(str(exc)))
            reason = f"Codex Vault Actor Runtime execution failed: {message}"
            post_run_target_tree_digest = target_mutation_digest(invocation.target_root)
            extra: dict[str, object] = {
                "sandbox_root_ref": sandbox.sandbox_root.as_posix(),
                "target_pre_run_tree_digest": target_pre_run_tree_digest,
                "sandbox_base_tree_digest": tree_digest(sandbox.sandbox_root),
                "post_run_target_tree_digest": post_run_target_tree_digest,
                "output_schema_ref": output_schema_ref.as_posix(),
                "output_schema_digest": output_schema_digest,
                "error_type": type(exc).__name__,
            }
            if post_run_target_tree_digest != target_pre_run_tree_digest:
                reason = "target tree changed outside Rail patch apply"
                events_ref, evidence_ref = write_runtime_evidence(
                    invocation.artifact_dir,
                    invocation.attempt_ref,
                    invocation.actor,
                    self._evidence_payload(
                        invocation,
                        readiness=readiness,
                        vault_environment=vault_environment,
                        status="interrupted",
                        blocked_category="policy",
                        error=reason,
                        raw_events=raw_events,
                        normalized_events=normalized_events,
                        extra=extra | {"policy_violation": {"reason": reason}},
                    ),
                    events=normalized_events or None,
                )
                return ActorResult(
                    status="interrupted",
                    structured_output={"error": reason},
                    events_ref=events_ref,
                    runtime_evidence_ref=evidence_ref,
                    blocked_category="policy",
                )
            materialization_violation = audit_vault_materialization(vault_environment, artifact_dir=invocation.artifact_dir)
            if materialization_violation is not None:
                reason = materialization_violation.reason
                events_ref, evidence_ref = write_runtime_evidence(
                    invocation.artifact_dir,
                    invocation.attempt_ref,
                    invocation.actor,
                    self._evidence_payload(
                        invocation,
                        readiness=readiness,
                        vault_environment=vault_environment,
                        status="interrupted",
                        blocked_category="policy",
                        error=reason,
                        raw_events=raw_events,
                        normalized_events=normalized_events,
                        extra=extra | {"policy_violation": materialization_violation.model_dump()},
                    ),
                    events=normalized_events or None,
                )
                return ActorResult(
                    status="interrupted",
                    structured_output={"error": reason},
                    events_ref=events_ref,
                    runtime_evidence_ref=evidence_ref,
                    blocked_category="policy",
                )
            policy_violation = _codex_event_policy_violation(
                raw_events,
                sandbox_root=sandbox.sandbox_root,
                invocation=invocation,
                runtime_project_root=self.project_root,
                user_codex_home=os.environ.get("CODEX_HOME"),
                rail_auth_home=codex_auth_home(environ=os.environ),
                shell_allowlist=set(self.policy.tools.shell.allowlist) & _READ_ONLY_SHELL_EXECUTABLES,
                shell_enabled=self.policy.tools.shell.enabled,
            )
            if policy_violation is not None:
                events_ref, evidence_ref = write_runtime_evidence(
                    invocation.artifact_dir,
                    invocation.attempt_ref,
                    invocation.actor,
                    self._evidence_payload(
                        invocation,
                        readiness=readiness,
                        vault_environment=vault_environment,
                        status="interrupted",
                        blocked_category="policy",
                        error=policy_violation,
                        raw_events=raw_events,
                        normalized_events=normalized_events,
                        extra=extra | {"policy_violation": {"reason": policy_violation}},
                    ),
                    events=normalized_events or None,
                )
                return ActorResult(
                    status="interrupted",
                    structured_output={"error": policy_violation},
                    events_ref=events_ref,
                    runtime_evidence_ref=evidence_ref,
                    blocked_category="policy",
                )
            events_ref, evidence_ref = write_runtime_evidence(
                invocation.artifact_dir,
                invocation.attempt_ref,
                invocation.actor,
                self._evidence_payload(
                    invocation,
                    readiness=readiness,
                    vault_environment=vault_environment,
                    status="interrupted",
                    blocked_category="runtime",
                    error=reason,
                    raw_events=raw_events,
                    normalized_events=normalized_events,
                    extra=extra,
                ),
                events=normalized_events or None,
            )
            return ActorResult(
                status="interrupted",
                structured_output={"error": reason},
                events_ref=events_ref,
                runtime_evidence_ref=evidence_ref,
                blocked_category="runtime",
            )

        post_run_target_tree_digest = target_mutation_digest(invocation.target_root)
        base_extra: dict[str, object] = {
            "sandbox_root_ref": sandbox.sandbox_root.as_posix(),
            "target_pre_run_tree_digest": target_pre_run_tree_digest,
            "sandbox_base_tree_digest": tree_digest(sandbox.sandbox_root),
            "post_run_target_tree_digest": post_run_target_tree_digest,
            "output_schema_ref": output_schema_ref.as_posix(),
            "output_schema_digest": output_schema_digest,
            "codex_returncode": command_result.returncode,
        }

        materialization_violation = audit_vault_materialization(vault_environment, artifact_dir=invocation.artifact_dir)
        if materialization_violation is not None:
            reason = materialization_violation.reason
            events_ref, evidence_ref = write_runtime_evidence(
                invocation.artifact_dir,
                invocation.attempt_ref,
                invocation.actor,
                self._evidence_payload(
                    invocation,
                    readiness=readiness,
                    vault_environment=vault_environment,
                    status="interrupted",
                    blocked_category="policy",
                    error=reason,
                    raw_events=raw_events,
                    normalized_events=normalized_events,
                    extra=base_extra | {"policy_violation": materialization_violation.model_dump()},
                ),
                events=normalized_events or None,
            )
            return ActorResult(
                status="interrupted",
                structured_output={"error": reason},
                events_ref=events_ref,
                runtime_evidence_ref=evidence_ref,
                blocked_category="policy",
            )

        policy_violation = _codex_event_policy_violation(
            raw_events,
            sandbox_root=sandbox.sandbox_root,
            invocation=invocation,
            runtime_project_root=self.project_root,
            user_codex_home=os.environ.get("CODEX_HOME"),
            rail_auth_home=codex_auth_home(environ=os.environ),
            shell_allowlist=set(self.policy.tools.shell.allowlist) & _READ_ONLY_SHELL_EXECUTABLES,
            shell_enabled=self.policy.tools.shell.enabled,
        )
        if policy_violation is not None:
            events_ref, evidence_ref = write_runtime_evidence(
                invocation.artifact_dir,
                invocation.attempt_ref,
                invocation.actor,
                self._evidence_payload(
                    invocation,
                    readiness=readiness,
                    vault_environment=vault_environment,
                    status="interrupted",
                    blocked_category="policy",
                    error=policy_violation,
                    raw_events=raw_events,
                    normalized_events=normalized_events,
                    extra=base_extra | {"policy_violation": {"reason": policy_violation}},
                ),
                events=normalized_events or None,
            )
            return ActorResult(
                status="interrupted",
                structured_output={"error": policy_violation},
                events_ref=events_ref,
                runtime_evidence_ref=evidence_ref,
                blocked_category="policy",
            )

        if post_run_target_tree_digest != target_pre_run_tree_digest:
            reason = "target tree changed outside Rail patch apply"
            events_ref, evidence_ref = write_runtime_evidence(
                invocation.artifact_dir,
                invocation.attempt_ref,
                invocation.actor,
                self._evidence_payload(
                    invocation,
                    readiness=readiness,
                    vault_environment=vault_environment,
                    status="interrupted",
                    blocked_category="policy",
                    error=reason,
                    raw_events=raw_events,
                    normalized_events=normalized_events,
                    extra=base_extra | {"policy_violation": {"reason": reason}},
                ),
                events=normalized_events or None,
            )
            return ActorResult(
                status="interrupted",
                structured_output={"error": reason},
                events_ref=events_ref,
                runtime_evidence_ref=evidence_ref,
                blocked_category="policy",
            )

        if command_result.returncode != 0:
            reason = "Codex command execution failed"
            events_ref, evidence_ref = write_runtime_evidence(
                invocation.artifact_dir,
                invocation.attempt_ref,
                invocation.actor,
                self._evidence_payload(
                    invocation,
                    readiness=readiness,
                    vault_environment=vault_environment,
                    status="interrupted",
                    blocked_category="runtime",
                    error=reason,
                    raw_events=raw_events,
                    normalized_events=normalized_events,
                    extra=base_extra | {"stderr": command_result.stderr},
                ),
                events=normalized_events or None,
            )
            return ActorResult(
                status="interrupted",
                structured_output={"error": reason},
                events_ref=events_ref,
                runtime_evidence_ref=evidence_ref,
                blocked_category="runtime",
            )

        if final_output is None:
            reason = "Codex command did not produce structured output"
            events_ref, evidence_ref = write_runtime_evidence(
                invocation.artifact_dir,
                invocation.attempt_ref,
                invocation.actor,
                self._evidence_payload(
                    invocation,
                    readiness=readiness,
                    vault_environment=vault_environment,
                    status="interrupted",
                    blocked_category="runtime",
                    error=reason,
                    raw_events=raw_events,
                    normalized_events=normalized_events,
                    extra=base_extra,
                ),
                events=normalized_events or None,
            )
            return ActorResult(
                status="interrupted",
                structured_output={"error": reason},
                events_ref=events_ref,
                runtime_evidence_ref=evidence_ref,
                blocked_category="runtime",
            )

        try:
            structured = entry.validate_output(final_output).model_dump(mode="json")
        except ValidationError as exc:
            reason = f"validation failed: {redact_secrets(str(exc))}"
            events_ref, evidence_ref = write_runtime_evidence(
                invocation.artifact_dir,
                invocation.attempt_ref,
                invocation.actor,
                self._evidence_payload(
                    invocation,
                    readiness=readiness,
                    vault_environment=vault_environment,
                    status="interrupted",
                    blocked_category="runtime",
                    error=reason,
                    raw_events=raw_events,
                    normalized_events=normalized_events,
                    extra=base_extra,
                ),
                events=normalized_events or None,
            )
            return ActorResult(
                status="interrupted",
                structured_output={"error": reason},
                events_ref=events_ref,
                runtime_evidence_ref=evidence_ref,
                blocked_category="runtime",
            )

        events_ref, evidence_ref = write_runtime_evidence(
            invocation.artifact_dir,
            invocation.attempt_ref,
            invocation.actor,
            self._evidence_payload(
                invocation,
                readiness=readiness,
                vault_environment=vault_environment,
                status="succeeded",
                raw_events=raw_events,
                normalized_events=normalized_events,
                structured_output=structured,
                extra=base_extra,
            ),
            events=normalized_events or None,
        )
        return ActorResult(
            status="succeeded",
            structured_output=structured,
            events_ref=events_ref,
            runtime_evidence_ref=evidence_ref,
            patch_bundle_ref=Path(structured["patch_bundle_ref"]) if structured.get("patch_bundle_ref") else None,
        )

    def _evidence_payload(
        self,
        invocation: ActorInvocation,
        *,
        readiness: CodexCommandReadiness,
        vault_environment: VaultEnvironment,
        status: str,
        blocked_category: str | None = None,
        error: str | None = None,
        raw_events: list[dict[str, object]] | None = None,
        normalized_events: list[dict[str, object]] | None = None,
        structured_output: dict[str, object] | None = None,
        extra: dict[str, object] | None = None,
    ) -> dict[str, object]:
        payload: dict[str, object] = {
            "status": status,
            "provider": "codex_vault",
            "actor": invocation.actor,
            "readiness": {
                "ready": readiness.ready,
                "reason": readiness.reason,
                "command_path_status": _command_path_status(readiness),
                "codex_version": readiness.codex_version,
            },
            "sealed_environment": _sealed_environment_summary(vault_environment, invocation.artifact_dir),
            "auth_materialization": {
                "status": "materialized",
                "copied_auth_material": vault_environment.copied_auth_material,
            },
            "runtime_provider": self.policy.runtime.provider,
            "codex_vault_home_ref": vault_environment.codex_home.relative_to(invocation.artifact_dir).as_posix(),
            "vault_evidence_dir_ref": vault_environment.evidence_dir.relative_to(invocation.artifact_dir).as_posix(),
        }
        if blocked_category is not None:
            payload["blocked_category"] = blocked_category
        if error is not None:
            payload["error"] = error
        if raw_events is not None:
            payload["raw_events"] = raw_events
        if normalized_events is not None:
            payload["normalized_events"] = normalized_events
        if structured_output is not None:
            payload["structured_output"] = structured_output
        if extra:
            payload.update(extra)
        return normalize_sdk_event(payload)


def resolve_codex_command() -> Path | None:
    command = shutil.which(CODEX_COMMAND_NAME)
    if command is None:
        return None
    return Path(command)


def run_codex_command(
    command: list[str],
    *,
    stdin: str | None = None,
    environ: dict[str, str] | None = None,
    timeout: int = 15,
) -> CodexCommandRunResult:
    completed = subprocess.run(
        command,
        input=stdin,
        env=environ,
        capture_output=True,
        text=True,
        timeout=timeout,
        check=False,
    )
    return CodexCommandRunResult(
        stdout=completed.stdout,
        stderr=completed.stderr,
        returncode=completed.returncode,
    )


def build_required_codex_exec_args(*, output_schema: Path, sandbox: Path, model: str) -> list[str]:
    args = [
        "exec",
        "--json",
        "--model",
        model,
        "--output-schema",
        output_schema.as_posix(),
        "--ignore-user-config",
        "--ignore-rules",
        "--ephemeral",
        "--sandbox",
        "read-only",
        "--cd",
        sandbox.as_posix(),
        "--skip-git-repo-check",
        "-c",
        "shell_environment_policy.inherit=none",
        "-",
    ]
    if _FORBIDDEN_CODEX_EXEC_FLAG in args or "--full-auto" in args:
        raise ValueError("forbidden Codex execution flag configured")
    return args


def _materialize_output_schema(invocation: ActorInvocation, schema_source: dict[str, Any]) -> tuple[Path, Path, str]:
    schema_ref = Path("actor_runtime") / "schemas" / f"{invocation.actor}.schema.json"
    schema_path = invocation.artifact_dir / schema_ref
    schema_path.parent.mkdir(parents=True, exist_ok=True)
    schema_path.write_text(
        json.dumps(compile_codex_output_schema(schema_source), sort_keys=True, indent=2),
        encoding="utf-8",
    )
    return schema_ref, schema_path, digest_file(schema_path)


def _parse_codex_json_events(stdout: str) -> ParsedCodexEvents:
    raw_events: list[dict[str, object]] = []
    final_output: dict[str, object] | None = None
    for line_number, line in enumerate(stdout.splitlines(), start=1):
        stripped = line.strip()
        if not stripped:
            continue
        try:
            event = json.loads(stripped)
        except json.JSONDecodeError as exc:
            parsed = ParsedCodexEvents(
                raw_events=raw_events,
                normalized_events=[normalize_sdk_event(event) for event in raw_events],
                final_output=final_output,
            )
            raise CodexEventParseError(f"Codex JSON event line {line_number} is not valid JSON", parsed) from exc
        if not isinstance(event, dict):
            parsed = ParsedCodexEvents(
                raw_events=raw_events,
                normalized_events=[normalize_sdk_event(event) for event in raw_events],
                final_output=final_output,
            )
            raise CodexEventParseError(f"Codex JSON event line {line_number} must be an object", parsed)
        raw_events.append(event)
        candidate = _structured_output_from_event(event)
        if candidate is not None:
            final_output = candidate
    normalized_events = [normalize_sdk_event(event) for event in raw_events]
    return ParsedCodexEvents(raw_events=raw_events, normalized_events=normalized_events, final_output=final_output)


def _structured_output_from_event(event: dict[str, object]) -> dict[str, object] | None:
    for key in ("final_output", "structured_output"):
        value = event.get(key)
        if isinstance(value, dict):
            return value
    output = event.get("output")
    if isinstance(output, dict) and str(event.get("type", "")).lower() in {"final_output", "result", "completed"}:
        return output
    message = event.get("message")
    if isinstance(message, dict):
        value = message.get("structured_output") or message.get("final_output")
        if isinstance(value, dict):
            return value
    msg = event.get("msg")
    if isinstance(msg, dict):
        value = _structured_output_from_event(msg)
        if value is not None:
            return value
    item = event.get("item")
    if isinstance(item, dict):
        if _is_actor_message_item(item):
            value = _structured_output_from_event_item(item)
            if value is not None:
                return value
    return None


def _is_actor_message_item(item: dict[str, object]) -> bool:
    item_type = item.get("type")
    if isinstance(item_type, str) and item_type.lower() in _ACTOR_MESSAGE_ITEM_TYPES:
        return True
    role = item.get("role")
    return isinstance(item_type, str) and item_type.lower() == "message" and isinstance(role, str) and role.lower() in {"agent", "assistant"}


def _structured_output_from_event_item(item: dict[str, object]) -> dict[str, object] | None:
    for key in ("final_output", "structured_output", "output"):
        value = item.get(key)
        if isinstance(value, dict):
            return value
    for text in _event_item_text_values(item):
        parsed = _json_object_from_text(text)
        if parsed is not None:
            return parsed
    return None


def _event_item_text_values(item: dict[str, object]) -> list[str]:
    values: list[str] = []
    for key in ("text", "output_text"):
        value = item.get(key)
        if isinstance(value, str):
            values.append(value)
    content = item.get("content")
    if isinstance(content, str):
        values.append(content)
    elif isinstance(content, dict):
        for key in ("text", "output_text", "content"):
            value = content.get(key)
            if isinstance(value, str):
                values.append(value)
    elif isinstance(content, list):
        for part in content:
            if isinstance(part, str):
                values.append(part)
            elif isinstance(part, dict):
                for key in ("text", "output_text", "content"):
                    value = part.get(key)
                    if isinstance(value, str):
                        values.append(value)
    return values


def _json_object_from_text(text: str) -> dict[str, object] | None:
    stripped = text.strip()
    if stripped.startswith("```"):
        lines = stripped.splitlines()
        if len(lines) >= 3 and lines[-1].strip() == "```":
            stripped = "\n".join(lines[1:-1]).strip()
    try:
        decoded = json.loads(stripped)
    except json.JSONDecodeError:
        return None
    return decoded if isinstance(decoded, dict) else None


def _codex_event_policy_violation(
    events: list[dict[str, object]],
    *,
    sandbox_root: Path,
    invocation: ActorInvocation,
    runtime_project_root: Path,
    user_codex_home: str | None,
    rail_auth_home: Path,
    shell_allowlist: set[str],
    shell_enabled: bool,
) -> str | None:
    capability_violation = audit_codex_event_capabilities(events)
    if capability_violation is not None:
        return capability_violation.reason
    forbidden_roots = [
        invocation.target_root.resolve(strict=False),
        invocation.artifact_dir.resolve(strict=False),
        runtime_project_root.resolve(strict=False),
        rail_auth_home.resolve(strict=False),
    ]
    if user_codex_home:
        forbidden_roots.append(Path(user_codex_home).resolve(strict=False))
    for event in events:
        tool_type = _event_tool_type(event)
        if tool_type == "validation":
            return "validation execution is not allowed"
        if tool_type == "shell":
            shell_events = _shell_events_from_codex_event(event, default_cwd=sandbox_root)
            if not shell_events:
                return "shell event is not parseable"
            for shell_event in shell_events:
                reason = _shell_event_policy_violation(
                    shell_event,
                    sandbox_root=sandbox_root,
                    forbidden_roots=forbidden_roots,
                    shell_allowlist=shell_allowlist,
                    shell_enabled=shell_enabled,
                )
                if reason is not None:
                    return reason
    return None


def _event_tool_type(event: dict[str, object]) -> str | None:
    values: list[object] = []
    for mapping, _cwd in _event_dicts(event):
        values.extend(
            [
                mapping.get("type"),
                mapping.get("event"),
                mapping.get("kind"),
                mapping.get("tool"),
                mapping.get("name"),
            ]
        )
    lowered = " ".join(str(value).lower() for value in values if value is not None)
    if "validation" in lowered:
        return "validation"
    if (
        "shell" in lowered
        or "command_execution" in lowered
        or "exec_command" in lowered
        or any("command" in mapping for mapping, _cwd in _event_dicts(event))
    ):
        return "shell"
    return None


def _event_dicts(event: dict[str, object]) -> list[tuple[dict[str, object], str | None]]:
    dicts: list[tuple[dict[str, object], str | None]] = []
    _append_event_dicts(event, dicts, inherited_cwd=None)
    return dicts


def _append_event_dicts(event: dict[str, object], dicts: list[tuple[dict[str, object], str | None]], *, inherited_cwd: str | None) -> None:
    cwd = _event_cwd(event) or inherited_cwd
    dicts.append((event, cwd))
    for key in ("item", "message", "msg"):
        child = event.get(key)
        if isinstance(child, dict):
            _append_event_dicts(child, dicts, inherited_cwd=cwd)
        elif isinstance(child, list):
            for item in child:
                if isinstance(item, dict):
                    _append_event_dicts(item, dicts, inherited_cwd=cwd)
    content = event.get("content")
    if isinstance(content, dict):
        _append_event_dicts(content, dicts, inherited_cwd=cwd)
    elif isinstance(content, list):
        for item in content:
            if isinstance(item, dict):
                _append_event_dicts(item, dicts, inherited_cwd=cwd)


def _event_cwd(event: dict[str, object]) -> str | None:
    value = event.get("cwd") or event.get("working_dir") or event.get("working_directory")
    return value if isinstance(value, str) else None


def _shell_event_from_codex_event(event: dict[str, object], *, default_cwd: Path | None = None) -> dict[str, object] | None:
    events = _shell_events_from_codex_event(event, default_cwd=default_cwd)
    return events[0] if events else None


def _shell_events_from_codex_event(event: dict[str, object], *, default_cwd: Path | None = None) -> list[dict[str, object]]:
    events: list[dict[str, object]] = []
    for mapping, inherited_cwd in _event_dicts(event):
        command = _command_text(mapping.get("command"))
        cwd = _event_cwd(mapping) or inherited_cwd
        if isinstance(command, str):
            if not isinstance(cwd, str) and default_cwd is not None:
                cwd = default_cwd.as_posix()
            if isinstance(cwd, str):
                command = _unwrap_codex_shell_wrapper(command) or command
                events.append({"command": command, "cwd": cwd})
    return events


def _command_text(value: object) -> str | None:
    if isinstance(value, str):
        return value
    if not isinstance(value, list) or not all(isinstance(part, str) for part in value):
        return None
    unwrapped = _unwrap_codex_shell_wrapper_args(value)
    if unwrapped is not None:
        return unwrapped
    return shlex.join(value)


def _unwrap_codex_shell_wrapper(command: str) -> str | None:
    try:
        args = shlex.split(command)
    except ValueError:
        return None
    if len(args) < 3:
        return None
    shell_path = Path(args[0])
    if shell_path not in _TRUSTED_SHELL_WRAPPER_PATHS:
        return None
    if args[1] not in {"-lc", "-c"}:
        return None
    return args[2]


def _unwrap_codex_shell_wrapper_args(args: list[str]) -> str | None:
    if len(args) < 3:
        return None
    shell_path = Path(args[0])
    if shell_path not in _TRUSTED_SHELL_WRAPPER_PATHS:
        return None
    if args[1] not in {"-lc", "-c"}:
        return None
    return args[2]


def _shell_event_policy_violation(
    event: dict[str, object],
    *,
    sandbox_root: Path,
    forbidden_roots: list[Path],
    shell_allowlist: set[str],
    shell_enabled: bool,
) -> str | None:
    if not shell_enabled:
        return "shell execution is disabled by policy"
    command_value = event.get("command")
    cwd_value = event.get("cwd")
    if not isinstance(command_value, str) or not isinstance(cwd_value, str):
        return "shell event is not parseable"
    cwd = Path(cwd_value).resolve(strict=False)
    sandbox_root = sandbox_root.resolve(strict=False)
    if cwd != sandbox_root and not cwd.is_relative_to(sandbox_root):
        return "shell cwd must stay inside sandbox"
    if _SHELL_OPERATOR_PATTERN.search(command_value) or "$" in command_value:
        return "shell command uses unsupported shell operators"
    try:
        args = shlex.split(command_value)
    except ValueError:
        return "shell command is not parseable"
    if not args:
        return "shell command is empty"
    executable_path = Path(args[0])
    if _executable_token_is_unsafe(args[0], executable_path):
        return "shell executable path is not allowed"
    executable = executable_path.name
    if executable not in shell_allowlist:
        return f"shell executable is not allowed: {executable}"
    write_flag_violation = _write_capable_shell_flag_violation(executable, args[1:])
    if write_flag_violation is not None:
        return write_flag_violation
    if executable_path.is_absolute():
        resolved_executable = executable_path.resolve(strict=False)
        if not _path_is_under_any(resolved_executable, _TRUSTED_SYSTEM_BINARY_ROOTS):
            return "absolute shell command path is not trusted"
        if resolved_executable.name != executable:
            return "absolute shell command resolved executable is not allowed"
    for root in forbidden_roots:
        root_text = root.as_posix()
        if root_text and root_text in command_value:
            return "shell command references a forbidden root"
    operands = _shell_path_operands(executable, args[1:])
    for arg in operands:
        if _argument_references_forbidden_root(arg, cwd=cwd, sandbox_root=sandbox_root, forbidden_roots=forbidden_roots):
            return "shell argument references a forbidden root"
        if _argument_escapes_sandbox(arg, cwd=cwd, sandbox_root=sandbox_root):
            return "shell argument escapes sandbox"
    return None


def _executable_token_is_unsafe(token: str, executable_path: Path) -> bool:
    if _SHELL_VARIABLE_PATTERN.search(token) is not None or token.startswith("~"):
        return True
    if executable_path.is_absolute():
        return False
    return "/" in token or len(executable_path.parts) != 1


def _write_capable_shell_flag_violation(executable: str, args: list[str]) -> str | None:
    if executable == "find" and any(
        arg in {"-delete", "-exec", "-execdir", "-ok", "-okdir", "-fdelete", "-fls", "-fprint", "-fprint0", "-fprintf"} for arg in args
    ):
        return f"shell executable uses write-capable flag: {executable}"
    if executable == "sed" and any(arg == "-i" or arg.startswith("-i") or arg == "--in-place" or arg.startswith("--in-place=") for arg in args):
        return f"shell executable uses write-capable flag: {executable}"
    if executable == "sed" and _sed_script_uses_unsafe_command(args):
        return f"shell executable uses write-capable flag: {executable}"
    if executable == "test" and any(arg in {"-w", "-G", "-O", "-N"} for arg in args):
        return f"shell executable uses write-capable flag: {executable}"
    if executable == "rg" and any(arg == "--pre" or arg.startswith("--pre=") for arg in args):
        return f"shell executable uses write-capable flag: {executable}"
    return None


def _sed_script_uses_unsafe_command(args: list[str]) -> bool:
    scripts: list[str] = []
    index = 0
    while index < len(args):
        arg = args[index]
        if arg in {"-f", "--file"} or arg.startswith("-f") or arg.startswith("--file=") or _sed_compact_option_uses_file_script(arg):
            return True
        if arg in {"-e", "--expression"} and index + 1 < len(args):
            scripts.append(args[index + 1])
            index += 2
            continue
        if arg.startswith("-e") and len(arg) > 2:
            scripts.append(arg[2:])
            index += 1
            continue
        if arg.startswith("--expression="):
            scripts.append(arg.split("=", 1)[1])
            index += 1
            continue
        if not arg.startswith("-"):
            scripts.append(arg)
            break
        index += 1
    return any(_sed_script_contains_unsafe_command(script) for script in scripts)


def _sed_compact_option_uses_file_script(arg: str) -> bool:
    return arg.startswith("-") and not arg.startswith("--") and "f" in arg[1:]


def _sed_script_contains_unsafe_command(script: str) -> bool:
    # r/R reads arbitrary files, w/W writes files, and e executes commands.
    # The initial codex_vault shell subset is read-only inspection, so these
    # sed commands are denied even when their target path is embedded in the
    # sed program instead of a normal shell argument.
    return re.search(r"(^|[;{}\s\d,$!/])(?:r|R|w|W|e)", script) is not None


def _shell_path_operands(executable: str, args: list[str]) -> list[str]:
    operands: list[str] = []
    end_of_options = False
    index = 0
    while index < len(args):
        arg = args[index]
        if end_of_options:
            operands.append(arg)
            index += 1
            continue
        if arg == "--":
            end_of_options = True
            index += 1
            continue
        option_path = _option_path_operand(executable, arg, args, index)
        if option_path is not None:
            operands.append(option_path)
        if arg.startswith("-"):
            if _dash_leading_token_may_be_path(arg):
                operands.append(arg)
            index += 2 if _option_consumes_next_path(executable, arg) and index + 1 < len(args) else 1
            continue
        operands.append(arg)
        index += 1
    return operands


def _option_path_operand(executable: str, arg: str, args: list[str], index: int) -> str | None:
    if executable == "rg":
        if arg in {"-f", "--file", "--ignore-file"} and index + 1 < len(args):
            return args[index + 1]
        for prefix in ("-f", "--file=", "--ignore-file="):
            if arg.startswith(prefix) and len(arg) > len(prefix):
                return arg[len(prefix) :]
    if executable == "find":
        if arg in {"-newer", "-samefile"} and index + 1 < len(args):
            return args[index + 1]
        if arg in {"-files0-from", "--files0-from"} and index + 1 < len(args):
            return args[index + 1]
        for prefix in ("-files0-from=", "--files0-from="):
            if arg.startswith(prefix) and len(arg) > len(prefix):
                return arg[len(prefix) :]
        if arg == "-f" and index + 1 < len(args):
            return args[index + 1]
        if arg.startswith("-f") and len(arg) > 2:
            return arg[2:]
    if executable == "wc":
        if arg == "--files0-from" and index + 1 < len(args):
            return args[index + 1]
        if arg.startswith("--files0-from=") and len(arg) > len("--files0-from="):
            return arg[len("--files0-from=") :]
    if executable == "test" and arg in {"-e", "-f", "-d", "-r", "-s", "-x"} and index + 1 < len(args):
        return args[index + 1]
    return None


def _option_consumes_next_path(executable: str, arg: str) -> bool:
    return (
        (executable == "rg" and arg in {"-f", "--file", "--ignore-file"})
        or (executable == "find" and arg in {"-f", "-newer", "-samefile", "-files0-from", "--files0-from"})
        or (executable == "wc" and arg == "--files0-from")
        or (executable == "test" and arg in {"-e", "-f", "-d", "-r", "-s", "-x"})
    )


def _dash_leading_token_may_be_path(arg: str) -> bool:
    return "/" in arg or ".." in Path(arg).parts


def _argument_references_forbidden_root(arg: str, *, cwd: Path, sandbox_root: Path, forbidden_roots: list[Path]) -> bool:
    if _SHELL_VARIABLE_PATTERN.search(arg) or arg.startswith("~"):
        return True
    path = Path(arg)
    resolved = path.resolve(strict=False) if path.is_absolute() else (cwd / path).resolve(strict=False)
    return any(resolved == root or resolved.is_relative_to(root) for root in forbidden_roots)


def _argument_escapes_sandbox(arg: str, *, cwd: Path, sandbox_root: Path) -> bool:
    if arg in {".", ""}:
        return False
    path = Path(arg)
    if not path.is_absolute() and ".." not in path.parts:
        return False
    resolved = path.resolve(strict=False) if path.is_absolute() else (cwd / path).resolve(strict=False)
    return resolved != sandbox_root and not resolved.is_relative_to(sandbox_root)


def _check_invocation_command_path(
    command_path: Path,
    *,
    artifact_dir: Path,
    target_root: Path,
    sandbox_root: Path,
) -> str | None:
    forbidden_roots = (
        artifact_dir.resolve(strict=False),
        target_root.resolve(strict=False),
        sandbox_root.resolve(strict=False),
    )
    unresolved_path = command_path if command_path.is_absolute() else command_path.absolute()
    paths = (unresolved_path, command_path.resolve(strict=False))
    if any(any(path == root or path.is_relative_to(root) for root in forbidden_roots) for path in paths):
        return "Codex command path is inside a forbidden invocation root"
    return None


def _sealed_environment_summary(vault_environment: VaultEnvironment, artifact_dir: Path) -> dict[str, object]:
    summary: dict[str, object] = {}
    for key in sorted(vault_environment.environ):
        if key in {"HOME", "CODEX_HOME"}:
            summary[key] = vault_environment.codex_home.relative_to(artifact_dir).as_posix()
        elif key in {"TMPDIR", "TMP", "TEMP"}:
            summary[key] = vault_environment.temp_dir.relative_to(artifact_dir).as_posix()
        elif key == "PATH":
            summary[key] = "set"
        else:
            summary[key] = "set"
    return summary


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
