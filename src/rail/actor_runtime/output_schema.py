from __future__ import annotations

from copy import deepcopy
from typing import Any, Mapping

_COMBINER_KEYS = ("oneOf", "anyOf", "allOf")
_UNSUPPORTED_CODEX_SCHEMA_KEYS = frozenset({"oneOf", "anyOf", "allOf", "not", "if", "then", "else"})
_SCHEMA_KEYS = ("items", "additionalProperties")


def compile_codex_output_schema(schema: Mapping[str, Any]) -> dict[str, Any]:
    """Compile Rail actor output schemas for Codex strict structured output."""

    return _compile_node(deepcopy(dict(schema)))


def _compile_node(value: Any) -> Any:
    if isinstance(value, list):
        return [_compile_node(item) for item in value]
    if not isinstance(value, dict):
        return value

    original_required = _string_set(value.get("required"))
    compiled: dict[str, Any] = {}
    for key, item in value.items():
        if key in _UNSUPPORTED_CODEX_SCHEMA_KEYS:
            continue
        if key == "properties" and isinstance(item, dict):
            compiled[key] = {str(property_name): _compile_node(property_schema) for property_name, property_schema in item.items()}
        elif key in _SCHEMA_KEYS:
            compiled[key] = _compile_node(item)
        else:
            compiled[key] = _compile_node(item)

    properties = compiled.get("properties")
    optional_properties: set[str] = set()
    if isinstance(properties, dict):
        optional_properties = set(properties) - original_required
        for property_name in sorted(optional_properties):
            property_schema = properties[property_name]
            if isinstance(property_schema, dict):
                properties[property_name] = _allow_null(property_schema)
        compiled["required"] = sorted(properties)
        for key in _COMBINER_KEYS:
            branches = compiled.get(key)
            if isinstance(branches, list):
                compiled[key] = [
                    _rewrite_presence_branch(branch, optional_properties=optional_properties, property_schemas=properties)
                    for branch in branches
                ]

    if _is_object_schema(compiled):
        additional = compiled.get("additionalProperties")
        if not isinstance(additional, dict):
            compiled["additionalProperties"] = False

    return compiled


def _rewrite_presence_branch(
    branch: Any,
    *,
    optional_properties: set[str],
    property_schemas: dict[str, Any],
) -> Any:
    if not isinstance(branch, dict) or not optional_properties:
        return branch

    rewritten = dict(branch)
    raw_branch_properties = rewritten.get("properties")
    branch_properties = dict(raw_branch_properties) if isinstance(raw_branch_properties, dict) else {}

    for property_name in _string_set(rewritten.get("required")) & optional_properties:
        property_schema = property_schemas.get(property_name)
        if isinstance(property_schema, dict):
            branch_properties[property_name] = _without_null(property_schema)

    null_properties = _presence_absent_properties(rewritten.get("not"), optional_properties=optional_properties)
    if null_properties is not None:
        rewritten.pop("not", None)
        for property_name in sorted(null_properties):
            branch_properties[property_name] = {"type": "null"}

    if branch_properties:
        rewritten["properties"] = branch_properties
    return rewritten


def _presence_absent_properties(schema: Any, *, optional_properties: set[str]) -> set[str] | None:
    if not isinstance(schema, dict):
        return None

    required = _string_set(schema.get("required"))
    if required:
        if required <= optional_properties and set(schema) == {"required"}:
            return required
        return None

    any_of = schema.get("anyOf")
    if not isinstance(any_of, list):
        return None

    absent: set[str] = set()
    for branch in any_of:
        if not isinstance(branch, dict) or set(branch) != {"required"}:
            return None
        branch_required = _string_set(branch.get("required"))
        if not branch_required or not branch_required <= optional_properties:
            return None
        absent.update(branch_required)
    return absent


def _allow_null(schema: dict[str, Any]) -> dict[str, Any]:
    if _allows_null(schema):
        return schema

    updated = deepcopy(schema)
    schema_type = updated.get("type")
    if isinstance(schema_type, str):
        updated["type"] = [schema_type, "null"]
        if isinstance(updated.get("enum"), list):
            updated["enum"] = [*updated["enum"], None]
        return updated
    if isinstance(schema_type, list):
        updated["type"] = [*schema_type, "null"]
        if isinstance(updated.get("enum"), list):
            updated["enum"] = [*updated["enum"], None]
        return updated

    return {"anyOf": [updated, {"type": "null"}]}


def _without_null(schema: dict[str, Any]) -> dict[str, Any]:
    updated = deepcopy(schema)
    schema_type = updated.get("type")
    if schema_type == "null":
        return {"not": {}}
    if isinstance(schema_type, list):
        non_null_types = [item for item in schema_type if item != "null"]
        if len(non_null_types) == 1:
            updated["type"] = non_null_types[0]
        else:
            updated["type"] = non_null_types
    if isinstance(updated.get("enum"), list):
        updated["enum"] = [item for item in updated["enum"] if item is not None]
    any_of = updated.get("anyOf")
    if isinstance(any_of, list):
        updated["anyOf"] = [branch for branch in any_of if not (isinstance(branch, dict) and branch.get("type") == "null")]
    return updated


def _allows_null(schema: dict[str, Any]) -> bool:
    schema_type = schema.get("type")
    if schema_type == "null":
        return True
    if isinstance(schema_type, list) and "null" in schema_type:
        return True
    any_of = schema.get("anyOf")
    return isinstance(any_of, list) and any(isinstance(branch, dict) and _allows_null(branch) for branch in any_of)


def _is_object_schema(schema: dict[str, Any]) -> bool:
    schema_type = schema.get("type")
    if schema_type == "object":
        return True
    if isinstance(schema_type, list) and "object" in schema_type:
        return True
    return "properties" in schema or "additionalProperties" in schema


def _string_set(value: Any) -> set[str]:
    if not isinstance(value, list):
        return set()
    return {item for item in value if isinstance(item, str)}
