from __future__ import annotations

import json
from pathlib import Path

import yaml

from rail.actor_runtime.evidence import write_runtime_evidence
from rail.actor_runtime.agents import AgentsActorRuntime, SDKRunResult
from rail.actor_runtime.runtime import ActorInvocation, ActorResult
from rail.policy import load_effective_policy
from rail.policy.schema import ActorRuntimePolicyV2
from rail.workspace.isolation import tree_digest


def fake_actor_output(actor: str) -> dict[str, object]:
    outputs: dict[str, dict[str, object]] = {
        "planner": {
            "summary": "Plan one bounded step.",
            "likely_files": ["src/rail/api.py"],
            "substeps": ["Inspect", "Implement", "Verify"],
            "risks": ["Incomplete validation"],
            "acceptance_criteria_refined": ["Tests pass"],
        },
        "context_builder": {
            "relevant_files": [{"path": "src/rail/api.py", "why": "Public API"}],
            "repo_patterns": ["Python package under src"],
            "forbidden_changes": ["Do not mutate target directly"],
        },
        "critic": {
            "priority_focus": ["Policy boundary"],
            "missing_requirements": [],
            "risk_hypotheses": [],
            "validation_expectations": ["pytest"],
            "generator_guardrails": ["patch only"],
            "blocked_assumptions": [],
        },
        "generator": {
            "changed_files": ["src/rail/api.py"],
            "patch_summary": ["Added API"],
            "tests_added_or_updated": ["tests/test_api_smoke.py"],
            "known_limits": [],
            "patch_bundle_ref": "patches/generator.patch.yaml",
        },
        "executor": {
            "format": "pass",
            "analyze": "pass",
            "tests": {"total": 1, "passed": 1, "failed": 0},
            "failure_details": [],
            "logs": ["pytest passed"],
        },
        "evaluator": {
            "decision": "pass",
            "evaluated_input_digest": "sha256:evaluator-input-not-bound",
            "findings": [],
            "reason_codes": [],
            "quality_confidence": "high",
        },
    }
    return outputs[actor]


def scripted_agents_runtime(
    target_root: Path, *, patch_path: str | None = None, patch_content: str | None = None
) -> AgentsActorRuntime:
    _ensure_validation_policy(target_root)

    def runner(agent, prompt: str, *, run_config: dict[str, object]) -> SDKRunResult:
        actor = agent.name.removeprefix("rail-")
        output = fake_actor_output(actor)
        if actor == "evaluator":
            actor_input = _actor_input_from_prompt(prompt)
            evaluator_input_digest = actor_input.get("evaluator_input_digest")
            if isinstance(evaluator_input_digest, str):
                output["evaluated_input_digest"] = evaluator_input_digest
        if actor == "generator":
            operations = []
            if patch_path is not None:
                operations.append({"op": "write", "path": patch_path, "content": patch_content or ""})
            output.pop("patch_bundle_ref", None)
            output["patch_bundle"] = {
                "schema_version": "1",
                "base_tree_digest": tree_digest(target_root),
                "operations": operations,
            }
        return SDKRunResult(final_output=output, trace_id=f"trace-{actor}")

    return AgentsActorRuntime(project_root=Path("."), policy=_sdk_policy(target_root), runner=runner)


class FakeActorRuntime:
    def run(self, invocation: ActorInvocation) -> ActorResult:
        output = fake_actor_output(invocation.actor)
        if invocation.actor == "evaluator":
            evaluator_input_digest = invocation.input.get("evaluator_input_digest")
            if isinstance(evaluator_input_digest, str):
                output["evaluated_input_digest"] = evaluator_input_digest
        patch_value = output.get("patch_bundle_ref")
        patch_ref = Path(patch_value) if isinstance(patch_value, str) and invocation.actor == "generator" else None
        if patch_ref is not None:
            _write_noop_patch_bundle(invocation, patch_ref)
        events_ref, evidence_ref = write_runtime_evidence(
            invocation.artifact_dir,
            invocation.actor,
            {
                "status": "succeeded",
                "actor": invocation.actor,
                "artifact_id": invocation.artifact_id,
                "policy_digest": invocation.policy_digest,
                "structured_output": output,
            },
        )
        return ActorResult(
            status="succeeded",
            structured_output=output,
            events_ref=events_ref,
            runtime_evidence_ref=evidence_ref,
            patch_bundle_ref=patch_ref,
        )


def _write_noop_patch_bundle(invocation: ActorInvocation, patch_ref: Path) -> None:
    if patch_ref.is_absolute() or ".." in patch_ref.parts:
        return
    request = yaml.safe_load((invocation.artifact_dir / "request.yaml").read_text(encoding="utf-8"))
    target_root = Path(str(request["project_root"]))
    patch_path = invocation.artifact_dir / patch_ref
    patch_path.parent.mkdir(parents=True, exist_ok=True)
    patch_path.write_text(
        yaml.safe_dump({"schema_version": "1", "base_tree_digest": tree_digest(target_root), "operations": []}),
        encoding="utf-8",
    )


def _actor_input_from_prompt(prompt: str) -> dict[str, object]:
    marker = "Actor input JSON:\n"
    if marker not in prompt:
        return {}
    payload = prompt.split(marker, 1)[1]
    decoded = json.loads(payload)
    return decoded if isinstance(decoded, dict) else {}


def _ensure_validation_policy(target_root: Path) -> None:
    policy = target_root / ".harness" / "supervisor" / "execution_policy.yaml"
    policy.parent.mkdir(parents=True, exist_ok=True)
    if policy.is_file():
        return
    policy.write_text(
        "version: 2\nvalidation:\n  commands:\n    - python -c \"import pathlib; assert pathlib.Path('.').exists()\"\n",
        encoding="utf-8",
    )


def _sdk_policy(target_root: Path) -> ActorRuntimePolicyV2:
    data = load_effective_policy(target_root).model_dump(mode="json")
    data["runtime"]["provider"] = "openai_agents_sdk"
    data["tools"]["shell"]["enabled"] = False
    data["tools"]["shell"]["allowlist"] = []
    return ActorRuntimePolicyV2.model_validate(data)
