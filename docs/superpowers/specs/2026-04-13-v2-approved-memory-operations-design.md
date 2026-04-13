# V2 Approved-Memory Operations Design

**Date:** 2026-04-13

## Goal

Make `v2` approved-memory reuse operational, bounded, and auditable without turning post-pass review backlog into a release blocker.

## Problem Statement

`rail` already contains most of the machinery needed for `v2` approved-memory operations:

- reviewed family memory can be promoted into `.harness/learning/approved/`
- runtime consideration can reuse, drop, or quarantine approved memory
- queue and family evidence snapshots can be regenerated from candidate and decision state

What is still missing is an explicit operational contract.

Without that contract, the repository can drift in ways that are hard to reason about:

- operators may not know which files are safe to edit directly
- release checks may miss broken queue or evidence state
- approved-memory lifecycle may be interpreted as versioned archive storage instead of one active family baseline
- pending review backlog may be confused with broken release state

`Workstream 3` should lock the existing runtime direction into a clear operator model and a clear gate contract.

## Scope

This design covers only `v2` approved-memory operations:

- approved-memory lifecycle
- same-family reuse boundaries
- queue and evidence snapshot ownership
- `v2` release-gate checks for operational consistency

This design does not add:

- time-based expiration for approved memory
- automatic archive files for superseded family memory
- release blocking based only on pending review backlog
- any change that re-couples `integrator` or review workflows to the `v1` pass/fail loop

## Design Principles

- keep `v1` bounded and unchanged
- make `v2` review-driven, not hidden
- distinguish human-authored inputs from rail-derived state
- allow release with backlog, but not with broken provenance or broken snapshots
- keep approved-memory reuse same-family only
- keep the lifecycle simple enough to audit from current files plus git history

## Operational Model

### Single active approved file per family

Approved memory is stored at one canonical location per task family:

- `.harness/learning/approved/<task_family>.yaml`

The canonical file is the only active approved-memory baseline for that family.

When a later review promotes a new same-family candidate:

- the same canonical file is overwritten
- the new content becomes the active baseline
- old content is preserved only through git history

`v2` will not introduce archive or version files under `.harness/learning/approved/`.

### Same-family reuse only

Approved-memory reuse remains bounded to the same family key already used by runtime consideration and family evidence indexing.

The runtime may reuse approved memory only when:

- the canonical approved file exists
- the file validates against `approved_family_memory`
- the filename, internal `task_family`, and `task_family_source` are consistent
- request compatibility checks still pass
- repository compatibility checks still pass
- evidence freshness and latest-reference checks still pass

If those conditions no longer hold, the runtime must not reuse the memory even if the canonical approved file is still present.

### No time-based expiration in v2

`v2` will not add age-based expiration such as "older than N days."

Reuse is stopped only when existing bounded checks detect a mismatch or conflict, including:

- schema invalidity
- contract or policy version mismatch
- repository condition mismatch
- latest approved or latest success reference mismatch
- newer family failure or other evidence conflicts

This keeps the lifecycle deterministic and avoids introducing arbitrary clocks into the release contract.

## File Ownership Model

`Workstream 3` formalizes two classes of files.

### Operator-authored inputs

Operators may create or edit only the review inputs and review decisions:

- `.harness/learning/feedback/*.yaml`
- `.harness/learning/reviews/*.yaml`
- `.harness/learning/hardening-reviews/*.yaml`
- `.harness/learning/learning_review_decisions/*.yaml`
- `.harness/learning/hardening_review_decisions/*.yaml`

These files represent human judgment and are the only editable control surface for learning and hardening operations.

### Rail-derived state

Rail owns and regenerates all state snapshots and approved-memory outputs:

- `.harness/learning/review_queue.yaml`
- `.harness/learning/hardening_queue.yaml`
- `.harness/learning/family_evidence_index.yaml`
- `.harness/learning/approved/*.yaml`

Operators should treat these files as read-only derived state.

If an operator wants to change what these files say, they should modify feedback or review inputs and then run the relevant `apply-*` command again.

## Lifecycle Rules

### Promote

When `apply-learning-review` receives a `promote` decision:

- the decision must target the canonical approved-memory path for that family
- the canonical approved file is written or overwritten
- the written file must validate against `approved_family_memory`
- queue and family evidence snapshots are regenerated afterward

### Hold or reject

When `apply-learning-review` receives `hold` or `reject`:

- the decision is recorded in the decision store
- queue and family evidence snapshots are regenerated
- the runtime may still keep an older approved family file on disk
- but reuse must stop whenever the evidence/index checks now indicate `drop` or `quarantine`

This deliberately separates "file still exists" from "file is still eligible for reuse."

### Hardening review

`apply-hardening-review` never produces reusable family memory.

It records the reviewed hardening decision and regenerates snapshots, but policy-affecting observations remain outside approved-memory promotion.

## Gate Contract

The `v2` release gate is for operational consistency, not backlog elimination.

### Gate failures

The gate should fail when any of the following are true:

- dependency, analysis, test, build, or deterministic smoke checks fail
- explicit post-pass `integrate` cannot run successfully
- `integration_result.yaml` is invalid or reports `release_readiness: blocked`
- any active approved-memory file is schema-invalid
- any active approved-memory file disagrees with its canonical same-family path naming
- `family_evidence_index.yaml` is invalid or inconsistent with current decision and approved-memory state
- `review_queue.yaml` is invalid or inconsistent with current candidate and decision state
- `hardening_queue.yaml` is invalid or inconsistent with current candidate and decision state
- a `promote` decision points at a non-canonical approved-memory path

### Non-failures

The gate should not fail only because:

- a learning review candidate is still pending
- a hardening candidate is still pending
- review backlog exists but snapshot state is still valid
- an older approved file remains on disk while current evidence causes reuse to drop or quarantine

This keeps `v2` review-driven without making the release hostage to every outstanding post-pass review.

## Operator Workflow

The intended workflow is:

1. run the main execution and any explicit post-pass integration
2. generate review drafts with `init-*`
3. edit only the generated operator-authored draft or decision files
4. apply the reviewed file with `apply-user-outcome-feedback`, `apply-learning-review`, or `apply-hardening-review`
5. let rail regenerate queues, family evidence, and approved-memory outputs
6. run the `v2` release gate to confirm the resulting state is coherent

The key workflow rule is:

- people edit review inputs
- rail computes operational state

## Verification Expectations

`Workstream 3` is complete only when the following are covered by tests and release checks.

### Runtime tests

- `promote` accepts only the canonical approved-memory path
- a later `promote` overwrites the single active family file
- `hold` and `reject` do not automatically delete the current approved family file
- representative conflict cases cause approved-memory consideration to become `drop` or `quarantine`

### Snapshot tests

- `review_queue.yaml` is regenerated deterministically from current candidate and decision state
- `hardening_queue.yaml` is regenerated deterministically from current candidate and decision state
- `family_evidence_index.yaml` is regenerated deterministically from current candidate, decision, and approved-memory state

### Gate tests

- healthy state passes the `v2` gate
- invalid approved memory fails the gate
- queue schema drift fails the gate
- family evidence drift fails the gate
- pending review backlog alone does not fail the gate

### Documentation checks

Release and README documentation must state that:

- approved memory is same-family only
- there is one active approved file per family
- previous approved content is tracked through git history
- queue and evidence files are derived state, not operator-edited state

## Success Criteria

`Workstream 3` is done when:

- approved-memory reuse is explicitly bounded to same-family evidence-backed cases
- operators know which files are editable and which are derived
- queue and evidence state can be regenerated and validated reliably
- `v2` gate catches broken operational state without failing on mere backlog
- implementation planning can proceed without open ambiguity about lifecycle or ownership
