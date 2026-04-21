# Critic Actor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a mandatory `critic` actor to every task-family graph, move actor model and reasoning selection into checked-in harness policy, remove environment-variable defaults and actor-level timeouts from structured actor execution, and prove critic impact in runtime reports.

**Architecture:** Extend the canonical graph to `planner -> context_builder -> critic -> generator -> executor -> evaluator`, treat `critic_report` as a required generator input, and load actor execution policy from repository-owned `actor_profiles.yaml` files under both project-local and embedded default supervisor assets. Keep `evaluator` as the only authoritative routing gate, and extend execution reporting so operators can see which actor profiles ran and how critic findings influenced terminal outcomes.

**Tech Stack:** Go CLI runtime in `cmd/rail` and `internal/runtime`, YAML harness policy under `.harness/supervisor`, embedded defaults under `assets/defaults`, artifact schemas under `.harness/templates` and `assets/defaults/templates`, Go tests in `internal/runtime`, `internal/contracts`, and `internal/cli`

---

### Task 1: Add Critic to the Checked-In Supervisor Contract

**Files:**
- Create: `.harness/actors/critic.md`
- Create: `assets/defaults/actors/critic.md`
- Create: `.harness/supervisor/actor_profiles.yaml`
- Create: `assets/defaults/supervisor/actor_profiles.yaml`
- Modify: `.harness/supervisor/task_router.yaml`
- Modify: `assets/defaults/supervisor/task_router.yaml`
- Modify: `.harness/supervisor/registry.yaml`
- Modify: `assets/defaults/supervisor/registry.yaml`
- Modify: `.harness/supervisor/context_contract.yaml`
- Modify: `assets/defaults/supervisor/context_contract.yaml`
- Modify: `.harness/actors/generator.md`
- Modify: `assets/defaults/actors/generator.md`
- Test: `internal/runtime/bootstrap_test.go`

- [ ] **Step 1: Make bootstrap fail until critic is part of the graph**

Update `internal/runtime/bootstrap_test.go` so `TestBootstrapCreatesExpectedArtifactSkeleton` expects:
- `critic_report.yaml`
- `actor_briefs/03_critic.md`
- `actor_briefs/06_evaluator.md`
- `workflow.Actors` to include `critic` between `context_builder` and `generator`

Run: `go test ./internal/runtime -run TestBootstrapCreatesExpectedArtifactSkeleton -count=1`
Expected: FAIL because the current actor graph and artifact skeleton do not include `critic`.

- [ ] **Step 2: Add `critic` to all task-family routes and registries**

Update both local and embedded supervisor files so every task family actor list becomes:

```yaml
actors:
  - planner
  - context_builder
  - critic
  - generator
  - executor
  - evaluator
```

Also add `critic_report` to each task-family `required_output` list in both registry files.

- [ ] **Step 3: Define the critic contract and generator dependency**

Update both `context_contract.yaml` files so:
- `critic` consumes `user_request`, `plan`, `context_pack`, `forbidden_changes`, and `rubric`
- `critic` outputs `critic_report`
- `generator` requires `critic_report` as an input
- workflow semantics still say `evaluator` is the conservative production gate

- [ ] **Step 4: Add actor instructions and repository-owned actor profiles**

Create `.harness/actors/critic.md` and `assets/defaults/actors/critic.md` with a non-coding brief that requires:
- `priority_focus`
- `missing_requirements`
- `risk_hypotheses`
- `validation_expectations`
- `generator_guardrails`
- `blocked_assumptions`

Create both `actor_profiles.yaml` files with explicit entries for `planner`, `context_builder`, `critic`, `generator`, and `evaluator`. Start with:

```yaml
version: 1
actors:
  planner: { model: gpt-5.4, reasoning: high }
  context_builder: { model: gpt-5.4-mini, reasoning: high }
  critic: { model: gpt-5.4, reasoning: high }
  generator: { model: gpt-5.4, reasoning: high }
  evaluator: { model: gpt-5.4, reasoning: high }
```

Update both generator briefs so the generator treats `critic_report` as required input rather than optional guidance.

- [ ] **Step 5: Re-run bootstrap coverage**

Run: `go test ./internal/runtime -run 'TestBootstrapCreatesExpectedArtifactSkeleton|TestValidateRequestAcceptsCurrentValidFixture' -count=1`
Expected: PASS with the new actor graph and artifact contract.

- [ ] **Step 6: Commit the supervisor contract slice**

```bash
git add .harness/actors/critic.md assets/defaults/actors/critic.md .harness/supervisor/actor_profiles.yaml assets/defaults/supervisor/actor_profiles.yaml .harness/supervisor/task_router.yaml assets/defaults/supervisor/task_router.yaml .harness/supervisor/registry.yaml assets/defaults/supervisor/registry.yaml .harness/supervisor/context_contract.yaml assets/defaults/supervisor/context_contract.yaml .harness/actors/generator.md assets/defaults/actors/generator.md internal/runtime/bootstrap_test.go
git commit -m "feat: add critic actor to supervisor contract"
```

### Task 2: Register `critic_report` as a First-Class Artifact

**Files:**
- Create: `.harness/templates/critic_report.schema.yaml`
- Create: `assets/defaults/templates/critic_report.schema.yaml`
- Modify: `internal/contracts/validator.go`
- Modify: `internal/contracts/validator_test.go`
- Modify: `internal/runtime/actor_runtime.go`
- Modify: `internal/runtime/actor_runtime_test.go`
- Modify: `internal/runtime/bootstrap.go`
- Modify: `internal/runtime/bootstrap_test.go`

- [ ] **Step 1: Add failing schema coverage before changing runtime code**

Extend `internal/contracts/validator_test.go` with a new case that validates a minimal `critic_report.yaml`, and extend `internal/runtime/actor_runtime_test.go` with a test that expects `actorOutputJSONSchema("critic_report")` to require all critic fields.

Run: `go test ./internal/contracts ./internal/runtime -run 'TestValidateArtifactFileSupportsLearningAndIntegrationSchemas|TestActorOutputJSONSchemaCriticReport' -count=1`
Expected: FAIL because `critic_report` is not registered yet.

- [ ] **Step 2: Define the critic report schema in both template trees**

Create `.harness/templates/critic_report.schema.yaml` and `assets/defaults/templates/critic_report.schema.yaml` with closed required fields:
- `priority_focus`
- `missing_requirements`
- `risk_hypotheses`
- `validation_expectations`
- `generator_guardrails`
- `blocked_assumptions`

Keep all fields machine-readable and array-or-string based. Do not make this a free-form review transcript.

- [ ] **Step 3: Register the schema and runtime output mapping**

Update:
- `internal/contracts/validator.go` so `ValidateArtifactFile(..., "critic_report")` resolves to `.harness/templates/critic_report.schema.yaml`
- `internal/runtime/actor_runtime.go` so `actorOutputJSONSchema("critic_report")` mirrors the checked-in schema
- `internal/runtime/bootstrap.go` so `canonicalOutputForActor("critic")`, `artifactFileName("critic_report")`, and `placeholderContent("critic_report")` all work

- [ ] **Step 4: Extend bootstrap placeholders**

Update `internal/runtime/bootstrap.go` so bootstrap creates `critic_report.yaml` and the placeholder contains every required field with empty-but-valid values. The placeholder should be schema-valid without implying any real critic findings.

- [ ] **Step 5: Re-run schema and bootstrap tests**

Run: `go test ./internal/contracts ./internal/runtime -run 'TestValidateArtifactFileSupportsLearningAndIntegrationSchemas|TestActorOutputJSONSchemaCriticReport|TestBootstrapCreatesExpectedArtifactSkeleton' -count=1`
Expected: PASS.

- [ ] **Step 6: Commit the artifact contract slice**

```bash
git add .harness/templates/critic_report.schema.yaml assets/defaults/templates/critic_report.schema.yaml internal/contracts/validator.go internal/contracts/validator_test.go internal/runtime/actor_runtime.go internal/runtime/actor_runtime_test.go internal/runtime/bootstrap.go internal/runtime/bootstrap_test.go
git commit -m "feat: register critic report artifacts"
```

### Task 3: Load Actor Profiles From Repository Policy and Remove Env/Timeout Defaults

**Files:**
- Create: `internal/runtime/actor_profiles.go`
- Create: `internal/runtime/actor_profiles_test.go`
- Modify: `internal/runtime/actor_runtime.go`
- Modify: `internal/runtime/actor_runtime_test.go`
- Modify: `internal/runtime/runner.go`
- Modify: `internal/runtime/runner_test.go`

- [ ] **Step 1: Write failing tests for checked-in actor profile resolution**

Create `internal/runtime/actor_profiles_test.go` with tests that:
- load valid `actor_profiles.yaml`
- reject missing actor entries
- reject unsupported reasoning values

Extend `internal/runtime/runner_test.go` real-mode coverage so the fake codex path captures actor order and the `-m` / `model_reasoning_effort` arguments used for each actor. Set `RAIL_ACTOR_MODEL` and `RAIL_ACTOR_REASONING_EFFORT` to wrong values in the test and assert the run still uses checked-in profiles.

Run: `go test ./internal/runtime -run 'TestLoadActorProfiles|TestExecuteRunsRealActorPathThroughCodex' -count=1`
Expected: FAIL because runtime still depends on env defaults and the old execution signature.

- [ ] **Step 2: Introduce an explicit actor-profile loader**

Create `internal/runtime/actor_profiles.go` with a typed loader that:
- reads `.harness/supervisor/actor_profiles.yaml` through existing asset resolution
- validates `version`
- requires entries for every structured actor in the workflow
- validates reasoning against the supported Codex values

Return a typed map or struct rather than passing raw YAML maps through the runner.

- [ ] **Step 3: Pass actor profiles explicitly into structured actor execution**

Refactor `internal/runtime/actor_runtime.go` so `runStructuredCodexCommand` accepts an explicit actor profile argument instead of reading `RAIL_ACTOR_MODEL` and `RAIL_ACTOR_REASONING_EFFORT`.

Remove:
- environment-variable default model selection
- environment-variable default reasoning selection
- actor-level timeout normalization and timeout enforcement for structured actors

Keep the command line deterministic by always emitting the profile-selected `-m <model>` and `-c model_reasoning_effort="<reasoning>"`.

- [ ] **Step 4: Update the runner to use checked-in profiles only**

Update `internal/runtime/runner.go` so the runner:
- loads actor profiles once per execute run
- resolves the current actor profile before each structured actor call
- stops passing a timeout into structured actor execution

Do not fall back to environment variables if the profile file is missing or invalid. Fail fast with a clear error.

- [ ] **Step 5: Re-run the runtime profile tests**

Run: `go test ./internal/runtime -run 'TestLoadActorProfiles|TestExecuteRunsRealActorPathThroughCodex' -count=1`
Expected: PASS, with captured execution showing `critic` in the actor list and profile-selected model/reasoning values even when env vars are set incorrectly.

- [ ] **Step 6: Commit the actor-profile runtime slice**

```bash
git add internal/runtime/actor_profiles.go internal/runtime/actor_profiles_test.go internal/runtime/actor_runtime.go internal/runtime/actor_runtime_test.go internal/runtime/runner.go internal/runtime/runner_test.go
git commit -m "feat: drive actor execution from checked-in profiles"
```

### Task 4: Wire Critic Through Smoke and Real Execution Paths

**Files:**
- Modify: `internal/runtime/runner.go`
- Modify: `internal/runtime/runner_test.go`
- Modify: `internal/runtime/bootstrap_test.go`

- [ ] **Step 1: Make smoke and real-mode execution tests fail until critic runs**

Update `internal/runtime/runner_test.go` so:
- smoke-path expectations include a valid `critic_report.yaml`
- real actor execution order becomes `planner`, `context_builder`, `critic`, `generator`, `evaluator`
- fake codex returns a structured `critic_report` when actor name is `critic`

Run: `go test ./internal/runtime -run 'TestExecuteRunsRealActorPathThroughCodex|TestExecutePreservesSupervisorTraceability' -count=1`
Expected: FAIL because the current loop skips `critic`.

- [ ] **Step 2: Add smoke-mode critic output**

Update `internal/runtime/runner.go` to return a valid smoke `critic_report` during `validation_profile: smoke` runs. Keep it narrow and deterministic:
- one priority focus
- empty or minimal arrays for unneeded fields
- no fake routing authority

- [ ] **Step 3: Insert critic into the real execution switch**

Update the real-mode actor switch in `internal/runtime/runner.go` so:
- `critic` is treated as a structured actor like `planner`, `context_builder`, `generator`, and `evaluator`
- its output is written to `critic_report.yaml`
- the execute loop naturally traverses `critic` before `generator`

- [ ] **Step 4: Re-run the execution-path tests**

Run: `go test ./internal/runtime -run 'TestExecuteRunsRealActorPathThroughCodex|TestExecutePreservesSupervisorTraceability|TestBootstrapCreatesExpectedArtifactSkeleton' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit the execution-loop slice**

```bash
git add internal/runtime/runner.go internal/runtime/runner_test.go internal/runtime/bootstrap_test.go
git commit -m "feat: run critic in smoke and real execution paths"
```

### Task 5: Report Critic Participation and Critic-to-Evaluator Value

**Files:**
- Modify: `.harness/templates/execution_report.schema.yaml`
- Modify: `assets/defaults/templates/execution_report.schema.yaml`
- Modify: `internal/runtime/router.go`
- Modify: `internal/runtime/router_test.go`
- Modify: `internal/cli/app_test.go`
- Modify: `test/fixtures/standard_route/blocked_environment/workflow.json`
- Modify: `test/fixtures/standard_route/blocked_environment/state.json`
- Modify: `test/fixtures/standard_route/rebuild_context/workflow.json`
- Modify: `test/fixtures/standard_route/rebuild_context/state.json`
- Modify: `test/fixtures/standard_route/split_task/workflow.json`
- Modify: `test/fixtures/standard_route/split_task/state.json`
- Modify: `test/fixtures/standard_route/tighten_validation/workflow.json`
- Modify: `test/fixtures/standard_route/tighten_validation/state.json`

- [ ] **Step 1: Add failing router expectations for critic-aware reporting**

Extend `internal/runtime/router_test.go` so terminal enrichment expects:
- `actor_profiles_used`
- `critic_findings_applied`
- `critic_to_evaluator_delta`
- actor graph traversal that includes `critic`

Run: `go test ./internal/runtime -run 'TestRouteEvaluationEnrichesExecutionReportForTerminalOutcome|TestRouteEvaluationCreatesTerminalSummaryForTerminalFixtures' -count=1`
Expected: FAIL because reports do not include the new critic-aware sections yet.

- [ ] **Step 2: Extend both execution report schemas**

Add optional fields in both `execution_report.schema.yaml` files for:
- `actor_profiles_used`
- `critic_findings_applied`
- `critic_to_evaluator_delta`

Keep the schema additive so existing executor-produced reports remain valid before router enrichment.

- [ ] **Step 3: Enrich router output using critic artifacts and actor profiles**

Update `internal/runtime/router.go` so final report enrichment:
- records the checked-in model and reasoning used per actor
- summarizes which critic findings were present and whether they were later resolved, confirmed, or left unmet
- includes a compact `critic_to_evaluator_delta` summary showing how pre-generation critique compares with final evaluation findings

Use the existing actor traversal helpers as the base. Do not give the critic routing authority in this step.

- [ ] **Step 4: Refresh standard-route fixtures**

Update the four `test/fixtures/standard_route/*/workflow.json` and `state.json` files so each fixture graph includes `critic` in the actor list and completed-actor path. Keep the terminal semantics unchanged.

- [ ] **Step 5: Re-run router and CLI coverage**

Run: `go test ./internal/runtime ./internal/cli -run 'TestRouteEvaluation|TestAppRun' -count=1`
Expected: PASS with critic-aware execution report enrichment and fixture compatibility.

- [ ] **Step 6: Commit the reporting slice**

```bash
git add .harness/templates/execution_report.schema.yaml assets/defaults/templates/execution_report.schema.yaml internal/runtime/router.go internal/runtime/router_test.go internal/cli/app_test.go test/fixtures/standard_route/blocked_environment/workflow.json test/fixtures/standard_route/blocked_environment/state.json test/fixtures/standard_route/rebuild_context/workflow.json test/fixtures/standard_route/rebuild_context/state.json test/fixtures/standard_route/split_task/workflow.json test/fixtures/standard_route/split_task/state.json test/fixtures/standard_route/tighten_validation/workflow.json test/fixtures/standard_route/tighten_validation/state.json
git commit -m "feat: report critic participation in execution artifacts"
```

### Task 6: Document the New Multi-Actor Policy and Run Full Verification

**Files:**
- Modify: `README.md`
- Modify: `README-kr.md`
- Modify: `docs/releases/v2-release-evidence-runbook.md`
- Reference: `docs/superpowers/specs/2026-04-20-critic-actor-design.md`

- [ ] **Step 1: Update operator-facing docs**

Update `README.md` and `README-kr.md` so they explicitly state:
- all task families now traverse `critic`
- actor model and reasoning come from checked-in `actor_profiles.yaml`
- environment variables are not the default actor-quality contract
- structured actors no longer use actor-level timeouts

Update `docs/releases/v2-release-evidence-runbook.md` so release evidence review mentions:
- `actor_profiles_used`
- `critic_findings_applied`
- `critic_to_evaluator_delta`

- [ ] **Step 2: Run the full repository test suite**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 3: Build the CLI**

Run: `go build -o build/rail ./cmd/rail`
Expected: exit code `0`.

- [ ] **Step 4: Validate a canonical request**

Run: `./build/rail validate-request --request .harness/requests/rail-bootstrap-smoke.yaml`
Expected: `Request is valid: .harness/requests/rail-bootstrap-smoke.yaml`

- [ ] **Step 5: Verify route-evaluation reporting on a copied fixture**

Copy one `test/fixtures/standard_route` artifact into a temporary `.harness/artifacts/<temp-id>` directory and run:

```bash
./build/rail route-evaluation --artifact .harness/artifacts/<temp-id>
```

Expected:
- summary includes a terminal status
- `execution_report.yaml` contains `actor_graph`, `actor_profiles_used`, `critic_findings_applied`, and `critic_to_evaluator_delta`
- `terminal_summary.md` and `supervisor_trace.md` show the critic stage in traversal

- [ ] **Step 6: Verify smoke execution traverses critic**

Create a smoke artifact through `run`, then execute it:

```bash
./build/rail run --request .harness/requests/rail-bootstrap-smoke.yaml --project-root /absolute/path/to/rail --task-id critic-smoke
./build/rail execute --artifact .harness/artifacts/critic-smoke
```

Expected:
- the run completes without actor-level timeout handling
- `critic_report.yaml` is present
- the final trace shows `planner -> context_builder -> critic -> generator -> executor -> evaluator`

- [ ] **Step 7: Commit the docs and verification slice**

```bash
git add README.md README-kr.md docs/releases/v2-release-evidence-runbook.md
git commit -m "docs: describe critic actor execution policy"
```
