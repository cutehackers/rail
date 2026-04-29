# AGENTS.md

## Purpose

`rail` is the skill-first harness control-plane repository and the source of the bundled Rail Codex skill.

The product behavior to preserve is:

- users describe work in natural language through the Rail skill
- the Rail skill turns that request into the correct Python request draft
- Rail executes a bounded Actor Runtime workflow against a separate target repository

This repository does not contain the downstream application under change. It owns the Python Rail Harness Runtime, supervisor and actor configuration under `.harness/`, bundled defaults, tests, examples, and Rail skill behavior.

## What To Change

- Public Python API: `src/rail/api.py`
- Harness behavior and policy: `.harness/`
- Actor Runtime: `src/rail/actor_runtime/`
- Supervisor, routing, evidence, and projection: `src/rail/supervisor/`, `src/rail/evaluator/`, `src/rail/artifacts/`
- Repo-owned skill: `skills/rail/SKILL.md`
- Bundled skill copy: `assets/skill/Rail/`
- Project docs, plans, and specs: `docs/`
- Canonical product contract: `docs/SPEC.md`
- Example target-repo conventions: `examples/`

Prefer changing the smallest set of files that actually own the behavior.

## Working Rules

- Do not treat this as a Flutter app repo.
- Do not implement downstream product code here. Changes here should affect request composition, validation, orchestration, routing, reporting, skills, or harness policy.
- Before changing Python code, tests, or product-facing docs, read `docs/ARCHITECTURE.md` and `docs/CONVENTIONS.md` and follow their boundaries and naming rules.
- Treat the Rail skill as a first-class product surface. If a change affects how users express work, verify whether `skills/rail/` and `assets/skill/Rail/` must change with it.
- Keep the end-user contract skill-first. Normal users should not need to know request YAML or wrapper details.
- Preserve the `.harness/` layout. It is part of the product, not incidental config.
- Keep supervisor behavior explicit and reviewable. Favor deterministic routing and traceable outputs over clever automation.
- Documentation security rule: do not include a user's home directory path in docs or examples. Replace home-folder paths with placeholders such as `/absolute/path/to/...`.
- Avoid editing generated artifacts in `.harness/artifacts/` unless the task is specifically about fixtures, evidence, or checked-in examples.
- Avoid editing `.worktrees/` or `.git/worktrees/` content from the main worktree.
- Treat untracked patch rejects or stale migration leftovers as local artifacts unless the task explicitly asks you to inspect or resolve them.

## Core Commands

Run all commands from the repo root.

```bash
uv run --python 3.12 pytest -q
uv run --python 3.12 ruff check src tests
uv run --python 3.12 mypy src/rail
```

Use focused pytest targets while developing, then run the full checks before claiming completion.

## Editing Guidance

- Keep request and artifact schemas aligned with actor expectations.
- Follow `docs/CONVENTIONS.md` for Python naming, module boundaries, typed data contracts, validation behavior, and test placement.
- When changing routing behavior, also update the relevant policy or evaluator guidance under `.harness/`.
- When changing the repo-owned skill, keep it aligned with the Python API workflow.
- Prefer extending existing docs in `docs/` when behavior, launch criteria, or operator expectations change.
- Keep constraints and definitions of done concrete and testable. Avoid vague policy text.
- Target mutation must flow through Rail-validated patch bundles and evaluator gates.

## Validation Expectations

Use the lightest validation that proves the change.

- For API, schema, or runtime changes, run focused tests for the touched package and then the full Python test suite.
- For request-shape changes, validate via `rail.normalize_request(...)` or a focused request test.
- For routing or artifact changes, exercise the smallest relevant supervisor flow and inspect the produced artifact output.
- If a task changes documented launch behavior or supervisor outcomes, update the matching docs and skill references.

## Repo Map

- `README.md`: operator-facing overview and quick start
- `docs/SPEC.md`: canonical product contract and release-ready criteria
- `docs/ARCHITECTURE.md`: product architecture and runtime boundaries
- `docs/CONVENTIONS.md`: Python Rail Harness coding conventions
- `src/rail/api.py`: public Python API
- `src/rail/request/`: request normalization and schema
- `src/rail/artifacts/`: artifact identity, storage, resume policy, and projections
- `src/rail/actor_runtime/`: SDK-backed Actor Runtime boundary
- `src/rail/supervisor/`: supervisor loop and routing
- `src/rail/evaluator/`: evaluator gate and evidence checks
- `src/rail/workspace/`: sandbox, patch bundle, apply, and validation evidence
- `.harness/actors/`: actor instructions
- `.harness/supervisor/`: routing and policy configuration
- `.harness/templates/`: schema definitions
- `.harness/rules/` and `.harness/rubrics/`: guardrails and evaluation criteria
- `.harness/requests/`: checked-in request examples and fixtures
- `.harness/learning/`: review-only learning and hardening state
- `skills/rail/`: repo-owned Rail skill and references
- `assets/skill/Rail/`: bundled Rail skill shipped with the installed product

## When In Doubt

- Read `README.md` first for the intended operator workflow.
- Read `docs/ARCHITECTURE.md` and `docs/CONVENTIONS.md` before changing Rail runtime code or tests.
- If the change touches request authoring UX, read the Rail skill before editing runtime behavior.
- Read the nearest schema, actor, or supervisor file before changing behavior.
- Keep changes conservative: explicit, reviewable, and easy to trace from request to supervisor outcome.
