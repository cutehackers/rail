from __future__ import annotations

import argparse
import json
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
from rail.live_smoke import LiveSmokeActor, LiveSmokeVerdict
from rail.live_smoke.repair_loop import LiveSmokeRepairLoop
from rail.live_smoke.repair_models import RepairLoopStatus
from rail.live_smoke.runner import LiveSmokeRunner


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
    if args.command == "smoke":
        return _run_smoke_command(args)
    parser.print_help()
    return 1


def _run_smoke_command(args: argparse.Namespace) -> int:
    if args.smoke_command == "repair":
        return _run_smoke_repair_command(args)

    if not getattr(args, "live", False):
        print("--live is required for actor live smoke")
        return 1

    if args.smoke_command == "actor":
        try:
            actor = LiveSmokeActor(args.actor_name)
        except ValueError:
            print(f"unsupported live smoke actor: {args.actor_name}")
            return 1
        runner = LiveSmokeRunner(report_root=args.report_root)
        reports = [runner.run_actor(actor)]
    elif args.smoke_command == "actors":
        runner = LiveSmokeRunner(report_root=args.report_root)
        reports = runner.run_all()
    else:
        return 1

    for report in reports:
        print(json.dumps(report.model_dump(mode="json"), sort_keys=True))

    return 0 if all(report.verdict == LiveSmokeVerdict.PASSED for report in reports) else 1


def _run_smoke_repair_command(args: argparse.Namespace) -> int:
    if not getattr(args, "live", False):
        print("--live is required for actor live smoke")
        return 1

    repair_loop = LiveSmokeRepairLoop(report_root=args.report_root)
    if args.repair_command == "actor":
        try:
            actor = LiveSmokeActor(args.actor_name)
        except ValueError:
            print(f"unsupported live smoke actor: {args.actor_name}")
            return 1
        report = repair_loop.run_actor(actor, apply=args.apply, max_iterations=args.max_iterations)
    elif args.repair_command == "actors":
        report = repair_loop.run_all(apply=args.apply, max_iterations=args.max_iterations)
    else:
        return 1

    print(json.dumps(report.model_dump(mode="json"), sort_keys=True))
    return 0 if report.status in {RepairLoopStatus.PASSED, RepairLoopStatus.REPAIRED} else 1


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

    smoke = subparsers.add_parser("smoke", help="run optional Rail smoke diagnostics")
    smoke_subparsers = smoke.add_subparsers(dest="smoke_command")

    smoke_actor = smoke_subparsers.add_parser("actor", help="run one actor live smoke")
    smoke_actor.add_argument("actor_name")
    smoke_actor.add_argument("--live", action="store_true", help="run the live actor smoke")
    smoke_actor.add_argument(
        "--report-root",
        type=Path,
        default=Path(".harness/live-smoke"),
        help="directory for live smoke reports; defaults to .harness/live-smoke",
    )

    smoke_actors = smoke_subparsers.add_parser("actors", help="run all actor live smokes")
    smoke_actors.add_argument("--live", action="store_true", help="run the live actor smokes")
    smoke_actors.add_argument(
        "--report-root",
        type=Path,
        default=Path(".harness/live-smoke"),
        help="directory for live smoke reports; defaults to .harness/live-smoke",
    )

    smoke_repair = smoke_subparsers.add_parser("repair", help="repair optional actor live smoke diagnostics")
    repair_subparsers = smoke_repair.add_subparsers(dest="repair_command")

    repair_actor = repair_subparsers.add_parser("actor", help="repair one actor live smoke")
    repair_actor.add_argument("actor_name")
    _add_smoke_repair_options(repair_actor)

    repair_actors = repair_subparsers.add_parser("actors", help="repair every actor live smoke")
    _add_smoke_repair_options(repair_actors)

    return parser


def _add_smoke_repair_options(parser: argparse.ArgumentParser) -> None:
    parser.add_argument("--live", action="store_true", help="run the live actor smoke")
    parser.add_argument("--apply", action="store_true", help="apply safe repair candidates and rerun live smoke")
    parser.add_argument(
        "--max-iterations",
        type=int,
        default=2,
        help="maximum repair iterations per actor; defaults to 2",
    )
    parser.add_argument(
        "--report-root",
        type=Path,
        default=Path(".harness/live-smoke"),
        help="directory for live smoke reports; defaults to .harness/live-smoke",
    )


def _optional_path(value: str | None) -> Path | None:
    return Path(value).expanduser() if value else None


if __name__ == "__main__":
    raise SystemExit(main())
