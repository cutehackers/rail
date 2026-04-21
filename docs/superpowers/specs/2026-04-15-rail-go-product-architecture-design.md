# Rail Go Product Architecture Design

**Date:** 2026-04-15

## Goal

Redesign `rail` from a checkout-driven Dart control repository into an installed Go product with a fast native CLI, built-in Codex skill support, and a project-local `.harness` workspace model.

## Problem Statement

The current installation and runtime model is not aligned with the product that `rail` is trying to be:

- installation is effectively `git clone` of a source repository
- the current skill install flow depends on the local checkout path
- the runtime assumes the control repository checkout is the active runtime root
- users must understand both repository setup and skill setup before first use
- the current Dart CLI shape is workable, but not the best fit for a short-lived control-plane product where startup cost and operational simplicity matter most

The product should instead behave like an installed tool:

- install once
- use from any target repository
- keep project-specific state inside the target repository
- keep product defaults and runtime logic inside the installed product

## Design Summary

- `rail` becomes a Go-based native CLI product.
- distribution is via a package manager such as Homebrew, not `git clone`
- `brew install rail` installs both the `rail` executable and the Rail Codex skill assets
- the Rail skill remains the primary natural-language interface for composing harness requests
- the `rail` CLI becomes the execution engine, validator, and orchestrator
- each target repository owns a local `.harness/` workspace
- product-default harness assets are embedded in the `rail` binary
- project-local `.harness/` files override embedded defaults on a file-by-file basis
- `README.md` is fully rewritten around the installed-product model

## Product Shape

### Installed Product

`rail` is an installed product, not a repository checkout requirement.

The intended user flow is:

```bash
brew install rail
cd /absolute/path/to/target-repo
rail init
```

After installation:

- `rail` is available as a normal shell command
- the Rail Codex skill is already installed and ready to use
- the user does not need to run a separate skill-install command
- the user does not need a local `rail` source checkout just to operate the product

### Source Repository Role

This repository remains the source of truth for:

- Go CLI implementation
- built-in default harness assets
- the bundled Rail skill source
- release packaging and distribution configuration
- architecture, operator, and product documentation

This repository is no longer the required runtime root for end users.

## Runtime Language Decision

The final runtime language is Go.

### Why Go

Go is the best fit for `rail` because:

- `rail` is a short-lived control-plane CLI, not a long-running application server
- the dominant performance concern is command startup latency and process overhead
- `rail` does extensive file I/O, YAML/JSON handling, and subprocess orchestration
- Go supports simple native binary distribution with no runtime dependency
- Go keeps operational complexity lower than a daemonized architecture or a lower-level systems rewrite

### Rejected End States

#### Dart source execution

This is not the target end state because it preserves the current runtime dependency and checkout-oriented product feel.

#### Long-running daemon architecture

This is not the preferred model because:

- it introduces persistent process state
- it complicates resource management
- it adds restart, lock, and stale-state concerns
- the actual `rail` workload benefits more from fast one-shot execution than from an always-on service

#### Rust

Rust may deliver excellent raw performance, but for this product it adds more implementation and maintenance cost than the problem warrants.

## Rail Skill and CLI Contract

The Rail skill remains a core product feature.

It is not downgraded into a trivial launcher.

### Rail Skill Responsibilities

The Rail skill is responsible for:

- accepting natural-language user intent
- inferring request fields
- asking at most minimal clarifying questions when required for safety
- structuring the request into a machine-oriented draft
- preserving the user-facing “describe the goal, constraints, and done criteria” experience

### CLI Responsibilities

The `rail` CLI is responsible for:

- schema validation
- request normalization
- default filling and guardrail enforcement
- writing request files
- bootstrapping artifact state
- actor execution and orchestration
- evaluation routing
- reporting and persistence

### Contract Boundary

The skill has primary authority over natural-language interpretation.

The CLI has final authority over request validity and official harness materialization.

The preferred interface is a request-draft payload, not a long chain of fragile flags. Conceptually:

```bash
rail compose-request --stdin
```

or:

```bash
rail compose-request --input /absolute/path/to/request-draft.json
```

The draft payload should carry fields such as:

- `request_version`
- `project_root`
- `task_type`
- `goal`
- `context`
- `constraints`
- `definition_of_done`
- `risk_tolerance`
- `validation_hints`

## Built-in Skill Installation

The product must not require a second manual skill installation step.

### Product Requirement

`brew install rail` installs:

- the `rail` native binary
- the Rail Codex skill assets
- any product metadata needed for Codex to discover and use the skill

The user must be able to invoke `Use the Rail skill` immediately after product installation.

### Skill Runtime Assumption

The installed skill must not depend on:

- a checked-out `rail` repository
- a hardcoded absolute checkout path
- shell scripts that symlink from a checkout into a user skill directory

The skill should assume only that:

- `rail` is on `PATH`
- the user is operating inside or near a target repository

## Project-Local `.harness` Model

Each target repository owns a local `.harness/` workspace.

This local workspace is the project’s control-plane state and override surface.

It is not a full copy of all product-default harness assets.

### Required Local Content

The minimal scaffold created by `rail init` is:

```text
.harness/
  project.yaml
  requests/
  artifacts/
  learning/
    feedback/
    reviews/
    hardening-reviews/
    approved/
    review_queue.yaml
    hardening_queue.yaml
    family_evidence_index.yaml
```

### Required Ownership Rules

These are always project-local:

- `.harness/project.yaml`
- `.harness/requests/`
- `.harness/artifacts/`
- `.harness/learning/`

These directories and files represent project-specific state, evidence, and reviewed memory. They are never satisfied by global fallback.

### Optional Override Surface

The following paths are optional project-local overrides:

- `.harness/supervisor/policy.yaml`
- `.harness/actors/*`
- `.harness/rules/*`
- `.harness/rubrics/*`
- `.harness/templates/*`

If a project does not define them, the built-in product defaults are used.

## Fallback and Override Semantics

The override model must stay simple and reviewable.

### Resolution Rules

1. If the project-local file exists, use it.
2. Otherwise, use the embedded product default.
3. Override is file-level, not deep merge.
4. Stateful directories remain local-only.

### Why File-Level Override

File-level replacement is preferred because:

- it keeps runtime behavior easy to explain
- it makes override provenance obvious during debugging
- it avoids hidden merge behavior across product upgrades
- it keeps advanced customization explicit rather than clever

Complex deep-merge semantics should not be added to the core product model.

## `rail init`

`rail init` becomes a first-class product command.

Its purpose is to:

- mark a repository as a Rail-managed project
- create the minimal `.harness/` scaffold
- create `project.yaml`
- avoid copying the full default control-plane definition into the target repository

### `project.yaml`

`project.yaml` is the identity and configuration root for a Rail-managed project.

Its minimum required fields should include:

- `schema_version`
- `project_name`
- `rail_compat_version`
- `default_validation_profile`

It may later include:

- repo-specific validation roots and targets
- task family defaults
- explicit local override declarations

## Embedded Product Assets

The Go binary should embed the default control-plane assets that define product behavior.

These embedded assets include:

- default actor instructions
- default supervisor policy and registry
- default rules and rubrics
- default templates and schema definitions
- any init scaffold templates needed for project bootstrap

This allows:

- installation without a companion runtime checkout
- deterministic default behavior across environments
- stable fallback behavior for advanced users who override only a subset of files

## Command Surface

The product should preserve the recognizable operator-facing command model where it still fits:

- `compose-request`
- `validate-request`
- `run`
- `execute`
- `route-evaluation`
- `init`

The user-facing concepts should stay stable even as the implementation moves from Dart to Go.

This reduces migration cost for:

- existing documentation
- the bundled Rail skill
- operator expectations
- future migration validation

## README Rewrite Requirements

`README.md` must be rewritten in full.

It should no longer describe the product as a source checkout runtime.

### New README Responsibilities

The rewritten README must:

- present `rail` as an installed product
- document package-manager-based installation
- document that the Rail skill is installed with the product
- explain `rail init`
- explain the project-local `.harness/` workspace model
- explain that the source repository is for development and contribution, not normal end-user runtime setup
- document the built-in-default plus project-override model

### Advanced Overrides Section

The README must include an advanced-user section that explains:

- which parts of `.harness` may be overridden locally
- when overrides are appropriate
- how override precedence works
- that override is file-level rather than deep merge
- that state directories remain project-local only
- that advanced overrides create local ownership for drift across product upgrades

The README should explicitly frame overrides as an advanced feature, not the default onboarding path.

## Migration Strategy

Migration should preserve product meaning while replacing implementation and distribution.

### Phase 1: Freeze Product Architecture

Document and approve:

- Go as the target runtime
- package-manager install model
- built-in skill installation
- project-local `.harness` workspace model
- embedded defaults with file-level override

### Phase 2: Build Go CLI Surface

Recreate the core command surface in Go:

- `compose-request`
- `validate-request`
- `run`
- `execute`
- `route-evaluation`
- `init`

### Phase 3: Implement Asset Embed and Override Loader

Implement:

- embedded default control-plane assets
- project-local override resolution
- project-local state handling

### Phase 4: Move Skill to Installed-Product Model

Update the bundled Rail skill so it:

- targets the installed `rail` binary
- uses request-draft composition
- no longer refers to the current checkout as runtime root

### Phase 5: Replace Checkout-Based Install Docs and Scripts

Remove or retire:

- checkout-based installation instructions
- path-dependent skill symlink scripts
- runtime assumptions tied to a source checkout

## Non-Negotiable Product Invariants

The redesign must preserve these invariants:

- the Rail skill remains the primary natural-language interface
- supervisor behavior stays explicit and reviewable
- artifact outputs remain traceable
- project-local `.harness` ownership remains visible
- request, artifact, and learning state stay concrete and inspectable
- advanced customization remains explicit, narrow, and file-based

## Completion Criteria

This redesign is considered implemented when all of the following are true:

- `rail` is shipped as a Go native binary
- end users can install `rail` without cloning the repository
- the Rail skill is installed as part of the product install path
- `rail init` creates the minimal local `.harness` scaffold
- project-local `.harness` state works without copying the full default harness tree
- built-in defaults are embedded in the binary
- local overrides take precedence over embedded defaults on a file-by-file basis
- `README.md` is fully rewritten to match the installed-product model
- no user-facing documentation describes the source checkout as the normal runtime installation path
