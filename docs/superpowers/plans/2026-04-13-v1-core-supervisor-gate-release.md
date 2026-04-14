# V1 Core Supervisor Gate Release Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Release `rail` as a production-credible `v1` product for the bounded core supervisor gate, while refactoring the runtime so `bin/rail.dart` becomes a thin entrypoint over maintainable modules.

**Architecture:** First lock the `v1` release boundary and remove `v2` leakage from the core path. Then add automated verification for the `v1` runtime, fix live release blockers, and split the current monolithic runtime into focused modules under `lib/src/` without changing the supported `v1` behavior.

**Tech Stack:** Dart CLI runtime, YAML contracts and schemas, `package:test`, Markdown docs, GitHub Actions CI

---

### Task 1: Freeze the V1/V2 Product Boundary

**Files:**
- Modify: [README.md](/absolute/path/to/rail/README.md)
- Modify: [docs/superpowers/specs/2026-04-13-v1-core-supervisor-gate-release-design.md](/absolute/path/to/rail/docs/superpowers/specs/2026-04-13-v1-core-supervisor-gate-release-design.md)
- Delete or archive: [docs/tasks.md](/absolute/path/to/rail/docs/tasks.md)
- Create: [docs/releases/v1-core-supervisor-gate.md](/absolute/path/to/rail/docs/releases/v1-core-supervisor-gate.md)
- Create: [docs/backlog/v1-core-supervisor-gate.md](/absolute/path/to/rail/docs/backlog/v1-core-supervisor-gate.md)
- Create: [docs/backlog/v2-integrator-and-learning.md](/absolute/path/to/rail/docs/backlog/v2-integrator-and-learning.md)
- Create: [docs/archive/launch-history.md](/absolute/path/to/rail/docs/archive/launch-history.md)

- [ ] **Step 1: Write the failing scope test in docs form**

Record the exact supported `v1` command set and explicitly mark the following as deferred:

```text
Deferred to v2:
- integrator
- apply-user-outcome-feedback
- apply-learning-review
- apply-hardening-review
- approved-memory / review-queue / hardening-queue operations
```

- [ ] **Step 2: Verify current docs fail the boundary**

Run:

```bash
rg -n "apply-user-outcome-feedback|apply-learning-review|apply-hardening-review|integrator|quality learning" README.md docs
```

Expected:
- matches show `v2` behavior is still mixed into current top-level docs

- [ ] **Step 3: Rewrite the release docs for the new boundary**

Update and create docs so:
- `README.md` leads with `v1` supported scope
- `docs/releases/v1-core-supervisor-gate.md` becomes the operator-facing release contract
- `docs/backlog/v1-core-supervisor-gate.md` contains only open `v1` work
- `docs/backlog/v2-integrator-and-learning.md` captures all deferred work

- [ ] **Step 4: Archive the old task history**

Move the completed launch-history content from `docs/tasks.md` into `docs/archive/launch-history.md`, then remove or replace `docs/tasks.md` with a short redirect note.

- [ ] **Step 5: Verify the docs now express the right boundary**

Run:

```bash
rg -n "Supported in v1|Deferred to v2|core supervisor gate|bounded corrective loop" README.md docs/releases docs/backlog docs/archive
```

Expected:
- `v1` and `v2` scope are clearly separated
- no operator-facing document implies that learning or integrator behavior is part of `v1`

- [ ] **Step 6: Commit**

```bash
git add README.md docs/releases/v1-core-supervisor-gate.md docs/backlog/v1-core-supervisor-gate.md docs/backlog/v2-integrator-and-learning.md docs/archive/launch-history.md docs/tasks.md docs/superpowers/specs/2026-04-13-v1-core-supervisor-gate-release-design.md
git commit -m "docs: split v1 release scope from deferred v2 work"
```

### Task 2: Add the Verification Foundation for a Real Release Gate

**Files:**
- Modify: [pubspec.yaml](/absolute/path/to/rail/pubspec.yaml)
- Create: [analysis_options.yaml](/absolute/path/to/rail/analysis_options.yaml)
- Create: [test/request/request_validation_test.dart](/absolute/path/to/rail/test/request/request_validation_test.dart)
- Create: [test/runtime/route_evaluation_test.dart](/absolute/path/to/rail/test/runtime/route_evaluation_test.dart)
- Create: [test/reporting/terminal_summary_test.dart](/absolute/path/to/rail/test/reporting/terminal_summary_test.dart)
- Create: [test/fixtures/](/absolute/path/to/rail/test/fixtures)

- [ ] **Step 1: Add the first failing tests**

Write tests that prove the release gate is not yet protected:

```dart
test('validation request fixture remains valid', () async { ... });
test('route evaluation maps validation evidence to tighten_validation', () async { ... });
test('terminal summary explains blocked_environment state', () async { ... });
```

- [ ] **Step 2: Run the tests to confirm the missing test foundation**

Run:

```bash
dart test
```

Expected:
- fails because test infrastructure and imports are not yet present

- [ ] **Step 3: Add the minimal test infrastructure**

Update `pubspec.yaml` with:

```yaml
dev_dependencies:
  test: ^1.25.0
  lints: ^5.0.0
```

Create `analysis_options.yaml` with:

```yaml
include: package:lints/recommended.yaml
```

- [ ] **Step 4: Add the smallest helpers needed for tests**

Create fixture helpers and importable code seams only where needed to make the first tests compile.

- [ ] **Step 5: Re-run tests and analyze**

Run:

```bash
dart pub get
dart test
dart analyze
```

Expected:
- tests still fail, but now fail on runtime behavior instead of missing infrastructure
- analyzer reports existing runtime issues in a stable way

- [ ] **Step 6: Commit**

```bash
git add pubspec.yaml analysis_options.yaml test
git commit -m "test: add v1 release verification foundation"
```

### Task 3: Remove V2 Contract Leakage from the V1 Execution Path

**Files:**
- Modify: [bin/rail.dart](/absolute/path/to/rail/bin/rail.dart)
- Modify: [.harness/templates/execution_report.schema.yaml](/absolute/path/to/rail/.harness/templates/execution_report.schema.yaml)
- Modify: [.harness/templates/context_pack.schema.yaml](/absolute/path/to/rail/.harness/templates/context_pack.schema.yaml)
- Modify: [.harness/supervisor/registry.yaml](/absolute/path/to/rail/.harness/supervisor/registry.yaml)
- Modify: [.harness/supervisor/context_contract.yaml](/absolute/path/to/rail/.harness/supervisor/context_contract.yaml)
- Test: [test/runtime/v1_execution_contract_test.dart](/absolute/path/to/rail/test/runtime/v1_execution_contract_test.dart)

- [ ] **Step 1: Write the failing blocker test**

Add a characterization test for the current bug:

```dart
test('smoke execute produces a schema-valid v1 execution report', () async { ... });
```

Assert that `run -> execute` on the smoke request succeeds without any `approved_memory_consideration` requirement.

- [ ] **Step 2: Reproduce the failure**

Run:

```bash
dart test test/runtime/v1_execution_contract_test.dart
```

Expected:
- FAIL because the current `v1` smoke path still depends on a `v2`-only execution report field

- [ ] **Step 3: Make the v1 contract explicit**

Choose one contract and apply it consistently:
- either make `approved_memory_consideration` optional for `v1`
- or move learning-only fields behind a `v2` schema/normalization path

The recommended approach is:
- keep `execution_report` minimal and `v1`-safe
- move learning-only additions behind `lib/src/v2/` and non-`v1` code paths

- [ ] **Step 4: Remove or gate v2 routing from v1 execution**

Ensure that:
- `run`
- `execute`
- `route-evaluation`

do not require `integrator` or learning-specific data for a `v1` run.

- [ ] **Step 5: Re-run the focused tests**

Run:

```bash
dart test test/runtime/v1_execution_contract_test.dart
dart run bin/rail.dart run --request .harness/requests/rail-bootstrap-smoke.yaml --project-root /absolute/path/to/rail --task-id v1-contract-smoke
dart run bin/rail.dart execute --artifact .harness/artifacts/v1-contract-smoke
```

Expected:
- tests pass
- smoke run completes without schema drift

- [ ] **Step 6: Commit**

```bash
git add bin/rail.dart .harness/templates/execution_report.schema.yaml .harness/templates/context_pack.schema.yaml .harness/supervisor/registry.yaml .harness/supervisor/context_contract.yaml test/runtime/v1_execution_contract_test.dart
git commit -m "fix: isolate v1 execution contracts from deferred v2 features"
```

### Task 4: Refactor `bin/rail.dart` into Maintainable V1 Modules

**Files:**
- Modify: [bin/rail.dart](/absolute/path/to/rail/bin/rail.dart)
- Create: [lib/src/cli/rail_cli.dart](/absolute/path/to/rail/lib/src/cli/rail_cli.dart)
- Create: [lib/src/request/request_service.dart](/absolute/path/to/rail/lib/src/request/request_service.dart)
- Create: [lib/src/bootstrap/bootstrap_service.dart](/absolute/path/to/rail/lib/src/bootstrap/bootstrap_service.dart)
- Create: [lib/src/runtime/runtime_service.dart](/absolute/path/to/rail/lib/src/runtime/runtime_service.dart)
- Create: [lib/src/contracts/schema_validator.dart](/absolute/path/to/rail/lib/src/contracts/schema_validator.dart)
- Create: [lib/src/contracts/harness_contracts.dart](/absolute/path/to/rail/lib/src/contracts/harness_contracts.dart)
- Create: [lib/src/reporting/reporting_service.dart](/absolute/path/to/rail/lib/src/reporting/reporting_service.dart)
- Create: [lib/src/process/process_runner.dart](/absolute/path/to/rail/lib/src/process/process_runner.dart)
- Create: [lib/src/v2/learning_service.dart](/absolute/path/to/rail/lib/src/v2/learning_service.dart)
- Create: [lib/src/v2/review_commands.dart](/absolute/path/to/rail/lib/src/v2/review_commands.dart)
- Test: [test/cli/cli_dispatch_test.dart](/absolute/path/to/rail/test/cli/cli_dispatch_test.dart)
- Test: [test/runtime/runtime_service_test.dart](/absolute/path/to/rail/test/runtime/runtime_service_test.dart)

- [ ] **Step 1: Write the failing dispatch test**

Add a test proving `bin/rail.dart` can delegate to a CLI application object:

```dart
test('main delegates command handling to RailCli', () async { ... });
```

- [ ] **Step 2: Run the focused test to confirm the current monolith**

Run:

```bash
dart test test/cli/cli_dispatch_test.dart
```

Expected:
- FAIL because dispatch is still embedded in `bin/rail.dart`

- [ ] **Step 3: Extract the first thin-entrypoint seam**

Move:
- command parsing
- command routing
- top-level usage text

into `lib/src/cli/rail_cli.dart`

Leave `bin/rail.dart` with only:

```dart
Future<void> main(List<String> args) => RailCli().run(args);
```

- [ ] **Step 4: Extract contracts, process, and reporting helpers**

Move focused responsibilities into:
- `schema_validator.dart`
- `harness_contracts.dart`
- `process_runner.dart`
- `reporting_service.dart`

Keep each file narrowly scoped.

- [ ] **Step 5: Extract request, bootstrap, and runtime services**

Move:
- request composition/validation
- workflow bootstrap
- execution loop and routing

into dedicated services, then replace direct calls in `bin/rail.dart` with service wiring.

- [ ] **Step 6: Move deferred features behind `lib/src/v2/`**

Relocate:
- `apply-user-outcome-feedback`
- `apply-learning-review`
- `apply-hardening-review`
- learning synchronization helpers

into explicit `v2` modules so the `v1` path is isolated in imports and ownership.

- [ ] **Step 7: Re-run tests and analyzer after each extraction batch**

Run:

```bash
dart test test/cli/cli_dispatch_test.dart test/runtime/runtime_service_test.dart
dart analyze
```

Expected:
- tests pass after each extraction
- warnings trend toward zero as dead code and null-safety drift are removed

- [ ] **Step 8: Commit**

```bash
git add bin/rail.dart lib test
git commit -m "refactor: split monolithic runtime into focused v1 modules"
```

### Task 5: Refresh Smoke and Standard Evidence Under the New V1 Gate

**Files:**
- Modify: [.harness/requests/rail-bootstrap-smoke.yaml](/absolute/path/to/rail/.harness/requests/rail-bootstrap-smoke.yaml)
- Modify: [.harness/requests/rail-standard-beacon-validation.yaml](/absolute/path/to/rail/.harness/requests/rail-standard-beacon-validation.yaml)
- Modify or remove stale artifacts under: [.harness/artifacts/](/absolute/path/to/rail/.harness/artifacts)
- Create: [test/integration/v1_smoke_release_test.dart](/absolute/path/to/rail/test/integration/v1_smoke_release_test.dart)
- Create: [test/integration/v1_standard_route_test.dart](/absolute/path/to/rail/test/integration/v1_standard_route_test.dart)

- [ ] **Step 1: Write failing integration tests for fresh verification**

Create tests that shell out to the CLI and assert:

```dart
test('fresh smoke run completes under the v1 gate', () async { ... });
test('standard validation route stays deterministic under the v1 gate', () async { ... });
```

- [ ] **Step 2: Run the integration tests and capture current failures**

Run:

```bash
dart test test/integration/v1_smoke_release_test.dart test/integration/v1_standard_route_test.dart
```

Expected:
- FAIL until stale artifacts and schema assumptions are repaired

- [ ] **Step 3: Remove or refresh stale checked-in evidence**

For every artifact that claims to represent `v1` release evidence:
- validate it against current schema
- refresh it if it is still needed
- delete it if it only preserves stale launch-history claims

- [ ] **Step 4: Rebuild the minimal v1 evidence set**

Keep only the smallest reproducible set:
- one smoke proof
- one representative standard-route proof per supported action family
- terminal artifact examples that are generated by the current runtime

- [ ] **Step 5: Re-run the integration suite**

Run:

```bash
dart test test/integration/v1_smoke_release_test.dart test/integration/v1_standard_route_test.dart
```

Expected:
- PASS with fresh artifacts and current runtime behavior

- [ ] **Step 6: Commit**

```bash
git add .harness/requests .harness/artifacts test/integration
git commit -m "test: refresh v1 smoke and standard release evidence"
```

### Task 6: Add Production Release Operations and CI

**Files:**
- Create: [.github/workflows/v1-release-gate.yml](/absolute/path/to/rail/.github/workflows/v1-release-gate.yml)
- Modify: [README.md](/absolute/path/to/rail/README.md)
- Modify: [docs/releases/v1-core-supervisor-gate.md](/absolute/path/to/rail/docs/releases/v1-core-supervisor-gate.md)
- Modify: [docs/backlog/v1-core-supervisor-gate.md](/absolute/path/to/rail/docs/backlog/v1-core-supervisor-gate.md)
- Delete: [bin/rail.dart.rej](/absolute/path/to/rail/bin/rail.dart.rej)

- [ ] **Step 1: Write the failing release checklist test**

Create a checklist in `docs/releases/v1-core-supervisor-gate.md` that requires:
- `dart analyze`
- `dart test`
- smoke integration
- standard route verification
- `dart compile exe bin/rail.dart`

Treat missing CI as a failing release condition.

- [ ] **Step 2: Reproduce the missing release automation**

Run:

```bash
test -f .github/workflows/v1-release-gate.yml
```

Expected:
- exit code 1 because the workflow does not yet exist

- [ ] **Step 3: Add the release workflow**

Create `.github/workflows/v1-release-gate.yml` to run:

```bash
dart pub get
dart analyze
dart test
dart compile exe bin/rail.dart -o build/rail
```

and, if integration tests are separated by tags or path, run those explicitly.

- [ ] **Step 4: Clean repository hygiene blockers**

Delete `bin/rail.dart.rej` and ensure no release docs reference stale files as proof of present correctness.

- [ ] **Step 5: Run the full local release gate**

Run:

```bash
dart pub get
dart analyze
dart test
dart compile exe bin/rail.dart -o build/rail
```

Expected:
- all commands pass locally
- the release docs exactly match the executed commands

- [ ] **Step 6: Record closure**

Update `docs/backlog/v1-core-supervisor-gate.md` so each completed release blocker is closed with fresh evidence and move any remainder to `docs/backlog/v2-integrator-and-learning.md` only if it is truly out of `v1` scope.

- [ ] **Step 7: Commit**

```bash
git add .github/workflows/v1-release-gate.yml README.md docs/releases/v1-core-supervisor-gate.md docs/backlog/v1-core-supervisor-gate.md docs/backlog/v2-integrator-and-learning.md bin/rail.dart.rej
git commit -m "build: add v1 production release gate"
```
