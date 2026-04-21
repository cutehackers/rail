# Codex Boundary and Actor Backend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Rail's Codex execution boundary explicit by introducing a reviewable actor backend policy, safer Codex CLI defaults, and persisted Codex runtime evidence while preserving Rail's existing actor artifact contracts.

**Architecture:** Keep Rail as the governance layer that owns request contracts, workflow state, artifacts, evaluator routing, and validation evidence. Move Codex CLI invocation details into a typed actor backend policy and command builder so Codex remains the agent runtime while Rail configures, validates, and audits its use.

**Tech Stack:** Go CLI runtime in `internal/runtime`, embedded defaults under `assets/defaults`, project-local harness policy under `.harness/supervisor`, markdown docs in `README.md` and `docs/ARCHITECTURE.md`, Go tests with `go test ./internal/runtime`

---

## File Structure

- Create `.harness/supervisor/actor_backend.yaml`: checked-in source-repo actor backend policy for contributors and parity tests.
- Create `assets/defaults/supervisor/actor_backend.yaml`: embedded product default used when a target repo has no local override.
- Create `internal/runtime/actor_backend.go`: typed backend policy loader, validation, and default backend selection.
- Create `internal/runtime/actor_backend_test.go`: unit coverage for embedded/local loading, unsafe sandbox rejection, and command-shaping defaults.
- Modify `internal/runtime/actor_runtime.go`: replace hard-coded Codex CLI flags with backend-driven command construction and optional JSON event capture.
- Modify `internal/runtime/actor_runtime_test.go`: update existing fake Codex tests and add backend flag/event assertions.
- Modify `internal/runtime/runner.go`: load backend policy during real execution and pass it into actor runs.
- Modify `internal/runtime/integration.go`: use the same backend policy for the post-pass integrator actor.
- Modify `README.md`, `docs/ARCHITECTURE.md`, and `docs/ARCHITECTURE-kr.md`: document Codex as runtime and Rail as governance control plane.

Do not add Flutter, Node, Python, or other platform profile work in this plan.

### Task 1: Add Actor Backend Policy Loading

**Files:**
- Create: `.harness/supervisor/actor_backend.yaml`
- Create: `assets/defaults/supervisor/actor_backend.yaml`
- Create: `internal/runtime/actor_backend.go`
- Create: `internal/runtime/actor_backend_test.go`

- [ ] **Step 1: Write failing tests for backend policy loading**

Create `internal/runtime/actor_backend_test.go` with tests for embedded defaults, project-local overrides, and sandbox validation:

```go
func TestLoadActorBackendPolicyUsesEmbeddedDefaults(t *testing.T) {
	policy, err := loadActorBackendPolicy(t.TempDir())
	if err != nil {
		t.Fatalf("loadActorBackendPolicy returned error: %v", err)
	}
	backend, err := policy.DefaultBackend()
	if err != nil {
		t.Fatalf("DefaultBackend returned error: %v", err)
	}
	if backend.Sandbox != "workspace-write" {
		t.Fatalf("unexpected sandbox: got %q want workspace-write", backend.Sandbox)
	}
	if backend.ApprovalPolicy != "never" {
		t.Fatalf("unexpected approval policy: got %q want never", backend.ApprovalPolicy)
	}
	if !backend.CaptureJSONEvents {
		t.Fatalf("expected embedded backend to capture json events")
	}
}

func TestLoadActorBackendPolicyRejectsUnsafeLocalFullAccess(t *testing.T) {
	projectRoot := writeActorBackendPolicyFixture(t, `
version: 1
execution_environment: local
default_backend: codex_cli
backends:
  codex_cli:
    command: codex
    subcommand: exec
    sandbox: danger-full-access
    approval_policy: never
    session_mode: per_actor
    ephemeral: true
    capture_json_events: true
    skip_git_repo_check: true
execution_environments:
  local:
    allowed_sandboxes: [workspace-write]
`)
	_, err := loadActorBackendPolicy(projectRoot)
	if err == nil {
		t.Fatalf("expected unsafe local full-access policy to fail")
	}
	if !strings.Contains(err.Error(), "sandbox danger-full-access is not allowed") {
		t.Fatalf("unexpected error: %v", err)
	}
}
```

Run: `go test ./internal/runtime -run TestLoadActorBackendPolicy -count=1`

Expected: FAIL because the loader and fixtures do not exist yet.

- [ ] **Step 2: Add backend policy fixtures**

Create both `.harness/supervisor/actor_backend.yaml` and `assets/defaults/supervisor/actor_backend.yaml`:

```yaml
version: 1
execution_environment: local
default_backend: codex_cli

backends:
  codex_cli:
    command: codex
    subcommand: exec
    sandbox: workspace-write
    approval_policy: never
    session_mode: per_actor
    ephemeral: true
    capture_json_events: true
    skip_git_repo_check: true

execution_environments:
  local:
    allowed_sandboxes:
      - workspace-write
  isolated_ci:
    allowed_sandboxes:
      - workspace-write
      - danger-full-access
  docker:
    allowed_sandboxes:
      - workspace-write
      - danger-full-access
```

- [ ] **Step 3: Implement typed backend policy loading**

Create `internal/runtime/actor_backend.go` with:

```go
type ActorBackendPolicy struct {
	Version              int                                `yaml:"version"`
	ExecutionEnvironment string                             `yaml:"execution_environment"`
	DefaultBackendName   string                             `yaml:"default_backend"`
	Backends             map[string]ActorBackendConfig      `yaml:"backends"`
	Environments         map[string]ActorBackendEnvironment `yaml:"execution_environments"`
}

type ActorBackendConfig struct {
	Command           string `yaml:"command"`
	Subcommand        string `yaml:"subcommand"`
	Sandbox           string `yaml:"sandbox"`
	ApprovalPolicy    string `yaml:"approval_policy"`
	SessionMode       string `yaml:"session_mode"`
	Ephemeral         bool   `yaml:"ephemeral"`
	CaptureJSONEvents bool   `yaml:"capture_json_events"`
	SkipGitRepoCheck  bool   `yaml:"skip_git_repo_check"`
}

type ActorBackendEnvironment struct {
	AllowedSandboxes []string `yaml:"allowed_sandboxes"`
}
```

Implement `loadActorBackendPolicy(projectRoot string) (ActorBackendPolicy, error)` using `assets.Resolve(projectRoot, ".harness/supervisor/actor_backend.yaml")`. Validate:

- `version == 1`
- `execution_environment` is present
- `default_backend` exists
- only `codex_cli` is supported for now
- `command == codex`
- `subcommand == exec`
- `sandbox` is `read-only`, `workspace-write`, or `danger-full-access`
- `approval_policy` is `untrusted`, `on-request`, or `never`
- `session_mode == per_actor`
- selected sandbox is listed in the selected environment's `allowed_sandboxes`

- [ ] **Step 4: Run backend loader tests**

Run: `go test ./internal/runtime -run TestLoadActorBackendPolicy -count=1`

Expected: PASS.

- [ ] **Step 5: Commit backend policy loading**

```bash
git add .harness/supervisor/actor_backend.yaml assets/defaults/supervisor/actor_backend.yaml internal/runtime/actor_backend.go internal/runtime/actor_backend_test.go
git commit -m "feat: add actor backend policy"
```

### Task 2: Build Codex CLI Commands From Backend Policy

**Files:**
- Modify: `internal/runtime/actor_runtime.go`
- Modify: `internal/runtime/actor_runtime_test.go`

- [ ] **Step 1: Write failing tests for backend-derived Codex flags**

In `internal/runtime/actor_runtime_test.go`, add a test that installs a fake `codex`, invokes the actor runtime with a backend policy, and records `sys.argv`.

Assert the invocation contains:

- `exec`
- `-m gpt-5.4-mini`
- `--cd <workingDirectory>`
- `--ephemeral` when configured
- `--color never`
- `-s workspace-write`
- `--skip-git-repo-check` when configured
- `-c model_reasoning_effort="high"`
- `-c approval_policy="never"`
- `--output-schema <schemaPath>`
- `--output-last-message <logPath>`
- `--json` when `capture_json_events` is true

Run: `go test ./internal/runtime -run TestRunCommandUsesActorBackendPolicy -count=1`

Expected: FAIL because `runCommand` still hard-codes its Codex flags and has no backend argument.

- [ ] **Step 2: Introduce a small command-spec boundary**

In `internal/runtime/actor_runtime.go`, add:

```go
type ActorCommandSpec struct {
	ActorName        string
	Profile          ActorProfile
	WorkingDirectory string
	Prompt           string
	LastMessagePath  string
	SchemaPath       string
	EventsPath       string
}
```

Add a helper:

```go
func buildCodexCLIArgs(backend ActorBackendConfig, spec ActorCommandSpec) []string
```

This helper should contain all flag construction. Keep it pure so tests can assert exact args without starting a process.

- [ ] **Step 3: Refactor `runCommand` to accept backend policy**

Change the signature from:

```go
func runCommand(actorName string, profile ActorProfile, workingDirectory string, prompt string, logPath string, schemaPath string) (map[string]any, error)
```

to:

```go
func runCommand(backend ActorBackendConfig, spec ActorCommandSpec) (map[string]any, error)
```

Keep actor profile normalization, watchdog behavior, schema output reading, and JSON decoding intact.

- [ ] **Step 4: Add optional JSON event capture**

When `backend.CaptureJSONEvents` is true:

- add `--json` to the Codex CLI args
- create `spec.EventsPath`
- write Codex stdout to both the existing progress buffer and the events file
- keep stderr in the progress buffer so failures still report useful output

Do not parse the JSONL stream in this task. Persisting it is enough.

- [ ] **Step 5: Update existing actor runtime tests**

Update existing `runCommand` tests to pass:

```go
backend := ActorBackendConfig{
	Command:           "codex",
	Subcommand:        "exec",
	Sandbox:           "workspace-write",
	ApprovalPolicy:    "never",
	SessionMode:       "per_actor",
	Ephemeral:         true,
	CaptureJSONEvents: false,
	SkipGitRepoCheck:  true,
}
spec := ActorCommandSpec{
	ActorName:        "planner",
	Profile:          ActorProfile{Model: "gpt-5.4-mini", Reasoning: "high"},
	WorkingDirectory: workingDirectory,
	Prompt:           "prompt",
	LastMessagePath:  logPath,
	SchemaPath:       schemaPath,
}
```

Run: `go test ./internal/runtime -run 'TestRunCommand|TestActorOutputJSONSchema' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit command construction refactor**

```bash
git add internal/runtime/actor_runtime.go internal/runtime/actor_runtime_test.go
git commit -m "feat: drive codex actor commands from backend policy"
```

### Task 3: Wire Backend Policy Through Runner and Integrator

**Files:**
- Modify: `internal/runtime/runner.go`
- Modify: `internal/runtime/runner_test.go`
- Modify: `internal/runtime/integration.go`
- Modify: `internal/runtime/integration_test.go`

- [ ] **Step 1: Add failing runner coverage for safer default sandbox**

Update the fake Codex script used by real-mode runner tests so it records the sandbox flag and `--json` flag in `.actor-log`.

Assert each real actor invocation uses `workspace-write`, not `danger-full-access`, and includes `--json`.

Run: `go test ./internal/runtime -run TestExecuteRunsRealActorPathThroughCodex -count=1`

Expected: FAIL until the runner loads and passes backend policy.

- [ ] **Step 2: Load backend policy once per execution**

In `Runner.Execute`, after resolving `workingDirectory`, call:

```go
backendPolicy, err := loadActorBackendPolicy(workingDirectory)
if err != nil {
	return "", fmt.Errorf("load actor backend policy: %w", err)
}
backend, err := backendPolicy.DefaultBackend()
if err != nil {
	return "", err
}
```

Pass `backend` into `r.runActor`.

- [ ] **Step 3: Materialize event paths in actor runs**

In `Runner.runActor`, compute:

```go
eventsPath := filepath.Join(runsDirectory, actorEventFileName(actorIndex, actorName, currentState.CompletedActors))
```

Add an `actorEventFileName` helper parallel to `actorLogFileName`, using names such as:

```text
03_critic-events.jsonl
03_critic-visit-02-events.jsonl
```

Pass `EventsPath` to `runCommand`.

- [ ] **Step 4: Update integrator backend wiring**

In `Runner.Integrate`, load the backend policy from the effective integration project root and pass it into `runIntegratorActor`.

Change:

```go
func runIntegratorActor(workingDirectory string, profile ActorProfile, ...)
```

to accept `backend ActorBackendConfig` and call `runCommand(backend, ActorCommandSpec{...})`.

- [ ] **Step 5: Add unsafe policy failure coverage**

Add a runner test that writes this project-local backend policy into the prepared target repo:

```yaml
version: 1
execution_environment: local
default_backend: codex_cli
backends:
  codex_cli:
    command: codex
    subcommand: exec
    sandbox: danger-full-access
    approval_policy: never
    session_mode: per_actor
    ephemeral: true
    capture_json_events: true
    skip_git_repo_check: true
execution_environments:
  local:
    allowed_sandboxes:
      - workspace-write
```

Assert `runner.Execute(artifactPath)` fails before invoking fake Codex, and the error mentions `sandbox danger-full-access is not allowed`.

- [ ] **Step 6: Run runtime integration tests**

Run: `go test ./internal/runtime -run 'TestExecuteRunsRealActorPathThroughCodex|TestExecuteUsesWorkflowProjectRootActorProfiles|TestIntegrate' -count=1`

Expected: PASS.

- [ ] **Step 7: Commit runner/integrator wiring**

```bash
git add internal/runtime/runner.go internal/runtime/runner_test.go internal/runtime/integration.go internal/runtime/integration_test.go
git commit -m "feat: wire actor backend policy through runtime"
```

### Task 4: Document the Codex Runtime Boundary

**Files:**
- Modify: `README.md`
- Modify: `docs/ARCHITECTURE.md`
- Modify: `docs/ARCHITECTURE-kr.md`

- [ ] **Step 1: Update README product boundary language**

In `README.md`, keep the current skill-first contract and add a short section after the product contract:

```markdown
## Codex Runtime Boundary

Rail is the governance control plane. Codex remains the agent runtime.

Rail owns request normalization, bounded workflow state, artifact contracts,
evaluation routing, validation evidence, and reviewed learning state. Codex
owns repository inspection, file editing, tool execution, sandbox enforcement,
rules, skills, hooks, and structured final actor output.
```

Also update the advanced notes to mention `.harness/supervisor/actor_backend.yaml`.

- [ ] **Step 2: Update English architecture docs**

In `docs/ARCHITECTURE.md`, update the runtime model and core components sections so they state:

- Rail configures Codex execution through actor backend policy.
- Codex CLI is the first backend.
- backend execution evidence is persisted under artifact `runs/`.
- local default sandbox is `workspace-write`.

- [ ] **Step 3: Update Korean architecture docs**

Mirror the same meaning in `docs/ARCHITECTURE-kr.md`. Keep examples generic and avoid any user home-directory path.

- [ ] **Step 4: Check documentation path lint manually**

Run:

```bash
python3 - <<'PY'
from pathlib import Path

needles = ["/" + "Users/", "~" + "/"]
for path in [Path("README.md"), Path("docs/ARCHITECTURE.md"), Path("docs/ARCHITECTURE-kr.md")]:
    for line_number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
        if any(needle in line for needle in needles):
            print(f"{path}:{line_number}:{line}")
PY
```

Expected: no matches.

- [ ] **Step 5: Commit documentation**

```bash
git add README.md docs/ARCHITECTURE.md docs/ARCHITECTURE-kr.md
git commit -m "docs: clarify rail codex runtime boundary"
```

### Task 5: Full Verification and Release Gate Check

**Files:**
- No planned source edits unless verification exposes a defect.

- [ ] **Step 1: Run focused runtime tests**

Run:

```bash
go test ./internal/runtime -count=1
```

Expected: PASS.

- [ ] **Step 2: Run full Go test suite**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 3: Build Rail**

Run:

```bash
go build -o build/rail ./cmd/rail
```

Expected: PASS and `build/rail` exists.

- [ ] **Step 4: Run the v2 release gate if runtime changes are complete**

Run:

```bash
./tool/v2_release_gate.sh
```

Expected: PASS. If this fails because local Codex or environment setup is unavailable, capture the exact failure and run the narrowest equivalent deterministic smoke checks.

- [ ] **Step 5: Inspect final diff**

Run:

```bash
git status --short
git diff --stat HEAD
```

Expected: only intentional backend policy, runtime, test, and documentation changes remain.

- [ ] **Step 6: Commit any verification fixes**

If verification required fixes:

```bash
git add <changed-files>
git commit -m "fix: stabilize actor backend verification"
```

If no fixes were needed, do not create an empty commit.

## Implementation Notes

- Keep actor final output schemas unchanged.
- Keep smoke profile behavior deterministic.
- Do not introduce platform profiles in this work.
- Do not move executor validation into Codex in this work.
- Do not add deep-merge semantics for `.harness` policy overrides.
- Keep project-local override precedence consistent with existing asset resolution.

## Handoff

Start with Task 1. Each task should be independently testable and committed before moving to the next task. If a later task reveals that the backend policy schema needs a small adjustment, update the spec or record the decision in the relevant commit message before continuing.
