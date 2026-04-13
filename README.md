# rail

`rail` is the standalone control-plane for the `v1 core supervisor gate`. It turns a user request into a bounded supervisor run against an external target repository, while keeping the runtime, schemas, prompts, and artifacts versioned in one place.

The application under change does not live in this repository. `rail` operates on a target repo passed through `--project-root`.

See [docs/releases/v1-core-supervisor-gate.md](docs/releases/v1-core-supervisor-gate.md) for the release contract and [docs/backlog/v2-integrator-and-learning.md](docs/backlog/v2-integrator-and-learning.md) for the deferred `v2` backlog.

## Install

Requirements:

- Dart SDK `^3.9.0`

Setup:

```bash
git clone <repo-url> rail
cd rail
dart pub get
dart run bin/rail.dart help
```

## What You Can Do With `rail`

From a user perspective, `rail` lets you:

- compose a structured request from a plain-language goal
- validate the request before running the supervisor
- bootstrap a task artifact set for a target repository
- execute the `planner -> context_builder -> generator -> executor -> evaluator` gate
- apply bounded evaluator feedback through deterministic routing
- keep requests, traces, and terminal artifacts in one dedicated control repository

Today, the default target profile is the built-in Flutter + Riverpod workflow.

## Project Structure

- `bin/rail.dart`: CLI entrypoint
- `lib/src/cli/`: command parsing and usage surface
- `lib/src/runtime/`: harness runtime and supervisor execution flow
- `.harness/`: actor briefs, supervisor rules, rubrics, schemas, and templates
- `skills/rail/SKILL.md`: repo-owned skill entrypoint
- `docs/`: release contracts, backlog, and design records
- `examples/`: example target-repo conventions and usage material

## Using The Codex Skill

If you use Codex, the repo-owned `Rail` skill is the highest-leverage entrypoint. Its main benefit is that it converts a natural-language task request into a structured harness request automatically, instead of making the user hand-author YAML or remember every CLI flag.

In practice, the skill will:

- infer request fields such as `task_type`, `goal`, `constraints`, `definition_of_done`, and `risk_tolerance`
- ask for at most one clarification only when the request is unsafe without it
- compose the request file with `compose-request`
- validate it
- bootstrap the supervisor workflow against `--project-root`

Example Codex prompt:

```text
Use the Rail skill from /Users/junhyounglee/workspace/rail.
Target repo: /absolute/path/to/target-repo
Task: Fix the intermittent profile refresh loading issue.
Constraint: Do not change the API contract.
Definition of done: pull-to-refresh no longer gets stuck, related tests pass, analyze stays clean.
```

This is the key user benefit of `rail`: the operator can stay at the level of intent, while the skill writes the structured request and kicks off the correct `v1` supervisor gate workflow.

## Example Workflow

Compose a request from a goal:

```bash
dart run bin/rail.dart compose-request \
  --goal "Fix intermittent profile refresh loading issue" \
  --task-type bug_fix \
  --feature profile
```

Validate the generated request:

```bash
dart run bin/rail.dart validate-request \
  --request .harness/requests/<generated>.yaml
```

Bootstrap the supervisor gate for a target repository:

```bash
dart run bin/rail.dart run \
  --request .harness/requests/<generated>.yaml \
  --project-root /absolute/path/to/target-repo \
  --task-id profile-refresh-fix
```

Execute the gate:

```bash
dart run bin/rail.dart execute \
  --artifact .harness/artifacts/profile-refresh-fix
```

If the run has already passed and you want a merge-ready handoff summary, run the explicit post-pass integrator. This step is outside the `v1` release gate.

```bash
dart run bin/rail.dart integrate \
  --artifact .harness/artifacts/profile-refresh-fix
```

Useful variations:

- start from a template with `dart run bin/rail.dart init-request`
- narrow standard validation with `--validation-root` and `--validation-target`
- use `--validation-profile smoke` for smoke-only executor validation
- inspect the next bounded action with `dart run bin/rail.dart route-evaluation --artifact <path>`
- produce a post-pass handoff summary with `dart run bin/rail.dart integrate --artifact <path>`

`run` records the target repo path in the artifact workflow, so `execute` can usually omit `--project-root`.

## Skill Installation

This repo includes a repo-owned skill at `skills/rail/SKILL.md`.

If you want to expose it through Codex global skills, use the installer script:

```bash
./scripts/install_skill.sh
```

That script creates `~/.codex/skills/rail` and installs a symlink to the repo-owned skill.

If you prefer to do the same step manually:

```bash
mkdir -p ~/.codex/skills/rail
ln -sfn "$(pwd)/skills/rail/SKILL.md" ~/.codex/skills/rail/SKILL.md
```

The important point is that the source of truth stays in this repository.

## Current scope

The `v1` runtime supports:

- request composition and validation
- artifact bootstrap
- actor brief generation
- sequential actor execution with `codex exec`
- evaluator-driven `revise` handling back to generator
- validation profiles (`standard`, `smoke`) for executor planning
- deterministic smoke fast-path execution for planner/context_builder/generator/executor/evaluator
- request-level validation roots and targets for narrowing standard-profile executor scope
- supervisor action loops that can route from evaluator back to generator, context_builder, or executor with bounded budgets

The `v1` runtime does not yet provide:

- parallel actor orchestration
- project-specific adapters beyond the default Flutter + Riverpod profile
- `integrator`
- quality-learning review and apply flows
- hardened end-to-end validation across all task types
