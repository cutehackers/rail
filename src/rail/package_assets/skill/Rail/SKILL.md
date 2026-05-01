---
name: Rail
description: Use when bootstrapping and executing the Rail harness workflow against a target application repository from a natural-language task request.
---

# Rail

Use this skill to turn a natural-language software task into a bounded Rail request draft and Python Rail Harness Runtime flow.

The target application repository is not the Rail source repository. It is carried in the draft as `project_root`.

## Purpose

The user should be able to describe the task naturally. You then:

1. decide whether this is fresh work or an existing artifact operation
2. infer the request draft fields
3. ask one concise clarification only when a missing field would make the request unsafe
4. call the Rail Python API or an API-backed wrapper
5. report the persisted artifact handle, projected result, and terminal summary

Do not make the user write YAML by hand unless they explicitly ask to inspect or export it.

## Required Input

You must know the target repository root. If it is missing, ask once for the target repo path.

## Draft Fields

Infer:

- `request_version`
- `project_root`
- `task_type`
- `goal`
- `context`
- `constraints`
- `definition_of_done`
- `risk_tolerance`
- `validation_profile` only when the user explicitly asks for smoke mode

Defaults:

- `request_version=1`
- `bug_fix`, `feature_addition`, `test_repair` -> `risk_tolerance=low`
- `safe_refactor` -> `risk_tolerance=medium`
- default execution mode is `real`, normalized to `validation_profile=standard`
- use `validation_profile=smoke` only for explicit smoke or harness-only verification

## Draft Shape

```json
{
  "request_version": "1",
  "project_root": "/absolute/path/to/target-repo",
  "task_type": "bug_fix",
  "goal": "Describe the requested outcome",
  "context": {
    "feature": "profile",
    "suspected_files": ["src/profile/service.py"],
    "related_files": [],
    "validation_roots": [],
    "validation_targets": []
  },
  "constraints": ["Short concrete constraint"],
  "definition_of_done": ["Observable requested behavior", "Related validation passes"],
  "risk_tolerance": "low",
  "validation_profile": "standard"
}
```

Reject unknown fields instead of inventing them.

`context.validation_roots` and `context.validation_targets` are path hints, not shell commands. Put command expectations in `definition_of_done` unless Rail exposes a first-class command field.

## Task Identity Decision

Start a fresh task when the user gives a new goal, bug, feature, refactor, or test repair. Fresh work calls `rail.start_task(draft)` and receives a new artifact handle. The persisted handle lives at `handle.artifact_dir / "handle.yaml"`.

Continue an existing artifact only when the user asks to continue, retry, inspect status/result, debug, integrate, or provides a known artifact handle or `handle.yaml` path. Existing artifact operations must not compose a new request or allocate a new artifact.

If the user references prior work but gives neither a new goal nor an artifact handle, ask one concise clarification:

```text
Should I start this as a fresh Rail task, or continue an existing artifact? If continuing, provide the artifact handle.
```

Do not ask users to choose task ids. Do not derive identity from `.harness/requests/request.yaml`.

## Python API Flow

Fresh task:

```python
import rail
from rail.artifacts.terminal_summary import project_terminal_summary

request = rail.specify(draft)
handle = rail.start_task(draft)
rail.supervise(handle)
result = rail.result(handle)
summary = project_terminal_summary(handle)
```

Existing artifact:

```python
import rail
from rail.artifacts.terminal_summary import project_terminal_summary

handle = rail.load_handle("/absolute/path/to/artifact/handle.yaml")
status = rail.status(handle)
result = rail.result(handle)
summary = project_terminal_summary(handle)
```

Optional command wrappers are acceptable only when they call the same Python API.

## Readiness And Blocking

`rail.supervise(handle)` checks Actor Runtime readiness before actor work. The default local provider is `codex_vault`, which uses Rail-owned Codex auth and an artifact-local actor environment. `openai_agents_sdk` is optional for operator/API-key environments. Missing auth material, policy blocks, validation failures, contamination, and environment problems must stop as blocked outcomes with secret-safe reasons.

`rail auth` is setup and diagnostics only. It can prepare or inspect the Rail-owned Codex auth home, but it is not a task-execution command surface.

When supervision blocks, do not continue by manually invoking actors or mutating the target. Report `rail.result(handle)` and the terminal summary so the user sees the blocked category, reason, evidence refs, and next step.

## Reporting

For fresh tasks, report:

- inferred `task_type`
- inferred execution mode
- target project root
- artifact handle
- terminal outcome from `rail.result(handle)`
- blocked category and reason from the terminal summary when blocked
- evidence refs
- residual risk
- next step

Do not claim implementation success from supervisor process output alone. Use result projection and terminal summary.

## Guardrails

- Keep `constraints` and `definition_of_done` concrete and testable.
- Do not invent requirements the user did not state.
- Do not expose secret values or target-local credential paths.
- Do not treat request files as run identity.
- Do not mutate the target directly; target mutation must happen through Rail-validated patch bundles.

For examples, see:

- `references/examples.md`
