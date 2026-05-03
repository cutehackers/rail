from __future__ import annotations

from rail.cli import main as cli_main


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
