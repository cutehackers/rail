from __future__ import annotations

from copy import deepcopy
from pathlib import Path
from typing import Any

import pytest
import yaml
from jsonschema import Draft202012Validator

from rail.actor_runtime.output_schema import compile_codex_output_schema


def test_compile_codex_output_schema_stricts_objects_and_nested_items():
    source: dict[str, Any] = {
        "type": "object",
        "required": ["name"],
        "properties": {
            "name": {"type": "string"},
            "items": {
                "type": "array",
                "items": {
                    "type": "object",
                    "required": ["id"],
                    "properties": {
                        "id": {"type": "string"},
                        "label": {"type": "string"},
                    },
                },
            },
        },
    }

    compiled = compile_codex_output_schema(source)

    assert compiled["additionalProperties"] is False
    assert compiled["required"] == ["items", "name"]
    assert compiled["properties"]["items"]["type"] == ["array", "null"]
    item_schema = compiled["properties"]["items"]["items"]
    assert item_schema["additionalProperties"] is False
    assert item_schema["required"] == ["id", "label"]
    assert item_schema["properties"]["label"]["type"] == ["string", "null"]
    assert_codex_strict_schema(compiled)


def test_compile_codex_output_schema_does_not_mutate_input():
    source: dict[str, Any] = {
        "type": "object",
        "required": ["name"],
        "properties": {
            "name": {"type": "string"},
            "notes": {"type": "array", "items": {"type": "string"}},
        },
    }
    original = deepcopy(source)

    compile_codex_output_schema(source)

    assert source == original


def test_strict_schema_helper_rejects_loose_objects_and_bad_required_fields():
    with pytest.raises(AssertionError, match="additionalProperties"):
        assert_codex_strict_schema({"type": "object", "required": ["name"], "properties": {"name": {"type": "string"}}})

    with pytest.raises(AssertionError, match="unknown required"):
        assert_codex_strict_schema(
            {
                "type": "object",
                "additionalProperties": False,
                "required": ["missing"],
                "properties": {"name": {"type": "string"}},
            }
        )

    assert_codex_strict_schema(
        {
            "type": "object",
            "additionalProperties": False,
            "required": ["name", "notes"],
            "properties": {
                "name": {"type": "string"},
                "notes": {"type": ["array", "null"], "items": {"type": "string"}},
            },
        }
    )


def test_real_plan_schema_compiles_for_codex():
    compiled = compile_codex_output_schema(_load_schema("plan.schema.yaml"))

    assert compiled["additionalProperties"] is False
    assert compiled["required"] == sorted(compiled["properties"])
    assert "assumptions" in compiled["required"]
    assert_codex_strict_schema(compiled)
    Draft202012Validator.check_schema(compiled)


def test_real_implementation_schema_preserves_patch_bundle_alternatives():
    compiled = compile_codex_output_schema(_load_schema("implementation_result.schema.yaml"))

    assert compiled["additionalProperties"] is False
    assert "oneOf" not in compiled
    assert "patch_bundle_ref" in compiled["properties"]
    assert "patch_bundle" in compiled["properties"]
    operation_schema = compiled["properties"]["patch_bundle"]["properties"]["operations"]["items"]
    assert operation_schema["properties"]["binary"]["type"] == "boolean"
    assert operation_schema["properties"]["executable"]["type"] == "boolean"
    assert_codex_strict_schema(compiled)
    validator = Draft202012Validator(compiled)
    for output in [
        _implementation_output(patch_bundle_ref="patches/generator.patch.yaml", patch_bundle=None),
        _implementation_output(
            patch_bundle_ref=None,
            patch_bundle={
                "schema_version": "1",
                "base_tree_digest": "sha256:abc",
                "operations": [
                    {
                        "op": "write",
                        "path": "app.txt",
                        "content": "new\n",
                        "binary": False,
                        "executable": False,
                    }
                ],
            },
        ),
        _implementation_output(patch_bundle_ref=None, patch_bundle=None),
    ]:
        validator.validate(output)


def test_real_execution_report_schema_uses_minimal_actor_contract():
    compiled = compile_codex_output_schema(_load_schema("execution_report.schema.yaml"))

    assert compiled["additionalProperties"] is False
    assert "allOf" not in compiled
    assert compiled["required"] == sorted(compiled["properties"])
    assert set(compiled["properties"]) == {"format", "analyze", "tests", "failure_details", "logs"}
    assert_codex_strict_schema(compiled)
    Draft202012Validator.check_schema(compiled)


def test_compiled_actor_schemas_omit_codex_unsupported_combiners():
    unsupported_keys = {"oneOf", "anyOf", "allOf", "not", "if", "then", "else"}

    for schema_name in (
        "plan.schema.yaml",
        "context_pack.schema.yaml",
        "critic_report.schema.yaml",
        "implementation_result.schema.yaml",
        "execution_report.schema.yaml",
        "evaluation_result.schema.yaml",
    ):
        compiled = compile_codex_output_schema(_load_schema(schema_name))
        assert not _contains_any_key(compiled, unsupported_keys), schema_name


def assert_codex_strict_schema(schema: Any) -> None:
    def walk(node: Any, path: str) -> None:
        if isinstance(node, list):
            for index, item in enumerate(node):
                walk(item, f"{path}[{index}]")
            return
        if not isinstance(node, dict):
            return
        if _has_object_shape(node):
            additional = node.get("additionalProperties")
            assert additional is False or isinstance(additional, dict), f"{path}: additionalProperties must be false"
            properties = node.get("properties")
            if isinstance(properties, dict):
                required = node.get("required")
                assert isinstance(required, list), f"{path}: required must list every property"
                property_names = set(properties)
                required_names = set(required)
                assert required_names <= property_names, f"{path}: unknown required fields {sorted(required_names - property_names)}"
                assert property_names <= required_names, f"{path}: missing required fields {sorted(property_names - required_names)}"
        for key in ("properties",):
            value = node.get(key)
            if isinstance(value, dict):
                for property_name, property_schema in value.items():
                    walk(property_schema, f"{path}.{property_name}")
        for key in ("items", "additionalProperties", "not", "if", "then", "else"):
            walk(node.get(key), f"{path}.{key}")
        for key in ("oneOf", "anyOf", "allOf"):
            walk(node.get(key), f"{path}.{key}")

    walk(schema, "$")


def _load_schema(name: str) -> dict[str, Any]:
    payload = yaml.safe_load((Path("assets/defaults/templates") / name).read_text(encoding="utf-8"))
    assert isinstance(payload, dict)
    return payload


def _implementation_output(*, patch_bundle_ref: str | None, patch_bundle: dict[str, Any] | None) -> dict[str, Any]:
    return {
        "changed_files": [],
        "patch_summary": [],
        "tests_added_or_updated": [],
        "known_limits": [],
        "patch_bundle_ref": patch_bundle_ref,
        "patch_bundle": patch_bundle,
    }


def _has_object_shape(schema: dict[str, Any]) -> bool:
    schema_type = schema.get("type")
    if schema_type == "object" or (isinstance(schema_type, list) and "object" in schema_type):
        return True
    return "additionalProperties" in schema


def _allows_null(schema: dict[str, Any]) -> bool:
    schema_type = schema.get("type")
    if schema_type == "null":
        return True
    if isinstance(schema_type, list) and "null" in schema_type:
        return True
    return any(_allows_null(option) for option in schema.get("anyOf", []) if isinstance(option, dict))


def _contains_any_key(value: Any, keys: set[str]) -> bool:
    if isinstance(value, dict):
        return any(key in keys or _contains_any_key(item, keys) for key, item in value.items())
    if isinstance(value, list):
        return any(_contains_any_key(item, keys) for item in value)
    return False
