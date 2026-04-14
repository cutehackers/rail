# rail

`rail` is the standalone control-plane for the `v1 core supervisor gate`. It turns a user request into a bounded supervisor run against an external target repository, while keeping the runtime, schemas, prompts, and artifacts versioned in one place.

The application under change does not live in this repository. `rail` operates on a target repo passed through `--project-root`.

See [docs/releases/v1-core-supervisor-gate.md](docs/releases/v1-core-supervisor-gate.md) for the `v1` release contract, [docs/releases/v2-integrator-and-learning-gate.md](docs/releases/v2-integrator-and-learning-gate.md) for the `v2` gate, and [docs/backlog/v2-integrator-and-learning.md](docs/backlog/v2-integrator-and-learning.md) for the deferred backlog.

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
Use the Rail skill from your local Rail checkout.
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

## V2 Review Workflow

`v2` adds explicit file-based learning review commands. The operator flow is:

1. generate a schema-valid draft from an artifact or candidate
2. edit the generated YAML with real reviewer intent
3. apply the reviewed file back into the learning store

Draft commands:

```bash
dart run bin/rail.dart init-user-outcome-feedback \
  --artifact .harness/artifacts/profile-refresh-fix

dart run bin/rail.dart init-learning-review \
  --candidate .harness/artifacts/profile-refresh-fix/quality_learning_candidates/<candidate>.yaml

dart run bin/rail.dart init-hardening-review \
  --candidate .harness/artifacts/profile-refresh-fix/hardening_candidates/<candidate>.yaml
```

Default draft locations:

- user outcome feedback: `.harness/learning/feedback/`
- learning review drafts: `.harness/learning/reviews/`
- hardening review drafts: `.harness/learning/hardening-reviews/`

Rail-derived state lives alongside those drafts and is not edited by hand:

- queue snapshots: `.harness/learning/review_queue.yaml` and `.harness/learning/hardening_queue.yaml`
- family evidence snapshot: `.harness/learning/family_evidence_index.yaml`
- approved memory: `.harness/learning/approved/<task_family>.yaml`

Apply commands:

```bash
dart run bin/rail.dart apply-user-outcome-feedback \
  --file .harness/learning/feedback/<draft>.yaml

dart run bin/rail.dart apply-learning-review \
  --file .harness/learning/reviews/<draft>.yaml

dart run bin/rail.dart apply-hardening-review \
  --file .harness/learning/hardening-reviews/<draft>.yaml
```

`--file` is the only supported apply flag for the file-based review commands.

When a same-family review is promoted, rail overwrites the single active approved file for that family. Previous approved content stays in git history; there are no archive files for older baselines.

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
- review drafts are operator-authored, while queue, evidence, and approved-memory files are rail-derived snapshots
- approved memory is same-family only, with one active approved file per family
- pending review backlog is allowed, but broken derived state is not

The `v1` scope is intentionally constrained and does not treat these as in-gate:

- parallel actor orchestration
- project-specific adapters beyond the default Flutter + Riverpod profile
- `integrator`
- quality-learning review and apply flows
- hardened end-to-end validation across all task types

The `v2` operator surface now includes explicit file-based `init-*` and `apply-*` learning workflows. Operators edit the draft and decision files; rail regenerates the queue and evidence snapshots, and updates approved memory only on `promote` at `.harness/learning/approved/<task_family>.yaml`.
