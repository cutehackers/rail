# Rail Harness Bundle

This `.harness/` tree is the default control-plane harness bundle for target repositories. It is source data for the Python Rail Harness Runtime, not a downstream application.

Core responsibilities:

- define actor prompts and structured output contracts
- define supervisor routing and policy defaults
- define request and artifact templates
- provide rules, rubrics, and review-only learning state
- keep project-local harness state explicit and reviewable

## Philosophy

- Accept minimal user input.
- Keep task scope narrow.
- Separate actor work, patch application, validation, and evaluation.
- Prefer traceable evidence over implicit runtime state.
- Mutate target repositories only through validated patch bundles.

## Structure

- `supervisor/`: routing, registry, execution policy, contract
- `actors/`: actor instructions
- `rules/`: forbidden changes, architecture rules, project profile rules
- `rubrics/`: task-type evaluation criteria
- `templates/`: request and output schemas
- `skills/`: auxiliary skill documents

Stateful directories such as `.harness/artifacts/`, `.harness/learning/`, `.harness/requests/`, and `.harness/fixtures/` belong to the project-local harness state and are not treated as disposable generated output.

## Operating Model

Normal users interact through the Rail skill and Python API:

```python
import rail

handle = rail.start_task(draft)
rail.supervise(handle)
projection = rail.result(handle)
```

The target project root is carried by the request draft and then by the artifact handle. Existing artifact operations use that handle rather than reconstructing identity from request files.

## Runtime Path

The Python supervisor runs the actor graph:

1. planner
2. context builder
3. critic
4. generator
5. executor
6. evaluator

The Actor Runtime returns structured actor output and evidence. Generator output may include a patch bundle reference or inline patch bundle. Rail validates and applies that bundle before validation evidence is recorded. Evaluator pass is accepted only when the evidence chain matches the request digest, effective policy digest, actor invocation digest, patch bundle digest, target tree digest, validation result, and evaluator input digest.

## Source Of Truth

- `registry.yaml`: task actor and output definitions
- `task_router.yaml`: task type routing and risk policy
- `context_contract.yaml`: actor input and output contract
- `supervisor/actor_runtime.yaml`: default Actor Runtime policy

The runtime uses these files to preserve deterministic routing, policy narrowing, and evidence checks.
