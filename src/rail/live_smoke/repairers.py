from __future__ import annotations

from collections.abc import Callable
from pathlib import Path

from rail.live_smoke.models import LiveSmokeActor, OwningSurface, SymptomClass
from rail.live_smoke.repair_evidence import RepairEvidenceSummary
from rail.live_smoke.repair_models import RepairCandidate, RepairRiskLevel
from rail.workspace.isolation import tree_digest
from rail.workspace.patch_bundle import PatchBundle, PatchOperation

Repairer = Callable[[RepairEvidenceSummary, Path], RepairCandidate | None]

_TOOL_PROBE_GUIDANCE = (
    "- In live smoke, do not probe unavailable tools; if a needed executable is outside "
    "`live_smoke_runtime_contract.allowed_shell_executables`, report the actor-specific unavailable result without running it.\n"
)
_EVALUATOR_DIGEST_GUIDANCE = (
    "- In live smoke, echo `evaluator_input_digest` exactly as `evaluated_input_digest`; do not recompute or omit it.\n"
)
_IMPLEMENTATION_SCHEMA_PATHS = (
    ".harness/templates/implementation_result.schema.yaml",
    "assets/defaults/templates/implementation_result.schema.yaml",
    "src/rail/package_assets/defaults/templates/implementation_result.schema.yaml",
)


def repairer_registry() -> dict[tuple[SymptomClass, OwningSurface], Repairer]:
    return {
        (SymptomClass.POLICY_VIOLATION, OwningSurface.RUNTIME_CONTRACT): _repair_shell_policy_guidance,
        (SymptomClass.SCHEMA_MISMATCH, OwningSurface.ACTOR_PROMPT): _repair_schema_drift,
        (SymptomClass.BEHAVIOR_SMOKE_FAILURE, OwningSurface.ACTOR_PROMPT): _repair_behavior_contract_prompt,
    }


def build_repair_candidate(summary: RepairEvidenceSummary, *, repo_root: Path) -> RepairCandidate | None:
    repairer = repairer_registry().get((summary.symptom_class, summary.owning_surface))
    if repairer is None:
        return None
    return repairer(summary, repo_root)


def _repair_shell_policy_guidance(summary: RepairEvidenceSummary, repo_root: Path) -> RepairCandidate | None:
    if "shell executable is not allowed" not in (summary.policy_violation_reason or summary.error_text):
        return None

    file_contents: dict[str, str] = {}
    for prompt_path in _actor_prompt_paths(summary.actor):
        updated = _append_line_if_missing(repo_root / prompt_path, _TOOL_PROBE_GUIDANCE)
        if updated is None:
            return None
        file_contents[prompt_path] = updated
    return _candidate(
        summary,
        repo_root=repo_root,
        file_contents=file_contents,
        risk_level=RepairRiskLevel.LOW,
        summary_text=f"Guide {summary.actor.value} live smoke away from forbidden tool probes.",
        validation_commands=["uv run --python 3.12 pytest tests/build/test_package_assets.py -q"],
        auto_apply=True,
    )


def _repair_schema_drift(summary: RepairEvidenceSummary, repo_root: Path) -> RepairCandidate | None:
    if "patch_bundle.operations" not in summary.error_text or "valid boolean" not in summary.error_text:
        return None

    file_contents: dict[str, str] = {}
    for file_path in _IMPLEMENTATION_SCHEMA_PATHS:
        updated = _require_patch_operation_booleans(repo_root / file_path)
        if updated is None:
            return None
        file_contents[file_path] = updated
    return _candidate(
        summary,
        repo_root=repo_root,
        file_contents=file_contents,
        risk_level=RepairRiskLevel.MEDIUM,
        summary_text="Align implementation result patch operation schema with required boolean fields.",
        validation_commands=[
            "uv run --python 3.12 pytest tests/actor_runtime/test_codex_output_schema.py tests/build/test_package_assets.py -q"
        ],
    )


def _repair_behavior_contract_prompt(summary: RepairEvidenceSummary, repo_root: Path) -> RepairCandidate | None:
    if summary.actor != LiveSmokeActor.EVALUATOR or "evaluator_input_digest" not in summary.error_text:
        return None

    file_contents: dict[str, str] = {}
    for prompt_path in _actor_prompt_paths(summary.actor):
        updated = _append_line_if_missing(repo_root / prompt_path, _EVALUATOR_DIGEST_GUIDANCE)
        if updated is None:
            return None
        file_contents[prompt_path] = updated
    return _candidate(
        summary,
        repo_root=repo_root,
        file_contents=file_contents,
        risk_level=RepairRiskLevel.LOW,
        summary_text="Guide evaluator live smoke to echo the bound evaluator input digest.",
        validation_commands=[
            "uv run --python 3.12 pytest tests/live_smoke/test_contracts.py tests/build/test_package_assets.py -q"
        ],
        auto_apply=True,
    )


def _candidate(
    summary: RepairEvidenceSummary,
    *,
    repo_root: Path,
    file_contents: dict[str, str],
    risk_level: RepairRiskLevel,
    summary_text: str,
    validation_commands: list[str],
    auto_apply: bool = False,
) -> RepairCandidate:
    operations = [
        PatchOperation(
            path=file_path,
            content=content,
            binary=False,
            executable=False,
        )
        for file_path, content in sorted(file_contents.items())
    ]
    return RepairCandidate(
        actor=summary.actor,
        symptom_class=summary.symptom_class,
        owning_surface=summary.owning_surface,
        source_report_path=summary.report_path,
        evidence_refs=summary.evidence_refs,
        file_paths=[operation.path for operation in operations],
        summary=summary_text,
        risk_level=risk_level,
        patch_bundle=PatchBundle(base_tree_digest=tree_digest(repo_root), operations=operations),
        validation_commands=validation_commands,
        preserves_fail_closed_policy=True,
        auto_apply=auto_apply,
    )


def _actor_prompt_paths(actor: LiveSmokeActor) -> tuple[str, str, str]:
    return (
        f".harness/actors/{actor.value}.md",
        f"assets/defaults/actors/{actor.value}.md",
        f"src/rail/package_assets/defaults/actors/{actor.value}.md",
    )


def _append_line_if_missing(path: Path, line: str) -> str | None:
    if not path.is_file():
        return None
    text = path.read_text(encoding="utf-8")
    if line.strip() in text:
        return text
    return f"{text.rstrip()}\n{line}"


def _require_patch_operation_booleans(path: Path) -> str | None:
    if not path.is_file():
        return None
    text = path.read_text(encoding="utf-8")
    if "            - executable" in text and "            - binary" in text:
        return text
    needle = "            - content\n"
    if needle not in text:
        return None
    return text.replace(needle, f"{needle}            - executable\n            - binary\n", 1)
