from __future__ import annotations

import sys

import pytest

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
