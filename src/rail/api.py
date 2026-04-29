from __future__ import annotations

from typing import Any

from rail.request import HarnessRequest, normalize_draft


def normalize_request(draft: Any) -> HarnessRequest:
    return normalize_draft(draft)


def start_task(draft: Any) -> Any:
    raise NotImplementedError("Task artifact allocation is implemented in Task 3.")


def supervise(handle: Any) -> Any:
    raise NotImplementedError("Supervisor graph execution is implemented in Task 5.")


def status(handle: Any) -> Any:
    raise NotImplementedError("Status projection is implemented in Task 10.")


def result(handle: Any) -> Any:
    raise NotImplementedError("Result projection is implemented in Task 10.")
