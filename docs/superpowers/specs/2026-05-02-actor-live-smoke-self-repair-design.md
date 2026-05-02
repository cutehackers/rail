# Actor Live Smoke Repair Design

## Goal

Define a `codex_vault` live smoke environment that can isolate actor-level
failures, classify the owning Rail control-plane surface, and produce repair
guidance that can later become safe automatic repair.

The long-term direction remains `detect -> localize -> repair -> rerun`.
However, v1 must not self-mutate the repository on shallow actor evidence.
The first implementation must prove isolated live execution and reliable
localization before automatic apply or commit behavior is allowed.

## Problem

The current optional live path is supervisor-oriented. It proves that an
end-to-end flow can run, but it is inefficient for debugging and repair:

- failures surface after unrelated actor work
- policy violations are discovered without a narrow actor-level diagnostic unit
- repeated live verification is expensive because the whole flow is replayed
- evidence often points to Rail-owned prompts, runtime contracts, or packaged
  assets rather than target repository code

Recent failures such as forbidden `grep` usage in `context_builder`, sandbox
path escape attempts, and interpreter-selection drift show that live smoke must
help locate Rail control-plane mistakes quickly. The first version should make
those failures actionable without creating an unsafe self-mutating harness.

## Scope

### V1 Scope

V1 covers:

- `codex_vault` as the only live provider
- actor-isolated live smoke for `planner` and `context_builder`
- real Codex command execution
- real sandbox preparation and real policy audit
- immutable fixture target snapshots
- report-only failure classification
- repair proposal generation for Rail-owned `prompt`, `runtime`, and `asset`
  surfaces
- one shared runner used by `pytest` and CLI entrypoints

V1 explicitly excludes:

- automatic patch application
- automatic branch creation or commits
- all non-initial actors unless they receive canonical seeded inputs in a later
  phase
- `openai_agents_sdk` live execution
- arbitrary external target repositories
- replacing supervisor end-to-end live smoke
- semantic grading of actor output quality beyond smoke-level contracts
- policy relaxation
- edits to target repositories, auth homes, runtime evidence, or smoke evidence

### Later Phases

Later phases may add:

- canonical seeded inputs for `critic`, `generator`, `executor`, and `evaluator`
- installed-wheel asset verification
- automatic apply-and-rerun in a disposable repair worktree
- repair branch creation and actor-scoped commits
- full actor coverage once upstream seed contracts are stable

Those phases require the safety gates defined in this document before
self-mutation is enabled.

## Design Principles

- Keep the normal product contract skill-first and Python-API-first.
- Fail closed on readiness, policy, and evidence ambiguity.
- Use real provider behavior, not mocks, for live smoke.
- Treat each actor as the primary diagnostic unit.
- Do not auto-apply or auto-commit changes until cross-surface safety gates
  exist.
- Keep live smoke artifacts separate from normal resumable Rail task artifacts.
- Use deterministic fixture snapshots instead of external target projects.
- Preserve policy strictness; repair Rail guidance and contracts instead of
  loosening guardrails.

## Runtime Model

### Fixture Target

Live smoke runs against a repo-owned fixture target workspace. The fixture is
small, deterministic, and shaped to exercise the actor prompts, runtime
contracts, and audit rules that Rail owns.

Each actor run receives a fresh immutable copy of the fixture. The runner records
a fixture digest before execution and rejects runs when the fixture source has
unexpected local changes.

Smoke outputs must live outside the copied fixture and outside normal Rail task
artifacts. Later actor runs must not be able to read prior smoke reports as
target context.

### Execution Boundary

Each v1 live smoke run includes:

1. fixture source digest check
2. fresh fixture target copy
3. smoke-only artifact/report directory allocation
4. real actor invocation construction
5. `codex_vault` sandbox materialization
6. real Codex command execution
7. normalized event capture
8. runtime evidence capture
9. policy audit
10. actor smoke contract evaluation
11. failure classification and repair proposal generation

The runner does not execute the full supervisor graph. It executes one actor at
a time while preserving the real provider boundary and the real audit path.

## Actor Coverage

### V1 Actors

V1 includes only `planner` and `context_builder`.

These actors are first because they can be isolated with request-level inputs and
do not require synthetic outputs from prior actors. They are also the actors most
directly involved in recent policy and context-gathering failures.

### Deferred Actors

`critic`, `generator`, `executor`, and `evaluator` are deferred until the runner
defines canonical seeded inputs for each actor. A deferred actor must not be
added to live smoke until its upstream input seed is:

- schema-valid
- versioned
- fixture-digest-bound
- realistic enough to exercise the actor's policy boundary
- clearly marked as synthetic so failures can be localized correctly

Without these seed contracts, a failing downstream actor could reflect bad
synthetic upstream state rather than a problem in that actor's prompt or runtime
contract.

## Actor Smoke Contracts

Each actor live smoke uses two layers of checks.

### Policy Smoke

Every actor must pass strict policy checks:

- readiness requirements are satisfied
- execution happens inside the expected sandbox boundary
- parent directories, host paths, and hidden user config are not read
- forbidden executables and forbidden shell patterns are not used
- disallowed auth or capability sources are not used
- normalized events and runtime evidence are recorded for the current actor run

Any policy failure is terminal for that actor attempt and becomes the primary
diagnostic input.

### Behavior Smoke

Behavior smoke checks only the minimum output shape needed to prove that the
actor executed its role. It must stay intentionally shallow so the live harness
does not become a flaky semantic grading system.

V1 behavior checks:

- `planner`: structured plan output exists and required goal, constraint, and
  completion fields are present
- `context_builder`: context pack output is schema-valid and includes non-empty
  relevant files, repo patterns, forbidden changes, and implementation hints

Later downstream actors may require stronger contracts, especially `executor`
and `evaluator`, because they own mutation and evidence decisions.

## Failure Classification

V1 classification is report-only. It must not trigger automatic patch
application.

The report records both a symptom class and a suspected owning surface.

Symptom classes:

- `readiness_failure`
- `provider_transient_failure`
- `policy_violation`
- `schema_mismatch`
- `fixture_digest_mismatch`
- `fixture_prep_failure`
- `evidence_writer_failure`
- `behavior_smoke_failure`
- `unknown_failure`

Owning surfaces:

- `actor_prompt`
- `runtime_invocation`
- `runtime_contract`
- `packaged_asset`
- `fixture`
- `provider`
- `operator_environment`
- `unknown`

The runner may produce a repair proposal only when:

- the symptom is reproducible in the current run
- the owning surface is Rail-owned
- the proposal does not relax policy
- the proposal does not alter target files, auth homes, evidence, or schemas to
  hide bad output

Examples:

- `grep` used by `context_builder`: classify as `policy_violation`, likely
  owning surface `actor_prompt` or `runtime_contract`
- parent directory read attempt: classify as `policy_violation`, likely owning
  surface `actor_prompt` or `runtime_contract`
- missing Rail auth: classify as `readiness_failure`, owning surface
  `operator_environment`
- malformed actor output: classify as `schema_mismatch` or
  `behavior_smoke_failure`, owning surface `actor_prompt` unless evidence points
  elsewhere

## Repair Proposal Model

V1 creates repair proposals but does not apply them automatically.

Allowed proposal surfaces:

- actor prompts
- runtime invocation and runtime contract logic
- packaged skill or packaged default assets

Typical proposals include:

- strengthening prompt wording to forbid invalid fallback behavior
- adding or tightening actor runtime contract fields
- aligning packaged skill copies with repo-owned skill instructions
- aligning packaged actor defaults with repo-owned actor prompts
- tightening executable guidance where policy already requires it

Forbidden proposal actions:

- relax policy to make a failing actor pass
- edit the fixture target to hide a Rail-owned problem
- edit any external target repository
- mutate auth homes
- modify runtime evidence or normalized events
- loosen output schemas only to accommodate bad actor output

The repair report must include:

- failed actor
- symptom class
- suspected owning surface
- evidence refs
- proposed file paths
- proposed change summary
- why the proposal preserves fail-closed behavior

## Automatic Repair Gate

Automatic apply-and-rerun is a later phase. It cannot be enabled until all of
these gates exist:

- disposable repair worktree creation
- clean-index precondition in the source worktree
- allowlisted staging paths
- generated smoke output exclusion
- no branch switching in the operator's current worktree
- per-actor repair commits only after rerun success
- cross-actor or supervisor smoke check for changes that can affect multiple
  actors
- installed-wheel asset verification when packaged assets are repaired
- explicit retry limit

Once these gates exist, automatic repair may use:

1. execute live smoke
2. classify failure
3. generate a minimal patch in the disposable repair worktree
4. rerun the same actor against a fresh fixture snapshot
5. run required cross-surface smoke checks for shared prompt/runtime/asset
   changes
6. commit the actor-scoped repair only on the repair branch

Until then, v1 remains report-only.

## Runner And Interfaces

### Shared Runner

The core implementation should live in one shared runner API that owns:

- fixture target preparation
- actor invocation construction
- provider execution
- evidence loading
- smoke contract evaluation
- failure classification
- repair proposal generation
- smoke report persistence

Both test and CLI entrypoints must call this shared runner rather than re-create
actor execution logic in parallel.

### Pytest Interface

`pytest` integration should support:

- selecting all v1 live actor smokes
- selecting one actor
- clear assertion messages that lead with actor name, symptom class, blocked
  reason, and report path

Readiness failures must be hard test failures, not skips, because the explicit
purpose of this suite is to validate that the live environment is ready.

### CLI Interface

The CLI should expose an operator-facing surface for ad hoc diagnosis:

- run one v1 actor live smoke
- run all v1 actor live smokes
- preserve smoke reports for inspection
- print repair proposals without applying them

The CLI is a thin wrapper over the shared runner. It must not introduce a
separate execution model.

## Artifact And Report Model

Live smoke outputs must not be confused with normal resumable Rail task
artifacts.

V1 uses a separate smoke report namespace that stores:

- actor name
- fixture digest
- invocation snapshot
- normalized events
- runtime evidence
- smoke verdict
- symptom class
- suspected owning surface
- repair proposal summary

These reports are diagnostic products, not resumable task handles.

Smoke report paths must be excluded from fixture target copies and from actor
context collection.

## Packaged Asset Verification

V1 may identify packaged asset drift, but it must distinguish repo-source
verification from installed-surface verification.

If a proposal touches packaged skill or default actor assets, the report must
state whether the installed wheel or installed tool surface was exercised. A
proposal that only checks repo-local files must not claim that the shipped
surface has been proven.

Installed-surface verification is required before later automatic repair can
commit packaged asset changes.

## Release And Operations

Actor-isolated live smoke is an explicit operator path. It is not part of the
default local test suite and it is not enabled by default in the release gate.

V1 should be wired behind an explicit flag so operators can opt in to live
diagnostics. Once invoked, readiness failures must fail closed.

Supervisor-level live smoke remains valuable as a separate end-to-end proof, but
it no longer carries the full burden of diagnosing actor-level drift.

## Success Criteria

V1 is successful when:

- `planner` and `context_builder` can be live-smoked in isolation through
  `codex_vault`
- policy failures are localized to actionable symptom and owning-surface pairs
- smoke reports include evidence refs and repair proposals for Rail-owned
  prompt, runtime, or asset surfaces
- fixture snapshots are immutable and digest-bound
- smoke artifacts cannot be mistaken for resumable Rail task artifacts
- no automatic patch, branch, or commit behavior exists in v1

The broader self-repair direction is successful only after later phases prove
that automatic apply, rerun, and commit behavior can run inside disposable
worktrees with allowlisted staging and cross-surface validation.

## Open Decisions Deferred From V1

- Canonical seeded input contracts for downstream actors
- Whether all actors should eventually be repaired independently or in groups
- Exact cross-surface checks required before auto-commit
- Whether successful repair commits should be squashed after review
- Whether supervisor live runs should consume actor-level smoke reports as
  prerequisites
- Whether additional providers should reuse the same repair model
