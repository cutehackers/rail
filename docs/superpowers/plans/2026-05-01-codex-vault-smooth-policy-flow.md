# Codex Vault Smooth Policy Flow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Rail's skill-first execution flow reach `codex_vault` Actor Runtime outcomes without session-local bypasses, monkey patches, prompt patching, or confusing coding-agent side effects.

**Architecture:** Keep guidance minimal and move enforcement into Rail-owned Runtime boundaries. The Rail skill reports artifact projections only; `codex_vault` owns schema compatibility, sandbox creation, auth materialization, post-run contamination audit, and policy-safe evidence. Actor prompts can describe the allowed behavior, but they are not the primary isolation boundary.

**Tech Stack:** Python 3.12, Pydantic contracts, PyYAML, Codex CLI subprocess execution, pytest, ruff, mypy, optional `codex_vault` live smoke.

---

## Scope And Non-Goals

This plan hardens the live `codex_vault` path after observed failures where a Rail skill session attempted runtime monkey patches, prompt rewrites, auth-home cleanup, and repeated same-artifact retries.

Keep the plan intentionally small:

- Do not revive the Go CLI.
- Do not add a task-execution CLI contract.
- Do not rely on long prompt guidance as the main safety mechanism.
- Do not loosen policy to make actor shell behavior pass.
- Do not require normal users to understand request YAML, wrapper details, or auth-home cleanup.
- Do not use `OPENAI_API_KEY` for the default local path.

The desired user contract remains:

```python
request = rail.specify(draft)
handle = rail.start_task(draft)
rail.supervise(handle)
result = rail.result(handle)
```

When supervision blocks, the Rail skill reports the projected result and stops.

## File Structure

Create:

- `src/rail/actor_runtime/output_schema.py` - compiles actor schemas into Codex-compatible strict JSON schemas.
- `src/rail/artifacts/run_attempts.py` - allocates attempt-scoped run directories and stable attempt refs.
- `tests/actor_runtime/test_codex_output_schema.py` - strict schema compiler tests.
- `tests/artifacts/test_run_attempts.py` - attempt allocation and path safety tests.

Modify:

- `src/rail/actor_runtime/codex_vault.py` - use strict output schemas, run post-run vault audit, include attempt refs in evidence, keep policy blocks before success.
- `src/rail/actor_runtime/evidence.py` - write runtime evidence under attempt-scoped paths.
- `src/rail/actor_runtime/runtime.py` - carry `attempt_ref` on `ActorInvocation`.
- `src/rail/actor_runtime/agents.py` - keep optional SDK evidence writes aligned with attempt-scoped evidence.
- `src/rail/actor_runtime/vault_audit.py` - audit post-run `CODEX_HOME` materialization.
- `src/rail/actor_runtime/vault_env.py` - copy only allowlisted auth material into actor-local `CODEX_HOME`.
- `src/rail/auth/credentials.py` - tolerate known Codex operational files in Rail-owned auth home while selecting only auth material.
- `src/rail/supervisor/supervise.py` - allocate one attempt per supervision call, pass it through actor invocations, project exact attempt evidence.
- `src/rail/artifacts/terminal_summary.py` - report exact attempt-scoped evidence refs.
- `src/rail/artifacts/projection.py` - read attempt-scoped evidence refs.
- `src/rail/workspace/sandbox.py` - share ignored path policy between pre-scan and copy.
- `.harness/actors/context_builder.md` - add minimal command contract only.
- `assets/defaults/actors/context_builder.md` - keep bundled actor aligned.
- `src/rail/package_assets/defaults/actors/context_builder.md` - keep packaged actor aligned.
- `skills/rail/SKILL.md` - state blocked behavior without runtime patching.
- `assets/skill/Rail/SKILL.md` - keep shipped skill aligned.
- `src/rail/package_assets/skill/Rail/SKILL.md` - keep packaged skill aligned.
- `tests/actor_runtime/test_codex_vault_environment.py` - auth material and post-run materialization tests.
- `tests/actor_runtime/test_codex_vault_runtime.py` - evidence, contamination, and policy-block tests.
- `tests/actor_runtime/test_agents_runtime.py` - optional SDK provider evidence path updates.
- `tests/actor_runtime_test_fixtures.py`, `tests/integration_flow_fixtures.py` - shared fake runtime evidence path updates.
- `tests/workspace/test_execution_isolation.py` - sandbox ignore/pre-scan tests.
- `tests/supervisor/test_routing.py` - attempt-scoped blocked projection tests.
- `tests/artifacts/test_projection.py` - projection reads attempt evidence.
- `tests/artifacts/test_terminal_summary.py` - terminal summary includes attempt evidence refs.
- `tests/build/test_package_assets.py` - skill and bundled actor alignment tests.
- `tests/e2e/test_optional_codex_vault_smoke.py` - optional live smoke checks for post-run contamination.
- `tests/e2e/test_optional_live_sdk_smoke.py` - optional SDK smoke evidence path updates.
- `docs/SPEC.md`, `docs/ARCHITECTURE.md`, `docs/CONVENTIONS.md` - concise product contract updates.

---

### Task 1: Lock The Rail Skill Blocked Contract

**Files:**
- Modify: `skills/rail/SKILL.md`
- Modify: `assets/skill/Rail/SKILL.md`
- Modify: `src/rail/package_assets/skill/Rail/SKILL.md`
- Modify: `tests/build/test_package_assets.py`
- Modify: `docs/SPEC.md`

- [ ] **Step 1: Write the failing skill contract test**

Add a package asset test that reads all three Rail skill copies and asserts they contain a blocked-flow rule equivalent to:

```text
When supervision blocks, report rail.result(handle) and terminal summary, then stop.
Do not monkey-patch Rail internals, actor prompts, sandbox functions, or auth-home contents.
```

Also assert active skill text does not instruct users or agents to patch `_materialize_output_schema`, patch sandbox functions, move auth directories, or retry blocked artifacts after ad-hoc runtime edits.

- [ ] **Step 2: Run the focused package asset test**

Run:

```bash
uv run --python 3.12 pytest tests/build/test_package_assets.py -q
```

Expected: FAIL until all skill copies carry the same blocked-flow rule.

- [ ] **Step 3: Update skill copies with minimal wording**

Add one short rule under readiness/blocking:

```markdown
When `rail.supervise(handle)` blocks, stop. Report `rail.result(handle)`,
terminal summary, blocked category, reason, evidence refs, and next step.
Do not patch Rail runtime internals, actor prompts, sandbox behavior, auth homes,
or target files to continue the same run.
```

- [ ] **Step 4: Update canonical product contract**

In `docs/SPEC.md`, state that the Rail skill is an execution caller and reporter, not a runtime repair surface.

- [ ] **Step 5: Verify**

Run:

```bash
uv run --python 3.12 pytest tests/build/test_package_assets.py tests/docs/test_removed_runtime_surfaces.py -q
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add skills/rail/SKILL.md assets/skill/Rail/SKILL.md src/rail/package_assets/skill/Rail/SKILL.md tests/build/test_package_assets.py docs/SPEC.md
git commit -m "docs: lock rail skill blocked behavior"
```

---

### Task 2: Compile Actor Output Schemas For Codex

**Files:**
- Create: `src/rail/actor_runtime/output_schema.py`
- Create: `tests/actor_runtime/test_codex_output_schema.py`
- Modify: `src/rail/actor_runtime/codex_vault.py`
- Modify: `tests/actor_runtime/test_codex_vault_runtime.py`

- [ ] **Step 1: Write failing strict schema tests**

Add unit tests for:

- object schemas with `properties` get `additionalProperties: false`
- nested objects are normalized
- arrays normalize `items`
- input schemas are not mutated
- a local `assert_codex_strict_schema(schema)` helper rejects object schemas without `additionalProperties: false`
- the same helper rejects required fields that are not present in `properties`
- the same helper accepts optional fields represented as nullable fields or branch-specific alternatives instead of requiring every branch alternative at once
- real `plan.schema.yaml` compiles into a Codex-compatible strict schema
- real `implementation_result.schema.yaml` remains satisfiable for all three patch-source branches
- real `execution_report.schema.yaml` keeps optional telemetry fields optional or nullable in the Codex-compatible form

Do not assert that every property on every object is blindly required. That breaks schemas with `oneOf`-governed alternatives such as `patch_bundle_ref` versus `patch_bundle`.

Example:

```python
def test_compile_codex_output_schema_preserves_patch_bundle_alternatives():
    source = yaml.safe_load(Path("assets/defaults/templates/implementation_result.schema.yaml").read_text())
    compiled = compile_codex_output_schema(source)
    assert compiled["additionalProperties"] is False
    assert "oneOf" in compiled
    assert "patch_bundle_ref" in compiled["properties"]
    assert "patch_bundle" in compiled["properties"]
```

- [ ] **Step 2: Implement the compiler**

Create `compile_codex_output_schema(schema: Mapping[str, Any]) -> dict[str, Any]`.

Rules:

- deep copy the schema
- if a mapping has object shape, set `additionalProperties: false` unless an explicit schema-valued `additionalProperties` is required by the contract
- if an object has simple required properties already, preserve them
- if Codex strictness requires every top-level property to be listed in `required`, preserve optional semantics by converting optional fields to nullable types or by keeping them inside branch-specific alternatives
- do not make `oneOf`, `anyOf`, `allOf`, or conditional branches unsatisfiable by requiring mutually exclusive fields at the same object level
- preserve nullable/optional fields by allowing `null` where needed rather than forcing branch alternatives to be simultaneously present
- recurse through `properties`, `items`, `oneOf`, `anyOf`, `allOf`, `not`, `if`, `then`, and `else`
- do not remove validation keywords that Codex accepts unless a focused test proves they are unsupported

The implementation must define a deterministic local strictness helper in tests. Passing the helper is necessary but not sufficient; the optional live smoke remains the final check against the real Codex CLI.

- [ ] **Step 3: Use the compiler in Codex Vault**

Update `_materialize_output_schema` to write the compiled schema instead of raw YAML schema source.

- [ ] **Step 4: Verify materialized schema**

Update runtime tests to read `actor_runtime/schemas/planner.schema.json` and assert it is strict. Also materialize `implementation_result.schema.json` in a focused test and assert it remains branch-satisfiable.

- [ ] **Step 5: Run checks**

```bash
uv run --python 3.12 pytest tests/actor_runtime/test_codex_output_schema.py tests/actor_runtime/test_codex_vault_runtime.py -q
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add src/rail/actor_runtime/output_schema.py src/rail/actor_runtime/codex_vault.py tests/actor_runtime/test_codex_output_schema.py tests/actor_runtime/test_codex_vault_runtime.py
git commit -m "fix: compile codex actor output schemas"
```

---

### Task 3: Add Attempt-Scoped Runtime Evidence

**Files:**
- Create: `src/rail/artifacts/run_attempts.py`
- Create: `tests/artifacts/test_run_attempts.py`
- Modify: `src/rail/actor_runtime/evidence.py`
- Modify: `src/rail/actor_runtime/runtime.py`
- Modify: `src/rail/actor_runtime/agents.py`
- Modify: `src/rail/supervisor/supervise.py`
- Modify: `src/rail/artifacts/terminal_summary.py`
- Modify: `src/rail/artifacts/projection.py`
- Modify: `tests/actor_runtime/test_codex_vault_runtime.py`
- Modify: `tests/actor_runtime/test_agents_runtime.py`
- Modify: `tests/actor_runtime/test_fake_runtime.py`
- Modify: `tests/actor_runtime_test_fixtures.py`
- Modify: `tests/integration_flow_fixtures.py`
- Modify: `tests/e2e/test_optional_live_sdk_smoke.py`
- Modify: `tests/e2e/test_optional_codex_vault_smoke.py`
- Modify: `tests/supervisor/test_routing.py`
- Modify: `tests/artifacts/test_projection.py`
- Modify: `tests/artifacts/test_terminal_summary.py`

- [ ] **Step 1: Write failing attempt allocation tests**

In `tests/artifacts/test_run_attempts.py`, cover:

```python
def test_allocate_run_attempt_uses_monotonic_refs(tmp_path):
    assert allocate_run_attempt(tmp_path) == "attempt-0001"
    assert allocate_run_attempt(tmp_path) == "attempt-0002"
    assert (tmp_path / "runs" / "attempt-0001").is_dir()
    assert (tmp_path / "runs" / "attempt-0002").is_dir()


def test_allocate_run_attempt_rejects_unsafe_existing_path(tmp_path):
    runs = tmp_path / "runs"
    runs.mkdir()
    (runs / "attempt-0001").symlink_to(tmp_path)
    with pytest.raises(ValueError, match="unsafe run attempt"):
        allocate_run_attempt(tmp_path)
```

- [ ] **Step 2: Add failing runtime evidence tests**

Update direct runtime tests to build invocations with an explicit attempt ref:

```python
attempt_ref = allocate_run_attempt(handle.artifact_dir)
invocation = build_invocation(handle, "planner", attempt_ref=attempt_ref)
result = runtime.run(invocation)
assert result.runtime_evidence_ref.as_posix().startswith("runs/attempt-0001/")
assert result.events_ref.as_posix().startswith("runs/attempt-0001/")
```

Apply the same rule to optional smoke tests and any direct runtime tests: attempts are allocated through `allocate_run_attempt()`, not by manually inventing directories.

Add a supervisor test that calls `rail.supervise(handle, runtime=fake_runtime)` twice and verifies both attempts keep separate evidence paths and the terminal summary points at the latest attempt only.

- [ ] **Step 3: Implement attempt allocation**

Implement `allocate_run_attempt(artifact_dir: Path) -> str`:

- creates `runs/`
- scans directories matching `attempt-NNNN`
- rejects symlinked or non-directory attempt entries
- creates the next attempt directory
- returns the ref string

- [ ] **Step 4: Carry attempt refs through invocations**

Add `attempt_ref: str` to `ActorInvocation`.

In `supervise_artifact`, allocate once per supervision call:

```python
attempt_ref = allocate_run_attempt(handle.artifact_dir)
...
invocation = build_invocation(..., attempt_ref=attempt_ref)
```

Use the same attempt for every actor in one supervisor call.

- [ ] **Step 5: Write attempt-scoped evidence**

Change `write_runtime_evidence` to write:

```text
runs/<attempt_ref>/<actor>.events.jsonl
runs/<attempt_ref>/<actor>.runtime_evidence.json
```

Require `attempt_ref` for new writes. Existing tests should be updated rather than preserving the old flat write path.

- [ ] **Step 6: Update projections and evaluator evidence discovery**

Update `run_status.yaml` to record the current `attempt_ref`.

Update `_runtime_evidence_refs`, terminal summary, and result projection to read evidence refs for the recorded current attempt only. Terminal summary should only include refs produced by the current terminal run, not stale overwritten refs.

- [ ] **Step 7: Verify**

Run:

```bash
uv run --python 3.12 pytest tests/artifacts/test_run_attempts.py tests/actor_runtime/test_codex_vault_runtime.py tests/actor_runtime/test_agents_runtime.py tests/actor_runtime/test_fake_runtime.py tests/supervisor/test_routing.py tests/artifacts/test_projection.py tests/artifacts/test_terminal_summary.py tests/e2e/test_optional_live_sdk_smoke.py tests/e2e/test_optional_codex_vault_smoke.py tests/integration/test_runtime_flow_slices.py -q
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add src/rail/artifacts/run_attempts.py src/rail/actor_runtime/evidence.py src/rail/actor_runtime/runtime.py src/rail/actor_runtime/agents.py src/rail/supervisor/supervise.py src/rail/artifacts/terminal_summary.py src/rail/artifacts/projection.py tests/artifacts/test_run_attempts.py tests/actor_runtime/test_codex_vault_runtime.py tests/actor_runtime/test_agents_runtime.py tests/actor_runtime/test_fake_runtime.py tests/actor_runtime_test_fixtures.py tests/integration_flow_fixtures.py tests/e2e/test_optional_live_sdk_smoke.py tests/e2e/test_optional_codex_vault_smoke.py tests/supervisor/test_routing.py tests/artifacts/test_projection.py tests/artifacts/test_terminal_summary.py
git commit -m "feat: scope runtime evidence by run attempt"
```

---

### Task 4: Tolerate Codex Auth Operational Files Without Copying Them

**Files:**
- Modify: `src/rail/auth/credentials.py`
- Modify: `src/rail/actor_runtime/vault_env.py`
- Modify: `src/rail/cli/setup_commands.py`
- Modify: `tests/auth/test_codex_auth.py`
- Modify: `tests/actor_runtime/test_codex_vault_environment.py`
- Modify: `tests/cli/test_setup_commands.py`

- [ ] **Step 1: Write failing auth tests**

Add tests where Rail-owned auth home contains:

```text
auth.json
log/
memories/
tmp/
```

Expected:

- `validate_codex_auth_material()` returns only `auth.json`
- `materialize_vault_environment()` copies only `auth.json`
- `rail auth status` reports ready

Keep tests that reject unknown files such as `config.toml`, `skills/`, `plugins/`, `mcp/`, `hooks/`, and symlinks.

Also add tests that known operational entries are safe only when they are non-symlink directories. A symlinked `log`, `memories`, or `tmp` entry must fail closed, and a regular file using one of those directory names must fail closed.

- [ ] **Step 2: Implement source auth selection**

In `credentials.py`, separate source-home validation from copied material selection:

- known operational directories may exist in the Rail-owned auth home
- only `auth.json` is returned as accepted material
- `auth.json` remains required, non-symlinked, regular, and private
- known operational entries must be non-symlink directories
- known operational entries are never copied into actor-local `CODEX_HOME`
- unknown config/plugin/skill material still fails closed

- [ ] **Step 3: Verify actor-local materialization**

In `vault_env.py`, keep `_CODEX_AUTH_COPY_ALLOWLIST = {"auth.json"}` and assert copied material remains `["auth.json"]`.

- [ ] **Step 4: Run checks**

```bash
uv run --python 3.12 pytest tests/auth/test_codex_auth.py tests/actor_runtime/test_codex_vault_environment.py tests/cli/test_setup_commands.py -q
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add src/rail/auth/credentials.py src/rail/actor_runtime/vault_env.py src/rail/cli/setup_commands.py tests/auth/test_codex_auth.py tests/actor_runtime/test_codex_vault_environment.py tests/cli/test_setup_commands.py
git commit -m "fix: tolerate codex auth operational files"
```

---

### Task 5: Make Sandbox Ignore Policy Consistent

**Files:**
- Modify: `src/rail/workspace/sandbox.py`
- Modify: `tests/workspace/test_execution_isolation.py`
- Modify: `docs/ARCHITECTURE.md`

- [ ] **Step 1: Write failing sandbox tests**

Add tests proving:

- hardlinks under `.harness/` do not block sandbox creation
- hardlinks outside ignored paths still block sandbox creation
- symlinks under `.git/` or `.harness/` do not block because ignored trees are never copied
- symlinks outside ignored paths still block

Example:

```python
def test_create_sandbox_skips_harness_during_link_prescan(tmp_path):
    target = tmp_path / "target"
    target.mkdir()
    (target / "app.txt").write_text("ok", encoding="utf-8")
    harness_file = target / ".harness" / "artifacts" / "old" / "applypatch"
    harness_file.parent.mkdir(parents=True)
    harness_file.write_text("tool", encoding="utf-8")
    os.link(harness_file, harness_file.parent / "applypatch-hardlink")

    sandbox = create_sandbox(target)

    assert (sandbox.sandbox_root / "app.txt").is_file()
    assert not (sandbox.sandbox_root / ".harness").exists()
```

- [ ] **Step 2: Implement shared ignore decision**

In `sandbox.py`, define one ignored top-level set:

```python
_SANDBOX_IGNORED_TOP_LEVEL = {".git", ".harness"}
```

Use the same helper for `_reject_target_links()` and `shutil.copytree(ignore=...)`.

- [ ] **Step 3: Keep digest semantics explicit**

Do not hide target mutation by changing the existing tree digest behavior unless a focused test requires it. If digest still includes `.harness`, document that sandbox copy ignore and target mutation digest are separate decisions.

- [ ] **Step 4: Run checks**

```bash
uv run --python 3.12 pytest tests/workspace/test_execution_isolation.py -q
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add src/rail/workspace/sandbox.py tests/workspace/test_execution_isolation.py docs/ARCHITECTURE.md
git commit -m "fix: align sandbox prescan ignore policy"
```

---

### Task 6: Audit Codex Vault After Each Actor Run

**Files:**
- Modify: `src/rail/actor_runtime/vault_audit.py`
- Modify: `src/rail/actor_runtime/codex_vault.py`
- Modify: `tests/actor_runtime/test_codex_vault_environment.py`
- Modify: `tests/actor_runtime/test_codex_vault_runtime.py`
- Modify: `tests/evaluator/test_gate.py`

- [ ] **Step 1: Write failing post-run contamination tests**

Update the fake runner or test fixture so a callback can access `environ["CODEX_HOME"]`, then create these under actor-local `codex_home` during a successful fake Codex command:

```text
plugins/
skills/
config.toml
mcp/
hooks/
rules/
```

Expected:

```python
assert result.status == "interrupted"
assert result.blocked_category == "policy"
assert "plugin" in result.structured_output["error"] or "skill" in result.structured_output["error"]
```

Also assert no evaluator pass can be accepted when runtime evidence contains a post-run policy violation.

Add focused failing tests for post-run contamination on non-success paths too:

- malformed JSON / `CodexEventParseError`
- nonzero Codex command exit
- schema-valid command result with actor output validation failure

- [ ] **Step 2: Extend vault audit**

Keep `audit_vault_materialization()` usable both before and after Codex execution. It should inspect actor-local `CODEX_HOME` and reject:

- `skills`
- `plugins`
- `mcp`
- `hooks`
- `rules`
- `config.toml`
- `config.json`
- `settings.json`
- non-allowlisted top-level files
- symlinks

- [ ] **Step 3: Run audit after subprocess execution**

In `CodexVaultActorRuntime.run()`, call post-run vault audit after any Codex subprocess has run, including normal return, malformed JSON / `CodexEventParseError`, nonzero exit, and actor output validation failure paths.

The audit must run before:

- command return code success handling
- final output validation success
- `ActorResult(status="succeeded")`

If violation exists, write policy-blocked evidence with raw and normalized events.

- [ ] **Step 4: Include violation evidence**

Runtime evidence should include:

```json
"policy_violation": {"reason": "..."},
"codex_vault_home_ref": "actor_runtime/actors/<actor>/<invocation>/codex_home"
```

No secret values or machine-specific home paths.

- [ ] **Step 5: Run checks**

```bash
uv run --python 3.12 pytest tests/actor_runtime/test_codex_vault_environment.py tests/actor_runtime/test_codex_vault_runtime.py tests/evaluator/test_gate.py -q
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add src/rail/actor_runtime/vault_audit.py src/rail/actor_runtime/codex_vault.py tests/actor_runtime/test_codex_vault_environment.py tests/actor_runtime/test_codex_vault_runtime.py tests/evaluator/test_gate.py
git commit -m "fix: audit codex vault after actor execution"
```

---

### Task 7: Reduce Actor Shell Friction With Minimal Context Guidance

**Files:**
- Modify: `src/rail/actor_runtime/runtime.py`
- Modify: `.harness/actors/context_builder.md`
- Modify: `assets/defaults/actors/context_builder.md`
- Modify: `src/rail/package_assets/defaults/actors/context_builder.md`
- Modify: `tests/actor_runtime/test_actor_catalog.py`
- Modify: `tests/actor_runtime/test_codex_vault_runtime.py`
- Modify: `tests/build/test_package_assets.py`

- [ ] **Step 1: Write failing prompt/input tests**

Add a test that `build_invocation(handle, "context_builder", ...)` includes a compact runtime contract in `input`:

```json
{
  "runtime_contract": {
    "filesystem_scope": "sandbox_relative_paths_only",
    "forbidden_shell_patterns": ["..", "|", "&&", ";", "$", "`"],
    "result_source": "rail_result_projection_only"
  }
}
```

Do not make this long. This is a reminder, not the safety boundary.

- [ ] **Step 2: Add minimal actor wording**

In context builder actor prompts, add only this class of guidance:

```markdown
Use sandbox-relative paths only. Do not inspect parent directories. Do not use shell pipelines or compound shell operators. Prefer direct commands such as `rg -n PATTERN path`, `find path -maxdepth N -type f -print`, and `sed -n A,Bp path`.
```

Do not add large policy explanations.

- [ ] **Step 3: Keep runtime policy authoritative**

Do not loosen `_shell_event_policy_violation()` to accept pipes or parent traversal. The prompt reduces friction; Runtime still blocks policy violations.

- [ ] **Step 4: Verify**

```bash
uv run --python 3.12 pytest tests/actor_runtime/test_actor_catalog.py tests/actor_runtime/test_codex_vault_runtime.py tests/build/test_package_assets.py -q
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add src/rail/actor_runtime/runtime.py .harness/actors/context_builder.md assets/defaults/actors/context_builder.md src/rail/package_assets/defaults/actors/context_builder.md tests/actor_runtime/test_actor_catalog.py tests/actor_runtime/test_codex_vault_runtime.py tests/build/test_package_assets.py
git commit -m "docs: add minimal actor runtime contract"
```

---

### Task 8: Strengthen Optional Codex Vault Live Smoke

**Files:**
- Modify: `tests/e2e/test_optional_codex_vault_smoke.py`
- Modify: `docs/ARCHITECTURE.md`
- Modify: `docs/SPEC.md`

- [ ] **Step 1: Write smoke expectations**

When `RAIL_CODEX_VAULT_LIVE_SMOKE=1`, the smoke must assert:

- `rail auth doctor` is ready
- a minimal actor reaches structured output
- materialized output schema is strict
- runtime evidence is attempt-scoped
- actor-local `CODEX_HOME` contains no forbidden `skills`, `plugins`, MCP, hooks, rules, or config after the run

- [ ] **Step 2: Keep live smoke opt-in**

Do not make live Codex execution mandatory for normal CI. Leave `scripts/release_gate.sh` unchanged unless a concrete missing release-gate behavior is proven by a focused test.

- [ ] **Step 3: Use a tiny temporary target**

The live smoke target should contain only:

```text
README.md
src/example.txt
```

No real user project paths. No machine-specific home path in docs or committed fixtures.

- [ ] **Step 4: Verify non-live behavior**

Run:

```bash
uv run --python 3.12 pytest tests/e2e/test_optional_codex_vault_smoke.py -q
```

Expected without env flag: PASS with live smoke skipped.

- [ ] **Step 5: Document operator verification**

In `docs/SPEC.md`, document the opt-in command:

```bash
RAIL_CODEX_VAULT_LIVE_SMOKE=1 uv run --python 3.12 pytest tests/e2e/test_optional_codex_vault_smoke.py -q
```

- [ ] **Step 6: Commit**

```bash
git add tests/e2e/test_optional_codex_vault_smoke.py docs/ARCHITECTURE.md docs/SPEC.md
git commit -m "test: harden optional codex vault smoke"
```

---

### Task 9: Final Verification And Release Decision

**Files:**
- Modify only if prior verification exposes a concrete issue.

- [ ] **Step 1: Run focused suites**

```bash
uv run --python 3.12 pytest tests/auth tests/actor_runtime tests/workspace tests/supervisor tests/artifacts tests/build -q
```

Expected: PASS.

- [ ] **Step 2: Run lint and typing**

```bash
uv run --python 3.12 ruff check src tests
uv run --python 3.12 mypy src/rail
```

Expected: PASS.

- [ ] **Step 3: Run full release gate**

```bash
scripts/release_gate.sh
```

Expected: PASS. Optional live smokes may be skipped unless explicitly enabled.

- [ ] **Step 4: Inspect active terminology**

```bash
uv run --python 3.12 pytest tests/docs/test_removed_runtime_surfaces.py -q
```

Expected:

- PASS
- no removed runtime surface or incorrect provider spelling appears in active docs, skills, code, or tests
- historical docs may be left only if the removed-surface guard explicitly allows them as historical records

- [ ] **Step 5: Manual release decision**

Release may proceed only if:

- skill blocked behavior is locked
- repeated supervision produces attempt-scoped evidence
- strict schemas are materialized
- sandbox ignores `.git` and `.harness` consistently
- post-run `codex_vault` contamination blocks success
- auth operational files no longer require manual cleanup
- optional live smoke passes when the operator enables it

- [ ] **Step 6: Commit any final docs/check updates**

```bash
git status --short
git add <only-intended-files>
git commit -m "chore: verify codex vault policy flow"
```

---

## Execution Notes

Prefer the task order above. Do not start by expanding actor prompts. Prompt text is a friction reducer only; Runtime checks decide safety.

If a task reveals that Codex CLI always materializes plugins or skills despite isolation flags, keep the fail-closed behavior and document the exact blocked outcome. Do not weaken the policy to pass the smoke.

If same-artifact retries are needed during debugging, they must produce a new attempt directory. The user-facing skill should still stop after the first blocked projection unless the user explicitly asks to inspect or resume.
