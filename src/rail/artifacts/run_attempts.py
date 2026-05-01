from __future__ import annotations

import re
from pathlib import Path

_ATTEMPT_PATTERN = re.compile(r"^attempt-(?P<number>\d{4})$")


def allocate_run_attempt(artifact_dir: Path) -> str:
    runs_dir = artifact_dir / "runs"
    runs_dir.mkdir(exist_ok=True)

    highest = 0
    for entry in runs_dir.iterdir():
        match = _ATTEMPT_PATTERN.match(entry.name)
        if match is None:
            continue
        if entry.is_symlink() or not entry.is_dir():
            raise ValueError("unsafe run attempt entry")
        highest = max(highest, int(match.group("number")))

    attempt_ref = f"attempt-{highest + 1:04d}"
    attempt_dir = runs_dir / attempt_ref
    try:
        attempt_dir.mkdir()
    except FileExistsError as exc:
        raise ValueError("unsafe run attempt entry") from exc
    return attempt_ref
