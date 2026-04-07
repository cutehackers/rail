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

## Evidence

- Smoke tighten-validation pass:
  - `/Users/junhyounglee/workspace/rail/.harness/artifacts/2026-04-07-supervisor-loop-smoke-fixes/state.json`
- Standard blocked-environment routing:
  - `/Users/junhyounglee/workspace/rail/.harness/artifacts/2026-04-07-standard-env-routing/state.json`
  - `/Users/junhyounglee/workspace/rail/.harness/artifacts/2026-04-07-standard-env-routing/evaluation_result.yaml`
- Standard supervisor trace:
  - `/Users/junhyounglee/workspace/rail/.harness/artifacts/2026-04-07-standard-trace-validation/supervisor_trace.md`
- Terminal outcome summary:
  - `/Users/junhyounglee/workspace/rail/.harness/artifacts/2026-04-07-standard-terminal-summary/terminal_summary.md`
- Deterministic standard route checks:
  - `/Users/junhyounglee/workspace/rail/.harness/artifacts/2026-04-07-standard-route-validation/state.json`
  - `/Users/junhyounglee/workspace/rail/.harness/artifacts/2026-04-07-standard-route-context/state.json`
  - `/Users/junhyounglee/workspace/rail/.harness/artifacts/2026-04-07-standard-route-implementation/state.json`
  - `/Users/junhyounglee/workspace/rail/.harness/artifacts/2026-04-07-standard-route-scope/state.json`

## Next production-facing priorities

- Tighten evaluator reason-code generation so generic `validation_*` failures are more specific by default
- Review and harden integrator semantics before enabling it in broader workflows
