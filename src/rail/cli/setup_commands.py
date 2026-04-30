from __future__ import annotations

import shutil
import subprocess
from collections.abc import Callable, Mapping
from importlib.metadata import PackageNotFoundError, version
from importlib.resources import files
import os
from pathlib import Path

from pydantic import BaseModel, ConfigDict

from rail.actor_runtime.agents import CredentialPreflight, validate_live_sdk_credentials
from rail.actor_runtime.codex_vault import (
    CodexCommandResolver,
    CodexCommandRunner,
    CodexCommandTrustChecker,
    CodexVaultActorRuntime,
    check_trusted_codex_command,
    resolve_codex_command,
    run_codex_command,
)
from rail.auth.credentials import (
    CredentialSource,
    codex_auth_home,
    discover_sdk_credential_sources,
    validate_codex_auth_material,
    validate_credential_source,
)
from rail.policy import load_effective_policy

PACKAGE_NAME = "rail-sdk"
CodexLoginRunner = Callable[[list[str], dict[str, str]], int]


class MigrationReport(BaseModel):
    model_config = ConfigDict(extra="forbid")

    package_name: str
    package_version: str
    skill_dir: Path
    skill_installed: bool
    openai_key_configured: bool
    old_homebrew_detected: bool
    next_steps: list[str]

    def render(self) -> str:
        lines = [
            f"{self.package_name} {self.package_version} migration",
            f"Rail skill: {'installed' if self.skill_installed else 'missing'} at {self.skill_dir}",
        ]
        if self.openai_key_configured:
            lines.append("OPENAI_API_KEY: configured")
        else:
            lines.append("OPENAI_API_KEY: missing")
        if self.old_homebrew_detected:
            lines.append("Old Homebrew rail: detected")
        lines.extend(f"Next: {step}" for step in self.next_steps)
        return "\n".join(lines)


class SetupDoctorReport(BaseModel):
    model_config = ConfigDict(extra="forbid")

    ready: bool
    project_root: Path
    skill_dir: Path
    skill_installed: bool
    package_version: str
    credential_categories: list[str]
    old_homebrew_detected: bool
    errors: list[str]
    warnings: list[str]
    next_steps: list[str]

    def render(self) -> str:
        status = "ready" if self.ready else "blocked"
        lines = [
            f"rail setup doctor: {status}",
            f"project_root: {self.project_root}",
            f"rail-sdk: {self.package_version}",
            f"Rail skill: {'installed' if self.skill_installed else 'missing'} at {self.skill_dir}",
        ]
        lines.extend(f"Error: {error}" for error in self.errors)
        lines.extend(f"Warning: {warning}" for warning in self.warnings)
        lines.extend(f"Next: {step}" for step in self.next_steps)
        return "\n".join(lines)


class CodexAuthStatusReport(BaseModel):
    model_config = ConfigDict(extra="forbid")

    ready: bool
    accepted_material: list[str]
    errors: list[str]
    next_steps: list[str]

    def render(self) -> str:
        status = "ready" if self.ready else "blocked"
        material = ", ".join(self.accepted_material) if self.accepted_material else "none"
        lines = [
            f"rail codex auth status: {status}",
            f"accepted auth material: {material}",
        ]
        lines.extend(f"Error: {error}" for error in self.errors)
        lines.extend(f"Next: {step}" for step in self.next_steps)
        return "\n".join(lines)


class CodexAuthDoctorReport(BaseModel):
    model_config = ConfigDict(extra="forbid")

    ready: bool
    auth_ready: bool
    command_ready: bool
    codex_version: str | None
    live_validation: bool
    errors: list[str]
    warnings: list[str]
    next_steps: list[str]

    def render(self) -> str:
        status = "ready" if self.ready else "blocked"
        lines = [
            f"rail codex auth doctor: {status}",
            f"auth material: {'ready' if self.auth_ready else 'blocked'}",
            f"Codex command: {'ready' if self.command_ready else 'blocked'}",
            f"live validation: {'enabled' if self.live_validation else 'skipped'}",
        ]
        if self.codex_version:
            lines.append(f"Codex version: {self.codex_version}")
        lines.extend(f"Error: {error}" for error in self.errors)
        lines.extend(f"Warning: {warning}" for warning in self.warnings)
        lines.extend(f"Next: {step}" for step in self.next_steps)
        return "\n".join(lines)


class CodexAuthLoginReport(BaseModel):
    model_config = ConfigDict(extra="forbid")

    returncode: int

    @property
    def ready(self) -> bool:
        return self.returncode == 0

    def render(self) -> str:
        status = "completed" if self.returncode == 0 else "failed"
        return f"rail codex auth login: {status}"


def migrate_skill(
    *,
    codex_home: Path | None = None,
    environ: Mapping[str, str] | None = None,
    homebrew_detector: Callable[[], bool] = lambda: detect_homebrew_rail(),
) -> MigrationReport:
    environ = os.environ if environ is None else environ
    skill_dir = _skill_dir(codex_home, environ=environ)
    _replace_skill_tree(skill_dir)
    old_homebrew_detected = homebrew_detector()
    openai_key_configured = bool(environ.get("OPENAI_API_KEY", "").strip())
    next_steps = _next_steps(
        skill_installed=True,
        openai_key_configured=openai_key_configured,
        old_homebrew_detected=old_homebrew_detected,
        sdk_credentials_required=True,
    )
    return MigrationReport(
        package_name=PACKAGE_NAME,
        package_version=package_version(),
        skill_dir=skill_dir,
        skill_installed=(skill_dir / "SKILL.md").is_file(),
        openai_key_configured=openai_key_configured,
        old_homebrew_detected=old_homebrew_detected,
        next_steps=next_steps,
    )


def build_setup_doctor_report(
    *,
    project_root: Path | None = None,
    codex_home: Path | None = None,
    environ: Mapping[str, str] | None = None,
    credential_preflight: CredentialPreflight | None = None,
    homebrew_detector: Callable[[], bool] = lambda: detect_homebrew_rail(),
) -> SetupDoctorReport:
    environ = os.environ if environ is None else environ
    root = (project_root or Path.cwd()).resolve()
    skill_dir = _skill_dir(codex_home, environ=environ)
    skill_installed = (skill_dir / "SKILL.md").is_file()
    old_homebrew_detected = homebrew_detector()
    policy = load_effective_policy(root)
    sdk_credentials_required = policy.runtime.provider == "openai_agents_sdk"
    sources = discover_sdk_credential_sources(environ)
    errors: list[str] = []
    warnings: list[str] = []

    if not skill_installed:
        errors.append("Rail skill is not installed")
    if sdk_credentials_required and not sources:
        errors.append("OPENAI_API_KEY is not configured")
    if sdk_credentials_required:
        errors.extend(_credential_errors(sources, project_root=root))
    if sdk_credentials_required and sources:
        preflight = credential_preflight or validate_live_sdk_credentials
        failure = preflight(sources, policy)
        if failure:
            errors.append(failure)
    if old_homebrew_detected:
        warnings.append("Old Homebrew rail formula is installed")

    next_steps = _next_steps(
        skill_installed=skill_installed,
        openai_key_configured=bool(sources),
        old_homebrew_detected=old_homebrew_detected,
        sdk_credentials_required=sdk_credentials_required,
    )
    return SetupDoctorReport(
        ready=not errors,
        project_root=root,
        skill_dir=skill_dir,
        skill_installed=skill_installed,
        package_version=package_version(),
        credential_categories=[source.category for source in sources],
        old_homebrew_detected=old_homebrew_detected,
        errors=errors,
        warnings=warnings,
        next_steps=next_steps,
    )


def build_codex_auth_status_report(*, environ: Mapping[str, str] | None = None) -> CodexAuthStatusReport:
    environ = os.environ if environ is None else environ
    auth_home = codex_auth_home(environ=environ)
    errors: list[str] = []
    accepted_material: list[str] = []
    try:
        accepted_material = [path.name for path in validate_codex_auth_material(auth_home)]
    except ValueError as exc:
        errors.append(str(exc))
    return CodexAuthStatusReport(
        ready=not errors,
        accepted_material=accepted_material,
        errors=errors,
        next_steps=_codex_auth_next_steps(auth_ready=not errors, command_ready=True),
    )


def build_codex_auth_doctor_report(
    *,
    project_root: Path | None = None,
    environ: Mapping[str, str] | None = None,
    command_resolver: CodexCommandResolver | None = None,
    command_trust_checker: CodexCommandTrustChecker | None = None,
    runner: CodexCommandRunner | None = None,
    live_check: bool = False,
) -> CodexAuthDoctorReport:
    environ = os.environ if environ is None else environ
    root = (project_root or Path.cwd()).resolve()
    auth_status = build_codex_auth_status_report(environ=environ)
    runtime = CodexVaultActorRuntime(
        project_root=root,
        policy=load_effective_policy(root),
        command_resolver=command_resolver or resolve_codex_command,
        command_trust_checker=command_trust_checker or check_trusted_codex_command,
        runner=runner or run_codex_command,
    )
    readiness = runtime.readiness()
    errors = [*auth_status.errors]
    if not readiness.ready:
        errors.append(readiness.reason)
    warnings: list[str] = []
    if live_check:
        warnings.append("Live Codex auth validation is not implemented for setup diagnostics")
    return CodexAuthDoctorReport(
        ready=not errors,
        auth_ready=auth_status.ready,
        command_ready=readiness.ready,
        codex_version=readiness.codex_version,
        live_validation=live_check,
        errors=errors,
        warnings=warnings,
        next_steps=_codex_auth_next_steps(auth_ready=auth_status.ready, command_ready=readiness.ready),
    )


def login_codex_auth(
    *,
    environ: Mapping[str, str] | None = None,
    runner: CodexLoginRunner | None = None,
) -> CodexAuthLoginReport:
    environ = os.environ if environ is None else environ
    auth_home = codex_auth_home(environ=environ)
    auth_home.mkdir(mode=0o700, parents=True, exist_ok=True)
    auth_home.chmod(0o700)
    env = dict(environ)
    env["CODEX_HOME"] = str(auth_home)
    login_runner = runner or run_codex_login
    return CodexAuthLoginReport(returncode=login_runner(["codex", "login"], env))


def run_codex_login(command: list[str], env: dict[str, str]) -> int:
    completed = subprocess.run(command, env=env, check=False)
    return completed.returncode


def detect_homebrew_rail() -> bool:
    brew = shutil.which("brew")
    if brew is None:
        return False
    completed = subprocess.run(
        [brew, "list", "--formula", "rail"],
        capture_output=True,
        text=True,
        check=False,
    )
    return completed.returncode == 0


def _credential_errors(sources: list[CredentialSource], *, project_root: Path) -> list[str]:
    errors: list[str] = []
    for source in sources:
        try:
            validate_credential_source(source, project_root)
        except ValueError as exc:
            errors.append(str(exc))
    return errors


def _replace_skill_tree(skill_dir: Path) -> None:
    if skill_dir.exists():
        if skill_dir.is_dir():
            shutil.rmtree(skill_dir)
        else:
            skill_dir.unlink()
    skill_dir.parent.mkdir(parents=True, exist_ok=True)
    _copy_resource_tree(files("rail").joinpath("package_assets", "skill", "Rail"), skill_dir)


def _copy_resource_tree(source, destination: Path) -> None:
    destination.mkdir(parents=True, exist_ok=True)
    for child in source.iterdir():
        target = destination / child.name
        if child.is_dir():
            _copy_resource_tree(child, target)
        else:
            target.write_bytes(child.read_bytes())


def _skill_dir(codex_home: Path | None, *, environ: Mapping[str, str]) -> Path:
    root = codex_home or Path(environ.get("CODEX_HOME", "") or Path.home() / ".codex")
    return root / "skills" / "rail"


def _next_steps(
    *,
    skill_installed: bool,
    openai_key_configured: bool,
    old_homebrew_detected: bool,
    sdk_credentials_required: bool,
) -> list[str]:
    steps: list[str] = []
    if old_homebrew_detected:
        steps.append("brew uninstall rail && brew cleanup rail")
    if not skill_installed:
        steps.append("rail migrate")
    if sdk_credentials_required and not openai_key_configured:
        steps.append("export OPENAI_API_KEY=...")
    if not steps:
        steps.append("Use the Rail skill from the target repository.")
    return steps


def _codex_auth_next_steps(*, auth_ready: bool, command_ready: bool) -> list[str]:
    steps: list[str] = []
    if not auth_ready:
        steps.append("rail auth login")
    if not command_ready:
        steps.append("Install or repair the Codex CLI.")
    if not steps:
        steps.append("Use the Rail skill from the target repository.")
    return steps


def package_version() -> str:
    try:
        return version(PACKAGE_NAME)
    except PackageNotFoundError:
        return "0.1.0"
