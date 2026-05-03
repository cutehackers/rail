from __future__ import annotations

from rail.cli import main as cli_main
from rail.live_smoke.models import LiveSmokeActor
from rail.live_smoke.repair_models import LiveSmokeRepairLoopReport, RepairLoopStatus


def test_smoke_actor_requires_live_flag(capsys) -> None:
    result = cli_main.main(["smoke", "actor", "planner"])

    assert result == 1
    assert "--live is required" in capsys.readouterr().out


def test_smoke_actors_requires_live_flag_without_runner(capsys, monkeypatch) -> None:
    def fail_runner(*_args, **_kwargs):
        raise AssertionError("LiveSmokeRunner must not be instantiated without --live")

    monkeypatch.setattr(cli_main, "LiveSmokeRunner", fail_runner)

    result = cli_main.main(["smoke", "actors"])

    assert result == 1
    assert "--live is required" in capsys.readouterr().out


def test_smoke_rejects_unknown_actor(capsys) -> None:
    result = cli_main.main(["smoke", "actor", "not_an_actor", "--live"])

    assert result == 1
    assert "unsupported live smoke actor" in capsys.readouterr().out


def test_smoke_repair_actor_requires_live_flag_without_loop(capsys, monkeypatch) -> None:
    def fail_loop(*_args, **_kwargs):
        raise AssertionError("LiveSmokeRepairLoop must not be instantiated without --live")

    monkeypatch.setattr(cli_main, "LiveSmokeRepairLoop", fail_loop)

    result = cli_main.main(["smoke", "repair", "actor", "generator"])

    assert result == 1
    assert "--live is required" in capsys.readouterr().out


def test_smoke_repair_rejects_unknown_actor(capsys) -> None:
    result = cli_main.main(["smoke", "repair", "actor", "not_an_actor", "--live"])

    assert result == 1
    assert "unsupported live smoke actor" in capsys.readouterr().out


def test_smoke_repair_actor_dry_run_prints_candidate_report_and_returns_nonzero(
    capsys,
    monkeypatch,
    tmp_path,
) -> None:
    class FakeRepairLoop:
        def __init__(self, *, report_root):
            self.report_root = report_root

        def run_actor(self, actor, *, apply=False, max_iterations=2):
            assert actor == LiveSmokeActor.GENERATOR
            assert apply is False
            assert max_iterations == 3
            return LiveSmokeRepairLoopReport(
                status=RepairLoopStatus.CANDIDATE_READY,
                actors=[actor],
                report_dir=self.report_root,
                apply=apply,
                max_iterations=max_iterations,
                iterations=[],
            )

    monkeypatch.setattr(cli_main, "LiveSmokeRepairLoop", FakeRepairLoop)

    result = cli_main.main(
        [
            "smoke",
            "repair",
            "actor",
            "generator",
            "--live",
            "--max-iterations",
            "3",
            "--report-root",
            str(tmp_path / "reports"),
        ]
    )

    output = capsys.readouterr().out
    assert result == 1
    assert '"status": "candidate_ready"' in output


def test_smoke_repair_actors_apply_returns_zero_when_repaired(capsys, monkeypatch, tmp_path) -> None:
    class FakeRepairLoop:
        def __init__(self, *, report_root):
            self.report_root = report_root

        def run_all(self, *, apply=False, max_iterations=2):
            assert apply is True
            assert max_iterations == 2
            return LiveSmokeRepairLoopReport(
                status=RepairLoopStatus.REPAIRED,
                actors=[LiveSmokeActor.PLANNER],
                report_dir=self.report_root,
                apply=apply,
                max_iterations=max_iterations,
                iterations=[],
            )

    monkeypatch.setattr(cli_main, "LiveSmokeRepairLoop", FakeRepairLoop)

    result = cli_main.main(
        [
            "smoke",
            "repair",
            "actors",
            "--live",
            "--apply",
            "--report-root",
            str(tmp_path / "reports"),
        ]
    )

    output = capsys.readouterr().out
    assert result == 0
    assert '"status": "repaired"' in output
