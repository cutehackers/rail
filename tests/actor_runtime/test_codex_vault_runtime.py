from __future__ import annotations

import json
from pathlib import Path
from typing import Any

import rail

from rail.actor_runtime import codex_vault
from rail.actor_runtime.codex_vault import CodexVaultActorRuntime
from rail.actor_runtime.runtime import build_invocation
from rail.actor_runtime.vault_env import VaultEnvironment
from rail.policy import load_effective_policy
from rail.workspace.isolation import tree_digest


def test_codex_vault_runtime_validates_actor_output_and_writes_evidence(tmp_path):
    target = _target_repo(tmp_path)
    (target / "app.txt").write_text("unchanged\n", encoding="utf-8")
    handle = rail.start_task(_draft(target))
    runner = FakeCodexRunner(
        final_output={
            "summary": "Plan",
            "likely_files": [],
            "substeps": [],
            "risks": [],
            "acceptance_criteria_refined": [],
        }
    )
    command = _fake_codex_command(tmp_path)
    runtime = _runtime(tmp_path, command=command, runner=runner)

    result = runtime.run(build_invocation(handle, "planner"))

    assert result.status == "succeeded"
    assert result.runtime_evidence_ref.as_posix().endswith("planner.runtime_evidence.json")
    assert result.structured_output["summary"] == "Plan"
    exec_command = runner.exec_commands[0]
    assert exec_command[:3] == [command.as_posix(), "exec", "--json"]
    assert exec_command[exec_command.index("--model") + 1] == runtime.policy.runtime.model
    assert "--full-auto" not in exec_command
    assert "--dangerously-bypass-approvals-and-sandbox" not in exec_command
    assert exec_command[-1] == "-"
    schema_path = Path(exec_command[exec_command.index("--output-schema") + 1])
    sandbox_root = Path(exec_command[exec_command.index("--cd") + 1])
    assert schema_path == handle.artifact_dir / "actor_runtime" / "schemas" / "planner.schema.json"
    assert sandbox_root != target
    assert sandbox_root.is_dir()
    assert "Run Rail actor planner" in runner.prompts[0]
    assert "You are the Planner actor." in runner.prompts[0]

    evidence = json.loads((handle.artifact_dir / result.runtime_evidence_ref).read_text(encoding="utf-8"))
    assert evidence["provider"] == "codex_vault"
    assert evidence["actor"] == "planner"
    assert evidence["readiness"]["ready"] is True
    assert evidence["sealed_environment"]["CODEX_HOME"] == "actor_runtime/codex_home"
    assert evidence["auth_materialization"]["status"] == "materialized"
    assert evidence["output_schema_ref"] == "actor_runtime/schemas/planner.schema.json"
    assert evidence["output_schema_digest"].startswith("sha256:")
    assert evidence["target_pre_run_tree_digest"] == evidence["post_run_target_tree_digest"]
    assert evidence["sandbox_base_tree_digest"] == tree_digest(sandbox_root)
    assert evidence["structured_output"]["summary"] == "Plan"
    assert evidence["raw_events"]
    assert evidence["normalized_events"]


def test_codex_vault_runtime_parses_codex_item_message_text_output(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    output = {
        "summary": "Plan from message item",
        "likely_files": [],
        "substeps": [],
        "risks": [],
        "acceptance_criteria_refined": [],
    }
    runner = FakeCodexRunner(
        final_output=output,
        final_event_shape="item_message_text",
    )
    runtime = _runtime(tmp_path, command=_fake_codex_command(tmp_path), runner=runner)

    result = runtime.run(build_invocation(handle, "planner"))

    assert result.status == "succeeded"
    assert result.structured_output["summary"] == "Plan from message item"


def test_codex_vault_runtime_parses_msg_wrapped_message_text_output(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    output = {
        "summary": "Plan from msg envelope",
        "likely_files": [],
        "substeps": [],
        "risks": [],
        "acceptance_criteria_refined": [],
    }
    runner = FakeCodexRunner(
        final_output=output,
        final_event_shape="msg_item_message_text",
    )
    runtime = _runtime(tmp_path, command=_fake_codex_command(tmp_path), runner=runner)

    result = runtime.run(build_invocation(handle, "planner"))

    assert result.status == "succeeded"
    assert result.structured_output["summary"] == "Plan from msg envelope"


def test_codex_vault_runtime_blocks_invalid_actor_output(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    runner = FakeCodexRunner(final_output={"wrong": True})
    runtime = _runtime(tmp_path, command=_fake_codex_command(tmp_path), runner=runner)

    result = runtime.run(build_invocation(handle, "planner"))

    assert result.status == "interrupted"
    assert result.blocked_category == "runtime"
    assert "validation failed" in result.structured_output["error"]


def test_codex_vault_runtime_blocks_target_mutation_before_patch_apply(tmp_path):
    target = _target_repo(tmp_path)
    (target / "app.txt").write_text("old\n", encoding="utf-8")
    handle = rail.start_task(_draft(target))
    runner = FakeCodexRunner(
        final_output={
            "summary": "Plan",
            "likely_files": [],
            "substeps": [],
            "risks": [],
            "acceptance_criteria_refined": [],
        },
        before_result=lambda: (target / "app.txt").write_text("mutated\n", encoding="utf-8"),
    )
    runtime = _runtime(tmp_path, command=_fake_codex_command(tmp_path), runner=runner)

    result = runtime.run(build_invocation(handle, "planner"))

    assert result.status == "interrupted"
    assert result.blocked_category == "policy"
    assert "target tree changed" in result.structured_output["error"]


def test_codex_vault_runtime_blocks_command_path_inside_target(tmp_path):
    target = _target_repo(tmp_path)
    command = target / "bin" / "codex"
    command.parent.mkdir()
    command.write_text("#!/bin/sh\nexit 0\n", encoding="utf-8")
    command.chmod(0o755)
    handle = rail.start_task(_draft(target))
    runtime = _runtime(tmp_path, command=command, runner=FakeCodexRunner(final_output={}))

    result = runtime.run(build_invocation(handle, "planner"))

    assert result.status == "interrupted"
    assert result.blocked_category == "policy"
    assert "Codex command path is inside a forbidden invocation root" in result.structured_output["error"]


def test_codex_vault_runtime_blocks_write_capable_shell_event(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    runner = FakeCodexRunner(
        final_output={
            "summary": "Plan",
            "likely_files": [],
            "substeps": [],
            "risks": [],
            "acceptance_criteria_refined": [],
        },
        extra_events=[{"type": "shell", "cwd": "__SANDBOX__", "command": "touch app.txt"}],
    )
    runtime = _runtime(tmp_path, command=_fake_codex_command(tmp_path), runner=runner)

    result = runtime.run(build_invocation(handle, "planner"))

    evidence = json.loads((handle.artifact_dir / result.runtime_evidence_ref).read_text(encoding="utf-8"))
    assert result.status == "interrupted"
    assert result.blocked_category == "policy"
    assert "shell executable is not allowed" in result.structured_output["error"]
    assert evidence["policy_violation"]["reason"] == "shell executable is not allowed: touch"


def test_codex_vault_runtime_blocks_nested_command_execution_event(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    runner = FakeCodexRunner(
        final_output={
            "summary": "Plan",
            "likely_files": [],
            "substeps": [],
            "risks": [],
            "acceptance_criteria_refined": [],
        },
        extra_events=[{"type": "item.started", "item": {"type": "command_execution", "cwd": "__SANDBOX__", "command": "touch app.txt"}}],
    )
    runtime = _runtime(tmp_path, command=_fake_codex_command(tmp_path), runner=runner)

    result = runtime.run(build_invocation(handle, "planner"))

    assert result.status == "interrupted"
    assert result.blocked_category == "policy"
    assert "shell executable is not allowed" in result.structured_output["error"]


def test_codex_vault_runtime_blocks_msg_wrapped_command_execution_event(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    runner = FakeCodexRunner(
        final_output={
            "summary": "Plan",
            "likely_files": [],
            "substeps": [],
            "risks": [],
            "acceptance_criteria_refined": [],
        },
        extra_events=[{"msg": {"type": "exec_command_begin", "cwd": "__SANDBOX__", "command": "touch app.txt"}}],
    )
    runtime = _runtime(tmp_path, command=_fake_codex_command(tmp_path), runner=runner)

    result = runtime.run(build_invocation(handle, "planner"))

    assert result.status == "interrupted"
    assert result.blocked_category == "policy"
    assert "shell executable is not allowed" in result.structured_output["error"]


def test_codex_vault_runtime_blocks_msg_wrapped_nested_item_command_execution_event(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    runner = FakeCodexRunner(
        final_output={
            "summary": "Plan",
            "likely_files": [],
            "substeps": [],
            "risks": [],
            "acceptance_criteria_refined": [],
        },
        extra_events=[{"msg": {"type": "item.started", "item": {"type": "command_execution", "command": "touch app.txt"}}}],
    )
    runtime = _runtime(tmp_path, command=_fake_codex_command(tmp_path), runner=runner)

    result = runtime.run(build_invocation(handle, "planner"))

    assert result.status == "interrupted"
    assert result.blocked_category == "policy"
    assert "shell executable is not allowed" in result.structured_output["error"]


def test_codex_vault_runtime_allows_msg_wrapped_shell_wrapper_read_only_command(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    runner = FakeCodexRunner(
        final_output={
            "summary": "Plan",
            "likely_files": [],
            "substeps": [],
            "risks": [],
            "acceptance_criteria_refined": [],
        },
        extra_events=[{"msg": {"type": "exec_command_begin", "command": "/bin/zsh -lc pwd"}}],
    )
    runtime = _runtime(tmp_path, command=_fake_codex_command(tmp_path), runner=runner)

    result = runtime.run(build_invocation(handle, "planner"))

    assert result.status == "succeeded"


def test_codex_vault_runtime_blocks_msg_wrapped_shell_wrapper_write_command(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    runner = FakeCodexRunner(
        final_output={
            "summary": "Plan",
            "likely_files": [],
            "substeps": [],
            "risks": [],
            "acceptance_criteria_refined": [],
        },
        extra_events=[{"msg": {"type": "exec_command_begin", "command": "/bin/zsh -lc 'touch app.txt'"}}],
    )
    runtime = _runtime(tmp_path, command=_fake_codex_command(tmp_path), runner=runner)

    result = runtime.run(build_invocation(handle, "planner"))

    assert result.status == "interrupted"
    assert result.blocked_category == "policy"
    assert "shell executable is not allowed" in result.structured_output["error"]


def test_codex_vault_runtime_blocks_nested_mcp_tool_call_event(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    runner = FakeCodexRunner(
        final_output={
            "summary": "Plan",
            "likely_files": [],
            "substeps": [],
            "risks": [],
            "acceptance_criteria_refined": [],
        },
        extra_events=[{"type": "item.started", "item": {"type": "mcp_tool_call", "server": "filesystem"}}],
    )
    runtime = _runtime(tmp_path, command=_fake_codex_command(tmp_path), runner=runner)

    result = runtime.run(build_invocation(handle, "planner"))

    assert result.status == "interrupted"
    assert result.blocked_category == "policy"
    assert "MCP invocation is not allowed" in result.structured_output["error"]


def test_codex_vault_runtime_blocks_shell_auth_home_variable_reference(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    runner = FakeCodexRunner(
        final_output={
            "summary": "Plan",
            "likely_files": [],
            "substeps": [],
            "risks": [],
            "acceptance_criteria_refined": [],
        },
        extra_events=[{"type": "shell", "cwd": "__SANDBOX__", "command": "cat $HOME/auth.json"}],
    )
    runtime = _runtime(tmp_path, command=_fake_codex_command(tmp_path), runner=runner)

    result = runtime.run(build_invocation(handle, "planner"))

    assert result.status == "interrupted"
    assert result.blocked_category == "policy"
    assert "unsupported shell operators" in result.structured_output["error"]


def test_codex_vault_runtime_blocks_shell_relative_sandbox_escape(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    runner = FakeCodexRunner(
        final_output={
            "summary": "Plan",
            "likely_files": [],
            "substeps": [],
            "risks": [],
            "acceptance_criteria_refined": [],
        },
        extra_events=[{"type": "shell", "cwd": "__SANDBOX__", "command": "cat ../secret.txt"}],
    )
    runtime = _runtime(tmp_path, command=_fake_codex_command(tmp_path), runner=runner)

    result = runtime.run(build_invocation(handle, "planner"))

    assert result.status == "interrupted"
    assert result.blocked_category == "policy"
    assert "shell argument escapes sandbox" in result.structured_output["error"]


def test_codex_vault_runtime_blocks_shell_ansi_c_quoted_escape(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    runner = FakeCodexRunner(
        final_output={
            "summary": "Plan",
            "likely_files": [],
            "substeps": [],
            "risks": [],
            "acceptance_criteria_refined": [],
        },
        extra_events=[{"type": "shell", "cwd": "__SANDBOX__", "command": "cat $'../secret.txt'"}],
    )
    runtime = _runtime(tmp_path, command=_fake_codex_command(tmp_path), runner=runner)

    result = runtime.run(build_invocation(handle, "planner"))

    assert result.status == "interrupted"
    assert result.blocked_category == "policy"
    assert "unsupported shell operators" in result.structured_output["error"]


def test_codex_vault_runtime_blocks_write_capable_allowed_shell_flags(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    runner = FakeCodexRunner(
        final_output={
            "summary": "Plan",
            "likely_files": [],
            "substeps": [],
            "risks": [],
            "acceptance_criteria_refined": [],
        },
        extra_events=[{"type": "shell", "cwd": "__SANDBOX__", "command": "find . -delete"}],
    )
    runtime = _runtime(tmp_path, command=_fake_codex_command(tmp_path), runner=runner)

    result = runtime.run(build_invocation(handle, "planner"))

    assert result.status == "interrupted"
    assert result.blocked_category == "policy"
    assert "write-capable flag" in result.structured_output["error"]


def test_codex_vault_runtime_blocks_extended_write_capable_shell_flags(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    runner = FakeCodexRunner(
        final_output={
            "summary": "Plan",
            "likely_files": [],
            "substeps": [],
            "risks": [],
            "acceptance_criteria_refined": [],
        },
        extra_events=[{"type": "shell", "cwd": "__SANDBOX__", "command": "sed --in-place s/a/b/ app.txt"}],
    )
    runtime = _runtime(tmp_path, command=_fake_codex_command(tmp_path), runner=runner)

    result = runtime.run(build_invocation(handle, "planner"))

    assert result.status == "interrupted"
    assert result.blocked_category == "policy"
    assert "write-capable flag" in result.structured_output["error"]


def test_codex_vault_runtime_blocks_more_write_capable_shell_forms(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    runner = FakeCodexRunner(
        final_output={
            "summary": "Plan",
            "likely_files": [],
            "substeps": [],
            "risks": [],
            "acceptance_criteria_refined": [],
        },
        extra_events=[{"type": "shell", "cwd": "__SANDBOX__", "command": "sed s/a/b/w out.txt app.txt"}],
    )
    runtime = _runtime(tmp_path, command=_fake_codex_command(tmp_path), runner=runner)

    result = runtime.run(build_invocation(handle, "planner"))

    assert result.status == "interrupted"
    assert result.blocked_category == "policy"
    assert "write-capable flag" in result.structured_output["error"]


def test_codex_vault_runtime_blocks_sed_addressed_write_commands(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    runner = FakeCodexRunner(
        final_output={
            "summary": "Plan",
            "likely_files": [],
            "substeps": [],
            "risks": [],
            "acceptance_criteria_refined": [],
        },
        extra_events=[{"type": "shell", "cwd": "__SANDBOX__", "command": "sed -n 1w out.txt app.txt"}],
    )
    runtime = _runtime(tmp_path, command=_fake_codex_command(tmp_path), runner=runner)

    result = runtime.run(build_invocation(handle, "planner"))

    assert result.status == "interrupted"
    assert result.blocked_category == "policy"
    assert "write-capable flag" in result.structured_output["error"]


def test_codex_vault_runtime_blocks_sed_compact_write_commands(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    runner = FakeCodexRunner(
        final_output={
            "summary": "Plan",
            "likely_files": [],
            "substeps": [],
            "risks": [],
            "acceptance_criteria_refined": [],
        },
        extra_events=[{"type": "shell", "cwd": "__SANDBOX__", "command": "sed 1wout.txt app.txt"}],
    )
    runtime = _runtime(tmp_path, command=_fake_codex_command(tmp_path), runner=runner)

    result = runtime.run(build_invocation(handle, "planner"))

    assert result.status == "interrupted"
    assert result.blocked_category == "policy"
    assert "write-capable flag" in result.structured_output["error"]


def test_codex_vault_runtime_blocks_sed_read_file_commands(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    runner = FakeCodexRunner(
        final_output={
            "summary": "Plan",
            "likely_files": [],
            "substeps": [],
            "risks": [],
            "acceptance_criteria_refined": [],
        },
        extra_events=[{"type": "shell", "cwd": "__SANDBOX__", "command": "sed 1r ../secret.txt app.txt"}],
    )
    runtime = _runtime(tmp_path, command=_fake_codex_command(tmp_path), runner=runner)

    result = runtime.run(build_invocation(handle, "planner"))

    assert result.status == "interrupted"
    assert result.blocked_category == "policy"
    assert "write-capable flag" in result.structured_output["error"]


def test_codex_vault_runtime_blocks_compact_sed_file_script_option(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    runner = FakeCodexRunner(
        final_output={
            "summary": "Plan",
            "likely_files": [],
            "substeps": [],
            "risks": [],
            "acceptance_criteria_refined": [],
        },
        extra_events=[{"type": "shell", "cwd": "__SANDBOX__", "command": "sed -f../secret.sed app.txt"}],
    )
    runtime = _runtime(tmp_path, command=_fake_codex_command(tmp_path), runner=runner)

    result = runtime.run(build_invocation(handle, "planner"))

    assert result.status == "interrupted"
    assert result.blocked_category == "policy"
    assert "write-capable flag" in result.structured_output["error"]


def test_codex_vault_runtime_blocks_relative_shell_executable_paths(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    runner = FakeCodexRunner(
        final_output={
            "summary": "Plan",
            "likely_files": [],
            "substeps": [],
            "risks": [],
            "acceptance_criteria_refined": [],
        },
        extra_events=[{"type": "shell", "cwd": "__SANDBOX__", "command": "./cat app.txt"}],
    )
    runtime = _runtime(tmp_path, command=_fake_codex_command(tmp_path), runner=runner)

    result = runtime.run(build_invocation(handle, "planner"))

    assert result.status == "interrupted"
    assert result.blocked_category == "policy"
    assert "shell executable path is not allowed" in result.structured_output["error"]


def test_codex_vault_runtime_allows_trusted_absolute_shell_binary_path(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    runner = FakeCodexRunner(
        final_output={
            "summary": "Plan",
            "likely_files": [],
            "substeps": [],
            "risks": [],
            "acceptance_criteria_refined": [],
        },
        extra_events=[{"type": "shell", "cwd": "__SANDBOX__", "command": "/bin/ls ."}],
    )
    runtime = _runtime(tmp_path, command=_fake_codex_command(tmp_path), runner=runner)

    result = runtime.run(build_invocation(handle, "planner"))

    assert result.status == "succeeded"


def test_codex_vault_runtime_blocks_rg_pre_execution(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    runner = FakeCodexRunner(
        final_output={
            "summary": "Plan",
            "likely_files": [],
            "substeps": [],
            "risks": [],
            "acceptance_criteria_refined": [],
        },
        extra_events=[{"type": "shell", "cwd": "__SANDBOX__", "command": "rg --pre ./cat needle ."}],
    )
    runtime = _runtime(tmp_path, command=_fake_codex_command(tmp_path), runner=runner)

    result = runtime.run(build_invocation(handle, "planner"))

    assert result.status == "interrupted"
    assert result.blocked_category == "policy"
    assert "write-capable flag" in result.structured_output["error"]


def test_codex_vault_runtime_blocks_shell_newline_chaining(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    runner = FakeCodexRunner(
        final_output={
            "summary": "Plan",
            "likely_files": [],
            "substeps": [],
            "risks": [],
            "acceptance_criteria_refined": [],
        },
        extra_events=[{"type": "shell", "cwd": "__SANDBOX__", "command": "cat app.txt\nrm app.txt"}],
    )
    runtime = _runtime(tmp_path, command=_fake_codex_command(tmp_path), runner=runner)

    result = runtime.run(build_invocation(handle, "planner"))

    assert result.status == "interrupted"
    assert result.blocked_category == "policy"
    assert "unsupported shell operators" in result.structured_output["error"]


def test_codex_vault_runtime_rejects_generator_output_with_multiple_patch_sources(tmp_path):
    target = _target_repo(tmp_path)
    handle = rail.start_task(_draft(target))
    runner = FakeCodexRunner(
        final_output={
            "changed_files": ["app.txt"],
            "patch_summary": ["Update app.txt"],
            "tests_added_or_updated": [],
            "known_limits": [],
            "patch_bundle_ref": "patches/generator.patch.yaml",
            "patch_bundle": {
                "schema_version": "1",
                "base_tree_digest": tree_digest(target),
                "operations": [{"op": "write", "path": "app.txt", "content": "new\n"}],
            },
        }
    )
    runtime = _runtime(tmp_path, command=_fake_codex_command(tmp_path), runner=runner)

    result = runtime.run(build_invocation(handle, "generator"))

    assert result.status == "interrupted"
    assert result.blocked_category == "runtime"
    assert "exactly one patch source" in result.structured_output["error"]
    schema = json.loads((handle.artifact_dir / "actor_runtime" / "schemas" / "generator.schema.json").read_text(encoding="utf-8"))
    assert "oneOf" in schema


class FakeCodexRunner:
    def __init__(
        self,
        *,
        final_output: dict[str, Any],
        extra_events: list[dict[str, Any]] | None = None,
        before_result=None,
        final_event_shape: str = "top_level_output",
    ) -> None:
        self.final_output = final_output
        self.extra_events = extra_events or []
        self.before_result = before_result
        self.final_event_shape = final_event_shape
        self.commands: list[list[str]] = []
        self.exec_commands: list[list[str]] = []
        self.prompts: list[str] = []

    def __call__(
        self,
        command: list[str],
        *,
        stdin: str | None = None,
        environ: dict[str, str] | None = None,
        timeout: int | None = None,
    ):
        self.commands.append(command)
        if command[1:] == ["--version"]:
            return codex_vault.CodexCommandRunResult(stdout="codex-cli 0.124.0", stderr="", returncode=0)
        if command[1:] == ["exec", "--help"]:
            return codex_vault.CodexCommandRunResult(
                stdout="\n".join(codex_vault.CODEX_EXEC_REQUIRED_HELP_FLAGS),
                stderr="",
                returncode=0,
            )
        if command[1:3] == ["exec", "--json"]:
            self.exec_commands.append(command)
            self.prompts.append(stdin or "")
            assert environ is not None
            assert timeout is not None
            sandbox_root = command[command.index("--cd") + 1]
            events = [
                {"type": "session", "status": "started"},
                *[_replace_sandbox(event, sandbox_root) for event in self.extra_events],
            ]
            if self.before_result is not None:
                self.before_result()
            events.append(self._final_event())
            return codex_vault.CodexCommandRunResult(
                stdout="\n".join(json.dumps(event, sort_keys=True) for event in events),
                stderr="",
                returncode=0,
            )
        raise AssertionError(f"unexpected command: {command}")

    def _final_event(self) -> dict[str, object]:
        if self.final_event_shape == "item_message_text":
            return {
                "type": "item.completed",
                "item": {
                    "type": "agent_message",
                    "content": [{"type": "output_text", "text": json.dumps(self.final_output, sort_keys=True)}],
                },
            }
        if self.final_event_shape == "msg_item_message_text":
            return {
                "msg": {
                    "type": "item.completed",
                    "item": {
                        "type": "agent_message",
                        "content": [{"type": "output_text", "text": json.dumps(self.final_output, sort_keys=True)}],
                    },
                }
            }
        return {"type": "final_output", "output": self.final_output}


def _replace_sandbox(event: dict[str, Any], sandbox_root: str) -> dict[str, Any]:
    replaced: dict[str, Any] = {}
    for key, value in event.items():
        if value == "__SANDBOX__":
            replaced[key] = sandbox_root
        elif isinstance(value, dict):
            replaced[key] = _replace_sandbox(value, sandbox_root)
        elif isinstance(value, list):
            replaced[key] = [
                _replace_sandbox(item, sandbox_root) if isinstance(item, dict) else (sandbox_root if item == "__SANDBOX__" else item)
                for item in value
            ]
        else:
            replaced[key] = value
    return replaced


def _runtime(tmp_path: Path, *, command: Path, runner: FakeCodexRunner) -> CodexVaultActorRuntime:
    return CodexVaultActorRuntime(
        project_root=Path("."),
        policy=load_effective_policy(tmp_path),
        command_resolver=lambda: command,
        command_trust_checker=lambda _path, _target_root, _artifact_dir: None,
        runner=runner,
        environment_materializer=_fake_vault_environment,
    )


def _fake_vault_environment(*, artifact_dir: Path, auth_home: Path, base_environ: dict[str, str], actor: str | None = None) -> VaultEnvironment:
    codex_home = artifact_dir / "actor_runtime" / "codex_home"
    evidence_dir = artifact_dir / "actor_runtime" / "evidence"
    temp_dir = artifact_dir / "actor_runtime" / "tmp"
    for path in (codex_home, evidence_dir, temp_dir):
        path.mkdir(parents=True, exist_ok=True)
    return VaultEnvironment(
        codex_home=codex_home,
        evidence_dir=evidence_dir,
        temp_dir=temp_dir,
        environ={
            "PATH": "/usr/bin",
            "HOME": str(codex_home),
            "CODEX_HOME": str(codex_home),
            "TMPDIR": str(temp_dir),
            "TMP": str(temp_dir),
            "TEMP": str(temp_dir),
        },
        copied_auth_material=["auth.json"],
    )


def _fake_codex_command(tmp_path: Path) -> Path:
    command = tmp_path / "trusted-bin" / "codex"
    command.parent.mkdir(exist_ok=True)
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
        "goal": "Exercise Codex Vault execution.",
        "definition_of_done": ["Codex Vault execution is parsed."],
    }
