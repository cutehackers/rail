# Live Smoke Auto-Repair Loop Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a bounded live smoke repair loop that turns repairable actor smoke failures into safe Rail-owned patch candidates, optionally applies them, and reruns the affected actor.

**Architecture:** Add typed repair models, evidence summarization, deterministic repairers, and a `LiveSmokeRepairLoop` orchestration layer under `rail.live_smoke`. Keep `LiveSmokeRunner` as the only actor execution surface and keep apply mode explicit, non-committing, and fail-closed.

**Tech Stack:** Python 3.12, Pydantic, pytest, existing Rail live smoke models, `validate_patch_bundle`, and CLI wrappers in `src/rail/cli/main.py`.

---

### Task 1: Add Repair Contracts

**Files:**
- Create: `src/rail/live_smoke/repair_models.py`
- Test: `tests/live_smoke/test_repair_models.py`

- [ ] Add failing tests for `RepairCandidate` rejecting unsafe file paths, high-risk auto-apply, non-fail-closed candidates, and invalid patch bundles.
- [ ] Implement `RepairRiskLevel`, `RepairCandidate`, `RepairIterationReport`, `RepairLoopStatus`, and `LiveSmokeRepairLoopReport`.
- [ ] Reuse `RepairProposal` validation rules for candidate file paths.
- [ ] Require `schema_version == "1"` and explicit actor/symptom/owning-surface fields.
- [ ] Run `uv run --python 3.12 pytest tests/live_smoke/test_repair_models.py -q`.
- [ ] Commit with `feat(live-smoke): add repair loop contracts`.

### Task 2: Summarize Runtime Evidence For Repair

**Files:**
- Create: `src/rail/live_smoke/repair_evidence.py`
- Test: `tests/live_smoke/test_repair_evidence.py`

- [ ] Add tests that load a live smoke report and attempt-scoped runtime evidence from fixture files.
- [ ] Extract only repair-relevant fields: actor, report path, symptom, owning surface, error text, policy violation reason, output schema ref, evidence refs, seed digest, and fixture digest.
- [ ] Redact secret-shaped values before exposing evidence summaries.
- [ ] Return a typed `RepairEvidenceSummary` model with `extra="forbid"`.
- [ ] Run `uv run --python 3.12 pytest tests/live_smoke/test_repair_evidence.py -q`.
- [ ] Commit with `feat(live-smoke): summarize repair evidence`.

### Task 3: Implement Deterministic Repairers

**Files:**
- Create: `src/rail/live_smoke/repairers.py`
- Test: `tests/live_smoke/test_repairers.py`

- [ ] Add a repairer registry keyed by `(SymptomClass, OwningSurface)`.
- [ ] Implement shell policy guidance repairer for `shell executable is not allowed`.
- [ ] Implement schema drift repairer for known actor schema/Pydantic mismatch patterns.
- [ ] Implement behavior contract prompt repairer for missing seed echo or missing required structured-output fields.
- [ ] Ensure every repairer returns either one low/medium-risk `RepairCandidate` or `None`.
- [ ] Assert no repairer widens shell policy or targets downstream/credential/artifact paths.
- [ ] Run `uv run --python 3.12 pytest tests/live_smoke/test_repairers.py -q`.
- [ ] Commit with `feat(live-smoke): add deterministic repairers`.

### Task 4: Orchestrate The Repair Loop

**Files:**
- Create: `src/rail/live_smoke/repair_loop.py`
- Modify: `src/rail/live_smoke/__init__.py`
- Test: `tests/live_smoke/test_repair_loop.py`

- [ ] Add tests for dry-run mode producing candidates without editing files.
- [ ] Add tests for apply mode applying a safe patch bundle, running focused validation, and rerunning the actor.
- [ ] Add tests for unrepairable provider/operator failures staying fail-closed.
- [ ] Add tests for budget exhaustion after `max_iterations`.
- [ ] Implement `LiveSmokeRepairLoop` using `LiveSmokeRunner.run_actor()` for every actor execution.
- [ ] Stop on dirty worktree by default before apply mode mutates files.
- [ ] Record pre/post tree digests for every applied candidate.
- [ ] Run `uv run --python 3.12 pytest tests/live_smoke/test_repair_loop.py -q`.
- [ ] Commit with `feat(live-smoke): orchestrate repair loop`.

### Task 5: Add CLI Repair Commands

**Files:**
- Modify: `src/rail/cli/main.py`
- Test: `tests/cli/test_live_smoke_commands.py`

- [ ] Add parser coverage for `rail smoke repair actor <name> --live`.
- [ ] Add parser coverage for `rail smoke repair actors --live`.
- [ ] Add `--apply` and `--max-iterations` flags.
- [ ] Make dry-run commands print typed repair loop JSON reports.
- [ ] Return non-zero for repair candidates in dry-run mode and zero only when selected actors pass.
- [ ] Run `uv run --python 3.12 pytest tests/cli/test_live_smoke_commands.py -q`.
- [ ] Commit with `feat(cli): add live smoke repair commands`.

### Task 6: Package And Docs Guard

**Files:**
- Modify: `tests/build/test_package_assets.py`
- Modify: `docs/ARCHITECTURE.md`
- Optional Modify: `docs/SPEC.md`

- [ ] Add package guard tests for any new repair-loop package assets or CLI contract text.
- [ ] Document that live smoke repair is a developer diagnostic and does not publish, commit, or mutate downstream targets.
- [ ] Ensure docs contain no local home paths.
- [ ] Run `uv run --python 3.12 pytest tests/build/test_package_assets.py tests/docs -q`.
- [ ] Commit with `docs(live-smoke): document repair loop boundary`.

### Task 7: Final Verification

**Files:**
- Verify only

- [ ] Run `uv run --python 3.12 pytest tests/live_smoke tests/cli/test_live_smoke_commands.py -q`.
- [ ] Run `uv run --python 3.12 ruff check src tests`.
- [ ] Run `uv run --python 3.12 mypy src/rail`.
- [ ] Run `uv run --python 3.12 pytest -q`.
- [ ] If Rail-owned Codex auth is ready, run `env -u OPENAI_API_KEY RAIL_CODEX_VAULT_LIVE_SMOKE=1 uv run --python 3.12 pytest tests/e2e/test_optional_codex_vault_smoke.py -q -s`.
- [ ] Run release dry run with `env -u OPENAI_API_KEY RAIL_CODEX_VAULT_LIVE_SMOKE=1 scripts/release_gate.sh`.
- [ ] Commit any final corrections with the narrowest accurate message.
