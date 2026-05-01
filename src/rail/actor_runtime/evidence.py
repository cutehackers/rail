from __future__ import annotations

import json
from pathlib import Path

from rail.auth.redaction import redact_secrets


def write_runtime_evidence(
    artifact_dir: Path,
    attempt_ref: str,
    actor: str,
    payload: dict[str, object],
    *,
    events: list[dict[str, object]] | None = None,
) -> tuple[Path, Path]:
    runs_dir = artifact_dir / "runs" / attempt_ref
    runs_dir.mkdir(parents=True, exist_ok=True)
    events_ref = Path("runs") / attempt_ref / f"{actor}.events.jsonl"
    evidence_ref = Path("runs") / attempt_ref / f"{actor}.runtime_evidence.json"
    event_payloads = events
    if event_payloads is None:
        payload_events = payload.get("normalized_events")
        if isinstance(payload_events, list) and all(isinstance(item, dict) for item in payload_events):
            event_payloads = payload_events
        else:
            event_payloads = [
                {"actor": actor, "event": "completed", "status": payload.get("status", "succeeded")},
            ]
    evidence = redact_secrets(payload)
    event_lines = [json.dumps(redact_secrets(event), sort_keys=True) for event in event_payloads]
    (artifact_dir / events_ref).write_text("\n".join(event_lines) + "\n", encoding="utf-8")
    (artifact_dir / evidence_ref).write_text(json.dumps(evidence, sort_keys=True), encoding="utf-8")
    return events_ref, evidence_ref
