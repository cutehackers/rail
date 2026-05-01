from __future__ import annotations

import subprocess
import tomllib
from pathlib import Path

from rail.actor_runtime import codex_vault
from rail.cli.setup_commands import (
    build_codex_auth_doctor_report,
    build_codex_auth_status_report,
    build_setup_doctor_report,
    login_codex_auth,
    migrate_skill,
)


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


def test_migrate_skill_default_path_points_to_codex_auth_not_openai_api_key(tmp_path):
    report = migrate_skill(codex_home=tmp_path, environ={}, homebrew_detector=lambda: False)

    rendered = report.render()
    assert "OPENAI_API_KEY" not in rendered
    assert "export OPENAI_API_KEY" not in report.next_steps
    assert report.next_steps == ["rail auth login", "rail auth doctor"]


def test_setup_doctor_with_codex_vault_default_does_not_require_openai_api_key(tmp_path):
    report = build_setup_doctor_report(
        project_root=tmp_path,
        codex_home=tmp_path,
        environ={},
        homebrew_detector=lambda: False,
    )

    assert report.ready is False
    assert report.project_root == tmp_path.resolve()
    assert "OPENAI_API_KEY is not configured" not in report.errors
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


def test_cli_main_runs_migrate_and_doctor_without_target_repo_argument(tmp_path, monkeypatch):
    from rail.cli.main import main

    monkeypatch.chdir(tmp_path)
    monkeypatch.delenv("OPENAI_API_KEY", raising=False)
    assert main(["migrate", "--codex-home", str(tmp_path)]) == 0
    assert (tmp_path / "skills" / "rail" / "SKILL.md").is_file()

    assert main(["doctor", "--codex-home", str(tmp_path)]) == 0


def test_cli_exposes_auth_status_doctor_and_login(tmp_path, monkeypatch, capsys):
    from rail.cli.main import main

    rail_home = tmp_path / "rail-home"
    auth_home = rail_home / "codex"
    auth_home.mkdir(parents=True)
    auth_file = auth_home / "auth.json"
    auth_file.write_text("{}", encoding="utf-8")
    auth_file.chmod(0o600)
    monkeypatch.setenv("RAIL_HOME", str(rail_home))

    commands: list[tuple[list[str], str]] = []

    def fake_login_runner(command: list[str], env: dict[str, str]) -> int:
        commands.append((command, env["CODEX_HOME"]))
        return 0

    monkeypatch.setattr("rail.cli.main.login_codex_auth", login_codex_auth)
    monkeypatch.setattr("rail.cli.main.run_codex_login", fake_login_runner)
    monkeypatch.setattr("rail.cli.main.resolve_codex_command", lambda: tmp_path / "bin" / "codex")
    monkeypatch.setattr(
        "rail.cli.main.check_trusted_codex_command",
        lambda _path, _target_root, _artifact_dir: None,
    )
    monkeypatch.setattr(
        "rail.cli.main.run_codex_command",
        _ready_codex_runner(),
    )

    assert main(["auth", "status"]) == 0
    status_output = capsys.readouterr().out
    assert "rail codex auth status: ready" in status_output
    assert str(auth_home) not in status_output
    assert str(rail_home) not in status_output

    assert main(["auth", "doctor", "--project-root", str(tmp_path)]) == 0
    doctor_output = capsys.readouterr().out
    assert "rail codex auth doctor: ready" in doctor_output
    assert str(auth_home) not in doctor_output
    assert str(rail_home) not in doctor_output

    assert main(["auth", "login"]) == 0
    assert commands == [(["codex", "login"], str(auth_home))]


def test_codex_auth_status_and_doctor_outputs_are_sanitized(tmp_path):
    rail_home = tmp_path / "private-rail-home"
    auth_home = rail_home / "codex"

    status = build_codex_auth_status_report(environ={"RAIL_HOME": str(rail_home)})
    status_output = status.render()
    assert status.ready is False
    assert "missing auth.json" in status.errors
    assert str(auth_home) not in status_output
    assert str(rail_home) not in status_output

    doctor = build_codex_auth_doctor_report(
        project_root=tmp_path,
        environ={"RAIL_HOME": str(rail_home)},
        command_resolver=lambda: None,
    )
    doctor_output = doctor.render()
    assert doctor.ready is False
    assert any("Codex command" in error for error in doctor.errors)
    assert str(auth_home) not in doctor_output
    assert str(rail_home) not in doctor_output


def test_codex_auth_login_sets_codex_home_to_rail_owned_auth_home(tmp_path):
    rail_home = tmp_path / "rail-home"
    calls: list[tuple[list[str], dict[str, str]]] = []

    def runner(command: list[str], env: dict[str, str]) -> int:
        calls.append((command, env))
        return 0

    report = login_codex_auth(environ={"RAIL_HOME": str(rail_home), "PATH": "/bin"}, runner=runner)

    assert report.returncode == 0
    assert calls[0][0] == ["codex", "login"]
    assert calls[0][1]["CODEX_HOME"] == str(rail_home / "codex")
    assert calls[0][1]["PATH"] == "/bin"
    assert str(rail_home) not in report.render()


def test_codex_auth_login_reports_local_execution_failure(tmp_path):
    rail_home = tmp_path / "rail-home"

    def runner(_command: list[str], _env: dict[str, str]) -> int:
        raise FileNotFoundError("codex")

    report = login_codex_auth(environ={"RAIL_HOME": str(rail_home)}, runner=runner)

    assert report.returncode == 1
    assert "failed" in report.render()
    assert str(rail_home) not in report.render()


def test_codex_auth_login_rejects_symlinked_auth_home(tmp_path):
    rail_home = tmp_path / "rail-home"
    real_auth = tmp_path / "real-auth"
    real_auth.mkdir()
    auth_home = rail_home / "codex"
    auth_home.parent.mkdir()
    auth_home.symlink_to(real_auth, target_is_directory=True)
    called = False

    def runner(_command: list[str], _env: dict[str, str]) -> int:
        nonlocal called
        called = True
        return 0

    report = login_codex_auth(environ={"RAIL_HOME": str(rail_home)}, runner=runner)

    assert report.returncode == 1
    assert called is False
    assert str(rail_home) not in report.render()


def test_run_codex_login_reports_subprocess_failure(monkeypatch):
    def fail_run(*_args, **_kwargs):
        raise subprocess.SubprocessError("boom")

    monkeypatch.setattr("subprocess.run", fail_run)

    from rail.cli.setup_commands import run_codex_login

    assert run_codex_login(["codex", "login"], {}) == 1


def test_codex_auth_doctor_renders_only_supported_version(tmp_path):
    rail_home = tmp_path / "rail-home"
    auth_home = rail_home / "codex"
    auth_home.mkdir(parents=True)
    auth_file = auth_home / "auth.json"
    auth_file.write_text("{}", encoding="utf-8")
    auth_file.chmod(0o600)

    def bad_version_runner(command: list[str]) -> codex_vault.CodexCommandRunResult:
        if command[1:] == ["--version"]:
            return codex_vault.CodexCommandRunResult(stdout="host path /private/tmp/secret", stderr="", returncode=0)
        raise AssertionError(f"unexpected command: {command}")

    report = build_codex_auth_doctor_report(
        project_root=tmp_path,
        environ={"RAIL_HOME": str(rail_home)},
        command_resolver=lambda: tmp_path / "bin" / "codex",
        command_trust_checker=lambda _path, _target_root, _artifact_dir: None,
        runner=bad_version_runner,
    )

    rendered = report.render()
    assert report.ready is False
    assert "host path" not in rendered
    assert "/private/tmp/secret" not in rendered


def _ready_codex_runner():
    def runner(command: list[str]) -> codex_vault.CodexCommandRunResult:
        if command[1:] == ["--version"]:
            return codex_vault.CodexCommandRunResult(stdout="codex-cli 0.124.0", stderr="", returncode=0)
        if command[1:] == ["exec", "--help"]:
            return codex_vault.CodexCommandRunResult(
                stdout="\n".join(codex_vault.CODEX_EXEC_REQUIRED_HELP_FLAGS),
                stderr="",
                returncode=0,
            )
        raise AssertionError(f"unexpected command: {command}")

    return runner
