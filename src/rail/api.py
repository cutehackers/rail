from __future__ import annotations

from typing import Any

from rail.artifacts import ArtifactHandle, ArtifactStore, TaskIdentityDecision, decide_identity
from rail.artifacts.handle import load_handle_file
from rail.artifacts.projection import project_result, project_status
from rail.request import HarnessRequest, normalize_draft
from rail.supervisor import supervise_artifact


def specify(draft: Any) -> HarnessRequest:
    return normalize_draft(draft)


def start_task(draft: Any) -> ArtifactHandle:
    request = specify(draft)
    return ArtifactStore.for_project(request.project_root).allocate(request)


def load_handle(path: Any) -> ArtifactHandle:
    return load_handle_file(path)


def decide_task_identity(user_intent: str, known_handle: ArtifactHandle | None = None) -> TaskIdentityDecision:
    return decide_identity(user_intent=user_intent, known_handle=known_handle)


def supervise(handle: Any, *, runtime: Any | None = None) -> Any:
    return supervise_artifact(handle, runtime=runtime)


def status(handle: Any) -> Any:
    return project_status(handle)


def result(handle: Any) -> Any:
    return project_result(handle)
