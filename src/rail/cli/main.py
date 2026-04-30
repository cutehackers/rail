from __future__ import annotations

import argparse
import os
from collections.abc import Sequence
from pathlib import Path

from rail.cli.setup_commands import PACKAGE_NAME, build_setup_doctor_report, migrate_skill, package_version


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

    return parser


def _optional_path(value: str | None) -> Path | None:
    return Path(value).expanduser() if value else None


if __name__ == "__main__":
    raise SystemExit(main())
