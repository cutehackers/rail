# rail

`rail` is a standalone harness control-plane repository.

It owns:

- the harness runtime CLI
- the supervisor/actor/rubric/rule/schema bundle under `.harness/`
- the repo-local `rail` skill definition

It does not contain the application under change.
The actual target repository is supplied with `--project-root`.

## What this solves

- removes harness logic from an app repository
- makes `rail` versioned as one product
- decouples the user-facing skill from a single hardcoded repo
- keeps requests and artifacts in one dedicated control repository

## Project layout

- `bin/rail.dart`: runtime entrypoint
- `.harness/`: supervisor config, actor docs, rubrics, rules, templates
- `skills/rail/SKILL.md`: repo-owned user entrypoint
- `docs/`: design and migration documents
- `examples/`: example usage and target repo conventions

## Quick start

1. `cd /Users/junhyounglee/workspace/rail`
2. `dart pub get`
3. Compose a request:
   - `dart run bin/rail.dart compose-request --goal "fix intermittent profile refresh loading issue" --task-type bug_fix --feature profile`
4. Validate it:
   - `dart run bin/rail.dart validate-request --request .harness/requests/<generated>.yaml`
5. Bootstrap the workflow against a target repo:
   - `dart run bin/rail.dart run --request .harness/requests/<generated>.yaml --project-root /absolute/path/to/target-repo`
6. Execute actors:
   - `dart run bin/rail.dart execute --artifact .harness/artifacts/<task-id>`

`run` records the target repo path in the workflow, so `execute` can usually omit `--project-root`.

## Skill installation

This repo includes a repo-owned skill at `skills/rail/SKILL.md`.

If you want to expose it through Codex global skills, install a symlink:

- `mkdir -p ~/.codex/skills/rail`
- `ln -sfn /Users/junhyounglee/workspace/rail/skills/rail/SKILL.md ~/.codex/skills/rail/SKILL.md`

You can replace the symlink with your own installer later. The important point is that the source of truth stays in this repo.

## Current scope

The runtime supports:

- request composition and validation
- artifact bootstrap
- actor brief generation
- sequential actor execution with `codex exec`
- evaluator-driven `revise` handling back to generator

The runtime does not yet provide:

- parallel actor orchestration
- project-specific adapters beyond the default Flutter + Riverpod profile
- hardened end-to-end validation across all task types

