# AGENTS.md

## Purpose

`rail` is the harness control-plane repository. It owns the local runtime CLI, the supervisor and actor configuration under `.harness/`, and the repo-local Rail skill definition. It does not contain the downstream application under change. When a workflow operates on an app, that app is passed separately with `--project-root`.

## What To Change

- Runtime entrypoint: `bin/rail.dart`
- Harness behavior and policy: `.harness/`
- Repo-owned skill: `skills/Rail/SKILL.md`
- Project docs, plans, and specs: `docs/`
- Example target-repo conventions: `examples/`

Prefer changing the smallest set of files that actually own the behavior.

## Working Rules

- Do not treat this as a Flutter app repo. It is a Dart CLI control-plane project.
- Do not implement downstream product code here. Changes here should affect request composition, validation, orchestration, routing, reporting, skills, or harness policy.
- Preserve the current file layout. The hidden `.harness/` tree is part of the product, not incidental config.
- Keep supervisor behavior explicit and reviewable. Favor deterministic routing and traceable outputs over clever automation.
- Documentation security rule: **DO NOT** include a user's home directory path in docs or examples. Treat any home-folder path like `/Users/<name>/...` or `~/<...>` in documentation as a warning-level lint issue and replace it with a sanitized placeholder such as `/absolute/path/to/...`.
- Avoid editing generated artifacts in `.harness/artifacts/` unless the task is specifically about fixtures, evidence, or checked-in examples.
- Avoid editing `.worktrees/` or `.git/worktrees/` content from the main worktree.
- Treat untracked patch rejects like `bin/rail.dart.rej` as local leftovers unless the task explicitly asks you to inspect or resolve them.

## Core Commands

Run all commands from the repo root.

```bash
dart pub get
dart run bin/rail.dart compose-request --goal "<goal>" --task-type <task_type>
dart run bin/rail.dart validate-request --request .harness/requests/<file>.yaml
dart run bin/rail.dart run --request .harness/requests/<file>.yaml --project-root /absolute/path/to/target-repo
dart run bin/rail.dart execute --artifact .harness/artifacts/<task-id>
dart run bin/rail.dart route-evaluation --artifact .harness/artifacts/<task-id>
```

If you need command syntax, `bin/rail.dart` contains the authoritative usage text.

## Editing Guidance

- Keep request and artifact schemas aligned with actor expectations.
- When changing routing behavior, also update the relevant policy or evaluator guidance under `.harness/`.
- When changing the repo-owned skill, keep it aligned with the runtime flags and current workflow.
- Prefer extending existing docs in `docs/` when behavior, launch criteria, or operator expectations change.
- Keep constraints and definitions of done concrete and testable. Avoid vague policy text.

## Validation Expectations

Use the lightest validation that proves the change.

- For CLI or schema changes, run the relevant `dart run bin/rail.dart ...` command against an existing request or fixture.
- For request-shape changes, validate a real request file with `validate-request`.
- For routing or artifact changes, exercise the smallest relevant harness flow and inspect the produced artifact output.
- If a task changes documented launch behavior or supervisor outcomes, update the matching docs in `docs/tasks.md` or `docs/superpowers/`.

## Repo Map

- `README.md`: operator-facing overview and quick start
- `bin/rail.dart`: CLI surface and runtime wiring
- `.harness/actors/`: actor instructions
- `.harness/supervisor/`: routing and policy configuration
- `.harness/templates/`: YAML schema definitions
- `.harness/rules/` and `.harness/rubrics/`: guardrails and evaluation criteria
- `.harness/requests/`: checked-in request examples and fixtures
- `.harness/learning/`: review-only learning and hardening state
- `skills/Rail/`: user-facing Rail skill and references

## When In Doubt

- Read `README.md` first for the intended operator workflow.
- Read the nearest schema, actor, or supervisor file before changing behavior.
- Keep changes conservative: explicit, reviewable, and easy to trace from request to supervisor outcome.
