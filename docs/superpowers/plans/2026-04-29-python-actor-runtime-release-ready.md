# Python Actor Runtime Release-Ready Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bring the Python Actor Runtime Rail harness from redesign-complete to release-ready for real skill-first use.

**Architecture:** Keep Rail as the governance harness and keep the OpenAI Agents SDK inside the Actor Runtime boundary. Add release-hardening around live runtime readiness, persisted artifact handles, real validation evidence, terminal reporting, test-only runtime isolation, and the active docs/release gate.

**Tech Stack:** Python 3.12, OpenAI Agents SDK, Pydantic, PyYAML, pytest, ruff, mypy.

---

## File Structure

- `src/rail/artifacts/handle.py`: persisted handle serialization and loading.
- `src/rail/artifacts/store.py`: handle binding and artifact identity checks.
- `src/rail/api.py`: public `load_handle` API and handle-based status/result flow.
- `src/rail/actor_runtime/agents.py`: live SDK runner boundary and runtime error mapping.
- `src/rail/auth/credentials.py`: readiness inputs for the SDK runner.
- `src/rail/cli/doctor.py`: secret-safe readiness report.
- `src/rail/workspace/validation_runner.py`: request/policy-owned validation command execution.
- `src/rail/workspace/validation.py`: validation evidence model with duration and command ownership.
- `src/rail/artifacts/terminal_summary.py`: terminal summary projection.
- `src/rail/artifacts/projection.py`: user-facing blocked/pass/reject result mapping.
- `tests/runtime_helpers.py`: test-only fake runtime helpers outside the production package.
- `skills/rail/SKILL.md`: handle-based release-ready workflow.
- `assets/skill/Rail/SKILL.md`: bundled skill copy.
- `docs/tasks.md`: active Python release-ready checklist.
- `tests/`: focused coverage for each boundary above.

## Task 1: Persist And Load Artifact Handles

**Files:**
- Create: `src/rail/artifacts/handle.py`
- Modify: `src/rail/artifacts/models.py`
- Modify: `src/rail/artifacts/store.py`
- Modify: `src/rail/artifacts/__init__.py`
- Modify: `src/rail/api.py`
- Test: `tests/artifacts/test_handle_resolution.py`

- [x] **Step 1: Write failing handle persistence tests**

Add `tests/artifacts/test_handle_resolution.py`:

```python
from __future__ import annotations

import pytest

import rail
from rail.artifacts.handle import load_handle_file
from rail.policy import load_effective_policy
from rail.artifacts.store import bind_effective_policy


def test_start_task_persists_reloadable_handle(tmp_path):
    target = tmp_path / "target"
    target.mkdir()
    handle = rail.start_task(
        {
            "project_root": str(target),
            "task_type": "bug_fix",
            "goal": "Persist handle.",
            "definition_of_done": ["Handle reloads."],
        }
    )
    bound = bind_effective_policy(handle, load_effective_policy(target))

    reloaded = load_handle_file(bound.artifact_dir / "handle.yaml")

    assert reloaded.artifact_id == bound.artifact_id
    assert reloaded.artifact_dir == bound.artifact_dir
    assert reloaded.project_root == bound.project_root
    assert reloaded.request_snapshot_digest == bound.request_snapshot_digest
    assert reloaded.effective_policy_digest == bound.effective_policy_digest


def test_load_handle_rejects_forged_artifact_id(tmp_path):
    target = tmp_path / "target"
    target.mkdir()
    handle = rail.start_task(
        {
            "project_root": str(target),
            "task_type": "bug_fix",
            "goal": "Reject forged handle.",
            "definition_of_done": ["Forgery is rejected."],
        }
    )
    path = handle.artifact_dir / "handle.yaml"
    text = path.read_text(encoding="utf-8")
    path.write_text(text.replace(handle.artifact_id, "rail-forged", 1), encoding="utf-8")

    with pytest.raises(ValueError, match="artifact_id"):
        rail.load_handle(path)
```

- [x] **Step 2: Run test to verify it fails**

Run:

```bash
uv run --python 3.12 pytest tests/artifacts/test_handle_resolution.py -q
```

Expected: fail because `rail.artifacts.handle` and `rail.load_handle` do not exist.

- [x] **Step 3: Implement handle persistence**

Create `src/rail/artifacts/handle.py`:

```python
from __future__ import annotations

from pathlib import Path

import yaml

from rail.artifacts.models import ArtifactHandle
from rail.artifacts.store import validate_artifact_handle


def write_handle_file(handle: ArtifactHandle) -> Path:
    path = handle.artifact_dir / "handle.yaml"
    path.write_text(yaml.safe_dump(handle.model_dump(mode="json"), sort_keys=True), encoding="utf-8")
    return path


def load_handle_file(path: str | Path) -> ArtifactHandle:
    handle_path = Path(path)
    if handle_path.is_symlink():
        raise ValueError("handle path must not be a symlink")
    payload = yaml.safe_load(handle_path.read_text(encoding="utf-8"))
    handle = ArtifactHandle.model_validate(payload)
    return validate_artifact_handle(handle)
```

Update `ArtifactStore.allocate` and `bind_effective_policy` to call `write_handle_file`.
Expose `load_handle` from `src/rail/api.py` and `src/rail/__init__.py`.

- [x] **Step 4: Verify**

Run:

```bash
uv run --python 3.12 pytest tests/artifacts/test_handle_resolution.py tests/artifacts/test_store.py -q
```

Expected: pass.

- [x] **Step 5: Commit**

```bash
git add src/rail/artifacts src/rail/api.py src/rail/__init__.py tests/artifacts
git commit -m "feat: persist artifact handles"
```

## Task 2: Add Live Agents SDK Runner Readiness

**Files:**
- Modify: `src/rail/actor_runtime/agents.py`
- Modify: `src/rail/auth/credentials.py`
- Modify: `src/rail/cli/doctor.py`
- Test: `tests/actor_runtime/test_agents_runtime.py`
- Test: `tests/auth/test_credentials.py`

- [x] **Step 1: Write failing readiness and runner tests**

Add tests asserting:

```python
def test_default_runner_requires_ready_credentials(tmp_path):
    from rail.actor_runtime.agents import AgentsActorRuntime
    from rail.policy import load_effective_policy

    runtime = AgentsActorRuntime(project_root=tmp_path, policy=load_effective_policy(tmp_path))

    assert runtime.readiness().ready is False
    assert "credential" in runtime.readiness().reason
```

Also add a runner injection test that proves configured runners still bypass live network for deterministic tests.

- [x] **Step 2: Run tests to verify failure**

Run:

```bash
uv run --python 3.12 pytest tests/actor_runtime/test_agents_runtime.py tests/auth/test_credentials.py -q
```

Expected: fail until runtime readiness exists.

- [x] **Step 3: Implement readiness model**

Add a Pydantic readiness model in `src/rail/actor_runtime/agents.py`:

```python
class RuntimeReadiness(BaseModel):
    model_config = ConfigDict(extra="forbid")

    ready: bool
    reason: str
    credential_source: str | None = None
```

Implement `AgentsActorRuntime.readiness()` so it returns `ready=False` when no explicit runner is injected and no approved SDK credential source is configured.

- [x] **Step 4: Implement live runner boundary**

Keep the live runner behind a small function that can be replaced in tests:

```python
def run_agent_live(agent: Agent[Any], prompt: str, *, run_config: dict[str, object]) -> SDKRunResult:
    from agents import Runner

    result = Runner.run_sync(agent, prompt, **run_config)
    return SDKRunResult(final_output=result.final_output, trace_id=str(result.trace_id))
```

If the actual SDK return shape differs, adapt inside this function only and update the test to assert Rail's `SDKRunResult` contract.

- [x] **Step 5: Verify**

Run:

```bash
uv run --python 3.12 pytest tests/actor_runtime/test_agents_runtime.py tests/auth/test_credentials.py -q
uv run --python 3.12 ruff check src tests
```

Expected: pass.

- [x] **Step 6: Commit**

```bash
git add src/rail/actor_runtime/agents.py src/rail/auth src/rail/cli tests/actor_runtime tests/auth
git commit -m "feat: add actor runtime readiness"
```

## Task 3: Replace Synthetic Validation With Rail-Owned Validation Runner

**Files:**
- Create: `src/rail/workspace/validation_runner.py`
- Modify: `src/rail/workspace/validation.py`
- Modify: `src/rail/supervisor/supervise.py`
- Test: `tests/workspace/test_validation_runner.py`
- Test: `tests/evaluator/test_gate.py`

- [x] **Step 1: Write failing validation runner tests**

Create `tests/workspace/test_validation_runner.py`:

```python
from __future__ import annotations

from rail.workspace.validation_runner import ValidationCommand, run_validation_command


def test_validation_runner_records_pass_with_redacted_logs(tmp_path):
    artifact = tmp_path / "artifact"
    artifact.mkdir()
    target = tmp_path / "target"
    target.mkdir()

    evidence = run_validation_command(
        artifact,
        target_root=target,
        command=ValidationCommand(argv=["python", "-c", "print('sk-test-secret')"], source="policy"),
        patch_digest="sha256:patch",
        request_digest="sha256:request",
        effective_policy_digest="sha256:policy",
        actor_invocation_digest="sha256:actor",
    )

    assert evidence.status == "pass"
    assert "sk-test-secret" not in (artifact / evidence.stdout_ref).read_text(encoding="utf-8")
```

- [x] **Step 2: Run test to verify failure**

Run:

```bash
uv run --python 3.12 pytest tests/workspace/test_validation_runner.py -q
```

Expected: fail because `validation_runner.py` does not exist.

- [x] **Step 3: Implement validation runner**

Create `ValidationCommand` and `run_validation_command`. Use `subprocess.run` with:

- `cwd=target_root`
- timeout from policy or default 30 seconds
- captured stdout/stderr
- no shell string execution
- command source restricted to `request` or `policy`

Record pre/post tree digests and set `mutation_status="mutated"` when they differ.

- [x] **Step 4: Wire supervisor executor phase**

Replace the synthetic `record_validation_evidence(..., command="policy:validation", exit_code=0)` call in `src/rail/supervisor/supervise.py` with the validation runner. Validation must come from request or policy commands such as `.harness/supervisor/execution_policy.yaml`; missing commands block terminal pass.

- [x] **Step 5: Verify**

Run:

```bash
uv run --python 3.12 pytest tests/workspace/test_validation_runner.py tests/evaluator/test_gate.py tests/supervisor/test_routing.py -q
```

Expected: pass.

- [x] **Step 6: Commit**

```bash
git add src/rail/workspace src/rail/supervisor tests/workspace tests/evaluator tests/supervisor
git commit -m "feat: run rail-owned validation evidence"
```

## Task 4: Add Terminal Summary Projection

**Files:**
- Create: `src/rail/artifacts/terminal_summary.py`
- Modify: `src/rail/artifacts/projection.py`
- Modify: `src/rail/supervisor/supervise.py`
- Test: `tests/artifacts/test_terminal_summary.py`

- [x] **Step 1: Write failing terminal summary tests**

Create `tests/artifacts/test_terminal_summary.py`:

```python
from __future__ import annotations

import rail
from rail.artifacts.terminal_summary import project_terminal_summary


def test_terminal_summary_explains_blocked_runtime(tmp_path):
    target = tmp_path / "target"
    target.mkdir()
    handle = rail.start_task(
        {
            "project_root": str(target),
            "task_type": "bug_fix",
            "goal": "Explain blocked state.",
            "definition_of_done": ["Summary explains the state."],
        }
    )
    rail.supervise(handle)

    summary = project_terminal_summary(handle)

    assert summary.outcome == "blocked"
    assert "runtime" in summary.reason.lower() or "actor" in summary.reason.lower()
    assert summary.next_step
```

- [x] **Step 2: Run test to verify failure**

Run:

```bash
uv run --python 3.12 pytest tests/artifacts/test_terminal_summary.py -q
```

Expected: fail because terminal summary projection does not exist.

- [x] **Step 3: Implement terminal summary model**

Create `TerminalSummaryProjection` with:

- `artifact_id`
- `outcome`
- `reason`
- `blocked_category`
- `evidence_refs`
- `next_step`

Write `terminal_summary.yaml` at supervisor completion.

- [x] **Step 4: Verify**

Run:

```bash
uv run --python 3.12 pytest tests/artifacts/test_terminal_summary.py tests/artifacts/test_projection.py -q
```

Expected: pass.

- [x] **Step 5: Commit**

```bash
git add src/rail/artifacts src/rail/supervisor tests/artifacts
git commit -m "feat: add terminal summary projection"
```

## Task 5: Isolate Test-Only Fake Runtime

**Files:**
- Modify: `tests/runtime_helpers.py`
- Modify: `src/rail/actor_runtime/runtime.py`
- Modify: `tests/actor_runtime/test_fake_runtime.py`
- Modify: `tests/supervisor/test_routing.py`
- Test: `tests/test_no_legacy_runtime_calls.py`

- [x] **Step 1: Write failing fake-runtime isolation test**

Add to `tests/actor_runtime/test_fake_runtime.py`:

```python
def test_fake_runtime_is_not_exported_from_production_runtime():
    import rail.actor_runtime.runtime as runtime

    assert not hasattr(runtime, "FakeActorRuntime")
```

- [x] **Step 2: Run test to verify failure**

Run:

```bash
uv run --python 3.12 pytest tests/actor_runtime/test_fake_runtime.py -q
```

Expected: fail because `FakeActorRuntime` still lives in production runtime.

- [x] **Step 3: Move fake runtime**

Move `FakeActorRuntime` outside the production package into test helpers. Update tests to import from `tests.runtime_helpers`. Keep `ActorRuntime`, `ActorInvocation`, and `ActorResult` in `runtime.py`.

- [x] **Step 4: Verify**

Run:

```bash
uv run --python 3.12 pytest tests/actor_runtime/test_fake_runtime.py tests/supervisor/test_routing.py tests/test_no_legacy_runtime_calls.py -q
```

Expected: pass.

- [x] **Step 5: Commit**

```bash
git add src/rail/actor_runtime tests/actor_runtime tests/supervisor tests/test_no_legacy_runtime_calls.py
git commit -m "refactor: isolate fake actor runtime"
```

## Task 6: Align Skill And Active Release Checklist

**Files:**
- Modify: `skills/rail/SKILL.md`
- Modify: `assets/skill/Rail/SKILL.md`
- Modify: `docs/tasks.md`
- Test: `tests/docs/test_no_home_paths.py`
- Test: `tests/docs/test_removed_runtime_surfaces.py`

- [x] **Step 1: Write failing docs assertion**

Extend `tests/docs/test_no_home_paths.py` to assert that `docs/tasks.md` references:

- `docs/superpowers/specs/2026-04-29-python-actor-runtime-release-ready.md`
- `docs/superpowers/plans/2026-04-29-python-actor-runtime-release-ready.md`

- [x] **Step 2: Run test to verify failure**

Run:

```bash
uv run --python 3.12 pytest tests/docs/test_no_home_paths.py -q
```

Expected: fail until `docs/tasks.md` is updated.

- [x] **Step 3: Update skill workflow**

Make both skill copies describe:

- fresh task uses `rail.start_task(draft)`
- existing work uses a persisted artifact handle
- readiness failure stops before actor execution
- result reporting uses `rail.result(handle)` and terminal summary

Keep the two skill files byte-for-byte aligned.

- [x] **Step 4: Update active release checklist**

Rewrite `docs/tasks.md` so the active release-ready boundary points to this Python spec and plan. Mark the redesign baseline as complete and leave live runtime, persisted resume, validation runner, terminal summary, fake runtime isolation, and release gate hardening unchecked until implemented.

- [x] **Step 5: Verify**

Run:

```bash
uv run --python 3.12 pytest tests/docs/test_no_home_paths.py tests/docs/test_removed_runtime_surfaces.py -q
cmp -s skills/rail/SKILL.md assets/skill/Rail/SKILL.md
```

Expected: pytest passes and `cmp` exits 0.

- [x] **Step 6: Commit**

```bash
git add skills/rail assets/skill/Rail docs/tasks.md tests/docs
git commit -m "docs: align release-ready workflow"
```

## Task 7: Define Release Gate Command

**Files:**
- Create: `scripts/python_release_gate.sh`
- Modify: `README.md`
- Modify: `docs/ARCHITECTURE.md`
- Test: `tests/docs/test_removed_runtime_surfaces.py`

- [x] **Step 1: Write failing release gate test**

Add a docs test that asserts `scripts/python_release_gate.sh` exists and does not contain old runtime commands.

- [x] **Step 2: Run test to verify failure**

Run:

```bash
uv run --python 3.12 pytest tests/docs/test_removed_runtime_surfaces.py -q
```

Expected: fail until the release gate script exists.

- [x] **Step 3: Add release gate script**

Create `scripts/python_release_gate.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

uv run --python 3.12 pytest -q
uv run --python 3.12 ruff check src tests
uv run --python 3.12 mypy src/rail
```

- [x] **Step 4: Document the gate**

Update active docs to point release operators at `scripts/python_release_gate.sh` as the local release gate. Do not describe this as a broad downstream task success proof; it is the Rail control-plane release gate.

- [x] **Step 5: Verify**

Run:

```bash
scripts/python_release_gate.sh
```

Expected: pass.

- [x] **Step 6: Commit**

```bash
git add scripts/python_release_gate.sh README.md docs/ARCHITECTURE.md tests/docs
git commit -m "build: add python release gate"
```
