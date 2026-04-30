# Python Actor Runtime Rail Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild Rail as a Python-first harness with an OpenAI Agents SDK powered Actor Runtime while preserving Rail's skill-first request, supervisor, artifact, policy, evaluator, and result contracts.

**Architecture:** Python becomes the only product runtime. The Actor Runtime executes actor invocations through the OpenAI Agents SDK, but Rail remains the authority for policy, artifacts, routing, patch application, validation, and evaluator decisions. There is no legacy Go or Codex CLI compatibility requirement.

**Tech Stack:** Python 3.12+, OpenAI Agents SDK, Pydantic, Typer or Click, pytest, ruff, mypy or pyright, PyYAML, jsonschema, Git CLI for sandbox/worktree operations where needed.

---

## File Structure

Create the Python runtime under `rail/`:

- `rail/cli/`: user-facing commands.
- `rail/api.py`: primary Python API facade; CLI commands wrap this API.
- `rail/request/`: request schema and natural-language draft normalization.
- `rail/policy/`: policy v2 schema, load, validation, and capability narrowing.
- `rail/supervisor/`: actor graph, routing, state transitions, retry budgets.
- `rail/actor_runtime/`: OpenAI Agents SDK integration, prompts, output schemas, normalized events, evidence.
- `rail/workspace/`: sandbox snapshot/worktree handling, patch bundle validation and apply.
- `rail/artifacts/`: artifact store models, read/write helpers, result projection.
- `rail/evaluator/`: evaluator gate and terminal decision helpers.
- `rail/auth/`: SDK credential discovery and secret-safe diagnostics.
- `tests/`: pytest coverage by package.

Update product surfaces:

- `.harness/`: replace CLI-shaped actor runtime policy with policy v2 examples.
- `skills/rail/`: update Rail skill workflow.
- `assets/skill/Rail/`: keep bundled skill aligned with repo-owned skill.
- `README.md` and architecture docs: describe Python Actor Runtime.

Remove product runtime surfaces after Python parity:

- `cmd/`
- `internal/`
- Go-specific build/test references
- Codex CLI runtime docs

## Task 0: Agents SDK Surface Spike

**Files:**
- Create: `spikes/agents_sdk_boundary.py`
- Create: `tests/spikes/test_agents_sdk_boundary.py`
- Create: `pyproject.toml`

- [x] **Step 1: Write the failing no-network SDK boundary test**

Write a test that imports the real OpenAI Agents SDK package and constructs the
SDK objects Rail depends on without making network calls:

- `Agent` or equivalent actor definition
- runner or run configuration object
- structured-output configuration
- tracing configuration with sensitive-data capture disabled where supported
- policy-controlled tool list with shell, network, MCP, and patch tools disabled
  by default
- sandbox capability object with no direct target mutation

- [x] **Step 2: Run the spike test and verify it fails**

Run: `pytest tests/spikes/test_agents_sdk_boundary.py -q`

Expected: FAIL until the dependency is pinned and the real SDK surface is known.

- [x] **Step 3: Pin the dependency and implement the spike**

Add the Agents SDK dependency to `pyproject.toml`. Implement the smallest
no-network construction test. Do not mock the SDK import in this spike.

- [x] **Step 4: Verify**

Run:

```bash
pytest tests/spikes/test_agents_sdk_boundary.py -q
```

Expected: pass without network access.

- [x] **Step 5: Commit**

```bash
git add pyproject.toml spikes tests/spikes
git commit -m "test: pin agents sdk boundary"
```

## Task 1: Python Public API Skeleton

**Files:**
- Create: `rail/__init__.py`
- Create: `rail/api.py`
- Create: `tests/test_api_smoke.py`
- Create: `tests/test_no_legacy_runtime_calls.py`
- Modify: `.gitignore`

- [x] **Step 1: Write the failing public API smoke test**

```python
import rail


def test_public_api_exports_harness_operations():
    assert callable(rail.start_task)
    assert callable(rail.supervise)
    assert callable(rail.status)
    assert callable(rail.result)
```

- [x] **Step 2: Run the test and verify it fails**

Run: `pytest tests/test_api_smoke.py -q`

Expected: FAIL because the public API does not exist.

- [x] **Step 3: Add minimal Python package and public API**

Implement `rail/api.py` and export from `rail/__init__.py`:

```python
from __future__ import annotations


def start_task(draft):
    raise NotImplementedError


def supervise(handle):
    raise NotImplementedError


def status(handle):
    raise NotImplementedError


def result(handle):
    raise NotImplementedError
```

- [x] **Step 4: Add legacy runtime guard**

Add a test that fails if new Python runtime code shells out to `codex exec`, Go
binaries, `./build/rail`, or legacy compatibility shims. The Python API is the
organizing boundary; CLI wrappers come later.

- [x] **Step 5: Verify**

Run:

```bash
pytest tests/test_api_smoke.py tests/test_no_legacy_runtime_calls.py -q
ruff check rail tests
```

Expected: tests pass and lint passes.

- [x] **Step 6: Commit**

```bash
git add pyproject.toml rail tests .gitignore
git commit -m "feat: scaffold python rail api"
```

## Task 2: Request Schema And Draft Normalization

**Files:**
- Create: `rail/request/schema.py`
- Create: `rail/request/normalize.py`
- Create: `tests/fixtures/skill_request_drafts/feature_addition.json`
- Modify: `rail/api.py`
- Test: `tests/request/test_compose_request.py`

- [x] **Step 1: Write failing schema normalization tests**

Test that a JSON draft with `project_root`, `task_type`, `goal`, `constraints`,
and `definition_of_done` becomes a normalized request with version `1`.
Also test a fixture shaped like the Rail skill's natural-language output so the
core skill-first contract is covered before the e2e phase.

- [x] **Step 2: Run tests and verify they fail**

Run: `pytest tests/request/test_compose_request.py -q`

Expected: FAIL because request modules do not exist.

- [x] **Step 3: Implement request models**

Use Pydantic models for:

- `RequestDraft`
- `HarnessRequest`
- `RequestContext`

Validation rules:

- `project_root` is required.
- `goal` is required.
- `task_type` is one of the current Rail task types.
- `constraints` and `definition_of_done` are arrays of non-empty strings.
- Docs/examples must use placeholder paths, not user home paths.
- Rail-skill-produced request drafts are accepted without hand-authored harness
  YAML.

- [x] **Step 4: Implement request normalization API**

Expose request normalization through the Python API:

```python
request = rail.specify(draft)
```

Do not require a target-local `.harness/requests/request.yaml` handoff for
execution. A later optional CLI/export command may write a request file for
inspection, but `start_task(draft)` must be able to allocate an artifact from an
in-memory draft.

- [x] **Step 5: Verify**

Run:

```bash
pytest tests/request/test_compose_request.py -q
ruff check rail tests
```

Expected: pass.

- [x] **Step 6: Commit**

```bash
git add rail/request rail/api.py tests/request
git commit -m "feat: add request draft normalization"
```

## Task 3: Artifact Store

**Files:**
- Create: `rail/artifacts/models.py`
- Create: `rail/artifacts/store.py`
- Create: `rail/artifacts/identity.py`
- Modify: `rail/api.py`
- Test: `tests/artifacts/test_store.py`
- Test: `tests/artifacts/test_task_identity.py`

- [x] **Step 1: Write failing artifact allocation tests**

Test that the direct Python API allocates an artifact and returns an artifact
handle:

```python
handle = rail.start_task(draft)
assert handle.artifact_id
assert handle.artifact_dir.is_absolute()
assert handle.project_root.is_absolute()
```

The command-line path, if added later, may exist only as a thin wrapper over this
API and must print or return the same artifact handle.

Test that allocation creates:

```text
.harness/artifacts/<task-id>/
  request.yaml
  state.yaml
  workflow.yaml
  run_status.yaml
  runs/
```

- [x] **Step 2: Write failing task identity tests**

Cover the policy:

- fresh goal with no artifact handle allocates a new artifact
- fresh goal does not reuse a blocked, rejected, failed, or existing artifact
- existing artifact handle resolves to existing artifact flow
- continue/resume/retry/status/result/debug/integrate without a known handle
  returns a clarification-needed decision
- existing artifact flow does not compose a new request
- existing artifact flow does not allocate a new artifact
- `.harness/requests/request.yaml` is not treated as run identity
- user-supplied task IDs are not part of the user-facing API
- artifact handles reject symlinked artifact directories, path traversal,
  mismatched project roots, missing request snapshots, and digest mismatches
- concurrent fresh allocations produce distinct artifact handles atomically

- [x] **Step 3: Run tests and verify they fail**

Run:

```bash
pytest tests/artifacts/test_store.py tests/artifacts/test_task_identity.py -q
```

Expected: FAIL because artifact store does not exist.

- [x] **Step 4: Implement artifact models**

Use Pydantic models for:

- `WorkflowState`
- `RunStatus`
- `ActorRunRecord`
- `TerminalSummary`
- `ArtifactHandle`
- `TaskIdentityDecision`

- [x] **Step 5: Implement allocation and identity decisions**

Generate opaque artifact IDs atomically. Do not derive identity from
`.harness/requests/request.yaml` or request filenames. Human-readable request
labels may be stored as metadata only. Do not require callers to hand-write task
IDs. Implement a direct Python API such as:

```python
handle = rail.start_task(draft)
decision = decide_task_identity(user_intent, known_handle=None)
```

The CLI, if retained, should only wrap this API. `ArtifactHandle` must include
canonical artifact directory, canonical project root, request snapshot digest,
effective policy digest when available, and schema version.

- [x] **Step 6: Verify**

Run:

```bash
pytest tests/artifacts/test_store.py tests/artifacts/test_task_identity.py -q
```

Expected: pass.

- [x] **Step 7: Commit**

```bash
git add rail/artifacts rail/api.py tests/artifacts
git commit -m "feat: add artifact handles and task identity"
```

## Task 4: Policy V2

**Files:**
- Create: `rail/policy/schema.py`
- Create: `rail/policy/load.py`
- Create: `rail/policy/validate.py`
- Create: `assets/defaults/supervisor/actor_runtime.yaml`
- Modify: `.harness/supervisor/actor_runtime.yaml`
- Test: `tests/policy/test_policy_v2.py`

- [x] **Step 1: Write failing policy tests**

Cover:

- valid default policy loads
- target policy can narrow capabilities
- target policy cannot grant direct mutation
- target policy cannot enable a tool, network mode, credential source, provider,
  approval behavior, or mutation mode that Rail/operator defaults did not
  authorize
- unknown keys are rejected at every policy level
- nested tool options cannot broaden an allowlist or resource ceiling
- per-field enum ordering is tested for every narrowing field
- canonical effective-policy digests are stable
- `direct_target_mutation: true` is rejected
- unknown runtime provider is rejected
- docs/examples do not contain home-directory paths

- [x] **Step 2: Run tests and verify they fail**

Run: `pytest tests/policy/test_policy_v2.py -q`

Expected: FAIL because policy v2 does not exist.

- [x] **Step 3: Implement policy models**

Implement policy v2 fields:

- `runtime`
- `actor_runtime`
- `workspace`
- `tools`
- `capabilities`
- `approval_policy`

Use closed Pydantic models or JSON Schema with `extra=forbid`. Define explicit
narrowing functions for every field instead of dictionary merging.

- [x] **Step 4: Implement policy loader**

Load Rail/operator defaults first, then apply target `.harness/` policy only as
a validated narrowing overlay. Keep source metadata for evidence, but do not
print machine-specific absolute paths in docs. Write canonical effective-policy
evidence and digest helpers for actor invocations.

- [x] **Step 5: Verify**

Run:

```bash
pytest tests/policy/test_policy_v2.py -q
```

Expected: pass.

- [x] **Step 6: Commit**

```bash
git add rail/policy assets/defaults/supervisor .harness/supervisor tests/policy
git commit -m "feat: add actor runtime policy v2"
```

## Task 4A: Credential And Secret Safety

**Files:**
- Create: `rail/auth/credentials.py`
- Create: `rail/auth/redaction.py`
- Create: `rail/cli/doctor.py`
- Test: `tests/auth/test_credentials.py`
- Test: `tests/auth/test_secret_redaction.py`

- [x] **Step 1: Write failing credential and redaction tests**

Cover:

- only approved operator credential source categories are accepted
- target-local credential files and target-local environment requests are
  rejected
- actor environment is minimum necessary
- fake secret canaries are redacted from SDK traces, normalized events,
  validation logs, runtime evidence, terminal summaries, and result projection
- `doctor` reports credential source category and runtime readiness without
  printing secret values or machine-specific secret paths

- [x] **Step 2: Run tests and verify they fail**

Run: `pytest tests/auth/test_credentials.py tests/auth/test_secret_redaction.py -q`

Expected: FAIL because auth modules do not exist.

- [x] **Step 3: Implement credential source and redaction models**

Implement allowlisted credential source categories, secret-bearing key
detection, redaction helpers, and artifact scan helpers that fail when seeded
secret canaries appear under artifacts.

- [x] **Step 4: Implement doctor diagnostics**

Expose direct API and optional CLI doctor diagnostics. Diagnostics must be
secret-safe and must not require Codex CLI.

- [x] **Step 5: Verify**

Run:

```bash
pytest tests/auth/test_credentials.py tests/auth/test_secret_redaction.py -q
```

Expected: pass.

- [x] **Step 6: Commit**

```bash
git add rail/auth rail/cli/doctor.py tests/auth
git commit -m "feat: add sdk credential safety"
```

## Task 4B: Actor Schema And Prompt Catalog

**Files:**
- Create: `rail/actor_runtime/prompts.py`
- Create: `rail/actor_runtime/schemas.py`
- Test: `tests/actor_runtime/test_actor_catalog.py`

- [x] **Step 1: Write failing actor catalog tests**

Cover:

- every supervisor actor has a prompt source
- every actor has an output schema
- `.harness/actors/*.md` prompts load deterministically
- `.harness/templates/*schema*` output contracts map to actor names
- fake actor outputs for planner, context_builder, critic, generator, executor,
  and evaluator validate against the loaded schemas

- [x] **Step 2: Run tests and verify they fail**

Run: `pytest tests/actor_runtime/test_actor_catalog.py -q`

Expected: FAIL because actor catalog does not exist.

- [x] **Step 3: Implement prompt and schema catalog**

Implement actor-to-prompt and actor-to-schema resolution before the real Actor
Runtime is wired. The supervisor and Actor Runtime must consume this catalog
instead of hardcoding prompt/schema paths.

- [x] **Step 4: Verify**

Run:

```bash
pytest tests/actor_runtime/test_actor_catalog.py -q
```

Expected: pass.

- [x] **Step 5: Commit**

```bash
git add rail/actor_runtime tests/actor_runtime
git commit -m "feat: add actor prompt and schema catalog"
```

## Task 5: Supervisor Graph And Routing

**Files:**
- Create: `rail/supervisor/graph.py`
- Create: `rail/supervisor/router.py`
- Create: `rail/supervisor/state.py`
- Modify: `rail/api.py`
- Test: `tests/supervisor/test_routing.py`

- [x] **Step 1: Write failing graph tests**

Test the initial graph:

```text
planner -> context_builder -> critic -> generator -> executor -> evaluator
```

Test revise routing and terminal pass/reject behavior.

- [x] **Step 2: Run tests and verify they fail**

Run: `pytest tests/supervisor/test_routing.py -q`

Expected: FAIL because supervisor modules do not exist.

- [x] **Step 3: Implement graph and state transitions**

Implement deterministic transitions and revision budgets. Do not invoke the
Actor Runtime yet; use fake actor results.

- [x] **Step 4: Implement supervision API stub**

Load artifact, load policy, step through graph, and write run status.

- [x] **Step 5: Verify**

Run:

```bash
pytest tests/supervisor/test_routing.py -q
```

Expected: pass.

- [x] **Step 6: Commit**

```bash
git add rail/supervisor rail/api.py tests/supervisor
git commit -m "feat: add python supervisor graph"
```

## Task 6: Actor Runtime Port And Fake Runtime

**Files:**
- Create: `rail/actor_runtime/runtime.py`
- Create: `rail/actor_runtime/evidence.py`
- Modify: `rail/actor_runtime/schemas.py`
- Test: `tests/actor_runtime/test_fake_runtime.py`

- [x] **Step 1: Write failing Actor Runtime contract tests**

Test that an `ActorInvocation` returns an `ActorResult` with:

- `status`
- `structured_output`
- `events_ref`
- `runtime_evidence_ref`
- optional `patch_bundle_ref`

- [x] **Step 2: Run tests and verify they fail**

Run: `pytest tests/actor_runtime/test_fake_runtime.py -q`

Expected: FAIL because Actor Runtime contract does not exist.

- [x] **Step 3: Implement runtime protocol**

Add:

- `ActorInvocation`
- `ActorResult`
- `ActorRuntime`
- `FakeActorRuntime`

- [x] **Step 4: Integrate fake runtime with supervisor**

Allow supervisor tests to run full graph using fake structured actor outputs.

- [x] **Step 5: Verify**

Run:

```bash
pytest tests/actor_runtime/test_fake_runtime.py tests/supervisor/test_routing.py -q
```

Expected: pass.

- [x] **Step 6: Commit**

```bash
git add rail/actor_runtime tests/actor_runtime tests/supervisor
git commit -m "feat: define actor runtime contract"
```

## Task 7: OpenAI Agents SDK Runtime

**Files:**
- Create: `rail/actor_runtime/agents.py`
- Create: `rail/actor_runtime/events.py`
- Modify: `rail/actor_runtime/prompts.py`
- Modify: `rail/actor_runtime/schemas.py`
- Modify: `rail/actor_runtime/runtime.py`
- Test: `tests/actor_runtime/test_agents_runtime.py`

- [x] **Step 1: Write tests with mocked SDK runner**

Mock the Agents SDK call boundary. Test:

- actor prompt is built from invocation
- output schema is passed to the SDK layer
- final SDK output is validated
- trace reference is persisted
- SDK errors map to Rail interruptions
- SDK tools are constructed only from the narrowed invocation policy
- shell, network, MCP, and file editing are disabled by default
- approvals and timeouts are mapped from policy, not SDK defaults
- environment and event capture are secret-safe

Also add an offline SDK-boundary test that instantiates the real SDK adapter and
tool objects with a fake transport or no-network test client. This test must
exercise real SDK object construction, structured-output configuration, and
policy-to-tool mapping without making live network calls.

- [x] **Step 2: Run tests and verify they fail**

Run: `pytest tests/actor_runtime/test_agents_runtime.py -q`

Expected: FAIL because SDK runtime does not exist.

- [x] **Step 3: Implement Agents SDK adapter**

Use a small wrapper so tests do not depend on live network calls. Keep the
wrapper responsible for SDK-specific object construction.

- [x] **Step 4: Implement policy-controlled tool construction**

Build the SDK tool set from the narrowed invocation policy. Do not create shell,
network, MCP, or patch tools unless the policy explicitly allows them.

- [x] **Step 5: Implement normalized events**

Convert SDK trace/tool events into Rail-owned JSONL event records. Do not expose
SDK internals as the only evidence contract.

- [x] **Step 6: Verify mocked and offline SDK-boundary runtime**

Run:

```bash
pytest tests/actor_runtime/test_agents_runtime.py -q
```

Expected: mocked tests and no-network real SDK adapter construction tests pass.

- [x] **Step 7: Commit**

```bash
git add rail/actor_runtime tests/actor_runtime
git commit -m "feat: add agents sdk actor runtime"
```

## Task 8: Workspace Sandbox And Patch Bundle

**Files:**
- Create: `rail/workspace/sandbox.py`
- Create: `rail/workspace/patch_bundle.py`
- Create: `rail/workspace/apply.py`
- Create: `rail/workspace/isolation.py`
- Test: `tests/workspace/test_patch_bundle.py`
- Test: `tests/workspace/test_execution_isolation.py`

- [x] **Step 1: Write failing sandbox and patch tests**

Cover:

- sandbox is created outside target source control by default
- actor edits produce a patch bundle
- absolute path writes are rejected
- `..` traversal is rejected
- symlink and hardlink escapes are rejected
- unsafe rename and delete operations are rejected
- executable bit changes are explicitly represented and policy-checked
- binary file operations are rejected unless policy explicitly allows them
- patch bundle size and file count limits are enforced
- patch apply checks the expected sandbox base revision before mutating target
- patch apply is revalidated immediately before write to avoid TOCTOU gaps
- writes to `.harness/artifacts` are rejected unless explicitly allowed for
  evidence
- target repo is unchanged until Rail applies patch
- shell commands run only inside the external sandbox/container/worktree
- target root is unavailable or read-only during actor tool execution
- absolute target paths are denied in tool inputs
- pre/post target tree digests prove actor tools did not mutate the target

- [x] **Step 2: Run tests and verify they fail**

Run:

```bash
pytest tests/workspace/test_patch_bundle.py tests/workspace/test_execution_isolation.py -q
```

Expected: FAIL because workspace module does not exist.

- [x] **Step 3: Implement sandbox creation**

Start with copy or git worktree strategy. Keep the implementation simple and
deterministic. The sandbox must live outside the target repository and must be
the only writable working tree exposed to actor tools.

- [x] **Step 4: Implement patch bundle validation**

Validate every file operation before apply. The patch bundle schema must include
operation type, relative path, expected old content or digest where practical,
mode changes, size metadata, and sandbox base identity.

- [x] **Step 5: Implement execution isolation checks**

Scrub environment variables, deny target-root absolute paths, record target tree
digests before and after actor execution, and fail if the target changed before
Rail applies a patch.

- [x] **Step 6: Verify**

Run:

```bash
pytest tests/workspace/test_patch_bundle.py tests/workspace/test_execution_isolation.py -q
```

Expected: pass.

- [x] **Step 7: Commit**

```bash
git add rail/workspace tests/workspace
git commit -m "feat: add sandbox patch workflow"
```

## Task 9: Validation Runner And Evaluator Gate

**Files:**
- Create: `rail/workspace/validation.py`
- Create: `rail/evaluator/gate.py`
- Create: `rail/artifacts/digests.py`
- Test: `tests/evaluator/test_gate.py`
- Test: `tests/evaluator/test_evidence_chain.py`

- [x] **Step 1: Write failing evaluator gate tests**

Test:

- validation command output is stored as evidence
- terminal success is rejected when required validation evidence is missing
- terminal success is rejected when required validation failed
- validation evidence must be current for the patch under evaluation
- validation commands are request-owned or policy-owned, not actor-invented
- validation runs with the same credential, network, and sandbox restrictions as
  actor tools
- validation-side mutations are detected and rejected unless policy explicitly
  allows them
- evaluator pass requires matching request, effective policy, actor invocation,
  patch bundle, tree state, validation, and evaluator input digests
- evaluator pass is required for terminal success
- evaluator revise routes back to generator
- evaluator reject creates terminal failure

- [x] **Step 2: Run tests and verify they fail**

Run:

```bash
pytest tests/evaluator/test_gate.py tests/evaluator/test_evidence_chain.py -q
```

Expected: FAIL because evaluator gate does not exist.

- [x] **Step 3: Implement validation evidence model**

Capture command, exit code, stdout/stderr paths, duration, status, credential
mode, network mode, sandbox ref, pre/post validation tree digests, and mutation
status.

- [x] **Step 4: Implement evaluator gate**

Keep evaluator decision deterministic from schema-valid evaluator output, but do
not allow `pass` unless required validation evidence is present, current, and
acceptable. Any validation waiver must be explicit in policy and visible in
runtime evidence. Compare content digests before terminal pass.

- [x] **Step 5: Verify**

Run:

```bash
pytest tests/evaluator/test_gate.py tests/evaluator/test_evidence_chain.py -q
```

Expected: pass.

- [x] **Step 6: Commit**

```bash
git add rail/workspace/validation.py rail/evaluator rail/artifacts/digests.py tests/evaluator
git commit -m "feat: add validation and evaluator gate"
```

## Task 10: Result And Status Projection

**Files:**
- Create: `rail/artifacts/projection.py`
- Modify: `rail/api.py`
- Test: `tests/artifacts/test_projection.py`

- [x] **Step 1: Write failing result projection tests**

Test `rail result --artifact ... --json` returns:

- outcome
- current phase
- terminal decision
- evidence refs
- changed files
- residual risk
- next step

- [x] **Step 2: Run tests and verify they fail**

Run: `pytest tests/artifacts/test_projection.py -q`

Expected: FAIL because projection does not exist.

- [x] **Step 3: Implement projection**

Projection reads artifacts only. It must not call the SDK.

- [x] **Step 4: Implement status and result API**

Status prints latest phase, actor, interruption, terminal state, and next step.

- [x] **Step 5: Verify**

Run:

```bash
pytest tests/artifacts/test_projection.py -q
```

Expected: pass.

- [x] **Step 6: Commit**

```bash
git add rail/artifacts/projection.py rail/api.py tests/artifacts
git commit -m "feat: add result projection"
```

## Task 10A: Runtime Integration Slices

**Files:**
- Modify: `rail/supervisor/router.py`
- Modify: `rail/supervisor/state.py`
- Modify: `rail/actor_runtime/runtime.py`
- Modify: `rail/workspace/patch_bundle.py`
- Modify: `rail/workspace/apply.py`
- Modify: `rail/workspace/validation.py`
- Modify: `rail/evaluator/gate.py`
- Modify: `rail/artifacts/projection.py`
- Test: `tests/integration/test_runtime_flow_slices.py`

- [x] **Step 1: Write failing integration slice tests**

Cover these flows separately:

- supervisor invokes `ActorRuntime` and persists actor result evidence
- generator result produces a patch bundle reference
- patch apply updates target only through Rail validation
- validation evidence is tied to the applied patch and tree digest
- evaluator consumes validation evidence and routes pass/revise/reject
- result/status projection reads only artifacts produced by the prior slices

- [x] **Step 2: Run tests and verify they fail**

Run: `pytest tests/integration/test_runtime_flow_slices.py -q`

Expected: FAIL until integration slices are wired.

- [x] **Step 3: Wire one integration boundary at a time**

Do not use the e2e smoke as a catch-all implementation task. If a slice fails,
fix the owning module and add focused tests there before updating the integration
test.

- [x] **Step 4: Verify**

Run:

```bash
pytest tests/integration/test_runtime_flow_slices.py -q
```

Expected: pass.

- [x] **Step 5: Commit**

```bash
git add rail tests/integration
git commit -m "feat: wire python harness runtime slices"
```

## Task 11: Skill And Documentation Update

**Files:**
- Modify: `README.md`
- Modify: `docs/ARCHITECTURE.md`
- Modify: `skills/rail/SKILL.md`
- Modify: `assets/skill/Rail/SKILL.md`
- Test: `tests/docs/test_no_home_paths.py`

- [x] **Step 1: Write failing docs lint**

Test docs and skill files do not contain user home paths or stale Go/Codex CLI
runtime guidance.

- [x] **Step 2: Run docs lint and verify it fails**

Run: `pytest tests/docs/test_no_home_paths.py -q`

Expected: FAIL until docs are updated.

- [x] **Step 3: Update user workflow docs**

Describe:

- Python Rail Harness Runtime
- Actor Runtime
- SDK credential doctor
- request composition
- run/supervise/result workflow
- sandbox patch mutation boundary
- artifact handle fresh/resume policy

- [x] **Step 4: Keep skill copies aligned**

Run:

```bash
cmp -s skills/rail/SKILL.md assets/skill/Rail/SKILL.md
diff -qr skills/rail/references assets/skill/Rail/references
```

Expected: no diff.

- [x] **Step 5: Verify docs**

Run:

```bash
pytest tests/docs/test_no_home_paths.py -q
rg -n -e '/U[s]ers/' -e '~[/]' -e '/h[o]me/' README.md docs skills/rail assets/skill/Rail
```

Expected: pytest passes; `rg` returns no machine-specific home path examples.

- [x] **Step 6: Commit**

```bash
git add README.md docs/ARCHITECTURE.md skills/rail assets/skill/Rail tests/docs
git commit -m "docs: describe python actor runtime workflow"
```

## Task 12: End-To-End Harness Smoke

**Files:**
- Create: `tests/e2e/test_python_harness_smoke.py`
- Create: `examples/python-target/`

- [x] **Step 1: Write failing e2e smoke**

Test a small target repo task:

Direct API path:

```python
handle = rail.start_task(draft)
rail.supervise(handle)
result = rail.result(handle)
```

CLI wrapper path, if present:

```bash
rail start --request-draft /absolute/path/to/request-draft.json
rail supervise --handle /absolute/path/to/target-repo/.harness/artifacts/<artifact-id>/handle.yaml
rail result --handle /absolute/path/to/target-repo/.harness/artifacts/<artifact-id>/handle.yaml --json
```

Use placeholders in docs; tests may create temp paths at runtime. The e2e must
also assert that a second fresh goal allocates a second artifact and that a
follow-up result/status operation uses the original artifact handle without
allocating a new artifact.

- [x] **Step 2: Run and verify it fails**

Run: `pytest tests/e2e/test_python_harness_smoke.py -q`

Expected: FAIL until all runtime paths are wired.

- [x] **Step 3: Verify prior wiring through e2e**

This task should not implement broad missing wiring. It should prove the prior
request, artifact handle, policy, supervisor, Actor Runtime, patch, validation,
evaluator, and projection tasks work together. If the e2e exposes a missing
boundary, add or fix the focused owning-module test first.

- [x] **Step 4: Verify full suite**

Run:

```bash
pytest -q
ruff check rail tests
mypy rail
```

Expected: pass.

- [x] **Step 5: Commit**

```bash
git add tests/e2e examples rail
git commit -m "test: add python rail harness smoke"
```

## Task 13: Remove Go And Codex CLI Runtime Surfaces

**Files:**
- Delete: `cmd/`
- Delete: `internal/`
- Delete: `go.mod`
- Delete: `go.sum`
- Delete: `assets/defaults/embed.go`
- Delete or rewrite: Go-specific tests and build references
- Modify: `README.md`
- Modify: docs under `docs/`
- Test: full Python suite

- [x] **Step 1: Write removal guard test**

Add a test that fails if product docs mention `codex exec`, `codex_cli`,
trusted PATH, Homebrew symlink handling, or Go runtime commands as the active
workflow.

The guard must also inventory Go product surfaces: `cmd/`, `internal/`,
`go.mod`, `go.sum`, `assets/defaults/embed.go`, Go CI jobs, Go release scripts,
and active docs that tell operators to run `go test`, `go build`, or
`./build/rail`.

- [x] **Step 2: Run guard and verify it fails**

Run: `pytest tests/docs/test_removed_runtime_surfaces.py -q`

Expected: FAIL before removal.

- [x] **Step 3: Remove Go runtime surfaces**

Delete Go product runtime, module files, and active build/test references only
after Task 12 passes. Historical design docs may mention Go only when clearly
framed as historical context.

- [x] **Step 4: Remove CLI runtime policy files**

Replace CLI-shaped policy files with policy v2. Keep `.harness/` layout but not
the old CLI semantics.

- [x] **Step 5: Verify**

Run:

```bash
pytest -q
ruff check rail tests
mypy rail
```

Expected: pass.

- [x] **Step 6: Commit**

```bash
git add -A
git commit -m "refactor: remove go and codex cli runtime"
```

## Final Acceptance

- [x] `rail.specify(draft)` creates a schema-valid request object.
- [x] `start_task(draft)` allocates an artifact and returns an artifact handle.
- [x] Existing artifact operations use the artifact handle without composing a
      new request or allocating a new artifact.
- [x] A new goal creates a fresh artifact even when an older artifact is blocked
      or rejected.
- [x] Any CLI command, if present, is a wrapper over the Python API.
- [x] `rail.supervise(handle)` runs the Python Actor Runtime path for a supplied
      artifact handle.
- [x] `rail.result(handle)` projects outcome from artifacts only.
- [x] Shell, validation, and file tools cannot mutate the target outside
      Rail-validated patch apply.
- [x] Credential canary tests prove secrets do not leak into artifacts or
      user-facing output.
- [x] Effective policy, actor invocation, patch, validation, and evaluator
      evidence are digest-linked before terminal pass.
- [x] No product workflow invokes `codex exec`.
- [x] No product workflow depends on trusted PATH, Homebrew symlink handling, or
      CLI flag compatibility.
- [x] Target mutation happens only through Rail-validated patch bundles.
- [x] Docs and skills use Actor Runtime terminology.
- [x] Docs and examples contain no user home-directory paths.
