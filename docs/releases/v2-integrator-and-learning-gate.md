# V2 Integrator and Learning Gate

## Release Contract

`rail v2` adds explicit post-pass integration and review-driven quality workflows on top of the `v1` core supervisor gate.

`v2` includes:

- explicit `integrator` execution as a post-pass handoff command
- contracted `integration_result` with quality/readiness fields
- file-based `init-*` and `apply-*` commands for user outcome, learning review, and hardening review
- operator-authored review inputs plus rail-derived queue, evidence, and approved-memory snapshots
- a canonical same-family approved-memory path at `.harness/learning/approved/<task_family>.yaml`
- operational checks for broken derived state, not for pending review backlog alone

`v2` intentionally excludes:

- changes to evaluator pass/fail routing rules
- unreviewed live reuse of quality memory
- direct hardening or policy updates outside reviewed apply commands

The canonical quality-improvement promotion and surveillance rules live in
`docs/releases/v2-quality-improvement-operating-model.md`.

The operator decision procedure lives in
`docs/releases/v2-release-evidence-runbook.md`.

When a later same-family review promotes new approved memory, rail overwrites the canonical file and keeps the older content only in git history.

## Gate Checklist

Recommended local gate:

```bash
./tool/v2_release_gate.sh
```

The gate should ensure:

- Go test and build complete cleanly
- the built Rail binary is used end-to-end
- a deterministic smoke route against the checked-in example target reaches terminal state
- explicit post-pass `integrate` is runnable
- `integration_result.yaml` validates against `.harness/templates/integration_result.schema.yaml`
- reviewed learning state validates after the integrator run

The current smoke target used by the release gate is:

- `examples/smoke-target/`

Real actor command construction is covered by runtime tests that assert
profile-selected model and reasoning arguments. The release gate intentionally
stays deterministic and does not require a live agent helper script.

After running the gate, operators should also confirm:

- `integration_result.release_readiness` is `ready` or `conditional` with explicit owner/action plan
- `integration_result.blocking_issues` is empty when release is claimed
- integration follow-up items are short, prioritized, and operator-owned
- pending review backlog may remain, but the gate should still fail on invalid approved memory, broken queue snapshots, or inconsistent family evidence

## Review-Driven Operator Flow

`v2` review workflows are explicit and file-based:

1. generate a schema-valid draft:
   - `init-user-outcome-feedback --artifact <artifact-dir>`
   - `init-learning-review --candidate <quality-candidate-ref>`
   - `init-hardening-review --candidate <hardening-candidate-ref>`
2. edit the draft with actual reviewer intent
3. apply the reviewed file:
   - `apply-user-outcome-feedback --file <path>`
   - `apply-learning-review --file <path>`
   - `apply-hardening-review --file <path>`

Default draft directories:

- `.harness/learning/feedback/`
- `.harness/learning/reviews/`
- `.harness/learning/hardening-reviews/`

Rail regenerates the derived state from those inputs:

- `.harness/learning/review_queue.yaml`
- `.harness/learning/hardening_queue.yaml`
- `.harness/learning/family_evidence_index.yaml`

Approved memory is updated only on `promote` at `.harness/learning/approved/<task_family>.yaml`; `hold` and `reject` validate the existing approved file and leave it unchanged.

`--file` is the only supported apply flag for the file-based review commands.

Quality-improvement review should follow the operating model document for:

- paired same-family comparison
- promotion versus hold/reject/harden decisions
- family watch and restriction handling
- the boundary between meaningful improvement and noise

## Minimum Evidence Requirements

For each release candidate:

1. keep the generated integrator artifact directory
2. keep the validated `integration_result.yaml`
3. keep `route-evaluation`/`execute` terminal artifacts referenced by that integration result
4. keep at least one representative handoff example in repository history or changelog for auditability

Representative example format:

- `summary`: what was changed
- `files_changed`: modified paths used for handoff
- `validation`: check list with evidence and status
- `risks`: residual risks with severity
- `follow_up`: concrete owner/action items
- `evidence_quality`: `adequate` or better
- `release_readiness`: `ready` or `conditional`

Checked-in reference example:

- `docs/archive/v2-integrator-evidence-example.yaml`

## Deferred Scope Control

These workflows remain outside the `v1` runtime gate even though they are part of the `v2` operator surface.
