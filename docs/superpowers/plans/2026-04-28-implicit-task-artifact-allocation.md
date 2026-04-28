# Implicit Task Artifact Allocation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ensure a new natural-language Rail task can start even when the default `.harness/artifacts/request` artifact already exists in a blocked or completed state.

**Architecture:** Keep explicit operator-provided `--task-id` behavior strict, but make implicit task allocation collision-safe. When `rail run` is called without `--task-id`, derive the base id from the request filename as today, then atomically reserve the first unused artifact id by creating the artifact directory before bootstrapping. Update the Rail skill, bundled skill, and examples to run `rail run` for fresh execution, capture the printed artifact path, and use that path for `supervise`, `status`, and `result`.

**Tech Stack:** Go CLI/runtime, existing Rail bootstrapper/router, YAML request artifacts, repo-owned Rail skill markdown.

---

## File Structure

- Modify `internal/runtime/runner.go`
  - Owns `Runner.Run`, default task id derivation, artifact directory resolution, and collision checks.
  - Add implicit allocation helper so only omitted `--task-id` gets automatic suffixing.
- Modify `internal/cli/run.go`
  - Reject blank explicit `--task-id` values before they reach `Runner.Run`.
- Modify `internal/runtime/runner_test.go`
  - Add regression tests for implicit allocation and explicit collision behavior.
- Modify `internal/cli/app_test.go`
  - Add CLI-level coverage showing `rail run --request ... --project-root ...` can create a suffixed artifact when `request` already exists.
  - Add CLI-level coverage that blank explicit `--task-id` is rejected instead of becoming an implicit run.
- Modify `skills/rail/SKILL.md`
  - Tell the skill to run `rail run` without asking users for task ids and to trust the artifact path printed by the command.
- Modify `assets/skill/Rail/SKILL.md`
  - Keep bundled installed skill behavior aligned with the repo-owned skill.
- Modify `skills/rail/references/examples.md`
  - Update expected behavior examples to mention fresh artifact allocation and artifact path capture.
- Modify `assets/skill/Rail/references/examples.md`
  - Keep bundled examples aligned.
- Optionally modify `README.md`
  - Add one operator note that implicit runs allocate a unique artifact id when the default exists, while explicit `--task-id` remains strict.

## Task 1: Runtime Implicit Allocation

**Files:**
- Modify: `internal/runtime/runner.go`
- Test: `internal/runtime/runner_test.go`

- [ ] **Step 1: Write failing tests for implicit allocation contract**

Add runtime tests that cover the full implicit allocation contract.

Test A: no existing artifact directory creates the unsuffixed default:

```go
artifactPath, err := runner.Run(requestPath, "")
if err != nil {
    t.Fatalf("Run returned error: %v", err)
}
if filepath.Base(artifactPath) != "request" {
    t.Fatalf("expected implicit allocation to use request, got %s", artifactPath)
}
```

Test B: existing non-empty `request` creates `request-2`:


```go
artifactPath, err := runner.Run(requestPath, "")
if err != nil {
    t.Fatalf("Run returned error: %v", err)
}
if filepath.Base(artifactPath) != "request-2" {
    t.Fatalf("expected implicit allocation to use request-2, got %s", artifactPath)
}
```

Also assert:

- the original `.harness/artifacts/request` directory still exists and is untouched
- new `workflow.json` has `taskId: request-2`
- new `state.json` has `taskId: request-2`

Test C: existing non-empty `request` and `request-2` creates `request-3`.

Test D: existing empty `request` is treated as reserved for implicit allocation, so the next implicit run creates `request-2`. This is intentional: implicit allocation uses atomic `os.Mkdir`, so any pre-existing directory is considered occupied. Explicit `--task-id request` may still reuse an empty directory through the existing strict availability check.

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/runtime -run 'TestRunAllocatesImplicitTaskID|TestRunAllocatesNextImplicitTaskIDSuffix|TestRunTreatsExistingEmptyImplicitArtifactAsReserved' -count=1
```

Expected: at least the collision tests FAIL with the current `artifact directory already exists and is not empty` error.

- [ ] **Step 3: Implement implicit task id allocation**

Change `Runner.Run` so it distinguishes explicit and implicit task ids:

```go
func (r *Runner) Run(requestPath, taskID string) (string, error) {
    explicitTaskID := strings.TrimSpace(taskID) != ""
    effectiveTaskID := strings.TrimSpace(taskID)
    if effectiveTaskID == "" {
        effectiveTaskID = defaultTaskID(requestPath)
    }

    if !explicitTaskID {
        allocatedTaskID, err := r.reserveImplicitTaskID(effectiveTaskID)
        if err != nil {
            return "", err
        }
        effectiveTaskID = allocatedTaskID
    }

    artifactDirectory, err := r.resolveArtifactDirectory(effectiveTaskID)
    if err != nil {
        return "", err
    }
    if err := ensureArtifactDirectoryAvailable(artifactDirectory); err != nil {
        return "", err
    }

    return r.bootstrapper.Bootstrap(requestPath, effectiveTaskID)
}
```

Add helper behavior:

```go
func (r *Runner) reserveImplicitTaskID(base string) (string, error) {
    candidate := strings.TrimSpace(base)
    if candidate == "" {
        candidate = "rail-task"
    }
    for index := 1; index <= 1000; index++ {
        suffix := ""
        if index > 1 {
            suffix = fmt.Sprintf("-%d", index)
        }
        taskID := candidate + suffix
        artifactDirectory, err := r.resolveArtifactDirectory(taskID)
        if err != nil {
            return "", err
        }
        if err := os.MkdirAll(filepath.Dir(artifactDirectory), 0o755); err != nil {
            return "", fmt.Errorf("create artifact root: %w", err)
        }
        if err := os.Mkdir(artifactDirectory, 0o755); err == nil {
            return taskID, nil
        } else if os.IsExist(err) {
            continue
        } else {
            return "", fmt.Errorf("reserve artifact directory: %w", err)
        }
    }
    return "", fmt.Errorf("could not allocate available artifact id for %s", candidate)
}
```

Keep `ensureArtifactDirectoryAvailable` for explicit task ids. Do not use it for implicit allocation, because it is check-then-write and cannot reserve a candidate atomically.

If `Bootstrap` fails after implicit reservation, remove the reserved directory only when it is still empty. If files were written, leave the artifact evidence in place, matching existing partial-failure behavior.

- [ ] **Step 4: Run runtime test to verify it passes**

Run:

```bash
go test ./internal/runtime -run 'TestRunAllocatesImplicitTaskID|TestRunAllocatesNextImplicitTaskIDSuffix|TestRunTreatsExistingEmptyImplicitArtifactAsReserved' -count=1
```

Expected: PASS.

## Task 2: Preserve Explicit `--task-id` Safety

**Files:**
- Modify: `internal/cli/run.go`
- Modify: `internal/runtime/runner_test.go`
- Modify: `internal/cli/app_test.go`

- [ ] **Step 1: Write failing or guarding test for explicit collision**

Add a test where `.harness/artifacts/request` exists and `runner.Run(requestPath, "request")` is called.

Expected behavior:

```go
_, err := runner.Run(requestPath, "request")
if err == nil {
    t.Fatalf("expected explicit task id collision to fail")
}
if !strings.Contains(err.Error(), "artifact directory already exists and is not empty") {
    t.Fatalf("unexpected error: %v", err)
}
```

- [ ] **Step 2: Write CLI test for blank explicit task id**

Add a CLI test that calls:

```go
got, output := runAppAndCaptureStderr(t, []string{
    "run",
    "--request", requestPath,
    "--project-root", projectRoot,
    "--task-id", "   ",
})
```

Expected:

- exit code is non-zero
- stderr contains `task id must not be blank`
- no `.harness/artifacts/request` artifact is created by this command

- [ ] **Step 3: Implement CLI validation**

In `internal/cli/run.go`, after consuming `--task-id`, reject blank values:

```go
taskID = args[i+1]
if strings.TrimSpace(taskID) == "" {
    return fmt.Errorf("task id must not be blank")
}
i++
```

This keeps omitted `--task-id` as the only implicit allocation path at the CLI/operator boundary.

- [ ] **Step 4: Run focused test**

Run:

```bash
go test ./internal/runtime -run 'TestRunAllocatesImplicitTaskID|TestRunRejectsExplicitTaskIDCollision' -count=1
go test ./internal/cli -run 'TestRunRejectsBlankTaskID' -count=1
```

Expected: PASS after Task 1 implementation. If explicit collision is accidentally auto-suffixed, this test must fail.

- [ ] **Step 5: Commit runtime allocation**

```bash
git add internal/runtime/runner.go internal/runtime/runner_test.go internal/cli/run.go internal/cli/app_test.go
git commit -m "fix(runtime): allocate unique implicit task artifacts"
```

## Task 3: CLI Regression Coverage

**Files:**
- Modify: `internal/cli/app_test.go`

- [ ] **Step 1: Add CLI test**

Add a CLI test that:

- prepares a target project
- writes a valid `.harness/requests/request.yaml`
- creates a non-empty `.harness/artifacts/request`
- runs:

```bash
rail run --request <target>/.harness/requests/request.yaml --project-root <target>
```

Expected stdout contains:

```text
.harness/artifacts/request-2
```

Do not pass `--task-id` in this test.

Also assert that the created artifact contains `workflow.json` and `state.json` with matching `request-2` task ids.

- [ ] **Step 2: Run CLI test**

Run:

```bash
go test ./internal/cli -run TestRunAllocatesImplicitTaskArtifactWhenDefaultExists -count=1
```

Expected: PASS.

- [ ] **Step 3: Commit CLI coverage**

```bash
git add internal/cli/app_test.go
git commit -m "test(cli): cover implicit task artifact allocation"
```

## Task 4: Update Rail Skill Contract

**Files:**
- Modify: `skills/rail/SKILL.md`
- Modify: `assets/skill/Rail/SKILL.md`
- Modify: `skills/rail/references/examples.md`
- Modify: `assets/skill/Rail/references/examples.md`
- Optionally modify: `README.md`

- [ ] **Step 1: Update skill fresh-start execution instructions**

Rewrite the later execution instructions in both skill files so a fresh task flow is explicit:

```markdown
When the user asks to start or execute a fresh natural-language task, use this sequence:

1. `rail compose-request --stdin`
2. `rail run --request <printed-request-path> --project-root <target-repo>` without `--task-id`
3. capture the artifact path printed by `rail run`
4. `rail auth doctor`
5. `rail supervise --artifact <printed-artifact-path>`
6. `rail result --artifact <printed-artifact-path> --json`

Do not ask users to choose task ids. Do not reconstruct the artifact path from the request filename; Rail may allocate a suffix such as `request-2` when an earlier artifact exists.
```

- [ ] **Step 2: Update command block if needed**

Include `rail run` in the later execution command list:

```bash
rail run --request /absolute/path/to/target-repo/.harness/requests/request.yaml --project-root /absolute/path/to/target-repo
rail supervise --artifact /absolute/path/to/target-repo/.harness/artifacts/<allocated-task-id>
```

- [ ] **Step 3: Update skill examples**

In both examples files, update each expected skill behavior section that currently stops at request materialization. Make the expected behavior say:

- materialize the normalized request with `rail compose-request --stdin`
- when execution is requested, run `rail run` without `--task-id`
- capture and report the artifact path printed by `rail run`
- use that artifact path for `supervise`, `status`, and `result`

- [ ] **Step 4: Add installed-skill migration note**

Add a release/operator note in README or a relevant docs file:

```markdown
Existing local Codex installs may have an older copied Rail skill. After upgrading Rail, run `rail install-codex-skill --repair` or `rail init` in the target repository to refresh the installed skill so agents use the printed artifact path from implicit allocation.
```

- [ ] **Step 5: Add optional README operator note**

If README currently implies request filename maps directly to artifact id, update it:

```markdown
When `rail run` is called without `--task-id`, Rail derives a base id from the request filename and atomically allocates the first unused artifact directory. The printed artifact path is the durable run identity. The artifact-local `request.yaml` snapshot is the source of truth for that run, because `.harness/requests/request.yaml` may be overwritten by the next natural-language task. Explicit `--task-id` remains strict and fails on collisions.
```

- [ ] **Step 6: Validate docs do not include home paths**

Run:

```bash
rg -n -e '/U[s]ers/' -e '~[/]' -e '/h[o]me/' skills/rail assets/skill/Rail README.md docs/superpowers/plans/2026-04-28-implicit-task-artifact-allocation.md
```

Expected: no matches for newly added examples.

- [ ] **Step 7: Commit skill/docs**

```bash
git add skills/rail/SKILL.md assets/skill/Rail/SKILL.md skills/rail/references/examples.md assets/skill/Rail/references/examples.md README.md docs/superpowers/plans/2026-04-28-implicit-task-artifact-allocation.md
git commit -m "docs(rail): clarify implicit task artifact allocation"
```

## Task 5: End-to-End Verification

**Files:**
- No code changes expected.

- [ ] **Step 1: Run focused packages**

```bash
go test ./internal/runtime ./internal/cli -count=1
```

Expected: PASS.

- [ ] **Step 2: Run full suite**

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 3: Build CLI**

```bash
go build -o build/rail ./cmd/rail
```

Expected: exit 0.

- [ ] **Step 4: Run release gate**

```bash
./tool/release_gate.sh all
```

Expected: pass for the full current release gate surface. If `tool/release_gate.sh all` is unavailable, run both `./tool/v1_release_gate.sh` and `./tool/v2_release_gate.sh`.

- [ ] **Step 5: Manual artifact allocation smoke check**

In a temporary target repo, run two fresh implicit runs against the same request path:

```bash
rail run --request /absolute/path/to/target-repo/.harness/requests/request.yaml --project-root /absolute/path/to/target-repo
rail run --request /absolute/path/to/target-repo/.harness/requests/request.yaml --project-root /absolute/path/to/target-repo
```

Expected:

- first stdout ends with `.harness/artifacts/request`
- second stdout ends with `.harness/artifacts/request-2`
- both artifacts have matching `workflow.json.taskId` values

## Acceptance Criteria

- A blocked `.harness/artifacts/request` no longer prevents a fresh implicit `rail run` from creating a new artifact.
- Explicit `--task-id request` still fails if `.harness/artifacts/request` already exists and is non-empty.
- Rail skill instructions no longer require users to choose task ids.
- Rail skill uses the artifact path returned by `rail run`, so suffixed ids are preserved through `supervise`, `status`, and `result`.
- Full tests, build, and `tool/v2_release_gate.sh` pass.
