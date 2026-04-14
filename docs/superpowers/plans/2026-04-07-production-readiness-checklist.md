# Production Readiness Checklist

Goal: make harness supervision explicit, bounded, and production-credible for `rail` skill driven requests.

## Completed in this iteration

- [x] Standard bootstrap latency reduced with request-driven fast-path `planner -> context_builder`
- [x] Supervisor action contract simplified to single-value `next_action`
- [x] Environment/tooling failures routed to `block_environment` instead of `revise_generator`
- [x] `reason_code` taxonomy documented for evaluator outputs
- [x] Runtime action mapping standardized for `environment`, `validation_scope`, generic `validation` / `requirements`, `context`, `implementation`, `architecture`, and `scope`
- [x] Deterministic routing command added for supervisor-path verification: `route-evaluation`
- [x] `route-evaluation` guarded to avoid re-routing non-evaluator or terminal artifacts
- [x] Standard action paths verified:
  - [x] `validation_scope_* -> tighten_validation`
  - [x] `context_* -> rebuild_context`
  - [x] `implementation_* -> revise_generator`
  - [x] `scope_* -> split_task`
  - [x] `environment_* -> block_environment`
- [x] Supervisor decision trace emitted into artifacts via `supervisor_trace.md`
- [x] Standard end-to-end run reaches evaluator and records terminal routing state
- [x] Runtime fills missing executor evidence with fallback `failure_details` / `logs` so terminal artifacts remain readable
- [x] Terminal outcomes are materialized into `terminal_summary.md`
- [x] Reason-code precedence over `next_action` is documented in both evaluator guidance and supervisor trace output
- [x] Generic `validation_*` / `requirements_*` routing was tightened into `validation_evidence_*`, `validation_requirement_*`, `requirements_coverage_*`, and `requirements_behavior_*`
- [x] Refined validation and requirements routing categories were verified with `standard` route artifacts
- [x] Terminal summaries now include outcome explanation and recommended next step
- [x] Terminal summary evidence exists for `passed`, `blocked_environment`, and `split_required`
- [x] Fresh hardening evidence recorded for conservative-pass weak proof, current-state context refresh, and exhausted refusal behavior
- [x] Launch gate wording now distinguishes a conservative pass policy, bounded context refresh, reviewable guardrail cost/value, and bounded refusal as a production-quality outcome
- [x] Long-term quality improvement is now defined as reviewable candidate accumulation, explicit review decisions, approved-memory reuse with provenance, separate hardening-candidate escalation for policy changes, and bounded same-family guidance injection instead of hidden adaptation

## Evidence

- Smoke tighten-validation pass:
  - `/absolute/path/to/rail/.harness/artifacts/2026-04-07-supervisor-loop-smoke-fixes/state.json`
- Standard blocked-environment routing:
  - `/absolute/path/to/rail/.harness/artifacts/2026-04-07-standard-env-routing/state.json`
  - `/absolute/path/to/rail/.harness/artifacts/2026-04-07-standard-env-routing/evaluation_result.yaml`
- Standard supervisor trace:
  - `/absolute/path/to/rail/.harness/artifacts/2026-04-07-standard-trace-validation/supervisor_trace.md`
- Terminal outcome summary:
  - `/absolute/path/to/rail/.harness/artifacts/2026-04-07-standard-terminal-summary/terminal_summary.md`
  - `/absolute/path/to/rail/.harness/artifacts/2026-04-10-standard-terminal-summary-passed/terminal_summary.md`
  - `/absolute/path/to/rail/.harness/artifacts/2026-04-10-standard-terminal-summary-blocked/terminal_summary.md`
  - `/absolute/path/to/rail/.harness/artifacts/2026-04-10-standard-terminal-summary-split/terminal_summary.md`
- Quality learning review artifacts:
  - `/absolute/path/to/rail/.harness/artifacts/2026-04-10-quality-learning-candidate/quality_learning_candidates/01.yaml`
  - `/absolute/path/to/rail/.harness/learning/review_queue.yaml`
  - `/absolute/path/to/rail/.harness/learning/hardening_queue.yaml`
  - `/absolute/path/to/rail/.harness/learning/approved/feature_addition.yaml`
  - `/absolute/path/to/rail/.harness/learning/family_evidence_index.yaml`
- Conservative-pass hardening evidence:
  - `/absolute/path/to/rail/.harness/artifacts/2026-04-10-conservative-pass-weak-proof/state.json`
  - `/absolute/path/to/rail/.harness/artifacts/2026-04-10-conservative-pass-weak-proof/evaluation_result.yaml`
  - `/absolute/path/to/rail/.harness/artifacts/2026-04-10-conservative-pass-weak-proof/supervisor_trace.md`
  - `/absolute/path/to/rail/.harness/artifacts/2026-04-10-conservative-pass-context-refresh/state.json`
  - `/absolute/path/to/rail/.harness/artifacts/2026-04-10-conservative-pass-context-refresh/evaluation_result.yaml`
  - `/absolute/path/to/rail/.harness/artifacts/2026-04-10-conservative-pass-context-refresh/supervisor_trace.md`
  - `/absolute/path/to/rail/.harness/artifacts/2026-04-10-conservative-pass-exhausted/state.json`
  - `/absolute/path/to/rail/.harness/artifacts/2026-04-10-conservative-pass-exhausted/evaluation_result.yaml`
  - `/absolute/path/to/rail/.harness/artifacts/2026-04-10-conservative-pass-exhausted/terminal_summary.md`
- Deterministic standard route checks:
  - `/absolute/path/to/rail/.harness/artifacts/2026-04-07-standard-route-validation/state.json`
  - `/absolute/path/to/rail/.harness/artifacts/2026-04-07-standard-route-context/state.json`
  - `/absolute/path/to/rail/.harness/artifacts/2026-04-07-standard-route-implementation/state.json`
  - `/absolute/path/to/rail/.harness/artifacts/2026-04-07-standard-route-scope/state.json`
  - `/absolute/path/to/rail/.harness/artifacts/2026-04-10-standard-route-validation-evidence/state.json`
  - `/absolute/path/to/rail/.harness/artifacts/2026-04-10-standard-route-validation-requirement/state.json`
  - `/absolute/path/to/rail/.harness/artifacts/2026-04-10-standard-route-requirements-coverage/state.json`
  - `/absolute/path/to/rail/.harness/artifacts/2026-04-10-standard-route-requirements-behavior/state.json`

## Next production-facing priorities

## Launch closure audit on 2026-04-10

- [x] `standard` requests can complete or terminate with a correct supervisor action
- [x] every supervisor action used in the core launch path has deterministic routing rules and checked-in evidence
- [x] terminal artifacts explain the core launch states without reading raw actor logs
- [x] rubric failures now map explicitly to reason-code families and supervisor actions in evaluator guidance
- [x] self-evolution loops have explicit budgets and explicit exhausted terminal states
- [x] `integrator` semantics are explicit as a post-pass handoff stage
- [x] `integrator` is intentionally excluded from the core launch gate until `integration_result` artifact evidence exists for broader use

## Current launch stance

- The core launch-ready supervisor gate is: `planner -> context_builder -> generator -> executor -> evaluator`
- The gate is conservative in both runtime and docs: weakly evidenced passes are refused, and bounded refusal is a valid production-quality result when evidence stays insufficient
- `context_refresh` is visible in runtime traces and bounded by retry budgets, so reviewers can see when the system re-grounded itself before another correction
- guardrail cost and guardrail value are reviewable from `supervisor_trace.md` and `terminal_summary.md` artifacts
- quality-learning runtime is review-only: it emits multiple reviewable candidates, keeps policy-affecting patterns in a separate hardening path, and only reuses approved memory within the same family with provenance intact
- bounded guidance injection is now explicit and traceable, so runtime adaptation is not inferred from unreviewed memory writes
- `integrator` remains outside the core gate unless fresh `integration_result` evidence is added
- post-`integrator` completion is outside the core terminal-summary claim for this launch pass
