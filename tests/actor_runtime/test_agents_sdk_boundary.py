from __future__ import annotations

import importlib


def test_agents_sdk_boundary_constructs_agents_without_network(monkeypatch):
    monkeypatch.delenv("OPENAI_API_KEY", raising=False)

    boundary = importlib.import_module("rail.actor_runtime.sdk_boundary")

    probe = boundary.build_agents_sdk_probe()

    assert probe.package_name == "openai-agents"
    assert probe.import_module == "agents"
    assert probe.runner_type == "Runner"
    assert probe.planner_agent_type == "Agent"
    assert probe.executor_agent_type == "Agent"
    assert probe.uses_structured_outputs is True
    assert probe.tool_count == 0


def test_actor_runtime_policy_starts_with_no_host_tools():
    boundary = importlib.import_module("rail.actor_runtime.sdk_boundary")

    policy = boundary.ActorRuntimePolicy.allow_no_host_tools()
    agents = boundary.build_rail_agents(policy)

    assert policy.allow_shell is False
    assert policy.allow_network is False
    assert policy.allow_filesystem is False
    assert all(len(agent.tools) == 0 for agent in agents)
