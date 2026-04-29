from __future__ import annotations

from pathlib import Path

import pytest

from rail.auth.credentials import (
    CredentialSource,
    build_actor_environment,
    discover_sdk_credential_sources,
    validate_credential_source,
)
from rail.actor_runtime.agents import RuntimeReadiness
from rail.cli.doctor import actor_runtime_doctor, credential_doctor


@pytest.mark.parametrize("category", ["operator_env", "operator_keychain", "ci_secret"])
def test_operator_credential_source_categories_are_accepted(tmp_path, category):
    source = CredentialSource(category=category, name="OPENAI_API_KEY")

    assert validate_credential_source(source, project_root=tmp_path) == source


@pytest.mark.parametrize("category", ["target_env", "target_file", "local_file"])
def test_target_or_local_credential_sources_are_rejected(tmp_path, category):
    source = CredentialSource(category=category, name="OPENAI_API_KEY")

    with pytest.raises(ValueError, match="credential source"):
        validate_credential_source(source, project_root=tmp_path)


def test_target_local_credential_file_is_rejected(tmp_path):
    target_secret = tmp_path / ".harness" / "secrets" / "openai.key"
    target_secret.parent.mkdir(parents=True)
    target_secret.write_text("secret", encoding="utf-8")
    source = CredentialSource(category="operator_env", name="OPENAI_API_KEY", path=target_secret)

    with pytest.raises(ValueError, match="target-local"):
        validate_credential_source(source, project_root=tmp_path)


def test_actor_environment_is_minimum_necessary(tmp_path):
    source = CredentialSource(category="operator_env", name="OPENAI_API_KEY", value="sk-test-secret")

    env = build_actor_environment([source], project_root=tmp_path)

    assert env == {"OPENAI_API_KEY": "sk-test-secret"}


def test_discovers_operator_env_sdk_credential(monkeypatch):
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test-secret")

    sources = discover_sdk_credential_sources()

    assert sources == [CredentialSource(category="operator_env", name="OPENAI_API_KEY", value="sk-test-secret")]


def test_ignores_empty_operator_env_sdk_credential(monkeypatch):
    monkeypatch.setenv("OPENAI_API_KEY", " ")

    assert discover_sdk_credential_sources() == []


def test_doctor_reports_category_without_secret_or_machine_path(tmp_path):
    report = credential_doctor(
        [
            CredentialSource(
                category="operator_env",
                name="OPENAI_API_KEY",
                value="sk-live-secret",
                path=Path("/absolute/path/to/operator-secret"),
            )
        ],
        project_root=tmp_path,
    )

    rendered = report.render()
    assert "operator_env" in rendered
    assert "ready" in rendered
    assert "sk-live-secret" not in rendered
    assert "/absolute/path/to/operator-secret" not in rendered


def test_actor_runtime_doctor_reports_readiness_without_secret():
    report = actor_runtime_doctor(
        RuntimeReadiness(
            ready=False,
            reason="operator SDK credential is not configured for OPENAI_API_KEY=sk-live-secret",
            credential_source=None,
        )
    )

    rendered = report.render()
    assert "blocked" in rendered
    assert "credential" in rendered
    assert "sk-live-secret" not in rendered
    assert "OPENAI_API_KEY=[REDACTED]" in rendered
