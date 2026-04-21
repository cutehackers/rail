# Rail Architecture

## Overview

Rail is an installed harness control-plane product. It does not host the downstream application being changed. Instead, Rail provides the packaged CLI, the bundled Rail skill, embedded default control-plane assets, and the runtime that operates against another repository.

The governing model is:

- the installed product owns reusable defaults and orchestration logic
- the target repository owns project-local state under `.harness/`
- the supervisor keeps workflow decisions explicit and reviewable

## Product Runtime Model

Rail is intended to be installed and used like any other developer tool:

```bash
brew install cutehackers/rail/rail
cd /absolute/path/to/target-repo
rail init
```

The installed product includes:

- the native `rail` CLI
- the bundled Rail Codex skill
- embedded defaults for supervisor policy, actors, rules, rubrics, and templates

The source repository is the development and contribution origin for those assets. It is not the required runtime root for end users.

Rail is distributed first through the `cutehackers/rail` Homebrew tap backed by
GitHub Releases from `https://github.com/cutehackers/rail`. GoReleaser builds
the tagged CLI artifacts, attaches checksums and provenance, and publishes the
tap formula that installs both the binary and bundled Codex skill assets.

## Core Runtime Components

### Rail CLI

The CLI is the execution engine. It is responsible for:

- request composition and normalization
- schema validation
- project initialization
- artifact bootstrap
- actor orchestration
- evaluation routing
- terminal reporting

### Bundled Rail Skill

The Rail skill is the natural-language entrypoint. It interprets the user goal, constraints, and definition of done, then hands a request draft to the CLI. The skill assumes `rail` is installed on `PATH`; it does not assume a local source checkout.

### Embedded Defaults

Rail ships embedded defaults for reusable harness assets. These packaged defaults cover:

- supervisor policy
- actor instructions
- rules and rubrics
- request and artifact templates

Embedded defaults are part of the product contract. They give every initialized project a known baseline even when the project has not customized its own harness files.

### Project-Local `.harness`

Each target repository owns a project-local `.harness/` workspace. This is the project-specific control-plane surface.

Required local state:

- `.harness/project.yaml`
- `.harness/requests/`
- `.harness/artifacts/`
- `.harness/learning/`

Those paths remain local because they hold project identity, run history, evidence, and reviewed memory. They are never satisfied by global fallback.

## Runtime Flow

The runtime flow stays explicit:

1. The user invokes the bundled Rail skill or the CLI directly.
2. Rail converts the request into a harness task.
3. Rail materializes or updates artifact state in the target repository.
4. The supervisor dispatches bounded actors in sequence.
5. Each actor writes schema-valid output back into the artifact set.
6. The evaluator decides whether the run passes, retries within budget, or stops.
7. Optional `v2` integration and learning flows extend the result after a passing core run.

This design deliberately favors reviewable transitions over hidden automation.

Rail currently maintains two execution profiles:

- `real` mode as the default actor path for actual target-repository work
- `smoke` mode as the fast deterministic path for control-plane verification

Actor command runs do not use actor-level wall-clock cutoffs. The runtime uses
`ActorWatchdog` as a progress guard: if an actor process stops producing
observable command output for the quiet window, Rail cancels that actor process
and reports `actor_watchdog_expired`.

## Supervisor, Actors, And Rubrics

The control relationship is stable:

- the supervisor owns workflow control
- actors own bounded execution
- rubrics and rules define how results are judged

Actors do not own the whole workflow. They perform a narrow step and return schema-valid output. The supervisor reads that output, applies policy, and decides what happens next.

That separation keeps the system maintainable:

- actor behavior can evolve without rewriting the whole control loop
- routing policy can change without rewriting every actor
- release decisions remain inspectable because evaluation criteria are explicit

## Advanced Overrides

Rail supports advanced customization through project-local `.harness` files. This is the intended advanced override surface.

Common override locations:

- `.harness/supervisor/`
- `.harness/actors/`
- `.harness/rules/`
- `.harness/rubrics/`
- `.harness/templates/`

Resolution follows a simple precedence model:

1. Use the project-local file when it exists.
2. Otherwise use the embedded defaults from the installed product.
3. Treat the result as a file-level override, not a deep merge.
4. Keep stateful directories project-local regardless of defaults.

### Why File-Level Override

The file-level override rule is intentional.

- it keeps override provenance obvious
- it makes debugging straightforward
- it avoids hidden merge semantics across product upgrades
- it keeps advanced customization explicit and reviewable

The product does not attempt partial merges of policy or template files. If a project wants to change a file, it owns the full file.

## Artifacts And Learning State

Rail separates run artifacts from reviewed learning state.

- run artifacts describe what happened for one task
- learning state describes what the system should remember after reviewed outcomes

In `v2`, operators edit review files such as feedback or learning decisions. Rail then regenerates queue and evidence state from those reviewed files so the derived state remains reproducible.

Approved memory is family-scoped. Promotion updates the active approved memory for a task family rather than building a hidden chain of mutable state.

## V1 And V2 Boundary

`v1` is the bounded core supervisor gate. It focuses on deterministic execution, corrective loops, and explicit terminal outcomes.

`v2` builds on `v1` with:

- an explicit integration handoff after a passing run
- explicit learning and hardening review flows

That boundary keeps the release gate focused while allowing improvement loops to remain reviewable and separately governed.

## Source Repository Structure

For contributors, this repository is organized around the product build and its packaged defaults:

- `cmd/rail/` contains the product entrypoint
- `internal/` contains the runtime, request, validation, install, and reporting packages
- `assets/defaults/` contains embedded default harness assets
- `assets/skill/` contains the bundled Rail skill source
- `packaging/` contains release packaging material such as the Homebrew formula
- `docs/` contains architecture, release, and operator documentation
