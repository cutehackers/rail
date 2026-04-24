---
name: Rail
description: Use when bootstrapping and executing the Rail harness workflow against a target application repository from a natural-language task request.
---

# Rail

Use this skill through the installed `rail` binary.

The target application repository is not the Rail source repository. It is carried in the request draft as `project_root`.

## Purpose

`rail` is the user-facing entrypoint for the local harness runtime.

The user should be able to describe the task in natural language.
You then:

1. infer the harness request fields
2. ask at most one concise clarification only if a missing field makes the request unsafe
3. emit a request draft
4. materialize the normalized request with `rail compose-request`
5. report the created request file and applied defaults
6. hand off validation and bootstrap as later workflow steps

Do not make the user write YAML by hand unless they explicitly ask to. Natural-language interpretation remains the primary UX; the CLI validates and materializes the official request file.

## Required runtime assumption

You must know the target repository root.

If the user has not already made it explicit, ask for the target repo path once.

## Infer These Fields

Convert the user request into:

- `request_version`
- `project_root`
- `task_type`
- `goal`
- `context`
- `constraints`
- `definition_of_done`
- `risk_tolerance`
- `validation_profile` when the user explicitly asks for `smoke` mode

Defaults:

- `request_version=1`
- `bug_fix`, `feature_addition`, `test_repair` -> `risk_tolerance=low`
- `safe_refactor` -> `risk_tolerance=medium`
- default execution mode is `real`, which normalizes to `validation_profile=standard`
- use `validation_profile=smoke` only when the user explicitly asks for smoke mode, harness-only verification, or fast control-plane proof
- `definition_of_done` should include:
  - observable requested behavior
  - related test expectation
  - analyze expectation

## Draft Contract

Emit a draft like:

```json
{
  "request_version": "1",
  "project_root": "/absolute/path/to/target-repo",
  "task_type": "bug_fix",
  "goal": "Describe the requested outcome",
  "context": {
    "feature": "profile",
    "suspected_files": [
      "internal/profile/service.go"
    ],
    "related_files": [],
    "validation_roots": [],
    "validation_targets": []
  },
  "constraints": [
    "Short concrete constraint"
  ],
  "definition_of_done": [
    "Observable requested behavior",
    "Related test expectation",
    "Analyze expectation"
  ],
  "risk_tolerance": "low",
  "validation_profile": "standard"
}
```

`project_root` is required. Reject unknown draft fields instead of inventing them.

Only emit fields you can support from the user request or a single safety clarification. Keep `context`, `constraints`, and `definition_of_done` concise and concrete.
Omit `validation_profile` unless the user explicitly wants `smoke`; `rail` will default to `real`/`standard`.

## Commands

Use the installed binary:

```bash
rail compose-request --stdin
rail compose-request --input /absolute/path/to/request-draft.json
rail status --artifact /absolute/path/to/target-repo/.harness/artifacts/<task-id>
```

When you need to refer to paths in explanations or examples, use placeholders such as `/absolute/path/to/request-draft.json` and `/absolute/path/to/target-repo` instead of machine-specific home-directory paths.

## Compose Request

Prefer `compose-request` over manually writing YAML.

The preferred flow is:

1. infer the draft from the natural-language request
2. send the draft to `rail compose-request --stdin` or save it and pass `--input`
3. let `rail` normalize defaults and write the request file under the target repo

If the user did not give reliable file hints or extra context, omit them instead of inventing them.

## Output To User

After bootstrapping, report:

- inferred `task_type`
- inferred execution mode (`real` or `smoke`)
- created request file
- target project root
- defaults that were applied
- that validation and execution follow in later workflow steps

Keep `compose-request` as the focus of this skill. `rail validate-request` and `rail run` are available once request materialization is complete, but they belong to the later workflow steps rather than the initial draft-composition step.

If a later `rail execute --artifact ...` run stops unexpectedly, read or print `run_status.yaml` with `rail status --artifact ...` and include that summary in the chat response. The status summary tells the user the latest phase, current actor, interruption kind, evidence files, and next step.

## Guardrails

- Do not claim the task is implemented just because the workflow bootstrapped.
- Do not invent constraints the user did not state.
- Keep `definition_of_done` testable.
- Keep `constraints` short and concrete.
- Do not assume a source checkout is the runtime root.

For examples, see:

- `references/examples.md`
