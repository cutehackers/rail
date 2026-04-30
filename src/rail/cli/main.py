from __future__ import annotations

import argparse
import os
from collections.abc import Sequence
from pathlib import Path

from rail.actor_runtime.codex_vault import check_trusted_codex_command, resolve_codex_command, run_codex_command
from rail.cli.setup_commands import (
    PACKAGE_NAME,
    build_codex_auth_doctor_report,
    build_codex_auth_status_report,
    build_setup_doctor_report,
    login_codex_auth,
    migrate_skill,
    package_version,
    run_codex_login,
)


def main(argv: Sequence[str] | None = None) -> int:
    parser = _parser()
    args = parser.parse_args(list(argv) if argv is not None else None)
    if args.command == "migrate":
        migration_report = migrate_skill(codex_home=_optional_path(args.codex_home), environ=os.environ)
        print(migration_report.render())
        return 0
    if args.command == "doctor":
        doctor_report = build_setup_doctor_report(
            project_root=_optional_path(args.project_root),
            codex_home=_optional_path(args.codex_home),
            environ=os.environ,
        )
        print(doctor_report.render())
        return 0 if doctor_report.ready else 1
    if args.command == "auth":
        if args.auth_command == "status":
            auth_status = build_codex_auth_status_report(environ=os.environ)
            print(auth_status.render())
            return 0 if auth_status.ready else 1
        if args.auth_command == "doctor":
            auth_doctor = build_codex_auth_doctor_report(
                project_root=_optional_path(args.project_root),
                environ=os.environ,
                command_resolver=resolve_codex_command,
                command_trust_checker=check_trusted_codex_command,
                runner=run_codex_command,
                live_check=args.live_check,
            )
            print(auth_doctor.render())
            return 0 if auth_doctor.ready else 1
        if args.auth_command == "login":
            auth_login = login_codex_auth(environ=os.environ, runner=run_codex_login)
            print(auth_login.render())
            return auth_login.returncode
    parser.print_help()
    return 1


def _parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="rail",
        description="Setup helpers for the rail-sdk Python package.",
    )
    parser.add_argument("--version", action="version", version=f"{PACKAGE_NAME} {package_version()}")
    subparsers = parser.add_subparsers(dest="command")

    migrate = subparsers.add_parser("migrate", help="install or refresh the Rail Codex skill")
    migrate.add_argument("--codex-home", help="Codex home directory; defaults to CODEX_HOME or ~/.codex")

    doctor = subparsers.add_parser("doctor", help="check Rail SDK setup readiness")
    doctor.add_argument("--codex-home", help="Codex home directory; defaults to CODEX_HOME or ~/.codex")
    doctor.add_argument("--project-root", help="target repository root; defaults to the current directory")

    auth = subparsers.add_parser("auth", help="manage Rail-owned Codex authentication setup")
    auth_subparsers = auth.add_subparsers(dest="auth_command")

    auth_subparsers.add_parser("login", help="run codex login in the Rail-owned Codex auth home")
    auth_subparsers.add_parser("status", help="check local Rail-owned Codex auth material")

    auth_doctor = auth_subparsers.add_parser("doctor", help="check Rail-owned Codex auth and command readiness")
    auth_doctor.add_argument("--project-root", help="target repository root; defaults to the current directory")
    auth_doctor.add_argument(
        "--live-check",
        action="store_true",
        help="run explicit live auth validation when supported by Rail",
    )

    return parser


def _optional_path(value: str | None) -> Path | None:
    return Path(value).expanduser() if value else None


if __name__ == "__main__":
    raise SystemExit(main())
