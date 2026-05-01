# Rail Python Coding Conventions

This document defines the coding conventions for the Python-first Rail Harness
Runtime. Agents changing production code, tests, bundled assets, or workflow
docs should read this together with `docs/ARCHITECTURE.md` before editing.

## Baseline Style

- Follow PEP 8 for Python style and naming.
- Use Python 3.12 language features where they simplify code without reducing
  readability.
- Keep modules small and named after the product behavior they own.
- Prefer explicit, typed data contracts over implicit dictionaries.
- Use Pydantic models for request, policy, artifact, evidence, and projection
  contracts that cross module boundaries.
- Keep side effects at Rail-owned boundaries: artifact store, supervisor,
  workspace apply, validation runner, and Actor Runtime providers.
- Do not introduce broad compatibility shims for removed runtime surfaces.

## Formatting And Static Checks

Run these before claiming code is complete:

```bash
uv run --python 3.12 pytest -q
uv run --python 3.12 ruff check src tests
uv run --python 3.12 mypy src/rail
```

For release readiness, run:

```bash
scripts/release_gate.sh
```

## Python Naming Conventions

This section adapts Dart-style naming rules to Python's standard PEP 8
conventions.

### Files And Modules

Python module names must be `snake_case` and should clearly describe the
module's responsibility. Avoid vague names such as `utils.py`.

Wrong:

```text
utils.py
helpers.py
common.py
base.py
```

Correct:

```text
email_validator.py
date_formatter.py
network_error_handler.py
auth_token_registry.py
```

Forbidden ambiguity: avoid names containing `utils`, `helpers`, `Util`,
`Helper`, or `Manager` unless the term is part of a precise external protocol
or an existing public API that cannot be changed.

Rail examples:

```text
validation_runner.py
validation_policy.py
terminal_summary.py
patch_bundle.py
```

### Classes

Class names must use `PascalCase` and describe a concrete product role.

Correct:

```python
class EmailValidator:
    """Validate email address format."""


class DateFormatter:
    """Format date values for a specific output contract."""


class NetworkErrorHandler:
    """Map network failures to product-level error states."""
```

Wrong:

```python
class Util:
    pass


class Helper:
    pass


class Manager:
    pass
```

Rail examples:

```python
class ArtifactHandle:
    pass


class ValidationEvidence:
    pass


class TerminalSummaryProjection:
    pass
```

### Functions And Variables

- Use `snake_case` for functions, methods, variables, and module constants.
- Use verb phrases for functions that perform work, such as
  `load_policy_validation_commands`.
- Use noun phrases for values and models, such as `artifact_dir` or
  `request_digest`.
- Prefer explicit names over short names when the value crosses a boundary.

Correct:

```python
def load_policy_validation_commands(target_root: Path) -> list[ValidationCommand]:
    ...


request_snapshot_digest = digest_request(request)
```

Wrong:

```python
def handle_it(path):
    ...


data = digest_request(request)
```

### Constants

- Use `UPPER_SNAKE_CASE` for module constants.
- Keep constants close to the module that owns the decision.
- Do not hide policy decisions in generic constants.

Correct:

```python
_VALIDATION_ENV_ALLOWLIST = {"PATH", "HOME", "TMPDIR"}
```

## Module Boundaries

Respect the architecture boundaries:

- `src/rail/request/`: request normalization and draft schema.
- `src/rail/artifacts/`: artifact identity, handle loading, and projections.
- `src/rail/actor_runtime/`: Actor Runtime providers and actor contracts.
- `src/rail/supervisor/`: deterministic workflow orchestration.
- `src/rail/evaluator/`: evaluator gate and terminal pass rules.
- `src/rail/workspace/`: patch bundle validation, target mutation, and
  validation evidence.
- `src/rail/auth/`: credential source validation and redaction.

Do not create a shared catch-all module. If a function seems generally useful,
name the module after the concrete Rail behavior it supports.

## Data Contracts

- Use Pydantic models for persisted or cross-boundary data.
- Set `ConfigDict(extra="forbid")` on contract models.
- Prefer `Literal[...]` for finite state values such as outcomes and sources.
- Keep digest fields explicit when they are part of an evidence chain.
- Runtime evidence refs must be attempt-scoped under `runs/attempt-NNNN/`.
  Projections and evaluator gates should read the current attempt recorded in
  `run_status.yaml`, not every historical attempt.
- Do not allow actor output to grant itself new permissions.

## Error Handling

- Fail closed when policy, validation, artifact identity, or credentials are
  ambiguous.
- Convert SDK/runtime failures into blocked artifact evidence instead of
  letting exceptions escape the supervisor.
- Redact secret-shaped values before writing `run_status.yaml`,
  `terminal_summary.yaml`, runtime evidence, validation logs, or result
  projections.
- Do not record a terminal pass without current validation evidence and an
  accepted evaluator gate.
- Keep the local runtime provider name as `codex_vault`; use
  `openai_agents_sdk` only for the optional operator/API-key provider.
- Keep `codex_vault` compatibility fixes inside the Actor Runtime boundary.
  The Rail skill reports blocked artifacts; it does not patch runtime internals
  or auth homes during a task session.

## Validation And Mutation

- Target mutation must happen through Rail-validated patch bundles.
- Validation commands must come from request or policy-owned configuration, not
  actor output.
- Validation evidence must state the actual environment guarantees. Do not
  claim network, sandbox, or credential isolation that the runner does not
  enforce.
- Detect mutations to protected harness state when validation runs.

## Tests

- Add focused tests for behavior changes before implementation when feasible.
- Prefer real artifact, policy, and validation flows over mocks.
- Test evidence files when the behavior depends on persisted state.
- Keep fake runtime helpers outside the production package.
- For docs and release behavior, extend docs guard tests when the rule should
  remain true.

## Documentation

- Keep user-facing docs skill-first and Python API first.
- Do not instruct users to manage task ids or hand-write request YAML for normal
  operation.
- Do not include local home directory paths in docs or examples.
- Keep `skills/rail/SKILL.md` and `assets/skill/Rail/SKILL.md` byte-for-byte
  aligned when changing Rail skill behavior.
