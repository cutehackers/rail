# Rail Runtime And Task Identity Recovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Rail reliably start fresh natural-language tasks and continue existing artifacts by fixing the sealed Codex runtime blockers, clarifying task identity rules in the Rail skill, and preventing command-like validation input from becoming broken execution plans.

**Architecture:** The Rail skill remains the user-facing control surface: it decides fresh task versus existing artifact before running commands. The runtime owns sealed actor readiness, Codex command resolution, actor-local materialization policy, and backend capability enforcement. Request normalization owns the contract for file-based validation hints so downstream actor briefs and executor previews stay deterministic.

**Tech Stack:** Go CLI/runtime under `cmd/rail`, `internal/cli`, `internal/request`, and `internal/runtime`; harness defaults under `.harness/` and `assets/defaults`; Rail skill copies under `skills/rail/SKILL.md` and `assets/skill/Rail/SKILL.md`; fake Codex binaries for runtime tests; release validation with `go test`, `go build`, and `./tool/release_gate.sh all`.

---

## Integrated Review

This plan supersedes:

- `docs/superpowers/plans/2026-04-28-rail-skill-task-identity-guidance.md`
- `docs/superpowers/plans/2026-04-28-molycard-rail-runtime-blockers.md`

The two source plans are directionally compatible:

- The task identity plan fixes user-facing ambiguity: a new goal should create a fresh implicit artifact even if another artifact is blocked, while continuation/status/result/integrate requests should use an existing artifact.
- The MolyCard runtime blocker plan fixes why the fresh artifact flow still failed in practice: `rail auth doctor` did not preflight sealed actor command resolution, and postflight policy treated current Codex CLI system materialization as user skill injection.

The combined plan changes the ordering:

1. First make `rail auth doctor` and sealed runtime policy accurately predict and enforce actor execution readiness.
2. Then make the Rail skill route fresh versus existing artifacts using that reliable runtime contract.
3. Then reject command-like `validation_targets` so non-Go targets do not generate malformed executor previews.

Resolved inconsistencies:

- Use the actual repo-owned skill path `skills/rail/...`; the bundled install asset remains `assets/skill/Rail/...`.
- Keep "users never choose task ids" from the task identity plan, but pair it with runtime preflight so fresh implicit artifacts do not immediately block.
- Keep "do not use absolute actor backend commands" from the runtime plan; the normal fix is doctor/readiness and trusted PATH resolution, not target-local policy overrides.
- Make doctor preflight project-aware with `rail auth doctor --project-root <target-repo>`; otherwise it cannot catch target-local actor backend policy overrides before `supervise`.
- Keep command-level validation out of scope for this release. The safer near-term behavior is to reject shell commands in `context.validation_targets` and keep validation commands in `definition_of_done` until a first-class schema exists.

## File Structure

- Modify `internal/runtime/actor_runtime_sealed.go`
  - Expose sealed Codex command readiness checks for doctor/preflight.
  - Replace broad actor-local materialization checks with narrow allowlists for marked system skills.
  - Keep user-home skill/plugin/hook/MCP checks strict.
- Modify `internal/runtime/actor_runtime.go`
  - Translate disabled backend capabilities into supported Codex CLI feature flags such as `--disable plugins`.
- Modify `internal/runtime/codex_cli_executor.go`
  - Keep execution flow unchanged unless runtime evidence needs policy metadata for newly allowed system skill materialization.
- Modify `internal/runtime/actor_runtime_test.go`
  - Add fake Codex tests for sealed command readiness, marked system skills, forbidden injected skills, plugin materialization, and all actor names.
- Modify `internal/runtime/event_audit_test.go`
  - Add or extend tests for user-home skill/plugin event injection.
- Modify `internal/cli/auth.go`
  - Make `rail auth doctor` verify actor runtime readiness after login readiness.
  - Keep a small package-level readiness function seam so CLI tests can stub runtime readiness without depending on fake Codex internals.
- Modify `internal/cli/auth_test.go`
  - Add doctor tests for missing, untrusted, and trusted sealed Codex command resolution, plus target-local unsafe actor backend policy.
- Create or modify `internal/request/validation_targets.go`
  - Own reusable semantic validation for command-looking validation target strings.
- Modify `internal/request/normalize.go`
  - Reject command-looking `context.validation_targets`.
- Modify `internal/contracts/validator.go`
  - Reject command-looking `context.validation_targets` for existing request YAML and `rail validate-request`, not only `compose-request`.
- Modify `internal/request/normalize_test.go`
  - Cover command-looking validation target rejection and ordinary file target acceptance.
- Modify `internal/runtime/bootstrap.go`
  - Call the same validation-target guard before building executor commands so direct bootstrap/run paths cannot bypass validation.
- Modify `internal/runtime/bootstrap_test.go`
  - Cover rejected command-looking validation targets at bootstrap/request materialization boundaries.
- Modify `skills/rail/SKILL.md`
  - Add task identity decision matrix and validation target guidance.
  - Clarify doctor/supervise blocked-environment flow.
- Modify `assets/skill/Rail/SKILL.md`
  - Keep bundled installed skill aligned with repo-owned skill.
- Modify `skills/rail/references/examples.md`
  - Add fresh task and existing artifact examples.
- Modify `assets/skill/Rail/references/examples.md`
  - Keep bundled examples aligned.
- Modify `internal/install/skill_test.go`
  - Assert bundled skill/examples contain the task identity contract and do not contain stale task-id guidance.
  - Correct repo-owned skill path casing if needed.
- Optional modify `README.md` or `docs/tasks.md`
  - Add a short troubleshooting note only if no current doc explains doctor/readiness and artifact identity.
- Optional modify `AGENTS.md`
  - Correct stale skill path references if the implementation is allowed to update repo-local agent instructions.

## Decision Contracts

### Task Identity

- Fresh task: a new goal, bug, feature, refactor, test repair, or other new natural-language work item.
- Existing artifact: continue, retry, supervise, inspect status/result, integrate, report, or debug a specific existing run.
- Explicit artifact path: always use that artifact directly; do not compose a new request and do not run `rail run`.
- Current-session artifact: use the artifact path printed by `rail run`, `rail status`, or `rail result`; do not reconstruct it from request filename.
- Ambiguous prior-work reference: ask one concise clarification only when the user gives neither a new goal nor an artifact path.
- User task ids: never ask users to choose task ids. Fresh work runs `rail run` without `--task-id`.

### Runtime Readiness

- `rail auth doctor` is ready only when both actor auth and sealed actor Codex command resolution are ready.
- Standard actor execution should run `rail auth doctor --project-root <target-repo>` so doctor checks the same target-local actor backend policy that `supervise` will load. For backward compatibility, plain `rail auth doctor` may default to the current working directory.
- Absolute actor backend commands remain invalid.
- Manual symlinks are not normal user guidance.
- Blocked artifacts remain evidence; new unrelated goals should start fresh implicit artifacts.

### Sealed Materialization Policy

- Allow only actor-local `CODEX_HOME/skills/.system/...` when the `.codex-system-skills.marker` exists.
- Reject actor-local `CODEX_HOME/skills/<non-.system>`.
- Reject actor-local `CODEX_HOME/skills/.system` without the marker.
- Reject user-home `.codex/skills`, `.codex/superpowers`, `.codex/plugins`, and `.codex/hooks`.
- Reject actor-local `superpowers`, `hooks`, and `mcp`.
- Disable plugins through Codex CLI feature flags; only consider a narrow metadata allowlist if current Codex still writes unavoidable metadata with plugins disabled.

### Validation Targets

- `context.validation_roots` are project-relative directories.
- `context.validation_targets` are project-relative files or test target paths.
- Shell commands such as `flutter analyze`, `dart format --line-length 120`, and `go test ./...` do not belong in `validation_targets`.
- Until Rail has first-class `validation_commands`, shell validation expectations belong in `definition_of_done`.

## Task 1: Runtime Doctor Preflight

**Files:**
- Modify: `internal/runtime/actor_runtime_sealed.go`
- Modify: `internal/cli/auth.go`
- Test: `internal/runtime/actor_runtime_test.go`
- Test: `internal/cli/auth_test.go`

- [ ] **Step 1: Write failing runtime readiness tests**

Add tests that exercise the same sealed command resolver used by actor execution:

```go
func TestActorRuntimeReadinessRejectsMissingCodexOnSealedPath(t *testing.T) {
    t.Setenv("PATH", t.TempDir())
    withDefaultTrustedPATHEntriesForTest(t, nil)

    err := CheckActorRuntimeReadinessForDoctor(t.TempDir())
    if err == nil || !strings.Contains(err.Error(), "unsafe_codex_path") {
        t.Fatalf("expected codex_vault path readiness failure, got %v", err)
    }
}

func TestActorRuntimeReadinessAcceptsTrustedCodex(t *testing.T) {
    fakeBin := t.TempDir()
    fakeCodexPath := writeFakeCodexExecutable(t, fakeBin)
    allowFakeCodexForTest(t, fakeCodexPath)
    t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

    if err := CheckActorRuntimeReadinessForDoctor(t.TempDir()); err != nil {
        t.Fatalf("expected actor runtime readiness, got %v", err)
    }
}
```

Do not rely on `PATH=t.TempDir()` alone. `sanitizeActorPATH` falls back to default trusted entries, so add a runtime package test seam that can override default trusted PATH entries during tests and restore it with `t.Cleanup`.

Use existing fake-Codex helpers where available; keep test-only overrides behind existing internal test markers. For runtime package tests, `allowFakeCodexForTest` and `testRailCodexAuthHome` are available. For CLI package tests, stub the readiness function in `internal/cli/auth.go` instead of duplicating runtime's internal fake-Codex marker protocol.

- [ ] **Step 2: Run focused tests and confirm failure**

```bash
go test ./internal/runtime -run 'TestActorRuntimeReadiness' -count=1
```

Expected: FAIL because the helper does not exist.

- [ ] **Step 3: Implement runtime readiness helper**

Create an exported or package-internal helper with a stable CLI-facing wrapper. It should load the actor backend policy for the supplied project root, get the default backend, build the same forbidden roots and sanitized PATH used for sealed actors, and call `resolveSealedCodexCommand`.

Keep returned errors sanitized: no concrete auth home or user home paths in doctor output.

- [ ] **Step 4: Add project-aware doctor option**

Extend auth option parsing with `--project-root`:

```go
type authOptions struct {
    codexCommand string
    projectRoot  string
}
```

Rules:

- `rail auth doctor --project-root /absolute/path/to/target-repo` checks that target repo's actor backend policy.
- `rail auth doctor` defaults to the current working directory for compatibility.
- `login`, `status`, and `logout` should reject `--project-root` unless there is a clear reason to accept and ignore it.
- `internal/cli/auth.go` should call a package-level variable such as `actorRuntimeReadinessCheck`, initialized to the runtime helper, so CLI tests can deterministically stub success/failure while runtime package tests cover real sealed resolver behavior.

- [ ] **Step 5: Wire `rail auth doctor`**

After `auth.RunCodexLoginStatus` succeeds, call the runtime readiness helper from `runAuthStatus(..., doctor=true)`.

Expected doctor output on success:

```text
Rail actor auth ready (source=rail_codex_login)
Rail actor runtime ready (backend=codex_cli)
Secret values are not printed.
```

Expected failure: nonzero exit with an actionable message that actor runtime cannot resolve `codex` on the sealed actor PATH.

Add assertions that doctor output and returned errors do not expose the user's home directory, the Rail auth home, raw PATH entries, or the full project root.

- [ ] **Step 6: Add target-local backend policy regression**

In `internal/cli/auth_test.go`, create a temporary target repo with `.harness/supervisor/actor_backend.yaml` whose backend command is an absolute path.

Run:

```bash
rail auth doctor --project-root /absolute/path/to/target-repo
```

Expected: doctor fails before `supervise` with a sanitized actor backend policy/readiness error.

- [ ] **Step 7: Run doctor tests**

```bash
go test ./internal/cli ./internal/runtime -run 'TestRunAuthDoctor|TestActorRuntimeReadiness' -count=1
```

Expected: PASS.

## Task 2: Backend Capability Flags

**Files:**
- Modify: `internal/runtime/actor_runtime.go`
- Test: `internal/runtime/actor_runtime_test.go`
- Inspect: `.harness/supervisor/actor_backend.yaml`
- Inspect: `assets/defaults/supervisor/actor_backend.yaml`

- [ ] **Step 1: Add failing CLI args assertions**

Extend `TestRunCommandUsesBackendPolicyForCodexInvocation` or add a focused test:

```go
func TestBuildCodexCLIArgsDisablesConfiguredCapabilities(t *testing.T) {
    args := buildCodexCLIArgs(defaultTestActorBackend(), ActorCommandSpec{
        ActorName:        "planner",
        Profile:          ActorProfile{Model: "gpt-5.4-mini", Reasoning: "high"},
        WorkingDirectory: t.TempDir(),
        Prompt:           "prompt",
        LastMessagePath:  filepath.Join(t.TempDir(), "last.json"),
        SchemaPath:       filepath.Join(t.TempDir(), "schema.json"),
    })
    joined := strings.Join(args, "\n")
    if !strings.Contains(joined, "--disable\nplugins") {
        t.Fatalf("expected plugins feature to be disabled, got %#v", args)
    }
}
```

- [ ] **Step 2: Run focused test and confirm failure**

```bash
go test ./internal/runtime -run TestBuildCodexCLIArgsDisablesConfiguredCapabilities -count=1
```

Expected: FAIL because the flag is missing.

- [ ] **Step 3: Implement capability-to-flag mapping**

In `buildCodexCLIArgs`, append supported feature disables before the prompt:

```go
if backend.Capabilities.Plugins == "disabled" {
    args = append(args, "--disable", "plugins")
}
if backend.Capabilities.Hooks == "disabled" {
    args = append(args, "--disable", "codex_hooks")
}
```

Only add feature names confirmed by current `codex features list` or existing CLI help. Do not invent MCP feature names.

- [ ] **Step 4: Update exact argv tests**

Update existing exact-argument expectations in `internal/runtime/actor_runtime_test.go` so they include the new feature-disable flags in the actual order produced by `buildCodexCLIArgs`.

Known affected tests currently assert full argv around:

- `TestRunCommandUsesBackendPolicyForCodexInvocation`
- JSON-event capture invocation tests
- shell environment policy invocation tests

- [ ] **Step 5: Verify invocation policy**

```bash
go test ./internal/runtime -run 'TestBuildCodexCLIArgs|TestRunCommandUsesBackendPolicyForCodexInvocation' -count=1
```

Expected: PASS.

## Task 3: Sealed Actor Materialization Policy

**Files:**
- Modify: `internal/runtime/actor_runtime_sealed.go`
- Test: `internal/runtime/actor_runtime_test.go`
- Test: `internal/runtime/event_audit_test.go`

- [ ] **Step 1: Add failing allowed-system-skill test**

Create a fake Codex script that writes:

```text
CODEX_HOME/skills/.system/.codex-system-skills.marker
CODEX_HOME/skills/.system/openai-docs/SKILL.md
```

and emits a valid output JSON file.

The test must configure the full sealed-runtime prerequisites before calling `runCommand`:

- write the fake Codex executable
- call `allowFakeCodexForTest(t, fakeCodexPath)`
- set `RAIL_CODEX_AUTH_HOME` to `testRailCodexAuthHome(t)`
- create output schema, last-message, events, artifact, and working directories inside the test temp root

Expected test behavior:

```go
_, err := runCommand(defaultTestActorBackend(), spec)
if err != nil {
    t.Fatalf("expected marked system skills to be allowed, got %v", err)
}
```

- [ ] **Step 2: Confirm failure**

```bash
go test ./internal/runtime -run TestRunCommandAllowsMarkedSystemSkillMaterialization -count=1
```

Expected: FAIL with `unexpected_skill_materialization`.

- [ ] **Step 3: Add forbidden materialization tests**

Cover:

- `CODEX_HOME/skills/injected/SKILL.md` -> `unexpected_skill_materialization`
- `CODEX_HOME/skills/.system/openai-docs/SKILL.md` without marker -> `unexpected_skill_materialization`
- `HOME/.codex/skills/injected/SKILL.md` -> `unexpected_skill_materialization`
- `CODEX_HOME/.tmp/plugins/...` with plugins disabled -> `unexpected_plugin_materialization`

- [ ] **Step 4: Implement narrow materialization inspection**

Replace the broad `directoryHasEntries(CODEX_HOME/skills)` check with a helper that reads direct entries under `CODEX_HOME/skills`.

Rules:

- Direct entry `.system` is allowed only when `.codex-system-skills.marker` exists inside it.
- Any other direct entry is forbidden.
- Permission errors remain policy failures.

Keep existing broad checks for user-home `.codex/*` and actor-local forbidden surfaces.

- [ ] **Step 5: Repeat across actor names**

Add a table-driven smoke with actor names:

```go
[]string{"planner", "context_builder", "critic", "generator", "executor", "evaluator"}
```

Expected: all actor names pass marked system skill materialization and still reject non-system injection.

- [ ] **Step 6: Run focused policy tests**

```bash
go test ./internal/runtime -run 'TestRunCommand.*Skill|TestRunCommand.*Plugin|TestPostflightSealedActorRuntime|TestAuditCodexEvents' -count=1
```

Expected: PASS.

## Task 4: Validation Target Contract

**Files:**
- Create or modify: `internal/request/validation_targets.go`
- Modify: `internal/request/normalize.go`
- Modify: `internal/contracts/validator.go`
- Test: `internal/request/normalize_test.go`
- Modify if needed: `internal/runtime/bootstrap.go`
- Test: `internal/runtime/bootstrap_test.go`
- Test: `internal/cli/app_test.go`
- Modify: `skills/rail/SKILL.md`
- Modify: `assets/skill/Rail/SKILL.md`

- [ ] **Step 1: Add failing shared validation tests**

Add a table-driven test for the shared validator:

```go
func TestValidateValidationTargetsRejectsCommandLikeValues(t *testing.T) {
    for _, target := range []string{
        "flutter analyze",
        "dart format --line-length 120",
        "go test ./...",
        "test/foo_test.dart && echo done",
    } {
        err := ValidateValidationTargets([]string{target})
        if err == nil || !strings.Contains(err.Error(), "validation_targets") {
            t.Fatalf("expected validation target rejection for %q, got %v", target, err)
        }
    }
}
```

Also keep file target acceptance:

```go
"test/features/battle/battle_rules_test.dart"
"lib/src/features/battle/domain/battle_engine.dart"
```

- [ ] **Step 2: Add failing compose-request normalization tests**

In request normalization tests, decode the draft before normalizing:

```go
for _, target := range []string{
    "flutter analyze",
    "dart format --line-length 120",
    "go test ./...",
    "test/foo_test.dart && echo done",
} {
    draft, decodeErr := DecodeDraft(strings.NewReader(fmt.Sprintf(`{
      "request_version": "1",
      "project_root": "/absolute/path/to/target-repo",
      "task_type": "test_repair",
      "goal": "validate target rejection",
      "context": {"validation_targets": [%q]},
      "definition_of_done": ["reject command-looking validation target"]
    }`, target)))
    if decodeErr != nil {
        t.Fatalf("decode draft: %v", decodeErr)
    }
    _, err := NormalizeDraft(draft)
    if err == nil || !strings.Contains(err.Error(), "validation_targets") {
        t.Fatalf("expected validation target rejection for %q, got %v", target, err)
    }
}
```

- [ ] **Step 3: Add failing validate-request/bootstrap tests**

Add coverage that does not use `NormalizeDraft`:

- `contracts.Validator.ValidateRequestFile` rejects a checked-in or temp request YAML with `context.validation_targets: ["go test ./..."]`.
- `Bootstrapper.Bootstrap` rejects the same request before writing an execution plan.
- CLI `validate-request` reports the validation error for command-like targets.

This closes the bypass where hand-written request YAML or direct runtime bootstrap skips draft normalization.

- [ ] **Step 4: Run request/contract/bootstrap tests and confirm failure**

```bash
go test ./internal/request ./internal/contracts ./internal/runtime ./internal/cli -run 'Test.*ValidationTargets|TestValidateRequest' -count=1
```

Expected: FAIL before implementation.

- [ ] **Step 5: Implement shared command-looking target rejection**

Reject targets containing shell operators or whitespace that makes them clearly command strings. Use conservative checks:

- contains `&&`, `||`, `;`, `|`, `<`, `>`
- starts with common command verbs followed by whitespace: `go `, `flutter `, `dart `, `npm `, `yarn `, `pnpm `, `pytest `, `cargo `
- contains ` --` as command-option syntax

Do not reject ordinary paths with spaces if the repo currently supports them elsewhere; if path-with-spaces support is ambiguous, reject with a clear error that validation targets must be project-relative file paths without shell syntax.

Call the shared validator from:

- `NormalizeDraft` for `compose-request`
- `parseCanonicalRequest` or `ValidateRequestFile` for existing YAML and `validate-request`
- `buildExecutionPlan` or bootstrap request loading as a fail-closed guard

- [ ] **Step 6: Update skill validation guidance**

In both skill copies, say:

```markdown
`context.validation_roots` and `context.validation_targets` are path hints, not shell commands. Put commands such as `flutter analyze`, `dart format --line-length 120`, or `go test ./...` in `definition_of_done` as validation expectations unless Rail has a first-class command field for them.
```

- [ ] **Step 7: Run request/bootstrap/install tests**

```bash
go test ./internal/request ./internal/contracts ./internal/runtime ./internal/cli ./internal/install -run 'Test.*ValidationTargets|TestBootstrap|TestValidateRequest|TestBundledSkill' -count=1
```

Expected: PASS.

## Task 5: Skill Task Identity Guidance

**Files:**
- Modify: `skills/rail/SKILL.md`
- Modify: `assets/skill/Rail/SKILL.md`
- Modify: `skills/rail/references/examples.md`
- Modify: `assets/skill/Rail/references/examples.md`
- Test: `internal/install/skill_test.go`

- [ ] **Step 1: Add failing skill contract assertions**

In `TestBundledSkillMatchesCurrentCLIWorkflow`, assert the bundled skill includes:

```go
for _, want := range []string{
    "## Task Identity Decision",
    "Start a fresh task when the user gives a new goal",
    "Continue an existing artifact only when the user asks to continue",
    "do not run `compose-request` or `rail run`",
    "Do not ask users to choose task ids",
    "Do not derive an artifact path from `.harness/requests/request.yaml`",
} {
    if !strings.Contains(skillDoc, want) {
        t.Fatalf("expected skill doc to include task identity guidance %q", want)
    }
}
```

Add negative assertions for stale guidance:

```go
for _, rejected := range []string{
    "ask the user for a task id",
    "reconstruct the artifact path from the request filename",
    "edit actor_backend.yaml to use an absolute codex path",
    "create a symlink to codex",
} {
    if strings.Contains(skillDoc, rejected) {
        t.Fatalf("expected skill doc to avoid stale guidance %q", rejected)
    }
}
```

- [ ] **Step 2: Keep repo-owned skill path tests aligned**

Keep `TestBundledSkillMatchesRepoOwnedSkillFiles` walking the actual repo-owned skill path `skills/rail`.

- [ ] **Step 3: Run install test and confirm failure**

```bash
go test ./internal/install -run 'TestBundledSkillMatchesCurrentCLIWorkflow|TestBundledSkillMatchesRepoOwnedSkillFiles' -count=1
```

Expected before skill edits: FAIL because task identity guidance is missing or path casing is stale.

- [ ] **Step 4: Add `Task Identity Decision` to skill copies**

Add this section after `Compose Request`:

```markdown
## Task Identity Decision

Before running commands, decide whether the user is starting fresh work or continuing an existing artifact.

Start a fresh task when the user gives a new goal, bug, feature, refactor, test repair, or other new natural-language work item. This remains true even when an earlier artifact is blocked or rejected. For fresh work, run `rail compose-request --stdin`, then `rail run --request <printed-request-path> --project-root <target-repo>` without `--task-id`, and use the artifact path printed by `rail run`.

Continue an existing artifact only when the user asks to continue, retry, supervise, inspect status/result, integrate, report, or debug an existing run, or when they provide an artifact path. For existing artifacts, do not run `compose-request` or `rail run`; use the supplied or previously printed artifact path with `rail supervise`, `rail status`, `rail result`, or `rail integrate`.

If the user references prior work but gives neither a new goal nor an artifact path, ask one concise clarification: "Should I start this as a fresh Rail task, or continue an existing artifact? If continuing, provide the artifact path."

Do not ask users to choose task ids. Do not derive an artifact path from `.harness/requests/request.yaml`; the only durable run identity is the artifact path printed by `rail run` or supplied by the user.
```

- [ ] **Step 5: Update execution flow text**

Change the fresh execution sequence to begin with:

```markdown
Use the Task Identity Decision section first. If the decision is fresh task, use this sequence:
```

Add existing artifact flow:

```markdown
If the decision is existing artifact, skip request composition and run the requested artifact command against the supplied or previously printed artifact path.
```

Update doctor guidance to use the target repo:

```markdown
Before standard actor execution, run `rail auth doctor --project-root <target-repo>`. Plain `rail auth doctor` only checks the current directory's runtime policy.
```

- [ ] **Step 6: Add examples**

In both examples files, add:

````markdown
## Task Identity Examples

Fresh task prompt:

```text
Use the Rail skill.
Target repo: /absolute/path/to/target-repo
Goal: Fix the checkout total rounding bug.
```

Expected behavior:

- treat this as fresh work
- materialize a request
- run `rail run` without `--task-id`
- use the printed artifact path for execution and reporting

Existing artifact prompt:

```text
Use the Rail skill.
Continue /absolute/path/to/target-repo/.harness/artifacts/request-2 and report the result.
```

Expected behavior:

- treat this as existing artifact work
- do not run `compose-request`
- do not run `rail run`
- run `rail status`, `rail supervise`, `rail result`, or `rail integrate` against the supplied artifact path as requested
````

- [ ] **Step 7: Verify skill parity**

```bash
cmp -s skills/rail/SKILL.md assets/skill/Rail/SKILL.md
cmp -s skills/rail/references/examples.md assets/skill/Rail/references/examples.md
go test ./internal/install -count=1
```

Expected: PASS.

## Task 6: MolyCard Failure Regression Flow

**Files:**
- Test: `internal/cli/auth_test.go`
- Test: `internal/runtime/actor_runtime_test.go`
- Optional fixture: `test/fixtures/standard_route/blocked_environment/`

- [ ] **Step 1: Add fake-Codex regression that mimics current self-materialization**

Fake Codex should:

- write marked `CODEX_HOME/skills/.system` files
- attempt no user-home reads
- emit clean JSON events
- write valid actor structured output

Expected: `runCommand` succeeds for at least planner.

- [ ] **Step 2: Add negative regression for plugin surfaces**

Fake Codex should inspect its argv. If `--disable plugins` is missing, it writes plugin files under actor-local `CODEX_HOME/.tmp/plugins`; if the flag is present, it does not write plugin files.

Expected: default backend invocation includes `--disable plugins`, so the fake does not materialize plugins and the actor can proceed. A separate direct postflight test that creates plugin files should still fail with `unexpected_plugin_materialization`.

- [ ] **Step 3: Add doctor regression**

`rail auth doctor` should fail when auth is configured but sealed actor command resolution fails.
`rail auth doctor --project-root <target-repo>` should fail when target-local actor backend policy uses an absolute command.

Expected: no `supervise` is needed to detect this class of environment issue.

- [ ] **Step 4: Run regression package tests**

```bash
go test ./internal/cli ./internal/runtime -count=1
```

Expected: PASS.

## Task 7: Documentation And Troubleshooting

**Files:**
- Optional modify: `README.md`
- Optional modify: `docs/tasks.md`
- Modify if needed: `docs/ARCHITECTURE.md`
- Modify if needed: `AGENTS.md`

- [ ] **Step 1: Inspect existing docs**

```bash
rg -n 'auth doctor|blocked_environment|artifact path|task id|validation_targets|actor runtime' README.md docs
```

Expected: identify whether a short note is needed.

- [ ] **Step 2: Add only the missing operator note**

If needed, document:

- New goals create fresh implicit artifacts.
- Existing artifact commands require an artifact path.
- `rail auth doctor` verifies both auth and actor runtime readiness.
- Standard actor execution uses `rail auth doctor --project-root <target-repo>` before `supervise`.
- `validation_targets` are path hints, not commands.
- Repo-owned skill path is `skills/rail`, while the bundled install asset path is `assets/skill/Rail`.

Do not add machine-specific paths.

- [ ] **Step 3: Run docs security lint**

```bash
rg -n -e '/U[s]ers/' -e '~[/]' -e '/h[o]me/' README.md docs skills/rail assets/skill/Rail
```

Expected: no matches from newly added content. Existing intentional placeholders should use `/absolute/path/to/...`.

## Task 8: Final Verification

**Files:**
- No additional files.

- [ ] **Step 1: Run focused suites**

```bash
go test ./internal/request ./internal/contracts ./internal/runtime ./internal/cli ./internal/install -count=1
```

Expected: PASS.

- [ ] **Step 2: Run full test suite**

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 3: Build CLI**

```bash
go build -o build/rail ./cmd/rail
```

Expected: exit 0.

- [ ] **Step 4: Check skill parity**

```bash
cmp -s skills/rail/SKILL.md assets/skill/Rail/SKILL.md
cmp -s skills/rail/references/examples.md assets/skill/Rail/references/examples.md
```

Expected: both commands exit 0.

- [ ] **Step 5: Check formatting and docs paths**

```bash
git diff --check
rg -n -e '/U[s]ers/' -e '~[/]' -e '/h[o]me/' README.md docs skills/rail assets/skill/Rail
```

Expected: `git diff --check` passes. Path lint has no matches from new content.

- [ ] **Step 6: Run release gate**

```bash
./tool/release_gate.sh all
```

Expected: PASS.

## Acceptance Criteria

- `rail auth doctor` no longer reports ready when the sealed actor cannot resolve `codex`.
- `rail auth doctor --project-root <target-repo>` catches target-local unsafe actor backend policy before `supervise`.
- Doctor runtime-readiness failures do not expose user home, auth home, raw PATH, or full project root values.
- Fresh user goals always use `rail run` without `--task-id` and use the printed artifact path.
- Existing artifact requests never compose a new request or run `rail run`.
- Blocked prior artifacts no longer force unrelated new goals to resume the blocked artifact.
- Marked actor-local system skills are allowed.
- User-home skills/plugins and actor-local non-system skills remain blocked.
- Disabled plugin capability is reflected in Codex CLI invocation, with no broad plugin allowlist.
- Command-like `validation_targets` are rejected with clear guidance.
- Repo-owned and bundled Rail skill files/examples remain aligned.
- Focused tests, full tests, build, and release gate pass.

## Commit Plan

Use focused commits in this order:

1. `fix(runtime): preflight codex_vault command in auth doctor`
2. `fix(runtime): allow marked system skills in sealed actor home`
3. `fix(request): reject command-like validation targets`
4. `docs(skill): clarify rail task identity flow`
5. `test(runtime): cover molycard actor runtime blockers`
