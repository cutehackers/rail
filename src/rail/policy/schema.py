from __future__ import annotations

from typing import Literal

from pydantic import BaseModel, ConfigDict, field_validator, model_validator

RuntimeProvider = Literal["openai_agents_sdk"]
MutationMode = Literal["patch_bundle", "read_only", "direct"]
NetworkMode = Literal["disabled", "restricted", "enabled"]
SandboxMode = Literal["external_worktree", "copy"]
ApprovalMode = Literal["never", "on_request", "always"]


class RuntimePolicy(BaseModel):
    model_config = ConfigDict(extra="forbid")

    provider: RuntimeProvider
    model: str
    timeout_seconds: int

    @field_validator("timeout_seconds")
    @classmethod
    def _positive_timeout(cls, value: int) -> int:
        if value <= 0:
            raise ValueError("timeout_seconds must be positive")
        return value


class ActorRuntimeSettings(BaseModel):
    model_config = ConfigDict(extra="forbid")

    max_actor_turns: int
    direct_target_mutation: bool

    @field_validator("max_actor_turns")
    @classmethod
    def _positive_turns(cls, value: int) -> int:
        if value <= 0:
            raise ValueError("max_actor_turns must be positive")
        return value


class WorkspacePolicy(BaseModel):
    model_config = ConfigDict(extra="forbid")

    mutation_mode: MutationMode
    network_mode: NetworkMode
    sandbox_mode: SandboxMode


class ShellToolPolicy(BaseModel):
    model_config = ConfigDict(extra="forbid")

    enabled: bool
    allowlist: list[str]
    timeout_seconds: int
    max_output_bytes: int


class FilesystemToolPolicy(BaseModel):
    model_config = ConfigDict(extra="forbid")

    enabled: bool
    allowlist: list[str]
    max_file_bytes: int


class SimpleToolPolicy(BaseModel):
    model_config = ConfigDict(extra="forbid")

    enabled: bool
    allowlist: list[str]


class ToolsPolicy(BaseModel):
    model_config = ConfigDict(extra="forbid")

    shell: ShellToolPolicy
    filesystem: FilesystemToolPolicy
    network: SimpleToolPolicy
    mcp: SimpleToolPolicy


class CapabilityPolicy(BaseModel):
    model_config = ConfigDict(extra="forbid")

    patch_apply: bool
    validation: bool
    binary_files: bool


class ApprovalPolicy(BaseModel):
    model_config = ConfigDict(extra="forbid")

    mode: ApprovalMode


class ActorRuntimePolicyV2(BaseModel):
    model_config = ConfigDict(extra="forbid")

    runtime: RuntimePolicy
    actor_runtime: ActorRuntimeSettings
    workspace: WorkspacePolicy
    tools: ToolsPolicy
    capabilities: CapabilityPolicy
    approval_policy: ApprovalPolicy

    @model_validator(mode="after")
    def _reject_direct_mutation(self) -> ActorRuntimePolicyV2:
        if self.actor_runtime.direct_target_mutation:
            raise ValueError("direct_target_mutation is not allowed")
        return self
