# rail

`rail` is an installed control-plane for Codex. It turns a natural-language engineering request into a bounded, reviewable harness workflow that runs against a separate target repository.

Rail is not the app under change. Rail owns the workflow, policy, artifacts, and reviewed learning state around that app.

## Product Model

Rail is designed to be used as a normal installed tool:

- install `rail` once
- use the bundled Rail skill from Codex
- keep project-specific state inside the target repository
- rely on embedded defaults unless the project overrides them locally

The source repository is the development and release origin for Rail. It is not the required runtime root for end users.

## Install

Install Rail as a packaged product:

```bash
brew install rail
```

The packaged install includes:

- the `rail` CLI
- the bundled Rail Codex skill
- embedded default harness assets

There is no separate manual Codex skill install step for end users.

## Quick Start

Initialize Rail inside the repository you want to operate on:

```bash
cd /absolute/path/to/target-repo
rail init
```

`rail init` creates the minimal project-local `.harness/` workspace:

- `.harness/project.yaml`
- `.harness/requests/`
- `.harness/artifacts/`
- `.harness/learning/`

After that, the normal entrypoint is the bundled Rail skill in Codex:

```text
Use the Rail skill.
Target repo: /absolute/path/to/target-repo
Goal: Fix the intermittent profile refresh loading issue.
Constraint: Do not change the API contract.
Definition of done: refresh completes reliably and related tests still pass.
```

The skill structures the request. Rail then materializes the workflow, validates it, and records reviewable outputs in the target repository.

## Standard Workflow

The common operator flow is:

1. initialize a target repository with `rail init`
2. draft a request with the Rail skill, `rail compose-request`, or `rail init-request`
3. validate the request with `rail validate-request`
4. bootstrap an artifact with `rail run`
5. execute the actor workflow with `rail execute`
6. if needed, refresh or re-run routing with `rail route-evaluation`
7. for `v2`, produce an integration handoff and reviewed learning updates

Direct CLI flow example:

```bash
rail init --project-root /absolute/path/to/target-repo

cat /absolute/path/to/request-draft.json | rail compose-request --stdin

rail validate-request --request /absolute/path/to/target-repo/.harness/requests/request.yaml

rail run \
  --request /absolute/path/to/target-repo/.harness/requests/request.yaml \
  --project-root /absolute/path/to/target-repo

rail execute \
  --artifact /absolute/path/to/target-repo/.harness/artifacts/request
```

## Command Surface

Rail currently exposes these operator commands:

- `init`
  - create the project-local `.harness` workspace
- `init-request`
  - write a request template into the current workspace
- `compose-request`
  - normalize a JSON request draft into `.harness/requests/request.yaml`
- `validate-request`
  - validate a request against Rail request schema
- `run`
  - bootstrap an artifact directory for a request
- `execute`
  - run the bounded actor chain for an artifact
- `route-evaluation`
  - re-evaluate or refresh persisted routing outputs for an artifact
- `validate-artifact`
  - validate an artifact file against a named schema
- `integrate`
  - produce the `v2` integration handoff after a passing run
- `init-user-outcome-feedback`
  - create a draft user outcome feedback file from an artifact
- `init-learning-review`
  - create a draft learning review file from a candidate
- `init-hardening-review`
  - create a draft hardening review file from a candidate
- `apply-user-outcome-feedback`
  - apply a reviewed feedback file and refresh derived state
- `apply-learning-review`
  - apply a reviewed learning decision and refresh derived state
- `apply-hardening-review`
  - apply a reviewed hardening decision and refresh derived state
- `verify-learning-state`
  - verify that derived learning state is coherent

## V1 And V2

Rail keeps the release surface explicit.

### V1

`v1` is the bounded core supervisor gate. It focuses on:

- request normalization
- artifact bootstrap
- deterministic actor execution
- evaluator-driven bounded retries
- explicit terminal outcomes

### V2

`v2` extends `v1` with:

- integration handoff generation
- explicit user outcome feedback files
- explicit learning review files
- explicit hardening review files
- derived learning-state verification

The important design choice is that review artifacts are operator-authored, while queues, evidence indexes, and approved family memory are Rail-derived state.

## Project-Local `.harness`

Each target repository owns its own `.harness/` directory. This is project-local state, not a shared global checkout.

Project-local state is where Rail stores:

- project identity
- requests
- artifacts
- reviewed feedback
- reviewed learning decisions
- approved family memory

These paths remain local because they are project-specific evidence, not reusable product defaults.

## Embedded Defaults And Overrides

Rail ships embedded defaults for reusable control-plane assets such as:

- supervisor policy
- actor instructions
- rules and rubrics
- request and artifact templates

Advanced users can override selected defaults by adding full files under the target repository:

- `.harness/supervisor/`
- `.harness/actors/`
- `.harness/rules/`
- `.harness/rubrics/`
- `.harness/templates/`

Override precedence is explicit:

1. if a project-local file exists, Rail uses it
2. otherwise Rail falls back to the embedded default
3. overrides are file-level, not deep merges
4. stateful paths such as `.harness/artifacts/`, `.harness/learning/`, and `.harness/requests/` are always local

This keeps provenance reviewable and avoids hidden merge behavior during upgrades.

## Bundled Rail Skill

The bundled Rail skill is the normal natural-language entrypoint for Codex.

Its role is to:

- interpret the user goal, constraints, and definition of done
- infer a structured request draft
- ask for clarification only when a missing field would make the request unsafe
- hand the normalized draft to `rail compose-request`

The user should not need to hand-author harness YAML for normal use.

## Source Repository Role

This repository exists for contributors and release engineers working on Rail itself.

It is the source of truth for:

- the Go CLI implementation
- embedded default harness assets under `assets/defaults/`
- the bundled Rail skill source under `assets/skill/`
- repo-owned skill sources under `skills/`
- release tooling and packaging
- architecture and release documentation

You do not need a checkout of this repository to use Rail as a product. You need an installed `rail` binary and a target repository with project-local `.harness` state.

## More Detail

- architecture: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)
- Korean architecture guide: [docs/ARCHITECTURE-kr.md](docs/ARCHITECTURE-kr.md)
- release docs: [docs/releases/](docs/releases/)
- active release tasks: [docs/tasks.md](docs/tasks.md)
