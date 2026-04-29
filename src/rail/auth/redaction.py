from __future__ import annotations

import re
from pathlib import Path
from typing import Any

_SECRET_PATTERNS = (
    re.compile(r"sk-[A-Za-z0-9_-]+"),
    re.compile(r"(?i)(OPENAI_API_KEY=)[^\s\"']+"),
)


def redact_secrets(value: Any, *, canaries: list[str] | None = None) -> Any:
    canaries = canaries or []
    if isinstance(value, dict):
        return {key: redact_secrets(item, canaries=canaries) for key, item in value.items()}
    if isinstance(value, list):
        return [redact_secrets(item, canaries=canaries) for item in value]
    if isinstance(value, tuple):
        return tuple(redact_secrets(item, canaries=canaries) for item in value)
    if isinstance(value, str):
        redacted = value
        for canary in canaries:
            redacted = redacted.replace(canary, "[REDACTED]")
        for pattern in _SECRET_PATTERNS:
            redacted = pattern.sub(lambda match: _redact_match(match), redacted)
        return redacted
    return value


def assert_no_secret_canaries(root: Path, *, canaries: list[str]) -> None:
    for path in root.rglob("*"):
        if not path.is_file():
            continue
        text = path.read_text(encoding="utf-8", errors="ignore")
        for canary in canaries:
            if canary in text:
                raise ValueError(f"secret canary leaked into artifact: {path.name}")


def _redact_match(match: re.Match[str]) -> str:
    if match.lastindex:
        return match.group(1) + "[REDACTED]"
    return "[REDACTED]"
