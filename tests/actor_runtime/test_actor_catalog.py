from __future__ import annotations

from pathlib import Path

from rail.actor_runtime.prompts import SUPERVISOR_ACTORS, load_actor_catalog
from tests.actor_runtime_test_fixtures import fake_actor_output


def test_every_supervisor_actor_has_prompt_and_schema_source():
    catalog = load_actor_catalog(Path("."))

    assert set(catalog) == set(SUPERVISOR_ACTORS)
    for actor, entry in catalog.items():
        assert entry.prompt_path == Path(".harness/actors") / f"{actor}.md"
        assert entry.prompt.strip()
        assert entry.schema_path.name.endswith(".schema.yaml")
        assert entry.schema_source["type"] == "object"
        assert entry.output_model is not None


def test_actor_catalog_falls_back_to_packaged_defaults(tmp_path):
    catalog = load_actor_catalog(tmp_path)

    assert set(catalog) == set(SUPERVISOR_ACTORS)
    assert catalog["planner"].prompt_path == Path("package_assets/defaults/actors/planner.md")
    assert catalog["planner"].schema_path == Path("package_assets/defaults/templates/plan.schema.yaml")
    assert catalog["planner"].prompt.strip()
    assert catalog["planner"].schema_source["type"] == "object"


def test_prompts_load_deterministically():
    first = load_actor_catalog(Path("."))
    second = load_actor_catalog(Path("."))

    assert {actor: entry.prompt_digest for actor, entry in first.items()} == {
        actor: entry.prompt_digest for actor, entry in second.items()
    }


def test_fake_actor_outputs_validate_against_catalog_models():
    catalog = load_actor_catalog(Path("."))

    for actor in SUPERVISOR_ACTORS:
        output = fake_actor_output(actor)
        validated = catalog[actor].validate_output(output)

        assert validated is not None


def test_output_contract_paths_map_to_actor_names():
    catalog = load_actor_catalog(Path("."))

    assert catalog["planner"].schema_path.name == "plan.schema.yaml"
    assert catalog["context_builder"].schema_path.name == "context_pack.schema.yaml"
    assert catalog["critic"].schema_path.name == "critic_report.schema.yaml"
    assert catalog["generator"].schema_path.name == "implementation_result.schema.yaml"
    assert catalog["executor"].schema_path.name == "execution_report.schema.yaml"
    assert catalog["evaluator"].schema_path.name == "evaluation_result.schema.yaml"
