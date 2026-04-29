from __future__ import annotations

from typing import Any

from rail.artifacts import ArtifactHandle, ArtifactStore, TaskIdentityDecision, decide_identity
from rail.request import HarnessRequest, normalize_draft
from rail.supervisor import supervise_artifact


def normalize_request(draft: Any) -> HarnessRequest:
    return normalize_draft(draft)


def start_task(draft: Any) -> ArtifactHandle:
    request = normalize_request(draft)
    return ArtifactStore.for_project(request.project_root).allocate(request)


def decide_task_identity(user_intent: str, known_handle: ArtifactHandle | None = None) -> TaskIdentityDecision:
    return decide_identity(user_intent=user_intent, known_handle=known_handle)


def supervise(handle: Any, *, runtime: Any | None = None) -> Any:
    return supervise_artifact(handle, runtime=runtime)


def status(handle: Any) -> Any:
    raise NotImplementedError("Status projection is implemented in Task 10.")


def result(handle: Any) -> Any:
    raise NotImplementedError("Result projection is implemented in Task 10.")
