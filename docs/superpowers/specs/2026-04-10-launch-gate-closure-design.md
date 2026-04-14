# Launch Gate Closure Design

## Goal
Make the `rail` harness documented launch-ready for the core supervisor pipeline by closing the remaining contract gaps, aligning rubric failures with routing, and turning the existing evidence into a final launch audit.

## Scope
- Re-open Tasks 1 through 8 in [docs/tasks.md](/absolute/path/to/rail/docs/tasks.md) as an audit pass.
- Treat Tasks 1 through 3 as already-implemented capabilities that must be re-validated against current docs and runtime behavior.
- Close Tasks 4 through 8 primarily through contract and documentation updates unless the docs would otherwise claim behavior the runtime does not support.

## Runtime facts this design relies on
- The runtime already classifies executor failures into machine-readable classes such as `environment_permission`, `environment_sandbox`, `tooling_unavailable`, `command_timeout`, `empty_output`, and `validation_failure`.
- The supervisor already has explicit bounded terminal states for `blocked_environment`, `split_required`, `evolution_exhausted`, and `revise_exhausted`.
- The runtime already records `supervisor_trace.md` and `terminal_summary.md`.
- The configured registries route `integrator` for `feature_addition` and `safe_refactor`; the runtime itself only checks whether the resolved workflow includes `integrator`.
- There is no checked-in `integration_result.yaml` evidence yet.

## Design decisions

### 1. Use the existing runtime as the source of truth
This pass does not invent a new launch model. The documentation will match the current runtime semantics:
- evaluator reason codes remain authoritative for routing
- executor evidence classes explain environment vs validation vs evidence failures
- retry and loop exhaustion remain bounded and visible in terminal artifacts

### 2. Make executor evidence interpretation explicit
The evaluator contract will explicitly say how to read executor failure classes:
- environment and tooling classes drive `environment_*` reason codes and `block_environment`
- `empty_output` and similarly weak evidence drive `validation_evidence_*`
- concrete validation failures still require rubric-aware classification into `validation_requirement_*`, `requirements_coverage_*`, or `requirements_behavior_*`

This closes the gap where the runtime had the evidence but the actor guidance did not say how to consume it.

### 3. Add one short rubric-to-routing map
The launch docs will contain an auditable table from rubric miss -> preferred reason code family -> supervisor action.

This is intentionally short:
- requirements and correctness misses map to requirement or validation families
- architecture misses map to `architecture_*`
- maintainability and blast-radius misses map to `implementation_*` or `scope_*`
- context and validation-target misses stay in their existing families

### 4. Make loop stop conditions explicit
The policy and docs will state:
- `revise_generator` is bounded by the risk-tolerance retry budget and ends in `revise_exhausted`
- `rebuild_context` and `tighten_validation` each have bounded budgets and end in `evolution_exhausted`
- `tighten_validation` no-op outcomes should be visible when narrowing fails
- broader loop control is budget-bounded first; this pass does not claim richer runtime no-op detection than the code actually enforces

### 5. Narrow `integrator` semantics
`integrator` will be documented as:
- post-pass only
- handoff-only
- not allowed to reopen routing or reinterpret evaluator pass/fail semantics

For this launch pass, the core supervisor gate is considered satisfied by the planner/context/generator/executor/evaluator pipeline. `integrator` remains specified but excluded from the launch gate until artifact evidence exists for broader use.

## Files to update
- `.harness/actors/evaluator.md`
- `.harness/actors/integrator.md`
- `.harness/supervisor/context_contract.yaml`
- `.harness/supervisor/policy.yaml`
- `docs/tasks.md`
- `docs/superpowers/plans/2026-04-07-production-readiness-checklist.md`

## Deliverables
- explicit evaluator guidance for executor evidence classes
- explicit rubric-to-routing mapping table
- explicit loop-bound and exhaustion semantics
- explicit integrator role and launch-scope exclusion
- final launch audit in the task list and readiness checklist
