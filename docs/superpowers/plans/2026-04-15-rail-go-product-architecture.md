# Rail Go Product Architecture Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the checkout-driven Dart runtime with an installed Go product that ships a native `rail` binary, bundles the Rail Codex skill, and uses a project-local `.harness` workspace with embedded defaults plus file-level overrides.

**Architecture:** Build the Go product in parallel with the existing Dart runtime, then migrate the skill, install surface, and project workspace model onto the new runtime before removing checkout-based assumptions. Keep the natural-language skill as the primary request-composition interface while moving validation, orchestration, and asset resolution into the Go binary.

**Tech Stack:** Go CLI, embedded product assets, YAML/JSON contracts, Markdown skill docs, Homebrew packaging, GitHub Actions CI, repository fixtures under `.harness/` and `test/`

---

## Planned File Structure

### New Go runtime surface

- `go.mod`
  - Go module definition for the product runtime.
- `cmd/rail/main.go`
  - Thin Go CLI entrypoint.
- `internal/cli/app.go`
  - Command parsing, dispatch, exit-code handling.
- `internal/cli/compose_request.go`
  - `compose-request` command and stdin/input-file handling.
- `internal/cli/validate_request.go`
  - `validate-request` command.
- `internal/cli/init_project.go`
  - `rail init` project bootstrap command.
- `internal/cli/run.go`
  - `run` orchestration entrypoint.
- `internal/cli/execute.go`
  - `execute` orchestration entrypoint.
- `internal/cli/route_evaluation.go`
  - `route-evaluation` command.
- `internal/request/draft.go`
  - Structured request-draft contract consumed by the CLI.
- `internal/request/normalize.go`
  - Default filling and request normalization.
- `internal/contracts/validator.go`
  - Schema validation helpers and contract loading.
- `internal/assets/embed.go`
  - Embedded default harness assets.
- `internal/assets/resolve.go`
  - Project-local override plus embedded-default resolution logic.
- `internal/project/init.go`
  - `.harness` scaffold creation and `project.yaml` generation.
- `internal/project/workspace.go`
  - Project root discovery and `.harness` workspace helpers.
- `internal/runtime/bootstrap.go`
  - Artifact bootstrap and workflow materialization.
- `internal/runtime/runner.go`
  - Actor execution loop and subprocess orchestration.
- `internal/runtime/router.go`
  - Evaluator routing and retry-budget enforcement.
- `internal/reporting/terminal_summary.go`
  - Terminal summary generation.
- `internal/reporting/state.go`
  - State persistence and artifact output writing.

### Embedded product assets and packaging

- `assets/defaults/actors/`
  - Default actor definitions copied from the current `.harness/actors/`.
- `assets/defaults/supervisor/`
  - Default supervisor registry, policy, and execution policy.
- `assets/defaults/rules/`
  - Default guardrails.
- `assets/defaults/rubrics/`
  - Default evaluation rubrics.
- `assets/defaults/templates/`
  - Default request and artifact schemas/templates.
- `assets/scaffold/project.yaml`
  - `rail init` scaffold template.
- `assets/skill/Rail/SKILL.md`
  - Installed-product version of the Rail skill.
- `packaging/homebrew/rail.rb`
  - Homebrew formula or release template for shipping the product and bundled skill.

### Tests and verification

- `internal/.../*_test.go`
  - Focused unit and contract tests by package.
- `testdata/`
  - Go test fixtures for requests, route decisions, and project initialization.
- `.github/workflows/go-release-gate.yml`
  - Go-native CI gate for build and test verification.

### Existing files that must be rewritten or retired

- `README.md`
  - Full rewrite to product-install model with advanced override guidance.
- `docs/ARCHITECTURE.md`
  - Update runtime description from Dart checkout model to Go installed-product model.
- `skills/Rail/SKILL.md`
  - Rewrite for installed binary, project discovery, and structured draft submission.
- `scripts/install_skill.sh`
  - Retire or replace with packaging-only guidance; no checkout-based symlink installs.
- `bin/rail.dart`
  - Keep temporarily for migration comparison, then remove from the normal user path.
- `pubspec.yaml`
  - Remove once Dart runtime is no longer part of the released product.

### Existing harness content to re-home

- `.harness/actors/`
- `.harness/supervisor/`
- `.harness/rules/`
- `.harness/rubrics/`
- `.harness/templates/`

These remain the source material for the initial embedded defaults, but the released product should stop treating the repository-local `.harness/` tree as the end-user runtime root.

---

### Task 1: Create the Go Product Skeleton Beside the Existing Dart Runtime

**Files:**
- Create: `/absolute/path/to/rail/go.mod`
- Create: `/absolute/path/to/rail/cmd/rail/main.go`
- Create: `/absolute/path/to/rail/internal/cli/app.go`
- Create: `/absolute/path/to/rail/internal/cli/app_test.go`
- Modify: `/absolute/path/to/rail/.gitignore`
- Modify: `/absolute/path/to/rail/.github/workflows/v1-release-gate.yml`
- Create: `/absolute/path/to/rail/.github/workflows/go-release-gate.yml`

- [ ] **Step 1: Write the failing CLI boot test**

Add a Go test that proves the new CLI entrypoint exists and exposes the expected top-level command set:

```go
func TestAppRegistersCoreCommands(t *testing.T) {
    app := NewApp()
    got := app.CommandNames()
    want := []string{"compose-request", "validate-request", "init", "run", "execute", "route-evaluation"}
    if diff := cmp.Diff(want, got); diff != "" {
        t.Fatalf("unexpected commands (-want +got):\n%s", diff)
    }
}
```

- [ ] **Step 2: Run the focused test to confirm the Go runtime does not exist yet**

Run:

```bash
go test ./internal/cli -run TestAppRegistersCoreCommands -v
```

Expected:
- FAIL because `go.mod` and the CLI package do not exist yet

- [ ] **Step 3: Create the minimal Go module and thin entrypoint**

Add:

```go
// cmd/rail/main.go
func main() {
    os.Exit(cli.NewApp().Run(os.Args[1:]))
}
```

and the smallest `NewApp` implementation that registers the six core commands without behavior yet.

- [ ] **Step 4: Re-run the focused Go test**

Run:

```bash
go test ./internal/cli -run TestAppRegistersCoreCommands -v
```

Expected:
- PASS

- [ ] **Step 5: Add a repo-level build smoke check**

Run:

```bash
go test ./...
go build ./cmd/rail
```

Expected:
- tests and build succeed with a minimal skeleton

- [ ] **Step 6: Commit**

```bash
git add go.mod cmd/rail/main.go internal/cli/app.go internal/cli/app_test.go .gitignore .github/workflows/go-release-gate.yml .github/workflows/v1-release-gate.yml
git commit -m "build: add go product skeleton for rail"
```

### Task 2: Embed Default Harness Assets and Add File-Level Override Resolution

**Files:**
- Create: `/absolute/path/to/rail/assets/defaults/actors/`
- Create: `/absolute/path/to/rail/assets/defaults/supervisor/`
- Create: `/absolute/path/to/rail/assets/defaults/rules/`
- Create: `/absolute/path/to/rail/assets/defaults/rubrics/`
- Create: `/absolute/path/to/rail/assets/defaults/templates/`
- Create: `/absolute/path/to/rail/internal/assets/embed.go`
- Create: `/absolute/path/to/rail/internal/assets/resolve.go`
- Create: `/absolute/path/to/rail/internal/assets/resolve_test.go`
- Modify: `/absolute/path/to/rail/.harness/README.md`

- [ ] **Step 1: Write the failing asset-resolution tests**

Add tests for:

```go
func TestResolveUsesEmbeddedDefaultWhenLocalOverrideMissing(t *testing.T) { ... }
func TestResolveUsesProjectLocalFileWhenOverrideExists(t *testing.T) { ... }
func TestResolveDoesNotFallbackForStateDirectories(t *testing.T) { ... }
```

- [ ] **Step 2: Run the asset tests to capture the missing behavior**

Run:

```bash
go test ./internal/assets -v
```

Expected:
- FAIL because there is no embedded asset package or resolver yet

- [ ] **Step 3: Create the embedded asset tree from current default harness content**

Copy the current default product content from:

- `.harness/actors/`
- `.harness/supervisor/`
- `.harness/rules/`
- `.harness/rubrics/`
- `.harness/templates/`

into `assets/defaults/` and wire it into `embed.FS`.

- [ ] **Step 4: Implement file-level resolution only**

Implement a resolver with this behavior:

```go
if localFileExists {
    return localFile
}
if pathIsStateful {
    return errorNoFallback
}
return embeddedDefault
```

Do not add deep merge or directory merge behavior.

- [ ] **Step 5: Re-run the focused tests**

Run:

```bash
go test ./internal/assets -v
```

Expected:
- PASS

- [ ] **Step 6: Commit**

```bash
git add assets/defaults internal/assets .harness/README.md
git commit -m "feat: add embedded harness defaults with file-level overrides"
```

### Task 3: Add Project Discovery and `rail init` with Minimal `.harness` Scaffold

**Files:**
- Create: `/absolute/path/to/rail/assets/scaffold/project.yaml`
- Create: `/absolute/path/to/rail/internal/project/workspace.go`
- Create: `/absolute/path/to/rail/internal/project/init.go`
- Create: `/absolute/path/to/rail/internal/project/init_test.go`
- Create: `/absolute/path/to/rail/internal/cli/init_project.go`
- Modify: `/absolute/path/to/rail/internal/cli/app.go`

- [ ] **Step 1: Write the failing project-init tests**

Add tests for:

```go
func TestInitCreatesMinimalHarnessWorkspace(t *testing.T) { ... }
func TestInitDoesNotCreateOverrideDirectoriesByDefault(t *testing.T) { ... }
func TestDiscoverProjectPrefersHarnessProjectFileOverGitRoot(t *testing.T) { ... }
```

- [ ] **Step 2: Run the focused project tests**

Run:

```bash
go test ./internal/project -v
```

Expected:
- FAIL because the project workspace package does not exist yet

- [ ] **Step 3: Implement the minimal scaffold**

Generate only:

```text
.harness/project.yaml
.harness/requests/
.harness/artifacts/
.harness/learning/feedback/
.harness/learning/reviews/
.harness/learning/hardening-reviews/
.harness/learning/approved/
.harness/learning/review_queue.yaml
.harness/learning/hardening_queue.yaml
.harness/learning/family_evidence_index.yaml
```

with no default creation of override directories.

- [ ] **Step 4: Add `rail init` command wiring**

Expose:

```bash
rail init
rail init --project-root /absolute/path/to/target-repo
```

with idempotent behavior when `.harness/project.yaml` already exists.

- [ ] **Step 5: Re-run the focused tests and a real filesystem smoke**

Run:

```bash
go test ./internal/project -v
tmpdir="$(mktemp -d)"
(cd "$tmpdir" && /absolute/path/to/rail/rail init)
find "$tmpdir/.harness" | sort
```

Expected:
- tests pass
- the generated tree matches the agreed minimal scaffold

- [ ] **Step 6: Commit**

```bash
git add assets/scaffold internal/project internal/cli/init_project.go internal/cli/app.go
git commit -m "feat: add project discovery and rail init scaffold"
```

### Task 4: Implement Structured Request Draft Input for `compose-request`

**Files:**
- Create: `/absolute/path/to/rail/internal/request/draft.go`
- Create: `/absolute/path/to/rail/internal/request/normalize.go`
- Create: `/absolute/path/to/rail/internal/request/normalize_test.go`
- Create: `/absolute/path/to/rail/internal/cli/compose_request.go`
- Modify: `/absolute/path/to/rail/internal/cli/app.go`
- Modify: `/absolute/path/to/rail/skills/Rail/SKILL.md`
- Modify: `/absolute/path/to/rail/skills/Rail/references/examples.md`

- [ ] **Step 1: Write the failing normalization tests**

Add tests that prove:

```go
func TestComposeRequestNormalizesDraftFromStdin(t *testing.T) { ... }
func TestComposeRequestFillsDefaultRiskToleranceByTaskType(t *testing.T) { ... }
func TestComposeRequestRejectsMissingGoal(t *testing.T) { ... }
```

- [ ] **Step 2: Run the focused request tests**

Run:

```bash
go test ./internal/request -v
```

Expected:
- FAIL because the request-draft contract does not exist yet

- [ ] **Step 3: Implement structured draft handling**

Define a draft contract such as:

```go
type Draft struct {
    RequestVersion    string   `json:"request_version" yaml:"request_version"`
    ProjectRoot       string   `json:"project_root" yaml:"project_root"`
    TaskType          string   `json:"task_type" yaml:"task_type"`
    Goal              string   `json:"goal" yaml:"goal"`
    Context           []string `json:"context" yaml:"context"`
    Constraints       []string `json:"constraints" yaml:"constraints"`
    DefinitionOfDone  []string `json:"definition_of_done" yaml:"definition_of_done"`
    RiskTolerance     string   `json:"risk_tolerance" yaml:"risk_tolerance"`
}
```

and support both:

```bash
rail compose-request --stdin
rail compose-request --input /absolute/path/to/draft.json
```

- [ ] **Step 4: Rewrite the Rail skill around the new contract**

Update `skills/Rail/SKILL.md` so it:

- keeps natural-language interpretation as the primary UX
- emits structured request drafts
- calls the installed `rail` binary instead of assuming a local checkout runtime root

- [ ] **Step 5: Re-run request tests and a CLI smoke**

Run:

```bash
go test ./internal/request ./internal/cli -v
cat /absolute/path/to/rail/testdata/request_draft.json | rail compose-request --stdin
```

Expected:
- tests pass
- CLI writes a normalized request file without checkout-based assumptions

- [ ] **Step 6: Commit**

```bash
git add internal/request internal/cli/compose_request.go internal/cli/app.go skills/Rail/SKILL.md skills/Rail/references/examples.md
git commit -m "feat: add structured request draft support for rail skill"
```

### Task 5: Port Request Validation, Artifact Bootstrap, and Route Evaluation to Go

**Files:**
- Create: `/absolute/path/to/rail/internal/contracts/validator.go`
- Create: `/absolute/path/to/rail/internal/runtime/bootstrap.go`
- Create: `/absolute/path/to/rail/internal/runtime/router.go`
- Create: `/absolute/path/to/rail/internal/runtime/bootstrap_test.go`
- Create: `/absolute/path/to/rail/internal/runtime/router_test.go`
- Create: `/absolute/path/to/rail/internal/cli/validate_request.go`
- Create: `/absolute/path/to/rail/internal/cli/route_evaluation.go`
- Modify: `/absolute/path/to/rail/internal/cli/app.go`
- Reuse fixtures from: `/absolute/path/to/rail/test/fixtures/standard_route/`

- [ ] **Step 1: Write the failing validation and routing tests**

Add tests such as:

```go
func TestValidateRequestAcceptsCurrentValidFixture(t *testing.T) { ... }
func TestRouteEvaluationMapsFixtureToTightenValidation(t *testing.T) { ... }
func TestBootstrapCreatesExpectedArtifactSkeleton(t *testing.T) { ... }
```

- [ ] **Step 2: Run the focused runtime tests**

Run:

```bash
go test ./internal/contracts ./internal/runtime -v
```

Expected:
- FAIL because the validator and bootstrap logic do not exist yet

- [ ] **Step 3: Implement validation and bootstrap from current contracts**

Use the existing request and route fixtures as the compatibility baseline.

Port:

- request validation against current schemas
- artifact directory creation
- resolved workflow/state bootstrap
- route-evaluation decisions for current evaluator outputs

- [ ] **Step 4: Add CLI command wiring**

Support:

```bash
rail validate-request --request /absolute/path/to/request.yaml
rail route-evaluation --artifact /absolute/path/to/evaluation_result.yaml
```

- [ ] **Step 5: Re-run focused tests and compare with existing fixtures**

Run:

```bash
go test ./internal/contracts ./internal/runtime ./internal/cli -v
```

Expected:
- tests pass
- the current checked-in fixtures remain valid compatibility inputs

- [ ] **Step 6: Commit**

```bash
git add internal/contracts internal/runtime internal/cli testdata
git commit -m "feat: port validation bootstrap and route evaluation to go"
```

### Task 6: Port `run` and `execute` Runtime Orchestration While Preserving Artifact Traceability

**Files:**
- Create: `/absolute/path/to/rail/internal/runtime/runner.go`
- Create: `/absolute/path/to/rail/internal/runtime/subprocess.go`
- Create: `/absolute/path/to/rail/internal/reporting/state.go`
- Create: `/absolute/path/to/rail/internal/reporting/terminal_summary.go`
- Create: `/absolute/path/to/rail/internal/runtime/runner_test.go`
- Create: `/absolute/path/to/rail/internal/reporting/terminal_summary_test.go`
- Create: `/absolute/path/to/rail/internal/cli/run.go`
- Create: `/absolute/path/to/rail/internal/cli/execute.go`
- Modify: `/absolute/path/to/rail/internal/cli/app.go`
- Reuse fixture references from: `/absolute/path/to/rail/.harness/requests/rail-bootstrap-smoke.yaml`

- [ ] **Step 1: Write the failing execution-contract tests**

Add tests that prove:

```go
func TestRunBootstrapsSmokeArtifact(t *testing.T) { ... }
func TestExecuteProducesTerminalSummaryAndState(t *testing.T) { ... }
func TestExecutePreservesSupervisorTraceability(t *testing.T) { ... }
```

- [ ] **Step 2: Run the focused runtime tests**

Run:

```bash
go test ./internal/runtime ./internal/reporting -v
```

Expected:
- FAIL because there is no Go execution loop yet

- [ ] **Step 3: Implement the minimal compatible runtime loop**

Port:

- subprocess invocation for actor execution
- artifact writes for `state.json`, `supervisor_trace.md`, and `terminal_summary.md`
- bounded routing loop behavior
- smoke fast-path compatibility where applicable

Keep the output model explicit and artifact-first.

- [ ] **Step 4: Add CLI entrypoints for `run` and `execute`**

Support:

```bash
rail run --request /absolute/path/to/request.yaml --project-root /absolute/path/to/target-repo
rail execute --artifact /absolute/path/to/artifact-dir
```

- [ ] **Step 5: Re-run focused tests and a real smoke workflow**

Run:

```bash
go test ./internal/runtime ./internal/reporting ./internal/cli -v
rail run --request .harness/requests/rail-bootstrap-smoke.yaml --project-root /absolute/path/to/rail --task-id go-smoke
rail execute --artifact .harness/artifacts/go-smoke
```

Expected:
- tests pass
- smoke workflow completes
- artifact outputs remain readable and traceable

- [ ] **Step 6: Commit**

```bash
git add internal/runtime internal/reporting internal/cli
git commit -m "feat: port run and execute orchestration to go"
```

### Task 7: Bundle the Installed Skill and Package the Product for Homebrew

**Files:**
- Create: `/absolute/path/to/rail/assets/skill/Rail/SKILL.md`
- Create: `/absolute/path/to/rail/assets/skill/Rail/references/examples.md`
- Create: `/absolute/path/to/rail/internal/install/skill.go`
- Create: `/absolute/path/to/rail/internal/install/skill_test.go`
- Create: `/absolute/path/to/rail/packaging/homebrew/rail.rb`
- Modify: `/absolute/path/to/rail/README.md`
- Modify: `/absolute/path/to/rail/scripts/install_skill.sh`

- [ ] **Step 1: Write the failing bundled-skill tests**

Add tests that prove:

```go
func TestInstallLayoutIncludesRailSkillAssets(t *testing.T) { ... }
func TestBundledSkillReferencesInstalledRailBinary(t *testing.T) { ... }
```

- [ ] **Step 2: Run the focused install tests**

Run:

```bash
go test ./internal/install -v
```

Expected:
- FAIL because there is no install/packaging helper yet

- [ ] **Step 3: Add bundled skill assets and install helpers**

Ensure the packaged skill:

- is shipped with the product
- references the installed `rail` binary
- does not depend on a checked-out repository path

- [ ] **Step 4: Replace checkout-era installation guidance**

Update `scripts/install_skill.sh` to either:

- emit a clear deprecation message pointing to packaged installs

or:

- remove it from the user path entirely and keep it only for local development tooling

Recommended: deprecate it with an explicit message first, then delete in a later cleanup commit.

- [ ] **Step 5: Add packaging verification notes**

Run:

```bash
go test ./internal/install -v
brew install --build-from-source ./packaging/homebrew/rail.rb
```

Expected:
- install tests pass
- the formula installs the binary and skill assets in the intended layout

- [ ] **Step 6: Commit**

```bash
git add assets/skill internal/install packaging/homebrew/rail.rb README.md scripts/install_skill.sh
git commit -m "build: bundle rail skill with packaged install"
```

### Task 8: Rewrite Product Documentation and Remove Checkout-Based User Guidance

**Files:**
- Modify: `/absolute/path/to/rail/README.md`
- Modify: `/absolute/path/to/rail/docs/ARCHITECTURE.md`
- Modify: `/absolute/path/to/rail/docs/ARCHITECTURE-kr.md`
- Modify: `/absolute/path/to/rail/docs/releases/v1-core-supervisor-gate.md`
- Modify: `/absolute/path/to/rail/docs/backlog/v1-core-supervisor-gate.md`
- Modify: `/absolute/path/to/rail/docs/tasks.md`
- Reference spec: `/absolute/path/to/rail/docs/superpowers/specs/2026-04-15-rail-go-product-architecture-design.md`

- [ ] **Step 1: Write the failing documentation checks**

Capture the outdated guidance with:

```bash
rg -n "git clone|dart pub get|install_skill|local Rail checkout|current checkout as the runtime root" README.md docs skills
```

Expected:
- matches prove the docs still describe the checkout-driven product

- [ ] **Step 2: Rewrite README completely**

The new README must cover:

- `brew install rail`
- built-in Rail skill install
- `rail init`
- project-local `.harness` model
- advanced overrides and file-level precedence
- source repository as development/contribution origin only

- [ ] **Step 3: Rewrite supporting docs to match the product model**

Update architecture and release docs so they stop treating the repository checkout as the end-user runtime root.

- [ ] **Step 4: Re-run the grep check**

Run:

```bash
rg -n "git clone|dart pub get|install_skill|local Rail checkout|current checkout as the runtime root" README.md docs skills
```

Expected:
- no user-facing documentation still describes the old model except in explicit migration/archive sections

- [ ] **Step 5: Verify advanced override guidance exists**

Run:

```bash
rg -n "Advanced Overrides|file-level override|embedded defaults|project-local \\.harness" README.md docs/ARCHITECTURE.md
```

Expected:
- the advanced override surface and precedence rules are documented

- [ ] **Step 6: Commit**

```bash
git add README.md docs/ARCHITECTURE.md docs/ARCHITECTURE-kr.md docs/releases/v1-core-supervisor-gate.md docs/backlog/v1-core-supervisor-gate.md docs/tasks.md
git commit -m "docs: rewrite rail docs for installed go product model"
```

### Task 9: Remove Dart-First Release Assumptions and Make Go the Primary Release Gate

**Files:**
- Modify: `/absolute/path/to/rail/.github/workflows/v1-release-gate.yml`
- Modify: `/absolute/path/to/rail/tool/v1_release_gate.sh`
- Modify: `/absolute/path/to/rail/tool/v2_release_gate.sh`
- Modify: `/absolute/path/to/rail/test/tool/release_gate_common_test.dart`
- Delete after parity: `/absolute/path/to/rail/bin/rail.dart`
- Delete after parity: `/absolute/path/to/rail/pubspec.yaml`

- [ ] **Step 1: Write the failing release-gate expectation update**

Add or update a gate test that expects:

```text
- go test ./...
- go build ./cmd/rail
- rail smoke workflow passes
```

instead of Dart compile or `dart run` as the release product path.

- [ ] **Step 2: Run the gate tests to confirm the old assumptions remain**

Run:

```bash
dart test test/tool/release_gate_common_test.dart
```

Expected:
- FAIL or expose stale references to Dart-first release behavior

- [ ] **Step 3: Switch the release scripts to Go-first verification**

Update the release gate to use:

```bash
go test ./...
go build ./cmd/rail
./rail run ...
./rail execute ...
```

Keep temporary migration notes only where the repo still carries Dart compatibility code.

- [ ] **Step 4: Remove Dart from the released product path**

Once Go parity is verified:

- delete `bin/rail.dart`
- delete `pubspec.yaml`
- remove remaining Dart runtime references from active release workflows

- [ ] **Step 5: Run the full release verification**

Run:

```bash
go test ./...
go build ./cmd/rail
./tool/v1_release_gate.sh
./tool/v2_release_gate.sh
```

Expected:
- Go build and tests pass
- release gates pass without relying on Dart runtime packaging

- [ ] **Step 6: Commit**

```bash
git add .github/workflows/v1-release-gate.yml tool/v1_release_gate.sh tool/v2_release_gate.sh test/tool/release_gate_common_test.dart
git rm bin/rail.dart pubspec.yaml
git commit -m "build: make go runtime the primary rail release path"
```
