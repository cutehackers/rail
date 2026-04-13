# V1 Core Supervisor Gate

## Release Contract

`rail v1` is the production release for the bounded core supervisor gate.

Supported in `v1`:

- `compose-request`
- `validate-request`
- `run --request ... --project-root ...`
- `execute --artifact ...`
- `route-evaluation`
- sequential execution of:
  - `planner`
  - `context_builder`
  - `generator`
  - `executor`
  - `evaluator`
- evaluator-driven bounded corrective loop
- deterministic supervisor actions
- terminal artifacts and release verification

Deferred to `v2`:

- `integrator`
- `apply-user-outcome-feedback`
- `apply-learning-review`
- `apply-hardening-review`
- approved-memory / review-queue / hardening-queue operations
- quality-improvement-over-time workflows

## Core Supervisor Gate

The supported `v1` runtime behavior is:

1. bootstrap a request and artifact set
2. execute `planner -> context_builder -> generator -> executor -> evaluator`
3. let `evaluator` select a deterministic next action
4. continue only within explicit retry budgets
5. stop in a visible terminal state

The bounded corrective loop is part of `v1`.

The self-learning loop is not part of `v1`.

## Operator Commands

```bash
dart pub get
dart run bin/rail.dart compose-request --goal <goal> --task-type <type>
dart run bin/rail.dart validate-request --request <request-file>
dart run bin/rail.dart run --request <request-file> --project-root <target-repo>
dart run bin/rail.dart execute --artifact <artifact-dir>
dart run bin/rail.dart route-evaluation --artifact <artifact-or-evaluation-result>
```

## Release Gate

Production release requires all of the following:

```bash
dart analyze
dart test
dart compile exe bin/rail.dart -o build/rail
```

And:

- fresh smoke verification succeeds with `run -> execute`
- representative standard route verification succeeds against current schemas
  - covered by `test/runtime/standard_route_fixtures_test.dart`
  - fixtures live under `test/fixtures/standard_route/`
- terminal artifacts remain readable without raw actor log inspection
- no deferred `v2` field or flow is required for the `v1` path

## Operational Notes

- `v1` is intentionally conservative. Weak evidence should refuse or route to a corrective action rather than pass.
- `v1` only claims the core supervisor gate. It does not claim post-pass integration or review-driven learning behavior.
