# Actor Live Smoke Self-Repair Design

## Goal

Define a `codex_vault`-only live smoke environment that runs each Rail actor in
isolation, uses the real Codex command path, detects invalid actor behavior,
and automatically repairs Rail-owned `prompt`, `runtime`, and `asset` logic
when the failure is attributable to the Rail control plane.

The goal is not only to detect regressions. The system must support
`detect -> localize -> apply -> rerun` so that a live smoke failure can drive
targeted self-repair inside this repository and re-validate the same actor
under the same fixture conditions.

## Problem

The current optional live path is supervisor-oriented. It is useful for proving
that the end-to-end flow can run, but it is inefficient for debugging and
repair:

- failures surface late and often after unrelated actor work
- policy violations are discovered, but the failing actor is not isolated as the
  primary diagnostic unit
- repeated live verification is expensive because the whole flow must be
  replayed
- evidence often shows that the fix belongs to Rail-owned prompts, runtime
  contracts, or packaged assets rather than to the target repository

Recent failures such as forbidden `grep` usage in `context_builder`, sandbox
path escape attempts, and interpreter-selection drift show that the main value
of live smoke is to expose Rail control-plane mistakes and close them quickly.

## Scope

This design covers:

- `codex_vault` as the only v1 live provider
- all supervisor actors as actor-isolated live smoke targets
- real Codex command execution
- real sandbox preparation and real policy audit
- actor-specific `policy smoke` and lightweight `behavior smoke`
- automatic self-repair when the fix belongs to Rail-owned `prompt`,
  `runtime`, or packaged `asset` surfaces
- `pytest` and CLI entrypoints built on one shared runner

This design does not cover:

- `openai_agents_sdk` live execution
- arbitrary external target repositories
- replacing supervisor end-to-end live smoke
- semantic grading of actor output quality beyond smoke-level contracts
- automatic policy relaxation
- automatic edits to the target repository, auth homes, or runtime evidence

## Design Principles

- Keep the normal product contract skill-first and Python-API-first.
- Fail closed on readiness, policy, and evidence ambiguity.
- Use real provider behavior, not mocks, for live smoke.
- Treat each actor as the primary repair unit.
- Limit automatic repair to small, reviewable Rail-owned changes.
- Preserve strong separation between live smoke artifacts and normal task
  artifacts.
- Prefer deterministic fixture targets over realistic but noisy external
  projects.

## Runtime Model

### Live Target

Live smoke runs against a repo-owned fixture target workspace stored inside this
repository. The fixture is small, deterministic, and intentionally shaped to
exercise the actor prompts, runtime contracts, and audit rules that Rail owns.

Each actor run uses a fresh copy of the fixture target so that one actor's
mutations cannot contaminate the next actor's live smoke result.

### Execution Boundary

Each live smoke run must include:

1. fixture target preparation
2. actor-local artifact and attempt directory allocation
3. real actor invocation construction
4. `codex_vault` sandbox materialization
5. real Codex command execution
6. normalized event capture
7. runtime evidence capture
8. policy audit
9. actor smoke contract evaluation

The runner does not execute the full supervisor graph. It executes one actor at
a time while preserving the real provider boundary and the real audit path.

## Actor Smoke Contracts

Each actor live smoke uses two layers of checks.

### Policy Smoke

Every actor must pass the same strict policy checks:

- readiness requirements are satisfied
- the actor runs inside the expected sandbox boundary
- the actor does not read parent directories, host paths, or hidden user config
- the actor does not invoke forbidden executables or forbidden shell patterns
- the actor does not use disallowed auth or capability sources
- evidence is recorded and attributable to the current actor run

Any policy failure is terminal for that actor attempt and becomes the primary
repair input.

### Behavior Smoke

Each actor also has a lightweight shape contract that checks only the minimum
required output for that actor's role. These checks must stay intentionally
shallow so the live harness remains a regression detector rather than a semantic
grading system.

Examples:

- `planner`: structured plan output exists and required goal/constraint/completion
  fields are present
- `context_builder`: context pack output is schema-valid and includes non-empty
  relevant files, repo patterns, forbidden changes, and implementation hints
- `critic`: critique output is schema-valid and contains actionable findings or
  explicit no-issue reasoning in the required shape
- `generator`: generation output is schema-valid and references the intended
  mutation surface
- `executor`: patch bundle output or explicit no-op output is shape-valid and
  attributable
- `evaluator`: terminal recommendation, risk, evidence refs, and next-step
  fields are shape-valid

Actor-specific behavior smoke must remain limited to structural and
minimum-presence checks. It must not attempt to score overall implementation
quality.

## Failure Classification

When an actor live smoke fails, the runner normalizes the failure into one of
the following classes:

- `policy`
- `prompt_drift`
- `runtime_contract_gap`
- `schema_mismatch`
- `asset_drift`
- `readiness`
- `unknown`

The classification result must identify both the failure class and the owning
repair surface:

- actor prompt
- runtime invocation or runtime contract
- packaged skill or packaged default asset

The runner must not classify a failure as automatically repairable unless the
owning surface is inside the Rail control plane and the proposed change falls
within the allowed automatic repair scope.

## Self-Repair Loop

### Allowed Repair Scope

Automatic repair is allowed only for Rail-owned:

- actor prompts
- runtime invocation and runtime contract logic
- packaged skill or packaged default assets

Typical repair actions include:

- strengthening prompt wording to forbid invalid fallback behavior
- adding or tightening actor runtime contract fields
- aligning packaged skill copies with repo-owned skill instructions
- aligning packaged actor defaults with repo-owned actor prompts
- tightening executable guidance where policy already requires it

### Forbidden Repair Scope

The harness must never automatically:

- relax policy to make a failing actor pass
- edit the fixture target to hide a Rail-owned problem
- edit any external target repository
- mutate auth homes
- modify runtime evidence or normalized event output
- loosen output schemas only to accommodate bad actor output

### Loop Shape

For one actor run:

1. execute live smoke
2. classify the failure
3. localize the owning Rail surface
4. generate a minimal repair patch
5. apply the patch
6. rerun the same actor against the same fixture shape
7. stop on pass or after the retry limit is reached

The retry limit must be low and explicit. v1 should cap the automatic repair
loop at a small fixed number of reruns so the system does not drift into
unbounded exploration.

## Persistence And Git Behavior

Automatic repair uses `apply-and-rerun`.

When a repair succeeds:

- the fix remains in the working tree
- the harness records the failure class, owning surface, applied patch summary,
  and rerun result
- the harness commits the change automatically

Git safety rules for v1:

- create a repair branch only when the first actual repair is needed
- use a repair-only branch, not the caller's current branch
- commit each actor's successful repair immediately as its own commit
- do not wait for all actors to pass before creating commits

This structure preserves precise blame and rollback boundaries for each actor
repair.

## Runner And Interfaces

### Shared Runner

The core implementation should live in one shared runner API that owns:

- fixture target preparation
- actor invocation construction
- provider execution
- evidence loading
- smoke contract evaluation
- failure classification
- automatic repair orchestration
- artifact and report persistence

Both test and CLI entrypoints must call this shared runner rather than re-create
actor execution logic in parallel.

### Pytest Interface

`pytest` integration should support:

- selecting all live actor smokes
- selecting one actor or a subset of actors
- clear assertion messages that lead with actor name, blocked reason, and
  artifact/report path

Readiness failures must be hard test failures, not skips, because the explicit
purpose of this suite is to validate that the live environment is actually ready
to execute and repair actors.

### CLI Interface

The CLI should expose an operator-facing surface for ad hoc diagnosis, such as:

- run one actor live smoke
- run all actor live smokes
- preserve artifacts and reports for inspection

The CLI is a thin wrapper over the shared runner. It must not introduce a
separate execution model.

## Artifact And Report Model

Live smoke outputs must not be confused with normal resumable Rail task
artifacts.

v1 should use a separate artifact namespace for actor live smoke data that
stores:

- actor name
- fixture identity
- invocation snapshot
- normalized events
- runtime evidence
- smoke verdict
- failure classification
- applied repair summary
- rerun verdict
- created repair branch and commit refs when applicable

These reports are diagnostic products, not resumable task handles.

## Release And Operations

Actor-isolated live smoke is an explicit operator path. It is not part of the
default local test suite and it is not enabled by default in the release gate.

v1 should be wired behind an explicit flag so operators can opt in to the full
live self-repair path when they want to validate or harden the Rail control
plane. Once invoked, readiness failures must fail closed.

Supervisor-level live smoke remains valuable as a separate end-to-end proof, but
it no longer carries the full burden of diagnosing or repairing actor-level
drift.

## Success Criteria

This design is successful when:

- each supervisor actor can be live-smoked in isolation through `codex_vault`
- failures are classified into actionable Rail-owned repair surfaces
- allowed failures can be repaired automatically through `prompt`, `runtime`, or
  `asset` changes
- the same actor can be rerun immediately after repair and produce a passing
  smoke result
- repair commits are isolated on a repair-only branch
- the live smoke outputs make debugging faster than replaying full supervisor
  runs

## Open Decisions Deferred From v1

- Whether multi-actor chained smoke should exist alongside isolated actor smoke
- Whether successful repairs should later be squashed into fewer commits
- Whether a supervised end-to-end live run should consume actor-level smoke
  reports as prerequisites
- Whether additional provider backends should reuse the same self-repair model
