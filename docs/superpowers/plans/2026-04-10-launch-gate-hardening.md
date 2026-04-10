# Launch Gate Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Harden the `rail` supervisor harness into a conservative production-quality gate that blocks ambiguous passes, refreshes context on every improvement cycle, and records whether guardrails improved quality or only added cost.

**Architecture:** Keep the existing core supervisor pipeline `planner -> context_builder -> generator -> executor -> evaluator`, but tighten its semantics. The work adds a conservative pass policy, current-state context refresh discipline, and reviewable guardrail cost/value signals to the runtime artifacts and surrounding contracts without expanding the core launch gate.

**Tech Stack:** Dart CLI runtime in `bin/rail.dart`, Markdown actor guidance, YAML contracts and schemas, checked-in `.harness/artifacts` evidence

---

### Task 1: Make the Conservative Quality Gate Explicit

**Files:**
- Modify: `.harness/actors/evaluator.md`
- Modify: `.harness/supervisor/context_contract.yaml`
- Modify: `.harness/supervisor/policy.yaml`
- Modify: `.harness/templates/evaluation_result.schema.yaml`

- [ ] **Step 1: Tighten evaluator pass rules**

Update `.harness/actors/evaluator.md` so `pass` requires:
- current-task grounding
- non-contradictory execution and validation evidence
- no unresolved material risk
- explicit preference for refusing weak proof over allowing ambiguous success

Add explicit guidance for:
- when weak evidence should stay in `validation_*`
- when stale or missing grounding must route to `context_*`
- when repeated weak proof should terminate instead of drifting

- [ ] **Step 2: Add contract language for current-state quality gating**

Update `.harness/supervisor/context_contract.yaml` so the workflow semantics describe:
- evaluator as the conservative production gate
- corrective actions as quality interventions rather than generic retries
- required current-state refresh before another corrective cycle proceeds

- [ ] **Step 3: Tighten supervisor policy wording**

Update `.harness/supervisor/policy.yaml` to state:
- `pass` is evidence-gated, not the default
- repeated retries must either improve quality or terminate explicitly
- context rebuilds exist to refresh stale grounding before another generator attempt

- [ ] **Step 4: Extend evaluation schema for explicit confidence**

Update `.harness/templates/evaluation_result.schema.yaml` to add one minimal machine-readable quality field such as `quality_confidence` with a tight enum, for example:

```yaml
quality_confidence:
  type: string
  enum:
    - high
    - medium
    - low
```

Keep the schema small. Do not add broad narrative fields when existing `findings` already carry the explanation.

- [ ] **Step 5: Validate the contract artifacts**

Run:

```bash
dart run bin/rail.dart validate-artifact --file .harness/artifacts/2026-04-10-standard-terminal-summary-passed/evaluation_result.yaml --schema evaluation_result
```

Expected:
- validation either passes immediately or exposes exactly which checked-in artifact must be refreshed after the schema change

### Task 2: Capture Guardrail Cost, Value, and Context Refresh in Runtime Artifacts

**Files:**
- Modify: `bin/rail.dart`
- Modify: `.harness/templates/execution_report.schema.yaml`

- [ ] **Step 1: Add a current-context refresh marker to supervisor state**

Update `bin/rail.dart` so every rebuild-driven improvement cycle records a small current-state marker in the artifact chain. Prefer a compact signal such as:
- context refresh count
- last refresh trigger
- last refresh reason-code family

Write it into the existing runtime state and expose it through the reports generated from state.

- [ ] **Step 2: Add guardrail cost accounting**

Update `bin/rail.dart` so `supervisor_trace.md` and `terminal_summary.md` clearly expose:
- number of generator revisions used
- number of context rebuilds used
- number of validation tightenings used
- whether the run ended in `passed`, `blocked_environment`, `split_required`, `evolution_exhausted`, or `revise_exhausted`

Do not create a new artifact type unless the existing reports cannot carry the signal clearly.

- [ ] **Step 3: Add minimal guardrail value reporting**

Update `bin/rail.dart` so terminal artifacts can answer:
- what failure or risk triggered intervention
- whether the final state improved confidence or ended in bounded refusal

Prefer deriving this from existing `reason_codes`, `actionHistory`, and the new `quality_confidence` field instead of inventing a large narrative system.

- [ ] **Step 4: Tighten execution report evidence for quality review**

If needed, update `.harness/templates/execution_report.schema.yaml` so executor evidence can support the new terminal explanations without schema drift. Keep any change narrow and backward-compatible with the current `failure_details` / `logs` model.

- [ ] **Step 5: Verify runtime-facing artifact generation**

Run:

```bash
dart run bin/rail.dart route-evaluation --artifact .harness/artifacts/2026-04-10-standard-route-validation-evidence/evaluation_result.yaml
```

Expected:
- routing still resolves deterministically
- the resulting terminal artifacts continue to render after the runtime changes

### Task 3: Add Evidence for Conservative Passes, Refreshed Context, and Bounded Refusal

**Files:**
- Modify: `.harness/requests/rail-standard-beacon-validation.yaml`
- Modify: `docs/superpowers/plans/2026-04-07-production-readiness-checklist.md`
- Create: `.harness/artifacts/2026-04-10-conservative-pass-weak-proof/`
- Create: `.harness/artifacts/2026-04-10-conservative-pass-context-refresh/`
- Create: `.harness/artifacts/2026-04-10-conservative-pass-exhausted/`

- [ ] **Step 1: Define a weak-proof scenario**

Create or refresh a `standard` scenario where:
- executor evidence is sparse or weak
- evaluator must refuse `pass`
- routing chooses `tighten_validation` or `revise_generator` instead of allowing success

- [ ] **Step 2: Define a stale-context scenario**

Create or refresh a `standard` scenario where:
- the first evaluation shows stale or insufficient grounding
- the system routes to `rebuild_context`
- the next artifact chain shows that the context was refreshed against current state before continuing

- [ ] **Step 3: Define a bounded-refusal scenario**

Create or refresh a `standard` scenario where:
- retries fail to materially improve evidence
- the harness ends in `evolution_exhausted` or `revise_exhausted`
- the terminal artifact makes the stop condition obvious

- [ ] **Step 4: Validate the new evidence artifacts**

Run for each new scenario:

```bash
dart run bin/rail.dart validate-artifact --file .harness/artifacts/<scenario>/evaluation_result.yaml --schema evaluation_result
dart run bin/rail.dart validate-artifact --file .harness/artifacts/<scenario>/execution_report.yaml --schema execution_report
dart run bin/rail.dart route-evaluation --artifact .harness/artifacts/<scenario>/evaluation_result.yaml
```

Expected:
- schemas validate
- routing stays deterministic
- terminal artifacts expose guardrail cost, value, and next step clearly

### Task 4: Close the Hardening Docs Around the New Quality Bar

**Files:**
- Modify: `docs/tasks.md`
- Modify: `docs/superpowers/plans/2026-04-07-production-readiness-checklist.md`
- Modify: `docs/superpowers/specs/2026-04-10-launch-gate-hardening-design.md`

- [ ] **Step 1: Update the task list**

Update `docs/tasks.md` so the launch gate language explicitly reflects:
- conservative pass policy
- current-state context refresh
- guardrail cost/value reviewability
- bounded refusal as a valid production-quality outcome

- [ ] **Step 2: Update the readiness checklist**

Add the new hardening evidence and mark the launch stance only where justified by fresh artifacts. Keep `integrator` out of the core gate unless new artifact evidence exists.

- [ ] **Step 3: Reconcile the spec with implementation details**

Update `docs/superpowers/specs/2026-04-10-launch-gate-hardening-design.md` only if runtime implementation forces a narrower or more precise interpretation than the current wording.

- [ ] **Step 4: Final verification pass for docs and artifacts**

Run:

```bash
dart run bin/rail.dart validate-artifact --file .harness/artifacts/2026-04-10-conservative-pass-weak-proof/evaluation_result.yaml --schema evaluation_result
dart run bin/rail.dart validate-artifact --file .harness/artifacts/2026-04-10-conservative-pass-context-refresh/execution_report.yaml --schema execution_report
dart run bin/rail.dart validate-artifact --file .harness/artifacts/2026-04-10-conservative-pass-exhausted/evaluation_result.yaml --schema evaluation_result
```

Expected:
- all refreshed artifacts validate against the tightened contracts
- the docs describe exactly the behavior now enforced by the runtime

### Task 5: Decide Whether the Hardening Pass Reached the Goal

**Files:**
- Modify: `docs/superpowers/plans/2026-04-07-production-readiness-checklist.md`
- Modify: `docs/tasks.md`

- [ ] **Step 1: Record the production stance**

Write a short closure section covering:
- whether the conservative pass gate is now real in runtime and docs
- whether context refresh is visible and bounded
- whether guardrail value can now be reviewed from artifacts

- [ ] **Step 2: Record what remains outside this cycle**

If the system still lacks proof for long-term quality improvement over time, record that as the next subproject rather than weakening the current quality claim.
