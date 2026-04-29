from __future__ import annotations

import hashlib
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import yaml
from pydantic import BaseModel

from rail.actor_runtime.schemas import ACTOR_OUTPUT_MODELS
from rail.resources import load_default_asset_yaml, read_default_asset_text

SUPERVISOR_ACTORS = ("planner", "context_builder", "critic", "generator", "executor", "evaluator")

_ACTOR_SCHEMA_FILES = {
    "planner": "plan.schema.yaml",
    "context_builder": "context_pack.schema.yaml",
    "critic": "critic_report.schema.yaml",
    "generator": "implementation_result.schema.yaml",
    "executor": "execution_report.schema.yaml",
    "evaluator": "evaluation_result.schema.yaml",
}


@dataclass(frozen=True)
class ActorCatalogEntry:
    actor: str
    prompt_path: Path
    prompt: str
    prompt_digest: str
    schema_path: Path
    schema_source: dict[str, Any]
    output_model: type[BaseModel]

    def validate_output(self, output: object) -> BaseModel:
        return self.output_model.model_validate(output)


def load_actor_catalog(project_root: Path) -> dict[str, ActorCatalogEntry]:
    catalog: dict[str, ActorCatalogEntry] = {}
    for actor in SUPERVISOR_ACTORS:
        prompt_path = project_root / ".harness" / "actors" / f"{actor}.md"
        prompt_ref = Path(".harness") / "actors" / f"{actor}.md"
        if prompt_path.is_file():
            prompt = prompt_path.read_text(encoding="utf-8")
        else:
            prompt_ref = Path("package_assets") / "defaults" / "actors" / f"{actor}.md"
            prompt = read_default_asset_text(f"defaults/actors/{actor}.md")
        schema_path = project_root / ".harness" / "templates" / _ACTOR_SCHEMA_FILES[actor]
        schema_ref = Path(".harness") / "templates" / _ACTOR_SCHEMA_FILES[actor]
        if schema_path.is_file():
            schema_source = yaml.safe_load(schema_path.read_text(encoding="utf-8")) or {}
        else:
            schema_ref = Path("package_assets") / "defaults" / "templates" / _ACTOR_SCHEMA_FILES[actor]
            schema_source = load_default_asset_yaml(f"defaults/templates/{_ACTOR_SCHEMA_FILES[actor]}")
        catalog[actor] = ActorCatalogEntry(
            actor=actor,
            prompt_path=prompt_ref,
            prompt=prompt,
            prompt_digest=_digest_text(prompt),
            schema_path=schema_ref,
            schema_source=schema_source,
            output_model=ACTOR_OUTPUT_MODELS[actor],
        )
    return catalog


def _digest_text(text: str) -> str:
    return "sha256:" + hashlib.sha256(text.encode("utf-8")).hexdigest()
