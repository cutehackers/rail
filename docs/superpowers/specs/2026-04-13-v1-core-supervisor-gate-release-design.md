# V1 Core Supervisor Gate Release Design

**Date:** 2026-04-13

## Goal

Ship `rail` as a production-release-credible control-plane for the bounded core supervisor gate, while redesigning the runtime so the current `bin/rail.dart` concentration no longer drives maintenance cost.

## Problem Statement

`rail` has a coherent product goal and a documented launch story, but the current repository is not yet release-ready:

- the runtime is concentrated in `bin/rail.dart`
- schema and runtime behavior have drifted in at least one live path
- checked-in evidence is partially stale relative to current contracts
- release verification is weaker than a production gate requires
- `integrator` and `quality learning` concerns are mixed into the same runtime surface as the core launch path

The first release should not try to ship every capability in the repository. It should ship the smallest trustworthy product boundary.

## Release Strategy

The product will be split into two release tracks.

### V1: Core Supervisor Gate

`v1` includes:

- request composition and request validation
- workflow bootstrap against external `--project-root`
- sequential actor execution for:
  - `planner`
  - `context_builder`
  - `generator`
  - `executor`
  - `evaluator`
- evaluator-driven bounded corrective loop
- deterministic supervisor actions:
  - `rebuild_context`
  - `revise_generator`
  - `tighten_validation`
  - `split_task`
  - `block_environment`
  - `pass`
- terminal artifact generation:
  - `state.json`
  - `supervisor_trace.md`
  - `terminal_summary.md`
- smoke and standard verification that can be reproduced from a fresh checkout
- compiled CLI build and release documentation

### V2: Deferred Scope

`v2` includes:

- `integrator`
- approved-memory reuse
- review queue and hardening queue flows
- `apply-user-outcome-feedback`
- `apply-learning-review`
- `apply-hardening-review`
- quality-improvement-over-time operating model

These capabilities may remain in the repository, but they must not be part of the `v1` release gate or required by the `v1` execution path.

## Core Supervisor Gate Definition

The `v1` product is not a single-pass pipeline. It includes one bounded supervisor corrective loop inside the same run.

The supported runtime behavior is:

1. bootstrap a request and artifact set
2. execute `planner -> context_builder -> generator -> executor -> evaluator`
3. let `evaluator` choose a deterministic next action
4. continue only within explicit retry budgets
5. stop in a visible terminal state when the run:
   - passes with sufficient evidence
   - is blocked by environment
   - must be split
   - exhausts bounded correction budgets

The `v1` loop explicitly excludes:

- post-pass `integrator` handoff
- long-term self-learning loops
- approved-memory mutation or review workflows

In short:

- bounded corrective loop included
- self-learning loop excluded

## Design Principles

- release the smallest trustworthy product boundary first
- keep the core release path explicit, bounded, and reproducible
- treat `pass` as an evidence-gated result, not a default
- separate `v1` release concerns from `v2` evolution concerns in both code and docs
- make stale artifacts and schema drift fail verification quickly
- reduce maintenance cost by moving from one large runtime file to focused modules

## Runtime Architecture

The runtime will be reorganized so `bin/rail.dart` becomes a thin entrypoint.

### Target structure

- `bin/rail.dart`
  - CLI entrypoint
  - command dispatch
  - process exit handling
- `lib/src/cli/`
  - command parsing
  - usage/help rendering
- `lib/src/request/`
  - compose-request
  - validate-request
  - defaults and normalization
- `lib/src/bootstrap/`
  - workflow materialization
  - artifact directory setup
  - actor brief generation
- `lib/src/runtime/`
  - supervisor state machine
  - actor execution loop
  - evaluator routing
  - retry budget enforcement
- `lib/src/contracts/`
  - registry loading
  - task router loading
  - context contract loading
  - schema validation
- `lib/src/reporting/`
  - state persistence
  - supervisor trace rendering
  - terminal summary rendering
  - artifact normalization
- `lib/src/process/`
  - shell command execution
  - `codex exec` wrapper
  - timeout and output capture
- `lib/src/v2/`
  - `integrator`
  - quality learning
  - apply-* commands

### Architectural rules

- the `v1` execution path must not import `lib/src/v2/`
- `bin/rail.dart` must not directly own runtime orchestration details
- schema normalization and artifact rendering must live outside the CLI entrypoint
- release-critical behavior must be testable without executing unrelated deferred features

## Release Gate

`v1` is release-ready only when all of the following are true:

- `dart analyze` returns clean with no warnings
- automated tests exist for core runtime behavior
- `dart test` passes
- `dart compile exe bin/rail.dart` succeeds
- smoke verification succeeds from a fresh artifact path using:
  - `run`
  - `execute`
- representative standard route fixtures validate against current schemas and current runtime behavior
- terminal artifacts are generated and readable without raw actor log inspection
- no `v2`-only required field or behavior can break a `v1` smoke or standard run
- release documentation matches the actual supported CLI and runtime behavior

## Verification Model

Verification will be split into four layers.

### 1. Unit and contract tests

- request normalization
- request validation
- registry and policy loading
- evaluator routing
- retry budget exhaustion
- terminal state rendering
- schema validation helpers

### 2. Golden and fixture tests

- `evaluation_result -> supervisor action`
- `terminal_summary.md` rendering
- artifact normalization
- representative `state.json` transitions

### 3. Smoke integration

- fresh request or checked request
- `run -> execute`
- generated artifacts validated against current schema
- terminal outcome verified

### 4. Release build verification

- `dart analyze`
- `dart test`
- `dart compile exe bin/rail.dart`

## Artifact Policy

Checked-in artifacts may still exist, but they are no longer the primary proof of readiness.

The new policy is:

- small checked-in fixtures are allowed
- stale checked-in evidence must fail validation and be refreshed or removed
- release trust comes from reproducible verification, not from historical artifact presence
- smoke and standard checks must be runnable from the current checkout without relying on old artifact assumptions

## Documentation and Backlog Redesign

The current `docs/tasks.md` is a completed-history document, not a good active release backlog. It should be replaced or archived.

### New document set

- `docs/releases/v1-core-supervisor-gate.md`
  - supported `v1` scope
  - non-supported deferred scope
  - release commands
  - operator expectations
- `docs/backlog/v1-core-supervisor-gate.md`
  - active release backlog
  - open blockers
  - verification gaps
- `docs/backlog/v2-integrator-and-learning.md`
  - deferred `v2` roadmap
- `docs/archive/launch-history.md`
  - optional archive destination for old completed task history

## Migration Plan

The work should proceed in this order:

1. define the `v1`/`v2` release boundary in docs
2. add or repair verification so the current runtime behavior is captured
3. remove `v2` requirements from the `v1` path
4. fix live release blockers in the existing runtime
5. split `bin/rail.dart` into focused modules
6. refresh smoke and standard evidence under the new `v1` gate
7. add release docs and CI-backed verification

This sequencing keeps release truth anchored in tests and reproducible commands rather than in refactor intent alone.

## Success Criteria

This design is successful when:

- `rail` can be released as a credible `v1` product focused on the core supervisor gate
- `v1` no longer depends on `integrator` or quality-learning behaviors
- the bounded corrective loop is part of the supported product and is clearly documented
- `bin/rail.dart` is reduced to a thin entrypoint over focused runtime modules
- fresh smoke and standard verification pass from the current checkout
- backlog and docs clearly distinguish `v1` shipped scope from `v2` deferred scope
