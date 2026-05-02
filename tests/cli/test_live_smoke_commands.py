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
