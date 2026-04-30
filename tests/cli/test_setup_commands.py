from __future__ import annotations

import tomllib
from pathlib import Path

from rail.cli.setup_commands import build_setup_doctor_report, migrate_skill


def test_console_scripts_expose_setup_commands():
    project = tomllib.loads(Path("pyproject.toml").read_text(encoding="utf-8"))["project"]

    assert project["scripts"] == {
        "rail": "rail.cli.main:main",
        "rail-sdk": "rail.cli.main:main",
    }


def test_migrate_skill_installs_packaged_rail_skill(tmp_path):
    report = migrate_skill(codex_home=tmp_path, environ={})

    skill = tmp_path / "skills" / "rail" / "SKILL.md"
    assert report.skill_installed is True
    assert report.skill_dir == skill.parent
    assert skill.read_text(encoding="utf-8").startswith("---")
    assert "Use this skill" in skill.read_text(encoding="utf-8")


def test_setup_doctor_uses_current_project_and_reports_missing_key(tmp_path):
    report = build_setup_doctor_report(
        project_root=tmp_path,
        codex_home=tmp_path,
        environ={},
        homebrew_detector=lambda: False,
    )

    assert report.ready is False
    assert report.project_root == tmp_path.resolve()
    assert "OPENAI_API_KEY is not configured" in report.errors
    assert "rail migrate" in report.next_steps


def test_setup_doctor_is_ready_after_migration_and_operator_key(tmp_path):
    migrate_skill(
        codex_home=tmp_path,
        environ={"OPENAI_API_KEY": "sk-test-secret"},
        homebrew_detector=lambda: False,
    )

    report = build_setup_doctor_report(
        project_root=tmp_path,
        codex_home=tmp_path,
        environ={"OPENAI_API_KEY": "sk-test-secret"},
        credential_preflight=lambda _sources, _policy: None,
        homebrew_detector=lambda: False,
    )

    assert report.ready is True
    assert report.errors == []


def test_cli_main_runs_migrate_and_doctor_without_target_repo_argument(tmp_path, monkeypatch, capsys):
    from rail.cli.main import main

    monkeypatch.chdir(tmp_path)
    monkeypatch.delenv("OPENAI_API_KEY", raising=False)
    assert main(["migrate", "--codex-home", str(tmp_path)]) == 0
    assert (tmp_path / "skills" / "rail" / "SKILL.md").is_file()

    assert main(["doctor", "--codex-home", str(tmp_path)]) == 1
    output = capsys.readouterr().out
    assert "OPENAI_API_KEY is not configured" in output
