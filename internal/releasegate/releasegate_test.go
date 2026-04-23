package releasegate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRejectsSmokeTaskIDsWithTraversalSegments(t *testing.T) {
	root := repoRoot(t)
	cmd := exec.Command("bash", "-lc", `source tool/release_gate_common.sh && rail_validate_smoke_task_id "../outside"`)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected traversal smoke task id to fail")
	}
	if !strings.Contains(string(output), "invalid smoke task id") {
		t.Fatalf("unexpected output: %s", string(output))
	}
}

func TestAcceptsSafeSmokeTaskIDs(t *testing.T) {
	root := repoRoot(t)
	cmd := exec.Command("bash", "-lc", `source tool/release_gate_common.sh && rail_validate_smoke_task_id "v2-integrator-smoke-ci_01"`)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected safe smoke task id to pass: %v\n%s", err, string(output))
	}
	if got := strings.TrimSpace(string(output)); got != "v2-integrator-smoke-ci_01" {
		t.Fatalf("unexpected validated smoke task id: %q", got)
	}
}

func TestV1ReleaseGateScriptIsGoFirst(t *testing.T) {
	script := readRepoFile(t, "tool", "v1_release_gate.sh")
	legacyRuntime := "bin/rail." + "d" + "art"
	legacy := "d" + "art"
	for _, expected := range []string{
		"go test ./...",
		"go build -o build/rail ./cmd/rail",
		"./build/rail run",
		"./build/rail execute",
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("v1 release gate missing %q", expected)
		}
	}
	for _, rejected := range []string{
		legacy + " pub get",
		legacy + " analyze",
		legacy + " test",
		legacy + " compile exe",
		legacy + " run " + legacyRuntime,
	} {
		if strings.Contains(script, rejected) {
			t.Fatalf("v1 release gate still contains %q", rejected)
		}
	}
}

func TestReleaseGateWorkflowProvisionsGoInsteadOfDart(t *testing.T) {
	workflow := readRepoFile(t, ".github", "workflows", "release-gate.yml")
	for _, expected := range []string{
		"matrix:",
		"run: ./tool/release_gate.sh",
		"actions/setup-go@v5",
		"go-version-file: go.mod",
	} {
		if !strings.Contains(workflow, expected) {
			t.Fatalf("release-gate workflow missing %q", expected)
		}
	}
	if strings.Contains(workflow, "d"+"art-lang/setup-"+("d"+"art")+"@v1") {
		t.Fatalf("release-gate workflow still provisions Dart")
	}
}

func TestReleaseWorkflowPublishesGoReleaserArtifactsAndAttestations(t *testing.T) {
	workflow := readRepoFile(t, ".github", "workflows", "release.yml")
	for _, expected := range []string{
		"Prepare release preflight",
		"Resolve release version",
		`./tool/prepare_release.sh "${{ steps.release_version.outputs.version }}" --preflight-only --allow-existing-tag`,
		`./tool/prepare_release.sh "${{ steps.release_version.outputs.version }}" --push --allow-existing-tag`,
		"HOMEBREW_TAP_GITHUB_TOKEN: ${{ secrets.HOMEBREW_TAP_GITHUB_TOKEN }}",
		"goreleaser/goreleaser-action",
		"args: release --clean",
		"id-token: write",
		"attestations: write",
		"actions/attest@v4",
		"dist/checksums.txt",
	} {
		if !strings.Contains(workflow, expected) {
			t.Fatalf("release workflow missing %q", expected)
		}
	}
}

func TestPrepareReleaseScriptRejectsExistingTags(t *testing.T) {
	repo := newPrepareReleaseFixture(t)
	version := "v99.99.99"
	createTag := exec.Command("git", "tag", version)
	createTag.Dir = repo
	if output, err := createTag.CombinedOutput(); err != nil {
		t.Fatalf("create test tag: %v\n%s", err, string(output))
	}
	t.Cleanup(func() {
		deleteTag := exec.Command("git", "tag", "-d", version)
		deleteTag.Dir = repo
		_ = deleteTag.Run()
	})

	cmd := exec.Command("bash", "-lc", `HOMEBREW_TAP_GITHUB_TOKEN=dummy ./tool/prepare_release.sh "$VERSION" --preflight-only`)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "VERSION="+version)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected existing release tag to fail")
	}
	if !strings.Contains(string(output), "release tag already exists") {
		t.Fatalf("unexpected output: %s", string(output))
	}
}

func TestPrepareReleaseScriptRequiresHomebrewTapToken(t *testing.T) {
	repo := newPrepareReleaseFixture(t)
	cmd := exec.Command("bash", "-lc", `unset HOMEBREW_TAP_GITHUB_TOKEN; ./tool/prepare_release.sh v9.9.9 --preflight-only --allow-existing-tag`)
	cmd.Dir = repo
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected missing Homebrew tap token to fail")
	}
	if !strings.Contains(string(output), "HOMEBREW_TAP_GITHUB_TOKEN is required") {
		t.Fatalf("unexpected output: %s", string(output))
	}
}

func TestPublishScriptPreparesReleasePullRequest(t *testing.T) {
	script := readRepoFile(t, "tool", "publish.sh")
	for _, expected := range []string{
		"log_step()",
		`log_step "01" "validate inputs"`,
		`log_step "02" "check repository state"`,
		`log_step "03" "check release uniqueness"`,
		`log_step "04" "create release branch"`,
		`log_step "05" "generate changelog"`,
		`log_step "06" "update homebrew formula"`,
		`log_step "07" "verify release edits"`,
		`log_step "08" "commit release prep"`,
		`log_step "09" "publish pull request"`,
		"# 01. Validate input and execution mode.",
		"# 02. Validate repository state before creating release edits.",
		"# 03. Ensure this version and release branch do not already exist.",
		"# 04. Create an isolated release branch from the current branch.",
		"# 05. Generate and insert the release changelog section.",
		"# 06. Update the source Homebrew formula to the release tag.",
		"# 07. Verify generated edits before committing.",
		"# 08. Commit the release preparation changes.",
		"# 09. Push the branch and open the release PR.",
		"CHANGELOG.md",
		"packaging/homebrew/rail.rb",
		"--agent-changelog",
		"agent_changelog=false",
		`branch="release/${version}"`,
		`git checkout -b "$branch"`,
		"restore_start_branch",
		"GH_PROMPT_DISABLED",
		"GIT_TERMINAL_PROMPT",
		"BatchMode=yes",
		"assert_origin_main_ancestor",
		"assert_pr_create_permission",
		"viewerPermission",
		"codex exec",
		"--sandbox read-only",
		`approval_policy="never"`,
		"validate_changelog_section",
		`git commit -m "chore: prepare ${version} release"`,
		`./tool/verify_release_formula.sh`,
		"gh pr create",
		"prepare_release.sh ${version} --push",
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("publish script missing %q", expected)
		}
	}
}

func TestPublishScriptRejectsReadOnlyPRPermission(t *testing.T) {
	repo := newPublishFixture(t)
	gitRun(t, repo, "checkout", "-b", "work/publish")

	fakeBin := filepath.Join(t.TempDir(), "fake-bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatalf("create fake bin: %v", err)
	}
	writeExecutable(t, filepath.Join(fakeBin, "gh"), strings.Join([]string{
		"#!/usr/bin/env bash",
		"set -euo pipefail",
		`if [[ "$1" == "repo" && "$2" == "view" ]]; then`,
		"  echo READ",
		"  exit 0",
		"fi",
		"exit 1",
		"",
	}, "\n"))

	cmd := exec.Command("./tool/publish.sh", "v7.8.7")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected read-only PR permission to fail")
	}
	if !strings.Contains(string(output), "gh cannot create pull requests for this repository") {
		t.Fatalf("unexpected output: %s", string(output))
	}
	if branch := strings.TrimSpace(gitOutput(t, repo, "branch", "--show-current")); branch != "work/publish" {
		t.Fatalf("publish should fail before creating release branch, got branch %q", branch)
	}
	if branches := gitOutput(t, repo, "branch", "--list", "release/*"); strings.TrimSpace(branches) != "" {
		t.Fatalf("publish should not create release branches without PR permission:\n%s", branches)
	}
}

func TestPublishScriptRejectsMainStartBranch(t *testing.T) {
	repo := newPublishFixture(t)

	cmd := exec.Command("./tool/publish.sh", "v7.8.8", "--local-only")
	cmd.Dir = repo
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected main start branch to fail")
	}
	if !strings.Contains(string(output), "publish must start from a non-main branch") {
		t.Fatalf("unexpected output: %s", string(output))
	}
}

func TestPublishScriptRejectsBranchBehindOriginMain(t *testing.T) {
	repo := newPublishFixture(t)
	gitRun(t, repo, "checkout", "-b", "work/publish")

	upstreamWork := t.TempDir()
	gitRun(t, upstreamWork, "clone", filepath.Join(filepath.Dir(repo), "origin.git"), "upstream")
	upstream := filepath.Join(upstreamWork, "upstream")
	gitRun(t, upstream, "config", "user.email", "rail-release-test@example.invalid")
	gitRun(t, upstream, "config", "user.name", "Rail Release Test")
	if err := os.WriteFile(filepath.Join(upstream, "upstream.txt"), []byte("upstream\n"), 0o644); err != nil {
		t.Fatalf("write upstream fixture file: %v", err)
	}
	gitRun(t, upstream, "add", "upstream.txt")
	gitRun(t, upstream, "commit", "-m", "test upstream main change")
	gitRun(t, upstream, "push", "origin", "main")

	fakeBin := filepath.Join(t.TempDir(), "fake-bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatalf("create fake bin: %v", err)
	}
	writeExecutable(t, filepath.Join(fakeBin, "gh"), "#!/usr/bin/env bash\nexit 0\n")

	cmd := exec.Command("./tool/publish.sh", "v7.8.8", "--local-only")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected stale publish branch to fail")
	}
	if !strings.Contains(string(output), "publish branch must contain origin/main") {
		t.Fatalf("unexpected output: %s", string(output))
	}
}

func TestPublishScriptUsesAgentChangelogWhenRequested(t *testing.T) {
	repo := newPublishFixture(t)
	gitRun(t, repo, "checkout", "-b", "work/publish")
	if err := os.WriteFile(filepath.Join(repo, "branch-only.txt"), []byte("must not enter release\n"), 0o644); err != nil {
		t.Fatalf("write branch-only fixture file: %v", err)
	}
	gitRun(t, repo, "add", "branch-only.txt")
	gitRun(t, repo, "commit", "-m", "test work branch only change")

	fakeBin := filepath.Join(t.TempDir(), "fake-bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatalf("create fake bin: %v", err)
	}
	writeExecutable(t, filepath.Join(fakeBin, "gh"), "#!/usr/bin/env bash\nexit 0\n")
	writeExecutable(t, filepath.Join(fakeBin, "codex"), strings.Join([]string{
		"#!/usr/bin/env bash",
		"set -euo pipefail",
		`out=""`,
		"while (($#)); do",
		`  if [[ "$1" == "-o" || "$1" == "--output-last-message" ]]; then`,
		`    out="$2"`,
		"    shift 2",
		"    continue",
		"  fi",
		"  shift",
		"done",
		`if [[ -z "$out" ]]; then`,
		`  echo "missing output file" >&2`,
		"  exit 1",
		"fi",
		`cat > "$out" <<'EOF'`,
		"## v7.8.9 - 2026-04-21",
		"",
		"### Changed",
		"",
		"- Summarized by fake changelog agent.",
		"",
		"### Verification",
		"",
		"- `tool/prepare_release.sh v7.8.9`",
		"EOF",
		"",
	}, "\n"))

	cmd := exec.Command("./tool/publish.sh", "v7.8.9", "--agent-changelog", "--local-only")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("publish with agent changelog failed: %v\n%s", err, string(output))
	}

	currentBranch := strings.TrimSpace(gitOutput(t, repo, "branch", "--show-current"))
	if currentBranch != "work/publish" {
		t.Fatalf("publish should return to work branch, got %q", currentBranch)
	}

	changelog := gitOutput(t, repo, "show", "release/v7.8.9:CHANGELOG.md")
	for _, expected := range []string{
		"## v7.8.9 - 2026-04-21",
		"Summarized by fake changelog agent.",
		"`tool/prepare_release.sh v7.8.9`",
	} {
		if !strings.Contains(changelog, expected) {
			t.Fatalf("CHANGELOG.md missing %q:\n%s", expected, changelog)
		}
	}
	if strings.Contains(changelog, "Prepared v7.8.9 release.") {
		t.Fatalf("agent changelog should replace the template changelog:\n%s", changelog)
	}
	workBranchChangelog := readFile(t, filepath.Join(repo, "CHANGELOG.md"))
	if strings.Contains(workBranchChangelog, "## v7.8.9") {
		t.Fatalf("work branch should not contain release changelog after publish returns:\n%s", workBranchChangelog)
	}

	formula := gitOutput(t, repo, "show", "release/v7.8.9:packaging/homebrew/rail.rb")
	for _, expected := range []string{`tag: "v7.8.9"`, `version "7.8.9"`} {
		if !strings.Contains(formula, expected) {
			t.Fatalf("formula missing %q:\n%s", expected, formula)
		}
	}

	status := gitOutput(t, repo, "status", "--short")
	if strings.TrimSpace(status) != "" {
		t.Fatalf("expected clean fixture repo after publish, got:\n%s", status)
	}
	commit := gitOutput(t, repo, "log", "-1", "--pretty=%s", "release/v7.8.9")
	if strings.TrimSpace(commit) != "chore: prepare v7.8.9 release" {
		t.Fatalf("unexpected commit subject: %q", strings.TrimSpace(commit))
	}
	if err := exec.Command("git", "-C", repo, "cat-file", "-e", "release/v7.8.9:branch-only.txt").Run(); err != nil {
		t.Fatalf("release branch should be created from the starting work branch")
	}
}

func TestPublishScriptRejectsInvalidVersion(t *testing.T) {
	root := repoRoot(t)
	cmd := exec.Command("bash", "-lc", `./tool/publish.sh 0.2.5`)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected invalid publish version to fail")
	}
	if !strings.Contains(string(output), "invalid release version") {
		t.Fatalf("unexpected output: %s", string(output))
	}
}

func TestPrepareReleaseScriptRequiresChangelogSection(t *testing.T) {
	repo := newPrepareReleaseFixture(t)
	cmd := exec.Command("bash", "-lc", `HOMEBREW_TAP_GITHUB_TOKEN=dummy ./tool/prepare_release.sh v9.9.9 --preflight-only --allow-existing-tag`)
	cmd.Dir = repo
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected missing changelog section to fail")
	}
	if !strings.Contains(string(output), "missing CHANGELOG.md section") {
		t.Fatalf("unexpected output: %s", string(output))
	}
}

func TestPrepareReleaseScriptRejectsTrackedOrPendingDist(t *testing.T) {
	repo := newPrepareReleaseFixture(t)
	formulaTag := readFormulaTagFromPath(t, filepath.Join(repo, "packaging", "homebrew", "rail.rb"))
	distFile := filepath.Join(repo, "dist", "prepare-release-test")
	if err := os.MkdirAll(filepath.Dir(distFile), 0o755); err != nil {
		t.Fatalf("create dist dir: %v", err)
	}
	if err := os.WriteFile(distFile, []byte("generated"), 0o644); err != nil {
		t.Fatalf("write dist sentinel: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(filepath.Join(repo, "dist"))
	})

	cmd := exec.Command("bash", "-lc", `HOMEBREW_TAP_GITHUB_TOKEN=dummy ./tool/prepare_release.sh "$FORMULA_TAG" --preflight-only --allow-existing-tag`)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "FORMULA_TAG="+formulaTag)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected pending dist to fail")
	}
	if !strings.Contains(string(output), "dist/ must not be tracked or pending in git") {
		t.Fatalf("unexpected output: %s", string(output))
	}
}

func TestPrepareReleaseScriptRejectsDirtyWorkingTree(t *testing.T) {
	repo := newPrepareReleaseFixture(t)
	formulaTag := readFormulaTagFromPath(t, filepath.Join(repo, "packaging", "homebrew", "rail.rb"))
	dirtyFile := filepath.Join(repo, ".prepare-release-dirty-test")
	if err := os.WriteFile(dirtyFile, []byte("dirty"), 0o644); err != nil {
		t.Fatalf("write dirty sentinel: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Remove(dirtyFile)
	})

	cmd := exec.Command("bash", "-lc", `HOMEBREW_TAP_GITHUB_TOKEN=dummy ./tool/prepare_release.sh "$FORMULA_TAG" --preflight-only --allow-existing-tag`)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "FORMULA_TAG="+formulaTag)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected dirty working tree to fail")
	}
	if !strings.Contains(string(output), "working tree is dirty") {
		t.Fatalf("unexpected output: %s", string(output))
	}
}

func TestPrepareReleasePreflightRunsOnPRBranches(t *testing.T) {
	repo := newPrepareReleaseFixture(t)
	formulaTag := readFormulaTagFromPath(t, filepath.Join(repo, "packaging", "homebrew", "rail.rb"))
	gitRun(t, repo, "checkout", "-b", "release/test-preflight")

	cmd := exec.Command("bash", "-lc", `HOMEBREW_TAP_GITHUB_TOKEN=dummy ./tool/prepare_release.sh "$FORMULA_TAG" --preflight-only --allow-existing-tag`)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "FORMULA_TAG="+formulaTag)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected preflight on PR branch to pass: %v\n%s", err, string(output))
	}
	if !strings.Contains(string(output), "release preflight passed") {
		t.Fatalf("unexpected output: %s", string(output))
	}
}

func TestPrepareReleasePushStillRequiresMain(t *testing.T) {
	repo := newPrepareReleaseFixture(t)
	formulaTag := readFormulaTagFromPath(t, filepath.Join(repo, "packaging", "homebrew", "rail.rb"))
	gitRun(t, repo, "checkout", "-b", "release/test-push")

	cmd := exec.Command("bash", "-lc", `HOMEBREW_TAP_GITHUB_TOKEN=dummy ./tool/prepare_release.sh "$FORMULA_TAG" --push --allow-existing-tag`)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "FORMULA_TAG="+formulaTag)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected push release on PR branch to fail")
	}
	if !strings.Contains(string(output), "release publish must run from main") {
		t.Fatalf("unexpected output: %s", string(output))
	}
}

func TestVerifyReleaseFormulaScriptRejectsMismatchedVersion(t *testing.T) {
	root := repoRoot(t)
	cmd := exec.Command("bash", "-lc", `GITHUB_REF_NAME=v9.9.9 ./tool/verify_release_formula.sh`)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected mismatched formula version to fail")
	}
	if !strings.Contains(string(output), "formula release mismatch") {
		t.Fatalf("unexpected output: %s", string(output))
	}
}

func TestGoReleaserConfigPublishesRailHomebrewTap(t *testing.T) {
	config := readRepoFile(t, ".goreleaser.yaml")
	for _, expected := range []string{
		"project_name: rail",
		"main: ./cmd/rail",
		"homepage: https://github.com/cutehackers/rail",
		"owner: cutehackers",
		"name: homebrew-rail",
		"Formula/rail.rb",
		"assets/skill/Rail/SKILL.md",
	} {
		if !strings.Contains(config, expected) {
			t.Fatalf(".goreleaser.yaml missing %q", expected)
		}
	}
	if strings.Contains(config, "example.com") {
		t.Fatalf(".goreleaser.yaml still contains placeholder URL")
	}
}

func TestV2ReleaseGateRunsFullGoFirstChecks(t *testing.T) {
	script := readRepoFile(t, "tool", "v2_release_gate.sh")
	legacyRuntime := "bin/rail." + "d" + "art"
	legacy := "d" + "art"
	for _, expected := range []string{
		"go test ./...",
		"go build -o build/rail ./cmd/rail",
		"mktemp -d",
		"PATH=\"$FAKE_BIN:$PATH\" ./build/rail integrate",
		"./build/rail run",
		"./build/rail execute",
		"./build/rail validate-artifact",
		"\"$REPO_ROOT/build/rail\" verify-learning-state",
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("v2 release gate missing %q", expected)
		}
	}
	for _, rejected := range []string{
		"Go CLI parity is incomplete for v2 release gate",
		legacy + " compile exe",
		legacy + " run " + legacyRuntime + " run",
		legacy + " run " + legacyRuntime + " execute",
	} {
		if strings.Contains(script, rejected) {
			t.Fatalf("v2 release gate still contains %q", rejected)
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("failed to resolve repo root: %v", err)
	}
	return root
}

func newPublishFixture(t *testing.T) string {
	t.Helper()

	temp := t.TempDir()
	repo := filepath.Join(temp, "repo")
	for _, dir := range []string{
		filepath.Join(repo, "tool"),
		filepath.Join(repo, "packaging", "homebrew"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("create fixture dir %s: %v", dir, err)
		}
	}

	copyFixtureFile(t, filepath.Join(repoRoot(t), "tool", "publish.sh"), filepath.Join(repo, "tool", "publish.sh"), 0o755)
	copyFixtureFile(t, filepath.Join(repoRoot(t), "tool", "verify_release_formula.sh"), filepath.Join(repo, "tool", "verify_release_formula.sh"), 0o755)
	copyFixtureFile(t, filepath.Join(repoRoot(t), "CHANGELOG.md"), filepath.Join(repo, "CHANGELOG.md"), 0o644)
	copyFixtureFile(t, filepath.Join(repoRoot(t), "packaging", "homebrew", "rail.rb"), filepath.Join(repo, "packaging", "homebrew", "rail.rb"), 0o644)

	gitRun(t, repo, "init", "-b", "main")
	gitRun(t, repo, "config", "user.email", "rail-release-test@example.invalid")
	gitRun(t, repo, "config", "user.name", "Rail Release Test")
	gitRun(t, repo, "add", ".")
	gitRun(t, repo, "commit", "-m", "test fixture")

	origin := filepath.Join(temp, "origin.git")
	gitRun(t, temp, "init", "--bare", origin)
	gitRun(t, repo, "remote", "add", "origin", origin)
	gitRun(t, repo, "push", "-u", "origin", "main")

	return repo
}

func newPrepareReleaseFixture(t *testing.T) string {
	t.Helper()

	temp := t.TempDir()
	repo := filepath.Join(temp, "repo")
	for _, dir := range []string{
		filepath.Join(repo, "tool"),
		filepath.Join(repo, "packaging", "homebrew"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("create fixture dir %s: %v", dir, err)
		}
	}

	copyFixtureFile(t, filepath.Join(repoRoot(t), "tool", "prepare_release.sh"), filepath.Join(repo, "tool", "prepare_release.sh"), 0o755)
	copyFixtureFile(t, filepath.Join(repoRoot(t), "tool", "verify_release_formula.sh"), filepath.Join(repo, "tool", "verify_release_formula.sh"), 0o755)
	copyFixtureFile(t, filepath.Join(repoRoot(t), "CHANGELOG.md"), filepath.Join(repo, "CHANGELOG.md"), 0o644)
	copyFixtureFile(t, filepath.Join(repoRoot(t), ".goreleaser.yaml"), filepath.Join(repo, ".goreleaser.yaml"), 0o644)
	copyFixtureFile(t, filepath.Join(repoRoot(t), "packaging", "homebrew", "rail.rb"), filepath.Join(repo, "packaging", "homebrew", "rail.rb"), 0o644)

	gitRun(t, repo, "init", "-b", "main")
	gitRun(t, repo, "config", "user.email", "rail-release-test@example.invalid")
	gitRun(t, repo, "config", "user.name", "Rail Release Test")
	gitRun(t, repo, "add", ".")
	gitRun(t, repo, "commit", "-m", "test fixture")

	origin := filepath.Join(temp, "origin.git")
	gitRun(t, temp, "init", "--bare", origin)
	gitRun(t, repo, "remote", "add", "origin", origin)
	gitRun(t, repo, "push", "-u", "origin", "main")

	return repo
}

func copyFixtureFile(t *testing.T, src, dst string, mode os.FileMode) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read fixture source %s: %v", src, err)
	}
	if err := os.WriteFile(dst, data, mode); err != nil {
		t.Fatalf("write fixture file %s: %v", dst, err)
	}
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write executable %s: %v", path, err)
	}
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(output))
	}
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(output))
	}
	return string(output)
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func readFormulaTag(t *testing.T) string {
	t.Helper()
	return readFormulaTagFromContent(t, readRepoFile(t, "packaging", "homebrew", "rail.rb"))
}

func readFormulaTagFromPath(t *testing.T, path string) string {
	t.Helper()
	return readFormulaTagFromContent(t, readFile(t, path))
}

func readFormulaTagFromContent(t *testing.T, formula string) string {
	t.Helper()
	for _, line := range strings.Split(formula, "\n") {
		if strings.Contains(line, `tag: "v`) {
			start := strings.Index(line, `tag: "`)
			if start == -1 {
				continue
			}
			rest := line[start+len(`tag: "`):]
			end := strings.Index(rest, `"`)
			if end == -1 {
				continue
			}
			return rest[:end]
		}
	}
	t.Fatal("could not find Homebrew formula tag")
	return ""
}

func readRepoFile(t *testing.T, parts ...string) string {
	t.Helper()
	path := filepath.Join(append([]string{repoRoot(t)}, parts...)...)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	return string(data)
}
