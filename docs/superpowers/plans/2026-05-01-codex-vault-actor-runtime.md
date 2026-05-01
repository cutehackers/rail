# Codex Vault Actor Runtime Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `codex_vault` the default local Actor Runtime provider while preserving Rail-owned policy, artifact evidence, patch-bundle mutation, and `rail.specify` as the public request API.

**Architecture:** Rail remains Python-first. The supervisor selects an Actor Runtime from effective policy; `codex_vault` prepares a Rail-controlled Codex environment, verifies command/auth readiness, runs one actor invocation in an artifact-local `CODEX_HOME`, and writes auditable evidence before any evaluator decision. `openai_agents_sdk` remains available only when Rail/operator defaults explicitly select it and SDK readiness succeeds.

**Tech Stack:** Python 3.12, Pydantic contracts, PyYAML policy files, Codex CLI subprocess execution, pytest, ruff, mypy.

---

## Scope And Guardrails

This plan implements the design in `docs/superpowers/specs/2026-04-30-codex-vault-actor-runtime-design.md`.

Do not revive the Go CLI or Go runtime. Do not add a task-execution CLI contract. `rail auth` remains setup and diagnostics only. Normal task execution remains Rail skill plus Python API: `rail.specify`, `rail.start_task`, `rail.supervise`, `rail.status`, and `rail.result`.

Target mutation must continue to happen only through Rail-validated patch bundles. The actor process may propose patch bundles through structured output, but it must not mutate the target directly.

## File Structure

Create:

- `src/rail/actor_runtime/factory.py` - chooses the concrete Actor Runtime from effective policy.
- `src/rail/actor_runtime/codex_vault.py` - `CodexVaultActorRuntime`, command readiness, subprocess adapter, event capture.
- `src/rail/actor_runtime/vault_env.py` - artifact-local `CODEX_HOME` materialization, auth allowlist copy, env scrub.
- `src/rail/actor_runtime/vault_audit.py` - contamination and runtime policy audit for skills/plugins/MCP/hooks/rules/config.
- `tests/actor_runtime/test_codex_vault_readiness.py` - deterministic readiness and command identity tests.
- `tests/actor_runtime/test_codex_vault_environment.py` - deterministic sealed environment and auth materialization tests.
- `tests/actor_runtime/test_codex_vault_runtime.py` - fake Codex execution, output parsing, evidence, blocked outcomes.

Modify:

- `src/rail/policy/schema.py` - add `codex_vault` to `RuntimeProvider`.
- `src/rail/policy/load.py` - load Rail/operator default policy from a trusted override path when explicitly configured.
- `src/rail/policy/validate.py` - keep target-local provider changes blocked; allow operator/default provider to be `codex_vault`.
- `.harness/supervisor/actor_runtime.yaml` - make `codex_vault` the default provider.
- `assets/defaults/supervisor/actor_runtime.yaml` - same default as repo policy.
- `src/rail/package_assets/defaults/supervisor/actor_runtime.yaml` - same default as repo policy.
- `src/rail/supervisor/supervise.py` - build runtime through `actor_runtime.factory`.
- `src/rail/actor_runtime/__init__.py` - export new runtime/factory types as needed.
- `src/rail/actor_runtime/runtime.py` - add provider-neutral readiness protocol only if needed.
- `src/rail/actor_runtime/evidence.py` - support richer event/evidence payloads without leaking secrets.
- `src/rail/auth/credentials.py` - add Codex auth-home discovery and auth-material validation.
- `src/rail/cli/main.py` - add setup-only `rail auth status` and `rail auth doctor` subcommands.
- `src/rail/cli/setup_commands.py` and `src/rail/cli/doctor.py` - remove API-key assumptions from normal setup readiness; add Codex Vault readiness reports.
- `tests/policy/test_policy_v2.py` - provider contract and narrowing tests.
- `tests/supervisor/test_routing.py` - default supervisor uses `codex_vault` and blocks safely when not ready.
- `tests/cli/test_setup_commands.py` - auth diagnostics and no default API-key requirement.
- `tests/build/test_package_assets.py` - packaged policy default and bundled skill asset checks.
- `tests/docs/test_removed_runtime_surfaces.py` - guard against removed request API names and incorrect provider spellings in active docs.
- `docs/SPEC.md`, `docs/ARCHITECTURE.md`, `docs/CONVENTIONS.md` - canonical runtime/provider contract.
- `skills/rail/SKILL.md`, `assets/skill/Rail/SKILL.md`, `src/rail/package_assets/skill/Rail/SKILL.md` - skill-facing setup/readiness wording.
- `scripts/check_installed_wheel.py` - expect `codex_vault` as the packaged default.

---

### Task 1: Finalize `rail.specify` Contract

**Files:**
- Modify: `src/rail/api.py`
- Modify: `src/rail/__init__.py`
- Modify: `src/rail/request/schema.py`
- Modify: `tests/request/test_compose_request.py`
- Modify: `tests/test_api_smoke.py`
- Modify: `scripts/check_installed_wheel.py`
- Modify: `docs/SPEC.md`
- Modify: `docs/ARCHITECTURE.md`
- Modify: `skills/rail/SKILL.md`
- Modify: `assets/skill/Rail/SKILL.md`
- Modify: `src/rail/package_assets/skill/Rail/SKILL.md`

- [x] **Step 1: Write or confirm the public API smoke test**

Ensure `tests/test_api_smoke.py` asserts:

```python
def test_public_api_exports_harness_operations():
    assert callable(rail.specify)
    assert not hasattr(rail, "normalize_request")
    assert callable(rail.start_task)
    assert callable(rail.supervise)
    assert callable(rail.status)
    assert callable(rail.result)
```

- [x] **Step 2: Run the focused API tests and verify they fail before implementation**

Run:

```bash
uv run --python 3.12 pytest tests/test_api_smoke.py tests/request/test_compose_request.py -q
```

Expected before implementation: FAIL if the removed request API is still exported or tests still call it.

- [x] **Step 3: Implement the API rename**

Expose only:

```python
def specify(draft: Any) -> HarnessRequest:
    return normalize_draft(draft)
```

Make `start_task(draft)` call `specify(draft)`. Remove the old request API export from `src/rail/__init__.py`.

- [x] **Step 4: Keep the internal request-version validator scoped**

In `src/rail/request/schema.py`, keep the request-version validator named `_validate_request_version`. Do not reintroduce the removed public request API function or export.

- [x] **Step 5: Update docs, skills, package smoke, and active examples**

Replace active user-facing examples with:

```python
request = rail.specify(draft)
```

- [x] **Step 6: Run focused checks**

Run:

```bash
uv run --python 3.12 pytest tests/test_api_smoke.py tests/request/test_compose_request.py -q
rg -n "rail\\.normalize_request|def normalize_request|from rail\\.api import .*normalize_request|\\\"normalize_request\\\"" src tests docs skills assets scripts AGENTS.md
```

Expected: pytest PASS. `rg` returns no public API, docs, skill, script, or test call sites for the removed request API. The request-version validator uses `_validate_request_version`.

- [x] **Step 7: Commit**

```bash
git add src/rail/api.py src/rail/__init__.py src/rail/request/schema.py tests/request/test_compose_request.py tests/test_api_smoke.py scripts/check_installed_wheel.py docs/SPEC.md docs/ARCHITECTURE.md skills/rail/SKILL.md assets/skill/Rail/SKILL.md src/rail/package_assets/skill/Rail/SKILL.md
git commit -m "refactor: rename request API to specify"
```

---

### Task 2: Add `codex_vault` Provider Policy

**Files:**
- Modify: `src/rail/policy/schema.py`
- Modify: `src/rail/policy/load.py`
- Modify: `src/rail/policy/validate.py`
- Modify: `.harness/supervisor/actor_runtime.yaml`
- Modify: `assets/defaults/supervisor/actor_runtime.yaml`
- Modify: `src/rail/package_assets/defaults/supervisor/actor_runtime.yaml`
- Modify: `tests/policy/test_policy_v2.py`
- Modify: `tests/build/test_package_assets.py`
- Modify: `scripts/check_installed_wheel.py`

- [x] **Step 1: Write failing provider tests**

Add tests:

```python
def test_default_actor_runtime_policy_loads_codex_vault():
    policy = load_effective_policy(Path("."))
    assert policy.runtime.provider == "codex_vault"
    assert policy.tools.shell.enabled is True
    assert set(policy.tools.shell.allowlist) == {"pwd", "ls", "find", "rg", "sed", "cat", "wc", "head", "tail", "stat", "test"}


def test_openai_agents_sdk_policy_is_valid_when_operator_default_selects_it(tmp_path):
    data = load_effective_policy(Path(".")).model_dump(mode="json")
    data["runtime"]["provider"] = "openai_agents_sdk"
    data["tools"]["shell"]["enabled"] = False
    data["tools"]["shell"]["allowlist"] = []
    policy = ActorRuntimePolicyV2.model_validate(data)
    assert policy.runtime.provider == "openai_agents_sdk"
    assert policy.tools.shell.enabled is False


def test_openai_agents_sdk_policy_rejects_enabled_host_tools(tmp_path):
    data = load_effective_policy(Path(".")).model_dump(mode="json")
    data["runtime"]["provider"] = "openai_agents_sdk"
    data["tools"]["shell"]["enabled"] = True
    with pytest.raises(ValueError, match="openai_agents_sdk"):
        ActorRuntimePolicyV2.model_validate(data)


def test_operator_default_policy_path_can_select_openai_agents_sdk(tmp_path, monkeypatch):
    target = tmp_path / "target"
    target.mkdir()
    policy_path = tmp_path / "operator-policy.yaml"
    data = load_effective_policy(Path(".")).model_dump(mode="json")
    data["runtime"]["provider"] = "openai_agents_sdk"
    data["tools"]["shell"]["enabled"] = False
    data["tools"]["shell"]["allowlist"] = []
    policy_path.write_text(yaml.safe_dump(data), encoding="utf-8")
    monkeypatch.setenv("RAIL_OPERATOR_ACTOR_RUNTIME_POLICY", str(policy_path))
    policy = load_effective_policy(target)
    assert policy.runtime.provider == "openai_agents_sdk"
    assert policy.tools.shell.enabled is False


def test_unknown_runtime_provider_is_rejected():
    data = load_effective_policy(Path(".")).model_dump(mode="json")
    data["runtime"]["provider"] = "codex_vualt"
    with pytest.raises(ValueError, match="provider"):
        ActorRuntimePolicyV2.model_validate(data)


def test_target_policy_cannot_change_provider_to_openai_agents_sdk(tmp_path):
    base = load_effective_policy(tmp_path)
    overlay = base.model_copy(deep=True)
    overlay.runtime.provider = "openai_agents_sdk"
    with pytest.raises(ValueError, match="runtime.provider"):
        narrow_policy(base, overlay)
```

- [x] **Step 2: Run provider tests and verify they fail**

Run:

```bash
uv run --python 3.12 pytest tests/policy/test_policy_v2.py tests/build/test_package_assets.py -q
```

Expected before implementation: FAIL because `RuntimeProvider` does not allow `codex_vault` and default YAML still uses `openai_agents_sdk`.

- [x] **Step 3: Add provider values and provider-specific tool validation**

Change:

```python
RuntimeProvider = Literal["codex_vault", "openai_agents_sdk"]
```

Add a model validator that rejects `openai_agents_sdk` when shell, filesystem, network, or MCP tools are enabled. `codex_vault` is the only provider in this plan that may use Rail-gated read-only shell inspection.

Keep `narrow_policy()` rejecting any target-local provider change.

- [x] **Step 4: Add trusted operator default policy loading**

In `src/rail/policy/load.py`, support:

```text
RAIL_OPERATOR_ACTOR_RUNTIME_POLICY=/absolute/path/to/operator/actor_runtime.yaml
```

Rules:

- path must be absolute
- file must exist
- file must not be symlinked
- file must not be under the target repository
- file must not be group-writable or world-writable
- when unset, Rail uses packaged defaults
- target-local `.harness/supervisor/actor_runtime.yaml` still only narrows the selected base policy

- [x] **Step 5: Change default policy assets**

In all three default policy files, change `runtime.provider` from `openai_agents_sdk` to `codex_vault`. Preserve the existing `model`, `timeout_seconds`, `actor_runtime`, `workspace`, `capabilities`, and `approval_policy` sections.

Also express the Rail-gated read-only inspection policy explicitly:

```yaml
tools:
  shell:
    enabled: true
    allowlist: [pwd, ls, find, rg, sed, cat, wc, head, tail, stat, test]
    timeout_seconds: 30
    max_output_bytes: 4000
```

Keep filesystem, network, and MCP tools disabled.

```yaml
runtime:
  provider: codex_vault
  model: gpt-5.2
  timeout_seconds: 180
```

- [x] **Step 6: Update installed-wheel smoke**

In `scripts/check_installed_wheel.py`, assert:

```python
assert policy.runtime.provider == "codex_vault"
```

- [x] **Step 7: Run focused policy checks**

Run:

```bash
uv run --python 3.12 pytest tests/policy/test_policy_v2.py tests/build/test_package_assets.py -q
```

Expected: PASS.

- [x] **Step 8: Commit**

```bash
git add src/rail/policy/schema.py src/rail/policy/load.py src/rail/policy/validate.py .harness/supervisor/actor_runtime.yaml assets/defaults/supervisor/actor_runtime.yaml src/rail/package_assets/defaults/supervisor/actor_runtime.yaml tests/policy/test_policy_v2.py tests/build/test_package_assets.py scripts/check_installed_wheel.py
git commit -m "feat: add codex vault runtime provider policy"
```

---

### Task 3: Add Runtime Factory

**Files:**
- Create: `src/rail/actor_runtime/factory.py`
- Modify: `src/rail/actor_runtime/runtime.py`
- Modify: `src/rail/supervisor/supervise.py`
- Modify: `src/rail/actor_runtime/__init__.py`
- Test: `tests/actor_runtime/test_runtime_factory.py`
- Test: `tests/supervisor/test_routing.py`

- [x] **Step 1: Write failing factory tests**

Create `tests/actor_runtime/test_runtime_factory.py`:

```python
from pathlib import Path

import pytest

from rail.actor_runtime.factory import build_actor_runtime
from rail.actor_runtime.agents import AgentsActorRuntime
from rail.policy import load_effective_policy


def test_factory_builds_codex_vault_for_default_policy(tmp_path):
    runtime = build_actor_runtime(project_root=Path("."), policy=load_effective_policy(tmp_path))
    assert runtime.__class__.__name__ == "CodexVaultActorRuntime"


def test_factory_builds_agents_runtime_when_policy_selects_sdk(tmp_path):
    policy = load_effective_policy(tmp_path)
    policy = policy.model_copy(update={"runtime": policy.runtime.model_copy(update={"provider": "openai_agents_sdk"})})
    runtime = build_actor_runtime(project_root=Path("."), policy=policy)
    assert isinstance(runtime, AgentsActorRuntime)


def test_factory_rejects_unknown_provider_shape(tmp_path):
    policy = load_effective_policy(tmp_path)
    policy.runtime.provider = "unknown"
    with pytest.raises(ValueError, match="unsupported runtime provider"):
        build_actor_runtime(project_root=Path("."), policy=policy)
```

- [x] **Step 2: Run factory tests and verify they fail**

Run:

```bash
uv run --python 3.12 pytest tests/actor_runtime/test_runtime_factory.py -q
```

Expected: FAIL because `factory.py` and `CodexVaultActorRuntime` do not exist.

- [x] **Step 3: Add factory**

Implement:

```python
def build_actor_runtime(*, project_root: Path, policy: ActorRuntimePolicyV2) -> ActorRuntime:
    if policy.runtime.provider == "codex_vault":
        return CodexVaultActorRuntime(project_root=project_root, policy=policy)
    if policy.runtime.provider == "openai_agents_sdk":
        return AgentsActorRuntime(project_root=project_root, policy=policy)
    raise ValueError(f"unsupported runtime provider: {policy.runtime.provider}")
```

- [x] **Step 4: Add target root to actor invocation**

In `src/rail/actor_runtime/runtime.py`, add:

```python
target_root: Path
```

to `ActorInvocation`. Set it from `handle.project_root` in `build_invocation()`. Add a focused assertion in `tests/supervisor/test_routing.py` that captured actor invocations include the target repository root, not the Rail source root.

- [x] **Step 5: Add temporary safe-blocking `CodexVaultActorRuntime` skeleton**

In `src/rail/actor_runtime/codex_vault.py`, add a runtime whose `run()` writes runtime evidence and returns interrupted with `blocked_category="environment"` until readiness is implemented.

- [x] **Step 6: Wire supervisor**

Replace direct `AgentsActorRuntime(...)` construction in `supervise_artifact()` with:

```python
runtime = runtime or build_actor_runtime(project_root=_rail_root(), policy=policy)
```

- [x] **Step 7: Run focused supervisor/factory tests**

Run:

```bash
uv run --python 3.12 pytest tests/actor_runtime/test_runtime_factory.py tests/supervisor/test_routing.py::test_default_supervise_blocks_environment_when_runtime_not_ready -q
```

Expected: PASS. Default supervision blocks safely, not terminal pass.

- [x] **Step 8: Commit**

```bash
git add src/rail/actor_runtime/factory.py src/rail/actor_runtime/codex_vault.py src/rail/actor_runtime/runtime.py src/rail/actor_runtime/__init__.py src/rail/supervisor/supervise.py tests/actor_runtime/test_runtime_factory.py tests/supervisor/test_routing.py
git commit -m "feat: select actor runtime from policy"
```

---

### Task 4: Codex Command Readiness

**Files:**
- Modify: `src/rail/actor_runtime/codex_vault.py`
- Test: `tests/actor_runtime/test_codex_vault_readiness.py`

- [x] **Step 1: Write failing readiness tests**

Add tests for:

```python
def test_readiness_blocks_when_codex_command_missing(tmp_path):
    runtime = CodexVaultActorRuntime(project_root=Path("."), policy=load_effective_policy(tmp_path), command_resolver=lambda: None)
    readiness = runtime.readiness()
    assert readiness.ready is False
    assert readiness.blocked_category == "environment"
    assert "Codex command" in readiness.reason


def test_readiness_blocks_unsupported_codex_version(tmp_path):
    runner = FakeCodexRunner(version_stdout="codex-cli 0.0.0")
    command = _fake_codex_command(tmp_path)
    runtime = CodexVaultActorRuntime(
        project_root=Path("."),
        policy=load_effective_policy(tmp_path),
        command_resolver=lambda: command,
        command_trust_checker=lambda _path, _target_root, _artifact_dir: None,
        runner=runner,
    )
    assert runtime.readiness().ready is False


def test_readiness_accepts_supported_command_identity(tmp_path):
    runner = FakeCodexRunner(version_stdout="codex-cli 0.124.0")
    command = _fake_codex_command(tmp_path)
    runtime = CodexVaultActorRuntime(
        project_root=Path("."),
        policy=load_effective_policy(tmp_path),
        command_resolver=lambda: command,
        command_trust_checker=lambda _path, _target_root, _artifact_dir: None,
        runner=runner,
    )
    assert runtime.readiness().ready is True


def _fake_codex_command(tmp_path: Path) -> Path:
    command = tmp_path / "bin" / "codex"
    command.parent.mkdir()
    command.write_text("#!/bin/sh\nexit 0\n", encoding="utf-8")
    command.chmod(0o755)
    return command
```

Use injected fakes. Do not require real Codex in deterministic tests.

- [x] **Step 2: Run readiness tests and verify they fail**

Run:

```bash
uv run --python 3.12 pytest tests/actor_runtime/test_codex_vault_readiness.py -q
```

Expected: FAIL because readiness injection points are missing.

- [x] **Step 3: Implement readiness model and injected command adapter**

Add:

```python
class CodexCommandReadiness(BaseModel):
    ready: bool
    reason: str
    command_path: Path | None = None
    codex_version: str | None = None
    blocked_category: Literal["environment"] | None = None
```

Resolve command with `shutil.which("codex")` by default. Verify supported version and required flags through the injected runner. If ambiguous, block.

Expose a `command_trust_checker` injection point for deterministic tests. Production readiness must use the default trust checker.

Use these concrete readiness rules:

- command name: `codex`
- version command: `codex --version`
- accepted version format: `codex-cli MAJOR.MINOR.PATCH`
- minimum supported version: `0.124.0`
- unsupported or unparsable version: block with `blocked_category="environment"`
- required command surface: `codex exec --help` must mention `--json`, `--output-schema`, `--ignore-user-config`, `--ignore-rules`, `--ephemeral`, `--sandbox`, and `--cd`
- required execution flags: `exec --json --output-schema /absolute/path/to/actor-output-schema.json --ignore-user-config --ignore-rules --ephemeral --sandbox read-only --cd /absolute/path/to/sandbox -c shell_environment_policy.inherit=none`
- forbidden execution flag: never pass `--dangerously-bypass-approvals-and-sandbox`
- command trust rule: the unresolved path returned by `shutil.which("codex")` must be absolute and under `/opt/homebrew/bin`, `/usr/local/bin`, or `/usr/bin`; after resolving symlinks, the canonical target must be under `/opt/homebrew/Caskroom/codex`, `/opt/homebrew/bin`, `/usr/local/bin`, or `/usr/bin`
- command path safety rule: the unresolved and resolved command paths must exist, must not be group-writable or world-writable, and must not live under a temporary directory such as `/tmp` or `/var/tmp`
- local calibration note: local Codex CLI `0.124.0` supports the listed `exec` flags; readiness tests remain authoritative for release behavior

- [x] **Step 4: Run focused tests**

Run:

```bash
uv run --python 3.12 pytest tests/actor_runtime/test_codex_vault_readiness.py -q
```

Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add src/rail/actor_runtime/codex_vault.py tests/actor_runtime/test_codex_vault_readiness.py
git commit -m "feat: preflight codex vault command readiness"
```

---

### Task 5: Rail-Owned Codex Auth Home

**Files:**
- Modify: `src/rail/auth/credentials.py`
- Modify: `src/rail/actor_runtime/codex_vault.py`
- Modify: `src/rail/cli/main.py`
- Modify: `src/rail/cli/setup_commands.py`
- Modify: `src/rail/cli/doctor.py`
- Test: `tests/auth/test_codex_auth.py`
- Test: `tests/cli/test_setup_commands.py`

- [x] **Step 1: Write failing auth tests**

Add deterministic tests:

```python
def test_codex_auth_home_defaults_to_rail_owned_location(tmp_path, monkeypatch):
    monkeypatch.setenv("RAIL_HOME", str(tmp_path / "rail-home"))
    assert codex_auth_home(environ=os.environ) == tmp_path / "rail-home" / "codex"


def test_auth_material_allowlist_rejects_unknown_files(tmp_path):
    auth_home = tmp_path / "auth"
    auth_home.mkdir()
    (auth_home / "auth.json").write_text("{}", encoding="utf-8")
    (auth_home / "skills").mkdir()
    with pytest.raises(ValueError, match="unknown auth material"):
        validate_codex_auth_material(auth_home)


def test_auth_material_rejects_group_or_world_writable_auth_file(tmp_path):
    auth_home = tmp_path / "auth"
    auth_home.mkdir()
    auth_file = auth_home / "auth.json"
    auth_file.write_text("{}", encoding="utf-8")
    auth_file.chmod(0o666)
    with pytest.raises(ValueError, match="unsafe auth material permissions"):
        validate_codex_auth_material(auth_home)
```

- [x] **Step 2: Run auth tests and verify they fail**

Run:

```bash
uv run --python 3.12 pytest tests/auth/test_codex_auth.py tests/cli/test_setup_commands.py -q
```

Expected: FAIL because Codex auth helpers and `rail auth` commands do not exist.

- [x] **Step 3: Implement auth-home helpers**

Add functions with no user-home paths in docs:

```python
def codex_auth_home(*, environ: Mapping[str, str]) -> Path:
    root = Path(environ.get("RAIL_HOME", "") or Path.home() / ".rail")
    return root / "codex"


def validate_codex_auth_material(auth_home: Path) -> list[Path]:
    allowed = {"auth.json"}
    material = list(auth_home.iterdir())
    unknown = [path.name for path in material if path.name not in allowed]
    if unknown:
        raise ValueError("unknown auth material")
    auth_file = auth_home / "auth.json"
    if not auth_file.is_file():
        raise ValueError("missing auth.json")
    if auth_file.stat().st_mode & 0o022:
        raise ValueError("unsafe auth material permissions")
    return [auth_file]
```

The initial auth-material allowlist is exactly `auth.json`. Unknown material blocks readiness. The auth home and `auth.json` must not be group-writable or world-writable. Missing `auth.json` blocks readiness. Future Codex auth shapes must be added by changing this allowlist and tests explicitly.

- [x] **Step 4: Add setup-only `rail auth` commands**

Add:

```bash
rail auth login
rail auth status
rail auth doctor
```

`rail auth login` runs `codex login` with `CODEX_HOME` set to the Rail-owned Codex auth home. `rail auth status` checks local auth material and permissions only. `rail auth doctor` checks auth material plus command readiness and may run a minimal auth validation only behind an explicit live-check flag. Do not add task execution commands. All reports must be secret-safe.

- [x] **Step 5: Remove default API-key requirement from setup doctor**

`rail doctor` should not require `OPENAI_API_KEY` when default provider is `codex_vault`. It may warn or diagnose SDK credentials only when policy selects `openai_agents_sdk`.

- [x] **Step 6: Run focused auth/CLI tests**

Run:

```bash
uv run --python 3.12 pytest tests/auth/test_codex_auth.py tests/cli/test_setup_commands.py -q
```

Expected: PASS.

- [x] **Step 7: Commit**

```bash
git add src/rail/auth/credentials.py src/rail/actor_runtime/codex_vault.py src/rail/cli/main.py src/rail/cli/setup_commands.py src/rail/cli/doctor.py tests/auth/test_codex_auth.py tests/cli/test_setup_commands.py
git commit -m "feat: add codex vault auth diagnostics"
```

---

### Task 6: Sealed Actor Environment Materialization

**Files:**
- Create: `src/rail/actor_runtime/vault_env.py`
- Modify: `src/rail/actor_runtime/codex_vault.py`
- Test: `tests/actor_runtime/test_codex_vault_environment.py`

- [x] **Step 1: Write failing environment tests**

Add tests:

```python
def test_materialize_vault_env_uses_artifact_local_codex_home(tmp_path):
    artifact_dir = tmp_path / "artifact"
    auth_home = tmp_path / "auth"
    auth_home.mkdir()
    (auth_home / "auth.json").write_text("{}", encoding="utf-8")
    env = materialize_vault_environment(artifact_dir=artifact_dir, auth_home=auth_home, base_environ={"HOME": "/should/not/leak"})
    assert env.codex_home == artifact_dir / "actor_runtime" / "codex_home"
    assert env.environ["CODEX_HOME"] == str(env.codex_home)
    assert env.environ["HOME"] == str(env.codex_home)


def test_materialize_vault_env_does_not_copy_user_codex_surfaces_from_environment(tmp_path):
    artifact_dir = tmp_path / "artifact"
    auth_home = tmp_path / "auth"
    auth_home.mkdir()
    (auth_home / "auth.json").write_text("{}", encoding="utf-8")
    user_codex_home = tmp_path / "user-codex"
    for name in ("skills", "plugins", "mcp", "hooks", "rules"):
        (user_codex_home / name).mkdir(parents=True)
    env = materialize_vault_environment(
        artifact_dir=artifact_dir,
        auth_home=auth_home,
        base_environ={"CODEX_HOME": str(user_codex_home)},
    )
    for name in ("skills", "plugins", "mcp", "hooks", "rules"):
        assert not (env.codex_home / name).exists()
```

- [x] **Step 2: Run environment tests and verify they fail**

Run:

```bash
uv run --python 3.12 pytest tests/actor_runtime/test_codex_vault_environment.py -q
```

Expected: FAIL because `vault_env.py` does not exist.

- [x] **Step 3: Implement materialization**

Create an artifact-local layout:

```text
artifact/actor_runtime/codex_home/
artifact/actor_runtime/evidence/
```

Copy only allowlisted auth files. Generate minimal config only if the supported Codex version requires it. Do not copy skills, plugins, MCP config, hooks, rules, or general user config.

For the initial implementation, copy only `auth.json`. Do not copy `config.toml`; actor execution uses `--ignore-user-config` and explicit flags instead.

- [x] **Step 4: Scrub process environment**

Only preserve required process variables such as `PATH`, `TMPDIR`, and explicit Rail-controlled paths. Set `HOME` and `CODEX_HOME` to the artifact-local Codex home.

- [x] **Step 5: Run focused environment tests**

Run:

```bash
uv run --python 3.12 pytest tests/actor_runtime/test_codex_vault_environment.py -q
```

Expected: PASS.

- [x] **Step 6: Commit**

```bash
git add src/rail/actor_runtime/vault_env.py src/rail/actor_runtime/codex_vault.py tests/actor_runtime/test_codex_vault_environment.py
git commit -m "feat: materialize codex vault environment"
```

---

### Task 7: Codex Vault Execution And Output Parsing

**Files:**
- Modify: `src/rail/actor_runtime/codex_vault.py`
- Modify: `src/rail/actor_runtime/evidence.py`
- Modify: `.harness/actors/generator.md`
- Modify: `.harness/templates/implementation_result.schema.yaml`
- Modify: `assets/defaults/actors/generator.md`
- Modify: `assets/defaults/templates/implementation_result.schema.yaml`
- Modify: `src/rail/package_assets/defaults/actors/generator.md`
- Modify: `src/rail/package_assets/defaults/templates/implementation_result.schema.yaml`
- Use: `src/rail/workspace/sandbox.py`
- Test: `tests/actor_runtime/test_codex_vault_runtime.py`
- Test: `tests/supervisor/test_routing.py`

- [x] **Step 1: Write failing execution tests**

Use a fake runner that returns JSON events and a final structured output:

```python
def test_codex_vault_runtime_validates_actor_output_and_writes_evidence(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    runner = FakeCodexRunner(final_output={"summary": "Plan", "likely_files": [], "substeps": [], "risks": [], "acceptance_criteria_refined": []})
    command = _fake_codex_command(tmp_path)
    runtime = CodexVaultActorRuntime(
        project_root=Path("."),
        policy=load_effective_policy(tmp_path),
        runner=runner,
        command_resolver=lambda: command,
        command_trust_checker=lambda _path, _target_root, _artifact_dir: None,
    )
    result = runtime.run(build_invocation(handle, "planner"))
    assert result.status == "succeeded"
    assert result.runtime_evidence_ref.as_posix().endswith("planner.runtime_evidence.json")
```

Add a second test where fake Codex returns invalid output and runtime blocks.

- [x] **Step 2: Run execution tests and verify they fail**

Run:

```bash
uv run --python 3.12 pytest tests/actor_runtime/test_codex_vault_runtime.py -q
```

Expected: FAIL because execution adapter does not parse output.

- [x] **Step 3: Create actor sandbox before Codex execution**

Use `rail.workspace.sandbox.create_sandbox(invocation.target_root)` or an equivalent Rail-owned helper. The Codex process must run with `--cd` set to the resolved sandbox root, never the target root. Runtime evidence must record:

- sandbox root reference
- target pre-run tree digest
- sandbox base tree digest
- post-run target tree digest

If the target tree digest changes before Rail applies a validated patch bundle, return a policy-blocked `ActorResult`.

Perform per-invocation path safety here, not in global `readiness()`: the unresolved and resolved Codex command paths must not be inside `invocation.artifact_dir`, must not be inside the target repository, and must not be inside the sandbox root.

- [x] **Step 4: Align generator patch-bundle output contract**

Update generator prompt and schema copies so the Generator is explicitly allowed to return exactly one of:

- `patch_bundle_ref`
- inline `patch_bundle`
- no patch when the correct result is read-only

Keep direct target mutation forbidden. Add or update tests proving a generator-produced patch bundle from `codex_vault` is accepted only through Rail patch validation and then applied by supervisor.

- [x] **Step 5: Materialize actor output schema file**

Before running Codex, write the actor output JSON schema to:

```text
artifact/actor_runtime/schemas/{actor}.schema.json
```

Use the actor catalog's schema source or the output model's JSON schema. Runtime evidence must include:

- `output_schema_ref`
- `output_schema_digest`

- [x] **Step 6: Implement subprocess adapter behind injection seam**

Default adapter runs the trusted Codex command with the sealed environment and timeout from policy:

```bash
codex exec --json --output-schema /absolute/path/to/actor-output-schema.json --ignore-user-config --ignore-rules --ephemeral --sandbox read-only --cd /absolute/path/to/sandbox -c shell_environment_policy.inherit=none -
```

The prompt is passed on stdin. Deterministic tests use an injected fake. Do not depend on real Codex in unit tests.

- [x] **Step 7: Gate Codex tool and shell use**

For the initial implementation, `codex_vault` allows only Rail-gated read-only repository inspection inside the sandbox. Do not pass `--full-auto`. Do not pass `--dangerously-bypass-approvals-and-sandbox`.

Allowed shell events must satisfy all of these checks:

- working directory is the sandbox root or a child of it
- command path is not absolute unless it resolves to a trusted system binary
- command has no shell redirection, pipe, command substitution, backgrounding, or chaining
- command executable is one of `pwd`, `ls`, `find`, `rg`, `sed`, `cat`, `wc`, `head`, `tail`, `stat`, or `test`
- command arguments do not reference the target root, artifact directory, Rail source checkout, user Codex home, or Rail-owned Codex auth home

Block MCP invocation, plugin invocation, validation command execution, write-capable shell events, and any shell event that cannot be parsed as read-only. Validation remains Rail-owned.

- [x] **Step 8: Validate structured output using actor catalog**

Use the same actor catalog output validation behavior as `AgentsActorRuntime`.

- [x] **Step 9: Persist raw/normalized events**

Runtime evidence must include:

- provider: `codex_vault`
- actor
- command readiness summary
- sealed environment summary without secrets
- auth materialization status
- output schema reference and digest
- structured output or blocked reason
- policy violation evidence when applicable

- [x] **Step 10: Run focused runtime tests**

Run:

```bash
uv run --python 3.12 pytest tests/actor_runtime/test_codex_vault_runtime.py -q
```

Expected: PASS.

- [x] **Step 11: Commit**

```bash
git add src/rail/actor_runtime/codex_vault.py src/rail/actor_runtime/evidence.py .harness/actors/generator.md .harness/templates/implementation_result.schema.yaml assets/defaults/actors/generator.md assets/defaults/templates/implementation_result.schema.yaml src/rail/package_assets/defaults/actors/generator.md src/rail/package_assets/defaults/templates/implementation_result.schema.yaml tests/actor_runtime/test_codex_vault_runtime.py tests/supervisor/test_routing.py
git commit -m "feat: execute actors through codex vault"
```

---

### Task 8: Contamination Audit And Evaluator Block

**Files:**
- Create: `src/rail/actor_runtime/vault_audit.py`
- Modify: `src/rail/actor_runtime/codex_vault.py`
- Modify: `src/rail/evaluator/gate.py` only if current evidence checks cannot see runtime policy violations.
- Test: `tests/actor_runtime/test_codex_vault_runtime.py`
- Test: `tests/evaluator/test_gate.py`
- Test: `tests/supervisor/test_routing.py`

- [x] **Step 1: Write failing contamination tests**

Add tests for fake event/materialization evidence:

```python
def test_parent_skill_contamination_blocks_actor(tmp_path):
    runner = FakeCodexRunner(events=[{"type": "skill_invocation", "name": "Rail"}])
    result = runtime.run(build_invocation(handle, "planner"))
    assert result.status == "interrupted"
    assert result.blocked_category == "policy"
    assert "skill" in result.structured_output["error"]


def test_plugin_or_mcp_contamination_blocks_actor(tmp_path):
    runner = FakeCodexRunner(events=[{"type": "mcp_invocation", "server": "filesystem"}])
    result = runtime.run(build_invocation(handle, "planner"))
    assert result.status == "interrupted"
    assert result.blocked_category == "policy"
    assert "MCP" in result.structured_output["error"]
```

- [x] **Step 2: Run audit tests and verify they fail**

Run:

```bash
uv run --python 3.12 pytest tests/actor_runtime/test_codex_vault_runtime.py tests/evaluator/test_gate.py tests/supervisor/test_routing.py -q
```

Expected: FAIL because contamination audit is missing.

- [x] **Step 3: Implement audit**

Detect:

- user or parent skill materialization
- user plugin materialization
- MCP config materialization
- hook materialization
- user rule materialization
- unexpected config inheritance
- direct target mutation outside Rail patch apply
- auth material outside the allowlist

Add focused tests for unsupported isolation flags, unsafe auth-home permissions, expired or invalid auth material, and direct target mutation detection.

Return a policy-blocked `ActorResult` before evaluator routing.

- [x] **Step 4: Ensure evaluator cannot pass after policy violation**

If a runtime policy violation reaches evidence, evaluator gate must block terminal success. Prefer blocking at actor runtime before evaluator; add evaluator gate coverage for defense in depth.

- [x] **Step 5: Run focused audit/gate tests**

Run:

```bash
uv run --python 3.12 pytest tests/actor_runtime/test_codex_vault_runtime.py tests/evaluator/test_gate.py tests/supervisor/test_routing.py -q
```

Expected: PASS.

- [x] **Step 6: Commit**

```bash
git add src/rail/actor_runtime/vault_audit.py src/rail/actor_runtime/codex_vault.py src/rail/evaluator/gate.py tests/actor_runtime/test_codex_vault_runtime.py tests/evaluator/test_gate.py tests/supervisor/test_routing.py
git commit -m "feat: block codex vault contamination"
```

---

### Task 9: Supervisor Projection And Blocked Results

**Files:**
- Modify: `src/rail/supervisor/supervise.py`
- Modify: `src/rail/artifacts/projection.py`
- Modify: `src/rail/artifacts/terminal_summary.py`
- Test: `tests/supervisor/test_routing.py`
- Test: `tests/artifacts/test_projection.py`
- Test: `tests/artifacts/test_terminal_summary.py`

- [x] **Step 1: Write failing projection tests**

Add tests proving:

- missing Codex command projects as `environment` blocked
- contamination projects as `policy` blocked
- terminal pass is not recorded after runtime policy violation
- no secret values appear in result projection or terminal summary

- [x] **Step 2: Run projection tests and verify they fail**

Run:

```bash
uv run --python 3.12 pytest tests/supervisor/test_routing.py tests/artifacts/test_projection.py tests/artifacts/test_terminal_summary.py -q
```

Expected: FAIL where result labels or summaries do not yet distinguish Codex Vault readiness/policy failures.

- [x] **Step 3: Implement projection updates**

Keep projection artifact-only. Do not call Codex or SDK from projection code. Add only stable blocked labels and secret-safe reason projection.

- [x] **Step 4: Run focused projection tests**

Run:

```bash
uv run --python 3.12 pytest tests/supervisor/test_routing.py tests/artifacts/test_projection.py tests/artifacts/test_terminal_summary.py -q
```

Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add src/rail/supervisor/supervise.py src/rail/artifacts/projection.py src/rail/artifacts/terminal_summary.py tests/supervisor/test_routing.py tests/artifacts/test_projection.py tests/artifacts/test_terminal_summary.py
git commit -m "feat: project codex vault blocked outcomes"
```

---

### Task 10: Documentation, Skill, And Guards

**Files:**
- Modify: `docs/SPEC.md`
- Modify: `docs/ARCHITECTURE.md`
- Modify: `docs/CONVENTIONS.md`
- Modify: `skills/rail/SKILL.md`
- Modify: `assets/skill/Rail/SKILL.md`
- Modify: `src/rail/package_assets/skill/Rail/SKILL.md`
- Modify: `tests/docs/test_removed_runtime_surfaces.py`
- Modify: `tests/docs/test_no_home_paths.py`

- [x] **Step 1: Write docs guard tests**

Extend active-doc guards:

```python
def test_active_docs_do_not_use_removed_request_api_or_wrong_provider_name():
    forbidden = [
        "rail.normalize_request",
        "def normalize_request",
        "sealed_codex",
        "vault_codex",
        "actor backend",
        "SDK-backed actors",
        "SDK-powered",
        "SDK traces",
        "SDK trace",
        "SDK Actor Runtime",
        "SDK-adapter",
        "SDK adapter",
    ]
    active_docs = [
        Path("README.md"),
        Path("README-kr.md"),
        Path("docs/SPEC.md"),
        Path("docs/ARCHITECTURE.md"),
        Path("docs/CONVENTIONS.md"),
        Path("docs/tasks.md"),
        Path("skills/rail/SKILL.md"),
        Path("assets/skill/Rail/SKILL.md"),
        Path("src/rail/package_assets/skill/Rail/SKILL.md"),
    ]
    for path in active_docs:
        text = path.read_text(encoding="utf-8")
        for term in forbidden:
            assert term not in text
```

Keep historical docs allowed only when explicitly framed as historical records.

- [x] **Step 2: Run docs tests and verify they fail before docs cleanup**

Run:

```bash
uv run --python 3.12 pytest tests/docs/test_removed_runtime_surfaces.py tests/docs/test_no_home_paths.py -q
```

Expected before cleanup: FAIL if any active doc still has removed terms or home-directory examples.

- [x] **Step 3: Update canonical docs**

Document:

- `codex_vault` is the default local Actor Runtime provider
- `openai_agents_sdk` is optional for operator/API-key environments
- `rail auth` is setup/diagnostics only
- target mutation remains patch-bundle only
- `rail.specify` is the only public request API

- [x] **Step 4: Update Rail skill copies**

Keep all three copies aligned. Normal user flow should not require request YAML, wrapper details, API keys, or runtime flags.

- [x] **Step 5: Run docs/skill checks**

Run:

```bash
uv run --python 3.12 pytest tests/docs/test_removed_runtime_surfaces.py tests/docs/test_no_home_paths.py tests/build/test_package_assets.py -q
rg -n "rail\\.normalize_request|def normalize_request|sealed_codex|vault_codex|actor backend|SDK-backed actors|SDK-powered|SDK traces|SDK trace|SDK Actor Runtime|SDK-adapter|SDK adapter" README.md README-kr.md docs/SPEC.md docs/ARCHITECTURE.md docs/CONVENTIONS.md docs/tasks.md skills/rail/SKILL.md assets/skill/Rail/SKILL.md src/rail/package_assets/skill/Rail/SKILL.md
```

Expected: pytest PASS. `rg` returns no matches.

- [x] **Step 6: Commit**

```bash
git add docs/SPEC.md docs/ARCHITECTURE.md docs/CONVENTIONS.md skills/rail/SKILL.md assets/skill/Rail/SKILL.md src/rail/package_assets/skill/Rail/SKILL.md tests/docs/test_removed_runtime_surfaces.py tests/docs/test_no_home_paths.py tests/build/test_package_assets.py
git commit -m "docs: document codex vault runtime contract"
```

---

### Task 11: Optional Live Smoke

**Files:**
- Modify: `tests/e2e/test_optional_live_sdk_smoke.py` or create `tests/e2e/test_optional_codex_vault_smoke.py`
- Modify: `scripts/release_gate.sh`

- [x] **Step 1: Write opt-in smoke test**

Create a test that skips unless an explicit environment flag is set:

```python
pytestmark = pytest.mark.skipif(
    os.environ.get("RAIL_CODEX_VAULT_LIVE_SMOKE") != "1",
    reason="codex_vault live smoke is opt-in",
)
```

The smoke must be non-mutating and use a temporary target.

- [x] **Step 2: Run without flag**

Run:

```bash
uv run --python 3.12 pytest tests/e2e/test_optional_codex_vault_smoke.py -q
```

Expected: SKIPPED.

- [x] **Step 3: Add release-gate hook**

Only run the live smoke when `RAIL_CODEX_VAULT_LIVE_SMOKE=1`. Do not make local release checks require live Codex by default.

- [x] **Step 4: Run focused e2e checks**

Run:

```bash
uv run --python 3.12 pytest tests/e2e/test_optional_codex_vault_smoke.py -q
```

Expected without flag: SKIPPED.

- [x] **Step 5: Commit**

```bash
git add tests/e2e/test_optional_codex_vault_smoke.py scripts/release_gate.sh
git commit -m "test: add optional codex vault live smoke"
```

---

### Task 12: Full Verification

**Files:**
- No implementation files unless verification exposes a bug.

- [x] **Step 1: Run focused suite**

Run:

```bash
uv run --python 3.12 pytest tests/request/test_compose_request.py tests/policy/test_policy_v2.py tests/actor_runtime tests/supervisor/test_routing.py tests/auth tests/cli tests/docs tests/build -q
```

Expected: PASS.

- [x] **Step 2: Run full test suite**

Run:

```bash
uv run --python 3.12 pytest -q
```

Expected: PASS.

- [x] **Step 3: Run lint**

Run:

```bash
uv run --python 3.12 ruff check src tests
```

Expected: PASS.

- [x] **Step 4: Run typing**

Run:

```bash
uv run --python 3.12 mypy src/rail
```

Expected: PASS.

- [x] **Step 5: Run docs/string guards**

Run:

```bash
uv run --python 3.12 pytest tests/docs/test_removed_runtime_surfaces.py -q
```

Expected: PASS. Keep removed request API names, incorrect provider spellings, and stale runtime terms out of active product surfaces through allowlisted pytest guards rather than broad repository text searches that self-match historical plans.

- [x] **Step 6: Commit verification fixes if needed**

Stage the files changed by verification fixes and commit them with:

```bash
git commit -m "fix: stabilize codex vault runtime checks"
```

---

## Final Acceptance

- [x] `rail.specify(draft)` is the only public request specification API.
- [x] The removed request API is not exported, documented, or used in active product code.
- [x] `codex_vault` is the default local Actor Runtime provider in repo and packaged policy defaults.
- [x] Default setup and migration docs do not require `OPENAI_API_KEY`; SDK credentials remain optional operator-only inputs.
- [x] Incorrect provider spellings and old provider names do not appear in active product surfaces.
- [x] `openai_agents_sdk` remains available for explicit operator/API-key environments.
- [x] Target-local policy cannot select or switch runtime provider.
- [x] Missing Codex command, unsupported version, unsupported isolation capability, missing auth, unknown auth material, and contamination block before actor execution or evaluator success.
- [x] Actor execution uses artifact-local `CODEX_HOME` and does not inherit parent/user skills, plugins, MCP config, hooks, rules, or general config.
- [x] Runtime evidence records readiness, environment summary, auth materialization, event stream, structured output, and policy violations without leaking secrets.
- [x] Target mutation remains patch-bundle only.
- [x] `rail auth` remains setup/diagnostics only.
- [x] No Go CLI/runtime path is revived.
