from __future__ import annotations

from typing import Any

from rail.request.schema import DEFAULT_RISK_TOLERANCE_BY_TASK_TYPE, HarnessRequest, RequestDraft


def normalize_draft(draft: Any) -> HarnessRequest:
    if isinstance(draft, HarnessRequest):
        return draft

    request_draft = draft if isinstance(draft, RequestDraft) else RequestDraft.model_validate(draft)
    risk_tolerance = request_draft.risk_tolerance or DEFAULT_RISK_TOLERANCE_BY_TASK_TYPE[request_draft.task_type]

    return HarnessRequest(
        request_version=request_draft.request_version,
        project_root=request_draft.project_root,
        task_type=request_draft.task_type,
        goal=request_draft.goal,
        context=request_draft.context,
        constraints=request_draft.constraints,
        definition_of_done=request_draft.definition_of_done,
        priority=request_draft.priority,
        risk_tolerance=risk_tolerance,
        validation_profile=request_draft.validation_profile,
    )
