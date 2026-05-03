# Downstream Actor Canonical Seeds Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add canonical seeded live smoke support for `critic`, `generator`, `executor`, and `evaluator`.

**Architecture:** Keep `LiveSmokeRunner` as the only execution surface. Add typed seed contracts in `rail.live_smoke`, pass seed metadata through `invocation.input["live_smoke_seed"]`, keep synthetic upstream outputs in `prior_outputs`, and extend behavior smoke to receive seed and invocation context.

**Tech Stack:** Python 3.12, Pydantic, pytest, existing Rail Actor Runtime, `codex_vault` live smoke.

---

### Task 1: Fix Fixture Digest Binding

**Files:**
- Modify: `src/rail/workspace/isolation.py`
- Test: `tests/live_smoke/test_fixtures.py`

- [ ] Add a regression test that copies the fixture under `.harness/live-smoke` and asserts the digest is not the empty SHA-256 digest.
- [ ] Change `tree_digest()` so `.git` and `.harness` are ignored relative to the supplied root.
- [ ] Run `uv run --python 3.12 pytest tests/live_smoke/test_fixtures.py -q`.
- [ ] Commit with `fix(live-smoke): bind fixture digest under harness paths`.

### Task 2: Add Typed Seed Contracts

**Files:**
- Create: `src/rail/live_smoke/seeds.py`
- Modify: `src/rail/live_smoke/models.py`
- Test: `tests/live_smoke/test_seeds.py`
- Test: `tests/live_smoke/test_contracts.py`

- [ ] Add `LiveSmokeActor` values for `critic`, `generator`, `executor`, and `evaluator`.
- [ ] Add `LiveSmokeSeed` with schema version, actor, fixture digest, synthetic marker, upstream output digests, validation evidence digest, expected patch paths, and seed digest.
- [ ] Add canonical prior-output builders that validate every synthetic upstream output against `ACTOR_OUTPUT_MODELS`.
- [ ] Add seed provenance fields to `LiveSmokeReport`.
- [ ] Run `uv run --python 3.12 pytest tests/live_smoke/test_seeds.py tests/live_smoke/test_contracts.py -q`.
- [ ] Commit with `feat(live-smoke): add canonical actor seeds`.

### Task 3: Extend Behavior Smoke Contracts

**Files:**
- Modify: `src/rail/live_smoke/contracts.py`
- Test: `tests/live_smoke/test_contracts.py`

- [ ] Change `evaluate_behavior_smoke()` to accept optional seed, target root, artifact dir, and invocation input context.
- [ ] Add critic non-empty critique checks.
- [ ] Add generator patch-bundle validation without applying the patch.
- [ ] Add executor count/log/failure-class checks.
- [ ] Add evaluator digest echo and `next_action` consistency checks.
- [ ] Run `uv run --python 3.12 pytest tests/live_smoke/test_contracts.py -q`.
- [ ] Commit with `feat(live-smoke): validate downstream actor smoke contracts`.

### Task 4: Wire Seeds Into The Runner And CLI

**Files:**
- Modify: `src/rail/live_smoke/runner.py`
- Modify: `src/rail/live_smoke/contracts.py`
- Modify: `src/rail/cli/main.py`
- Test: `tests/live_smoke/test_runner.py`
- Test: `tests/cli/test_live_smoke_commands.py`

- [ ] Make `run_all()` include all supported actors.
- [ ] Build canonical prior outputs and a `live_smoke_seed` before invoking each actor.
- [ ] Pass `live_smoke_runtime_contract` with the allowed read-only shell executable set and explicit tool-probe prohibition.
- [ ] Bind evaluator input digest with supervisor-equivalent digest ordering.
- [ ] Persist seed schema version, seed digest, and synthetic marker in reports.
- [ ] Update CLI unknown-actor tests now that downstream actors are supported.
- [ ] Run `uv run --python 3.12 pytest tests/live_smoke/test_runner.py tests/cli/test_live_smoke_commands.py -q`.
- [ ] Commit with `feat(live-smoke): run downstream actors with canonical seeds`.

### Task 5: Extend Optional Live Smoke Coverage

**Files:**
- Modify: `tests/e2e/test_optional_codex_vault_smoke.py`
- Test: `tests/e2e/test_optional_codex_vault_smoke.py`

- [ ] Parametrize optional live smoke over all supported live smoke actors.
- [ ] Keep `OPENAI_API_KEY` unset for the live smoke.
- [ ] Run the skipped-by-default test without the env flag.
- [ ] When operator auth is ready, run `env -u OPENAI_API_KEY RAIL_CODEX_VAULT_LIVE_SMOKE=1 uv run --python 3.12 pytest tests/e2e/test_optional_codex_vault_smoke.py -q -s`.
- [ ] Commit with `test(live-smoke): cover all seeded actors`.

### Task 6: Final Verification

**Files:**
- Verify only

- [ ] Run `uv run --python 3.12 pytest tests/live_smoke tests/cli/test_live_smoke_commands.py tests/e2e/test_optional_codex_vault_smoke.py -q`.
- [ ] Run `uv run --python 3.12 ruff check src tests`.
- [ ] Run `uv run --python 3.12 mypy src/rail`.
- [ ] Run `uv run --python 3.12 pytest -q`.
- [ ] If live auth remains ready, rerun the full actor live smoke with `RAIL_CODEX_VAULT_LIVE_SMOKE=1`.
