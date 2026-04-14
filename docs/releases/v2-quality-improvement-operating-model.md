# V2 Quality Improvement Operating Model

## Purpose

This document is the canonical operating rule set for `v2` quality improvement.

Its job is to answer one question safely:

`When should a reviewed same-family pattern become reusable guidance?`

The answer is intentionally conservative. `rail` should improve over time, but it
must do so through reviewable evidence rather than hidden adaptation.

## Primary Decision Unit

The primary decision unit is a paired same-family comparison:

- one `baseline` run
- one `follow_up` run
- the same `task_family`

The canonical comparison artifact is `quality_improvement_comparison`.

Rolling aggregates are allowed, but only as secondary monitoring. They may raise
review priority or restrict a family. They may not promote reusable guidance by
themselves.

## Improvement Definition

For `v2`, quality improvement means:

- `user outcome` improved
- `release readiness` did not get worse
- supporting validation evidence is at least sufficient
- the apparent improvement is not explained only by excessive intervention cost

In short:

`quality improvement = better user outcome, under non-regressing release readiness, backed by validation evidence, at justified intervention cost`

## Rating Axes

Every reviewed candidate should be judged across four independent axes.

### 1. User Outcome

Allowed values:

- `improved`
- `unchanged`
- `regressed`
- `unresolved`

This is the primary axis. No candidate may be promoted unless `user_outcome =
improved`.

### 2. Release Readiness Delta

Allowed values:

- `better`
- `same`
- `worse`
- `unresolved`

This is the gate-protection axis. A candidate may not be promoted when
`release_readiness_delta = worse`.

### 3. Validation Evidence Strength

Allowed values:

- `strong`
- `sufficient`
- `weak`
- `missing`

This is the evidence-quality axis. A candidate may not be promoted when
`validation_evidence_strength = weak | missing`.

### 4. Intervention Cost Assessment

Allowed values:

- `justified`
- `borderline`
- `high_cost_only`
- `unknown`

This is the anti-noise axis. A candidate may not be promoted when
`intervention_cost_assessment = high_cost_only | unknown`.

## Promotion Dispositions

The final review outcome must be one of:

- `promote`
- `hold`
- `reject`
- `harden`

Meanings:

- `promote`: the same-family pattern is reusable and may update approved memory
- `hold`: improvement may be real, but the evidence is not yet strong enough
- `reject`: the pattern should not become reusable guidance
- `harden`: the observation points to policy, safety, or hardening work rather than reusable family memory

Automatic promotion is forbidden. These rules support human review; they do not
replace it.

## Conservative Promotion Rule

`promote` is allowed only when all of the following are true:

- `user_outcome = improved`
- `release_readiness_delta = same | better`
- `validation_evidence_strength = strong | sufficient`
- `intervention_cost_assessment = justified`

Default outcomes for the remaining cases:

- `hold`
  - evidence is incomplete
  - the result is promising but still unresolved
  - the cost case is `borderline`
- `reject`
  - `user_outcome = regressed`
  - the claimed improvement is not supported on review
- `harden`
  - the observation implies policy, safety, or cross-family risk

## Operating Assist Rules

Operating assist rules optimize review throughput. They may reorder work. They
may not change a review disposition automatically.

### Promotion Candidate Priority

Allowed values:

- `high`
- `normal`
- `low`

Assignment:

- `high`
  - all `promote` conditions are already satisfied, but human approval has not yet happened
- `normal`
  - `user_outcome = improved`, but one supporting axis remains unresolved or weak
- `low`
  - `user_outcome = unchanged | regressed | unresolved`
  - or `release_readiness_delta = worse`

### Review SLA

- `high`: review within 3 days
- `normal`: review within 7 days
- `low`: review within 14 days

### Stale Hold Threshold

- `high` hold older than 7 days: `stale`
- `normal` hold older than 14 days: `stale`
- `low` hold older than 21 days: `stale`

### Bundle Re-Review Escalation

Within a rolling 30-day window for the same `task_family`:

- 2 `high` holds:
  - trigger `bundle_re_review`
- 3 `high` holds:
  - trigger `release_owner_escalation`

These events increase review priority only. They do not permit automatic
promotion.

## Family Risk Surveillance

Family monitoring exists to stop low-quality or unstable guidance from spreading.

### Family Watch Status

Allowed values:

- `normal`
- `watch`
- `restricted`

### Set `watch` When Any Condition Is True

Across the most recent 10 reviewed candidates or eligible reuse events in the
same family:

- `reject + harden >= 3`
- `promote/reuse` followed by `release_readiness_delta = worse` at least once
- `quarantine + drop >= 4`

### Set `restricted` When Any Condition Is True

Across the most recent 10 reviewed candidates or eligible reuse events in the
same family:

- `reject + harden >= 5`
- `promote/reuse` followed by `release_readiness_delta = worse` at least twice
- `quarantine + drop >= 6`

### Status Effects

- `watch`
  - `promote` requires a second reviewer
- `restricted`
  - new `promote` decisions are not allowed
  - only `hold`, `reject`, or `harden` are allowed
  - status may be lifted only after 5 consecutive clean reviewed candidates

A `clean reviewed candidate` means:

- `user_outcome != regressed`
- `release_readiness_delta != worse`
- `intervention_cost_assessment != high_cost_only`
- final disposition is not `harden`

## Required Evidence Sources

Promotion and monitoring decisions should be traceable to the current file-based
`v2` artifacts:

- `quality_learning_candidate`
- `user_outcome_feedback`
- `learning_review_decision`
- `approved_family_memory`
- `quality_improvement_comparison`
- `integration_result`
- derived queues and family evidence snapshots

When a paired comparison exists, it is the preferred proof artifact for claiming
long-term improvement.

## Non-Goals

This operating model does not allow:

- automatic pass-bar changes
- automatic policy mutation
- automatic promotion from aggregate metrics alone
- cross-family memory reuse
- promotion based only on lower retry counts or higher internal confidence

## Release-Ready Interpretation

Workstream 4 is complete only when this operating model is treated as the
canonical rule set for:

- promotion review
- hold escalation
- family watch and restriction
- the difference between meaningful improvement and noise
