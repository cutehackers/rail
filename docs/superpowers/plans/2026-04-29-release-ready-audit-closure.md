# Release-Ready Audit Closure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the remaining `docs/SPEC.md` release-ready gap by making the Python package distribution and release gate prove the installed Rail product boundary.

**Architecture:** Keep the Python API as the product authority and keep source-checkout files as development inputs only. Package the bundled Rail skill and Rail-owned default harness assets as installed resources, then make the release gate build and inspect the distribution artifact before release-ready can be claimed.

**Tech Stack:** Python 3.12, setuptools/uv build, importlib.resources, pytest, ruff, mypy.

---

## File Structure

- `pyproject.toml`: package configuration for bundled non-code resources.
- `src/rail/resources.py`: package-resource lookup for installed Rail defaults and bundled skill assets.
- `src/rail/policy/load.py`: load default actor runtime policy through package resources instead of repository-parent paths.
- `src/rail/actor_runtime/prompts.py`: load actor prompts and schemas from target `.harness` when present, with Rail packaged defaults as the fallback.
- `scripts/check_python_package_assets.py`: inspect built wheel and sdist for required Rail assets.
- `scripts/python_release_gate.sh`: run build and package asset checks as part of the canonical release gate.
- `tests/build/test_package_assets.py`: focused tests for package asset inspection behavior.
- `tests/policy/test_policy_v2.py`: installed-resource coverage for default policy loading.
- `tests/actor_runtime/test_actor_catalog.py`: packaged-default fallback coverage for actor prompts and schemas.
- `docs/tasks.md`: active checklist state for the release-ready audit gap.

## Task 1: Add Package Asset Inspection

**Files:**
- Create: `scripts/check_python_package_assets.py`
- Create: `tests/build/test_package_assets.py`
- Modify: `pyproject.toml`

- [ ] **Step 1: Write the failing package asset test**

Create `tests/build/test_package_assets.py` with a fixture-built fake wheel or
archive that proves the checker fails when `assets/skill/Rail/SKILL.md`,
`assets/skill/Rail/references/examples.md`, `assets/defaults/actors/planner.md`,
`assets/defaults/templates/plan.schema.yaml`, and
`assets/defaults/supervisor/actor_runtime.yaml` are absent.

- [ ] **Step 2: Run the focused test**

Run:

```bash
uv run --python 3.12 pytest tests/build/test_package_assets.py -q
```

Expected: fail until the checker exists.

- [ ] **Step 3: Implement `scripts/check_python_package_assets.py`**

The script should:

- accept `dist/` by default
- find the built `.whl` and `.tar.gz`
- inspect archive members without extracting
- require the bundled Rail skill files
- require the default actor, template, supervisor, rule, and rubric assets that runtime behavior needs
- print concise missing-asset findings
- exit non-zero on any missing required asset

- [ ] **Step 4: Configure package inclusion**

Update `pyproject.toml` so `uv build` includes the required assets in both
wheel and sdist. Prefer package resources that can be read through
`importlib.resources` after installation. Do not rely on a source checkout path.

- [ ] **Step 5: Verify package asset inspection**

Run:

```bash
rm -rf dist
uv build
uv run --python 3.12 python scripts/check_python_package_assets.py dist
```

Expected: build succeeds and the checker reports all required assets present.

## Task 2: Replace Source-Checkout Resource Assumptions

**Files:**
- Create: `src/rail/resources.py`
- Modify: `src/rail/policy/load.py`
- Modify: `src/rail/actor_runtime/prompts.py`
- Test: `tests/policy/test_policy_v2.py`
- Test: `tests/actor_runtime/test_actor_catalog.py`

- [ ] **Step 1: Write failing resource-loading tests**

Add tests that monkeypatch or isolate the current working directory so runtime
defaults cannot be found through repository-parent paths. Assert that:

- `load_effective_policy(project_root)` still loads the Rail default actor
  runtime policy
- `load_actor_catalog(project_root)` can fall back to packaged actor prompts
  and schema files when the target does not provide those files

- [ ] **Step 2: Run focused tests**

Run:

```bash
uv run --python 3.12 pytest tests/policy/test_policy_v2.py tests/actor_runtime/test_actor_catalog.py -q
```

Expected: fail on source-checkout assumptions.

- [ ] **Step 3: Add package resource helpers**

Create `src/rail/resources.py` with small functions for reading packaged text
and YAML resources. Keep the API concrete, for example:

- `read_default_asset_text(relative_path: str) -> str`
- `load_default_asset_yaml(relative_path: str) -> dict[str, object]`

- [ ] **Step 4: Update policy loading**

Change `src/rail/policy/load.py` so the base policy comes from packaged
`assets/defaults/supervisor/actor_runtime.yaml`. Target policy overlays should
still be read from the target repository and may only narrow the packaged base.

- [ ] **Step 5: Update actor catalog loading**

Change `src/rail/actor_runtime/prompts.py` so target `.harness/actors` and
`.harness/templates` remain the first choice when present, with packaged
defaults as fallback. The returned `prompt_path` and `schema_path` should remain
reviewable references; when using a packaged resource, use a stable logical
resource label rather than a machine-specific path.

- [ ] **Step 6: Verify focused behavior**

Run:

```bash
uv run --python 3.12 pytest tests/policy/test_policy_v2.py tests/actor_runtime/test_actor_catalog.py -q
```

Expected: pass.

## Task 3: Canonicalize The Release Gate

**Files:**
- Modify: `scripts/python_release_gate.sh`
- Modify: `tests/docs/test_removed_runtime_surfaces.py`
- Modify: `README.md`
- Modify: `docs/ARCHITECTURE.md`

- [ ] **Step 1: Extend docs guard expectations**

Update the docs guard test so the release gate must run:

- `uv build`
- `scripts/check_python_package_assets.py`
- the existing full Python tests, lint, typing checks, no-legacy guard, naming
  convention guard, deterministic SDK-adapter smoke, and optional live SDK smoke

- [ ] **Step 2: Run the docs guard**

Run:

```bash
uv run --python 3.12 pytest tests/docs/test_removed_runtime_surfaces.py -q
```

Expected: fail until the release gate is updated.

- [ ] **Step 3: Update the release gate**

Modify `scripts/python_release_gate.sh` to remove stale build artifacts, run
`uv build`, run `scripts/check_python_package_assets.py`, then run the existing
test/lint/type checks. Keep optional live SDK smoke skipped by default and
enabled only by `RAIL_ACTOR_RUNTIME_LIVE_SMOKE=1`.

- [ ] **Step 4: Update active docs**

Update `README.md` and `docs/ARCHITECTURE.md` so the release gate description
mentions package build and package asset inspection.

- [ ] **Step 5: Verify the full gate**

Run:

```bash
scripts/python_release_gate.sh
```

Expected: package build, package asset check, tests, lint, and typing all pass;
optional live SDK smoke remains skipped unless explicitly enabled.

## Task 4: Run SPEC-Based Critical Review

**Files:**
- Modify: `docs/tasks.md`
- Optional Modify: implementation or tests named by review findings

- [ ] **Step 1: Request independent review**

Use a subagent reviewer against the main worktree, not a removed worktree path.
Give it only:

- `docs/SPEC.md`
- `docs/tasks.md`
- `docs/ARCHITECTURE.md`
- `scripts/python_release_gate.sh`
- the relevant source and test paths changed by this plan

Ask whether every `docs/SPEC.md` release-ready criterion is enforced by code,
tests, release gate, or explicit operator-only evidence.

- [ ] **Step 2: Classify findings**

For every finding, classify it as:

- release blocker
- should-fix before tag
- documented residual risk
- incorrect finding with rationale

- [ ] **Step 3: Fix blockers**

Implement fixes for release blockers using focused tests first.

- [ ] **Step 4: Update `docs/tasks.md`**

Mark the package distribution, installed-resource loading, release gate, and
critical review tasks complete only after the fixes and review are done.

- [ ] **Step 5: Run final verification**

Run:

```bash
scripts/python_release_gate.sh
```

Expected: pass.
