# Task Index Redirect

The completed launch-history task list now lives at [docs/archive/launch-history.md](/Users/junhyounglee/workspace/rail/.worktrees/v1-core-supervisor-gate/docs/archive/launch-history.md).

For active work:

- `v1` release backlog: [docs/backlog/v1-core-supervisor-gate.md](/Users/junhyounglee/workspace/rail/.worktrees/v1-core-supervisor-gate/docs/backlog/v1-core-supervisor-gate.md)
- `v2` deferred backlog: [docs/backlog/v2-integrator-and-learning.md](/Users/junhyounglee/workspace/rail/.worktrees/v1-core-supervisor-gate/docs/backlog/v2-integrator-and-learning.md)

Done when:

- each loop has a visible stop condition
- exhausted loops produce an explicit, reviewable final state
- runtime emits reviewable candidates rather than silently adapting from unreviewed memory
- policy-affecting patterns surface to hardening review without becoming reusable family memory
- approved-memory reuse is same-family only and provenance-backed
- guidance injection stays bounded and traceable to its source evidence

Evidence to record:

- `.harness/artifacts/2026-04-10-quality-learning-candidate/quality_learning_candidates/01.yaml`
- `.harness/learning/approved/feature_addition.yaml`
- `.harness/learning/review_queue.yaml`
- `.harness/learning/hardening_queue.yaml`
- `.harness/learning/family_evidence_index.yaml`

Closure note:

- runtime already materializes `evolution_exhausted` and `revise_exhausted`
- supervisor policy now makes those stop conditions explicit instead of implicit
- runtime explicitly surfaces a validation-tightening no-op when narrowing fails
- terminal summary and supervisor trace already expose the exhausted state and remaining budgets
- long-term quality improvement is now defined as reviewable candidate accumulation, explicit review decisions, approved-memory reuse with provenance, separate hardening-candidate escalation for policy changes, and bounded same-family guidance injection instead of hidden adaptation
- no reviewed artifact path may be treated as reusable memory until it appears in the approved family memory file with provenance intact

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
- quality learning is review-only, candidate-based, and provenance-backed; policy-affecting patterns stay in the hardening path unless a human explicitly promotes them
- same-family approved-memory reuse is bounded and traceable instead of being inferred from runtime adaptation

Intentional exclusion:

- `integrator` is specified as a post-pass handoff stage but is not required for launch-gate closure until checked-in `integration_result` evidence exists

Next subproject:

- continue review-only quality learning: accumulate candidates, route policy-affecting patterns to hardening, and promote only after explicit human review decisions with provenance

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
