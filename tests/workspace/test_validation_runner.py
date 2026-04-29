from __future__ import annotations

import sys

import pytest

import rail.workspace.validation_runner as validation_runner
from rail.workspace.isolation import tree_digest
from rail.workspace.validation_runner import ValidationCommand, run_validation_command


def test_validation_runner_records_pass_with_redacted_logs(tmp_path):
    artifact = tmp_path / "artifact"
    target = tmp_path / "target"
    artifact.mkdir()
    target.mkdir()

    evidence = run_validation_command(
        artifact_dir=artifact,
        target_root=target,
        command=ValidationCommand(
            argv=[sys.executable, "-c", "print('OPENAI_API_KEY=sk-secret-value')"],
            source="policy",
        ),
        patch_digest="sha256:patch",
        request_digest="sha256:request",
        effective_policy_digest="sha256:policy",
        actor_invocation_digest="sha256:actor",
    )

    assert evidence.status == "pass"
    assert evidence.source == "policy"
    assert evidence.mutation_status == "clean"
    assert evidence.duration_ms >= 0
    assert "sk-secret-value" not in (artifact / evidence.stdout_ref).read_text(encoding="utf-8")


def test_validation_runner_rejects_actor_owned_command(tmp_path):
    artifact = tmp_path / "artifact"
    target = tmp_path / "target"
    artifact.mkdir()
    target.mkdir()

    with pytest.raises(ValueError, match="request or policy"):
        run_validation_command(
            artifact_dir=artifact,
            target_root=target,
            command=ValidationCommand(argv=[sys.executable, "-c", "print('actor')"], source="actor"),
            patch_digest="sha256:patch",
            request_digest="sha256:request",
            effective_policy_digest="sha256:policy",
            actor_invocation_digest="sha256:actor",
        )


def test_validation_runner_records_target_mutation(tmp_path):
    artifact = tmp_path / "artifact"
    target = tmp_path / "target"
    artifact.mkdir()
    target.mkdir()
    before = tree_digest(target)

    evidence = run_validation_command(
        artifact_dir=artifact,
        target_root=target,
        command=ValidationCommand(
            argv=[sys.executable, "-c", "from pathlib import Path; Path('generated.txt').write_text('changed')"],
            source="request",
        ),
        patch_digest="sha256:patch",
        request_digest="sha256:request",
        effective_policy_digest="sha256:policy",
        actor_invocation_digest="sha256:actor",
    )

    assert evidence.status == "pass"
    assert evidence.mutation_status == "mutated"
    assert evidence.tree_digest != before


def test_validation_runner_scrubs_secret_environment(tmp_path, monkeypatch):
    artifact = tmp_path / "artifact"
    target = tmp_path / "target"
    artifact.mkdir()
    target.mkdir()
    monkeypatch.setenv("OPENAI_API_KEY", "sk-secret-value")

    evidence = run_validation_command(
        artifact_dir=artifact,
        target_root=target,
        command=ValidationCommand(
            argv=[sys.executable, "-c", "import os; print(os.environ.get('OPENAI_API_KEY', 'missing'))"],
            source="policy",
        ),
        patch_digest="sha256:patch",
        request_digest="sha256:request",
        effective_policy_digest="sha256:policy",
        actor_invocation_digest="sha256:actor",
    )

    assert evidence.credential_mode == "scrubbed"
    assert "sk-secret-value" not in (artifact / evidence.stdout_ref).read_text(encoding="utf-8")
    assert "missing" in (artifact / evidence.stdout_ref).read_text(encoding="utf-8")


def test_validation_runner_records_harness_mutation(tmp_path):
    artifact = tmp_path / "artifact"
    target = tmp_path / "target"
    artifact.mkdir()
    target.mkdir()

    evidence = run_validation_command(
        artifact_dir=artifact,
        target_root=target,
        command=ValidationCommand(
            argv=[sys.executable, "-c", "from pathlib import Path; Path('.harness/touched').parent.mkdir(); Path('.harness/touched').write_text('changed')"],
            source="policy",
        ),
        patch_digest="sha256:patch",
        request_digest="sha256:request",
        effective_policy_digest="sha256:policy",
        actor_invocation_digest="sha256:actor",
    )

    assert evidence.mutation_status == "mutated"
    assert evidence.network_mode == "disabled"


def test_validation_runner_records_missing_command_as_failure(tmp_path):
    artifact = tmp_path / "artifact"
    target = tmp_path / "target"
    artifact.mkdir()
    target.mkdir()

    evidence = run_validation_command(
        artifact_dir=artifact,
        target_root=target,
        command=ValidationCommand(argv=["definitely-not-a-rail-command"], source="policy"),
        patch_digest="sha256:patch",
        request_digest="sha256:request",
        effective_policy_digest="sha256:policy",
        actor_invocation_digest="sha256:actor",
    )

    assert evidence.status == "fail"
    assert evidence.exit_code == 127


def test_validation_runner_does_not_execute_when_disabled_network_sandbox_unavailable(tmp_path, monkeypatch):
    artifact = tmp_path / "artifact"
    target = tmp_path / "target"
    marker = target / "marker.txt"
    artifact.mkdir()
    target.mkdir()
    monkeypatch.setattr(validation_runner, "_TRUSTED_SANDBOX_EXEC", tmp_path / "missing-sandbox-exec", raising=False)

    evidence = run_validation_command(
        artifact_dir=artifact,
        target_root=target,
        command=ValidationCommand(
            argv=[sys.executable, "-c", "from pathlib import Path; Path('marker.txt').write_text('executed')"],
            source="policy",
        ),
        patch_digest="sha256:patch",
        request_digest="sha256:request",
        effective_policy_digest="sha256:policy",
        actor_invocation_digest="sha256:actor",
        network_mode="disabled",
    )

    assert evidence.status == "fail"
    assert evidence.network_mode == "unavailable"
    assert not marker.exists()


def test_validation_runner_ignores_target_controlled_sandbox_exec_on_path(tmp_path, monkeypatch):
    artifact = tmp_path / "artifact"
    target = tmp_path / "target"
    command_marker = target / "command-marker.txt"
    shim_marker = target / "shim-marker.txt"
    artifact.mkdir()
    target.mkdir()
    shim = target / "sandbox-exec"
    shim.write_text(f"#!/bin/sh\nprintf shim > {shim_marker}\nexit 0\n", encoding="utf-8")
    shim.chmod(0o755)
    monkeypatch.setenv("PATH", str(target))
    monkeypatch.setattr(validation_runner, "_TRUSTED_SANDBOX_EXEC", tmp_path / "missing-sandbox-exec", raising=False)

    evidence = run_validation_command(
        artifact_dir=artifact,
        target_root=target,
        command=ValidationCommand(
            argv=[sys.executable, "-c", "from pathlib import Path; Path('command-marker.txt').write_text('executed')"],
            source="policy",
        ),
        patch_digest="sha256:patch",
        request_digest="sha256:request",
        effective_policy_digest="sha256:policy",
        actor_invocation_digest="sha256:actor",
        network_mode="disabled",
    )

    assert evidence.status == "fail"
    assert evidence.network_mode == "unavailable"
    assert not command_marker.exists()
    assert not shim_marker.exists()
