from __future__ import annotations

from pathlib import Path

from pydantic import BaseModel, ConfigDict

from rail.actor_runtime.agents import RuntimeReadiness
from rail.auth.credentials import CredentialSource, validate_credential_source
from rail.auth.redaction import redact_secrets


class CredentialDoctorReport(BaseModel):
    model_config = ConfigDict(extra="forbid")

    ready: bool
    categories: list[str]
    errors: list[str]

    def render(self) -> str:
        status = "ready" if self.ready else "blocked"
        categories = ", ".join(self.categories) if self.categories else "none"
        errors = "; ".join(self.errors)
        return f"credential doctor: {status}; categories: {categories}; errors: {errors}"


def credential_doctor(sources: list[CredentialSource], *, project_root: Path) -> CredentialDoctorReport:
    categories: list[str] = []
    errors: list[str] = []
    for source in sources:
        categories.append(source.category)
        try:
            validate_credential_source(source, project_root)
        except ValueError as exc:
            errors.append(str(exc))

    return CredentialDoctorReport(ready=not errors and bool(sources), categories=categories, errors=errors)


def actor_runtime_doctor(readiness: RuntimeReadiness) -> CredentialDoctorReport:
    categories = [readiness.credential_source] if readiness.credential_source else []
    errors = [] if readiness.ready else [str(redact_secrets(readiness.reason))]

    return CredentialDoctorReport(ready=readiness.ready, categories=categories, errors=errors)
