from __future__ import annotations

import stat
from pathlib import Path

import pytest
import yaml

from rail.policy import load_effective_policy
from rail.policy import load as policy_load
from rail.policy.schema import ActorRuntimePolicyV2
from rail.policy.validate import digest_policy, narrow_policy


def test_default_actor_runtime_policy_loads():
    policy = load_effective_policy(Path("."))

    assert policy.runtime.provider == "codex_vault"
    assert policy.actor_runtime.direct_target_mutation is False
    assert policy.tools.shell.enabled is True
    assert set(policy.tools.shell.allowlist) == {"pwd", "ls", "find", "rg", "sed", "cat", "wc", "head", "tail", "stat", "test"}
    assert policy.tools.filesystem.enabled is False
    assert policy.tools.network.enabled is False
    assert policy.tools.mcp.enabled is False
    assert policy.workspace.mutation_mode == "patch_bundle"


def test_default_policy_load_is_not_controlled_by_current_working_directory(tmp_path, monkeypatch):
    hostile_cwd = tmp_path / "hostile"
    hostile_policy = hostile_cwd / "assets" / "defaults" / "supervisor"
    hostile_policy.mkdir(parents=True)
    (hostile_policy / "actor_runtime.yaml").write_text("runtime:\n  provider: codex_cli\n", encoding="utf-8")
    monkeypatch.chdir(hostile_cwd)

    policy = load_effective_policy(tmp_path)

    assert policy.runtime.provider == "codex_vault"


def test_default_policy_load_does_not_require_source_checkout_asset_path(tmp_path, monkeypatch):
    monkeypatch.setattr(policy_load, "_DEFAULT_POLICY_PATH", tmp_path / "missing.yaml", raising=False)

    policy = load_effective_policy(tmp_path)

    assert policy.runtime.provider == "codex_vault"


def test_openai_agents_sdk_policy_is_valid_when_operator_default_selects_it(tmp_path):
    data = load_effective_policy(Path(".")).model_dump(mode="json")
    data["runtime"]["provider"] = "openai_agents_sdk"
    data["tools"]["shell"]["enabled"] = False
    data["tools"]["shell"]["allowlist"] = []

    policy = ActorRuntimePolicyV2.model_validate(data)

    assert policy.runtime.provider == "openai_agents_sdk"
    assert policy.tools.shell.enabled is False


@pytest.mark.parametrize(
    "tool_update",
    [
        lambda data: data["tools"]["shell"].update({"enabled": True}),
        lambda data: data["tools"]["filesystem"].update({"enabled": True}),
        lambda data: data["tools"]["network"].update({"enabled": True}),
        lambda data: data["tools"]["mcp"].update({"enabled": True}),
    ],
)
def test_openai_agents_sdk_policy_rejects_enabled_host_tools(tool_update):
    data = load_effective_policy(Path(".")).model_dump(mode="json")
    data["runtime"]["provider"] = "openai_agents_sdk"
    data["tools"]["shell"]["enabled"] = False
    data["tools"]["shell"]["allowlist"] = []
    tool_update(data)

    with pytest.raises(ValueError, match="openai_agents_sdk"):
        ActorRuntimePolicyV2.model_validate(data)


def test_operator_default_policy_path_can_select_openai_agents_sdk(tmp_path, monkeypatch):
    target = tmp_path / "target"
    target.mkdir()
    policy_path = tmp_path / "operator-policy.yaml"
    data = load_effective_policy(Path(".")).model_dump(mode="json")
    data["runtime"]["provider"] = "openai_agents_sdk"
    data["tools"]["shell"]["enabled"] = False
    data["tools"]["shell"]["allowlist"] = []
    policy_path.write_text(yaml.safe_dump(data), encoding="utf-8")
    monkeypatch.setenv("RAIL_OPERATOR_ACTOR_RUNTIME_POLICY", str(policy_path))

    policy = load_effective_policy(target)

    assert policy.runtime.provider == "openai_agents_sdk"
    assert policy.tools.shell.enabled is False


def test_operator_default_policy_path_must_be_absolute(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    monkeypatch.setenv("RAIL_OPERATOR_ACTOR_RUNTIME_POLICY", "operator-policy.yaml")

    with pytest.raises(ValueError, match="absolute"):
        load_effective_policy(tmp_path)


def test_operator_default_policy_path_must_exist(tmp_path, monkeypatch):
    monkeypatch.setenv("RAIL_OPERATOR_ACTOR_RUNTIME_POLICY", str(tmp_path / "missing.yaml"))

    with pytest.raises(ValueError, match="exist"):
        load_effective_policy(tmp_path)


def test_operator_default_policy_path_must_not_be_symlink(tmp_path, monkeypatch):
    real_policy = tmp_path / "operator-policy.yaml"
    real_policy.write_text(yaml.safe_dump(load_effective_policy(Path(".")).model_dump(mode="json")), encoding="utf-8")
    symlink = tmp_path / "policy-link.yaml"
    symlink.symlink_to(real_policy)
    monkeypatch.setenv("RAIL_OPERATOR_ACTOR_RUNTIME_POLICY", str(symlink))

    with pytest.raises(ValueError, match="symlink"):
        load_effective_policy(tmp_path)


def test_operator_default_policy_path_must_not_be_inside_target_repo(tmp_path, monkeypatch):
    target = tmp_path / "target"
    policy_dir = target / ".harness" / "supervisor"
    policy_dir.mkdir(parents=True)
    policy_path = policy_dir / "operator-policy.yaml"
    policy_path.write_text(yaml.safe_dump(load_effective_policy(Path(".")).model_dump(mode="json")), encoding="utf-8")
    monkeypatch.setenv("RAIL_OPERATOR_ACTOR_RUNTIME_POLICY", str(policy_path))

    with pytest.raises(ValueError, match="target"):
        load_effective_policy(target)


@pytest.mark.parametrize("writable_bit", [stat.S_IWGRP, stat.S_IWOTH])
def test_operator_default_policy_path_must_not_be_group_or_world_writable(tmp_path, monkeypatch, writable_bit):
    target = tmp_path / "target"
    target.mkdir()
    policy_path = tmp_path / "operator-policy.yaml"
    policy_path.write_text(yaml.safe_dump(load_effective_policy(Path(".")).model_dump(mode="json")), encoding="utf-8")
    policy_path.chmod(policy_path.stat().st_mode | writable_bit)
    monkeypatch.setenv("RAIL_OPERATOR_ACTOR_RUNTIME_POLICY", str(policy_path))

    with pytest.raises(ValueError, match="writable"):
        load_effective_policy(target)


def test_target_policy_can_narrow_capabilities(tmp_path):
    overlay = ActorRuntimePolicyV2.model_validate(
        {
            "runtime": {"provider": "codex_vault", "model": "gpt-5.2", "timeout_seconds": 120},
            "actor_runtime": {"max_actor_turns": 2, "direct_target_mutation": False},
            "workspace": {"mutation_mode": "patch_bundle", "network_mode": "disabled", "sandbox_mode": "external_worktree"},
            "tools": {
                "shell": {"enabled": False, "allowlist": [], "timeout_seconds": 20, "max_output_bytes": 2000},
                "filesystem": {"enabled": False, "allowlist": [], "max_file_bytes": 100000},
                "network": {"enabled": False, "allowlist": []},
                "mcp": {"enabled": False, "allowlist": []},
            },
            "capabilities": {"patch_apply": True, "validation": True, "binary_files": False},
            "approval_policy": {"mode": "never"},
        }
    )

    narrowed = narrow_policy(load_effective_policy(tmp_path), overlay)

    assert narrowed.runtime.timeout_seconds == 120
    assert narrowed.actor_runtime.max_actor_turns == 2
    assert narrowed.tools.shell.max_output_bytes == 2000
    assert narrowed.workspace.network_mode == "disabled"


def test_target_policy_cannot_grant_direct_mutation(tmp_path):
    base = load_effective_policy(tmp_path)
    overlay = base.model_copy(deep=True)
    overlay.actor_runtime.direct_target_mutation = True

    with pytest.raises(ValueError, match="direct_target_mutation"):
        narrow_policy(base, overlay)


def test_target_policy_cannot_enable_disabled_tool(tmp_path):
    base = load_effective_policy(tmp_path)
    overlay = base.model_copy(deep=True)
    overlay.tools.filesystem.enabled = True

    with pytest.raises(ValueError, match="filesystem.enabled"):
        narrow_policy(base, overlay)


def test_unknown_policy_keys_are_rejected():
    with pytest.raises(ValueError, match="extra"):
        ActorRuntimePolicyV2.model_validate(
            {
                "runtime": {"provider": "openai_agents_sdk", "model": "gpt-5.2", "timeout_seconds": 180},
                "actor_runtime": {"max_actor_turns": 3, "direct_target_mutation": False},
                "workspace": {"mutation_mode": "patch_bundle", "network_mode": "disabled", "sandbox_mode": "external_worktree"},
                "tools": {
                    "shell": {"enabled": False, "allowlist": [], "timeout_seconds": 30, "max_output_bytes": 4000},
                    "filesystem": {"enabled": False, "allowlist": [], "max_file_bytes": 100000},
                    "network": {"enabled": False, "allowlist": []},
                    "mcp": {"enabled": False, "allowlist": []},
                },
                "capabilities": {"patch_apply": True, "validation": True, "binary_files": False},
                "approval_policy": {"mode": "never"},
                "surprise": True,
            }
        )


def test_nested_tool_allowlist_and_resource_caps_cannot_broaden(tmp_path):
    base = load_effective_policy(tmp_path)
    overlay = base.model_copy(deep=True)
    overlay.tools.filesystem.allowlist.append("target/**")

    with pytest.raises(ValueError, match="filesystem.allowlist"):
        narrow_policy(base, overlay)

    overlay = base.model_copy(deep=True)
    overlay.tools.shell.max_output_bytes = base.tools.shell.max_output_bytes + 1
    with pytest.raises(ValueError, match="shell.max_output_bytes"):
        narrow_policy(base, overlay)


@pytest.mark.parametrize(
    ("field", "broaden"),
    [
        ("workspace.network_mode", lambda p: setattr(p.workspace, "network_mode", "restricted")),
        ("workspace.mutation_mode", lambda p: setattr(p.workspace, "mutation_mode", "direct")),
        ("approval_policy.mode", lambda p: setattr(p.approval_policy, "mode", "on_request")),
    ],
)
def test_enum_ordering_rejects_broader_values(tmp_path, field, broaden):
    base = load_effective_policy(tmp_path)
    overlay = base.model_copy(deep=True)
    broaden(overlay)

    with pytest.raises(ValueError, match=field):
        narrow_policy(base, overlay)


def test_effective_policy_digest_is_stable(tmp_path):
    policy = load_effective_policy(tmp_path)

    assert digest_policy(policy) == digest_policy(ActorRuntimePolicyV2.model_validate(policy.model_dump(mode="json")))
    assert digest_policy(policy).startswith("sha256:")


def test_unknown_runtime_provider_is_rejected():
    data = load_effective_policy(Path(".")).model_dump(mode="json")
    data["runtime"]["provider"] = "codex_vualt"

    with pytest.raises(ValueError, match="provider"):
        ActorRuntimePolicyV2.model_validate(data)


def test_target_policy_cannot_change_provider_to_openai_agents_sdk(tmp_path):
    base = load_effective_policy(tmp_path)
    overlay = base.model_copy(deep=True)
    overlay.runtime.provider = "openai_agents_sdk"

    with pytest.raises(ValueError, match="runtime.provider"):
        narrow_policy(base, overlay)


def test_policy_docs_do_not_contain_home_directory_examples():
    docs = [
        Path("docs/superpowers/specs/2026-04-29-python-actor-runtime-rail-redesign.md"),
        Path("docs/superpowers/plans/2026-04-29-python-actor-runtime-rail-redesign.md"),
    ]

    for doc in docs:
        text = doc.read_text(encoding="utf-8")
        assert "/Users/" not in text
        assert "~/" not in text
