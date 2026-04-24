# Sealed Actor Runtime and Workflow Continuity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Rail actor execution independent from the user's normal Codex session and make workflow progress resumable from Rail artifacts rather than agent memory.

**Architecture:** Extend the existing actor backend policy with actor isolation flags, capability policy, clean subprocess environment handling, event audit, and continuity artifacts. Keep Codex as the agent runtime while Rail owns the policy, state, evidence, and final reporting contract.

**Tech Stack:** Go runtime in `internal/runtime`, YAML artifacts under `.harness/`, embedded defaults under `assets/defaults`, Markdown docs in `docs/`, tests with `go test ./internal/runtime` and `go test ./...`.

---

## File Structure

- Modify `.harness/supervisor/actor_backend.yaml`: add actor isolation flags and default capability policy for the source repo harness.
- Modify `assets/defaults/supervisor/actor_backend.yaml`: keep packaged defaults aligned with the source repo harness.
- Modify `internal/runtime/actor_backend.go`: extend `ActorBackendConfig`, validate isolation flags and capability policy.
- Modify `internal/runtime/actor_backend_test.go`: cover embedded defaults, local overrides, and invalid capability policy.
- Modify `internal/runtime/actor_runtime.go`: add isolated Codex CLI flags, build a clean actor environment, and keep event capture compatible.
- Modify `internal/runtime/actor_runtime_test.go`: assert isolated args and clean environment behavior with fake Codex.
- Create `internal/runtime/event_audit.go`: inspect Codex JSONL event streams for unexpected user-surface injection.
- Create `internal/runtime/event_audit_test.go`: fixture-driven audit tests for skill/plugin/MCP/hook violations.
- Modify `internal/runtime/runner.go`: run event audit after each Codex-backed actor and fail before routing if policy is violated.
- Modify `internal/runtime/bootstrap.go`: materialize continuity artifacts and add them to actor briefs.
- Modify `internal/runtime/bootstrap_test.go`: assert continuity artifacts and brief references are created.
- Create `internal/runtime/continuity.go`: own continuity artifact default content and update helpers.
- Create `internal/runtime/continuity_test.go`: unit coverage for `next_action.yaml`, `evidence.yaml`, and `work_ledger.md` updates.
- Modify `internal/runtime/router.go`: update continuity artifacts when evaluator routes pass, revise, reject, or block.
- Modify `internal/reporting/terminal_summary.go`: reflect final answer contract and policy violations in terminal summaries.
- Modify `internal/reporting/terminal_summary_test.go`: prevent unsupported success claims when evidence or policy is missing.
- Modify `docs/ARCHITECTURE.md` and `docs/ARCHITECTURE-kr.md`: explain user-facing Codex versus sealed actor Codex.

Do not change downstream application code. Do not change request draft UX in the Rail skill unless implementation proves the skill must describe new output behavior.

## Task 1: Add Immediate Actor Isolation Flags

**Files:**
- Modify: `.harness/supervisor/actor_backend.yaml`
- Modify: `assets/defaults/supervisor/actor_backend.yaml`
- Modify: `internal/runtime/actor_backend.go`
- Modify: `internal/runtime/actor_backend_test.go`
- Modify: `internal/runtime/actor_runtime.go`
- Modify: `internal/runtime/actor_runtime_test.go`

- [x] **Step 1: Write failing backend policy tests**

Add a test to `internal/runtime/actor_backend_test.go` that loads embedded defaults and asserts the new flags:

```go
func TestLoadActorBackendPolicyDefaultsToIsolatedCodexSession(t *testing.T) {
	policy, err := loadActorBackendPolicy(t.TempDir())
	if err != nil {
		t.Fatalf("loadActorBackendPolicy returned error: %v", err)
	}
	backend, err := policy.DefaultBackend()
	if err != nil {
		t.Fatalf("DefaultBackend returned error: %v", err)
	}
	if !backend.IgnoreUserConfig {
		t.Fatal("expected actor backend to ignore user config by default")
	}
	if !backend.IgnoreRules {
		t.Fatal("expected actor backend to ignore user rules by default")
	}
}
```

Run: `go test ./internal/runtime -run TestLoadActorBackendPolicyDefaultsToIsolatedCodexSession -count=1`

Expected: FAIL because `IgnoreUserConfig` and `IgnoreRules` do not exist yet.

- [x] **Step 2: Add policy fields**

Extend `ActorBackendConfig` in `internal/runtime/actor_backend.go`:

```go
IgnoreUserConfig bool `yaml:"ignore_user_config"`
IgnoreRules      bool `yaml:"ignore_rules"`
```

No defaulting should happen in Go for now. The embedded and source policies must explicitly set both fields.

- [x] **Step 3: Update backend policy YAML**

Add these keys to both `.harness/supervisor/actor_backend.yaml` and `assets/defaults/supervisor/actor_backend.yaml` under `backends.codex_cli`:

```yaml
    ignore_user_config: true
    ignore_rules: true
```

- [x] **Step 4: Add CLI args test**

Update `TestRunCommandUsesBackendPolicyForCodexInvocation` in `internal/runtime/actor_runtime_test.go` so expected args include:

```go
"--ignore-user-config",
"--ignore-rules",
```

Place them near other Codex execution mode flags after `--skip-git-repo-check`.

Run: `go test ./internal/runtime -run TestRunCommandUsesBackendPolicyForCodexInvocation -count=1`

Expected: FAIL until `buildCodexCLIArgs` is updated.

- [x] **Step 5: Implement CLI arg construction**

Modify `buildCodexCLIArgs` in `internal/runtime/actor_runtime.go`:

```go
if backend.IgnoreUserConfig {
	args = append(args, "--ignore-user-config")
}
if backend.IgnoreRules {
	args = append(args, "--ignore-rules")
}
```

Keep this in the pure arg builder so fake Codex tests can prove the exact command shape.

- [x] **Step 6: Run focused tests**

Run:

```bash
go test ./internal/runtime -run 'TestLoadActorBackendPolicyDefaultsToIsolatedCodexSession|TestRunCommandUsesBackendPolicyForCodexInvocation' -count=1
```

Expected: PASS.

- [x] **Step 7: Commit immediate isolation flags**

```bash
git add .harness/supervisor/actor_backend.yaml assets/defaults/supervisor/actor_backend.yaml internal/runtime/actor_backend.go internal/runtime/actor_backend_test.go internal/runtime/actor_runtime.go internal/runtime/actor_runtime_test.go
git commit -m "feat: isolate codex actor sessions"
```

## Task 2: Add Actor Capability Policy

**Files:**
- Modify: `.harness/supervisor/actor_backend.yaml`
- Modify: `assets/defaults/supervisor/actor_backend.yaml`
- Modify: `internal/runtime/actor_backend.go`
- Modify: `internal/runtime/actor_backend_test.go`

- [x] **Step 1: Write failing capability default test**

Add this test to `internal/runtime/actor_backend_test.go`:

```go
func TestLoadActorBackendPolicyDefaultsToRestrictedCapabilities(t *testing.T) {
	policy, err := loadActorBackendPolicy(t.TempDir())
	if err != nil {
		t.Fatalf("loadActorBackendPolicy returned error: %v", err)
	}
	backend, err := policy.DefaultBackend()
	if err != nil {
		t.Fatalf("DefaultBackend returned error: %v", err)
	}
	want := ActorBackendCapabilities{
		UserSkills:  "disabled",
		UserRules:   "disabled",
		Plugins:     "disabled",
		MCP:         "disabled",
		Hooks:       "disabled",
		Shell:       "allowed",
		FileEditing: "allowed",
	}
	if backend.Capabilities != want {
		t.Fatalf("unexpected capabilities: got %#v want %#v", backend.Capabilities, want)
	}
}
```

Run: `go test ./internal/runtime -run TestLoadActorBackendPolicyDefaultsToRestrictedCapabilities -count=1`

Expected: FAIL because `ActorBackendCapabilities` does not exist yet.

- [x] **Step 2: Add capability struct**

Add to `internal/runtime/actor_backend.go`:

```go
type ActorBackendCapabilities struct {
	UserSkills  string `yaml:"user_skills"`
	UserRules   string `yaml:"user_rules"`
	Plugins     string `yaml:"plugins"`
	MCP         string `yaml:"mcp"`
	Hooks       string `yaml:"hooks"`
	Shell       string `yaml:"shell"`
	FileEditing string `yaml:"file_editing"`
}
```

Add to `ActorBackendConfig`:

```go
Capabilities ActorBackendCapabilities `yaml:"capabilities"`
```

- [x] **Step 3: Update policy YAML**

Add to both backend policy files:

```yaml
    capabilities:
      user_skills: disabled
      user_rules: disabled
      plugins: disabled
      mcp: disabled
      hooks: disabled
      shell: allowed
      file_editing: allowed
```

- [x] **Step 4: Validate capability values**

Add validation in `validateActorBackendConfig`:

```go
func validateActorBackendCapabilities(config ActorBackendConfig) error {
	disabledOnly := map[string]string{
		"user_skills": config.Capabilities.UserSkills,
		"user_rules":  config.Capabilities.UserRules,
		"plugins":     config.Capabilities.Plugins,
		"mcp":         config.Capabilities.MCP,
		"hooks":       config.Capabilities.Hooks,
	}
	for name, value := range disabledOnly {
		if value != "disabled" {
			return fmt.Errorf("actor backend capability %s must be disabled, got %q", name, value)
		}
	}
	if config.Capabilities.Shell != "allowed" {
		return fmt.Errorf("actor backend capability shell must be allowed, got %q", config.Capabilities.Shell)
	}
	if config.Capabilities.FileEditing != "allowed" {
		return fmt.Errorf("actor backend capability file_editing must be allowed, got %q", config.Capabilities.FileEditing)
	}
	return nil
}
```

Call it from `validateActorBackendConfig`.

- [x] **Step 5: Add invalid override test**

Add a local policy fixture that sets `plugins: allowed` and assert load failure includes `capability plugins must be disabled`.

Run:

```bash
go test ./internal/runtime -run 'TestLoadActorBackendPolicy.*Capabilities' -count=1
```

Expected: PASS.

- [x] **Step 6: Commit capability policy**

```bash
git add .harness/supervisor/actor_backend.yaml assets/defaults/supervisor/actor_backend.yaml internal/runtime/actor_backend.go internal/runtime/actor_backend_test.go
git commit -m "feat: add actor capability policy"
```

## Task 3: Build a Clean Actor Environment

**Files:**
- Modify: `internal/runtime/actor_runtime.go`
- Modify: `internal/runtime/actor_runtime_test.go`

- [x] **Step 1: Write failing environment builder test**

Add this test to `internal/runtime/actor_runtime_test.go`:

```go
func TestBuildActorEnvironmentDropsUserCodexSurface(t *testing.T) {
	env := buildActorEnvironment([]string{
		"PATH=/usr/bin",
		"CODEX_HOME=/tmp/user-codex",
		"RAIL_TEST_INVOCATION_PATH=/tmp/invocation.json",
		"HOME=/tmp/home",
	})
	joined := strings.Join(env, "\n")
	if !strings.Contains(joined, "PATH=/usr/bin") {
		t.Fatalf("expected PATH to be preserved, got %v", env)
	}
	if strings.Contains(joined, "CODEX_HOME=") {
		t.Fatalf("expected CODEX_HOME to be removed, got %v", env)
	}
	if strings.Contains(joined, "HOME=") {
		t.Fatalf("expected HOME to be removed from actor env, got %v", env)
	}
	if !strings.Contains(joined, "RAIL_TEST_INVOCATION_PATH=/tmp/invocation.json") {
		t.Fatalf("expected test harness env to be preserved for fake codex tests, got %v", env)
	}
}
```

Run: `go test ./internal/runtime -run TestBuildActorEnvironmentDropsUserCodexSurface -count=1`

Expected: FAIL because `buildActorEnvironment` does not exist yet.

- [x] **Step 2: Implement environment builder**

Add `buildActorEnvironment(parent []string) []string` to `internal/runtime/actor_runtime.go`.

Start conservatively:

```go
func buildActorEnvironment(parent []string) []string {
	allowedPrefixes := []string{
		"PATH=",
		"RAIL_TEST_",
	}
	env := make([]string, 0, len(parent))
	for _, entry := range parent {
		for _, prefix := range allowedPrefixes {
			if strings.HasPrefix(entry, prefix) {
				env = append(env, entry)
				break
			}
		}
	}
	return env
}
```

This intentionally keeps the first version small. Auth-specific handling can be added only after there is a concrete failing real-run requirement.

- [x] **Step 3: Wire environment into actor command**

In `runCommand`, set:

```go
cmd.Env = buildActorEnvironment(os.Environ())
```

Place this after `cmd.Dir = spec.WorkingDirectory`.

- [x] **Step 4: Preserve fake Codex tests**

Run:

```bash
go test ./internal/runtime -run 'TestBuildActorEnvironmentDropsUserCodexSurface|TestRunCommandUsesBackendPolicyForCodexInvocation' -count=1
```

Expected: PASS.

- [x] **Step 5: Commit clean actor environment**

```bash
git add internal/runtime/actor_runtime.go internal/runtime/actor_runtime_test.go
git commit -m "feat: clean codex actor environment"
```

## Task 4: Add Codex Event Audit

**Files:**
- Create: `internal/runtime/event_audit.go`
- Create: `internal/runtime/event_audit_test.go`
- Modify: `internal/runtime/runner.go`
- Modify: `internal/runtime/integration.go`

- [x] **Step 1: Write failing audit tests**

Create `internal/runtime/event_audit_test.go`:

```go
func TestAuditCodexEventsRejectsUnexpectedSkillInjection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(path, []byte(`{"type":"item.started","item":{"type":"command_execution","command":"sed -n '1,220p' /tmp/codex/superpowers/skills/using-superpowers/SKILL.md"}}`+"\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	err := auditCodexEvents(path)
	if err == nil {
		t.Fatal("expected event audit to reject unexpected skill injection")
	}
	if !strings.Contains(err.Error(), "unexpected_skill_injection") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAuditCodexEventsAllowsBasicThreadEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(path, []byte(`{"type":"thread.started","thread_id":"test"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := auditCodexEvents(path); err != nil {
		t.Fatalf("expected basic event stream to pass, got %v", err)
	}
}
```

Run: `go test ./internal/runtime -run TestAuditCodexEvents -count=1`

Expected: FAIL because `auditCodexEvents` does not exist yet.

- [x] **Step 2: Implement audit function**

Create `internal/runtime/event_audit.go`.

The initial audit can inspect each JSONL line as text. Use narrow patterns first:

```go
var codexEventViolationPatterns = []struct {
	Code    string
	Pattern string
}{
	{Code: "unexpected_skill_injection", Pattern: "/skills/"},
	{Code: "unexpected_skill_injection", Pattern: "superpowers/skills"},
	{Code: "unexpected_plugin_load", Pattern: ".codex/.tmp/plugins"},
}
```

Return an error like:

```go
fmt.Errorf("backend_policy_violation: %s in %s", match.Code, path)
```

Do not overfit to a user-specific home path.

- [x] **Step 3: Audit after actor command**

In `Runner.runActor`, after `runCommand` succeeds for Codex-backed actors and before returning the response, call:

```go
if backend.CaptureJSONEvents {
	if err := auditCodexEvents(eventsPath); err != nil {
		return nil, err
	}
}
```

Keep smoke actors unchanged.

- [x] **Step 4: Audit integrator events**

In `internal/runtime/integration.go`, run the same audit after the integrator actor command when JSON events are captured.

- [x] **Step 5: Run focused tests**

Run:

```bash
go test ./internal/runtime -run 'TestAuditCodexEvents|TestRunCommandUsesBackendPolicyForCodexInvocation' -count=1
```

Expected: PASS.

- [x] **Step 6: Commit event audit**

```bash
git add internal/runtime/event_audit.go internal/runtime/event_audit_test.go internal/runtime/runner.go internal/runtime/integration.go
git commit -m "feat: audit codex actor events"
```

## Task 5: Materialize Continuity Artifacts

**Files:**
- Create: `internal/runtime/continuity.go`
- Create: `internal/runtime/continuity_test.go`
- Modify: `internal/runtime/bootstrap.go`
- Modify: `internal/runtime/bootstrap_test.go`

- [x] **Step 1: Write failing bootstrap test**

In `internal/runtime/bootstrap_test.go`, extend the artifact skeleton test to assert these files exist:

```go
for _, relPath := range []string{
	"work_ledger.md",
	"next_action.yaml",
	"evidence.yaml",
	"final_answer_contract.yaml",
} {
	if _, err := os.Stat(filepath.Join(artifactPath, relPath)); err != nil {
		t.Fatalf("expected continuity artifact %q to exist: %v", relPath, err)
	}
}
```

Run: `go test ./internal/runtime -run TestBootstrapCreatesExpectedArtifactSkeleton -count=1`

Expected: FAIL because continuity artifacts are not created yet.

- [x] **Step 2: Add continuity helpers**

Create `internal/runtime/continuity.go` with constants:

```go
const (
	workLedgerFileName          = "work_ledger.md"
	nextActionFileName          = "next_action.yaml"
	evidenceFileName            = "evidence.yaml"
	finalAnswerContractFileName = "final_answer_contract.yaml"
)
```

Add `initialWorkLedger(workflow Workflow) string`, `initialNextAction(workflow Workflow) map[string]any`, `initialEvidence() map[string]any`, and `initialFinalAnswerContract() map[string]any`.

- [x] **Step 3: Materialize files during bootstrap**

In `Bootstrapper.Bootstrap`, after placeholder outputs are created, write:

```go
os.WriteFile(filepath.Join(artifactDirectory, workLedgerFileName), []byte(initialWorkLedger(workflow)), 0o644)
writeYAML(filepath.Join(artifactDirectory, nextActionFileName), initialNextAction(workflow))
writeYAML(filepath.Join(artifactDirectory, evidenceFileName), initialEvidence())
writeYAML(filepath.Join(artifactDirectory, finalAnswerContractFileName), initialFinalAnswerContract())
```

Use existing `writeYAML` helper for YAML files.

- [x] **Step 4: Add continuity input paths to actor briefs**

Modify `buildActorBrief` so every actor brief includes:

```markdown
## Continuity Inputs
- `work_ledger`: `<artifactDirectory>/work_ledger.md`
- `next_action`: `<artifactDirectory>/next_action.yaml`
- `evidence`: `<artifactDirectory>/evidence.yaml`
```

- [x] **Step 5: Test brief references**

Add assertions to `internal/runtime/bootstrap_test.go` that the planner and generator briefs contain `work_ledger.md`, `next_action.yaml`, and `evidence.yaml`.

Run:

```bash
go test ./internal/runtime -run 'TestBootstrapCreatesExpectedArtifactSkeleton|TestBootstrapCreatesExpectedArtifactSkeletonWithEmbeddedDefaults' -count=1
```

Expected: PASS.

- [x] **Step 6: Commit continuity bootstrap**

```bash
git add internal/runtime/continuity.go internal/runtime/continuity_test.go internal/runtime/bootstrap.go internal/runtime/bootstrap_test.go
git commit -m "feat: bootstrap workflow continuity artifacts"
```

## Task 6: Update Continuity During Supervisor Transitions

**Files:**
- Modify: `internal/runtime/continuity.go`
- Modify: `internal/runtime/continuity_test.go`
- Modify: `internal/runtime/runner.go`
- Modify: `internal/runtime/router.go`
- Modify: `internal/runtime/router_test.go`

- [x] **Step 1: Write failing next action update test**

Add a test to `internal/runtime/continuity_test.go`:

```go
func TestBuildNextActionAfterEvaluatorReviseTargetsGenerator(t *testing.T) {
	next := buildNextActionAfterEvaluation(State{
		LastDecision:    stringPtr("revise"),
		LastReasonCodes: []string{"tests_failed"},
	}, "generator")
	if next["actor"] != "generator" {
		t.Fatalf("unexpected actor: %#v", next["actor"])
	}
	if next["reason"] != "evaluator_requested_revision" {
		t.Fatalf("unexpected reason: %#v", next["reason"])
	}
}
```

Run: `go test ./internal/runtime -run TestBuildNextActionAfterEvaluatorReviseTargetsGenerator -count=1`

Expected: FAIL because the helper does not exist yet.

- [x] **Step 2: Implement transition helper**

Add helpers to `continuity.go`:

```go
func buildNextActionAfterActor(actorName string, nextActor *string) map[string]any
func buildNextActionAfterEvaluation(state State, fallbackActor string) map[string]any
func appendWorkLedgerEntry(path string, heading string, lines []string) error
```

Keep these helpers deterministic and small.

- [x] **Step 3: Update runner actor transitions**

In `Runner.Execute`, after `advanceAfterActor` and before writing state, update `next_action.yaml` and append to `work_ledger.md`.

For non-evaluator actor transitions:

```yaml
reason: actor_completed
actor: <next actor>
evidence_to_read:
  - <current actor output artifact>
```

- [x] **Step 4: Update evaluator routing transitions**

In `Router.RouteEvaluation`, after state/action is finalized, write a `next_action.yaml` that reflects:

- pass: no next actor, reason `terminal_pass`
- reject: no next actor, reason `terminal_reject`
- block_environment: no next actor, reason `environment_blocked`
- revise_generator: actor `generator`, reason `evaluator_requested_revision`
- rebuild_context: actor `context_builder`, reason `context_rebuild_requested`
- tighten_validation: actor `executor`, reason `validation_tighten_requested`

- [x] **Step 5: Add router integration assertion**

Update a relevant `internal/runtime/router_test.go` test to read `next_action.yaml` after a revise route and assert it points to the correct next actor and reason.

Run:

```bash
go test ./internal/runtime -run 'TestBuildNextAction|TestRouteEvaluation' -count=1
```

Expected: PASS.

- [x] **Step 6: Commit supervisor continuity updates**

```bash
git add internal/runtime/continuity.go internal/runtime/continuity_test.go internal/runtime/runner.go internal/runtime/router.go internal/runtime/router_test.go
git commit -m "feat: update continuity through supervisor transitions"
```

## Task 7: Strengthen Final Reporting Contract

**Files:**
- Modify: `internal/reporting/terminal_summary.go`
- Modify: `internal/reporting/terminal_summary_test.go`
- Modify: `internal/runtime/router.go`
- Modify: `internal/runtime/router_test.go`

- [x] **Step 1: Write failing terminal summary test**

Add a test in `internal/reporting/terminal_summary_test.go` that builds a terminal summary for a blocked or policy-violating run and asserts it does not say the work passed.

Representative assertion:

```go
if strings.Contains(summary, "status: `passed`") {
	t.Fatalf("policy violation summary must not report passed:\n%s", summary)
}
if !strings.Contains(summary, "backend_policy_violation") {
	t.Fatalf("expected policy violation to be visible:\n%s", summary)
}
```

Run: `go test ./internal/reporting -run TestTerminalSummary -count=1`

Expected: FAIL until reporting carries the policy violation reason.

- [x] **Step 2: Thread policy violation reason into terminal summary data**

Use existing `TerminalOutcome` or equivalent reporting structure. Add the smallest field needed, such as:

```go
PolicyViolations []string
```

Do not change artifact schemas unless tests prove they are required.

- [x] **Step 3: Include final answer contract guidance**

Update summary output to include a short section when policy violations or missing validation evidence exist:

```markdown
## Reporting Limits

- Final answer must not claim successful implementation because policy violations were detected.
```

- [x] **Step 4: Route event audit failures as blocked**

If event audit errors currently bubble as raw `Execute` errors, keep that behavior for Task 4. In this task, add a controlled path only if the runtime already has enough state to produce terminal summary. Prefer a small follow-up if this grows large.

- [x] **Step 5: Run reporting tests**

Run:

```bash
go test ./internal/reporting -count=1
go test ./internal/runtime -run 'TestRouteEvaluation|TestExecute' -count=1
```

Expected: PASS.

- [x] **Step 6: Commit final reporting contract**

```bash
git add internal/reporting/terminal_summary.go internal/reporting/terminal_summary_test.go internal/runtime/router.go internal/runtime/router_test.go
git commit -m "feat: enforce final reporting contract"
```

## Task 8: Update Architecture Docs

**Files:**
- Modify: `docs/ARCHITECTURE.md`
- Modify: `docs/ARCHITECTURE-kr.md`
- Modify: `README.md`

- [ ] **Step 1: Update architecture wording**

In both architecture docs, distinguish:

- user-facing Codex session: may use the Rail skill and user interaction.
- sealed actor Codex session: must use Rail backend policy and must not inherit user skills/rules/plugins/hooks by default.

Keep the explanation short and consistent with `docs/superpowers/specs/2026-04-24-sealed-actor-runtime-and-continuity-design.md`.

- [ ] **Step 2: Update README advanced backend section**

Add a short note under the actor backend discussion:

```markdown
Actor Codex runs are isolated from the user's normal Codex skill/rule surface by default. Rail treats actor events and artifacts as governance evidence, not as conversational memory.
```

Use placeholder paths only. Do not include a home-directory path.

- [ ] **Step 3: Check documentation path hygiene**

Run:

```bash
rg -n '(/'Users'/|~[/])' README.md docs/ARCHITECTURE.md docs/ARCHITECTURE-kr.md docs/superpowers/specs/2026-04-24-sealed-actor-runtime-and-continuity-design.md docs/superpowers/plans/2026-04-24-sealed-actor-runtime-and-continuity.md
```

Expected: no output.

- [ ] **Step 4: Commit docs**

```bash
git add README.md docs/ARCHITECTURE.md docs/ARCHITECTURE-kr.md
git commit -m "docs: explain sealed actor execution"
```

## Task 9: Full Verification

**Files:**
- No planned file edits.

- [ ] **Step 1: Run focused runtime tests**

Run:

```bash
go test ./internal/runtime -count=1
```

Expected: PASS.

- [ ] **Step 2: Run focused reporting tests**

Run:

```bash
go test ./internal/reporting -count=1
```

Expected: PASS.

- [ ] **Step 3: Run full test suite**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 4: Build CLI**

Run:

```bash
go build -o build/rail ./cmd/rail
```

Expected: PASS.

- [ ] **Step 5: Smoke existing request**

Run:

```bash
./build/rail validate-request --request .harness/requests/rail-bootstrap-smoke.yaml
```

Expected: PASS.

- [ ] **Step 6: Confirm working tree scope**

Run:

```bash
git status --short
```

Expected: only intended files are modified, or clean after commits.

## Execution Notes

- Implement Tasks 1-4 first if the immediate goal is to stop superpowers from firing inside actor runs.
- Implement Tasks 5-7 next if the goal is reliable workflow continuation and final answers.
- Task 8 can be done after the runtime behavior is real, but architecture docs should not lag behind release.
- Keep each task independently commit-worthy. If a task grows large, split it before coding.
