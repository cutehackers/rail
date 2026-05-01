from __future__ import annotations

from pathlib import Path
from typing import Any, Final, Literal

from pydantic import BaseModel, ConfigDict, Field, field_validator

RequestVersion = Literal["1"]
TaskType = Literal["bug_fix", "feature_addition", "safe_refactor", "test_repair"]
RiskTolerance = Literal["low", "medium", "high"]
ValidationProfile = Literal["standard", "smoke"]

DEFAULT_REQUEST_VERSION: Final = "1"
DEFAULT_PRIORITY: Final = "medium"
DEFAULT_RISK_TOLERANCE_BY_TASK_TYPE: Final[dict[str, RiskTolerance]] = {
    "bug_fix": "low",
    "feature_addition": "low",
    "safe_refactor": "medium",
    "test_repair": "low",
}
_VALIDATION_PROFILE_ALIASES: Final[dict[str, ValidationProfile]] = {
    "": "standard",
    "real": "standard",
    "standard": "standard",
    "smoke": "smoke",
}


class RequestContext(BaseModel):
    model_config = ConfigDict(extra="forbid")

    feature: str = ""
    suspected_files: list[str] = Field(default_factory=list)
    related_files: list[str] = Field(default_factory=list)
    validation_roots: list[str] = Field(default_factory=list)
    validation_targets: list[str] = Field(default_factory=list)

    @field_validator("feature", mode="before")
    @classmethod
    def _normalize_feature(cls, value: Any) -> str:
        return _trim_optional_string(value)

    @field_validator("suspected_files", "related_files", "validation_roots", "validation_targets", mode="before")
    @classmethod
    def _normalize_string_lists(cls, value: Any) -> list[str]:
        return _normalize_string_list(value)


class RequestDraft(BaseModel):
    model_config = ConfigDict(extra="forbid", populate_by_name=True)

    request_version: str = Field(default=DEFAULT_REQUEST_VERSION, alias="request_version")
    project_root: str = Field(alias="project_root")
    task_type: TaskType
    goal: str
    context: RequestContext = Field(default_factory=RequestContext)
    constraints: list[str] = Field(default_factory=list)
    definition_of_done: list[str] = Field(default_factory=list, alias="definition_of_done")
    priority: str = DEFAULT_PRIORITY
    risk_tolerance: RiskTolerance | None = Field(default=None, alias="risk_tolerance")
    validation_profile: ValidationProfile = Field(default="standard", alias="validation_profile")

    @field_validator("request_version", mode="before")
    @classmethod
    def _validate_request_version(cls, value: Any) -> str:
        version = _trim_optional_string(value) or DEFAULT_REQUEST_VERSION
        if version != DEFAULT_REQUEST_VERSION:
            raise ValueError("unsupported request_version")
        return version

    @field_validator("project_root", mode="before")
    @classmethod
    def _normalize_project_root(cls, value: Any) -> str:
        project_root = _trim_required_string(value, "project_root")
        if not Path(project_root).is_absolute():
            raise ValueError("project_root must be an absolute path")
        return project_root

    @field_validator("task_type", mode="before")
    @classmethod
    def _normalize_task_type(cls, value: Any) -> str:
        return _trim_required_string(value, "task_type").lower()

    @field_validator("goal", mode="before")
    @classmethod
    def _normalize_goal(cls, value: Any) -> str:
        return _trim_required_string(value, "goal")

    @field_validator("constraints", "definition_of_done", mode="before")
    @classmethod
    def _normalize_string_lists(cls, value: Any) -> list[str]:
        return _normalize_string_list(value)

    @field_validator("priority", mode="before")
    @classmethod
    def _normalize_priority(cls, value: Any) -> str:
        return _trim_optional_string(value) or DEFAULT_PRIORITY

    @field_validator("risk_tolerance", mode="before")
    @classmethod
    def _normalize_risk_tolerance(cls, value: Any) -> str | None:
        normalized = _trim_optional_string(value).lower()
        return normalized or None

    @field_validator("validation_profile", mode="before")
    @classmethod
    def _normalize_validation_profile(cls, value: Any) -> str:
        normalized = _trim_optional_string(value).lower()
        try:
            return _VALIDATION_PROFILE_ALIASES[normalized]
        except KeyError as exc:
            raise ValueError("unsupported validation_profile") from exc


class HarnessRequest(BaseModel):
    model_config = ConfigDict(extra="forbid", populate_by_name=True)

    request_version: RequestVersion = Field(alias="request_version")
    project_root: str = Field(alias="project_root")
    task_type: TaskType
    goal: str
    context: RequestContext
    constraints: list[str]
    definition_of_done: list[str] = Field(alias="definition_of_done")
    priority: str
    risk_tolerance: RiskTolerance = Field(alias="risk_tolerance")
    validation_profile: ValidationProfile = Field(alias="validation_profile")


def _trim_optional_string(value: Any) -> str:
    if value is None:
        return ""
    if not isinstance(value, str):
        raise ValueError("expected a string")
    return value.strip()


def _trim_required_string(value: Any, field: str) -> str:
    normalized = _trim_optional_string(value)
    if not normalized:
        raise ValueError(f"{field} is required")
    return normalized


def _normalize_string_list(value: Any) -> list[str]:
    if value is None:
        return []
    if not isinstance(value, list):
        raise ValueError("expected a list of strings")

    normalized: list[str] = []
    for item in value:
        trimmed = _trim_optional_string(item)
        if trimmed:
            normalized.append(trimmed)
    return normalized
