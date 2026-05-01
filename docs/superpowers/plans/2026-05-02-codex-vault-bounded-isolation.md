# Codex Vault Bounded Isolation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the `codex_vault` isolation contract explicit and testable by separating Codex-owned bootstrap material, capability provenance, and policy-forbidden capability use.

**Architecture:** Keep Actor Runtime isolation and target mutation controls unchanged. Refactor `codex_vault` audit into focused contracts: a Codex bootstrap profile, a structured audit violation model, material/provenance checks, and capability-use checks that do not block passive discovery/cache events. Runtime evidence should carry structured policy violation details while terminal results keep secret-safe human-readable reasons.

**Tech Stack:** Python 3.12, Pydantic contracts, pathlib, tomllib, pytest, ruff, mypy, Rail artifact evidence.

---

## Scope And Guardrails

This plan implements the design in `docs/superpowers/specs/2026-05-02-codex-vault-bounded-isolation-design.md`.

Do not remove Actor Runtime isolation. Do not allow parent/user Codex skills, MCP servers, hooks, rules, plugin tools, or target-local policy broadening. Do not create a task-execution CLI contract. Normal task execution remains Rail skill plus Python API: `rail.specify`, `rail.start_task`, `rail.supervise`, `rail.status`, and `rail.result`.

The key behavioral change is not "allow more things." It is:

- allow passive Codex-owned actor-local bootstrap material
- continue blocking parent/user/target-local/behavior-affecting unknown provenance
- continue blocking Rail-policy-forbidden capability use
- stop blocking metadata/discovery/cache events just because they mention words like `skill`, `plugin`, or `config`

## File Structure

Create:

- `src/rail/actor_runtime/codex_bootstrap_profile.py` - explicit actor-local Codex bootstrap material profile; no event scanning.
- `tests/actor_runtime/test_codex_bootstrap_profile.py` - focused profile tests for allowed bootstrap and blocked unsafe/profile-mismatch material.

Modify:

- `src/rail/actor_runtime/vault_audit.py` - define `VaultAuditViolation`, return structured violations, delegate bootstrap shape checks to `codex_bootstrap_profile`, and split materialization/provenance/capability-use checks.
- `src/rail/actor_runtime/codex_vault.py` - consume structured `VaultAuditViolation` objects, write structured `policy_violation` evidence, and keep `ActorResult.structured_output["error"]` as a secret-safe string.
- `tests/actor_runtime/test_vault_audit.py` - update string-return assertions to structured violation assertions and add provenance/capability tests.
- `tests/actor_runtime/test_codex_vault_runtime.py` - update runtime policy evidence assertions and add passive discovery event regression tests.
- `docs/SPEC.md` - document bounded isolation as the canonical product contract.
- `docs/ARCHITECTURE.md` - document the bootstrap/provenance/capability audit layers in Actor Runtime architecture.
- `tests/docs/test_removed_runtime_surfaces.py` or a new docs guard test - protect active docs from banned terms and ensure bounded isolation wording remains present.

Do not modify:

- Public request API names.
- `.harness` actor graph.
- Patch bundle apply semantics.
- Validation runner authority.

---

### Task 1: Document The Product Contract

**Files:**
- Modify: `docs/SPEC.md`
- Modify: `docs/ARCHITECTURE.md`
- Modify or create: `tests/docs/test_removed_runtime_surfaces.py`

- [ ] **Step 1: Write the docs guard test**

Add a focused test that checks active product docs describe bounded isolation and avoid removed terminology:

```python
from pathlib import Path


def test_active_docs_define_codex_vault_bounded_isolation():
    combined = "\n".join(
        Path(path).read_text(encoding="utf-8")
        for path in ("docs/SPEC.md", "docs/ARCHITECTURE.md")
    )
    assert "bounded isolation" in combined
    assert "passive Codex-owned bootstrap" in combined
    assert "capability use" in combined
    assert "provenance" in combined
    forbidden_terms = ["actor " + "backend", "vault_" + "codex", "normalize_" + "request"]
    for term in forbidden_terms:
        assert term not in combined.lower()
```

- [ ] **Step 2: Run the docs guard and verify it fails**

Run:

```bash
uv run --python 3.12 pytest tests/docs/test_removed_runtime_surfaces.py -q
```

Expected: FAIL because `docs/SPEC.md` and `docs/ARCHITECTURE.md` do not yet define the bounded isolation contract.

- [ ] **Step 3: Update `docs/SPEC.md`**

Add a concise subsection under the Actor Runtime or policy boundary section:

```markdown
### Codex Vault Bounded Isolation

`codex_vault` provides bounded isolation. Rail does not require the
actor-local `CODEX_HOME` to remain empty. Rail does require that parent,
user, target-local, or behavior-affecting unknown capabilities do not flow
into actor execution.

Passive Codex-owned bootstrap material, discovery metadata, and cache state may
exist inside the actor-local `CODEX_HOME`. Capability use remains governed by
Rail policy. Parent or user skills, plugin tools, MCP servers, hooks, rules,
inherited config, direct target mutation, and actor-invented validation remain
blocked.
```

- [ ] **Step 4: Update `docs/ARCHITECTURE.md`**

In the Actor Runtime section, describe three audit layers:

```markdown
The `codex_vault` provider audits actor execution in three layers:

- bootstrap profile audit recognizes passive Codex-owned actor-local material
- provenance audit blocks parent/user/target-local or behavior-affecting
  unknown capability sources
- capability-use audit blocks Rail-policy-forbidden tool, skill, plugin, MCP,
  hook, rule, config, target mutation, or validation behavior
```

- [ ] **Step 5: Run focused docs tests**

Run:

```bash
uv run --python 3.12 pytest tests/docs/test_removed_runtime_surfaces.py tests/docs/test_no_home_paths.py -q
rg -n 'actor[ ]backend|vault[_]codex|normalize[_]request|/Users/|~/' docs/SPEC.md docs/ARCHITECTURE.md
```

Expected: pytest PASS. `rg` returns no matches.

- [ ] **Step 6: Commit**

```bash
git add docs/SPEC.md docs/ARCHITECTURE.md tests/docs/test_removed_runtime_surfaces.py
git commit -m "docs: define bounded codex vault isolation"
```

---

### Task 2: Add Structured Vault Audit Violations

**Files:**
- Modify: `src/rail/actor_runtime/vault_audit.py`
- Modify: `tests/actor_runtime/test_vault_audit.py`

- [ ] **Step 1: Write failing tests for structured violations**

Update existing assertions from string comparisons to structured checks:

```python
def test_vault_audit_rejects_user_skill_materialization(tmp_path):
    artifact_dir = tmp_path / "artifact"
    env = _vault_environment(artifact_dir)
    env.codex_home.mkdir(parents=True)
    (env.codex_home / "auth.json").write_text("{}", encoding="utf-8")
    user_skill = env.codex_home / "skills" / "rail"
    user_skill.mkdir(parents=True)
    (user_skill / "SKILL.md").write_text("# Rail\n", encoding="utf-8")

    violation = audit_vault_materialization(env, artifact_dir=artifact_dir)

    assert violation is not None
    assert violation.code == "user_skill_materialized"
    assert violation.audit_layer == "provenance"
    assert violation.reason == "user-controlled skill materialized in actor-local CODEX_HOME"
    assert violation.path_ref == "actor_runtime/codex_home/skills/rail"
```

Add a second test for auth material:

```python
def test_vault_audit_reports_unknown_auth_material_code(tmp_path):
    artifact_dir = tmp_path / "artifact"
    env = _vault_environment(artifact_dir).model_copy(update={"copied_auth_material": ["auth.json", "session.db"]})
    env.codex_home.mkdir(parents=True)
    (env.codex_home / "auth.json").write_text("{}", encoding="utf-8")

    violation = audit_vault_materialization(env, artifact_dir=artifact_dir)

    assert violation is not None
    assert violation.code == "unknown_auth_material"
    assert violation.audit_layer == "materialization"
```

- [ ] **Step 2: Run tests and verify they fail**

Run:

```bash
uv run --python 3.12 pytest tests/actor_runtime/test_vault_audit.py -q
```

Expected: FAIL because `audit_vault_materialization` still returns `str | None`.

- [ ] **Step 3: Add the structured model**

In `src/rail/actor_runtime/vault_audit.py`, add:

```python
from typing import Literal

from pydantic import BaseModel, ConfigDict


VaultAuditLayer = Literal["materialization", "bootstrap", "provenance", "capability"]


class VaultAuditViolation(BaseModel):
    model_config = ConfigDict(extra="forbid")

    category: Literal["policy"] = "policy"
    code: str
    reason: str
    audit_layer: VaultAuditLayer
    path_ref: str | None = None
```

Add a helper:

```python
def _violation(*, code: str, reason: str, audit_layer: VaultAuditLayer, path: Path | None = None, artifact_dir: Path | None = None) -> VaultAuditViolation:
    path_ref = None
    if path is not None and artifact_dir is not None:
        try:
            path_ref = path.relative_to(artifact_dir).as_posix()
        except ValueError:
            path_ref = None
    return VaultAuditViolation(code=code, reason=reason, audit_layer=audit_layer, path_ref=path_ref)
```

- [ ] **Step 4: Convert materialization returns**

Change:

```python
def audit_vault_materialization(...) -> str | None:
```

to:

```python
def audit_vault_materialization(...) -> VaultAuditViolation | None:
```

Replace string returns with `_violation(...)` while preserving human-readable reasons.

- [ ] **Step 5: Run focused tests**

Run:

```bash
uv run --python 3.12 pytest tests/actor_runtime/test_vault_audit.py -q
```

Expected: PASS after updating all assertions.

- [ ] **Step 6: Commit**

```bash
git add src/rail/actor_runtime/vault_audit.py tests/actor_runtime/test_vault_audit.py
git commit -m "refactor: structure codex vault audit violations"
```

---

### Task 3: Extract The Codex Bootstrap Profile

**Files:**
- Create: `src/rail/actor_runtime/codex_bootstrap_profile.py`
- Create: `tests/actor_runtime/test_codex_bootstrap_profile.py`
- Modify: `src/rail/actor_runtime/vault_audit.py`
- Modify: `tests/actor_runtime/test_vault_audit.py`

- [ ] **Step 1: Write profile tests**

Create `tests/actor_runtime/test_codex_bootstrap_profile.py`:

```python
from pathlib import Path

from rail.actor_runtime.codex_bootstrap_profile import bootstrap_profile_violation


def test_bootstrap_profile_allows_passive_codex_material(tmp_path):
    codex_home = tmp_path / "codex_home"
    codex_home.mkdir()
    for directory in ("cache/codex_apps_tools", "plugins/cache/openai-curated/github/hash", "shell_snapshots", "tmp", "memories"):
        (codex_home / directory).mkdir(parents=True)
    (codex_home / ".tmp" / "plugins" / ".git").mkdir(parents=True)
    (codex_home / "installation_id").write_text("id\n", encoding="utf-8")
    (codex_home / "models_cache.json").write_text("{}", encoding="utf-8")
    (codex_home / "config.toml").write_text('[plugins."github@openai-curated"]\nenabled = true\n', encoding="utf-8")
    system_skills = codex_home / "skills" / ".system"
    system_skills.mkdir(parents=True)
    (system_skills / ".codex-system-skills.marker").write_text("marker\n", encoding="utf-8")
    (system_skills / "openai-docs").mkdir()

    violations = [bootstrap_profile_violation(path, codex_home=codex_home) for path in codex_home.iterdir()]

    assert all(violation is None for violation in violations)


def test_bootstrap_profile_rejects_custom_plugin_material(tmp_path):
    codex_home = tmp_path / "codex_home"
    custom_plugin = codex_home / "plugins" / "custom"
    custom_plugin.mkdir(parents=True)

    violation = bootstrap_profile_violation(codex_home / "plugins", codex_home=codex_home)

    assert violation is not None
    assert violation.code == "user_plugin_materialized"
    assert violation.audit_layer == "provenance"
```

- [ ] **Step 2: Run tests and verify they fail**

Run:

```bash
uv run --python 3.12 pytest tests/actor_runtime/test_codex_bootstrap_profile.py -q
```

Expected: FAIL because the module does not exist.

- [ ] **Step 3: Implement `codex_bootstrap_profile.py`**

Add a focused module:

```python
from __future__ import annotations

import tomllib
from pathlib import Path

from rail.actor_runtime.vault_audit import VaultAuditViolation

ALLOWED_OPERATIONAL_DIRS = {"cache", "log", "memories", "shell_snapshots", "tmp", ".tmp"}
ALLOWED_OPERATIONAL_FILES = {"installation_id", "models_cache.json"}
ALLOWED_CODEX_SYSTEM_SKILLS = {"imagegen", "openai-docs", "plugin-creator", "skill-creator", "skill-installer"}
FORBIDDEN_CODEX_HOME_ENTRIES = {
    "mcp": ("mcp_config_materialized", "MCP config materialization is not allowed"),
    "hooks": ("hook_materialized", "hook materialization is not allowed"),
    "rules": ("user_rule_materialized", "user rule materialization is not allowed"),
    "config.json": ("inherited_config_applied", "unexpected config inheritance is not allowed"),
    "settings.json": ("inherited_config_applied", "unexpected config inheritance is not allowed"),
}


def bootstrap_profile_violation(path: Path, *, codex_home: Path) -> VaultAuditViolation | None:
    ...
```

Implementation requirements:

- allowed operational directories must be directories and recursively symlink-free
- allowed operational files must be files and not symlinks
- `config.toml` must parse with `tomllib` and contain only OpenAI-curated plugin enable stanzas
- `skills` must contain only `.system`, a `.codex-system-skills.marker`, and allowlisted system skill directories
- `plugins` must contain only `cache`
- `mcp`, `hooks`, `rules`, `config.json`, and `settings.json` block
- unknown material returns `bootstrap_profile_mismatch` unless it is clearly user skill/plugin/config, in which case use the specific provenance code

- [ ] **Step 4: Remove duplicated profile logic from `vault_audit.py`**

Move constants and helper functions out of `vault_audit.py`:

- `_ALLOWED_OPERATIONAL_DIRS`
- `_ALLOWED_OPERATIONAL_FILES`
- `_ALLOWED_CODEX_SYSTEM_SKILLS`
- `_FORBIDDEN_CODEX_HOME_ENTRIES`
- `_config_toml_violation`
- `_skills_materialization_violation`
- `_plugins_materialization_violation`

Replace `_codex_home_entry_violation(child)` with:

```python
bootstrap_violation = bootstrap_profile_violation(child, codex_home=vault_environment.codex_home)
if bootstrap_violation is not None:
    return bootstrap_violation
```

- [ ] **Step 5: Run focused tests**

Run:

```bash
uv run --python 3.12 pytest tests/actor_runtime/test_codex_bootstrap_profile.py tests/actor_runtime/test_vault_audit.py -q
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add src/rail/actor_runtime/codex_bootstrap_profile.py src/rail/actor_runtime/vault_audit.py tests/actor_runtime/test_codex_bootstrap_profile.py tests/actor_runtime/test_vault_audit.py
git commit -m "refactor: extract codex bootstrap profile"
```

---

### Task 4: Make Capability Audit Precise

**Files:**
- Modify: `src/rail/actor_runtime/vault_audit.py`
- Modify: `src/rail/actor_runtime/codex_vault.py`
- Modify: `tests/actor_runtime/test_vault_audit.py`
- Modify: `tests/actor_runtime/test_codex_vault_runtime.py`

- [ ] **Step 1: Write tests that passive discovery does not block**

Add to `tests/actor_runtime/test_vault_audit.py`:

```python
from rail.actor_runtime.vault_audit import audit_codex_event_capabilities


def test_capability_audit_allows_passive_plugin_cache_discovery():
    events = [
        {"type": "event", "category": "plugin_cache", "message": "plugin cache synchronized"},
        {"type": "event", "category": "skill_registry", "message": "system skill registry indexed"},
        {"type": "event", "category": "config", "message": "actor-local config inspected"},
    ]

    violation = audit_codex_event_capabilities(events)

    assert violation is None
```

Add blocking tests:

```python
def test_capability_audit_blocks_plugin_tool_execution():
    events = [{"type": "tool_call", "tool": "plugin.github.search", "name": "github"}]

    violation = audit_codex_event_capabilities(events)

    assert violation is not None
    assert violation.code == "plugin_capability_used"
    assert violation.audit_layer == "capability"


def test_capability_audit_blocks_user_skill_invocation():
    events = [{"type": "skill_invocation", "name": "rail", "source": "user"}]

    violation = audit_codex_event_capabilities(events)

    assert violation is not None
    assert violation.code == "skill_capability_used"
```

- [ ] **Step 2: Write runtime regression test for passive discovery events**

In `tests/actor_runtime/test_codex_vault_runtime.py`, add:

```python
def test_codex_vault_runtime_allows_passive_codex_discovery_events(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    runner = FakeCodexRunner(
        final_output={
            "summary": "Plan",
            "likely_files": [],
            "substeps": [],
            "risks": [],
            "acceptance_criteria_refined": [],
        },
        extra_events=[
            {"type": "event", "category": "plugin_cache", "message": "plugin cache synchronized"},
            {"type": "event", "category": "skill_registry", "message": "system skill registry indexed"},
        ],
    )
    runtime = _runtime(tmp_path, command=_fake_codex_command(tmp_path), runner=runner)

    result = runtime.run(build_invocation(handle, "planner"))

    assert result.status == "succeeded"
```

- [ ] **Step 3: Run tests and verify they fail**

Run:

```bash
uv run --python 3.12 pytest tests/actor_runtime/test_vault_audit.py tests/actor_runtime/test_codex_vault_runtime.py::test_codex_vault_runtime_allows_passive_codex_discovery_events -q
```

Expected: FAIL because current keyword matching treats `skill`, `plugin`, or `config` mentions as contamination.

- [ ] **Step 4: Replace broad keyword matching**

Rename or replace:

```python
def audit_codex_event_contamination(events: list[dict[str, object]]) -> str | None:
```

with:

```python
def audit_codex_event_capabilities(events: list[dict[str, object]]) -> VaultAuditViolation | None:
```

Detection rules:

- Block MCP when event `type`, `tool`, or `kind` is a tool invocation and names MCP.
- Block plugin when event indicates a tool call, command call, or capability execution, not passive cache/discovery.
- Block skill when event type/kind is `skill_invocation`, `skill_execution`, or a tool/capability call sourced from `user`, `parent`, `target`, or unknown behavior-affecting source.
- Block hooks/rules/config only when event indicates application/execution/load that changes behavior, not passive actor-local inspection.
- Do not block events whose category/message merely mention `plugin_cache`, `skill_registry`, `metadata`, `discovery`, or `actor-local config inspected`.

Suggested helper:

```python
_CAPABILITY_EVENT_TYPES = {"tool_call", "tool_invocation", "function_call", "skill_invocation", "skill_execution", "mcp_call", "hook_execution", "rule_applied", "config_loaded"}
_PASSIVE_EVENT_CATEGORIES = {"plugin_cache", "skill_registry", "metadata", "discovery"}
```

- [ ] **Step 5: Update `_codex_event_policy_violation`**

In `src/rail/actor_runtime/codex_vault.py`, call the new capability audit:

```python
capability_violation = audit_codex_event_capabilities(events)
if capability_violation is not None:
    return capability_violation
```

If `_codex_event_policy_violation` still returns strings for shell policy violations, convert those through a helper in the next task. For this task, keep the smallest compatibility bridge if needed.

- [ ] **Step 6: Run focused tests**

Run:

```bash
uv run --python 3.12 pytest tests/actor_runtime/test_vault_audit.py tests/actor_runtime/test_codex_vault_runtime.py -q
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add src/rail/actor_runtime/vault_audit.py src/rail/actor_runtime/codex_vault.py tests/actor_runtime/test_vault_audit.py tests/actor_runtime/test_codex_vault_runtime.py
git commit -m "fix: distinguish passive discovery from capability use"
```

---

### Task 5: Write Structured Policy Violation Evidence

**Files:**
- Modify: `src/rail/actor_runtime/codex_vault.py`
- Modify: `tests/actor_runtime/test_codex_vault_runtime.py`
- Modify: `tests/artifacts/test_projection.py` if projections assert specific evidence shape
- Modify: `tests/supervisor/test_routing.py` if terminal summaries assert reason shape

- [ ] **Step 1: Write runtime evidence tests**

Add a test that policy evidence contains both a string reason and structured fields:

```python
def test_codex_vault_runtime_writes_structured_policy_violation(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    runner = FakeCodexRunner(
        final_output={
            "summary": "Plan",
            "likely_files": [],
            "substeps": [],
            "risks": [],
            "acceptance_criteria_refined": [],
        },
        extra_events=[{"type": "tool_call", "tool": "plugin.github.search", "name": "github"}],
    )
    runtime = _runtime(tmp_path, command=_fake_codex_command(tmp_path), runner=runner)

    result = runtime.run(build_invocation(handle, "planner"))

    evidence = json.loads((handle.artifact_dir / result.runtime_evidence_ref).read_text(encoding="utf-8"))
    assert result.status == "interrupted"
    assert result.blocked_category == "policy"
    assert result.structured_output["error"] == "plugin capability use is not allowed"
    assert evidence["policy_violation"]["code"] == "plugin_capability_used"
    assert evidence["policy_violation"]["audit_layer"] == "capability"
    assert evidence["policy_violation"]["reason"] == "plugin capability use is not allowed"
```

- [ ] **Step 2: Run test and verify it fails**

Run:

```bash
uv run --python 3.12 pytest tests/actor_runtime/test_codex_vault_runtime.py::test_codex_vault_runtime_writes_structured_policy_violation -q
```

Expected: FAIL because evidence currently stores only `{"reason": "..."}`.

- [ ] **Step 3: Add a policy violation evidence helper**

In `src/rail/actor_runtime/codex_vault.py`, add:

```python
from rail.actor_runtime.vault_audit import VaultAuditViolation


def _policy_violation_payload(violation: VaultAuditViolation | str) -> dict[str, object]:
    if isinstance(violation, VaultAuditViolation):
        return violation.model_dump(mode="json", exclude_none=True)
    return {"reason": violation}


def _policy_violation_reason(violation: VaultAuditViolation | str) -> str:
    return violation.reason if isinstance(violation, VaultAuditViolation) else violation
```

- [ ] **Step 4: Update all policy block call sites**

Every place that currently does:

```python
extra={"policy_violation": {"reason": materialization_violation}}
structured_output={"error": materialization_violation}
error=materialization_violation
```

should become:

```python
reason = _policy_violation_reason(violation)
extra={"policy_violation": _policy_violation_payload(violation)}
structured_output={"error": reason}
error=reason
```

Keep shell policy violations working by converting shell strings to structured violations only where practical. If a full shell conversion would spread the task too far, leave shell as string-compatible through `_policy_violation_payload`.

- [ ] **Step 5: Run focused tests**

Run:

```bash
uv run --python 3.12 pytest tests/actor_runtime/test_codex_vault_runtime.py tests/artifacts/test_projection.py tests/supervisor/test_routing.py -q
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add src/rail/actor_runtime/codex_vault.py tests/actor_runtime/test_codex_vault_runtime.py tests/artifacts/test_projection.py tests/supervisor/test_routing.py
git commit -m "fix: write structured codex vault policy evidence"
```

---

### Task 6: Verification And Installed SDK Smoke

**Files:**
- Modify only if tests reveal necessary docs or package smoke updates.

- [ ] **Step 1: Run focused Actor Runtime tests**

Run:

```bash
uv run --python 3.12 pytest tests/actor_runtime/test_codex_bootstrap_profile.py tests/actor_runtime/test_vault_audit.py tests/actor_runtime/test_codex_vault_environment.py tests/actor_runtime/test_codex_vault_runtime.py -q
```

Expected: PASS.

- [ ] **Step 2: Run docs and projection tests**

Run:

```bash
uv run --python 3.12 pytest tests/docs tests/artifacts/test_projection.py tests/supervisor/test_routing.py -q
```

Expected: PASS.

- [ ] **Step 3: Run full checks**

Run:

```bash
uv run --python 3.12 pytest -q
uv run --python 3.12 ruff check src tests
uv run --python 3.12 mypy src/rail
```

Expected: pytest PASS, ruff PASS, mypy PASS.

- [ ] **Step 4: Run local installed SDK smoke only after full checks pass**

Install the local package into the uv tool environment used by the Rail skill:

```bash
uv tool install --python 3.13 --force .
```

Then verify the installed package imports and uses the new audit code:

```bash
/absolute/path/to/rail-sdk/bin/python3 - <<'PY'
import rail
from rail.actor_runtime.vault_audit import VaultAuditViolation
print(rail.__file__)
print(VaultAuditViolation(code="bootstrap_profile_mismatch", reason="test", audit_layer="bootstrap").model_dump(mode="json"))
PY
```

Expected: import succeeds and prints a structured violation dict. Replace `/absolute/path/to/rail-sdk/bin/python3` with the local uv tool path; do not commit machine-specific paths to docs.

- [ ] **Step 5: Final git status**

Run:

```bash
git status --short --branch
```

Expected: clean working tree after all task commits.

---

## Implementation Notes

- Keep `VaultAuditViolation.reason` as the user-facing terminal string. Do not expose local absolute credential paths.
- Keep `path_ref` artifact-relative where possible. If a path cannot be made artifact-relative, omit it instead of writing host paths.
- Do not let passive events through if they include a tool call, external server, command execution, or mutable side effect.
- Do not weaken shell policy. The existing read-only shell allowlist and sandbox checks remain separate from this plan.
- Avoid broad substring checks for `skill`, `plugin`, `mcp`, `hook`, `rule`, or `config`. They caused the false positives this plan is meant to remove.
- Keep the Rail skill copies unchanged unless implementation reveals a user-facing blocked-message contract change.
