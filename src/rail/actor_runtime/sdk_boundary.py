from __future__ import annotations

from dataclasses import dataclass
from typing import Final

from agents import Agent, Runner
from pydantic import BaseModel, ConfigDict, Field

_PACKAGE_NAME: Final = "openai-agents"
_IMPORT_MODULE: Final = "agents"


class ActorPlan(BaseModel):
    model_config = ConfigDict(extra="forbid")

    summary: str = Field(description="Bounded plan summary for the next Rail actor step.")
    next_actor: str = Field(description="Deterministic next actor selected by supervisor policy.")


class ActorExecution(BaseModel):
    model_config = ConfigDict(extra="forbid")

    summary: str = Field(description="Execution result summary.")
    patch_bundle_ref: str | None = Field(default=None, description="Reference to Rail-owned patch bundle evidence.")


@dataclass(frozen=True)
class ActorRuntimePolicy:
    allow_shell: bool
    allow_network: bool
    allow_filesystem: bool

    @classmethod
    def allow_no_host_tools(cls) -> ActorRuntimePolicy:
        return cls(allow_shell=False, allow_network=False, allow_filesystem=False)


@dataclass(frozen=True)
class AgentsSDKProbe:
    package_name: str
    import_module: str
    runner_type: str
    planner_agent_type: str
    executor_agent_type: str
    uses_structured_outputs: bool
    tool_count: int


def build_rail_agents(policy: ActorRuntimePolicy) -> tuple[Agent[None], Agent[None]]:
    tools = _tools_for_policy(policy)
    return (
        Agent(
            name="rail-planner",
            instructions="Plan one bounded Rail workflow step using only the provided request and policy context.",
            tools=list(tools),
            output_type=ActorPlan,
        ),
        Agent(
            name="rail-executor",
            instructions="Return a patch bundle reference and evidence summary without directly mutating host state.",
            tools=list(tools),
            output_type=ActorExecution,
        ),
    )


def build_agents_sdk_probe() -> AgentsSDKProbe:
    agents = build_rail_agents(ActorRuntimePolicy.allow_no_host_tools())
    return AgentsSDKProbe(
        package_name=_PACKAGE_NAME,
        import_module=_IMPORT_MODULE,
        runner_type=Runner.__name__,
        planner_agent_type=type(agents[0]).__name__,
        executor_agent_type=type(agents[1]).__name__,
        uses_structured_outputs=all(getattr(agent, "output_type", None) is not None for agent in agents),
        tool_count=sum(len(agent.tools) for agent in agents),
    )


def _tools_for_policy(policy: ActorRuntimePolicy) -> tuple[object, ...]:
    if policy.allow_shell or policy.allow_network or policy.allow_filesystem:
        raise ValueError("Task 0 only permits constructing actors with host tools disabled.")
    return ()
