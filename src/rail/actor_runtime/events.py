from __future__ import annotations

from rail.auth.redaction import redact_secrets


def normalize_sdk_event(payload: dict[str, object]) -> dict[str, object]:
    return redact_secrets(payload)
