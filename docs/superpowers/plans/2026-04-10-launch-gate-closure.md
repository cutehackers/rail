# Launch Gate Closure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the `rail` harness launch gate at the documentation and contract level so the core supervisor pipeline is auditable as production-ready.

**Architecture:** Keep the runtime as the source of truth and align the surrounding contracts with it. The work is limited to actor guidance, supervisor contract/policy docs, and the final launch audit so the project can justify readiness from docs and checked-in artifacts alone.

**Tech Stack:** Markdown, YAML, existing Dart runtime semantics in `bin/rail.dart`

---

### Task 1: Write the launch-closure design artifacts

**Files:**
- Create: `docs/superpowers/specs/2026-04-10-launch-gate-closure-design.md`
- Create: `docs/superpowers/plans/2026-04-10-launch-gate-closure.md`

- [ ] **Step 1: Capture the launch-closure design**

Write the design doc with:
- scope
- runtime facts relied on
- executor-evidence interpretation
- rubric-routing alignment
- loop-bound semantics
- integrator launch-scope treatment

- [ ] **Step 2: Capture the implementation plan**

Write this plan so the remaining documentation edits are explicit and auditable.

### Task 2: Align actor contracts with runtime semantics

**Files:**
- Modify: `.harness/actors/evaluator.md`
- Modify: `.harness/actors/integrator.md`
- Modify: `.harness/supervisor/context_contract.yaml`
- Modify: `.harness/supervisor/policy.yaml`

- [ ] **Step 1: Update evaluator guidance**

Add:
- executor failure-class interpretation
- rubric miss to reason-code family mapping
- action selection rules tied to current runtime routing

- [ ] **Step 2: Update integrator guidance**

Make `integrator` explicitly:
- post-pass only
- handoff-only
- unable to reopen revision routing

- [ ] **Step 3: Update shared supervisor contracts**

Document:
- integrator inputs and handoff semantics
- action budgets by loop type
- exhaustion terminal states
- budget-bounded retry and no-op reporting rules that match current runtime behavior

### Task 3: Close the launch docs

**Files:**
- Modify: `docs/tasks.md`
- Modify: `docs/superpowers/plans/2026-04-07-production-readiness-checklist.md`

- [ ] **Step 1: Update the task list**

For Tasks 4 through 8:
- mark documentation-closed items complete where justified
- explicitly record any launch-scope exclusion
- point each item at the evidence or contract that closes it

- [ ] **Step 2: Write the final audit**

Add a short audit section covering:
- each launch requirement
- satisfied vs excluded status
- the final documented launch stance for `integrator`

### Task 4: Hand off the documented launch state

**Files:**
- Modify: `docs/tasks.md`
- Modify: `docs/superpowers/plans/2026-04-07-production-readiness-checklist.md`

- [ ] **Step 1: Summarize what is now launch-ready**

Summarize the core supervisor gate as:
- explicit
- bounded
- evidence-backed

- [ ] **Step 2: Call out the remaining review item**

Record that broader `integrator` verification remains a review item before relying on it as a launch-gating artifact producer.
