from __future__ import annotations

import pytest

from rail.auth.redaction import assert_no_secret_canaries, redact_secrets


def test_redacts_secret_canaries_from_nested_runtime_payloads():
    payload = {
        "trace": "token rail-secret-canary-123",
        "events": [{"message": "OPENAI_API_KEY=sk-test-abc"}],
        "terminal_summary": "done with rail-secret-canary-123",
        "result_projection": {"note": "safe"},
    }

    redacted = redact_secrets(payload, canaries=["rail-secret-canary-123"])

    assert "rail-secret-canary-123" not in str(redacted)
    assert "sk-test-abc" not in str(redacted)
    assert "[REDACTED]" in str(redacted)


def test_artifact_secret_canary_scan_fails_when_secret_is_present(tmp_path):
    artifact = tmp_path / "artifact"
    artifact.mkdir()
    (artifact / "events.jsonl").write_text('{"message":"rail-secret-canary-456"}\n', encoding="utf-8")

    with pytest.raises(ValueError, match="secret canary"):
        assert_no_secret_canaries(artifact, canaries=["rail-secret-canary-456"])


def test_artifact_secret_canary_scan_passes_after_redaction(tmp_path):
    artifact = tmp_path / "artifact"
    artifact.mkdir()
    (artifact / "events.jsonl").write_text('{"message":"[REDACTED]"}\n', encoding="utf-8")

    assert_no_secret_canaries(artifact, canaries=["rail-secret-canary-456"])
