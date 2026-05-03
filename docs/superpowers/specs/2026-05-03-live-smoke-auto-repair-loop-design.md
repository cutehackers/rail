# Live Smoke Auto-Repair Loop Design

## Goal

Add a bounded developer-facing auto-repair loop for Rail actor live smoke. The
loop runs canonical seeded live smoke, classifies failures, creates safe
Rail-owned repair candidates, optionally applies those candidates, and reruns
the affected actor until it passes or the repair budget is exhausted.

This is not a release publisher and not a downstream task runner. Version bump
and publishing remain operator-owned through the existing release flow.

## Context

The canonical seeded live smoke now covers all supervisor actors:

```text
planner -> context_builder -> critic -> generator -> executor -> evaluator
```

Each smoke report records actor, fixture digest, seed digest, evidence refs,
symptom class, owning surface, and an optional safe `repair_proposal`. The next
step is to make this signal actionable without repeating ad hoc manual repair
work after every live failure.

## Chosen Approach

Use a report-driven repair loop with deterministic repair candidates first.

The loop does not ask actors to edit Rail directly and does not relax policy to
make a failing run pass. It consumes the same `LiveSmokeReport` and runtime
evidence produced by `LiveSmokeRunner`, maps repairable failures to explicit
candidate patch bundles, applies only safe Rail-owned patches when requested,
then reruns the actor live smoke.

This is intentionally narrower than a generic LLM self-editing agent. It gives
repeatable fixes for known classes and leaves unknown or high-risk failures as
reviewable reports.

## Alternatives Considered

### Generic LLM Repair Actor

An LLM repair actor could inspect evidence and modify Rail code directly. This
maximizes flexibility but reintroduces the same policy and tool-control problem
that live smoke is meant to catch. It also makes failure provenance harder to
review. This option is deferred.

### Manual Reports Only

Keeping only repair proposals is safest but does not meet the goal of automatic
repair after repeat live smoke failures. It also leaves the operator doing the
same schema/prompt/runtime-contract fixes by hand.

### Deterministic Candidate Loop

The selected approach adds automation only where the fix surface is explicit,
bounded, and testable. It can be extended later with more repairers without
changing the runner contract.

## Repair Scope

Auto-repair may target only Rail-owned development surfaces already accepted by
`RepairProposal` validation:

- `.harness/actors/`
- `.harness/templates/`
- `src/rail/actor_runtime/`
- `src/rail/live_smoke/`
- `src/rail/package_assets/`
- selected release/package guard files already allowed by the model

Auto-repair must not target:

- downstream target repositories
- `.harness/artifacts/`
- `.git/` or worktree metadata
- Codex auth homes or operator home config
- live smoke fixture target files unless a dedicated fixture repairer explicitly
  owns that surface
- release version metadata, tags, or publishing scripts as part of a repair run

## Repair Loop Contract

Add `LiveSmokeRepairLoop` under `rail.live_smoke`.

Inputs:

- actor or all actors
- report root
- `apply`: false by default
- `max_iterations`: default `2`
- runtime factory, for tests
- optional repairer registry

Outputs:

- `LiveSmokeRepairLoopReport`
- per-iteration live smoke reports
- per-iteration repair candidates
- applied patch digests when apply mode is enabled
- terminal status: `passed`, `candidate_ready`, `repaired`, `unrepairable`,
  `budget_exhausted`, or `failed_validation`

The loop writes a JSON report beside live smoke reports and returns a typed
model to callers.

## Candidate Contract

Add a typed `RepairCandidate` model.

Fields:

- `schema_version`
- `actor`
- `symptom_class`
- `owning_surface`
- `source_report_path`
- `evidence_refs`
- `file_paths`
- `summary`
- `risk_level`: `low`, `medium`, or `high`
- `patch_bundle`
- `validation_commands`
- `preserves_fail_closed_policy`

Validation rules:

- file paths must satisfy `RepairProposal` path safety rules
- patch operations must validate through `validate_patch_bundle`
- candidates must preserve fail-closed policy
- high-risk candidates are never auto-applied
- apply mode refuses candidates that touch files outside the report proposal

## Initial Repairers

The first implementation should include repairers for failure classes that have
already occurred in live smoke hardening:

### Shell Policy Guidance

Input:

- `SymptomClass.POLICY_VIOLATION`
- `OwningSurface.RUNTIME_CONTRACT`
- evidence error contains `shell executable is not allowed`

Output:

- patch candidate for the actor prompt copies under `.harness/actors/`,
  `assets/defaults/actors/`, and
  `src/rail/package_assets/defaults/actors/`
- never widens the runtime shell allowlist
- adds guidance to avoid probing forbidden tools and to report unavailable
  tooling through structured output

### Actor Schema Drift

Input:

- `SymptomClass.SCHEMA_MISMATCH`
- runtime evidence contains Pydantic validation error text

Output:

- patch candidate for matching `.harness/templates/` and packaged template
  copies when a schema source is stale
- refuses repair when it cannot map the validation path to a known actor schema

### Behavior Contract Drift

Input:

- `SymptomClass.BEHAVIOR_SMOKE_FAILURE`
- `OwningSurface.ACTOR_PROMPT`

Output:

- prompt-copy patch candidate when the behavior check identifies a missing
  structured-output field or seed echo
- refuses repair for semantic quality failures that require product judgment

## Apply Mode

Default mode is dry-run. It creates reports and candidates but does not edit
files.

Apply mode:

1. runs live smoke
2. builds one repair candidate for a repairable failure
3. validates the candidate patch bundle
4. applies it to the Rail repo
5. runs focused tests named by the candidate
6. reruns the failed actor live smoke
7. stops when the actor passes or the budget is exhausted

Apply mode never commits. The operator or calling agent commits after reviewing
the final diff and verification.

When all actors are selected, apply mode checks for a dirty worktree once before
the repair run starts. Repairs may intentionally make the Rail repo dirty; later
actors in the same all-actor run must continue against that reviewed in-memory
repair state rather than failing only because an earlier actor produced a
candidate patch.

## CLI Surface

Add a development-oriented smoke repair command:

```bash
rail smoke repair actor generator --live --report-root .harness/live-smoke
rail smoke repair actor generator --live --apply --max-iterations 2
rail smoke repair actors --live --apply --max-iterations 2
```

Without `--apply`, the command is a dry run and exits non-zero when repair
candidates exist. With `--apply`, it exits zero only when all selected actors
pass after repair.

## Safety And Failure Handling

- Unknown failures remain unrepairable.
- Provider and operator environment failures remain unrepairable.
- Policy violations are repairable only when the repair tightens actor guidance
  or runtime contract metadata; they never broaden policy.
- Every applied patch records a pre/post tree digest.
- Every loop iteration writes stable report snapshots and can be inspected after
  interruption, even if later reruns overwrite the actor's latest smoke report.
- The loop stops on dirty worktree before apply mode starts unless an explicit
  test-only override is provided.

## Success Criteria

- dry-run repair reports can be produced from failed live smoke reports
- safe repair candidates validate file paths and patch bundles
- apply mode can fix a seeded fake failure and rerun the affected actor
- unrepairable failures remain fail-closed
- all repair actions are visible in typed reports
- `scripts/release_gate.sh` continues to pass after implementation
