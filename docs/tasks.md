# Harness Launch Tasks

Goal: make the `rail` harness launch-ready by satisfying the core requirements:

- supervisor orchestration must be explicit and improve output quality
- the structure must stay simple and legible
- rubric-driven requests through the `rail` skill must produce clear supervisor outcomes
- self-evolution must be visible, bounded, and reviewable through the supervisor pipeline

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
- every supervisor action used in production has deterministic routing rules and real evidence
- terminal artifacts explain what happened without reading raw actor logs
- rubric failures map cleanly to reason codes and supervisor actions
- self-evolution loops are bounded, non-noisy, and do not waste retries on no-op actions
- integrator behavior is explicit enough to enable safely in broader workflows

---

## Task 1: Tighten `validation_*` and `requirements_*` reason codes

Status: complete on 2026-04-10

Why this is first:

- supervisor quality still depends too much on broad `validation_*` and `requirements_*` categories
- launch requires deterministic routing and clear outcomes

Primary files:

- `/Users/junhyounglee/workspace/rail/.harness/actors/evaluator.md`
- `/Users/junhyounglee/workspace/rail/.harness/templates/evaluation_result.schema.yaml`
- `/Users/junhyounglee/workspace/rail/bin/rail.dart`
- `/Users/junhyounglee/workspace/rail/docs/superpowers/plans/2026-04-07-production-readiness-checklist.md`

Steps:

- [ ] define a narrower taxonomy for generic validation failures
- [ ] separate validation-target problems from missing-validation-evidence problems
- [ ] separate unmet-requirement problems from patch-quality problems
- [ ] update runtime routing so each refined code still maps deterministically
- [ ] document the taxonomy and precedence rules in evaluator guidance

Done when:

- the common generic `validation_*` / `requirements_*` cases now emit more specific codes by default
- the runtime routing table stays simple and predictable
- the evaluator guidance and runtime behavior say the same thing

Evidence to record:

- `/Users/junhyounglee/workspace/rail/.harness/artifacts/2026-04-10-standard-route-validation-evidence/state.json`
- `/Users/junhyounglee/workspace/rail/.harness/artifacts/2026-04-10-standard-route-validation-requirement/state.json`
- `/Users/junhyounglee/workspace/rail/.harness/artifacts/2026-04-10-standard-route-requirements-coverage/state.json`
- `/Users/junhyounglee/workspace/rail/.harness/artifacts/2026-04-10-standard-route-requirements-behavior/state.json`

---

## Task 2: Verify the remaining `standard` action loops end-to-end

Status: complete for routing evidence on 2026-04-10

Why this is second:

- routing rules are not enough; launch requires real `standard` path evidence
- only some supervisor actions are fully exercised today

Primary files:

- `/Users/junhyounglee/workspace/rail/bin/rail.dart`
- `/Users/junhyounglee/workspace/rail/.harness/requests/`
- `/Users/junhyounglee/workspace/rail/docs/superpowers/plans/2026-04-07-production-readiness-checklist.md`

Steps:

- [ ] create or update `standard` fixtures that force `rebuild_context`
- [ ] create or update `standard` fixtures that force `revise_generator`
- [ ] create or update `standard` fixtures that force `split_task`
- [ ] run each path through `evaluator` and capture final state
- [ ] confirm `supervisor_trace.md` and `terminal_summary.md` remain readable for each path

Done when:

- `rebuild_context`, `revise_generator`, `split_task`, `tighten_validation`, and `block_environment` all have `standard` evidence
- each path shows correct final state and action history

Evidence to record:

- `/Users/junhyounglee/workspace/rail/.harness/artifacts/2026-04-07-standard-route-validation/state.json`
- `/Users/junhyounglee/workspace/rail/.harness/artifacts/2026-04-07-standard-route-context/state.json`
- `/Users/junhyounglee/workspace/rail/.harness/artifacts/2026-04-07-standard-route-implementation/state.json`
- `/Users/junhyounglee/workspace/rail/.harness/artifacts/2026-04-07-standard-route-scope/state.json`
- `/Users/junhyounglee/workspace/rail/.harness/artifacts/2026-04-07-standard-env-routing/state.json`
- `/Users/junhyounglee/workspace/rail/.harness/artifacts/2026-04-10-standard-route-validation-evidence/state.json`
- `/Users/junhyounglee/workspace/rail/.harness/artifacts/2026-04-10-standard-route-validation-requirement/state.json`
- `/Users/junhyounglee/workspace/rail/.harness/artifacts/2026-04-10-standard-route-requirements-coverage/state.json`
- `/Users/junhyounglee/workspace/rail/.harness/artifacts/2026-04-10-standard-route-requirements-behavior/state.json`

---

## Task 3: Make terminal outcomes obvious to a skill user

Status: complete for core launch states on 2026-04-10

Why this matters:

- a launch-ready product cannot require reading raw internals to understand failure or success
- this directly serves the “clear supervisor result” requirement

Primary files:

- `/Users/junhyounglee/workspace/rail/bin/rail.dart`
- `/Users/junhyounglee/workspace/rail/.harness/actors/evaluator.md`
- `/Users/junhyounglee/workspace/rail/docs/superpowers/plans/2026-04-07-production-readiness-checklist.md`

Steps:

- [ ] standardize `terminal_summary.md` sections for `passed`, `blocked_environment`, `split_required`, `evolution_exhausted`, and `rejected`
- [ ] ensure the summary explains the action chosen, why it was chosen, and what should happen next
- [ ] ensure the summary stays useful even when executor output is sparse

Done when:

- the terminal summary alone is enough to explain the run outcome to a human reviewer
- the language is consistent across all terminal states

Evidence to record:

- `/Users/junhyounglee/workspace/rail/.harness/artifacts/2026-04-10-standard-terminal-summary-passed/terminal_summary.md`
- `/Users/junhyounglee/workspace/rail/.harness/artifacts/2026-04-10-standard-terminal-summary-blocked/terminal_summary.md`
- `/Users/junhyounglee/workspace/rail/.harness/artifacts/2026-04-10-standard-terminal-summary-split/terminal_summary.md`

---

## Task 4: Harden executor evidence collection under real failure modes

Why this matters:

- fallback evidence now exists, but launch requires operationally useful failure reporting
- evaluator decisions are only as good as executor evidence

Primary files:

- `/Users/junhyounglee/workspace/rail/bin/rail.dart`
- `/Users/junhyounglee/workspace/rail/.harness/actors/executor.md`
- `/Users/junhyounglee/workspace/rail/.harness/templates/execution_report.schema.yaml`
- `/Users/junhyounglee/workspace/rail/docs/superpowers/plans/2026-04-07-production-readiness-checklist.md`

Steps:

- [ ] classify executor failures by type: tooling unavailable, permission denied, sandbox blocked, command timeout, empty output
- [ ] make those classes visible in `failure_details` and `logs`
- [ ] keep the schema simple while making failure evidence materially better
- [ ] ensure evaluator can distinguish environment failure from implementation failure using executor evidence alone

Done when:

- environment failures and command failures are clearly separable in the execution report
- evaluator no longer has to infer obvious environment problems from weak logs

Evidence to record:

- one artifact for a permission/sandbox-style failure
- one artifact for a command/test failure

---

## Task 5: Align rubric failures with supervisor routing

Why this matters:

- launch requires the rubric, evaluator, and supervisor to behave like one system
- right now the connection exists, but it is not yet explicit enough

Primary files:

- `/Users/junhyounglee/workspace/rail/.harness/actors/evaluator.md`
- `/Users/junhyounglee/workspace/rail/.harness/supervisor/context_contract.yaml`
- `/Users/junhyounglee/workspace/rail/.harness/supervisor/policy.yaml`
- `/Users/junhyounglee/workspace/rail/docs/superpowers/plans/2026-04-07-production-readiness-checklist.md`

Steps:

- [ ] define which rubric misses should produce `context_*`, `implementation_*`, `validation_*`, `scope_*`, or `architecture_*`
- [ ] document which supervisor action each rubric-related code should drive
- [ ] keep the mapping short enough to audit

Done when:

- a reviewer can trace a rubric failure to a reason code and then to a supervisor action without guesswork

Evidence to record:

- one mapping table in docs
- at least one artifact showing rubric-aligned routing

---

## Task 6: Bound self-evolution loops more clearly

Why this matters:

- launch requires self-evolution to improve quality, not create churn
- current budgets exist, but the stopping story can still be sharper

Primary files:

- `/Users/junhyounglee/workspace/rail/bin/rail.dart`
- `/Users/junhyounglee/workspace/rail/.harness/supervisor/policy.yaml`
- `/Users/junhyounglee/workspace/rail/docs/superpowers/plans/2026-04-07-production-readiness-checklist.md`

Steps:

- [ ] document loop budgets by action
- [ ] ensure no-op transitions always terminate clearly
- [ ] make retry exhaustion visible in `terminal_summary.md`
- [ ] verify that repeated no-value iterations do not continue silently

Done when:

- each loop has a visible stop condition
- exhausted loops produce an explicit, reviewable final state

Evidence to record:

- one `evolution_exhausted` artifact
- one `revise_exhausted` or equivalent bounded-stop artifact

---

## Task 7: Harden `integrator` semantics before enabling broader use

Why this matters:

- `integrator` is still the least production-ready stage
- launch should not rely on ambiguous post-pass behavior

Primary files:

- `/Users/junhyounglee/workspace/rail/bin/rail.dart`
- `/Users/junhyounglee/workspace/rail/.harness/actors/integrator.md`
- `/Users/junhyounglee/workspace/rail/.harness/supervisor/registry.yaml`
- `/Users/junhyounglee/workspace/rail/docs/superpowers/plans/2026-04-07-production-readiness-checklist.md`

Steps:

- [ ] define when `integrator` should run and when it should not
- [ ] define exact inputs, outputs, and termination semantics
- [ ] make sure `integrator` does not blur the meaning of `pass`
- [ ] either verify the stage with evidence or explicitly keep it disabled for launch

Done when:

- `integrator` is either safely specified and verified, or intentionally excluded from the launch scope

Evidence to record:

- updated workflow semantics
- artifact evidence if enabled

---

## Task 8: Close the launch gate with a final audit

Why this is last:

- this task exists to prove the harness now meets the core requirements

Primary files:

- `/Users/junhyounglee/workspace/rail/docs/tasks.md`
- `/Users/junhyounglee/workspace/rail/docs/superpowers/plans/2026-04-07-production-readiness-checklist.md`

Steps:

- [ ] review every task above for evidence completeness
- [ ] mark each launch requirement as satisfied or blocked
- [ ] write a short final launch note describing what is ready and what remains intentionally out of scope

Done when:

- the checklist is fully closed or any remaining exclusion is explicit and accepted
- launch readiness can be justified from docs and artifacts alone

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
