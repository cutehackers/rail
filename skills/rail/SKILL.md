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
rail auth doctor
rail compose-request --stdin
rail compose-request --input /absolute/path/to/request-draft.json
rail run --request /absolute/path/to/target-repo/.harness/requests/request.yaml --project-root /absolute/path/to/target-repo
rail supervise --artifact /absolute/path/to/target-repo/.harness/artifacts/<allocated-task-id>
rail result --artifact /absolute/path/to/target-repo/.harness/artifacts/<allocated-task-id> --json
rail status --artifact /absolute/path/to/target-repo/.harness/artifacts/<allocated-task-id>
```

Before any standard actor execution, run `rail auth doctor`. If it fails because actor auth is not configured, run `rail auth login` once, complete the Codex browser login flow, then retry `rail auth doctor`. Do not run `rail auth login` on every skill trigger. Do not ask users to pass API keys in task prompts; Rail stores Codex login state in a Rail-owned auth home outside the request and does not print secret values. The login persists for the local machine account across target repositories unless the user runs `rail auth logout`, the credential expires, or the Rail auth home is removed.

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
- artifact path printed by `rail run`, if a fresh run was started
- that validation and execution follow in later workflow steps, if only request composition was requested

Keep `compose-request` as the focus of this skill. `rail validate-request` and `rail run` are available once request materialization is complete, but they belong to the later workflow steps rather than the initial draft-composition step.

When the user asks to start or execute a fresh natural-language task, use this sequence:

1. run `rail compose-request --stdin`
2. run `rail run --request <printed-request-path> --project-root <target-repo>` without `--task-id`
3. capture the artifact path printed by `rail run`
4. run `rail auth doctor`
5. run `rail supervise --artifact <printed-artifact-path>`
6. run `rail result --artifact <printed-artifact-path> --json`

Do not ask users to choose task ids. Do not reconstruct the artifact path from the request filename; Rail may allocate a suffix such as `request-2` when an earlier artifact exists. The printed artifact directory is the durable run identity, and the artifact-local `request.yaml` snapshot is the source of truth for that run because `.harness/requests/request.yaml` may be overwritten by the next natural-language task.

For later execution steps on an existing artifact, run `rail auth doctor` before standard actor execution, then run `rail supervise --artifact ...` for the artifact. `supervise` reruns retryable actor/session interruptions within its retry budget and stops only on terminal status or a non-retryable blocker.

If `rail auth doctor` is not ready, do not start `supervise`. Run `rail auth login` first, complete browser login, then report that actor auth is ready before continuing. This prevents the actor loop from stopping later with `blocked_environment` due to missing sealed actor credentials.

Whenever the artifact exists after `supervise`, always run `rail result --artifact ... --json`. Use that result JSON as the reporting contract for the chat response: report outcome, evidence, residual risk, and the recommended next step from the projected result.

If a later execution run still stops unexpectedly and `rail result --artifact ... --json` is not available, read or print `run_status.yaml` with `rail status --artifact ...` and include that summary in the chat response. The status summary tells the user the latest phase, current actor, interruption kind, evidence files, and next step.

## Guardrails

- Do not claim the task is implemented just because the workflow bootstrapped.
- Do not invent constraints the user did not state.
- Keep `definition_of_done` testable.
- Keep `constraints` short and concrete.
- Do not assume a source checkout is the runtime root.
- Do not report harness success from `supervise` process output alone. Use `rail result --artifact ... --json` as the reporting contract.

For examples, see:

- `references/examples.md`
