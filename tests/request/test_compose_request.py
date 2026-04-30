from __future__ import annotations

import json
from pathlib import Path

import pytest

import rail


def test_specify_trims_and_defaults_canonical_fields():
    request = rail.specify(
        {
            "request_version": " ",
            "project_root": " /absolute/path/to/target-repo ",
            "task_type": " safe_refactor ",
            "goal": "  Split the harness API into focused modules  ",
            "context": {
                "feature": " actor runtime ",
                "suspected_files": [" src/rail/api.py ", ""],
                "related_files": [" docs/superpowers/plans/2026-04-29-python-actor-runtime-rail-redesign.md "],
                "validation_roots": [" src ", " tests "],
                "validation_targets": [" tests/test_api_smoke.py "],
            },
            "constraints": [" preserve skill-first UX ", " "],
            "definition_of_done": [" public API stays importable ", " tests pass "],
        }
    )

    assert request.request_version == "1"
    assert request.project_root == "/absolute/path/to/target-repo"
    assert request.task_type == "safe_refactor"
    assert request.goal == "Split the harness API into focused modules"
    assert request.context.feature == "actor runtime"
    assert request.context.suspected_files == ["src/rail/api.py"]
    assert request.context.related_files == [
        "docs/superpowers/plans/2026-04-29-python-actor-runtime-rail-redesign.md"
    ]
    assert request.context.validation_roots == ["src", "tests"]
    assert request.context.validation_targets == ["tests/test_api_smoke.py"]
    assert request.constraints == ["preserve skill-first UX"]
    assert request.definition_of_done == ["public API stays importable", "tests pass"]
    assert request.priority == "medium"
    assert request.risk_tolerance == "medium"
    assert request.validation_profile == "standard"


def test_specify_accepts_skill_feature_addition_fixture():
    draft = json.loads(Path("tests/fixtures/skill_request_drafts/feature_addition.json").read_text(encoding="utf-8"))

    request = rail.specify(draft)

    assert request.request_version == "1"
    assert request.project_root == "/absolute/path/to/target-repo"
    assert request.task_type == "feature_addition"
    assert request.context.feature == "account_settings"
    assert request.context.validation_roots == ["lib", "test"]
    assert request.validation_profile == "standard"
    assert request.risk_tolerance == "low"
    assert request.model_dump(by_alias=True)["request_version"] == "1"


@pytest.mark.parametrize(
    ("field", "draft"),
    [
        ("project_root", {"task_type": "bug_fix", "goal": "Fix the bug"}),
        ("goal", {"project_root": "/absolute/path/to/target-repo", "task_type": "bug_fix"}),
    ],
)
def test_specify_requires_project_root_and_goal(field, draft):
    with pytest.raises(ValueError, match=field):
        rail.specify(draft)


def test_specify_rejects_unsupported_task_type():
    with pytest.raises(ValueError, match="task_type"):
        rail.specify(
            {
                "project_root": "/absolute/path/to/target-repo",
                "task_type": "review_only",
                "goal": "Review the change",
            }
        )
