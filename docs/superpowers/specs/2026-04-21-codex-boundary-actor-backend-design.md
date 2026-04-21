# Codex Boundary and Actor Backend Design

**Date:** 2026-04-21

**Goal**

Clarify Rail's product boundary with Codex so Rail remains a Codex-native governance control plane instead of duplicating Codex's agent runtime. The design introduces an explicit actor backend layer, safer Codex execution defaults, and richer Codex run evidence while preserving Rail's request, artifact, evaluator, and learning contracts.

## Problem Statement

Rail currently runs real actors by spawning `codex exec` for each Codex-backed actor. This proves that Rail can orchestrate a bounded actor graph, but it also creates an unclear boundary:

- Codex already provides the agent runtime, tool loop, sandbox selection, approval policy, rules, skills, and structured output support.
- Rail currently wraps Codex with its own actor loop, artifact contracts, evaluator routing, and deterministic executor.
- The current actor command hard-codes `danger-full-access`, `approval_policy=never`, `--ephemeral`, and per-actor `codex exec` sessions.
- Rail captures only the actor's final schema-valid message, not the richer Codex event stream that would help debugging and audit.

The risk is that Rail looks like a second harness competing with Codex instead of a governance layer that makes Codex coding work reviewable, policy-bound, and evidence-backed.

## Product Boundary

Rail should own governance.

Rail owns:

- natural-language request normalization through the Rail skill
- `.harness` request, artifact, and evidence contracts
- bounded workflow state and actor dependency ordering
- evaluator pass, revise, reject routing
- deterministic validation execution and validation evidence
- reviewed learning state
- PR and release decision evidence summaries

Codex should own agent execution.

Codex owns:

- model and tool-loop execution
- repository inspection and file editing within the selected sandbox
- command approval and sandbox enforcement
- AGENTS.md, Codex rules, Codex skills, hooks, and MCP integrations
- structured final actor output generation
- GitHub Action and non-interactive execution primitives

Rail should not reimplement Codex sandboxing, command mediation, or GitHub action bootstrapping. Rail should configure and audit those surfaces.

## Scope

This spec covers:

- adding an explicit actor backend configuration model
- moving hard-coded Codex CLI flags into reviewable runtime policy
- changing the default local Codex sandbox posture away from full access
- capturing Codex JSON event streams as Rail run evidence
- preserving Rail's actor output schemas and evaluator authority
- documenting which Codex capabilities Rail delegates to

This spec does not cover:

- Flutter, Node, Python, or other platform profiles
- changing `plan`, `context_pack`, `critic_report`, `implementation_result`, `execution_report`, or `evaluation_result` schemas
- replacing the deterministic executor with a Codex actor
- implementing a hosted Rail service
- replacing the current Rail skill contract

## Design Principles

- Rail is a governance control plane, not a competing agent runtime.
- Codex execution should be configured through explicit Rail policy, not hard-coded flags.
- Local execution should default to the least privilege that still supports normal coding work.
- Full-access execution is not currently authorized by target-local `.harness` policy; trusted environment support is a future concern.
- Codex event evidence should be retained when available.
- Rail artifacts remain the durable audit contract even when the underlying Codex backend evolves.
- The evaluator remains the authoritative supervisor gate.

## Architecture

The revised execution model has three layers:

```text
Rail governance layer
  request contract
  workflow state
  artifact schemas
  evaluator routing
  learning and release evidence

Actor backend layer
  backend type
  sandbox policy
  approval policy
  session mode
  event capture
  output schema wiring

Codex runtime layer
  codex exec or future SDK/MCP backend
  model/tool loop
  sandbox and approvals
  rules, skills, hooks, MCP
  structured final output
```

Rail continues to call Codex for actor work, but only through the actor backend abstraction.

## Actor Backend Configuration

Introduce a reviewable actor backend policy file, initially under the supervisor surface:

```yaml
version: 1

default_backend: codex_cli

backends:
  codex_cli:
    command: codex
    subcommand: exec
    sandbox: workspace-write
    approval_policy: never
    session_mode: per_actor
    ephemeral: true
    capture_json_events: true
    skip_git_repo_check: true

execution_environments:
  local:
    allowed_sandboxes:
      - workspace-write
```

The exact filename can be finalized during implementation. Two viable locations are:

- `.harness/supervisor/actor_backend.yaml`
- `.harness/supervisor/actor_profiles.yaml` extended with a `backend` section

The recommended initial choice is a separate `actor_backend.yaml` because backend execution policy is distinct from actor model and reasoning selection.

## Backend Behavior

### Codex CLI Backend

The first backend is the current behavior made explicit:

- invoke `codex exec`
- pass the actor model and reasoning from actor profiles
- pass the actor prompt generated by Rail
- pass `--output-schema` for the actor output contract
- pass `--output-last-message` for the final schema-valid response
- optionally pass `--json` and persist JSONL event evidence

The backend must construct flags from policy rather than hard-coded defaults.

### Future Backends

The backend interface should allow later additions without changing workflow state:

- `codex_cli`: current non-interactive CLI path
- `codex_sdk`: future session reuse and richer telemetry path
- `codex_mcp`: future tool-server integration path
- `smoke`: deterministic local backend used by smoke profile

Only `codex_cli` and existing `smoke` behavior are in scope for the first implementation.

## Sandbox and Approval Policy

The current hard-coded `danger-full-access` posture should no longer be the default.

Default local policy:

- sandbox: `workspace-write`
- approval policy: `never`
- full access: disallowed

Current full-access policy:

- `danger-full-access` is rejected outright by backend policy validation.
- Target-local `.harness` policy cannot authorize full access, even if `allowed_sandboxes` includes it.
- Isolated CI, Docker, and explicit operator opt-in require a future trusted policy source outside the target repository.

If a project requests `danger-full-access`, Rail should fail before invoking Codex and explain the unsupported unsafe configuration.

## Evidence Capture

Rail should preserve two classes of actor output:

1. Contract output
   - current schema-valid actor response
   - persisted as `plan.yaml`, `context_pack.yaml`, `critic_report.yaml`, `implementation_result.yaml`, or `evaluation_result.yaml`

2. Runtime evidence
   - Codex JSONL event stream when enabled
   - command/tool/error telemetry when available
   - persisted under the existing `runs/` directory

Representative output paths:

```text
.harness/artifacts/<task-id>/runs/03_critic-events.jsonl
.harness/artifacts/<task-id>/runs/03_critic-last-message.txt
.harness/artifacts/<task-id>/runs/03_critic-output-schema.json
```

Rail summaries should continue to use the schema-valid actor artifacts as the primary contract, but terminal summaries and integration evidence may cite event evidence when useful.

## Actor Workflow Impact

The actor flow remains:

```text
planner -> context_builder -> critic -> generator -> executor -> evaluator
```

The design does not remove role separation. The improvement is that Rail stops treating Codex CLI invocation details as hidden runtime constants.

The executor remains deterministic Rail subprocess validation. This is intentional because validation evidence should be repeatable and comparable across actor revisions. A later design may add sandboxed executor modes, but this spec does not move validation into Codex.

## GitHub Integration Boundary

Rail should not replace the Codex GitHub Action. Instead:

- Codex GitHub Action can provide the base Codex execution environment.
- Rail validates requests and artifacts.
- Rail produces PR-ready evidence summaries.
- Rail can upload or reference `.harness` artifacts as workflow evidence.

This keeps Rail focused on governance and auditability rather than CI runner bootstrapping.

## Migration Plan

1. Add actor backend policy loading with embedded defaults.
2. Preserve current behavior behind the `codex_cli` backend, except for safer default sandbox policy.
3. Replace hard-coded Codex CLI flags with backend-derived flags.
4. Add validation for unsafe sandbox/environment combinations.
5. Add optional Codex JSON event capture.
6. Update README and architecture docs to describe Rail as a Codex governance layer.
7. Keep smoke profile behavior deterministic and unchanged except for policy documentation.

## Validation Strategy

Unit tests:

- backend policy loads from embedded defaults
- project-local backend policy overrides embedded defaults
- `codex_cli` command construction matches configured sandbox, approval policy, model, reasoning, schema path, and output paths
- unsafe `danger-full-access` policy is rejected, including when a target-local allow-list includes it
- JSON event capture path is materialized when enabled

Runtime smoke tests:

- smoke profile still runs without live Codex
- standard profile command construction can be tested with a fake command runner
- existing artifact schema validation still passes

Documentation checks:

- docs describe Codex as the runtime layer
- docs describe Rail as the governance layer
- examples avoid machine-specific home-directory paths

## Open Questions

- Should `actor_backend.yaml` be a separate supervisor file, or should backend config live inside `actor_profiles.yaml`?
- Should JSON event capture be enabled by default, or only in CI and debug modes?
- Should `session_mode: per_run` wait for Codex SDK support, or should the CLI backend attempt `codex exec resume` reuse later?
- Should deterministic executor commands run under a separate shell sandbox in local mode?

## Acceptance Criteria

- Rail's docs clearly state that Codex is the agent runtime and Rail is the governance control plane.
- Codex CLI flags are no longer hard-coded in actor runtime.
- Local default actor execution uses `workspace-write`, not `danger-full-access`.
- Full-access execution requires an allowed environment or explicit opt-in.
- Actor final outputs remain schema-valid and unchanged.
- Codex runtime evidence can be persisted under `runs/` without replacing Rail artifacts.
- Existing smoke validation remains deterministic.
