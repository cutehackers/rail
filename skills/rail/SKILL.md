---
name: rail
description: Bootstrap and execute the rail harness workflow against a target Flutter/Riverpod repository from a natural-language task request.
---

# Rail

Use this skill from the `rail` control repository:

- `/Users/junhyounglee/workspace/rail`

The target application repository is not this repo. It is passed as `--project-root`.

## Purpose

`rail` is the user-facing entrypoint for the local harness runtime.

The user should be able to describe the task in natural language.
You then:

1. infer the harness request fields
2. ask at most one concise clarification only if a missing field makes the request unsafe
3. write the structured request file
4. validate the request
5. bootstrap the workflow against the target repo
6. summarize the artifact location and next actor step

Do not make the user write YAML by hand unless they explicitly ask to.

## Required runtime assumption

You must know the target repository root.

If the user has not already made it explicit, ask for the target repo path once.

## Infer These Fields

Convert the user request into:

- `task_type`
- `goal`
- `context`
- `constraints`
- `definition_of_done`
- `risk_tolerance`

Defaults:

- `bug_fix`, `feature_addition`, `test_repair` -> `risk_tolerance=low`
- `safe_refactor` -> `risk_tolerance=medium`
- `definition_of_done` should include:
  - observable requested behavior
  - related test expectation
  - analyze expectation

## Commands

From `/Users/junhyounglee/workspace/rail`, run:

```bash
dart run bin/rail.dart compose-request --goal <goal> --task-type <task_type> ...
dart run bin/rail.dart validate-request --request <request-file>
dart run bin/rail.dart run --request <request-file> --project-root <target-repo>
```

Only use `--force` when the user explicitly wants to overwrite an existing artifact.

## Compose Request

Prefer `compose-request` over manually writing YAML.

Map inferred fields to:

- `--task-type`
- `--goal`
- `--feature`
- `--suspected-file`
- `--related-file`
- `--validation-root`
- `--validation-target`
- `--constraint`
- `--dod`
- `--risk-tolerance`
- `--validation-profile`
- `--priority`

If the user did not give reliable file hints, omit them instead of inventing them.

Use `--validation-profile smoke` only for harness smoke or control-plane verification tasks where full target-repo lint/test would be disproportionate.
In `smoke` mode, rail may satisfy actor execution through deterministic control-plane outputs instead of full nested actor generation.
For `standard` mode, prefer passing `--validation-root` and `--validation-target` when you already know the affected package or test path so executor validation stays narrow.

## Output To User

After bootstrapping, report:

- inferred `task_type`
- created request file
- target project root
- generated artifact directory
- defaults that were applied
- that bootstrap is complete and actor execution is a separate step unless you were asked to continue

## Guardrails

- Do not claim the task is implemented just because the workflow bootstrapped.
- Do not invent constraints the user did not state.
- Keep `definition_of_done` testable.
- Keep `constraints` short and concrete.

For examples, see:

- `references/examples.md`
