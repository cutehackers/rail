# Actor Live Smoke Repair Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a v1 `codex_vault` actor-isolated live smoke runner for `planner` and `context_builder` that produces hard-fail diagnostics and repair proposals without auto-applying changes.

**Architecture:** Add a focused `rail.live_smoke` package that owns fixture preparation, actor invocation, smoke contracts, failure classification, and report persistence. Keep provider execution inside the existing `CodexVaultActorRuntime`; the new runner orchestrates one actor at a time and never bypasses existing policy audit. Expose the runner through both pytest and a thin `rail smoke ...` CLI wrapper.

**Tech Stack:** Python 3.12, Pydantic v2 contracts, existing Rail artifact/runtime APIs, pytest, argparse CLI.

---

## File Structure

- Create `src/rail/live_smoke/__init__.py`: export the public live smoke runner types.
- Create `src/rail/live_smoke/models.py`: Pydantic contracts for actor names, symptom classes, owning surfaces, smoke verdicts, and report payloads.
- Create `src/rail/live_smoke/fixtures.py`: package-asset fixture location, immutable copy, and fixture digest checks.
- Create `src/rail/live_smoke/contracts.py`: v1 actor list and behavior smoke checks for `planner` and `context_builder`.
- Create `src/rail/live_smoke/classification.py`: map runtime results and exceptions to symptom classes, owning surfaces, and repair proposals.
- Create `src/rail/live_smoke/runner.py`: shared orchestration for one actor or all v1 actors.
- Create `src/rail/package_assets/live_smoke/fixture_target/README.md`: target fixture overview.
- Create `src/rail/package_assets/live_smoke/fixture_target/docs/ARCHITECTURE.md`: small architecture fixture.
- Create `src/rail/package_assets/live_smoke/fixture_target/docs/CONVENTIONS.md`: small conventions fixture.
- Create `src/rail/package_assets/live_smoke/fixture_target/app/service.py`: simple source file for context collection.
- Create `src/rail/package_assets/live_smoke/fixture_target/tests/test_service.py`: simple test file for context collection.
- Modify `pyproject.toml`: include live smoke fixture assets in package data.
- Modify `src/rail/cli/main.py`: add `rail smoke actor <name> --live` and `rail smoke actors --live`.
- Modify `tests/e2e/test_optional_codex_vault_smoke.py`: replace direct planner-only live smoke with shared runner tests for v1 actors.
- Create `tests/live_smoke/test_fixtures.py`: unit tests for fixture copy, digest, and report exclusion.
- Create `tests/live_smoke/test_contracts.py`: unit tests for v1 actor list and behavior smoke checks.
- Create `tests/live_smoke/test_classification.py`: unit tests for failure classification and repair proposal boundaries.
- Create `tests/live_smoke/test_runner.py`: unit tests for runner orchestration using fake runtime.
- Create `tests/cli/test_live_smoke_commands.py`: CLI parser and command behavior tests with fake runner.
- Modify `scripts/check_python_package_assets.py`: assert package fixture assets are present in built wheel.

---

### Task 1: Live Smoke Data Contracts

**Files:**
- Create: `src/rail/live_smoke/__init__.py`
- Create: `src/rail/live_smoke/models.py`
- Test: `tests/live_smoke/test_contracts.py`

- [ ] **Step 1: Write failing tests for model constraints**

Add `tests/live_smoke/test_contracts.py`:

```python
from __future__ import annotations

from pathlib import Path

import pytest
from pydantic import ValidationError

from rail.live_smoke.models import (
    LiveSmokeActor,
    LiveSmokeReport,
    LiveSmokeVerdict,
    OwningSurface,
    RepairProposal,
    SymptomClass,
)


def test_live_smoke_report_rejects_unknown_fields(tmp_path: Path) -> None:
    with pytest.raises(ValidationError):
        LiveSmokeReport(
            actor=LiveSmokeActor.PLANNER,
            verdict=LiveSmokeVerdict.PASSED,
            symptom_class=None,
            owning_surface=None,
            report_dir=tmp_path,
            fixture_digest="sha256:abc",
            evidence_refs=[],
            repair_proposal=None,
            unexpected=True,
        )


def test_repair_proposal_records_safe_rail_owned_surface() -> None:
    proposal = RepairProposal(
        owning_surface=OwningSurface.ACTOR_PROMPT,
        file_paths=[".harness/actors/context_builder.md"],
        summary="Forbid grep fallback in context collection.",
        preserves_fail_closed_policy=True,
    )

    assert proposal.owning_surface == OwningSurface.ACTOR_PROMPT
    assert proposal.preserves_fail_closed_policy is True


def test_symptom_classes_include_non_actor_environment_failures() -> None:
    assert SymptomClass.READINESS_FAILURE.value == "readiness_failure"
    assert SymptomClass.PROVIDER_TRANSIENT_FAILURE.value == "provider_transient_failure"
    assert SymptomClass.EVIDENCE_WRITER_FAILURE.value == "evidence_writer_failure"
```

- [ ] **Step 2: Run tests and verify they fail**

Run: `uv run --python 3.12 pytest tests/live_smoke/test_contracts.py -q`

Expected: FAIL with `ModuleNotFoundError: No module named 'rail.live_smoke'`.

- [ ] **Step 3: Implement the contracts**

Create `src/rail/live_smoke/models.py`:

```python
from __future__ import annotations

from enum import StrEnum
from pathlib import Path

from pydantic import BaseModel, ConfigDict


class LiveSmokeActor(StrEnum):
    PLANNER = "planner"
    CONTEXT_BUILDER = "context_builder"


class LiveSmokeVerdict(StrEnum):
    PASSED = "passed"
    FAILED = "failed"


class SymptomClass(StrEnum):
    READINESS_FAILURE = "readiness_failure"
    PROVIDER_TRANSIENT_FAILURE = "provider_transient_failure"
    POLICY_VIOLATION = "policy_violation"
    SCHEMA_MISMATCH = "schema_mismatch"
    FIXTURE_DIGEST_MISMATCH = "fixture_digest_mismatch"
    FIXTURE_PREP_FAILURE = "fixture_prep_failure"
    EVIDENCE_WRITER_FAILURE = "evidence_writer_failure"
    BEHAVIOR_SMOKE_FAILURE = "behavior_smoke_failure"
    UNKNOWN_FAILURE = "unknown_failure"


class OwningSurface(StrEnum):
    ACTOR_PROMPT = "actor_prompt"
    RUNTIME_INVOCATION = "runtime_invocation"
    RUNTIME_CONTRACT = "runtime_contract"
    PACKAGED_ASSET = "packaged_asset"
    FIXTURE = "fixture"
    PROVIDER = "provider"
    OPERATOR_ENVIRONMENT = "operator_environment"
    UNKNOWN = "unknown"


class RepairProposal(BaseModel):
    model_config = ConfigDict(extra="forbid")

    owning_surface: OwningSurface
    file_paths: list[str]
    summary: str
    preserves_fail_closed_policy: bool


class LiveSmokeReport(BaseModel):
    model_config = ConfigDict(extra="forbid")

    actor: LiveSmokeActor
    verdict: LiveSmokeVerdict
    symptom_class: SymptomClass | None
    owning_surface: OwningSurface | None
    report_dir: Path
    fixture_digest: str
    evidence_refs: list[str]
    repair_proposal: RepairProposal | None
```

Create `src/rail/live_smoke/__init__.py`:

```python
from rail.live_smoke.models import (
    LiveSmokeActor,
    LiveSmokeReport,
    LiveSmokeVerdict,
    OwningSurface,
    RepairProposal,
    SymptomClass,
)

__all__ = [
    "LiveSmokeActor",
    "LiveSmokeReport",
    "LiveSmokeVerdict",
    "OwningSurface",
    "RepairProposal",
    "SymptomClass",
]
```

- [ ] **Step 4: Run tests and verify they pass**

Run: `uv run --python 3.12 pytest tests/live_smoke/test_contracts.py -q`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add src/rail/live_smoke/__init__.py src/rail/live_smoke/models.py tests/live_smoke/test_contracts.py
git commit -m "feat(live-smoke): add actor smoke report contracts"
```

---

### Task 2: Fixture Target And Immutable Copy

**Files:**
- Create: `src/rail/package_assets/live_smoke/fixture_target/README.md`
- Create: `src/rail/package_assets/live_smoke/fixture_target/docs/ARCHITECTURE.md`
- Create: `src/rail/package_assets/live_smoke/fixture_target/docs/CONVENTIONS.md`
- Create: `src/rail/package_assets/live_smoke/fixture_target/app/service.py`
- Create: `src/rail/package_assets/live_smoke/fixture_target/tests/test_service.py`
- Create: `src/rail/live_smoke/fixtures.py`
- Modify: `pyproject.toml`
- Test: `tests/live_smoke/test_fixtures.py`

- [ ] **Step 1: Write failing fixture tests**

Create `tests/live_smoke/test_fixtures.py`:

```python
from __future__ import annotations

from pathlib import Path

from rail.live_smoke.fixtures import copy_fixture_target, live_smoke_fixture_source


def test_live_smoke_fixture_source_contains_expected_files() -> None:
    source = live_smoke_fixture_source()

    assert (source / "README.md").is_file()
    assert (source / "docs" / "ARCHITECTURE.md").is_file()
    assert (source / "docs" / "CONVENTIONS.md").is_file()
    assert (source / "app" / "service.py").is_file()
    assert (source / "tests" / "test_service.py").is_file()


def test_copy_fixture_target_records_digest_and_excludes_smoke_reports(tmp_path: Path) -> None:
    report_root = tmp_path / "smoke-reports"
    copied = copy_fixture_target(tmp_path / "target", report_root=report_root)

    assert copied.target_root.is_dir()
    assert copied.fixture_digest.startswith("sha256:")
    assert copied.target_root != live_smoke_fixture_source()
    assert not (copied.target_root / "smoke-reports").exists()
```

- [ ] **Step 2: Run tests and verify they fail**

Run: `uv run --python 3.12 pytest tests/live_smoke/test_fixtures.py -q`

Expected: FAIL with missing `rail.live_smoke.fixtures`.

- [ ] **Step 3: Add fixture files**

Create `src/rail/package_assets/live_smoke/fixture_target/README.md`:

```markdown
# Live Smoke Fixture Target

This target is intentionally small. It gives Rail live smoke actors a stable
repository shape with source, tests, architecture notes, and conventions.
```

Create `src/rail/package_assets/live_smoke/fixture_target/docs/ARCHITECTURE.md`:

```markdown
# Architecture

The fixture uses a small service module and a focused test file. Actors should
collect only files needed to understand this structure.
```

Create `src/rail/package_assets/live_smoke/fixture_target/docs/CONVENTIONS.md`:

```markdown
# Conventions

- Keep source files under `app/`.
- Keep tests under `tests/`.
- Prefer direct commands that operate on sandbox-relative paths.
```

Create `src/rail/package_assets/live_smoke/fixture_target/app/service.py`:

```python
from __future__ import annotations


def normalize_title(value: str) -> str:
    return " ".join(value.strip().split()).title()
```

Create `src/rail/package_assets/live_smoke/fixture_target/tests/test_service.py`:

```python
from app.service import normalize_title


def test_normalize_title() -> None:
    assert normalize_title(" rail   smoke ") == "Rail Smoke"
```

- [ ] **Step 4: Implement fixture copy**

Create `src/rail/live_smoke/fixtures.py`:

```python
from __future__ import annotations

import shutil
from dataclasses import dataclass
from importlib.resources import files
from pathlib import Path

from rail.artifacts.digests import digest_bytes
from rail.workspace.isolation import tree_digest


@dataclass(frozen=True)
class CopiedFixtureTarget:
    target_root: Path
    fixture_digest: str


def live_smoke_fixture_source() -> Path:
    return Path(str(files("rail.package_assets") / "live_smoke" / "fixture_target"))


def copy_fixture_target(target_root: Path, *, report_root: Path) -> CopiedFixtureTarget:
    source = live_smoke_fixture_source()
    if not source.is_dir():
        raise FileNotFoundError(f"live smoke fixture source is missing: {source}")
    if target_root.exists():
        shutil.rmtree(target_root)
    shutil.copytree(source, target_root, ignore=_ignore_smoke_outputs(report_root))
    return CopiedFixtureTarget(target_root=target_root, fixture_digest=tree_digest(target_root))


def _ignore_smoke_outputs(report_root: Path):
    report_name = report_root.name

    def ignore(_directory: str, names: list[str]) -> set[str]:
        return {name for name in names if name == report_name}

    return ignore
```

Remove `digest_bytes` if ruff reports it unused.

- [ ] **Step 5: Include fixture assets in package data**

Modify `pyproject.toml`:

```toml
[tool.setuptools.package-data]
rail = [
    "package_assets/**/*.md",
    "package_assets/**/*.yaml",
    "package_assets/live_smoke/**/*",
]
```

- [ ] **Step 6: Run tests and verify they pass**

Run: `uv run --python 3.12 pytest tests/live_smoke/test_fixtures.py -q`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add pyproject.toml src/rail/live_smoke/fixtures.py src/rail/package_assets/live_smoke tests/live_smoke/test_fixtures.py
git commit -m "feat(live-smoke): add immutable fixture target"
```

---

### Task 3: Behavior Contracts And Failure Classification

**Files:**
- Create: `src/rail/live_smoke/contracts.py`
- Create: `src/rail/live_smoke/classification.py`
- Test: `tests/live_smoke/test_contracts.py`
- Test: `tests/live_smoke/test_classification.py`

- [ ] **Step 1: Extend contract tests**

Append to `tests/live_smoke/test_contracts.py`:

```python
from rail.live_smoke.contracts import V1_LIVE_SMOKE_ACTORS, evaluate_behavior_smoke


def test_v1_live_smoke_actor_scope_is_planner_and_context_builder_only() -> None:
    assert V1_LIVE_SMOKE_ACTORS == (LiveSmokeActor.PLANNER, LiveSmokeActor.CONTEXT_BUILDER)


def test_planner_behavior_smoke_requires_minimum_fields() -> None:
    error = evaluate_behavior_smoke(
        LiveSmokeActor.PLANNER,
        {"summary": "Plan", "substeps": [], "risks": [], "acceptance_criteria_refined": []},
    )

    assert error is None


def test_context_builder_behavior_smoke_requires_non_empty_context() -> None:
    error = evaluate_behavior_smoke(
        LiveSmokeActor.CONTEXT_BUILDER,
        {
            "relevant_files": [{"path": "README.md", "why": "entry point"}],
            "repo_patterns": ["small service module"],
            "test_patterns": ["pytest unit test"],
            "forbidden_changes": ["do not edit auth"],
            "implementation_hints": ["keep changes scoped"],
        },
    )

    assert error is None


def test_context_builder_behavior_smoke_rejects_empty_relevant_files() -> None:
    error = evaluate_behavior_smoke(
        LiveSmokeActor.CONTEXT_BUILDER,
        {
            "relevant_files": [],
            "repo_patterns": ["pattern"],
            "test_patterns": ["test"],
            "forbidden_changes": ["forbidden"],
            "implementation_hints": ["hint"],
        },
    )

    assert error == "context_builder output must include non-empty relevant_files"
```

- [ ] **Step 2: Add classification tests**

Create `tests/live_smoke/test_classification.py`:

```python
from __future__ import annotations

from pathlib import Path

from rail.actor_runtime.runtime import ActorResult
from rail.live_smoke.classification import classify_actor_result
from rail.live_smoke.models import LiveSmokeActor, OwningSurface, SymptomClass


def test_classifies_grep_policy_violation_as_prompt_or_contract_gap() -> None:
    result = ActorResult(
        status="interrupted",
        structured_output={"error": "shell executable is not allowed: grep"},
        events_ref=Path("runs/attempt-0001/context_builder.events.jsonl"),
        runtime_evidence_ref=Path("runs/attempt-0001/context_builder.runtime_evidence.json"),
        blocked_category="policy",
    )

    classification = classify_actor_result(LiveSmokeActor.CONTEXT_BUILDER, result, behavior_error=None)

    assert classification.symptom_class == SymptomClass.POLICY_VIOLATION
    assert classification.owning_surface == OwningSurface.RUNTIME_CONTRACT
    assert classification.repair_proposal is not None
    assert classification.repair_proposal.preserves_fail_closed_policy is True


def test_classifies_behavior_error_without_repair_when_output_is_missing() -> None:
    result = ActorResult(
        status="succeeded",
        structured_output={"relevant_files": []},
        events_ref=Path("runs/attempt-0001/context_builder.events.jsonl"),
        runtime_evidence_ref=Path("runs/attempt-0001/context_builder.runtime_evidence.json"),
    )

    classification = classify_actor_result(
        LiveSmokeActor.CONTEXT_BUILDER,
        result,
        behavior_error="context_builder output must include non-empty relevant_files",
    )

    assert classification.symptom_class == SymptomClass.BEHAVIOR_SMOKE_FAILURE
    assert classification.owning_surface == OwningSurface.ACTOR_PROMPT
```

- [ ] **Step 3: Run tests and verify they fail**

Run: `uv run --python 3.12 pytest tests/live_smoke/test_contracts.py tests/live_smoke/test_classification.py -q`

Expected: FAIL with missing `contracts` and `classification` modules.

- [ ] **Step 4: Implement behavior contracts**

Create `src/rail/live_smoke/contracts.py`:

```python
from __future__ import annotations

from collections.abc import Mapping

from rail.live_smoke.models import LiveSmokeActor

V1_LIVE_SMOKE_ACTORS = (LiveSmokeActor.PLANNER, LiveSmokeActor.CONTEXT_BUILDER)


def evaluate_behavior_smoke(actor: LiveSmokeActor, output: Mapping[str, object]) -> str | None:
    if actor == LiveSmokeActor.PLANNER:
        return _require_keys(
            actor,
            output,
            ("summary", "substeps", "risks", "acceptance_criteria_refined"),
        )
    if actor == LiveSmokeActor.CONTEXT_BUILDER:
        missing = _require_keys(
            actor,
            output,
            ("relevant_files", "repo_patterns", "test_patterns", "forbidden_changes", "implementation_hints"),
        )
        if missing is not None:
            return missing
        for key in ("relevant_files", "repo_patterns", "forbidden_changes", "implementation_hints"):
            value = output.get(key)
            if not isinstance(value, list) or not value:
                return f"{actor.value} output must include non-empty {key}"
        return None
    return f"unsupported live smoke actor: {actor.value}"


def _require_keys(actor: LiveSmokeActor, output: Mapping[str, object], keys: tuple[str, ...]) -> str | None:
    for key in keys:
        if key not in output:
            return f"{actor.value} output is missing {key}"
    return None
```

- [ ] **Step 5: Implement classification**

Create `src/rail/live_smoke/classification.py`:

```python
from __future__ import annotations

from pydantic import BaseModel, ConfigDict

from rail.actor_runtime.runtime import ActorResult
from rail.live_smoke.models import LiveSmokeActor, OwningSurface, RepairProposal, SymptomClass


class LiveSmokeClassification(BaseModel):
    model_config = ConfigDict(extra="forbid")

    symptom_class: SymptomClass | None
    owning_surface: OwningSurface | None
    repair_proposal: RepairProposal | None


def classify_actor_result(
    actor: LiveSmokeActor,
    result: ActorResult,
    *,
    behavior_error: str | None,
) -> LiveSmokeClassification:
    error_text = str(result.structured_output.get("error", ""))
    if result.blocked_category == "policy":
        return _classify_policy_violation(actor, error_text)
    if result.blocked_category == "environment":
        return LiveSmokeClassification(
            symptom_class=SymptomClass.READINESS_FAILURE,
            owning_surface=OwningSurface.OPERATOR_ENVIRONMENT,
            repair_proposal=None,
        )
    if result.blocked_category == "runtime":
        return LiveSmokeClassification(
            symptom_class=SymptomClass.SCHEMA_MISMATCH if "validation" in error_text else SymptomClass.UNKNOWN_FAILURE,
            owning_surface=OwningSurface.ACTOR_PROMPT,
            repair_proposal=None,
        )
    if behavior_error is not None:
        return LiveSmokeClassification(
            symptom_class=SymptomClass.BEHAVIOR_SMOKE_FAILURE,
            owning_surface=OwningSurface.ACTOR_PROMPT,
            repair_proposal=None,
        )
    if result.status == "succeeded":
        return LiveSmokeClassification(symptom_class=None, owning_surface=None, repair_proposal=None)
    return LiveSmokeClassification(
        symptom_class=SymptomClass.UNKNOWN_FAILURE,
        owning_surface=OwningSurface.UNKNOWN,
        repair_proposal=None,
    )


def _classify_policy_violation(actor: LiveSmokeActor, error_text: str) -> LiveSmokeClassification:
    if "shell executable is not allowed" in error_text:
        return LiveSmokeClassification(
            symptom_class=SymptomClass.POLICY_VIOLATION,
            owning_surface=OwningSurface.RUNTIME_CONTRACT,
            repair_proposal=RepairProposal(
                owning_surface=OwningSurface.RUNTIME_CONTRACT,
                file_paths=["src/rail/actor_runtime/runtime.py", ".harness/actors/context_builder.md"],
                summary=f"Constrain {actor.value} command guidance to policy-allowed executables.",
                preserves_fail_closed_policy=True,
            ),
        )
    return LiveSmokeClassification(
        symptom_class=SymptomClass.POLICY_VIOLATION,
        owning_surface=OwningSurface.UNKNOWN,
        repair_proposal=None,
    )
```

- [ ] **Step 6: Run tests and verify they pass**

Run: `uv run --python 3.12 pytest tests/live_smoke/test_contracts.py tests/live_smoke/test_classification.py -q`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add src/rail/live_smoke/contracts.py src/rail/live_smoke/classification.py tests/live_smoke/test_contracts.py tests/live_smoke/test_classification.py
git commit -m "feat(live-smoke): classify actor smoke failures"
```

---

### Task 4: Shared Runner

**Files:**
- Create: `src/rail/live_smoke/runner.py`
- Test: `tests/live_smoke/test_runner.py`

- [ ] **Step 1: Write runner tests with a fake runtime**

Create `tests/live_smoke/test_runner.py`:

```python
from __future__ import annotations

from pathlib import Path

from rail.actor_runtime.runtime import ActorResult
from rail.live_smoke.models import LiveSmokeActor, LiveSmokeVerdict, SymptomClass
from rail.live_smoke.runner import LiveSmokeRunner


class FakeRuntime:
    def __init__(self, result: ActorResult) -> None:
        self.result = result
        self.invocation_actors: list[str] = []

    def run(self, invocation):
        self.invocation_actors.append(invocation.actor)
        return self.result


def test_runner_passes_planner_smoke(tmp_path: Path) -> None:
    runtime = FakeRuntime(
        ActorResult(
            status="succeeded",
            structured_output={"summary": "Plan", "substeps": [], "risks": [], "acceptance_criteria_refined": []},
            events_ref=Path("runs/attempt-0001/planner.events.jsonl"),
            runtime_evidence_ref=Path("runs/attempt-0001/planner.runtime_evidence.json"),
        )
    )
    runner = LiveSmokeRunner(report_root=tmp_path / "reports", runtime_factory=lambda _target: runtime)

    report = runner.run_actor(LiveSmokeActor.PLANNER)

    assert report.verdict == LiveSmokeVerdict.PASSED
    assert report.symptom_class is None
    assert runtime.invocation_actors == ["planner"]


def test_runner_reports_context_builder_policy_failure(tmp_path: Path) -> None:
    runtime = FakeRuntime(
        ActorResult(
            status="interrupted",
            structured_output={"error": "shell executable is not allowed: grep"},
            events_ref=Path("runs/attempt-0001/context_builder.events.jsonl"),
            runtime_evidence_ref=Path("runs/attempt-0001/context_builder.runtime_evidence.json"),
            blocked_category="policy",
        )
    )
    runner = LiveSmokeRunner(report_root=tmp_path / "reports", runtime_factory=lambda _target: runtime)

    report = runner.run_actor(LiveSmokeActor.CONTEXT_BUILDER)

    assert report.verdict == LiveSmokeVerdict.FAILED
    assert report.symptom_class == SymptomClass.POLICY_VIOLATION
    assert report.repair_proposal is not None
```

- [ ] **Step 2: Run tests and verify they fail**

Run: `uv run --python 3.12 pytest tests/live_smoke/test_runner.py -q`

Expected: FAIL with missing `rail.live_smoke.runner`.

- [ ] **Step 3: Implement the runner**

Create `src/rail/live_smoke/runner.py`:

```python
from __future__ import annotations

import json
from collections.abc import Callable
from pathlib import Path

import rail
from rail.artifacts.run_attempts import allocate_run_attempt
from rail.actor_runtime.codex_vault import CodexVaultActorRuntime
from rail.actor_runtime.runtime import ActorRuntime, build_invocation
from rail.live_smoke.classification import classify_actor_result
from rail.live_smoke.contracts import V1_LIVE_SMOKE_ACTORS, evaluate_behavior_smoke
from rail.live_smoke.fixtures import copy_fixture_target
from rail.live_smoke.models import LiveSmokeActor, LiveSmokeReport, LiveSmokeVerdict
from rail.policy import load_effective_policy

RuntimeFactory = Callable[[Path], ActorRuntime]


class LiveSmokeRunner:
    def __init__(
        self,
        *,
        report_root: Path,
        runtime_factory: RuntimeFactory | None = None,
    ) -> None:
        self.report_root = report_root
        self.runtime_factory = runtime_factory or self._default_runtime_factory

    def run_all(self) -> list[LiveSmokeReport]:
        return [self.run_actor(actor) for actor in V1_LIVE_SMOKE_ACTORS]

    def run_actor(self, actor: LiveSmokeActor) -> LiveSmokeReport:
        actor_report_root = self.report_root / actor.value
        fixture = copy_fixture_target(actor_report_root / "target", report_root=self.report_root)
        handle = rail.start_task(
            {
                "project_root": str(fixture.target_root),
                "task_type": "bug_fix",
                "goal": f"Run {actor.value} live smoke without mutating target.",
                "definition_of_done": [f"{actor.value} returns smoke-valid structured output."],
            }
        )
        runtime = self.runtime_factory(fixture.target_root)
        prior_outputs = _prior_outputs_for(actor)
        invocation = build_invocation(
            handle,
            actor.value,
            attempt_ref=allocate_run_attempt(handle.artifact_dir),
            prior_outputs=prior_outputs,
        )
        result = runtime.run(invocation)
        behavior_error = None
        if result.status == "succeeded":
            behavior_error = evaluate_behavior_smoke(actor, result.structured_output)
        classification = classify_actor_result(actor, result, behavior_error=behavior_error)
        verdict = LiveSmokeVerdict.PASSED if result.status == "succeeded" and behavior_error is None else LiveSmokeVerdict.FAILED
        report = LiveSmokeReport(
            actor=actor,
            verdict=verdict,
            symptom_class=classification.symptom_class,
            owning_surface=classification.owning_surface,
            report_dir=actor_report_root,
            fixture_digest=fixture.fixture_digest,
            evidence_refs=[result.events_ref.as_posix(), result.runtime_evidence_ref.as_posix()],
            repair_proposal=classification.repair_proposal,
        )
        actor_report_root.mkdir(parents=True, exist_ok=True)
        (actor_report_root / "live_smoke_report.json").write_text(
            report.model_dump_json(indent=2),
            encoding="utf-8",
        )
        return report

    @staticmethod
    def _default_runtime_factory(target_root: Path) -> ActorRuntime:
        return CodexVaultActorRuntime(project_root=Path("."), policy=load_effective_policy(target_root))


def _prior_outputs_for(actor: LiveSmokeActor) -> dict[str, dict[str, object]]:
    if actor != LiveSmokeActor.CONTEXT_BUILDER:
        return {}
    return {
        "planner": {
            "summary": "Inspect the fixture target and report the implementation context.",
            "likely_files": ["README.md", "docs/ARCHITECTURE.md", "docs/CONVENTIONS.md", "app/service.py"],
            "substeps": ["Read fixture docs", "Inspect app service", "Summarize test patterns"],
            "risks": ["Do not read outside the sandbox"],
            "acceptance_criteria_refined": ["Context pack references fixture files only"],
        }
    }
```

Remove the unused `json` import if ruff reports it.

- [ ] **Step 4: Run tests and verify they pass**

Run: `uv run --python 3.12 pytest tests/live_smoke/test_runner.py -q`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add src/rail/live_smoke/runner.py tests/live_smoke/test_runner.py
git commit -m "feat(live-smoke): add shared actor smoke runner"
```

---

### Task 5: Optional Live Pytest Integration

**Files:**
- Modify: `tests/e2e/test_optional_codex_vault_smoke.py`

- [ ] **Step 1: Replace direct planner-only test with shared runner test**

Modify `tests/e2e/test_optional_codex_vault_smoke.py` to:

```python
from __future__ import annotations

import os

import pytest

from rail.live_smoke.contracts import V1_LIVE_SMOKE_ACTORS
from rail.live_smoke.models import LiveSmokeActor, LiveSmokeVerdict
from rail.live_smoke.runner import LiveSmokeRunner

pytestmark = pytest.mark.skipif(
    os.environ.get("RAIL_CODEX_VAULT_LIVE_SMOKE") != "1",
    reason="codex_vault live smoke is opt-in",
)


@pytest.mark.parametrize("actor", V1_LIVE_SMOKE_ACTORS)
def test_optional_codex_vault_live_smoke_runs_v1_actor_without_openai_api_key(tmp_path, monkeypatch, actor: LiveSmokeActor):
    monkeypatch.delenv("OPENAI_API_KEY", raising=False)
    runner = LiveSmokeRunner(report_root=tmp_path / "reports")

    report = runner.run_actor(actor)

    assert report.verdict == LiveSmokeVerdict.PASSED, (
        f"actor={report.actor.value} symptom={report.symptom_class} "
        f"surface={report.owning_surface} report={report.report_dir}"
    )
```

- [ ] **Step 2: Run the non-live collection path**

Run: `uv run --python 3.12 pytest tests/e2e/test_optional_codex_vault_smoke.py -q`

Expected: SKIPPED when `RAIL_CODEX_VAULT_LIVE_SMOKE` is not `1`.

- [ ] **Step 3: Run focused non-live unit tests**

Run: `uv run --python 3.12 pytest tests/live_smoke -q`

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add tests/e2e/test_optional_codex_vault_smoke.py
git commit -m "test(live-smoke): route optional codex vault smoke through runner"
```

---

### Task 6: CLI Smoke Commands

**Files:**
- Modify: `src/rail/cli/main.py`
- Create: `tests/cli/test_live_smoke_commands.py`

- [ ] **Step 1: Write CLI tests**

Create `tests/cli/test_live_smoke_commands.py`:

```python
from __future__ import annotations

from rail.cli import main as cli_main


def test_smoke_actor_requires_live_flag(capsys) -> None:
    result = cli_main.main(["smoke", "actor", "planner"])

    assert result == 1
    assert "--live is required" in capsys.readouterr().out


def test_smoke_rejects_unknown_actor(capsys) -> None:
    result = cli_main.main(["smoke", "actor", "executor", "--live"])

    assert result == 1
    assert "unsupported v1 live smoke actor" in capsys.readouterr().out
```

- [ ] **Step 2: Run tests and verify they fail**

Run: `uv run --python 3.12 pytest tests/cli/test_live_smoke_commands.py -q`

Expected: FAIL because `smoke` command does not exist.

- [ ] **Step 3: Add CLI parser and command handling**

Modify `src/rail/cli/main.py`:

```python
from rail.live_smoke.contracts import V1_LIVE_SMOKE_ACTORS
from rail.live_smoke.models import LiveSmokeActor, LiveSmokeVerdict
from rail.live_smoke.runner import LiveSmokeRunner
```

Add command handling in `main()` before `parser.print_help()`:

```python
    if args.command == "smoke":
        if not args.live:
            print("--live is required for actor live smoke")
            return 1
        report_root = Path(args.report_root).expanduser()
        runner = LiveSmokeRunner(report_root=report_root)
        if args.smoke_command == "actor":
            try:
                actor = LiveSmokeActor(args.actor)
            except ValueError:
                print(f"unsupported v1 live smoke actor: {args.actor}")
                return 1
            report = runner.run_actor(actor)
            print(report.model_dump_json(indent=2))
            return 0 if report.verdict == LiveSmokeVerdict.PASSED else 1
        if args.smoke_command == "actors":
            reports = runner.run_all()
            for report in reports:
                print(report.model_dump_json(indent=2))
            return 0 if all(report.verdict == LiveSmokeVerdict.PASSED for report in reports) else 1
```

Add parser definitions in `_parser()`:

```python
    smoke = subparsers.add_parser("smoke", help="run explicit Rail live smoke diagnostics")
    smoke.add_argument("--live", action="store_true", help="required for live actor smoke execution")
    smoke.add_argument(
        "--report-root",
        default=".harness/live-smoke",
        help="directory for smoke diagnostic reports",
    )
    smoke_subparsers = smoke.add_subparsers(dest="smoke_command")

    smoke_actor = smoke_subparsers.add_parser("actor", help="run one v1 actor live smoke")
    smoke_actor.add_argument("actor", help="v1 actor name")

    smoke_subparsers.add_parser("actors", help="run all v1 actor live smokes")
```

- [ ] **Step 4: Run CLI tests and verify they pass**

Run: `uv run --python 3.12 pytest tests/cli/test_live_smoke_commands.py -q`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add src/rail/cli/main.py tests/cli/test_live_smoke_commands.py
git commit -m "feat(cli): add actor live smoke commands"
```

---

### Task 7: Package Asset Gate

**Files:**
- Modify: `scripts/check_python_package_assets.py`
- Test: existing package asset check through release gate or focused script invocation

- [ ] **Step 1: Inspect the existing script**

Read `scripts/check_python_package_assets.py` and locate the current required package asset assertions.

- [ ] **Step 2: Add live smoke fixture assertions**

Add checks equivalent to:

```python
required_assets.extend(
    [
        "rail/package_assets/live_smoke/fixture_target/README.md",
        "rail/package_assets/live_smoke/fixture_target/docs/ARCHITECTURE.md",
        "rail/package_assets/live_smoke/fixture_target/docs/CONVENTIONS.md",
        "rail/package_assets/live_smoke/fixture_target/app/service.py",
        "rail/package_assets/live_smoke/fixture_target/tests/test_service.py",
    ]
)
```

Adjust the exact variable name to match the script.

- [ ] **Step 3: Run focused package asset check**

Run: `uv run --python 3.12 python scripts/check_python_package_assets.py`

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add scripts/check_python_package_assets.py
git commit -m "test(package): require live smoke fixture assets"
```

---

### Task 8: Final Verification

**Files:**
- All files touched by Tasks 1-7

- [ ] **Step 1: Run focused tests**

Run:

```bash
uv run --python 3.12 pytest tests/live_smoke tests/cli/test_live_smoke_commands.py tests/e2e/test_optional_codex_vault_smoke.py -q
```

Expected: PASS with the optional live e2e tests skipped unless `RAIL_CODEX_VAULT_LIVE_SMOKE=1`.

- [ ] **Step 2: Run full Python test suite**

Run: `uv run --python 3.12 pytest -q`

Expected: PASS.

- [ ] **Step 3: Run lint**

Run: `uv run --python 3.12 ruff check src tests`

Expected: PASS.

- [ ] **Step 4: Run typing**

Run: `uv run --python 3.12 mypy src/rail`

Expected: PASS.

- [ ] **Step 5: Run release gate without live flag**

Run: `scripts/release_gate.sh`

Expected: PASS with optional live smoke skipped.

- [ ] **Step 6: Optional operator live check**

Run only in an environment with Rail-owned Codex auth ready:

```bash
RAIL_CODEX_VAULT_LIVE_SMOKE=1 uv run --python 3.12 pytest tests/e2e/test_optional_codex_vault_smoke.py -q
```

Expected: PASS for `planner` and `context_builder`, or hard failure with a report path and repair proposal.

- [ ] **Step 7: Commit final integration fixes if any**

```bash
git status --short
git add <only-intended-files>
git commit -m "fix(live-smoke): stabilize actor smoke integration"
```

Skip this commit when there are no final integration fixes.

---

## Self-Review

Spec coverage:

- V1 provider scope is covered by Tasks 4-6.
- `planner` and `context_builder` actor coverage is covered by Tasks 3-5.
- Real Codex execution remains in `CodexVaultActorRuntime` and is exercised by Task 5 when the live flag is enabled.
- Fixture immutability and digest reporting are covered by Task 2.
- Report-only classification and repair proposals are covered by Tasks 1, 3, and 4.
- CLI and pytest shared runner integration is covered by Tasks 4-6.
- Package asset verification is covered by Task 7.
- No automatic apply, branch, or commit behavior is implemented in v1.

No placeholder sections remain. Function and model names are consistent across tasks.
