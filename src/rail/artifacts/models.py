from __future__ import annotations

from datetime import datetime
from pathlib import Path
from typing import Literal

from pydantic import BaseModel, ConfigDict, Field


class ArtifactHandle(BaseModel):
    model_config = ConfigDict(extra="forbid")

    schema_version: Literal["1"] = "1"
    artifact_id: str
    artifact_dir: Path
    project_root: Path
    request_snapshot_digest: str
    effective_policy_digest: str | None = None
    created_at: datetime


class WorkflowState(BaseModel):
    model_config = ConfigDict(extra="forbid")

    schema_version: Literal["1"] = "1"
    artifact_id: str
    status: Literal["created"] = "created"


class RunStatus(BaseModel):
    model_config = ConfigDict(extra="forbid")

    schema_version: Literal["1"] = "1"
    artifact_id: str
    status: Literal["created"] = "created"


class ActorRunRecord(BaseModel):
    model_config = ConfigDict(extra="forbid")

    schema_version: Literal["1"] = "1"
    actor: str
    status: str


class TerminalSummary(BaseModel):
    model_config = ConfigDict(extra="forbid")

    schema_version: Literal["1"] = "1"
    artifact_id: str
    summary: str = ""


class TaskIdentityDecision(BaseModel):
    model_config = ConfigDict(extra="forbid")

    flow: Literal["fresh_task", "existing_artifact", "clarification_needed"]
    reason: str
    handle: ArtifactHandle | None = None
    requires_clarification: bool = Field(default=False)
