# V2 Integrator and Learning Backlog

## Deferred to V2

- `integrator` as a supported post-pass handoff stage
- `apply-user-outcome-feedback`
- `apply-learning-review`
- `apply-hardening-review`
- approved-memory reuse in live runs
- review queue and hardening queue lifecycle management
- quality-improvement-over-time operating model

## Entry Conditions

Start `v2` only after `v1` is released and stable:

- the core supervisor gate is production-ready
- release verification is automated
- `bin/rail.dart` has been modularized enough that deferred flows can live outside the `v1` path

## Success Criteria

`v2` should ship only when:

- deferred commands are isolated from the `v1` runtime path
- learning workflows are review-driven and operationally documented
- post-pass integration is backed by fresh evidence, not historical launch claims
