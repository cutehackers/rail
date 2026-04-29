from __future__ import annotations

import json
from pathlib import Path

from rail.auth.redaction import redact_secrets


def write_runtime_evidence(artifact_dir: Path, actor: str, payload: dict[str, object]) -> tuple[Path, Path]:
    runs_dir = artifact_dir / "runs"
    runs_dir.mkdir(exist_ok=True)
    events_ref = Path("runs") / f"{actor}.events.jsonl"
    evidence_ref = Path("runs") / f"{actor}.runtime_evidence.json"
    event = redact_secrets({"actor": actor, "event": "completed", "status": payload.get("status", "succeeded")})
    evidence = redact_secrets(payload)
    (artifact_dir / events_ref).write_text(json.dumps(event, sort_keys=True) + "\n", encoding="utf-8")
    (artifact_dir / evidence_ref).write_text(json.dumps(evidence, sort_keys=True), encoding="utf-8")
    return events_ref, evidence_ref
