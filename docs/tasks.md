# Harness Launch Tasks

Goal: make the `rail` harness launch-ready by satisfying the core requirements:

- supervisor orchestration must be explicit and improve output quality
- the structure must stay simple and legible
- rubric-driven requests through the `rail` skill must produce clear supervisor outcomes
- conservative pass behavior, current-state refresh, and guardrail reviewability must be visible, bounded, and reviewable through the supervisor pipeline

Current baseline:

- smoke orchestration is working
- standard orchestration reaches `evaluator`
- supervisor action routing exists and is traceable
- environment failures are routed to `block_environment`
- terminal outcome summaries are materialized

Launch is not achieved yet. The remaining work is below, in execution order.

---

## Launch gate

The product is launch-ready only when all of the following are true:

- `standard` requests can complete or terminate with a correct supervisor action
- the launch gate is conservative in both runtime and docs: weakly evidenced passes are refused, and bounded refusal is a valid production-quality outcome when evidence stays insufficient
- every supervisor action used in production has deterministic routing rules and real evidence
- terminal artifacts explain the core launch states without reading raw actor logs
- rubric failures map cleanly to reason codes and supervisor actions
- current-state context refresh is visible and bounded before another improvement cycle proceeds
- guardrail cost and guardrail value are reviewable from artifacts, so reviewers can tell whether an intervention improved the result or only added noise
- self-evolution loops are bounded, non-noisy, and make exhausted or no-op outcomes explicit where the runtime supports them
- integrator behavior is explicit enough to keep it safely outside the core launch gate until broader-workflow evidence exists
- long-term quality-improvement-over-time proof is still missing and remains the next subproject

---

## Task 1: Tighten `validation_*` and `requirements_*` reason codes

Status: complete on 2026-04-10

Why this is first:

- supervisor quality still depends too much on broad `validation_*` and `requirements_*` categories
- launch requires deterministic routing and clear outcomes

Primary files:

- `.harness/actors/evaluator.md`
- `.harness/templates/evaluation_result.schema.yaml`
- `bin/rail.dart`
- `docs/superpowers/plans/2026-04-07-production-readiness-checklist.md`

Steps:

- [x] define a narrower taxonomy for generic validation failures
- [x] separate validation-target problems from missing-validation-evidence problems
- [x] separate unmet-requirement problems from patch-quality problems
- [x] update runtime routing so each refined code still maps deterministically
- [x] document the taxonomy and precedence rules in evaluator guidance

Done when:

- the common generic `validation_*` / `requirements_*` cases now emit more specific codes by default
- the runtime routing table stays simple and predictable
- the evaluator guidance and runtime behavior say the same thing

Evidence to record:

- `.harness/artifacts/2026-04-10-standard-route-validation-evidence/state.json`
- `.harness/artifacts/2026-04-10-standard-route-validation-requirement/state.json`
- `.harness/artifacts/2026-04-10-standard-route-requirements-coverage/state.json`
- `.harness/artifacts/2026-04-10-standard-route-requirements-behavior/state.json`

---

## Task 2: Verify the remaining `standard` action loops end-to-end

Status: complete for routing evidence on 2026-04-10

Why this is second:

- routing rules are not enough; launch requires real `standard` path evidence
- only some supervisor actions are fully exercised today

Primary files:

- `bin/rail.dart`
- `.harness/requests/`
- `docs/superpowers/plans/2026-04-07-production-readiness-checklist.md`

Steps:

- [x] create or update `standard` fixtures that force `rebuild_context`
- [x] create or update `standard` fixtures that force `revise_generator`
- [x] create or update `standard` fixtures that force `split_task`
- [x] run each path through `evaluator` and capture final state
- [x] confirm `supervisor_trace.md` and `terminal_summary.md` remain readable for each path

Done when:

- `rebuild_context`, `revise_generator`, `split_task`, `tighten_validation`, and `block_environment` all have `standard` evidence
- each path shows correct final state and action history

Evidence to record:

- `.harness/artifacts/2026-04-07-standard-route-validation/state.json`
- `.harness/artifacts/2026-04-07-standard-route-context/state.json`
- `.harness/artifacts/2026-04-07-standard-route-implementation/state.json`
- `.harness/artifacts/2026-04-07-standard-route-scope/state.json`
- `.harness/artifacts/2026-04-07-standard-env-routing/state.json`
- `.harness/artifacts/2026-04-10-standard-route-validation-evidence/state.json`
- `.harness/artifacts/2026-04-10-standard-route-validation-requirement/state.json`
- `.harness/artifacts/2026-04-10-standard-route-requirements-coverage/state.json`
- `.harness/artifacts/2026-04-10-standard-route-requirements-behavior/state.json`

---

## Task 3: Make terminal outcomes obvious to a skill user

Status: complete for core launch states on 2026-04-10

Why this matters:

- a launch-ready product cannot require reading raw internals to understand failure or success
- this directly serves the “clear supervisor result” requirement

Primary files:

- `bin/rail.dart`
- `.harness/actors/evaluator.md`
- `docs/superpowers/plans/2026-04-07-production-readiness-checklist.md`

Steps:

- [x] standardize `terminal_summary.md` sections for `passed`, `blocked_environment`, `split_required`, `evolution_exhausted`, and `rejected`
- [x] ensure the summary explains the action chosen, why it was chosen, and what should happen next
- [x] ensure the summary stays useful even when executor output is sparse

Done when:

- the terminal summary alone is enough to explain the run outcome to a human reviewer
- the language is consistent across all terminal states

Evidence to record:

- `.harness/artifacts/2026-04-10-standard-terminal-summary-passed/terminal_summary.md`
- `.harness/artifacts/2026-04-10-standard-terminal-summary-blocked/terminal_summary.md`
- `.harness/artifacts/2026-04-10-standard-terminal-summary-split/terminal_summary.md`

---

## Task 4: Harden executor evidence collection under real failure modes

Status: complete at the contract level on 2026-04-10

Why this matters:

- fallback evidence now exists, but launch requires operationally useful failure reporting
- evaluator decisions are only as good as executor evidence

Primary files:

- `bin/rail.dart`
- `.harness/actors/executor.md`
- `.harness/templates/execution_report.schema.yaml`
- `docs/superpowers/plans/2026-04-07-production-readiness-checklist.md`

Steps:

- [x] classify executor failures by type: tooling unavailable, permission denied, sandbox blocked, command timeout, empty output
- [x] make those classes visible in `failure_details` and `logs`
- [x] keep the schema simple while making failure evidence materially better
- [x] ensure evaluator can distinguish environment failure from implementation failure using executor evidence alone

Done when:

- environment failures and command failures are clearly separable in the execution report
- evaluator no longer has to infer obvious environment problems from weak logs

Evidence to record:

- one artifact for a permission/sandbox-style failure
- one artifact for a command/test failure

Closure note:

- runtime executor normalization already emits machine-readable failure classes
- evaluator guidance now explicitly consumes those classes before choosing `reason_codes` or supervisor action
- the artifact requirements remain the review step for broader re-validation, but the launch contract is now explicit

---

## Task 5: Align rubric failures with supervisor routing

Status: complete on 2026-04-10

Why this matters:

- launch requires the rubric, evaluator, and supervisor to behave like one system
- right now the connection exists, but it is not yet explicit enough

Primary files:

- `.harness/actors/evaluator.md`
- `.harness/supervisor/context_contract.yaml`
- `.harness/supervisor/policy.yaml`
- `docs/superpowers/plans/2026-04-07-production-readiness-checklist.md`

Steps:

- [x] define which rubric misses should produce `context_*`, `implementation_*`, `validation_*`, `scope_*`, or `architecture_*`
- [x] document which supervisor action each rubric-related code should drive
- [x] keep the mapping short enough to audit

Done when:

- a reviewer can trace a rubric failure to a reason code and then to a supervisor action without guesswork

Evidence to record:

- one mapping table in docs
- at least one artifact showing rubric-aligned routing

Closure note:

- the mapping table now lives in `.harness/actors/evaluator.md`
- existing routing artifacts under `.harness/artifacts/2026-04-07-*` and `.harness/artifacts/2026-04-10-*` remain the checked-in routing evidence

---

## Task 6: Bound self-evolution loops more clearly

Status: complete at the contract level on 2026-04-10

Why this matters:

- launch requires self-evolution to improve quality, not create churn
- current budgets exist, but the stopping story can still be sharper

Primary files:

- `bin/rail.dart`
- `.harness/supervisor/policy.yaml`
- `docs/superpowers/plans/2026-04-07-production-readiness-checklist.md`

Steps:

- [x] document loop budgets by action
- [x] ensure bounded retry transitions terminate clearly
- [x] make retry exhaustion visible in `terminal_summary.md`
- [x] verify that repeated retries do not continue silently beyond budget

Done when:

- each loop has a visible stop condition
- exhausted loops produce an explicit, reviewable final state

Evidence to record:

- one `evolution_exhausted` artifact
- one `revise_exhausted` or equivalent bounded-stop artifact

Closure note:

- runtime already materializes `evolution_exhausted` and `revise_exhausted`
- supervisor policy now makes those stop conditions explicit instead of implicit
- runtime explicitly surfaces a validation-tightening no-op when narrowing fails
- terminal summary and supervisor trace already expose the exhausted state and remaining budgets

---

## Task 7: Harden `integrator` semantics before enabling broader use

Status: complete as a launch-scope exclusion on 2026-04-10

Why this matters:

- `integrator` is still the least production-ready stage
- launch should not rely on ambiguous post-pass behavior

Primary files:

- `bin/rail.dart`
- `.harness/actors/integrator.md`
- `.harness/supervisor/registry.yaml`
- `docs/superpowers/plans/2026-04-07-production-readiness-checklist.md`

Steps:

- [x] define when `integrator` should run and when it should not
- [x] define exact inputs, outputs, and termination semantics
- [x] make sure `integrator` does not blur the meaning of `pass`
- [x] either verify the stage with evidence or explicitly keep it disabled for launch

Done when:

- `integrator` is either safely specified and verified, or intentionally excluded from the launch scope

Evidence to record:

- updated workflow semantics
- artifact evidence if enabled

Closure note:

- `integrator` is now explicitly a post-pass handoff stage
- the core launch gate excludes `integrator` until `integration_result` artifact evidence exists
- this keeps pass/fail semantics authoritative at evaluator while preserving the future handoff stage

---

## Task 8: Close the launch gate with a final audit

Status: complete on 2026-04-10

Why this is last:

- this task exists to prove the harness now meets the core requirements

Primary files:

- `docs/tasks.md`
- `docs/superpowers/plans/2026-04-07-production-readiness-checklist.md`

Steps:

- [x] review every task above for evidence completeness
- [x] mark each launch requirement as satisfied or blocked
- [x] write a short final launch note describing what is ready and what remains intentionally out of scope

Done when:

- the checklist is fully closed or any remaining exclusion is explicit and accepted
- launch readiness can be justified from docs and artifacts alone

## Final launch note

The `rail` harness is documented launch-ready for the core supervisor decision pipeline:

- routing is explicit and bounded
- the gate is conservative in both runtime and docs, so weakly evidenced passes are refused instead of being accepted optimistically
- core launch terminal outcomes are readable without raw logs
- rubric failures now map cleanly to reason-code families and supervisor actions
- self-evolution stop conditions are explicit in both runtime behavior and supervisor policy
- current-state context refresh is visible and bounded before another corrective cycle proceeds
- guardrail cost and guardrail value are reviewable from checked-in artifacts

Intentional exclusion:

- `integrator` is specified as a post-pass handoff stage but is not required for launch-gate closure until checked-in `integration_result` evidence exists

Next subproject:

- prove long-term quality improvement over time; this cycle only establishes the conservative gate, bounded refresh, and reviewable guardrail signals

---

## Recommended execution order

1. Task 1
2. Task 2
3. Task 3
4. Task 4
5. Task 5
6. Task 6
7. Task 7
8. Task 8

This order keeps the work simple:

- first fix routing precision
- then prove real `standard` behavior
- then improve clarity and durability
- then settle `integrator`
- finally close the launch gate
