# Actor Executor Port and Result Projection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `rail result` read-model projection for harness outcomes, then refactor actor execution behind an executor-neutral `ActorExecutor` port without changing current `codex_cli` behavior.

**Architecture:** Keep `run_status.yaml` and `terminal_summary.md` as canonical artifacts. Add pure runtime projection builders, a thin CLI command, and a Codex-skill reporting flow before porting `codex exec` into a `codex_cli` adapter. When `run_status.yaml` is absent and Rail synthesizes status from `state.json`, the projection must cite `state.json` rather than a missing status file. The executor port should speak Rail concepts while CLI flags remain adapter-local.

**Tech Stack:** Go CLI in `cmd/rail`, `internal/cli`, and `internal/runtime`; Markdown skill/docs updates; tests with existing smoke artifacts and fake Codex command helpers.

---

## Design References

- Spec: `docs/superpowers/specs/2026-04-27-actor-backend-port-and-result-projection-design.md`
- Existing status implementation: `internal/runtime/run_status.go`
- Existing terminal summary implementation: `internal/runtime/router.go`
- Existing CLI command pattern: `internal/cli/status.go`, `internal/cli/supervise.go`, `internal/cli/app.go`
- Existing actor execution: `internal/runtime/actor_runtime.go`
- Existing sealed runtime: `internal/runtime/actor_runtime_sealed.go`
- Existing backend policy: `internal/runtime/actor_backend.go`

## File Structure

- Create: `internal/runtime/result_projection.go`
  - Owns `HarnessResult`, JSON/human projection data, status normalization, terminal/non-terminal summary derivation, and latest artifact scanning.
- Create: `internal/runtime/result_projection_test.go`
  - Unit tests for initialized, interrupted, terminal, missing terminal summary, and latest-artifact projection.
- Create: `internal/cli/result.go`
  - Parses `rail result` flags and writes human or JSON output.
- Modify: `internal/cli/app.go`
  - Registers and dispatches the `result` command.
- Modify: `internal/cli/app_test.go`
  - Adds command registration and end-to-end CLI tests.
- Modify: `internal/cli/supervise.go`
  - Prints or points to the same result projection after supervise success and failure.
- Modify: `skills/rail/SKILL.md`
  - Updates later execution flow to run `rail result --json` after `rail supervise`.
- Modify: `assets/skill/Rail/SKILL.md`
  - Mirrors the repo-owned skill changes.
- Create: `internal/runtime/actor_executor_port.go`
  - Defines executor-neutral `ActorExecutor`, `ActorInvocation`, `ActorResult`, and supporting types.
- Create: `internal/runtime/codex_cli_executor.go`
  - Moves `codex exec` command construction/execution behind a `CodexCLIExecutor` adapter.
- Modify: `internal/runtime/actor_runtime.go`
  - Shrinks to shared schema/redaction helpers or delegates through the new adapter.
- Modify: `internal/runtime/runner.go`
  - Calls the `ActorExecutor` port instead of directly calling `runCommand`.
- Modify: `internal/runtime/actor_runtime_test.go`
  - Updates CLI-arg and sealed-runtime tests to exercise the `codex_cli` adapter.

---

### Task 1: Result Projection Runtime Model

**Files:**
- Create: `internal/runtime/result_projection.go`
- Create: `internal/runtime/result_projection_test.go`

- [ ] **Step 1: Write failing test for initialized projection**

Add a test that bootstraps a smoke artifact and projects its result before execution.

```go
func TestProjectHarnessResultForInitializedArtifact(t *testing.T) {
    projectRoot, requestPath := prepareSmokeProject(t)
    runner, err := NewRunner(projectRoot)
    if err != nil {
        t.Fatalf("NewRunner returned error: %v", err)
    }
    artifactPath, err := runner.Run(requestPath, "result-initialized")
    if err != nil {
        t.Fatalf("Run returned error: %v", err)
    }

    result, err := ProjectHarnessResultForArtifact(projectRoot, artifactPath)
    if err != nil {
        t.Fatalf("ProjectHarnessResultForArtifact returned error: %v", err)
    }
    if result.Status != "initialized" || result.Phase != "bootstrap" || result.Terminal {
        t.Fatalf("unexpected initialized result: %#v", result)
    }
    if result.CurrentActor != "planner" {
        t.Fatalf("expected planner current actor, got %q", result.CurrentActor)
    }
    if !slices.Contains(result.Evidence, "run_status.yaml") {
        t.Fatalf("expected run_status.yaml evidence, got %#v", result.Evidence)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/runtime -run TestProjectHarnessResultForInitializedArtifact -count=1
```

Expected: FAIL because `ProjectHarnessResultForArtifact` is undefined.

- [ ] **Step 3: Add result projection types**

In `internal/runtime/result_projection.go`, add:

```go
type HarnessResult struct {
    SchemaVersion       int               `json:"schema_version"`
    ArtifactDir         string            `json:"artifact_dir"`
    Status              string            `json:"status"`
    RawStatus           string            `json:"raw_status,omitempty"`
    Phase               string            `json:"phase"`
    CurrentActor        string            `json:"current_actor,omitempty"`
    LastSuccessfulActor string            `json:"last_successful_actor,omitempty"`
    InterruptionKind    string            `json:"interruption_kind,omitempty"`
    Message             string            `json:"message,omitempty"`
    Terminal            bool              `json:"terminal"`
    HumanSummary        string            `json:"human_summary"`
    RecommendedNextStep string            `json:"recommended_next_step,omitempty"`
    Evidence            []string          `json:"evidence"`
    SourceArtifacts     map[string]string `json:"source_artifacts"`
    UpdatedAt           string            `json:"updated_at,omitempty"`
}
```

Add `ProjectHarnessResultForArtifact(projectRoot, artifactPath string) (HarnessResult, error)` that:

- resolves the artifact through `NewRouter(projectRoot).resolveArtifactDirectory`
- reads `ReadRunStatus`
- normalizes the projected status
- adds `run_status.yaml` to evidence when that file exists; if status is
  synthesized from `state.json`, cite `state.json` instead
- avoids backfilling stale `evaluation_result.yaml`, `execution_report.yaml`,
  or `supervisor_trace.md` into non-terminal projections just because files
  exist
- builds a concise `HumanSummary`
- does not write any files

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
go test ./internal/runtime -run TestProjectHarnessResultForInitializedArtifact -count=1
```

Expected: PASS.

- [ ] **Step 5: Add interrupted projection test**

Seed a bad current actor, call `runner.Execute`, then project the result.

```go
func TestProjectHarnessResultForInterruptedArtifact(t *testing.T) {
    projectRoot, requestPath := prepareSmokeProject(t)
    runner, err := NewRunner(projectRoot)
    if err != nil {
        t.Fatalf("NewRunner returned error: %v", err)
    }
    artifactPath, err := runner.Run(requestPath, "result-interrupted")
    if err != nil {
        t.Fatalf("Run returned error: %v", err)
    }

    statePath := filepath.Join(artifactPath, "state.json")
    data, err := os.ReadFile(statePath)
    if err != nil {
        t.Fatalf("read state fixture: %v", err)
    }
    mutated := strings.Replace(string(data), `"currentActor": "planner"`, `"currentActor": "missing_actor"`, 1)
    if err := os.WriteFile(statePath, []byte(mutated), 0o644); err != nil {
        t.Fatalf("write state fixture: %v", err)
    }
    if _, err := runner.Execute(artifactPath); err == nil {
        t.Fatalf("expected Execute to interrupt")
    }

    result, err := ProjectHarnessResultForArtifact(projectRoot, artifactPath)
    if err != nil {
        t.Fatalf("ProjectHarnessResultForArtifact returned error: %v", err)
    }
    if result.Status != "interrupted" || result.Phase != "actor_resolution" {
        t.Fatalf("unexpected interrupted result: %#v", result)
    }
    if result.CurrentActor != "missing_actor" {
        t.Fatalf("expected missing_actor, got %q", result.CurrentActor)
    }
    if result.RecommendedNextStep == "" {
        t.Fatalf("expected recommended next step")
    }
}
```

- [ ] **Step 6: Add terminal projection test**

Execute a smoke artifact to completion and assert:

- `Status == "passed"`
- `Terminal == true`
- `Evidence` includes `terminal_summary.md`
- `SourceArtifacts["terminal_summary"] == "terminal_summary.md"`
- `HumanSummary` does not require reading raw logs

- [ ] **Step 7: Add status normalization helper**

Implement a small helper:

```go
func projectedResultStatus(status RunStatus) string {
    switch status.Status {
    case "blocked_environment", "split_required":
        return "blocked"
    case "revise_exhausted", "evolution_exhausted":
        return "rejected"
    case "passed", "rejected", "initialized", "in_progress", "retrying", "interrupted":
        return status.Status
    default:
        if status.InterruptionKind != "" {
            return "interrupted"
        }
        return fallbackString(status.Status, "unknown")
    }
}
```

- [ ] **Step 8: Run focused runtime tests**

Run:

```bash
go test ./internal/runtime -run 'TestProjectHarnessResult' -count=1
```

Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/runtime/result_projection.go internal/runtime/result_projection_test.go
git commit -m "feat: add harness result projection model"
```

---

### Task 2: `rail result` CLI

**Files:**
- Create: `internal/cli/result.go`
- Modify: `internal/cli/app.go`
- Modify: `internal/cli/app_test.go`

- [ ] **Step 1: Write failing command registration test**

Update `TestAppRegistersCoreCommands` in `internal/cli/app_test.go` to include `result` after `status`.

- [ ] **Step 2: Run registration test to verify it fails**

Run:

```bash
go test ./internal/cli -run TestAppRegistersCoreCommands -count=1
```

Expected: FAIL because `result` is not registered.

- [ ] **Step 3: Wire the command name**

In `internal/cli/app.go`, add `result` to `commands` and dispatch:

```go
if args[0] == "result" {
    if err := RunResult(args[1:], os.Stdout); err != nil {
        _, _ = fmt.Fprintln(os.Stderr, err)
        return 1
    }
    return 0
}
```

- [ ] **Step 4: Add CLI parser and output**

Create `internal/cli/result.go`:

```go
func RunResult(args []string, stdout io.Writer) error {
    var artifactPath string
    jsonOutput := false
    for i := 0; i < len(args); i++ {
        switch args[i] {
        case "--artifact":
            if i+1 >= len(args) {
                return fmt.Errorf("missing value for --artifact")
            }
            artifactPath = args[i+1]
            i++
        case "--json":
            jsonOutput = true
        default:
            return fmt.Errorf("unknown result flag: %s", args[i])
        }
    }
    if strings.TrimSpace(artifactPath) == "" {
        return fmt.Errorf("result requires --artifact")
    }
    workspace, err := discoverWorkspaceFromPath(artifactPath)
    if err != nil {
        return err
    }
    resolvedArtifactPath, err := resolveWorkspaceInputPath(workspace.Root, artifactPath)
    if err != nil {
        return err
    }
    result, err := runtime.ProjectHarnessResultForArtifact(workspace.Root, resolvedArtifactPath)
    if err != nil {
        return err
    }
    if jsonOutput {
        data, err := json.MarshalIndent(result, "", "  ")
        if err != nil {
            return err
        }
        _, err = fmt.Fprintln(stdout, string(data))
        return err
    }
    _, err = fmt.Fprint(stdout, runtime.FormatHarnessResult(result))
    return err
}
```

- [ ] **Step 5: Add human formatting**

In `internal/runtime/result_projection.go`, add `FormatHarnessResult(result HarnessResult) string` that prints:

```text
Rail result: passed

Phase: terminal
Current actor: none

What happened:
...

Next step:
...
```

Keep it concise. Do not dump full terminal summaries or raw logs.

- [ ] **Step 6: Add CLI smoke test for human output**

Add `TestAppRunPrintsHarnessResultForResultCommand`:

- run smoke request
- execute artifact
- call `NewApp().Run([]string{"result", "--artifact", artifactPath})`
- assert output contains `Rail result: passed`, `What happened:`, and `terminal_summary.md`

- [ ] **Step 7: Add CLI smoke test for JSON output**

Add `TestAppRunPrintsHarnessResultJSON`:

- run smoke request
- call `result --artifact <artifact> --json` before execution
- decode JSON into `map[string]any`
- assert `schema_version == 1`, `status == initialized`, `terminal == false`

- [ ] **Step 8: Run focused CLI tests**

Run:

```bash
go test ./internal/cli -run 'TestAppRegistersCoreCommands|TestAppRunPrintsHarnessResult' -count=1
```

Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/cli/app.go internal/cli/app_test.go internal/cli/result.go internal/runtime/result_projection.go
git commit -m "feat: add rail result command"
```

---

### Task 3: Latest Result Selection

**Files:**
- Modify: `internal/runtime/result_projection.go`
- Modify: `internal/runtime/result_projection_test.go`
- Modify: `internal/cli/result.go`
- Modify: `internal/cli/app_test.go`

- [ ] **Step 1: Write failing latest projection test**

Add a runtime test that creates two artifacts and writes distinct `run_status.yaml` `updated_at` values. Assert the latest projection selects the newer one.

```go
func TestProjectLatestHarnessResultSelectsNewestRunStatus(t *testing.T) {
    projectRoot, requestPath := prepareSmokeProject(t)
    runner, err := NewRunner(projectRoot)
    if err != nil {
        t.Fatalf("NewRunner returned error: %v", err)
    }
    oldArtifact, err := runner.Run(requestPath, "result-latest-old")
    if err != nil {
        t.Fatalf("old Run returned error: %v", err)
    }
    newArtifact, err := runner.Run(requestPath, "result-latest-new")
    if err != nil {
        t.Fatalf("new Run returned error: %v", err)
    }
    oldStatus, _ := ReadRunStatus(oldArtifact)
    oldStatus.UpdatedAt = "2026-04-27T00:00:00Z"
    if err := writeYAML(filepath.Join(oldArtifact, runStatusFileName), oldStatus); err != nil {
        t.Fatalf("write old status: %v", err)
    }
    newStatus, _ := ReadRunStatus(newArtifact)
    newStatus.UpdatedAt = "2026-04-27T00:01:00Z"
    if err := writeYAML(filepath.Join(newArtifact, runStatusFileName), newStatus); err != nil {
        t.Fatalf("write new status: %v", err)
    }

    result, err := ProjectLatestHarnessResult(projectRoot)
    if err != nil {
        t.Fatalf("ProjectLatestHarnessResult returned error: %v", err)
    }
    if result.ArtifactDir != newArtifact {
        t.Fatalf("expected latest artifact %q, got %q", newArtifact, result.ArtifactDir)
    }
}
```

- [ ] **Step 2: Implement latest scan**

Add `ProjectLatestHarnessResult(projectRoot string) (HarnessResult, error)`:

- scan `<projectRoot>/.harness/artifacts/*/run_status.yaml`
- parse `updated_at`
- ignore unreadable or invalid candidates
- choose newest timestamp
- call `ProjectHarnessResultForArtifact`
- return a clear error if no candidates exist

- [ ] **Step 3: Extend `rail result` parser**

Support:

```bash
rail result --latest --project-root /absolute/path/to/target-repo
rail result --latest --project-root /absolute/path/to/target-repo --json
```

Rules:

- `--artifact` and `--latest` are mutually exclusive.
- `--latest` requires `--project-root`.
- do not write a latest pointer file.

- [ ] **Step 4: Add CLI latest test**

Add `TestAppRunPrintsLatestHarnessResult` in `internal/cli/app_test.go`.

- [ ] **Step 5: Run focused tests**

Run:

```bash
go test ./internal/runtime -run TestProjectLatestHarnessResult -count=1
go test ./internal/cli -run TestAppRunPrintsLatestHarnessResult -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/runtime/result_projection.go internal/runtime/result_projection_test.go internal/cli/result.go internal/cli/app_test.go
git commit -m "feat: select latest harness result"
```

---

### Task 4: Supervise Output and Rail Skill Reporting

**Files:**
- Modify: `internal/cli/supervise.go`
- Modify: `internal/cli/app_test.go`
- Modify: `skills/rail/SKILL.md`
- Modify: `assets/skill/Rail/SKILL.md`

- [ ] **Step 1: Write failing supervise output test**

Update `TestAppRunSupervisesArtifactForSuperviseCommand` so successful output must include:

- `Rail result: passed`
- `What happened:`
- `terminal_summary.md`

Keep existing `supervised` assertion if desired, but make the result projection the user-facing anchor.

- [ ] **Step 2: Update supervise success output**

In `internal/cli/supervise.go`, after `runner.Supervise` succeeds:

- print the existing supervise summary
- compute `runtime.ProjectHarnessResultForArtifact`
- print `runtime.FormatHarnessResult`

If projection fails after successful supervise, return that projection error because user reporting is now part of the command contract.

- [ ] **Step 3: Update supervise failure output**

On supervise error:

- keep current status fallback if projection fails
- prefer `rail result` projection when available
- then return the original supervise error

This preserves failure exit code while giving Codex a stable human result.

- [ ] **Step 4: Run CLI supervise tests**

Run:

```bash
go test ./internal/cli -run 'TestAppRunSupervisesArtifactForSuperviseCommand|TestAppRunSupervisePrintsStatusWhenBlocked' -count=1
```

Expected: PASS.

- [ ] **Step 5: Update Rail skill execution contract**

In both `skills/rail/SKILL.md` and `assets/skill/Rail/SKILL.md`, update later execution guidance:

```text
For later execution steps:
1. run `rail auth doctor` before standard actor execution
2. run `rail supervise --artifact ...`
3. always run `rail result --artifact ... --json` afterward when the artifact exists
4. use that result JSON to report outcome, evidence, residual risk, and next step
```

Add guardrail:

```text
Do not report harness success from `supervise` process output alone. Use `rail result --json` as the reporting contract.
```

- [ ] **Step 6: Check skill copies stay aligned**

Run:

```bash
diff -u skills/rail/SKILL.md assets/skill/Rail/SKILL.md
```

Expected: no unexpected differences except path casing or packaging metadata that already exists. If the files should be identical, make them identical.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/supervise.go internal/cli/app_test.go skills/rail/SKILL.md assets/skill/Rail/SKILL.md
git commit -m "feat: report supervise results through rail result"
```

---

### Task 5: Executor-Neutral Actor Port

**Files:**
- Create: `internal/runtime/actor_executor_port.go`
- Create: `internal/runtime/codex_cli_executor.go`
- Modify: `internal/runtime/actor_runtime.go`
- Modify: `internal/runtime/runner.go`
- Modify: `internal/runtime/actor_runtime_test.go`

- [ ] **Step 1: Write failing adapter args test**

Move or add a test named `TestCodexCLIExecutorBuildsExpectedArgs` that constructs an executor-neutral `ActorInvocation` and asserts the adapter still produces the current `codex exec` args.

Expected current args include:

- `exec`
- `-m <model>`
- `--cd <workingDirectory>`
- `--ephemeral`
- `--color never`
- `-s workspace-write`
- `--skip-git-repo-check`
- `--ignore-user-config`
- `--ignore-rules`
- model reasoning config
- approval policy config
- shell environment policy configs
- `--output-schema`
- `--output-last-message`
- prompt

- [ ] **Step 2: Add executor-neutral types**

Create `internal/runtime/actor_executor_port.go`:

```go
type ActorExecutor interface {
    RunActor(context.Context, ActorInvocation) (ActorResult, error)
}

type ActorInvocation struct {
    ActorName         string
    ActorRunID        string
    WorkingDirectory  string
    ArtifactDirectory string
    Prompt            string
    OutputSchemaPath  string
    LastMessagePath   string
    EventsPath        string
    Profile           ActorProfile
    Policy            ActorBackendConfig
}

type ActorResult struct {
    StructuredOutput map[string]any
    LastMessagePath  string
    EventsPath       string
    ProvenancePath   string
}
```

Use `ActorBackendConfig` temporarily as `Policy` to avoid a large policy migration in this task. Do not expose CLI-only fields in the invocation itself.

- [ ] **Step 3: Create `CodexCLIExecutor` adapter**

Create `internal/runtime/codex_cli_executor.go` with:

```go
type CodexCLIExecutor struct{}

func (CodexCLIExecutor) RunActor(ctx context.Context, invocation ActorInvocation) (ActorResult, error) {
    // normalize profile
    // prepare sealed runtime
    // execute sealed codex command
    // audit events
    // decode last message
}
```

Move the command execution body from `runCommand` into this method.

- [ ] **Step 4: Keep compatibility wrapper**

Keep `runCommand(backend ActorBackendConfig, spec ActorCommandSpec)` as a thin wrapper during migration:

```go
func runCommand(backend ActorBackendConfig, spec ActorCommandSpec) (map[string]any, error) {
    result, err := CodexCLIExecutor{}.RunActor(context.Background(), invocationFromCommandSpec(backend, spec))
    if err != nil {
        return nil, err
    }
    return result.StructuredOutput, nil
}
```

This keeps existing tests passing while allowing `runner.go` to move later.

- [ ] **Step 5: Run actor runtime tests**

Run:

```bash
go test ./internal/runtime -run 'TestCodexCLIExecutor|TestBuildCodexCLIArgs|TestRunCommand|TestPrepareSealedActorRuntime' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/runtime/actor_executor_port.go internal/runtime/codex_cli_executor.go internal/runtime/actor_runtime.go internal/runtime/actor_runtime_test.go
git commit -m "refactor: introduce actor executor port"
```

---

### Task 6: Runner Uses ActorExecutor Port

**Files:**
- Modify: `internal/runtime/runner.go`
- Modify: `internal/runtime/runner_test.go`
- Modify: `internal/runtime/integration_test.go` if fake Codex setup needs adjustment

- [ ] **Step 1: Add executor field to Runner**

Update `Runner` construction so real actor execution can use an `ActorExecutor`:

```go
type Runner struct {
    // existing fields
    actorExecutor ActorExecutor
}
```

Default to `CodexCLIExecutor{}` in `NewRunner`.

- [ ] **Step 2: Route actor execution through port**

In `runner.go`, replace direct `runCommand` usage with:

```go
result, err := r.actorExecutor.RunActor(ctx, ActorInvocation{...})
response := result.StructuredOutput
```

Keep smoke actor behavior deterministic and unchanged.

- [ ] **Step 3: Add fake executor test**

Add a focused test that injects a fake `ActorExecutor` into `Runner` and proves `runner.Execute` can complete planner/context/critic/generator/evaluator actor calls without invoking Codex directly.

The fake executor should return minimal schema-valid outputs by actor name.

- [ ] **Step 4: Preserve existing fake Codex tests**

Run existing integration tests that assert Codex CLI command construction and sealed runtime behavior. Do not weaken those tests.

Run:

```bash
go test ./internal/runtime -run 'TestExecute|TestSupervise|TestRunCommand|TestCodex' -count=1
go test ./internal/runtime -run TestStandardProfile -count=1
```

Expected: PASS or no tests for patterns that do not exist.

- [ ] **Step 5: Commit**

```bash
git add internal/runtime/runner.go internal/runtime/runner_test.go internal/runtime/integration_test.go
git commit -m "refactor: execute actors through executor port"
```

---

### Task 7: Normalized Runtime Evidence

**Files:**
- Modify: `internal/runtime/codex_cli_executor.go`
- Modify: `internal/runtime/actor_runtime_sealed.go`
- Modify: `internal/runtime/actor_runtime_test.go`

- [ ] **Step 1: Write failing evidence test**

Add a test that runs the `codex_cli` adapter with fake Codex and expects:

```text
runs/01_planner-runtime-evidence.yaml
```

with:

- `schema_version: 1`
- `backend_type: codex_cli`
- `actor: planner`
- `raw_event_log: ...`
- `provenance: ...`
- `policy.sandbox: workspace-write`
- `redaction.secret_values_written: false`

- [ ] **Step 2: Add evidence writer**

Add a small struct and writer in `codex_cli_executor.go`:

```go
type RuntimeEvidence struct {
    SchemaVersion int `yaml:"schema_version"`
    BackendType string `yaml:"backend_type"`
    Actor string `yaml:"actor"`
    ActorRunID string `yaml:"actor_run_id"`
    Status string `yaml:"status"`
    RawEventLog string `yaml:"raw_event_log,omitempty"`
    Provenance string `yaml:"provenance,omitempty"`
    Policy map[string]any `yaml:"policy"`
    Redaction map[string]any `yaml:"redaction"`
    PolicyViolations []string `yaml:"policy_violations"`
}
```

Write it after actor execution and event audit. Use relative paths from the artifact directory when possible.

- [ ] **Step 3: Include evidence ref in `ActorResult`**

Set `ActorResult.RuntimeEvidence` or an equivalent path field so future summaries can cite normalized evidence.

- [ ] **Step 4: Run focused evidence tests**

Run:

```bash
go test ./internal/runtime -run 'TestCodexCLIExecutor.*Evidence|TestRunCommand' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/runtime/codex_cli_executor.go internal/runtime/actor_runtime_sealed.go internal/runtime/actor_runtime_test.go
git commit -m "feat: write normalized actor runtime evidence"
```

---

### Task 8: Final Validation and Docs

**Files:**
- Modify only if needed: `README.md`, `docs/ARCHITECTURE.md`, `docs/ARCHITECTURE-kr.md`
- Modify only if behavior changed: `docs/superpowers/specs/2026-04-27-actor-backend-port-and-result-projection-design.md`

- [ ] **Step 1: Run full Go test suite**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 2: Build CLI**

Run:

```bash
go build -o build/rail ./cmd/rail
```

Expected: PASS.

- [ ] **Step 3: Smoke `rail result` manually**

Run against a smoke request or fixture:

```bash
./build/rail run --request .harness/requests/smoke.yaml --project-root /absolute/path/to/target-repo --task-id result-smoke
./build/rail supervise --artifact /absolute/path/to/target-repo/.harness/artifacts/result-smoke
./build/rail result --artifact /absolute/path/to/target-repo/.harness/artifacts/result-smoke
./build/rail result --artifact /absolute/path/to/target-repo/.harness/artifacts/result-smoke --json
```

Use a real existing request path instead of inventing one if `.harness/requests/smoke.yaml` is not present.

Expected:

- human result is concise and understandable
- JSON decodes successfully
- no `result.json` or `result.md` files are written

- [ ] **Step 4: Documentation path lint**

Run:

```bash
rg -n '(/U[s]ers/|~[/])' README.md docs skills/rail assets/skill/Rail
```

Expected: no documentation examples with concrete home-directory paths.

- [ ] **Step 5: Review git diff**

Run:

```bash
git status --short
git diff --stat
```

Expected:

- only intended files changed
- no generated `.harness/artifacts/` edits
- no `.worktrees/` edits

- [ ] **Step 6: Commit final docs if needed**

```bash
git add README.md docs/ARCHITECTURE.md docs/ARCHITECTURE-kr.md docs/superpowers/specs/2026-04-27-actor-backend-port-and-result-projection-design.md
git commit -m "docs: align rail result and backend port guidance"
```

Skip this commit if no docs changed in this task.
