# V2 Release Evidence Runbook

## Purpose

This runbook explains how an operator should execute the `v2` gate, review the
resulting evidence, and make a consistent release decision.

Use this document together with:

- `docs/releases/v2-integrator-and-learning-gate.md`
- `docs/releases/v2-quality-improvement-operating-model.md`

## Scope

This runbook covers the `v2 integrator and learning gate` boundary only.

It does not define packaging, installer strategy, or external distribution.

## Gate Execution

Run from the repository root:

```bash
./tool/v2_release_gate.sh
```

Optional override:

- `RAIL_RELEASE_SMOKE_TASK_ID=<task-id>` changes the artifact directory name
- `task-id` must be a safe directory token using only letters, digits, `.`, `_`, or `-`

Default output artifact:

- `.harness/artifacts/v2-integrator-smoke-ci/`

The gate is expected to prove:

- dependency hygiene, analysis, tests, and binary compilation still pass
- `run`, `execute`, and `integrate` all complete for the smoke fixture
- `integration_result.yaml` is schema-valid
- learning-state snapshots remain coherent

## Review Procedure

After the script exits successfully, review:

- `.harness/artifacts/<task-id>/integration_result.yaml`
- `.harness/artifacts/<task-id>/evaluation_result.yaml`
- `.harness/artifacts/<task-id>/execution_report.yaml`

Apply the following checks in order.

### 1. Release Readiness

Interpret `integration_result.release_readiness` as follows:

- `ready`
  - the handoff is acceptable without additional release gating work
- `conditional`
  - the handoff is acceptable only with the listed caveats and follow-up
- `blocked`
  - do not claim release readiness

Hard rule:

- if `release_readiness = blocked`, stop and treat the candidate as not
  release-ready

### 2. Blocking Issues

Review `integration_result.blocking_issues`.

Decision rule:

- release claim allowed only when `blocking_issues: []`
- any non-empty `blocking_issues` list means the release claim is denied until
  the issue is resolved and the gate is rerun

### 3. Validation Evidence

Review each `integration_result.validation` entry.

Decision rule:

- release claim allowed only when every required check is `pass` or, for a
  limited-scope handoff, intentionally `warning` with explicit follow-up
- any `fail` or `blocked` validation entry denies the release claim

### 4. Evidence Quality

Review `integration_result.evidence_quality`.

Decision rule:

- `adequate` or `high_confidence` is acceptable for a release candidate
- `draft` is not enough to claim release readiness

### 5. Risks And Follow-Up

Review `integration_result.risks` and `integration_result.follow_up`.

Decision rule:

- `critical` risk denies the release claim
- `high` risk requires explicit release-owner acceptance and should normally
  remain `conditional`
- every `follow_up` item must have a concrete `owner`
- `conditional` handoff is acceptable only when follow-up items are explicit,
  short, and owner-assigned

## Current Smoke-CI Interpretation

The current `v2_release_gate.sh` path is a control-plane smoke gate.

Expected interpretation:

- the gate may end in `conditional`
- `blocking_issues` must still be empty
- follow-up should explain that the result proves smoke orchestration, not full
  non-smoke repair scope

This is acceptable for repository health and CI.

It is not by itself proof of a broad downstream repair release.

## Ownership Rules

Use these ownership defaults unless a stricter release process is defined.

- `release_readiness`
  - owner: release operator
  - responsibility: run the gate, review the artifact, and decide whether the
    candidate can be called release-ready
- `blocking_issues`
  - owner: release operator until reassigned
  - responsibility: deny release claim, route the blocker to the responsible
    implementer, and require a fresh rerun before reopening the decision
- `follow_up`
  - owner: the named `follow_up.owner` entry
  - responsibility: close the caveat or accept it explicitly as out of scope
- evidence retention
  - owner: release operator
  - responsibility: preserve the final artifact set used for the decision

## Evidence Retention

For every `v2` release candidate, preserve:

- the full artifact directory under `.harness/artifacts/<task-id>/`
- `integration_result.yaml`
- `evaluation_result.yaml`
- `execution_report.yaml`
  - `execution_report.yaml` now includes `actor_graph`, `actor_profiles_used`,
    `critic_findings_applied`, `critic_to_evaluator_delta`,
    `quality_trajectory`, and `quality_improvement_summary`
  - review `actor_profiles_used` to confirm the run used the checked-in actor
    execution policy intended for release
  - review `critic_findings_applied` and `critic_to_evaluator_delta` to confirm
    the critic stage materially influenced the evaluator outcome
- the exact gate command used, including any `RAIL_RELEASE_SMOKE_TASK_ID`
  override

Representative repository example:

- `docs/archive/v2-integrator-evidence-example.yaml`

Use the current checked-in schema and release docs as the comparison shape when
reviewing future handoffs, not an older archived example that may predate the
current reporting contract.

At minimum, confirm the reviewed `execution_report.yaml` includes:

- `actor_graph`
- `actor_profiles_used`
- `critic_findings_applied`
- `critic_to_evaluator_delta`
- `quality_trajectory`
- `quality_improvement_summary`

Treat archived examples as historical reference only, not as the source of
truth for the current evidence shape.

Also keep the scope of this smoke/release evidence review narrow:

- it proves the reviewed artifact set matches the current smoke gate outcome
- it does not by itself prove repository-wide invariants such as “critic is
  mandatory in all task families” or “structured actors never use actor-level
  timeout”
- those broader guarantees come from the repository-wide verification and test
  coverage run for the release candidate, not from this single smoke artifact

## Release Decision Summary

Use this final rule set:

- release-ready
  - gate command exits `0`
  - `release_readiness = ready | conditional`
  - `blocking_issues: []`
  - evidence quality is `adequate` or better
  - no validation entry is `fail` or `blocked`
- not release-ready
  - gate command fails
  - or `release_readiness = blocked`
  - or `blocking_issues` is non-empty
  - or evidence quality is `draft`
  - or unresolved critical risk remains
