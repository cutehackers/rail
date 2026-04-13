# V2 Integrator and Learning Backlog

## Mission

`v2` starts after the `v1` core supervisor gate release.

Its job is to add post-pass integration and review-driven quality learning without weakening the bounded `v1` path.

## Deferred Scope

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

These entry conditions are now satisfied by `v0.1.0`.

## Recommended Work Sequence

### Workstream 1: Explicit `integrator`

Start here.

Goal:

- enable `integrator` only as an explicit post-pass stage after evaluator has already terminated the core gate

This work should define:

- when `integrator` runs
- what artifact it consumes
- what `integration_result` must prove
- when integration failure is informational versus release-blocking
- how operator-facing summaries describe integration separately from evaluator pass/fail semantics

Done when:

- `integrator` is isolated from the `v1` loop
- its inputs and outputs are explicit
- checked-in evidence exists for at least one representative `integration_result`

### Workstream 2: Review-Driven Learning Flows

Goal:

- turn learning and hardening into explicit offline review workflows rather than hidden runtime adaptation

This work should define:

- candidate generation boundaries
- review decision artifacts
- apply command behavior
- provenance requirements for any reusable memory

Done when:

- `apply-user-outcome-feedback`, `apply-learning-review`, and `apply-hardening-review` have clear command contracts
- reviewed artifacts are separate from reusable artifacts
- policy-affecting changes route to hardening instead of leaking into family memory

### Workstream 3: Approved-Memory Operations

Goal:

- make approved-memory reuse operational, bounded, and auditable

This work should define:

- approved-memory file format and lifecycle
- review queue and hardening queue ownership
- same-family reuse rules
- provenance and expiration expectations

Done when:

- approved-memory reuse is same-family only
- queue lifecycle is explicit and reviewable
- no live run depends on unreviewed memory

### Workstream 4: Quality Improvement Operating Model

Goal:

- describe how repeated execution produces better future outcomes without creating hidden behavior drift

This work should define:

- what quality signals are retained
- how operators review them
- when patterns become reusable guidance
- what metrics or artifact evidence indicate improvement versus noise

Done when:

- long-term quality improvement is operationally documented
- promotion from candidate to reusable guidance is review-driven and provenance-backed

## Success Criteria

`v2` should ship only when:

- `integrator` is verified as a post-pass handoff stage, not as a hidden extension of `v1` pass semantics
- deferred commands are isolated from the `v1` runtime path
- learning workflows are review-driven and operationally documented
- post-pass integration is backed by fresh evidence, not historical launch claims

## Non-Goals

`v2` is not a reason to reopen `v1` scope.

Do not:

- re-couple `integrator` to evaluator pass/fail semantics
- make approved-memory mandatory for the core gate
- let learning/apply commands become implicit runtime behavior
