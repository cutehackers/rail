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

func TestV1ReleaseWorkflowProvisionsGoInsteadOfDart(t *testing.T) {
	workflow := readRepoFile(t, ".github", "workflows", "v1-release-gate.yml")
	for _, expected := range []string{
		"actions/setup-go@v5",
		"go-version-file: go.mod",
	} {
		if !strings.Contains(workflow, expected) {
			t.Fatalf("workflow missing %q", expected)
		}
	}
	if strings.Contains(workflow, "d"+"art-lang/setup-"+("d"+"art")+"@v1") {
		t.Fatalf("workflow still provisions Dart")
	}
}

func TestReleaseWorkflowPublishesGoReleaserArtifactsAndAttestations(t *testing.T) {
	workflow := readRepoFile(t, ".github", "workflows", "release.yml")
	for _, expected := range []string{
		"Prepare release preflight",
		`./tool/prepare_release.sh "$GITHUB_REF_NAME" --preflight-only --allow-existing-tag`,
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
	root := repoRoot(t)
	version := "v99.99.99"
	createTag := exec.Command("git", "tag", version)
	createTag.Dir = root
	if output, err := createTag.CombinedOutput(); err != nil {
		t.Fatalf("create test tag: %v\n%s", err, string(output))
	}
	t.Cleanup(func() {
		deleteTag := exec.Command("git", "tag", "-d", version)
		deleteTag.Dir = root
		_ = deleteTag.Run()
	})

	cmd := exec.Command("bash", "-lc", `HOMEBREW_TAP_GITHUB_TOKEN=dummy ./tool/prepare_release.sh "$VERSION" --preflight-only`)
	cmd.Dir = root
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
	root := repoRoot(t)
	cmd := exec.Command("bash", "-lc", `unset HOMEBREW_TAP_GITHUB_TOKEN; ./tool/prepare_release.sh v9.9.9 --preflight-only --allow-existing-tag`)
	cmd.Dir = root
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
		"CHANGELOG.md",
		"packaging/homebrew/rail.rb",
		`branch="release/${version}"`,
		`git checkout -b "$branch"`,
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
	root := repoRoot(t)
	cmd := exec.Command("bash", "-lc", `HOMEBREW_TAP_GITHUB_TOKEN=dummy ./tool/prepare_release.sh v9.9.9 --preflight-only --allow-existing-tag`)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected missing changelog section to fail")
	}
	if !strings.Contains(string(output), "missing CHANGELOG.md section") {
		t.Fatalf("unexpected output: %s", string(output))
	}
}

func TestPrepareReleaseScriptRejectsTrackedOrPendingDist(t *testing.T) {
	root := repoRoot(t)
	formulaTag := readFormulaTag(t)
	distFile := filepath.Join(root, "dist", "prepare-release-test")
	if err := os.MkdirAll(filepath.Dir(distFile), 0o755); err != nil {
		t.Fatalf("create dist dir: %v", err)
	}
	if err := os.WriteFile(distFile, []byte("generated"), 0o644); err != nil {
		t.Fatalf("write dist sentinel: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(filepath.Join(root, "dist"))
	})

	cmd := exec.Command("bash", "-lc", `HOMEBREW_TAP_GITHUB_TOKEN=dummy ./tool/prepare_release.sh "$FORMULA_TAG" --preflight-only --allow-existing-tag`)
	cmd.Dir = root
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
	root := repoRoot(t)
	formulaTag := readFormulaTag(t)
	dirtyFile := filepath.Join(root, ".prepare-release-dirty-test")
	if err := os.WriteFile(dirtyFile, []byte("dirty"), 0o644); err != nil {
		t.Fatalf("write dirty sentinel: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Remove(dirtyFile)
	})

	cmd := exec.Command("bash", "-lc", `HOMEBREW_TAP_GITHUB_TOKEN=dummy ./tool/prepare_release.sh "$FORMULA_TAG" --preflight-only --allow-existing-tag`)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "FORMULA_TAG="+formulaTag)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected dirty working tree to fail")
	}
	if !strings.Contains(string(output), "working tree is dirty") {
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

func readFormulaTag(t *testing.T) string {
	t.Helper()
	formula := readRepoFile(t, "packaging", "homebrew", "rail.rb")
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
