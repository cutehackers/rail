from __future__ import annotations

from rail.artifacts.models import ArtifactHandle, TaskIdentityDecision
from rail.artifacts.store import validate_artifact_handle

_EXISTING_ARTIFACT_INTENT_MARKERS = (
    "continue",
    "resume",
    "retry",
    "status",
    "result",
    "debug",
    "integrate",
)


def decide_identity(user_intent: str, known_handle: ArtifactHandle | None = None) -> TaskIdentityDecision:
    if known_handle is not None:
        handle = validate_artifact_handle(known_handle)
        return TaskIdentityDecision(
            flow="existing_artifact",
            reason="A validated artifact handle was supplied.",
            handle=handle,
            requires_clarification=False,
        )

    normalized_intent = user_intent.casefold()
    if any(marker in normalized_intent for marker in _EXISTING_ARTIFACT_INTENT_MARKERS):
        return TaskIdentityDecision(
            flow="clarification_needed",
            reason="Existing-artifact intent requires an artifact handle; request files are not run identity.",
            handle=None,
            requires_clarification=True,
        )

    return TaskIdentityDecision(
        flow="fresh_task",
        reason="No artifact handle was supplied, so Rail will allocate a fresh artifact for this goal.",
        handle=None,
        requires_clarification=False,
    )
