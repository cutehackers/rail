from __future__ import annotations

from typing import Literal

from pydantic import BaseModel, ConfigDict

from rail.supervisor.graph import SUPERVISOR_GRAPH


class SupervisorState(BaseModel):
    model_config = ConfigDict(extra="forbid")

    artifact_id: str
    current_actor: str
    revision_budget: int = 1
    terminal: bool = False
    outcome: Literal["pass", "reject"] | None = None

    @classmethod
    def created(cls, artifact_id: str) -> SupervisorState:
        return cls(artifact_id=artifact_id, current_actor=SUPERVISOR_GRAPH[0])

    def finish(self, outcome: Literal["pass", "reject"]) -> SupervisorState:
        return self.model_copy(update={"terminal": True, "outcome": outcome})
