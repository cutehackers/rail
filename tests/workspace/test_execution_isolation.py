from __future__ import annotations

from pathlib import Path

import pytest

from rail.workspace.isolation import (
    assert_target_unchanged,
    deny_target_path_input,
    scrub_actor_environment,
    tree_digest,
)
from rail.workspace.sandbox import create_sandbox, write_sandbox_file


def test_absolute_target_paths_are_denied_in_tool_inputs(tmp_path):
    target = _target(tmp_path)

    with pytest.raises(ValueError, match="target path"):
        deny_target_path_input(str(target / "app.txt"), target)


def test_environment_is_scrubbed_for_actor_tools():
    env = scrub_actor_environment({"OPENAI_API_KEY": "secret", "PATH": "/bin", "HOME": "/home/user", "RAIL_ARTIFACT": "ok"})

    assert env == {"OPENAI_API_KEY": "secret", "RAIL_ARTIFACT": "ok"}


def test_sandbox_write_rejects_symlink_escape(tmp_path):
    target = _target(tmp_path)
    outside = tmp_path / "outside.txt"
    outside.write_text("outside", encoding="utf-8")
    sandbox = create_sandbox(target)
    (sandbox.sandbox_root / "link.txt").symlink_to(outside)

    with pytest.raises(ValueError, match="symlink"):
        write_sandbox_file(sandbox, "link.txt", "escape")


def test_sandbox_creation_rejects_target_symlink_to_host_file(tmp_path):
    target = _target(tmp_path)
    outside = tmp_path / "outside-secret.txt"
    outside.write_text("secret", encoding="utf-8")
    (target / "linked-secret.txt").symlink_to(outside)

    with pytest.raises(ValueError, match="symlink"):
        create_sandbox(target)


def test_sandbox_creation_ignores_harness_entries_during_link_scan(tmp_path):
    target = _target(tmp_path)
    outside = tmp_path / "outside-generated-file"
    outside.write_text("generated", encoding="utf-8")
    harness_file = target / ".harness" / "artifacts" / "applypatch"
    harness_file.parent.mkdir(parents=True)
    try:
        harness_file.hardlink_to(outside)
    except OSError:
        pytest.skip("hardlinks are not supported on this filesystem")

    sandbox = create_sandbox(target)

    assert not (sandbox.sandbox_root / ".harness").exists()


def test_pre_post_tree_digest_proves_actor_did_not_mutate_target(tmp_path):
    target = _target(tmp_path)
    (target / "app.txt").write_text("old\n", encoding="utf-8")
    before = tree_digest(target)
    sandbox = create_sandbox(target)
    write_sandbox_file(sandbox, "app.txt", "new\n")
    after = tree_digest(target)

    assert_target_unchanged(before, after)


def _target(tmp_path: Path) -> Path:
    target = tmp_path / "target"
    target.mkdir()
    return target
