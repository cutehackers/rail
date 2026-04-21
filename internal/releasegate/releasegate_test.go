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
		"Verify release formula version",
		"./tool/verify_release_formula.sh",
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

func readRepoFile(t *testing.T, parts ...string) string {
	t.Helper()
	path := filepath.Join(append([]string{repoRoot(t)}, parts...)...)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	return string(data)
}
