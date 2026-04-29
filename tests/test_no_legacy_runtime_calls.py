from __future__ import annotations

from pathlib import Path


LEGACY_RUNTIME_MARKERS = (
    "codex exec",
    "./build/rail",
    "go run",
    "cmd/rail",
    "internal/runtime",
)


def test_python_actor_runtime_does_not_call_legacy_cli_or_go_runtime():
    source_root = Path("src/rail")
    python_sources = sorted(source_root.rglob("*.py"))

    assert python_sources

    findings: list[str] = []
    for source in python_sources:
        text = source.read_text(encoding="utf-8")
        for marker in LEGACY_RUNTIME_MARKERS:
            if marker in text:
                findings.append(f"{source}: {marker}")

    assert findings == []
