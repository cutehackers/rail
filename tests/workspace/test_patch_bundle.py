from __future__ import annotations

from pathlib import Path

import pytest

from rail.workspace.apply import apply_patch_bundle
from pydantic import ValidationError

from rail.workspace.patch_bundle import (
    PatchBundle,
    PatchOperation,
    PatchValidationPolicy,
    build_patch_bundle,
    validate_patch_bundle,
)
from rail.workspace.sandbox import create_sandbox, write_sandbox_file


def test_sandbox_is_created_outside_target_and_target_unchanged_until_apply(tmp_path):
    target = _target(tmp_path)
    (target / "app.txt").write_text("old\n", encoding="utf-8")
    sandbox = create_sandbox(target)

    assert not sandbox.sandbox_root.is_relative_to(target)

    write_sandbox_file(sandbox, "app.txt", "new\n")
    bundle = build_patch_bundle(sandbox, ["app.txt"])

    assert (target / "app.txt").read_text(encoding="utf-8") == "old\n"
    apply_patch_bundle(bundle, target)
    assert (target / "app.txt").read_text(encoding="utf-8") == "new\n"


@pytest.mark.parametrize("path", ["/tmp/escape.txt", "../escape.txt", ".harness/artifacts/run.txt"])
def test_patch_bundle_rejects_unsafe_paths(path):
    bundle = PatchBundle(base_tree_digest="sha256:base", operations=[PatchOperation(path=path, content="x")])

    with pytest.raises(ValueError):
        validate_patch_bundle(bundle)


def test_patch_bundle_rejects_size_and_file_count_limits():
    many = PatchBundle(
        base_tree_digest="sha256:base",
        operations=[PatchOperation(path=f"f{i}.txt", content="x") for i in range(101)],
    )
    with pytest.raises(ValueError, match="file count"):
        validate_patch_bundle(many, policy=PatchValidationPolicy(max_files=100))

    large = PatchBundle(base_tree_digest="sha256:base", operations=[PatchOperation(path="large.txt", content="x" * 11)])
    with pytest.raises(ValueError, match="size"):
        validate_patch_bundle(large, policy=PatchValidationPolicy(max_bytes=10))


def test_patch_bundle_rejects_binary_and_executable_without_policy():
    binary = PatchBundle(base_tree_digest="sha256:base", operations=[PatchOperation(path="asset.bin", content="x", binary=True)])
    with pytest.raises(ValueError, match="binary"):
        validate_patch_bundle(binary)

    executable = PatchBundle(
        base_tree_digest="sha256:base", operations=[PatchOperation(path="script.sh", content="echo ok\n", executable=True)]
    )
    with pytest.raises(ValueError, match="executable"):
        validate_patch_bundle(executable)


def test_patch_bundle_cannot_self_authorize_policy_flags():
    with pytest.raises(ValidationError):
        PatchBundle(
            base_tree_digest="sha256:base",
            operations=[PatchOperation(path=".harness/artifacts/run.txt", content="x")],
            allow_artifact_writes=True,
        )


def test_patch_bundle_policy_flags_come_from_effective_policy():
    binary = PatchBundle(base_tree_digest="sha256:base", operations=[PatchOperation(path="asset.bin", content="x", binary=True)])

    validate_patch_bundle(binary, policy=PatchValidationPolicy(allow_binary=True))


def test_multi_file_apply_succeeds_without_partial_self_digest_failure(tmp_path):
    target = _target(tmp_path)
    (target / "a.txt").write_text("old a\n", encoding="utf-8")
    (target / "b.txt").write_text("old b\n", encoding="utf-8")
    sandbox = create_sandbox(target)
    write_sandbox_file(sandbox, "a.txt", "new a\n")
    write_sandbox_file(sandbox, "b.txt", "new b\n")
    bundle = build_patch_bundle(sandbox, ["a.txt", "b.txt"])

    apply_patch_bundle(bundle, target)

    assert (target / "a.txt").read_text(encoding="utf-8") == "new a\n"
    assert (target / "b.txt").read_text(encoding="utf-8") == "new b\n"


def test_apply_rejects_stale_target_tree(tmp_path):
    target = _target(tmp_path)
    (target / "app.txt").write_text("old\n", encoding="utf-8")
    sandbox = create_sandbox(target)
    write_sandbox_file(sandbox, "app.txt", "new\n")
    bundle = build_patch_bundle(sandbox, ["app.txt"])
    (target / "other.txt").write_text("changed concurrently\n", encoding="utf-8")

    with pytest.raises(ValueError, match="base tree"):
        apply_patch_bundle(bundle, target)


def _target(tmp_path: Path) -> Path:
    target = tmp_path / "target"
    target.mkdir()
    return target
