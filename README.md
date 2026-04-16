# rail

`rail` is an installed harness control-plane for Codex. It turns a natural-language engineering request into a bounded, reviewable workflow that runs against a separate target repository.

The product model is simple:

- install `rail` once as a normal CLI
- use the bundled Rail skill from Codex
- keep project-specific state inside the target repository
- rely on embedded product defaults unless the project overrides them

This source repository is the development and release origin for Rail. It is not the required runtime root for end users.

## Install

Install Rail as a packaged product:

```bash
brew install rail
```

The packaged install includes:

- the `rail` CLI
- the built-in Rail Codex skill
- embedded default harness assets used at runtime

There is no separate manual skill-install step for end users.

## First Use

Initialize Rail inside the repository you want to operate on:

```bash
cd /absolute/path/to/target-repo
rail init
```

`rail init` creates the minimal project-local `.harness/` workspace that Rail needs for requests, artifacts, and reviewed learning state.

After initialization, a typical Rail workflow starts from the bundled Codex skill:

```text
Use the Rail skill.
Target repo: /absolute/path/to/target-repo
Goal: Fix the intermittent profile refresh loading issue.
Constraint: Do not change the API contract.
Definition of done: refresh completes reliably and related tests still pass.
```

The skill structures the request. The `rail` CLI validates it, materializes the workflow, and records reviewable outputs.

## Project-Local `.harness`

Each target repository owns its own `.harness/` directory. This is the project-local control-plane surface, not a global shared checkout.

The local workspace holds:

- `.harness/project.yaml`
- `.harness/requests/`
- `.harness/artifacts/`
- `.harness/learning/`

These paths always belong to the target repository because they capture project-specific configuration, execution history, evidence, and reviewed memory.

Rail also ships embedded defaults for reusable control-plane assets such as:

- actor instructions
- supervisor policy
- rules and rubrics
- request and artifact templates

If a project does not override those files locally, Rail resolves them from the embedded defaults in the installed product.

## Advanced Overrides

Advanced users can override selected control-plane files in the target repository without forking the product install.

Supported override areas include:

- `.harness/supervisor/`
- `.harness/actors/`
- `.harness/rules/`
- `.harness/rubrics/`
- `.harness/templates/`

Override precedence is explicit:

1. If a project-local `.harness` file exists, Rail uses it.
2. Otherwise Rail falls back to the embedded defaults bundled with the installed product.
3. Override resolution is a file-level override, not a deep merge.
4. Stateful project data stays project-local even when no override exists.

The file-level override rule is deliberate. It keeps provenance obvious during debugging, makes upgrades reviewable, and avoids hidden merge behavior between project customizations and packaged defaults.

Example:

- `.harness/supervisor/policy.yaml` present: use the project version
- `.harness/supervisor/policy.yaml` absent: use the embedded default
- `.harness/artifacts/`: always local project state, never satisfied by global fallback

The architecture details behind embedded defaults, project-local `.harness`, and file-level override behavior are documented in [docs/ARCHITECTURE.md](/Users/junhyounglee/workspace/rail/docs/ARCHITECTURE.md).

## What Rail Owns

Rail owns the control-plane, not the downstream product under change.

In practice that means Rail owns:

- request composition and validation
- artifact bootstrap and reporting
- supervisor policy and actor routing
- evaluation and bounded corrective loops
- reviewable learning and hardening workflows

The target repository remains the place where application code is inspected, changed, and validated.

## Source Repository Role

This repository exists for contributors and release engineers who are developing Rail itself.

It is the source of truth for:

- the Go CLI implementation
- embedded default assets under `assets/`
- the bundled Rail skill source
- packaging and release configuration
- architecture, release, and operator documentation

You do not need a local checkout of this repository to use Rail as a product. You only need an installed `rail` binary and a target repository with a project-local `.harness/` workspace.

## More Detail

- System architecture: [docs/ARCHITECTURE.md](/Users/junhyounglee/workspace/rail/docs/ARCHITECTURE.md)
- Korean architecture guide: [docs/ARCHITECTURE-kr.md](/Users/junhyounglee/workspace/rail/docs/ARCHITECTURE-kr.md)
- Release contracts: [docs/releases/](/Users/junhyounglee/workspace/rail/docs/releases)
- Active release tasks: [docs/tasks.md](/Users/junhyounglee/workspace/rail/docs/tasks.md)
