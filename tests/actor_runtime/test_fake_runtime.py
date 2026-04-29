from __future__ import annotations

from pathlib import Path

import rail
from rail.actor_runtime.runtime import ActorInvocation, FakeActorRuntime


def test_fake_actor_runtime_returns_contract_result(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    runtime = FakeActorRuntime()
    invocation = ActorInvocation(
        actor="generator",
        artifact_id=handle.artifact_id,
        artifact_dir=handle.artifact_dir,
        prompt="Generate a patch bundle.",
        input={"goal": "test"},
        policy_digest="sha256:test",
    )

    result = runtime.run(invocation)

    assert result.status == "succeeded"
    assert result.structured_output["changed_files"]
    assert result.events_ref == Path("runs/generator.events.jsonl")
    assert result.runtime_evidence_ref == Path("runs/generator.runtime_evidence.json")
    assert result.patch_bundle_ref == Path("patches/generator.patch.yaml")
    assert (handle.artifact_dir / result.events_ref).is_file()
    assert (handle.artifact_dir / result.runtime_evidence_ref).is_file()


def test_supervisor_invokes_fake_runtime_and_persists_actor_evidence(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))

    state = rail.supervise(handle, runtime=FakeActorRuntime())

    assert state.outcome == "pass"
    for actor in ("planner", "context_builder", "critic", "generator", "executor", "evaluator"):
        assert (handle.artifact_dir / "runs" / f"{actor}.events.jsonl").is_file()
        assert (handle.artifact_dir / "runs" / f"{actor}.runtime_evidence.json").is_file()


def _target_repo(tmp_path: Path) -> Path:
    target = tmp_path / "target-repo"
    target.mkdir(parents=True, exist_ok=True)
    return target


def _draft(target: Path) -> dict[str, object]:
    return {
        "project_root": str(target),
        "task_type": "bug_fix",
        "goal": "Exercise fake runtime.",
        "definition_of_done": ["Actor evidence is written."],
    }
