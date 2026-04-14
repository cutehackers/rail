# Supervisor Loop Evolution Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the rail supervisor choose explicit next actions after evaluation so orchestration quality improves through clear, bounded iteration loops.

**Architecture:** Extend evaluator output from coarse `pass/revise/reject` into structured supervisor actions and reason codes, then teach the runtime state machine to route to the right next stage with per-action budgets. Keep the design minimal by reusing the existing linear actor chain and only adding supervisor-level decision vocabulary instead of new actors.

**Tech Stack:** Dart CLI runtime, YAML schemas, harness supervisor configs

---

### Task 1: Define Supervisor Action Contract

**Files:**
- Modify: `/absolute/path/to/rail/.harness/templates/evaluation_result.schema.yaml`
- Modify: `/absolute/path/to/rail/.harness/actors/evaluator.md`
- Modify: `/absolute/path/to/rail/.harness/supervisor/context_contract.yaml`

- [ ] Add structured single-value `next_action` for supervisor routing.
- [ ] Add `reason_codes` so evaluator rationale is machine-readable.
- [ ] Update evaluator instructions to emit bounded orchestration actions instead of vague free text.

### Task 2: Add Supervisor Loop Budgets

**Files:**
- Modify: `/absolute/path/to/rail/.harness/supervisor/policy.yaml`
- Modify: `/absolute/path/to/rail/bin/rail.dart`

- [ ] Define per-action loop budgets in policy.
- [ ] Load those budgets into the runtime workflow model.
- [ ] Initialize artifact state with loop budgets and action history.

### Task 3: Route Evaluator Outcomes Through Supervisor

**Files:**
- Modify: `/absolute/path/to/rail/bin/rail.dart`

- [ ] Replace evaluator-only generator retry logic with action-based routing.
- [ ] Support `revise_generator`, `rebuild_context`, `tighten_validation`, `split_task`, `block_environment`, `pass`, and `reject`.
- [ ] Record `lastDecision`, `lastReasonCodes`, and `actionHistory` in state so artifacts show why iteration happened.

### Task 4: Keep Smoke Path Compatible

**Files:**
- Modify: `/absolute/path/to/rail/bin/rail.dart`

- [ ] Update deterministic smoke evaluator output to satisfy the new schema.
- [ ] Ensure smoke still terminates with `pass` and omitted `next_action`.

### Task 5: Document Runtime Semantics

**Files:**
- Modify: `/absolute/path/to/rail/README.md`
- Modify: `/absolute/path/to/rail/skills/rail/SKILL.md`
- Modify: `/absolute/path/to/rail/docs/superpowers/plans/2026-04-06-rail-bootstrap-followups.md`

- [ ] Explain that supervisor orchestration is now action-driven.
- [ ] Document bounded self-evolution loops and what they are for.
- [ ] Update follow-up notes to reflect the new production direction.
