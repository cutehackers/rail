# Actor Executor Port and Result Projection Design

**Date:** 2026-04-27

**Goal**

Refine Rail's long-term Codex integration plan after the sealed actor runtime
and browser-based Rail actor auth work. Rail should keep its governance
contracts stable while making Codex execution executor-neutral and making harness
outcomes easy for Codex and humans to report.

## Decision Summary

The current direction is fundamentally sound:

- Rail remains the governance control plane.
- Codex remains the agent execution runtime.
- Actor boundaries, artifact contracts, evaluator authority, and learning state
  remain Rail-owned.

The design needs two corrections before implementation continues:

1. The actor executor layer must become a true Rail-level port, not a Codex CLI
   flag abstraction.
2. Harness result reporting should start as a derived `rail result` projection
   over existing artifacts, not as new durable `result.json` or `result.md`
   source files.

## Current Baseline

The v0.5.0 codebase already includes important runtime hardening:

- `internal/runtime/actor_backend.go` loads reviewable backend policy from
  `.harness/supervisor/actor_backend.yaml`.
- `internal/runtime/actor_runtime.go` invokes `codex exec`, passes structured
  actor output schema paths, captures JSON events, and reads the final actor
  message.
- `internal/runtime/actor_runtime_sealed.go` prepares artifact-local sealed
  actor runtime directories, resolves a trusted Codex command, sanitizes `PATH`,
  sets isolated `CODEX_HOME`, `HOME`, `XDG_*`, and temp directories, materializes
  only Rail actor auth material, and records non-secret provenance.
- `internal/auth/actor_auth.go` owns the Rail Codex auth home, browser login
  wrappers, auth home validation, and allowlisted auth materialization into
  sealed actor `CODEX_HOME`.
- `internal/runtime/run_status.go` writes `run_status.yaml` and formats status
  summaries.
- `internal/runtime/router.go` writes terminal `terminal_summary.md` when a run
  reaches a terminal supervisor outcome.
- `internal/cli/status.go`, `internal/cli/execute.go`, and
  `internal/cli/supervise.go` expose the current status path.

This means the next design step is not to invent sealed execution or result
state from scratch. The next step is to separate stable Rail ports from current
Codex CLI adapter details and improve the user-facing projection over the
artifacts Rail already writes.

## Non-Goals

- Do not replace Codex with a Rail-native LLM/tool runtime.
- Do not collapse the actor graph into one long Codex session.
- Do not make `terminal_summary.md`, `run_status.yaml`,
  `evaluation_result.yaml`, or `execution_report.yaml` obsolete.
- Do not persist `result.json` or `result.md` in the first result-reporting
  iteration.
- Do not authorize `danger-full-access` from target-local `.harness` policy.
- Do not move deterministic validation into an LLM actor.

## Target Architecture

```text
User-facing Codex session
  Rail skill
  request drafting
  rail auth doctor preflight
  rail supervise execution
  rail result projection
  human-facing outcome summary

Rail governance layer
  request normalization
  workflow state
  actor graph
  artifact schemas
  evaluator routing
  validation evidence
  learning state

Rail executor ports
  ActorExecutor
  ValidationRunner
  RuntimeEvidenceSink
  ResultProjector

Executor adapters
  codex_cli
  future codex_sdk
  future codex_mcp
  smoke

Codex runtime
  model/tool loop
  repository inspection
  file editing
  structured actor output
  event stream
  sandbox and approval enforcement
```

The governance layer must speak in Rail concepts. Codex CLI flags are adapter
implementation details.

## Actor Executor Port

Introduce an executor-neutral port before adding new executor types:

```go
type ActorExecutor interface {
    RunActor(context.Context, ActorInvocation) (ActorResult, error)
}

type ActorInvocation struct {
    ActorName         string
    ActorRunID        string
    WorkingDirectory  string
    ArtifactDirectory string
    Prompt            string
    OutputSchemaPath  string
    LastMessagePath   string
    EventSink         RuntimeEvidenceSink
    Profile           ActorProfile
    Policy            ActorExecutionPolicy
}

type ActorResult struct {
    Status             ActorRunStatus
    StructuredOutput   map[string]any
    LastMessagePath    string
    RuntimeEvidence    RuntimeEvidenceRef
    BackendProvenance  BackendProvenance
    InterruptionKind   string
    Message            string
}
```

The port should not expose CLI-only concepts such as `subcommand`,
`--output-last-message`, `--skip-git-repo-check`, or `--ignore-rules`.

`codex_cli` should become one adapter:

```text
ActorInvocation
  -> codex_cli adapter
    -> build codex exec args
    -> prepare sealed runtime
    -> execute command
    -> audit raw JSONL events
    -> return ActorResult
```

Future `codex_sdk` and `codex_mcp` adapters should implement the same port
without inheriting CLI-specific configuration fields.

## Policy Split

Current target-local `.harness/supervisor/actor_backend.yaml` can remain the
reviewable project policy surface, but it must not become the only trust source.

Separate policy into two levels:

1. **Target-local policy**
   - selects or narrows allowed backend behavior
   - sets actor execution intent such as sandbox, approval policy, event capture,
     and disabled capabilities
   - cannot grant additional trust

2. **Trusted operator or installation policy**
   - authorizes backend kinds
   - authorizes execution environments
   - authorizes credential source behavior
   - may later authorize CI-specific or container-specific elevated access
   - remains outside target-local `.harness`

Initial implementation should continue to reject full access even if a
target-local file asks for it.

## Session Mode

`session_mode: per_actor` is the only currently real mode.

`per_run` should be treated as reserved until the executor port supports explicit
session lifecycle operations:

```go
type StatefulActorExecutor interface {
    StartRunSession(context.Context, RunSessionSpec) (RunSession, error)
}
```

Until then, documentation and validation should avoid implying that `per_run`
is configurable.

## Runtime Evidence

Raw Codex JSONL events are useful debug evidence, but they should not be the
only durable evidence contract. Add a normalized Rail-owned evidence read model:

```yaml
schema_version: 1
backend_type: codex_cli
actor: generator
actor_run_id: 04_generator
status: completed
policy:
  sandbox: workspace-write
  approval_policy: never
  capabilities:
    user_skills: disabled
    plugins: disabled
    mcp: disabled
raw_event_log: runs/04_generator-events.jsonl
provenance: runtime/04_generator/actor_environment.yaml
redaction:
  applied: true
  secret_values_written: false
policy_violations: []
```

The exact filename can be adapter-specific at first, for example
`runs/04_generator-runtime-evidence.yaml`. The important contract is that
summaries and tests can read Rail-owned evidence fields instead of depending on
Codex event JSON shape stability.

## Validation Runner Port

The deterministic executor should remain non-LLM, but long term it should sit
behind a policy-controlled validation runner instead of raw shell execution.

Introduce this as a later phase:

```go
type ValidationRunner interface {
    RunValidation(context.Context, ValidationInvocation) (ValidationResult, error)
}
```

Initial scope:

- keep current deterministic validation behavior
- record that executor sandboxing remains a known follow-up
- move toward typed validation commands instead of opaque shell strings
- attach validation provenance to execution evidence

This preserves repeatable validation while reducing the mismatch where Codex
actors are sealed but executor commands are not under the same policy model.

## Result Projection

Do not add persisted `result.json` or `result.md` in the first iteration.

Add a derived CLI surface:

```bash
rail result --artifact /absolute/path/to/target-repo/.harness/artifacts/<task-id>
rail result --artifact /absolute/path/to/target-repo/.harness/artifacts/<task-id> --json
rail result --latest --project-root /absolute/path/to/target-repo
```

`rail result` is a projection, not a new source of truth.

For non-terminal states, derive from:

- `run_status.yaml`
- `state.json` when needed for completed actor counts
- `work_ledger.md` and `next_action.yaml` as evidence references only

For terminal states, derive from:

- `run_status.yaml`
- `terminal_summary.md`
- `evaluation_result.yaml`
- `execution_report.yaml`
- `supervisor_trace.md` as evidence reference

The JSON output is a stable read model for Codex:

```json
{
  "schema_version": 1,
  "artifact_dir": "/absolute/path/to/target-repo/.harness/artifacts/<task-id>",
  "status": "interrupted",
  "phase": "actor_execution",
  "current_actor": "generator",
  "last_successful_actor": "critic",
  "interruption_kind": "actor_watchdog_expired",
  "message": "Actor stopped producing observable progress.",
  "terminal": false,
  "human_summary": "Rail stopped during generator execution before a valid implementation result was accepted.",
  "recommended_next_step": "Rerun rail supervise after inspecting the actor runtime evidence.",
  "evidence": [
    "run_status.yaml",
    "runs/",
    "work_ledger.md",
    "next_action.yaml"
  ],
  "source_artifacts": {
    "run_status": "run_status.yaml"
  },
  "updated_at": "2026-04-27T00:00:00Z"
}
```

The human output should be concise and suitable for direct Codex reporting:

```text
Rail result: interrupted

Phase: actor_execution
Current actor: generator
Last successful actor: critic
Reason: actor_watchdog_expired

What happened:
Rail stopped before the generator returned a valid implementation result.

Next step:
Inspect run_status.yaml and runs/ evidence, then rerun rail supervise.
```

## Latest Result Selection

Avoid a durable `.harness/artifacts/latest` pointer in the first iteration.

Implement `rail result --latest --project-root ...` by scanning
`.harness/artifacts/*/run_status.yaml` and selecting the newest reliable
`updated_at`.

If scanning becomes too slow later, add a pointer with a precise name such as:

```text
.harness/artifacts/.last-started-artifact
```

Do not call it simply `latest`, because "latest" can mean latest started,
latest updated, latest terminal, or latest supervised.

## Rail Skill Contract

The Rail skill should use the following execution flow for standard actor work:

```text
1. rail auth doctor
2. rail compose-request --stdin
3. rail supervise --artifact <artifact>
4. rail result --artifact <artifact> --json
5. summarize the result for the user
```

If `rail supervise` returns an error, the skill should still run:

```bash
rail result --artifact <artifact> --json
```

when the artifact directory exists. This prevents the user-facing Codex answer
from depending only on process stderr.

The final user answer should include:

- outcome: passed, rejected, interrupted, blocked, or in progress
- completed scope
- changed files when available
- validation evidence when available
- interruption or rejection reason when relevant
- residual risk
- recommended next step

The skill should not ask users to inspect raw artifact files unless the result
projection points to specific evidence.

## Status Semantics

Keep `run_status.yaml` as the live source of truth.

Normalize projected status into a small result taxonomy:

```text
initialized
in_progress
retrying
passed
rejected
blocked
interrupted
failed
```

Map existing supervisor statuses into that taxonomy without changing the
underlying `state.json` state machine in the first iteration. For example:

- `blocked_environment` -> projected `blocked`
- `split_required` -> projected `blocked`
- `revise_exhausted` -> projected `rejected`
- `evolution_exhausted` -> projected `rejected`
- `actor_watchdog_expired` in `run_status.yaml` -> projected `interrupted`

## Migration Plan

1. Add this design as the follow-up to the existing Codex boundary and sealed
   actor runtime designs.
2. Add `internal/runtime/result_projection.go` with pure read-model builders.
3. Add `internal/cli/result.go` and wire `rail result`.
4. Update `supervise` success and failure output to point at or print the same
   result projection.
5. Update `skills/rail/SKILL.md` and `assets/skill/Rail/SKILL.md` so Codex runs
   `rail result --json` after `supervise`.
6. Introduce executor-neutral `ActorExecutor`, `ActorInvocation`, and
   `ActorResult` types.
7. Move current `codex exec` construction into a `codex_cli` adapter while
   keeping current behavior.
8. Add normalized runtime evidence after the adapter boundary is in place.
9. Add a validation-runner design and implementation phase after result
   projection and actor executor porting are stable.

## Validation Strategy

Unit tests:

- `rail result --artifact` renders interrupted `run_status.yaml`.
- `rail result --artifact --json` emits stable JSON for interrupted runs.
- terminal projection includes `terminal_summary.md` evidence.
- `--latest --project-root` selects the newest `run_status.yaml` by
  `updated_at`.
- result projection does not write `result.json` or `result.md`.
- `codex_cli` adapter receives executor-neutral `ActorInvocation` and still
  constructs the current expected `codex exec` command.
- sealed actor runtime provenance remains available through the adapter result.

Runtime smoke:

- smoke profile still completes without live Codex.
- standard profile with fake Codex still records sealed runtime provenance and
  event evidence.
- interrupted fake Codex run produces a useful `rail result --json` summary.

Docs checks:

- examples use `/absolute/path/to/...` placeholders.
- docs do not include concrete user home directory paths.
- Rail skill and bundled skill stay aligned.

## Acceptance Criteria

- Rail has an executor-neutral actor execution port.
- `codex_cli` is an adapter behind that port.
- CLI-only concepts do not leak into the executor-neutral invocation/result
  types.
- `rail result` gives Codex a stable machine-readable outcome projection.
- `rail result` gives operators a concise human-readable outcome.
- No new durable `result.json` or `result.md` source files are required.
- Existing `run_status.yaml` and `terminal_summary.md` remain canonical.
- The Rail skill reports harness completion, interruption, rejection, and
  blocked states without asking users to inspect raw artifacts first.

## Open Questions

- Should `rail result` default to human output and use `--json`, or should it
  support `--format human|json` from the start?
- Should result projection include changed files directly from
  `implementation_result.yaml`, or only cite that artifact in the first
  iteration?
- Which trusted policy location should eventually authorize non-local or
  elevated execution environments?
- Should normalized runtime evidence be one file per actor run or one aggregate
  file per artifact?
- Should `rail auth doctor` be included in `rail supervise` preflight, or remain
  skill-driven to keep CLI commands composable?
