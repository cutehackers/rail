from __future__ import annotations

import os
from pathlib import Path

import pytest

import rail.workspace.apply as apply_module
from rail.workspace.apply import apply_patch_bundle
from rail.workspace.isolation import tree_digest
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


def test_apply_rejects_hardlink_targets_without_mutating_outside_file(tmp_path):
    target = _target(tmp_path)
    outside = tmp_path / "outside.txt"
    outside.write_text("outside\n", encoding="utf-8")
    linked = target / "linked.txt"
    try:
        os.link(outside, linked)
    except OSError:
        pytest.skip("hardlinks are not supported on this filesystem")
    bundle = PatchBundle(
        base_tree_digest=tree_digest(target),
        operations=[PatchOperation(path="linked.txt", content="changed\n")],
    )

    with pytest.raises(ValueError, match="hardlink"):
        apply_patch_bundle(bundle, target)

    assert outside.read_text(encoding="utf-8") == "outside\n"


def test_apply_rejects_symlink_targets_without_mutating_link_destination(tmp_path):
    target = _target(tmp_path)
    destination = target / "real.txt"
    destination.write_text("real\n", encoding="utf-8")
    link = target / "link.txt"
    link.symlink_to(destination)
    bundle = PatchBundle(
        base_tree_digest=tree_digest(target),
        operations=[PatchOperation(path="link.txt", content="changed\n")],
    )

    with pytest.raises(ValueError, match="symlink"):
        apply_patch_bundle(bundle, target)

    assert destination.read_text(encoding="utf-8") == "real\n"


def test_apply_rechecks_link_safety_at_write_time(tmp_path, monkeypatch):
    target = _target(tmp_path)
    app = target / "app.txt"
    outside = tmp_path / "outside.txt"
    app.write_text("old\n", encoding="utf-8")
    outside.write_text("outside\n", encoding="utf-8")
    base_digest = tree_digest(target)
    bundle = PatchBundle(
        base_tree_digest=base_digest,
        operations=[PatchOperation(path="app.txt", content="changed\n")],
    )
    calls = 0

    def swapping_tree_digest(root: Path) -> str:
        nonlocal calls
        calls += 1
        if calls == 2:
            app.unlink()
            app.symlink_to(outside)
        return base_digest

    monkeypatch.setattr(apply_module, "tree_digest", swapping_tree_digest)

    with pytest.raises(ValueError, match="symlink"):
        apply_patch_bundle(bundle, target)

    assert outside.read_text(encoding="utf-8") == "outside\n"


def test_apply_replaces_existing_file_instead_of_mutating_late_hardlink(tmp_path, monkeypatch):
    target = _target(tmp_path)
    app = target / "app.txt"
    outside_link = tmp_path / "outside-link.txt"
    app.write_text("old\n", encoding="utf-8")
    bundle = PatchBundle(
        base_tree_digest=tree_digest(target),
        operations=[PatchOperation(path="app.txt", content="new\n")],
    )
    original_assert = apply_module._assert_regular_unlinked_file
    checks = 0

    def create_hardlink_after_second_check(fd: int) -> os.stat_result:
        nonlocal checks
        file_stat = original_assert(fd)
        checks += 1
        if checks == 2:
            os.link(app, outside_link)
        return file_stat

    monkeypatch.setattr(apply_module, "_assert_regular_unlinked_file", create_hardlink_after_second_check)

    apply_patch_bundle(bundle, target)

    assert app.read_text(encoding="utf-8") == "new\n"
    assert outside_link.read_text(encoding="utf-8") == "old\n"


def test_apply_preserves_existing_file_mode_when_replacing(tmp_path):
    target = _target(tmp_path)
    private = target / "private.conf"
    script = target / "tool.sh"
    private.write_text("old secret\n", encoding="utf-8")
    script.write_text("#!/bin/sh\nexit 0\n", encoding="utf-8")
    private.chmod(0o600)
    script.chmod(0o755)
    bundle = PatchBundle(
        base_tree_digest=tree_digest(target),
        operations=[
            PatchOperation(path="private.conf", content="new secret\n"),
            PatchOperation(path="tool.sh", content="#!/bin/sh\nexit 1\n"),
        ],
    )

    apply_patch_bundle(bundle, target)

    assert stat_mode(private) == 0o600
    assert stat_mode(script) == 0o755


def test_apply_removes_partial_new_file_when_write_fails(tmp_path, monkeypatch):
    target = _target(tmp_path)
    bundle = PatchBundle(
        base_tree_digest=tree_digest(target),
        operations=[PatchOperation(path="new.txt", content="new content\n")],
    )
    real_fdopen = apply_module.os.fdopen

    class FailingWriteStream:
        def __init__(self, stream):
            self.stream = stream

        def __enter__(self):
            return self

        def __exit__(self, exc_type, exc, traceback):
            self.stream.close()
            return False

        def fileno(self) -> int:
            return self.stream.fileno()

        def write(self, payload: bytes) -> int:
            self.stream.write(payload[:1])
            raise OSError("simulated disk full")

        def __getattr__(self, name: str):
            return getattr(self.stream, name)

    def failing_fdopen(fd: int, mode: str = "r", *args, **kwargs):
        stream = real_fdopen(fd, mode, *args, **kwargs)
        if "w" in mode and "b" in mode:
            return FailingWriteStream(stream)
        return stream

    monkeypatch.setattr(apply_module.os, "fdopen", failing_fdopen)

    with pytest.raises(OSError, match="simulated disk full"):
        apply_patch_bundle(bundle, target)

    assert not (target / "new.txt").exists()
    assert not list(target.glob(".rail-tmp-*"))


def test_apply_creates_executable_file_when_policy_allows(tmp_path):
    target = _target(tmp_path)
    bundle = PatchBundle(
        base_tree_digest=tree_digest(target),
        operations=[PatchOperation(path="tool.sh", content="#!/bin/sh\nexit 0\n", executable=True)],
    )

    apply_patch_bundle(bundle, target, policy=PatchValidationPolicy(allow_executable=True))

    assert stat_mode(target / "tool.sh") == 0o755


def test_apply_removes_created_parent_directories_on_later_failure(tmp_path, monkeypatch):
    target = _target(tmp_path)
    bundle = PatchBundle(
        base_tree_digest=tree_digest(target),
        operations=[
            PatchOperation(path="newdir/file.txt", content="created\n"),
            PatchOperation(path="other.txt", content="fail\n"),
        ],
    )
    real_write = apply_module._write_text_without_following_links
    calls = 0

    def fail_second_write(*args, **kwargs):
        nonlocal calls
        calls += 1
        if calls == 2:
            raise OSError("simulated second write failure")
        return real_write(*args, **kwargs)

    monkeypatch.setattr(apply_module, "_write_text_without_following_links", fail_second_write)

    with pytest.raises(OSError, match="simulated second write failure"):
        apply_patch_bundle(bundle, target)

    assert not (target / "newdir").exists()


def stat_mode(path: Path) -> int:
    return path.stat().st_mode & 0o777


def _target(tmp_path: Path) -> Path:
    target = tmp_path / "target"
    target.mkdir()
    return target
