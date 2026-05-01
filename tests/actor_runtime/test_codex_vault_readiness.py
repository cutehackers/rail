from __future__ import annotations

import json
import subprocess
from pathlib import Path

import rail
from rail.actor_runtime import codex_vault
from rail.actor_runtime.codex_vault import CodexVaultActorRuntime
from rail.actor_runtime.runtime import build_invocation
from rail.policy import load_effective_policy


def test_readiness_blocks_when_codex_command_missing(tmp_path):
    runtime = CodexVaultActorRuntime(
        project_root=Path("."),
        policy=load_effective_policy(tmp_path),
        command_resolver=lambda: None,
    )

    readiness = runtime.readiness()

    assert readiness.ready is False
    assert readiness.blocked_category == "environment"
    assert "Codex command" in readiness.reason
    assert readiness.command_path is None


def test_readiness_blocks_unsupported_codex_version(tmp_path):
    runner = FakeCodexRunner(version_stdout="codex-cli 0.0.0")
    command = _fake_codex_command(tmp_path)
    runtime = CodexVaultActorRuntime(
        project_root=Path("."),
        policy=load_effective_policy(tmp_path),
        command_resolver=lambda: command,
        command_trust_checker=lambda _path, _target_root, _artifact_dir: None,
        runner=runner,
    )

    readiness = runtime.readiness()

    assert readiness.ready is False
    assert readiness.blocked_category == "environment"
    assert readiness.command_path == command
    assert readiness.codex_version == "codex-cli 0.0.0"
    assert "minimum supported version" in readiness.reason
    assert runner.commands[0] == [command.as_posix(), "--version"]


def test_readiness_accepts_supported_command_identity(tmp_path):
    runner = FakeCodexRunner(version_stdout="codex-cli 0.124.0")
    command = _fake_codex_command(tmp_path)
    runtime = CodexVaultActorRuntime(
        project_root=Path("."),
        policy=load_effective_policy(tmp_path),
        command_resolver=lambda: command,
        command_trust_checker=lambda _path, _target_root, _artifact_dir: None,
        runner=runner,
    )

    readiness = runtime.readiness()

    assert readiness.ready is True
    assert readiness.blocked_category is None
    assert readiness.command_path == command
    assert readiness.codex_version == "codex-cli 0.124.0"
    assert runner.commands == [
        [command.as_posix(), "--version"],
        [command.as_posix(), "exec", "--help"],
    ]


def test_readiness_blocks_when_help_omits_required_exec_flag(tmp_path):
    runner = FakeCodexRunner(version_stdout="codex-cli 0.124.0", omitted_help_flag="--ephemeral")
    command = _fake_codex_command(tmp_path)
    runtime = CodexVaultActorRuntime(
        project_root=Path("."),
        policy=load_effective_policy(tmp_path),
        command_resolver=lambda: command,
        command_trust_checker=lambda _path, _target_root, _artifact_dir: None,
        runner=runner,
    )

    readiness = runtime.readiness()

    assert readiness.ready is False
    assert readiness.blocked_category == "environment"
    assert "--ephemeral" in readiness.reason


def test_default_trust_checker_rejects_temp_command_path(tmp_path):
    command = _fake_codex_command(tmp_path)

    reason = codex_vault.check_trusted_codex_command(command, tmp_path, None)

    assert reason is not None
    assert "trusted install directory" in reason or "temporary directory" in reason


def test_default_trust_checker_rejects_unsafe_permissions(tmp_path):
    homebrew_bin = tmp_path / "opt" / "homebrew" / "bin"
    command = homebrew_bin / "codex"
    command.parent.mkdir(parents=True)
    command.write_text("#!/bin/sh\nexit 0\n", encoding="utf-8")
    command.chmod(0o777)

    reason = codex_vault._check_codex_command_permissions(command, command)

    assert reason is not None
    assert "group-writable or world-writable" in reason


def test_required_execution_flags_do_not_include_forbidden_bypass_flag():
    flags = codex_vault.build_required_codex_exec_args(
        output_schema=Path("/absolute/path/to/actor-output-schema.json"),
        sandbox=Path("/absolute/path/to/sandbox"),
        model="gpt-test",
    )

    assert "--dangerously-bypass-approvals-and-sandbox" not in flags
    assert flags[flags.index("--model") + 1] == "gpt-test"
    assert "--skip-git-repo-check" in flags


def test_readiness_blocks_when_runner_raises_timeout(tmp_path):
    command = _fake_codex_command(tmp_path)

    def runner(_command: list[str]):
        raise subprocess.TimeoutExpired(cmd="codex --version", timeout=15)

    runtime = CodexVaultActorRuntime(
        project_root=Path("."),
        policy=load_effective_policy(tmp_path),
        command_resolver=lambda: command,
        command_trust_checker=lambda _path, _target_root, _artifact_dir: None,
        runner=runner,
    )

    readiness = runtime.readiness()

    assert readiness.ready is False
    assert readiness.blocked_category == "environment"
    assert "check failed" in readiness.reason


def test_readiness_blocks_when_runner_raises_os_error(tmp_path):
    command = _fake_codex_command(tmp_path)

    def runner(_command: list[str]):
        raise PermissionError("not executable")

    runtime = CodexVaultActorRuntime(
        project_root=Path("."),
        policy=load_effective_policy(tmp_path),
        command_resolver=lambda: command,
        command_trust_checker=lambda _path, _target_root, _artifact_dir: None,
        runner=runner,
    )

    readiness = runtime.readiness()

    assert readiness.ready is False
    assert readiness.blocked_category == "environment"
    assert "check failed" in readiness.reason


def test_rejected_command_path_is_not_written_to_runtime_evidence(tmp_path):
    target = _target_repo(tmp_path)
    handle = rail.start_task(_draft(target))
    untrusted_command = tmp_path / "private-home" / "bin" / "codex"
    untrusted_command.parent.mkdir(parents=True)
    untrusted_command.write_text("#!/bin/sh\nexit 0\n", encoding="utf-8")
    runtime = CodexVaultActorRuntime(
        project_root=Path("."),
        policy=load_effective_policy(tmp_path),
        command_resolver=lambda: untrusted_command,
        command_trust_checker=lambda _path, _target_root, _artifact_dir: "Codex command path is untrusted",
    )

    result = runtime.run(build_invocation(handle, "planner"))

    evidence = json.loads((handle.artifact_dir / result.runtime_evidence_ref).read_text(encoding="utf-8"))
    assert result.blocked_category == "environment"
    assert "command_path" not in evidence
    assert evidence["command_path_status"] == "untrusted"
    assert untrusted_command.as_posix() not in json.dumps(evidence)


class FakeCodexRunner:
    def __init__(self, *, version_stdout: str, omitted_help_flag: str | None = None) -> None:
        self.version_stdout = version_stdout
        self.omitted_help_flag = omitted_help_flag
        self.commands: list[list[str]] = []

    def __call__(self, command: list[str]):
        self.commands.append(command)
        if command[1:] == ["--version"]:
            return codex_vault.CodexCommandRunResult(stdout=self.version_stdout, stderr="", returncode=0)
        if command[1:] == ["exec", "--help"]:
            flags = set(codex_vault.CODEX_EXEC_REQUIRED_HELP_FLAGS)
            if self.omitted_help_flag is not None:
                flags.remove(self.omitted_help_flag)
            return codex_vault.CodexCommandRunResult(stdout="\n".join(sorted(flags)), stderr="", returncode=0)
        raise AssertionError(f"unexpected command: {command}")


def _fake_codex_command(tmp_path: Path) -> Path:
    command = tmp_path / "bin" / "codex"
    command.parent.mkdir()
    command.write_text("#!/bin/sh\nexit 0\n", encoding="utf-8")
    command.chmod(0o755)
    return command


def _target_repo(tmp_path: Path) -> Path:
    target = tmp_path / "target-repo"
    target.mkdir(parents=True, exist_ok=True)
    return target


def _draft(target: Path) -> dict[str, object]:
    return {
        "project_root": str(target),
        "task_type": "bug_fix",
        "goal": "Check Codex readiness.",
        "definition_of_done": ["Readiness blocks safely."],
    }
