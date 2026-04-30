#!/usr/bin/env python3
from __future__ import annotations

import shutil
import subprocess
import sys
import tempfile
from pathlib import Path


def main(argv: list[str]) -> int:
    dist_dir = Path(argv[1]) if len(argv) > 1 else Path("dist")
    wheel = _single_wheel(dist_dir)
    if wheel is None:
        print("Expected exactly one wheel in dist.", file=sys.stderr)
        return 1
    if shutil.which("uv") is None:
        print("uv is required for installed wheel smoke.", file=sys.stderr)
        return 1

    with tempfile.TemporaryDirectory(prefix="rail-wheel-smoke-") as temp:
        temp_dir = Path(temp)
        venv = temp_dir / ".venv"
        subprocess.run(["uv", "venv", "--python", "3.12", str(venv)], check=True)
        python = venv / "bin" / "python"
        rail = venv / "bin" / "rail"
        rail_sdk = venv / "bin" / "rail-sdk"
        subprocess.run(["uv", "pip", "install", "--python", str(python), str(wheel)], check=True)
        subprocess.run([str(python), "-c", _SMOKE_CODE], cwd=temp_dir, check=True)
        subprocess.run([str(rail), "--version"], cwd=temp_dir, check=True)
        subprocess.run([str(rail_sdk), "--version"], cwd=temp_dir, check=True)

    print("Installed wheel smoke passed.")
    return 0


def _single_wheel(dist_dir: Path) -> Path | None:
    matches = sorted(dist_dir.glob("*.whl"))
    if len(matches) != 1:
        return None
    return matches[0].resolve(strict=True)


_SMOKE_CODE = r"""
from pathlib import Path

import rail
from rail.actor_runtime.prompts import load_actor_catalog
from rail.policy import load_effective_policy

target = Path("target")
target.mkdir()

request = rail.normalize_request(
    {
        "project_root": str(target.resolve()),
        "task_type": "bug_fix",
        "goal": "Smoke installed wheel resources.",
        "definition_of_done": ["Installed resources load."],
    }
)
policy = load_effective_policy(target)
catalog = load_actor_catalog(target)

assert request.goal == "Smoke installed wheel resources."
assert policy.runtime.provider == "openai_agents_sdk"
assert catalog["planner"].prompt
assert catalog["planner"].schema_source["type"] == "object"
"""


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
