# Release-Ready Gap Closure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the remaining `docs/SPEC.md` release-ready gaps that remain after the package distribution audit.

**Status:** Implemented and verified with `scripts/python_release_gate.sh`; optional live SDK smoke remains operator-gated.

**Final Review Follow-up:** Post-implementation P2 findings were fixed by
persisting the actual actor that blocked a workflow and by bounding live
credential preflight with `policy.runtime.timeout_seconds`.

**Architecture:** Keep the Rail product boundary Python API first and skill first. Runtime readiness must block before actor work, terminal projections must expose blocked categories, evaluator pass must be bound to supervisor-provided evidence, and release docs must point to the actual gate script.

**Tech Stack:** Python 3.12, Pydantic contracts, pytest, ruff, mypy, `scripts/python_release_gate.sh`.

---

### Task 1: Runtime Readiness Blocks Invalid Credentials Before Actor Work

**Files:**
- Modify: `src/rail/actor_runtime/agents.py`
- Test: `tests/actor_runtime/test_agents_runtime.py`

- [x] **Step 1: Write the failing test**
  Add a test that configures live runtime with an operator credential value that is syntactically invalid and a runner that fails if called. Assert `AgentsActorRuntime.run(...)` returns `status == "interrupted"`, writes runtime evidence, redacts the credential, and does not call the runner.

- [x] **Step 2: Run test to verify it fails**
  Run: `uv run --python 3.12 pytest tests/actor_runtime/test_agents_runtime.py::test_live_runner_blocks_invalid_operator_credential_before_actor_work -q`
  Expected: FAIL because readiness currently accepts any non-empty operator credential.

- [x] **Step 3: Write minimal implementation**
  Extend readiness to validate operator credential shape before actor execution. Keep the report secret-safe and record only credential source category.

- [x] **Step 4: Run focused tests**
  Run: `uv run --python 3.12 pytest tests/actor_runtime/test_agents_runtime.py -q`
  Expected: PASS.

### Task 2: Result Projection Distinguishes Blocked Categories

**Files:**
- Modify: `src/rail/artifacts/projection.py`
- Test: `tests/artifacts/test_projection.py`

- [x] **Step 1: Write the failing tests**
  Add tests for `rail.result(handle)` on blocked runtime, validation, policy, and environment terminal summaries. Assert the projection exposes `blocked_category`, a category-specific `outcome_label`, and terminal reason/next step without requiring raw artifact inspection.

- [x] **Step 2: Run tests to verify failure**
  Run: `uv run --python 3.12 pytest tests/artifacts/test_projection.py -q`
  Expected: FAIL because `ResultProjection` does not expose blocked category or reason.

- [x] **Step 3: Write minimal implementation**
  Add explicit result projection fields that mirror terminal summary blocked category, reason, and category-specific outcome label while preserving existing fields.

- [x] **Step 4: Run focused tests**
  Run: `uv run --python 3.12 pytest tests/artifacts/test_projection.py -q`
  Expected: PASS.

### Task 3: Environment-Blocked Outcome Is Reachable And Tested

**Files:**
- Modify: `src/rail/supervisor/supervise.py`
- Test: `tests/supervisor/test_routing.py`

- [x] **Step 1: Write the failing test**
  Add a default-supervise test where live runtime is not enabled or credentials are missing. Assert the terminal artifact is `outcome == "blocked"` with `blocked_category == "environment"`, and `rail.result(handle)` preserves that category.

- [x] **Step 2: Run test to verify failure**
  Run: `uv run --python 3.12 pytest tests/supervisor/test_routing.py::test_default_supervise_blocks_environment_when_runtime_not_ready -q`
  Expected: FAIL because runtime readiness currently maps to runtime-blocked.

- [x] **Step 3: Write minimal implementation**
  Classify Actor Runtime readiness failures as environment-blocked while keeping actor execution exceptions and actor interruptions as runtime-blocked.

- [x] **Step 4: Run focused tests**
  Run: `uv run --python 3.12 pytest tests/supervisor/test_routing.py tests/artifacts/test_terminal_summary.py -q`
  Expected: PASS.

### Task 4: Evaluator And Validation Digest Binding Is Not Tautological

**Files:**
- Modify: `src/rail/supervisor/supervise.py`
- Modify: `src/rail/evaluator/gate.py` if the gate input contract needs a clearer name
- Test: `tests/supervisor/test_routing.py`
- Test: `tests/evaluator/test_gate.py`

- [x] **Step 1: Write the failing test**
  Add tests that tamper with evaluator actor output and validation evidence digest so mismatched supervisor-bound evaluator or validation digests are blocked, not accepted.

- [x] **Step 2: Run test to verify failure**
  Run: `uv run --python 3.12 pytest tests/supervisor/test_routing.py::test_supervise_blocks_evaluator_output_digest_mismatch -q`
  Expected: FAIL because the supervisor currently passes `digest_payload(result.structured_output)` to the gate and does not bind validation evidence by digest.

- [x] **Step 3: Write minimal implementation**
  Compute a supervisor-owned evaluator input digest before the evaluator actor call from the evaluator invocation payload, include the validation evidence digest in that input, and require evaluator output to carry the same evaluator input digest before any evaluator decision is accepted.

- [x] **Step 4: Run focused tests**
  Run: `uv run --python 3.12 pytest tests/evaluator/test_gate.py tests/supervisor/test_routing.py -q`
  Expected: PASS.

### Task 5: Optional Live SDK Smoke Scope Matches SPEC

**Files:**
- Modify: `tests/e2e/test_optional_live_sdk_smoke.py`
- Maybe modify: `.harness/` examples only if needed for a full supervise smoke
- Docs: `docs/SPEC.md`

- [x] **Step 1: Decide scope from SPEC**
  If the live smoke is intended to prove only SDK adapter readiness, update `docs/SPEC.md` language to say so. If it must prove the full supervisor path, update the smoke to exercise `rail.supervise(handle)` with real runtime and a minimal target validation policy.

- [x] **Step 2: Add or update tests/docs**
  Keep live smoke skipped by default and enabled only by `RAIL_ACTOR_RUNTIME_LIVE_SMOKE=1` plus operator credentials.

### Task 6: Canonicalize Release Docs And Checklist

**Files:**
- Modify: `docs/SPEC.md`
- Modify: `docs/tasks.md`
- Modify: `docs/superpowers/plans/2026-04-29-release-ready-audit-closure.md`
- Maybe modify: `README.md` only if release gate wording diverges

- [x] **Step 1: Update SPEC release gate**
  Make `scripts/python_release_gate.sh` the canonical mandatory local release gate and list its build, package asset, installed-wheel smoke, pytest, docs/no-legacy guards, deterministic smoke, ruff, and mypy checks.

- [x] **Step 2: Update task checklist**
  Replace stale distribution-boundary status with the actual remaining release blockers or mark the release-ready gap closed after implementation and verification.

- [x] **Step 3: Run docs guards**
  Run the focused docs tests, then the full release gate.

### Task 7: Final Critical Review And Release Gate

**Files:**
- No planned production edits unless review finds a blocker.

- [x] **Step 1: Run full gate**
  Run: `scripts/python_release_gate.sh`
  Expected: PASS.

- [x] **Step 2: Dispatch critical review subagents**
  Ask one reviewer to compare `docs/SPEC.md` with implementation/tests and another to review code quality and release gate canonicalization.

- [x] **Step 3: Fix P1/P2 findings**
  Fix any valid release blockers, rerun focused tests, then rerun the full gate.
