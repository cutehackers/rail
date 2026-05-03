from __future__ import annotations

import json
import shlex
import shutil
import subprocess
from collections.abc import Callable
from pathlib import Path

from rail.artifacts.digests import digest_payload
from rail.actor_runtime.runtime import ActorRuntime
from rail.live_smoke.contracts import LIVE_SMOKE_ACTORS
from rail.live_smoke.models import LiveSmokeActor, LiveSmokeReport, LiveSmokeVerdict
from rail.live_smoke.repair_evidence import summarize_repair_evidence
from rail.live_smoke.repair_models import (
    LiveSmokeRepairLoopReport,
    RepairCandidate,
    RepairIterationReport,
    RepairLoopStatus,
    RepairRiskLevel,
)
from rail.live_smoke.repairers import build_repair_candidate
from rail.live_smoke.runner import LiveSmokeRunner
from rail.workspace.apply import apply_patch_bundle
from rail.workspace.isolation import tree_digest

RuntimeFactory = Callable[[Path], ActorRuntime]
ValidationRunner = Callable[[RepairCandidate, Path], bool]

_REPAIR_LOOP_REPORT_NAME = "repair_loop_report.json"


class LiveSmokeRepairLoop:
    def __init__(
        self,
        *,
        report_root: Path,
        repo_root: Path = Path("."),
        runtime_factory: RuntimeFactory | None = None,
        validation_runner: ValidationRunner | None = None,
        allow_dirty_worktree: bool = False,
    ) -> None:
        self.report_root = report_root
        self.repo_root = repo_root
        self.runtime_factory = runtime_factory
        self.validation_runner = validation_runner or _run_validation_commands
        self.allow_dirty_worktree = allow_dirty_worktree

    def run_all(self, *, apply: bool = False, max_iterations: int = 2) -> LiveSmokeRepairLoopReport:
        if max_iterations < 1:
            raise ValueError("max_iterations must be at least 1")
        if apply and not self.allow_dirty_worktree and _worktree_is_dirty(self.repo_root):
            report = LiveSmokeRepairLoopReport(
                status=RepairLoopStatus.FAILED_VALIDATION,
                actors=list(LIVE_SMOKE_ACTORS),
                report_dir=self.report_root,
                apply=apply,
                max_iterations=max_iterations,
                iterations=[],
            )
            _write_loop_report(report)
            return report

        actor_reports = [
            self._run_actor(
                actor,
                apply=apply,
                max_iterations=max_iterations,
                write_report=False,
                enforce_clean_worktree=False,
            )
            for actor in LIVE_SMOKE_ACTORS
        ]
        report = LiveSmokeRepairLoopReport(
            status=_combined_status([actor_report.status for actor_report in actor_reports]),
            actors=list(LIVE_SMOKE_ACTORS),
            report_dir=self.report_root,
            apply=apply,
            max_iterations=max_iterations,
            iterations=[iteration for actor_report in actor_reports for iteration in actor_report.iterations],
        )
        _write_loop_report(report)
        return report

    def run_actor(
        self,
        actor: LiveSmokeActor,
        *,
        apply: bool = False,
        max_iterations: int = 2,
        write_report: bool = True,
    ) -> LiveSmokeRepairLoopReport:
        return self._run_actor(
            actor,
            apply=apply,
            max_iterations=max_iterations,
            write_report=write_report,
            enforce_clean_worktree=True,
        )

    def _run_actor(
        self,
        actor: LiveSmokeActor,
        *,
        apply: bool,
        max_iterations: int,
        write_report: bool,
        enforce_clean_worktree: bool,
    ) -> LiveSmokeRepairLoopReport:
        if max_iterations < 1:
            raise ValueError("max_iterations must be at least 1")
        if apply and enforce_clean_worktree and not self.allow_dirty_worktree and _worktree_is_dirty(self.repo_root):
            report = self._failed_validation_report(actor, apply=apply, max_iterations=max_iterations, iterations=[])
            if write_report:
                _write_loop_report(report)
            return report

        iterations: list[RepairIterationReport] = []
        applied_any = False
        for iteration_number in range(1, max_iterations + 1):
            smoke_report = self._runner().run_actor(actor)
            report_path = _snapshot_smoke_report(
                smoke_report.report_dir / "live_smoke_report.json",
                report_root=self.report_root,
                actor=actor,
                iteration=iteration_number,
            )
            if smoke_report.verdict == LiveSmokeVerdict.PASSED:
                report = LiveSmokeRepairLoopReport(
                    status=RepairLoopStatus.REPAIRED if applied_any else RepairLoopStatus.PASSED,
                    actors=[actor],
                    report_dir=self.report_root,
                    apply=apply,
                    max_iterations=max_iterations,
                    iterations=[
                        *iterations,
                        RepairIterationReport(
                            iteration=iteration_number,
                            actor=actor,
                            report_path=report_path,
                            candidate=None,
                            applied_patch_digest=None,
                            pre_apply_tree_digest=None,
                            post_apply_tree_digest=None,
                        ),
                    ],
                )
                if write_report:
                    _write_loop_report(report)
                return report

            candidate = _candidate_for(smoke_report, report_path=report_path, repo_root=self.repo_root)
            if candidate is None:
                report = LiveSmokeRepairLoopReport(
                    status=RepairLoopStatus.UNREPAIRABLE,
                    actors=[actor],
                    report_dir=self.report_root,
                    apply=apply,
                    max_iterations=max_iterations,
                    iterations=[
                        *iterations,
                        RepairIterationReport(
                            iteration=iteration_number,
                            actor=actor,
                            report_path=report_path,
                            candidate=None,
                            applied_patch_digest=None,
                            pre_apply_tree_digest=None,
                            post_apply_tree_digest=None,
                        ),
                    ],
                )
                if write_report:
                    _write_loop_report(report)
                return report

            if not apply:
                report = LiveSmokeRepairLoopReport(
                    status=RepairLoopStatus.CANDIDATE_READY,
                    actors=[actor],
                    report_dir=self.report_root,
                    apply=apply,
                    max_iterations=max_iterations,
                    iterations=[
                        *iterations,
                        RepairIterationReport(
                            iteration=iteration_number,
                            actor=actor,
                            report_path=report_path,
                            candidate=candidate,
                            applied_patch_digest=None,
                            pre_apply_tree_digest=None,
                            post_apply_tree_digest=None,
                        ),
                    ],
                )
                if write_report:
                    _write_loop_report(report)
                return report

            if candidate.risk_level == RepairRiskLevel.HIGH:
                report = self._failed_validation_report(
                    actor,
                    apply=apply,
                    max_iterations=max_iterations,
                    iterations=iterations,
                )
                if write_report:
                    _write_loop_report(report)
                return report

            pre_apply_tree_digest = tree_digest(self.repo_root)
            apply_patch_bundle(candidate.patch_bundle, self.repo_root)
            post_apply_tree_digest = tree_digest(self.repo_root)
            patch_digest = digest_payload(candidate.patch_bundle.model_dump(mode="json"))
            iterations.append(
                RepairIterationReport(
                    iteration=iteration_number,
                    actor=actor,
                    report_path=report_path,
                    candidate=candidate,
                    applied_patch_digest=patch_digest,
                    pre_apply_tree_digest=pre_apply_tree_digest,
                    post_apply_tree_digest=post_apply_tree_digest,
                )
            )
            applied_any = True
            if not self.validation_runner(candidate, self.repo_root):
                report = self._failed_validation_report(
                    actor,
                    apply=apply,
                    max_iterations=max_iterations,
                    iterations=iterations,
                )
                if write_report:
                    _write_loop_report(report)
                return report

        report = LiveSmokeRepairLoopReport(
            status=RepairLoopStatus.BUDGET_EXHAUSTED,
            actors=[actor],
            report_dir=self.report_root,
            apply=apply,
            max_iterations=max_iterations,
            iterations=iterations,
        )
        if write_report:
            _write_loop_report(report)
        return report

    def _runner(self) -> LiveSmokeRunner:
        return LiveSmokeRunner(report_root=self.report_root, runtime_factory=self.runtime_factory)

    def _failed_validation_report(
        self,
        actor: LiveSmokeActor,
        *,
        apply: bool,
        max_iterations: int,
        iterations: list[RepairIterationReport],
    ) -> LiveSmokeRepairLoopReport:
        return LiveSmokeRepairLoopReport(
            status=RepairLoopStatus.FAILED_VALIDATION,
            actors=[actor],
            report_dir=self.report_root,
            apply=apply,
            max_iterations=max_iterations,
            iterations=iterations,
        )


def _candidate_for(smoke_report: LiveSmokeReport, *, report_path: Path, repo_root: Path) -> RepairCandidate | None:
    if smoke_report.symptom_class is None or smoke_report.owning_surface is None:
        return None
    if not report_path.is_file():
        return None
    summary = summarize_repair_evidence(report_path)
    return build_repair_candidate(summary, repo_root=repo_root)


def _run_validation_commands(candidate: RepairCandidate, repo_root: Path) -> bool:
    for command in candidate.validation_commands:
        result = subprocess.run(
            shlex.split(command),
            cwd=repo_root,
            capture_output=True,
            text=True,
            check=False,
            timeout=120,
        )
        if result.returncode != 0:
            return False
    return True


def _combined_status(statuses: list[RepairLoopStatus]) -> RepairLoopStatus:
    for status in (
        RepairLoopStatus.FAILED_VALIDATION,
        RepairLoopStatus.BUDGET_EXHAUSTED,
        RepairLoopStatus.UNREPAIRABLE,
        RepairLoopStatus.CANDIDATE_READY,
        RepairLoopStatus.REPAIRED,
    ):
        if status in statuses:
            return status
    return RepairLoopStatus.PASSED


def _snapshot_smoke_report(report_path: Path, *, report_root: Path, actor: LiveSmokeActor, iteration: int) -> Path:
    if not report_path.is_file():
        return report_path
    snapshot_path = (
        report_root
        / "repair_iterations"
        / actor.value
        / f"iteration-{iteration:04d}"
        / "live_smoke_report.json"
    )
    snapshot_path.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(report_path, snapshot_path)
    return snapshot_path


def _write_loop_report(report: LiveSmokeRepairLoopReport) -> None:
    report.report_dir.mkdir(parents=True, exist_ok=True)
    (report.report_dir / _REPAIR_LOOP_REPORT_NAME).write_text(
        json.dumps(report.model_dump(mode="json"), sort_keys=True, indent=2),
        encoding="utf-8",
    )


def _worktree_is_dirty(repo_root: Path) -> bool:
    if not (repo_root / ".git").exists():
        return False
    result = subprocess.run(
        ["git", "status", "--porcelain"],
        cwd=repo_root,
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode != 0:
        return True
    return bool(result.stdout.strip())
