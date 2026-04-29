# Python Actor Runtime Rail Redesign

**Date:** 2026-04-29

## Goal

Redesign Rail as a Python-first harness built on the OpenAI Agents SDK while
preserving Rail's product concept: users describe work in natural language,
Rail turns it into a bounded harness request, and Rail supervises actor work
against a separate target repository with explicit policy, artifacts, evidence,
and evaluator authority.

This is not a compatibility migration. Rail is experimental, so the redesign can
remove the Go runtime and the Codex CLI execution path instead of preserving
legacy behavior.

## Decision Summary

- Replace the Go runtime with a Python Rail Harness Runtime.
- Replace the `codex exec` integration with a Python **Actor Runtime** built on
  the OpenAI Agents SDK.
- Remove `codex_cli`, PATH trust logic, Homebrew symlink handling, CLI flag
  compatibility checks, `CODEX_HOME`-based actor sealing, and stdout JSONL as
  the primary evidence contract.
- Keep the Rail skill-first UX, request normalization, supervisor graph,
  policy gates, artifact contracts, evaluator gate, and result projection.
- Use **Actor Runtime** as the standard term for the component that executes
  actors. Avoid using "backend" as the primary architecture term.

## Why Python-First

The current failure mode is structural. The Go runtime has to treat Codex CLI as
an external subprocess and then defend every boundary around it: command
resolution, trusted PATH, CLI flags, isolated `CODEX_HOME`, user skill leakage,
stdout JSONL parsing, last-message files, and postflight filesystem audits.
Those concerns are not Rail's product value.

The OpenAI Agents SDK is the better center of gravity for a code-first agent
application. The official Agents SDK guidance says to use the SDK when the
application owns orchestration, tool execution, approvals, and state. It also
documents Sandbox agents for file, command, package, port, snapshot, and memory
workloads. Rail is exactly an orchestration and governance harness, so the SDK
should be the actor execution substrate while Rail remains the harness authority.

Relevant OpenAI docs:

- [Agents SDK overview](https://developers.openai.com/api/docs/guides/agents)
- [Apply Patch tool](https://developers.openai.com/api/docs/guides/tools-apply-patch)
- [Local shell tool](https://developers.openai.com/api/docs/guides/tools-local-shell)

The Apply Patch docs are especially important because they frame patching as a
model-produced operation that the host application applies. That matches Rail's
desired mutation boundary: the Actor Runtime may propose edits, but Rail applies
or rejects them.

## Non-Goals

- Do not preserve Go runtime compatibility.
- Do not preserve `codex_cli` compatibility.
- Do not keep a CLI subprocess fallback.
- Do not keep target-local policy as a source of additional trust.
- Do not let SDK traces or SDK session state become the source of truth for Rail
  outcomes.
- Do not let actors directly mutate the target repository as a trusted final
  state.
- Do not collapse the actor graph into one long autonomous SDK session.
- Do not move deterministic validation or evaluator authority into an LLM-only
  decision.

## Product Contract To Preserve

Rail continues to own:

- natural-language request composition through the Rail skill
- request normalization and validation
- task identity and artifact allocation
- `.harness/` actor, supervisor, rule, rubric, and template surfaces
- deterministic supervisor routing
- explicit policy gates
- bounded actor outputs
- validation evidence
- evaluator pass/revise/reject authority
- terminal summaries
- `rail result` projection
- reviewed learning state

The Actor Runtime owns only actor execution:

- model/tool loop
- actor prompt execution
- structured actor output
- SDK trace capture
- sandboxed exploration and editing
- patch bundle production
- normalized tool/event evidence

## Target Architecture

```text
User
  -> Rail skill
  -> Python Rail API
     -> optional CLI wrapper
  -> Request normalizer
  -> Artifact store
  -> Supervisor graph
  -> Policy gate
  -> Python Actor Runtime
       -> OpenAI Agents SDK
       -> sandbox workspace
       -> tools / patch generation / trace capture
  -> Runtime evidence
  -> Evaluator gate
  -> Result projection
```

Rail is not a thin SDK wrapper. Rail is the governance harness around SDK actor
execution.

## Task Identity And Resume Policy

Rail should not make users manage task IDs. The durable identity of a run is an
**artifact handle** returned by Rail when a fresh task is started. A handle may
be rendered as a structured object, an artifact path for local operators, or a
future UI link, but the product contract is the same: follow-up operations use
the handle, not a manually chosen task ID.

An `ArtifactHandle` must be schema-validated and bound to the project and run:

```yaml
schema_version: 1
artifact_id: <opaque-generated-id>
artifact_dir: /absolute/path/to/artifact
project_root: /absolute/path/to/target-repo
request_snapshot_digest: sha256:...
effective_policy_digest: sha256:...
created_at: 2026-04-29T00:00:00Z
```

Rail must canonicalize every handle before use. Reject handles when the artifact
directory or project root is symlinked, path-traversed, outside the expected
artifact store, mismatched with the current project, missing its request
snapshot, or inconsistent with recorded digests. A remembered handle from a
previous interaction is usable only when it still resolves to the same canonical
project root and artifact record; otherwise ask for clarification.

Use this decision flow before any execution:

```text
User request
  -> includes an artifact handle/path?
       yes -> existing artifact flow
       no
  -> explicitly asks to continue, resume, retry, supervise, inspect status,
     inspect result, debug a run, or integrate prior run output?
       yes -> ask for artifact handle if none is known in the current session
       no
  -> describes a new goal, bug, feature, refactor, test repair, or other new
     work item?
       yes -> fresh task flow
       no
  -> ambiguous reference to prior work?
       ask one concise clarification: fresh task or existing artifact?
```

Fresh task flow:

1. Normalize the natural-language request into a request draft.
2. Materialize a request snapshot.
3. Allocate a new artifact atomically.
4. Return the artifact handle.
5. Supervise using that artifact handle.

Existing artifact flow:

1. Resolve the supplied or remembered artifact handle.
2. Do not compose a new request.
3. Do not allocate a new artifact.
4. Run the requested operation against the existing artifact: supervise,
   status, result, debug, retry, or integrate.

Blocked, rejected, or failed artifacts do not automatically capture future
work. If the user gives a new goal, Rail starts a fresh task even when a prior
artifact is blocked. Resume happens only when the user asks to continue prior
work or provides an artifact handle.

The request file under `.harness/requests/` is not a durable run identity. It
may be overwritten by later natural-language work. The artifact-local request
snapshot and artifact handle are the source of truth for a run.

Implementation should expose a direct Python API first:

```python
handle = rail.start_task(draft)
rail.supervise(handle)
rail.result(handle)
```

The command-line interface, if present, should be a thin wrapper over that API.
It does not need to preserve the existing `--task-id` surface. If an internal
artifact ID is needed, Rail generates it; users should not be asked to choose it.

Artifact IDs should be opaque generated IDs, not request filename derivatives.
Human-readable labels may be stored as metadata, but they are not identity.

## New Runtime Vocabulary

- **Rail Harness Runtime:** the whole Python runtime that owns request,
  supervisor, policy, artifacts, validation, and result projection.
- **Actor Runtime:** the Python Agents SDK powered component that runs one Rail
  actor invocation and returns a schema-valid result plus evidence.
- **Supervisor:** the deterministic Rail graph and routing logic.
- **Policy Gate:** the code path that allows, narrows, pauses, or rejects actor
  capabilities and outputs.
- **Artifact Store:** the durable workflow record under `.harness/artifacts/`.
- **Patch Bundle:** the Actor Runtime's proposed filesystem mutation package.
  Rail validates and applies it.

## Proposed Python Layout

```text
rail/
  __init__.py
  cli/
    main.py
    compose_request.py
    run.py
    supervise.py
    result.py
    status.py
    doctor.py
  request/
    schema.py
    normalize.py
  policy/
    schema.py
    load.py
    validate.py
  supervisor/
    graph.py
    router.py
    state.py
  actor_runtime/
    runtime.py
    agents.py
    prompts.py
    tools.py
    schemas.py
    events.py
    evidence.py
  workspace/
    sandbox.py
    patch_bundle.py
    apply.py
    validation.py
  artifacts/
    store.py
    models.py
    projection.py
  evaluator/
    gate.py
  auth/
    credentials.py
tests/
pyproject.toml
```

The existing `.harness/`, `skills/rail/`, and `assets/skill/Rail/` product
surfaces remain first-class, but their content must be updated to describe the
Python Actor Runtime instead of the Go/Codex CLI runtime.

## Policy V2

The redesign should introduce a new policy shape rather than adapting the
CLI-shaped `actor_backend.yaml`.

```yaml
version: 2
runtime: python_actor_runtime

actor_runtime:
  provider: openai_agents_sdk
  session_mode: per_actor
  structured_output: required
  tracing: required

workspace:
  mode: sandbox_patch
  direct_target_mutation: false

tools:
  shell: policy_controlled
  file_editing: patch_bundle
  network: disabled_by_default
  mcp: disabled_by_default

capabilities:
  user_skills: disabled
  user_rules: disabled
  plugins: disabled
  hooks: disabled

approval_policy: never
```

Target-local `.harness` policy can narrow allowed behavior but must not grant
new trust. Any future elevated trust must come from an operator-owned Rail
installation policy outside the target repository.

Policy composition must load Rail/operator defaults first, then apply
target-local policy as a validated narrowing overlay. A target repository must
not be able to enable a runtime provider, tool, network mode, credential source,
direct mutation mode, or approval behavior that the operator defaults did not
already authorize.

Policy narrowing must be implemented as an explicit lattice, not ad hoc field
merging. The policy schema must reject unknown fields, fail closed on missing
values, define per-field enum ordering, validate nested tool options, cap
timeouts and resource limits, and write the canonical effective policy digest to
runtime evidence for each actor invocation.

## Actor Runtime Contract

`ActorInvocation`:

```yaml
actor_name: planner
actor_run_id: 01_planner
prompt: ...
output_schema_ref: runs/01_planner-output-schema.json
artifact_dir: .harness/artifacts/<task-id>
workspace:
  target_root: /absolute/path/to/target-repo
  sandbox_root: /absolute/path/to/rail-sandbox/<task-id>/01_planner
  sandbox_ref: workspace/01_planner-ref.yaml
  mode: read_only_snapshot
policy:
  approval_policy: never
  shell: disabled
  file_editing: disabled
  network: disabled
```

`ActorResult`:

```yaml
status: completed
structured_output: {}
trace_ref: traces/01_planner.json
events_ref: runs/01_planner-events.jsonl
runtime_evidence_ref: runs/01_planner-runtime-evidence.yaml
patch_bundle_ref: null
interruption: null
```

Every actor result must be schema-validated before the supervisor consumes it.
SDK traces are evidence, not decision authority.

## Workspace And Mutation Boundary

The Actor Runtime must not directly commit trusted changes to the target
repository. The intended flow is:

```text
target repository
  -> Rail creates external sandbox snapshot/worktree
  -> Actor Runtime explores or edits sandbox
  -> Actor Runtime emits patch bundle and evidence
  -> Rail validates patch paths and policy
  -> Rail applies accepted patch to target repository
  -> Rail runs validation
  -> Evaluator decides pass/revise/reject
```

This keeps the most important authority boundary in Rail: an actor can propose
changes, but Rail decides whether they can affect the target.

The sandbox should live outside the target repository by default. Artifacts
should store sandbox references and evidence, not the sandbox itself as a
trusted source tree. This avoids target source-control churn and makes cleanup,
path validation, and patch-base checks explicit.

## Execution Isolation And Tool Safety

Actor tools must not be able to mutate the target repository directly. Shell,
validation, and file-editing tools run only inside an isolated sandbox,
container, or worktree where the target repository is unavailable or mounted
read-only. The Actor Runtime must not pass host paths that allow commands to
write into the canonical target root.

Required safeguards:

- scrub environment variables before actor and validation execution
- deny absolute paths that point at the target root
- deny symlink and hardlink escapes from sandbox roots
- capture target tree digest before actor execution, before patch apply, after
  patch apply, before validation, and after validation
- fail the run if the target tree changes before Rail applies a patch
- fail or explicitly report validation-side mutations unless policy allows them
- execute validation commands from request or policy, not from actor invention
- apply accepted changes only through Rail-validated patch bundles

Local shell is therefore not a permission to run arbitrary host commands. It is
a request for Rail to execute a policy-checked command in an isolated execution
environment and return evidence.

## Supervisor Graph

The initial graph remains conceptually unchanged:

```text
planner
  -> context_builder
  -> critic
  -> generator
  -> executor
  -> evaluator
```

Routing remains deterministic and artifact-driven. The redesign can rename files
or schemas, but it must not move the final quality decision into the Actor
Runtime.

## Auth And Secrets

The redesign should not copy the Codex CLI auth model. Python Rail should use
SDK-compatible credentials and enforce these rules:

- no secret values in artifacts
- no secret values in terminal summaries
- no secret values in result projection
- credentials live outside target-local `.harness`
- actors receive only the minimum capability needed for the invocation
- policy evidence records credential source category, not credential values or
  machine-specific secret paths
- target-local credential files and environment requests are rejected
- SDK traces, normalized events, validation logs, terminal summaries, and result
  projections are filtered through the same redaction pipeline
- test fixtures seed fake secret canaries and fail if they appear anywhere under
  artifacts or user-facing output

## Evidence Contract

Each actor run should write:

- normalized event log
- SDK trace reference or exported trace
- runtime evidence YAML
- structured actor output
- optional patch bundle
- optional validation output

Evidence must be stable enough for `rail result` and humans to inspect without
knowing SDK internals.

Evidence must also be hash-linked. Each run record should include content
digests for:

- request snapshot
- effective policy
- actor invocation
- actor output
- patch bundle
- pre-apply target tree
- post-apply target tree
- validation evidence
- evaluator input

The evaluator gate must compare these digests before terminal `pass`. A pass is
invalid when the evaluator is judging stale output, stale validation evidence, or
evidence produced under a different effective policy.

## Validation Gates

The redesign is not complete until these gates pass:

- A fresh task can be composed from natural language through the Rail skill.
- A task can allocate an artifact without hand-authored YAML.
- Fresh work returns an artifact handle and does not require user-selected task
  IDs.
- Existing artifact operations resume only when an artifact handle is supplied
  or the user explicitly asks to continue prior work.
- The supervisor can run all required actor phases through the Python Actor
  Runtime.
- Actor outputs validate against Rail-owned schemas.
- Target repository mutation happens only through Rail-validated patch bundles.
- Path traversal and forbidden path writes are rejected.
- Validation evidence is stored under the artifact.
- Evaluator pass/revise/reject is required before terminal success.
- `rail result --json` works without knowing SDK internals.
- The docs contain no machine-specific home-directory paths.

## Rollout Strategy

Because legacy compatibility is not required, implementation should be direct:

1. Build the Python runtime skeleton and tests.
2. Recreate request/artifact/policy/supervisor contracts in Python.
3. Add Actor Runtime with fake SDK tests first.
4. Add real OpenAI Agents SDK integration.
5. Add sandbox and patch bundle mutation boundary.
6. Update Rail skill and docs.
7. Remove Go runtime and Codex CLI runtime surfaces.

No phase should keep compatibility code unless it is needed as a temporary
scaffold within the same implementation branch.
