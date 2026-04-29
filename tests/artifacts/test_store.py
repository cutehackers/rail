from __future__ import annotations

from concurrent.futures import ThreadPoolExecutor
from pathlib import Path

import pytest
import yaml

import rail
from rail.policy import load_effective_policy


def test_start_task_allocates_artifact_handle_and_layout(tmp_path):
    target = _target_repo(tmp_path)

    handle = rail.start_task(_draft(target))

    assert handle.schema_version == "1"
    assert handle.artifact_id
    assert handle.artifact_dir.is_absolute()
    assert handle.project_root == target.resolve()
    assert handle.request_snapshot_digest.startswith("sha256:")
    assert (handle.artifact_dir / "request.yaml").is_file()
    assert (handle.artifact_dir / "state.yaml").is_file()
    assert (handle.artifact_dir / "workflow.yaml").is_file()
    assert (handle.artifact_dir / "run_status.yaml").is_file()
    assert (handle.artifact_dir / "runs").is_dir()


def test_concurrent_fresh_allocations_produce_distinct_handles(tmp_path):
    target = _target_repo(tmp_path)

    with ThreadPoolExecutor(max_workers=8) as executor:
        handles = list(executor.map(lambda _: rail.start_task(_draft(target)), range(8)))

    assert len({handle.artifact_id for handle in handles}) == 8
    assert len({handle.artifact_dir for handle in handles}) == 8


def test_validate_handle_rejects_missing_request_snapshot(tmp_path):
    from rail.artifacts.store import validate_artifact_handle

    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    (handle.artifact_dir / "request.yaml").unlink()

    with pytest.raises(ValueError, match="request snapshot"):
        validate_artifact_handle(handle)


def test_validate_handle_rejects_digest_mismatch(tmp_path):
    from rail.artifacts.store import validate_artifact_handle

    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    (handle.artifact_dir / "request.yaml").write_text("goal: tampered\n", encoding="utf-8")

    with pytest.raises(ValueError, match="digest"):
        validate_artifact_handle(handle)


def test_validate_handle_rejects_symlinked_artifact_dir(tmp_path):
    from rail.artifacts.store import validate_artifact_handle

    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    symlink_path = handle.artifact_dir.parent / "linked-artifact"
    symlink_path.symlink_to(handle.artifact_dir, target_is_directory=True)
    forged = handle.model_copy(update={"artifact_dir": symlink_path})

    with pytest.raises(ValueError, match="symlink"):
        validate_artifact_handle(forged)


def test_validate_handle_rejects_mismatched_project_root(tmp_path):
    from rail.artifacts.store import validate_artifact_handle

    handle = rail.start_task(_draft(_target_repo(tmp_path / "a")))
    other_target = _target_repo(tmp_path / "b")
    forged = handle.model_copy(update={"project_root": other_target})

    with pytest.raises(ValueError, match="project_root"):
        validate_artifact_handle(forged)


def test_validate_handle_rejects_artifact_dir_outside_project_store(tmp_path):
    from rail.artifacts.store import validate_artifact_handle

    target = _target_repo(tmp_path)
    handle = rail.start_task(_draft(target))
    outside = target / ".harness" / "outside"
    outside.mkdir(parents=True)
    forged = handle.model_copy(update={"artifact_dir": handle.artifact_dir.parent / ".." / "outside"})

    with pytest.raises(ValueError, match="artifact_dir"):
        validate_artifact_handle(forged)


def test_validate_handle_rejects_tampered_artifact_identity_files(tmp_path):
    from rail.artifacts.store import validate_artifact_handle

    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    state_path = handle.artifact_dir / "state.yaml"
    state = yaml.safe_load(state_path.read_text(encoding="utf-8"))
    state["artifact_id"] = "rail-forged"
    state_path.write_text(yaml.safe_dump(state), encoding="utf-8")

    with pytest.raises(ValueError, match="artifact_id"):
        validate_artifact_handle(handle)


def test_validate_handle_binds_effective_policy_digest_to_persisted_policy(tmp_path):
    from rail.artifacts.store import bind_effective_policy, validate_artifact_handle
    from rail.policy import digest_policy

    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    policy = load_effective_policy(handle.project_root)

    bound = bind_effective_policy(handle, policy)

    assert bound.effective_policy_digest == digest_policy(policy)
    assert validate_artifact_handle(bound).effective_policy_digest == bound.effective_policy_digest
    forged = bound.model_copy(update={"effective_policy_digest": "sha256:forged"})
    with pytest.raises(ValueError, match="policy"):
        validate_artifact_handle(forged)


def test_validate_handle_rejects_policy_digest_without_persisted_policy(tmp_path):
    from rail.artifacts.store import validate_artifact_handle

    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    forged = handle.model_copy(update={"effective_policy_digest": "sha256:missing"})

    with pytest.raises(ValueError, match="policy"):
        validate_artifact_handle(forged)


def _target_repo(tmp_path: Path) -> Path:
    target = tmp_path / "target-repo"
    target.mkdir(parents=True, exist_ok=True)
    return target


def _draft(target: Path) -> dict[str, object]:
    return {
        "project_root": str(target),
        "task_type": "bug_fix",
        "goal": "Fix the failing harness behavior.",
        "constraints": ["Keep the workflow bounded."],
        "definition_of_done": ["The artifact handle is valid."],
    }
