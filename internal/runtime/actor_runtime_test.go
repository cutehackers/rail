package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"rail/internal/assets"
	"rail/internal/auth"

	"gopkg.in/yaml.v3"
)

func TestPrepareSealedActorRuntimeDropsUserCodexSurface(t *testing.T) {
	artifactDirectory := t.TempDir()
	workingDirectory := t.TempDir()
	fakeBin := t.TempDir()
	fakeCodexPath := filepath.Join(fakeBin, "codex")
	if err := os.WriteFile(fakeCodexPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("failed to write fake codex: %v", err)
	}
	parentHome := filepath.Join(t.TempDir(), "home")
	parentCodexHome := filepath.Join(parentHome, ".codex")
	if err := os.MkdirAll(parentCodexHome, 0o700); err != nil {
		t.Fatalf("failed to create parent codex home: %v", err)
	}
	authHome := testRailCodexAuthHomeWithToken(t, "test-token")

	sealed, err := prepareSealedActorRuntime(defaultTestActorBackend(), testActorCommandSpec(t, artifactDirectory, workingDirectory, "planner"), fakeCodexParentEnv(t, fakeBin, fakeCodexPath,
		"CODEX_HOME="+parentCodexHome,
		"HOME="+parentHome,
		"XDG_CONFIG_HOME=/tmp/xdg-config",
		"XDG_DATA_HOME=/tmp/xdg-data",
		"XDG_CACHE_HOME=/tmp/xdg-cache",
		"RAIL_CODEX_AUTH_HOME="+authHome,
		"OPENAI_ORG_ID=org-id",
		"OPENAI_PROJECT=project-id",
		"HTTPS_PROXY=https://proxy.example",
		"SSL_CERT_FILE=/tmp/cert.pem",
		"RAIL_TEST_INVOCATION_PATH=/tmp/invocation.json",
		"SSH_AUTH_SOCK=/tmp/socket",
		"GIT_CONFIG_GLOBAL=/tmp/gitconfig",
	))
	if err != nil {
		t.Fatalf("prepareSealedActorRuntime returned error: %v", err)
	}

	envMap := envMap(sealed.Env)
	if strings.Contains(envMap["PATH"], fakeBin) {
		t.Fatalf("expected sealed shell PATH to exclude test-only codex directory %q, got %q", fakeBin, envMap["PATH"])
	}
	for _, key := range []string{"CODEX_HOME", "HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "TMPDIR"} {
		value := envMap[key]
		if value == "" {
			t.Fatalf("expected sealed env to set %s, got %v", key, sealed.Env)
		}
		if !strings.HasPrefix(value, filepath.Join(artifactDirectory, "runtime", "01_planner")) {
			t.Fatalf("expected %s to be artifact-local, got %q", key, value)
		}
	}
	for _, forbidden := range []string{parentCodexHome, parentHome, "/tmp/xdg-config", "/tmp/xdg-data", "/tmp/xdg-cache"} {
		if strings.Contains(strings.Join(sealed.Env, "\n"), forbidden) {
			t.Fatalf("expected sealed env to exclude parent path %q, got %v", forbidden, sealed.Env)
		}
	}
	for key, want := range map[string]string{
		"HTTPS_PROXY":               "https://proxy.example",
		"SSL_CERT_FILE":             "/tmp/cert.pem",
		"RAIL_TEST_INVOCATION_PATH": "/tmp/invocation.json",
	} {
		if got := envMap[key]; got != want {
			t.Fatalf("expected %s=%q, got %q", key, want, got)
		}
	}
	for _, forbiddenKey := range []string{"OPENAI_API_KEY", "OPENAI_ORG_ID", "OPENAI_PROJECT", "RAIL_CODEX_AUTH_HOME", "SSH_AUTH_SOCK", "GIT_CONFIG_GLOBAL"} {
		if _, ok := envMap[forbiddenKey]; ok {
			t.Fatalf("expected %s to be removed from sealed env, got %v", forbiddenKey, sealed.Env)
		}
	}
	copiedAuth, err := os.ReadFile(filepath.Join(sealed.CodexHome, auth.CodexAuthFileName))
	if err != nil {
		t.Fatalf("expected Rail Codex auth to be materialized: %v", err)
	}
	if string(copiedAuth) != `{"tokens":"test-token"}` {
		t.Fatalf("unexpected materialized auth content: %q", copiedAuth)
	}
	expectedCommandPath, err := filepath.EvalSymlinks(fakeCodexPath)
	if err != nil {
		t.Fatalf("failed to resolve fake codex path: %v", err)
	}
	if sealed.CommandPath != expectedCommandPath {
		t.Fatalf("expected sealed command path %q, got %q", expectedCommandPath, sealed.CommandPath)
	}
}

func TestActorRuntimeReadinessRejectsMissingCodexOnSealedPath(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	withDefaultTrustedPATHEntriesForTest(t, nil)

	err := CheckActorRuntimeReadinessForDoctor(t.TempDir())
	if err == nil {
		t.Fatalf("expected actor runtime readiness to fail without trusted codex")
	}
	if !strings.Contains(err.Error(), "unsafe_codex_path") {
		t.Fatalf("expected unsafe_codex_path failure, got %v", err)
	}
}

func TestActorRuntimeReadinessAcceptsInternalTestCodex(t *testing.T) {
	fakeBin := t.TempDir()
	fakeCodexPath := filepath.Join(fakeBin, "codex")
	if err := os.WriteFile(fakeCodexPath, []byte("#!/bin/sh\nprintf '%s\\n' '--disable <FEATURE>'\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("failed to write fake codex: %v", err)
	}
	allowFakeCodexForTest(t, fakeCodexPath)
	t.Setenv("PATH", fakeBin)

	if err := CheckActorRuntimeReadinessForDoctor(t.TempDir()); err != nil {
		t.Fatalf("expected actor runtime readiness, got %v", err)
	}
}

func TestActorRuntimeReadinessRejectsCodexWithoutActorFlagSupport(t *testing.T) {
	fakeBin := t.TempDir()
	fakeCodexPath := filepath.Join(fakeBin, "codex")
	script := `#!/usr/bin/env bash
set -euo pipefail
for arg in "$@"; do
  if [[ "$arg" == "--disable" ]]; then
    printf 'unknown option: --disable\n' >&2
    exit 64
  fi
done
printf 'legacy codex help\n'
`
	if err := os.WriteFile(fakeCodexPath, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake codex: %v", err)
	}
	allowFakeCodexForTest(t, fakeCodexPath)
	t.Setenv("PATH", fakeBin)

	err := CheckActorRuntimeReadinessForDoctor(t.TempDir())
	if err == nil {
		t.Fatalf("expected actor runtime readiness to reject unsupported codex CLI")
	}
	if !strings.Contains(err.Error(), "unsupported_codex_cli") {
		t.Fatalf("expected unsupported_codex_cli failure, got %v", err)
	}
}

func TestActorRuntimeReadinessRejectsUnsafeBackendPolicy(t *testing.T) {
	projectRoot := t.TempDir()
	supervisorDirectory := filepath.Join(projectRoot, ".harness", "supervisor")
	if err := os.MkdirAll(supervisorDirectory, 0o755); err != nil {
		t.Fatalf("failed to create supervisor directory: %v", err)
	}
	policy := []byte(`version: 1
execution_environment: local
default_backend: codex_cli
backends:
  codex_cli:
    command: /opt/homebrew/bin/codex
    subcommand: exec
    sandbox: workspace-write
    approval_policy: never
    session_mode: per_actor
    ephemeral: true
    capture_json_events: true
    skip_git_repo_check: true
    ignore_user_config: true
    ignore_rules: true
    capabilities:
      user_skills: disabled
      user_rules: disabled
      plugins: disabled
      mcp: disabled
      hooks: disabled
      shell: allowed
      file_editing: allowed
execution_environments:
  local:
    allowed_sandboxes:
      - workspace-write
`)
	if err := os.WriteFile(filepath.Join(supervisorDirectory, "actor_backend.yaml"), policy, 0o644); err != nil {
		t.Fatalf("failed to write actor backend policy: %v", err)
	}

	err := CheckActorRuntimeReadinessForDoctor(projectRoot)
	if err == nil {
		t.Fatalf("expected actor runtime readiness to reject unsafe backend policy")
	}
	if !strings.Contains(err.Error(), "actor backend command must be codex") {
		t.Fatalf("expected unsafe backend command error, got %v", err)
	}
}

func envMap(env []string) map[string]string {
	values := make(map[string]string, len(env))
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			values[key] = value
		}
	}
	return values
}

func fakeCodexParentEnv(t *testing.T, fakeBin string, fakeCodexPath string, extra ...string) []string {
	t.Helper()
	writeInternalTestCodexMarker(t, fakeCodexPath)
	env := []string{
		"PATH=" + fakeBin,
		"RAIL_INTERNAL_TEST_ALLOW_UNTRUSTED_CODEX_PATH=" + internalTestCodexOverrideValue,
		"RAIL_INTERNAL_TEST_CODEX_PATH=" + fakeCodexPath,
	}
	return append(env, extra...)
}

func testRailCodexAuthHome(t *testing.T) string {
	t.Helper()
	authHome := t.TempDir()
	if err := os.Chmod(authHome, 0o700); err != nil {
		t.Fatalf("chmod fake auth home: %v", err)
	}
	if err := os.WriteFile(filepath.Join(authHome, ".rail-auth-home"), []byte("version: 1\n"), 0o600); err != nil {
		t.Fatalf("write fake rail auth marker: %v", err)
	}
	if err := os.WriteFile(filepath.Join(authHome, "auth.json"), []byte(`{"fake":"auth"}`), 0o600); err != nil {
		t.Fatalf("write fake auth.json: %v", err)
	}
	return authHome
}

func testRailCodexAuthHomeWithToken(t *testing.T, token string) string {
	t.Helper()
	authHome := filepath.Join(t.TempDir(), "rail-codex-auth")
	if err := auth.EnsureCodexAuthHome(authHome); err != nil {
		t.Fatalf("EnsureCodexAuthHome returned error: %v", err)
	}
	authJSON := []byte(`{"tokens":"` + token + `"}`)
	if err := os.WriteFile(filepath.Join(authHome, auth.CodexAuthFileName), authJSON, 0o600); err != nil {
		t.Fatalf("failed to write auth.json: %v", err)
	}
	return authHome
}

func allowFakeCodexForTest(t *testing.T, fakeCodexPath string) {
	t.Helper()
	writeInternalTestCodexMarker(t, fakeCodexPath)
	t.Setenv("RAIL_INTERNAL_TEST_ALLOW_UNTRUSTED_CODEX_PATH", internalTestCodexOverrideValue)
	t.Setenv("RAIL_INTERNAL_TEST_CODEX_PATH", fakeCodexPath)
}

func withDefaultTrustedPATHEntriesForTest(t *testing.T, entries []string) {
	t.Helper()
	previous := defaultTrustedPATHEntriesForSealedActor
	defaultTrustedPATHEntriesForSealedActor = func() []string {
		return append([]string(nil), entries...)
	}
	t.Cleanup(func() {
		defaultTrustedPATHEntriesForSealedActor = previous
	})
}

func writeInternalTestCodexMarker(t *testing.T, fakeCodexPath string) {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(fakeCodexPath)
	if err != nil {
		t.Fatalf("failed to resolve fake codex path: %v", err)
	}
	markerPath := filepath.Join(filepath.Dir(resolved), internalTestCodexMarker)
	if err := os.WriteFile(markerPath, []byte(filepath.Clean(resolved)+"\n"), 0o600); err != nil {
		t.Fatalf("failed to write internal test codex marker: %v", err)
	}
}

func testActorCommandSpec(t *testing.T, artifactDirectory string, workingDirectory string, actorName string) ActorCommandSpec {
	t.Helper()
	return ActorCommandSpec{
		ActorName:         actorName,
		Profile:           ActorProfile{Model: "gpt-5.4-mini", Reasoning: "high"},
		WorkingDirectory:  workingDirectory,
		Prompt:            "prompt",
		LastMessagePath:   filepath.Join(artifactDirectory, "response.json"),
		SchemaPath:        filepath.Join(artifactDirectory, "schema.json"),
		ArtifactDirectory: artifactDirectory,
		ActorRunID:        "01_" + actorName,
	}
}

func TestPrepareSealedActorRuntimeRequiresRailCodexAuth(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	artifactDirectory := t.TempDir()
	workingDirectory := t.TempDir()
	fakeBin := t.TempDir()
	fakeCodexPath := filepath.Join(fakeBin, "codex")
	if err := os.WriteFile(fakeCodexPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("failed to write fake codex: %v", err)
	}
	authHome := filepath.Join(t.TempDir(), "rail-codex-auth")
	if err := auth.EnsureCodexAuthHome(authHome); err != nil {
		t.Fatalf("EnsureCodexAuthHome returned error: %v", err)
	}

	_, err := prepareSealedActorRuntime(defaultTestActorBackend(), testActorCommandSpec(t, artifactDirectory, workingDirectory, "planner"), fakeCodexParentEnv(t, fakeBin, fakeCodexPath,
		"CODEX_HOME="+filepath.Join(t.TempDir(), ".codex"),
		"RAIL_CODEX_AUTH_HOME="+authHome,
	))
	if err == nil {
		t.Fatalf("expected sealed runtime setup to reject missing Rail Codex auth")
	}
	if !strings.Contains(err.Error(), "rail_actor_auth_not_configured") {
		t.Fatalf("expected missing Rail Codex auth violation, got %v", err)
	}
}

func TestPrepareSealedActorRuntimeMaterializesRailCodexAuth(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	artifactDirectory := t.TempDir()
	workingDirectory := t.TempDir()
	fakeBin := t.TempDir()
	fakeCodexPath := filepath.Join(fakeBin, "codex")
	if err := os.WriteFile(fakeCodexPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("failed to write fake codex: %v", err)
	}
	authHome := testRailCodexAuthHomeWithToken(t, "file-token")

	sealed, err := prepareSealedActorRuntime(defaultTestActorBackend(), testActorCommandSpec(t, artifactDirectory, workingDirectory, "planner"), fakeCodexParentEnv(t, fakeBin, fakeCodexPath,
		"RAIL_CODEX_AUTH_HOME="+authHome,
	))
	if err != nil {
		t.Fatalf("prepareSealedActorRuntime returned error: %v", err)
	}
	envMap := envMap(sealed.Env)
	if _, ok := envMap["OPENAI_API_KEY"]; ok {
		t.Fatalf("expected sealed actor env to omit OPENAI_API_KEY, got %v", sealed.Env)
	}
	if _, ok := envMap["RAIL_CODEX_AUTH_HOME"]; ok {
		t.Fatalf("expected sealed actor env to omit RAIL_CODEX_AUTH_HOME path, got %v", sealed.Env)
	}
	copiedAuth, err := os.ReadFile(filepath.Join(sealed.CodexHome, auth.CodexAuthFileName))
	if err != nil {
		t.Fatalf("expected Rail Codex auth to be materialized: %v", err)
	}
	if string(copiedAuth) != `{"tokens":"file-token"}` {
		t.Fatalf("unexpected materialized auth content: %q", copiedAuth)
	}
	data, err := os.ReadFile(sealed.ProvenancePath)
	if err != nil {
		t.Fatalf("failed to read provenance: %v", err)
	}
	provenance := string(data)
	for _, forbidden := range []string{"file-token", authHome} {
		if strings.Contains(provenance, forbidden) {
			t.Fatalf("expected provenance to omit auth secret/path %q, got:\n%s", forbidden, provenance)
		}
	}
	for _, expected := range []string{"auth_source: rail_codex_login", "auth_materialized: true", "auth.json"} {
		if !strings.Contains(provenance, expected) {
			t.Fatalf("expected provenance to contain %q, got:\n%s", expected, provenance)
		}
	}
}

func TestCodexLaunchesOnlyThroughSealedRuntime(t *testing.T) {
	repoRoot := testRepoRoot(t)
	runtimeFiles, err := filepath.Glob(filepath.Join(repoRoot, "internal", "runtime", "*.go"))
	if err != nil {
		t.Fatalf("failed to list runtime files: %v", err)
	}
	for _, path := range runtimeFiles {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read %s: %v", path, err)
		}
		if strings.Contains(string(data), "exec.CommandContext(ctx, backend.Command") ||
			strings.Contains(string(data), "exec.Command(backend.Command") ||
			strings.Contains(string(data), `exec.CommandContext(ctx, "codex"`) ||
			strings.Contains(string(data), `exec.Command("codex"`) {
			t.Fatalf("codex actor backend must launch through sealed command path, found direct codex launch in %s", path)
		}
	}
}

func TestPrepareSealedActorRuntimeRejectsUnsafeCodexPath(t *testing.T) {
	artifactDirectory := t.TempDir()
	workingDirectory := t.TempDir()
	parentHome := filepath.Join(t.TempDir(), "home")
	unsafeBin := filepath.Join(parentHome, ".codex", "bin")
	if err := os.MkdirAll(unsafeBin, 0o700); err != nil {
		t.Fatalf("failed to create unsafe bin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(unsafeBin, "codex"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("failed to write fake codex: %v", err)
	}

	_, err := prepareSealedActorRuntime(defaultTestActorBackend(), testActorCommandSpec(t, artifactDirectory, workingDirectory, "planner"), []string{
		"PATH=" + unsafeBin,
		"RAIL_CODEX_AUTH_HOME=" + testRailCodexAuthHome(t),
	})
	if err == nil {
		t.Fatalf("expected sealed runtime setup to reject codex under .codex")
	}
	if !strings.Contains(err.Error(), "unsafe_codex_path") {
		t.Fatalf("expected unsafe codex path violation, got %v", err)
	}
}

func TestPrepareSealedActorRuntimeRejectsCodexUnderParentHome(t *testing.T) {
	artifactDirectory := t.TempDir()
	workingDirectory := t.TempDir()
	parentHome := filepath.Join(t.TempDir(), "home")
	userBin := filepath.Join(parentHome, "bin")
	if err := os.MkdirAll(userBin, 0o700); err != nil {
		t.Fatalf("failed to create user bin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(userBin, "codex"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("failed to write fake codex: %v", err)
	}

	_, err := prepareSealedActorRuntime(defaultTestActorBackend(), testActorCommandSpec(t, artifactDirectory, workingDirectory, "planner"), []string{
		"PATH=" + userBin,
		"HOME=" + parentHome,
		"RAIL_CODEX_AUTH_HOME=" + testRailCodexAuthHome(t),
	})
	if err == nil {
		t.Fatalf("expected sealed runtime setup to reject codex under parent HOME")
	}
	if !strings.Contains(err.Error(), "unsafe_codex_path") {
		t.Fatalf("expected unsafe codex path violation, got %v", err)
	}
}

func TestSanitizeActorPATHDropsPrivateTempCodexDirectory(t *testing.T) {
	fakeBin := t.TempDir()
	if err := os.WriteFile(filepath.Join(fakeBin, "codex"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("failed to write fake codex: %v", err)
	}

	pathEntries, err := sanitizeActorPATH(fakeBin, nil)
	if err != nil && !strings.Contains(err.Error(), "unsafe_codex_path") {
		t.Fatalf("expected unsafe path violation or trusted fallback, got %v", err)
	}
	for _, entry := range pathEntries {
		if pathIsWithin(entry, fakeBin) {
			t.Fatalf("expected private temp codex directory to be omitted from sealed PATH, got %v", pathEntries)
		}
	}
}

func TestResolveTestCodexCommandRequiresInternalMarker(t *testing.T) {
	fakeBin := t.TempDir()
	fakeCodexPath := filepath.Join(fakeBin, "codex")
	if err := os.WriteFile(fakeCodexPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("failed to write fake codex: %v", err)
	}

	_, ok, err := resolveTestCodexCommand(map[string]string{
		"RAIL_INTERNAL_TEST_ALLOW_UNTRUSTED_CODEX_PATH": internalTestCodexOverrideValue,
		"RAIL_INTERNAL_TEST_CODEX_PATH":                 fakeCodexPath,
	})
	if !ok {
		t.Fatalf("expected internal test override to be recognized")
	}
	if err == nil || !strings.Contains(err.Error(), "internal test codex marker is missing") {
		t.Fatalf("expected missing marker violation, got %v", err)
	}
}

func TestPrepareSealedActorRuntimeProvenanceDoesNotExposeSecretValues(t *testing.T) {
	artifactDirectory := t.TempDir()
	workingDirectory := t.TempDir()
	fakeBin := t.TempDir()
	fakeCodexPath := filepath.Join(fakeBin, "codex")
	if err := os.WriteFile(fakeCodexPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("failed to write fake codex: %v", err)
	}
	authHome := testRailCodexAuthHomeWithToken(t, "super-secret-token")

	sealed, err := prepareSealedActorRuntime(defaultTestActorBackend(), testActorCommandSpec(t, artifactDirectory, workingDirectory, "planner"), fakeCodexParentEnv(t, fakeBin, fakeCodexPath,
		"RAIL_CODEX_AUTH_HOME="+authHome,
		"HTTPS_PROXY=https://user:password@proxy.example",
	))
	if err != nil {
		t.Fatalf("prepareSealedActorRuntime returned error: %v", err)
	}
	data, err := os.ReadFile(sealed.ProvenancePath)
	if err != nil {
		t.Fatalf("failed to read provenance: %v", err)
	}
	provenance := string(data)
	for _, forbidden := range []string{"super-secret-token", authHome, "user:password"} {
		if strings.Contains(provenance, forbidden) {
			t.Fatalf("expected provenance to redact secret value %q, got:\n%s", forbidden, provenance)
		}
	}
	for _, expected := range []string{"HTTPS_PROXY", "command_path:", "auth_source: rail_codex_login", "auth_materialized: true", "auth.json"} {
		if !strings.Contains(provenance, expected) {
			t.Fatalf("expected provenance to contain %q, got:\n%s", expected, provenance)
		}
	}
}

func TestRunCommandStopsWhenActorWatchdogSeesNoProgress(t *testing.T) {
	workingDirectory := t.TempDir()
	fakeBin := t.TempDir()
	logPath := filepath.Join(workingDirectory, "response.json")
	schemaPath := filepath.Join(workingDirectory, "schema.json")
	fakeCodexPath := filepath.Join(fakeBin, "codex")

	script := `#!/usr/bin/env python3
import time

time.sleep(0.5)
`
	if err := os.WriteFile(fakeCodexPath, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake codex: %v", err)
	}

	defaultActorWatchdogConfig = ActorWatchdogConfig{
		QuietWindow: 25 * time.Millisecond,
		CheckEvery:  5 * time.Millisecond,
	}
	t.Cleanup(func() {
		defaultActorWatchdogConfig = productionActorWatchdogConfig()
	})
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	allowFakeCodexForTest(t, fakeCodexPath)
	t.Setenv("RAIL_CODEX_AUTH_HOME", testRailCodexAuthHome(t))

	_, err := runCommand(defaultTestActorBackend(), ActorCommandSpec{
		ActorName:         "planner",
		Profile:           ActorProfile{Model: "gpt-5.4-mini", Reasoning: "high"},
		WorkingDirectory:  workingDirectory,
		Prompt:            "prompt",
		LastMessagePath:   logPath,
		SchemaPath:        schemaPath,
		ArtifactDirectory: workingDirectory,
		ActorRunID:        "01_planner",
	})
	if err == nil {
		t.Fatalf("expected runCommand to fail when actor watchdog sees no progress")
	}
	if !strings.Contains(err.Error(), "actor_watchdog_expired") {
		t.Fatalf("expected actor_watchdog_expired error, got %v", err)
	}
}

func TestRunCommandKeepsRunningWhenActorWatchdogSeesProgress(t *testing.T) {
	workingDirectory := t.TempDir()
	fakeBin := t.TempDir()
	logPath := filepath.Join(workingDirectory, "response.json")
	schemaPath := filepath.Join(workingDirectory, "schema.json")
	fakeCodexPath := filepath.Join(fakeBin, "codex")

	script := `#!/usr/bin/env bash
set -euo pipefail

output_path=""
while (($# > 0)); do
  if [[ "$1" == "--output-last-message" ]]; then
    shift
    output_path="$1"
  fi
  shift || true
done

for index in 1 2 3 4 5 6; do
  printf 'progress %s\n' "$index"
  sleep 0.2
done

printf '{"summary":"ok"}' >"$output_path"
`
	if err := os.WriteFile(fakeCodexPath, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake codex: %v", err)
	}

	defaultActorWatchdogConfig = ActorWatchdogConfig{
		QuietWindow: 3 * time.Second,
		CheckEvery:  25 * time.Millisecond,
	}
	t.Cleanup(func() {
		defaultActorWatchdogConfig = productionActorWatchdogConfig()
	})
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	allowFakeCodexForTest(t, fakeCodexPath)
	t.Setenv("RAIL_CODEX_AUTH_HOME", testRailCodexAuthHome(t))

	response, err := runCommand(defaultTestActorBackend(), ActorCommandSpec{
		ActorName:         "planner",
		Profile:           ActorProfile{Model: "gpt-5.4-mini", Reasoning: "high"},
		WorkingDirectory:  workingDirectory,
		Prompt:            "prompt",
		LastMessagePath:   logPath,
		SchemaPath:        schemaPath,
		ArtifactDirectory: workingDirectory,
		ActorRunID:        "01_planner",
	})
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}
	if got, want := response["summary"], "ok"; got != want {
		t.Fatalf("unexpected actor response summary: got %#v want %#v", got, want)
	}
}

func TestRunCommandUsesNormalizedActorProfileWithoutEnvFallbacks(t *testing.T) {
	workingDirectory := t.TempDir()
	fakeBin := t.TempDir()
	invocationPath := filepath.Join(workingDirectory, "invocation.json")
	logPath := filepath.Join(workingDirectory, "response.json")
	schemaPath := filepath.Join(workingDirectory, "schema.json")
	fakeCodexPath := filepath.Join(fakeBin, "codex")

	script := `#!/usr/bin/env python3
import json
import os
import re
import sys

invocation_path = os.environ["RAIL_TEST_INVOCATION_PATH"]
output_path = None
model = ""
reasoning = ""

for index, value in enumerate(sys.argv):
    if value == "--output-last-message" and index + 1 < len(sys.argv):
        output_path = sys.argv[index + 1]
    if value == "-m" and index + 1 < len(sys.argv):
        model = sys.argv[index + 1]
    if value == "-c" and index + 1 < len(sys.argv):
        match = re.match(r'model_reasoning_effort="([^"]+)"', sys.argv[index + 1])
        if match:
            reasoning = match.group(1)

with open(invocation_path, "w", encoding="utf-8") as handle:
    json.dump({"model": model, "reasoning": reasoning}, handle)

with open(output_path, "w", encoding="utf-8") as handle:
    json.dump({"summary": "ok"}, handle)
`
	if err := os.WriteFile(fakeCodexPath, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake codex: %v", err)
	}

	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	allowFakeCodexForTest(t, fakeCodexPath)
	t.Setenv("RAIL_CODEX_AUTH_HOME", testRailCodexAuthHome(t))
	t.Setenv("RAIL_ACTOR_MODEL", "wrong-model")
	t.Setenv("RAIL_ACTOR_REASONING_EFFORT", "low")
	t.Setenv("RAIL_TEST_INVOCATION_PATH", invocationPath)

	response, err := runCommand(defaultTestActorBackend(), ActorCommandSpec{
		ActorName:         "planner",
		Profile:           ActorProfile{Model: " gpt-5.4-mini ", Reasoning: " high "},
		WorkingDirectory:  workingDirectory,
		Prompt:            "prompt",
		LastMessagePath:   logPath,
		SchemaPath:        schemaPath,
		ArtifactDirectory: workingDirectory,
		ActorRunID:        "01_planner",
	})
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}
	if got, want := response["summary"], "ok"; got != want {
		t.Fatalf("unexpected actor response summary: got %#v want %#v", got, want)
	}

	data, err := os.ReadFile(invocationPath)
	if err != nil {
		t.Fatalf("failed to read invocation log: %v", err)
	}

	var invocation struct {
		Model     string `json:"model"`
		Reasoning string `json:"reasoning"`
	}
	if err := json.Unmarshal(data, &invocation); err != nil {
		t.Fatalf("failed to decode invocation log: %v", err)
	}
	if invocation.Model != "gpt-5.4-mini" {
		t.Fatalf("expected runCommand to use normalized profile model, got %q", invocation.Model)
	}
	if invocation.Reasoning != "high" {
		t.Fatalf("expected runCommand to use normalized profile reasoning, got %q", invocation.Reasoning)
	}
}

func TestBuildCodexCLIArgsUsesBackendPolicy(t *testing.T) {
	workingDirectory := t.TempDir()
	logPath := filepath.Join(workingDirectory, "response.json")
	schemaPath := filepath.Join(workingDirectory, "schema.json")

	args := buildCodexCLIArgs(defaultTestActorBackend(), ActorCommandSpec{
		ActorName:        "planner",
		Profile:          ActorProfile{Model: "gpt-5.4-mini", Reasoning: "high"},
		WorkingDirectory: workingDirectory,
		Prompt:           "prompt",
		LastMessagePath:  logPath,
		SchemaPath:       schemaPath,
	})

	want := []string{
		"exec",
		"-m", "gpt-5.4-mini",
		"--cd", workingDirectory,
		"--ephemeral",
		"--color", "never",
		"-s", "workspace-write",
		"--skip-git-repo-check",
		"--ignore-user-config",
		"--ignore-rules",
		"-c", `model_reasoning_effort="high"`,
		"-c", `approval_policy="never"`,
		"-c", `shell_environment_policy.inherit="none"`,
		"-c", `shell_environment_policy.include_only=["PATH","HOME","TMPDIR","TMP","TEMP","XDG_CONFIG_HOME","XDG_DATA_HOME","XDG_CACHE_HOME"]`,
		"--disable", "plugins",
		"--disable", "codex_hooks",
		"--output-schema", schemaPath,
		"--output-last-message", logPath,
		"prompt",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("unexpected codex args:\ngot  %#v\nwant %#v", args, want)
	}
}

func TestCodexCLIExecutorBuildsExpectedArgs(t *testing.T) {
	workingDirectory := t.TempDir()
	logPath := filepath.Join(workingDirectory, "response.json")
	schemaPath := filepath.Join(workingDirectory, "schema.json")
	eventsPath := filepath.Join(workingDirectory, "events.jsonl")
	backend := defaultTestActorBackend()
	backend.CaptureJSONEvents = true

	args := buildCodexCLIArgsForInvocation(ActorInvocation{
		ActorName:         "planner",
		ActorRunID:        "01_planner",
		WorkingDirectory:  workingDirectory,
		ArtifactDirectory: workingDirectory,
		Prompt:            "prompt",
		OutputSchemaPath:  schemaPath,
		LastMessagePath:   logPath,
		EventsPath:        eventsPath,
		Profile:           ActorProfile{Model: "gpt-5.4-mini", Reasoning: "high"},
		Policy:            backend,
	})

	want := []string{
		"exec",
		"-m", "gpt-5.4-mini",
		"--cd", workingDirectory,
		"--ephemeral",
		"--color", "never",
		"-s", "workspace-write",
		"--skip-git-repo-check",
		"--ignore-user-config",
		"--ignore-rules",
		"-c", `model_reasoning_effort="high"`,
		"-c", `approval_policy="never"`,
		"-c", `shell_environment_policy.inherit="none"`,
		"-c", `shell_environment_policy.include_only=["PATH","HOME","TMPDIR","TMP","TEMP","XDG_CONFIG_HOME","XDG_DATA_HOME","XDG_CACHE_HOME"]`,
		"--disable", "plugins",
		"--disable", "codex_hooks",
		"--output-schema", schemaPath,
		"--output-last-message", logPath,
		"--json",
		"prompt",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("unexpected codex backend args:\ngot  %#v\nwant %#v", args, want)
	}
}

func TestCodexCLIExecutorPreservesCallerCancellation(t *testing.T) {
	workingDirectory := t.TempDir()
	fakeBin := t.TempDir()
	logPath := filepath.Join(workingDirectory, "response.json")
	schemaPath := filepath.Join(workingDirectory, "schema.json")
	eventsPath := filepath.Join(workingDirectory, "events.jsonl")
	fakeCodexPath := filepath.Join(fakeBin, "codex")

	script := `#!/usr/bin/env python3
import time

print("not-json", flush=True)
time.sleep(5)
`
	if err := os.WriteFile(fakeCodexPath, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake codex: %v", err)
	}

	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	allowFakeCodexForTest(t, fakeCodexPath)
	t.Setenv("RAIL_CODEX_AUTH_HOME", testRailCodexAuthHome(t))

	backend := defaultTestActorBackend()
	backend.CaptureJSONEvents = true
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := (CodexCLIExecutor{}).RunActor(ctx, ActorInvocation{
		ActorName:         "planner",
		ActorRunID:        "01_planner",
		WorkingDirectory:  workingDirectory,
		ArtifactDirectory: workingDirectory,
		Prompt:            "prompt",
		OutputSchemaPath:  schemaPath,
		LastMessagePath:   logPath,
		EventsPath:        eventsPath,
		Profile:           ActorProfile{Model: "gpt-5.4-mini", Reasoning: "high"},
		Policy:            backend,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected caller cancellation to win over event audit errors, got %v", err)
	}
}

func TestCodexCLIExecutorWritesRuntimeEvidence(t *testing.T) {
	artifactDirectory := t.TempDir()
	workingDirectory := t.TempDir()
	runsDirectory := filepath.Join(artifactDirectory, "runs")
	if err := os.MkdirAll(runsDirectory, 0o755); err != nil {
		t.Fatalf("failed to create runs directory: %v", err)
	}
	fakeBin := t.TempDir()
	logPath := filepath.Join(runsDirectory, "01_planner-last-message.txt")
	schemaPath := filepath.Join(runsDirectory, "01_planner-output-schema.json")
	eventsPath := filepath.Join(runsDirectory, "01_planner-events.jsonl")
	fakeCodexPath := filepath.Join(fakeBin, "codex")

	script := `#!/usr/bin/env python3
import json
import sys

output_path = None
for index, value in enumerate(sys.argv):
    if value == "--output-last-message" and index + 1 < len(sys.argv):
        output_path = sys.argv[index + 1]
        break

print('{"event":"started"}')
with open(output_path, "w", encoding="utf-8") as handle:
    json.dump({"summary": "ok"}, handle)
`
	if err := os.WriteFile(fakeCodexPath, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake codex: %v", err)
	}

	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	allowFakeCodexForTest(t, fakeCodexPath)
	t.Setenv("RAIL_CODEX_AUTH_HOME", testRailCodexAuthHome(t))

	backend := defaultTestActorBackend()
	backend.CaptureJSONEvents = true
	result, err := (CodexCLIExecutor{}).RunActor(context.Background(), ActorInvocation{
		ActorName:         "planner",
		ActorRunID:        "01_planner",
		WorkingDirectory:  workingDirectory,
		ArtifactDirectory: artifactDirectory,
		Prompt:            "prompt",
		OutputSchemaPath:  schemaPath,
		LastMessagePath:   logPath,
		EventsPath:        eventsPath,
		Profile:           ActorProfile{Model: "gpt-5.4-mini", Reasoning: "high"},
		Policy:            backend,
	})
	if err != nil {
		t.Fatalf("RunActor returned error: %v", err)
	}

	evidencePath := filepath.Join(runsDirectory, "01_planner-runtime-evidence.yaml")
	if result.RuntimeEvidencePath != evidencePath {
		t.Fatalf("RuntimeEvidencePath: got %q want %q", result.RuntimeEvidencePath, evidencePath)
	}
	eventsInfo, err := os.Stat(eventsPath)
	if err != nil {
		t.Fatalf("failed to stat events log: %v", err)
	}
	if got, want := eventsInfo.Mode().Perm(), os.FileMode(0o600); got != want {
		t.Fatalf("events log permissions: got %v want %v", got, want)
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatalf("failed to read runtime evidence: %v", err)
	}

	var evidence struct {
		SchemaVersion int      `yaml:"schema_version"`
		BackendType   string   `yaml:"backend_type"`
		Actor         string   `yaml:"actor"`
		ActorRunID    string   `yaml:"actor_run_id"`
		Status        string   `yaml:"status"`
		RawEventLog   string   `yaml:"raw_event_log"`
		Provenance    string   `yaml:"provenance"`
		Violations    []string `yaml:"policy_violations"`
		Policy        struct {
			Sandbox string `yaml:"sandbox"`
		} `yaml:"policy"`
		Redaction struct {
			SecretValuesWritten bool `yaml:"secret_values_written"`
		} `yaml:"redaction"`
	}
	if err := yaml.Unmarshal(data, &evidence); err != nil {
		t.Fatalf("failed to decode runtime evidence: %v\n%s", err, data)
	}
	if evidence.SchemaVersion != 1 {
		t.Fatalf("schema_version: got %d want 1", evidence.SchemaVersion)
	}
	if evidence.BackendType != "codex_cli" {
		t.Fatalf("backend_type: got %q want codex_cli", evidence.BackendType)
	}
	if evidence.Actor != "planner" {
		t.Fatalf("actor: got %q want planner", evidence.Actor)
	}
	if evidence.ActorRunID != "01_planner" {
		t.Fatalf("actor_run_id: got %q want 01_planner", evidence.ActorRunID)
	}
	if evidence.Status != "passed" {
		t.Fatalf("status: got %q want passed", evidence.Status)
	}
	if evidence.RawEventLog != "runs/01_planner-events.jsonl" {
		t.Fatalf("raw_event_log: got %q", evidence.RawEventLog)
	}
	if evidence.Provenance != "runtime/01_planner/actor_environment.yaml" {
		t.Fatalf("provenance: got %q", evidence.Provenance)
	}
	if evidence.Policy.Sandbox != "workspace-write" {
		t.Fatalf("policy.sandbox: got %q want workspace-write", evidence.Policy.Sandbox)
	}
	if evidence.Redaction.SecretValuesWritten {
		t.Fatalf("redaction.secret_values_written: got true want false")
	}
	if evidence.Violations == nil || len(evidence.Violations) != 0 {
		t.Fatalf("policy_violations: got %#v want empty list", evidence.Violations)
	}
}

func TestCodexCLIExecutorRejectsEscapedInvocationPaths(t *testing.T) {
	artifactDirectory := t.TempDir()
	runsDirectory := filepath.Join(artifactDirectory, "runs")
	if err := os.MkdirAll(runsDirectory, 0o755); err != nil {
		t.Fatalf("failed to create runs directory: %v", err)
	}
	backend := defaultTestActorBackend()
	backend.CaptureJSONEvents = true

	baseInvocation := ActorInvocation{
		ActorName:         "planner",
		ActorRunID:        "01_planner",
		WorkingDirectory:  t.TempDir(),
		ArtifactDirectory: artifactDirectory,
		Prompt:            "prompt",
		OutputSchemaPath:  filepath.Join(runsDirectory, "01_planner-output-schema.json"),
		LastMessagePath:   filepath.Join(runsDirectory, "01_planner-last-message.txt"),
		EventsPath:        filepath.Join(runsDirectory, "01_planner-events.jsonl"),
		Profile:           ActorProfile{Model: "gpt-5.4-mini", Reasoning: "high"},
		Policy:            backend,
	}

	for _, tc := range []struct {
		name   string
		mutate func(*ActorInvocation, string)
	}{
		{
			name: "output_schema",
			mutate: func(invocation *ActorInvocation, outsidePath string) {
				invocation.OutputSchemaPath = outsidePath
			},
		},
		{
			name: "last_message",
			mutate: func(invocation *ActorInvocation, outsidePath string) {
				invocation.LastMessagePath = outsidePath
			},
		},
		{
			name: "events",
			mutate: func(invocation *ActorInvocation, outsidePath string) {
				invocation.EventsPath = outsidePath
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outsidePath := filepath.Join(t.TempDir(), "escaped")
			invocation := baseInvocation
			tc.mutate(&invocation, outsidePath)

			_, err := (CodexCLIExecutor{}).RunActor(context.Background(), invocation)
			if err == nil {
				t.Fatalf("expected escaped %s path to be rejected", tc.name)
			}
			if !strings.Contains(err.Error(), "escapes artifact directory") {
				t.Fatalf("expected containment error, got %v", err)
			}
			if _, statErr := os.Stat(outsidePath); !os.IsNotExist(statErr) {
				t.Fatalf("expected escaped path not to be created, stat err=%v", statErr)
			}
		})
	}
}

func TestCodexCLIExecutorRejectsSymlinkEscapedInvocationPaths(t *testing.T) {
	artifactDirectory := t.TempDir()
	outsideRunsDirectory := t.TempDir()
	runsDirectory := filepath.Join(artifactDirectory, "runs")
	if err := os.Symlink(outsideRunsDirectory, runsDirectory); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	backend := defaultTestActorBackend()
	backend.CaptureJSONEvents = true

	_, err := (CodexCLIExecutor{}).RunActor(context.Background(), ActorInvocation{
		ActorName:         "planner",
		ActorRunID:        "01_planner",
		WorkingDirectory:  t.TempDir(),
		ArtifactDirectory: artifactDirectory,
		Prompt:            "prompt",
		OutputSchemaPath:  filepath.Join(runsDirectory, "01_planner-output-schema.json"),
		LastMessagePath:   filepath.Join(runsDirectory, "01_planner-last-message.txt"),
		EventsPath:        filepath.Join(runsDirectory, "01_planner-events.jsonl"),
		Profile:           ActorProfile{Model: "gpt-5.4-mini", Reasoning: "high"},
		Policy:            backend,
	})
	if err == nil {
		t.Fatalf("expected symlink-escaped runs directory to be rejected")
	}
	if !strings.Contains(err.Error(), "escapes artifact directory") {
		t.Fatalf("expected containment error, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(outsideRunsDirectory, "01_planner-events.jsonl")); !os.IsNotExist(statErr) {
		t.Fatalf("expected outside events log not to be created, stat err=%v", statErr)
	}
}

func TestCodexCLIExecutorDoesNotWritePassedEvidenceForMalformedOutput(t *testing.T) {
	artifactDirectory := t.TempDir()
	workingDirectory := t.TempDir()
	runsDirectory := filepath.Join(artifactDirectory, "runs")
	if err := os.MkdirAll(runsDirectory, 0o755); err != nil {
		t.Fatalf("failed to create runs directory: %v", err)
	}
	fakeBin := t.TempDir()
	logPath := filepath.Join(runsDirectory, "01_planner-last-message.txt")
	schemaPath := filepath.Join(runsDirectory, "01_planner-output-schema.json")
	eventsPath := filepath.Join(runsDirectory, "01_planner-events.jsonl")
	fakeCodexPath := filepath.Join(fakeBin, "codex")

	script := `#!/usr/bin/env python3
import sys

output_path = None
for index, value in enumerate(sys.argv):
    if value == "--output-last-message" and index + 1 < len(sys.argv):
        output_path = sys.argv[index + 1]
        break

print('{"event":"started"}')
with open(output_path, "w", encoding="utf-8") as handle:
    handle.write("not-json")
`
	if err := os.WriteFile(fakeCodexPath, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake codex: %v", err)
	}

	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	allowFakeCodexForTest(t, fakeCodexPath)
	t.Setenv("RAIL_CODEX_AUTH_HOME", testRailCodexAuthHome(t))

	backend := defaultTestActorBackend()
	backend.CaptureJSONEvents = true
	_, err := (CodexCLIExecutor{}).RunActor(context.Background(), ActorInvocation{
		ActorName:         "planner",
		ActorRunID:        "01_planner",
		WorkingDirectory:  workingDirectory,
		ArtifactDirectory: artifactDirectory,
		Prompt:            "prompt",
		OutputSchemaPath:  schemaPath,
		LastMessagePath:   logPath,
		EventsPath:        eventsPath,
		Profile:           ActorProfile{Model: "gpt-5.4-mini", Reasoning: "high"},
		Policy:            backend,
	})
	if err == nil {
		t.Fatalf("expected malformed actor output to fail")
	}
	if !strings.Contains(err.Error(), "decode planner actor response") {
		t.Fatalf("expected decode error, got %v", err)
	}
	evidencePath := filepath.Join(runsDirectory, "01_planner-runtime-evidence.yaml")
	if _, statErr := os.Stat(evidencePath); !os.IsNotExist(statErr) {
		t.Fatalf("expected passed runtime evidence not to be written, stat err=%v", statErr)
	}
}

func TestBuildCodexCLIArgsBlocksSecretEnvFromActorShell(t *testing.T) {
	workingDirectory := t.TempDir()
	args := buildCodexCLIArgs(defaultTestActorBackend(), ActorCommandSpec{
		ActorName:        "planner",
		Profile:          ActorProfile{Model: "gpt-5.4-mini", Reasoning: "high"},
		WorkingDirectory: workingDirectory,
		Prompt:           "prompt",
		LastMessagePath:  filepath.Join(workingDirectory, "response.json"),
		SchemaPath:       filepath.Join(workingDirectory, "schema.json"),
	})
	joined := strings.Join(args, "\n")
	if !strings.Contains(joined, `shell_environment_policy.inherit="none"`) {
		t.Fatalf("expected shell env inheritance to be disabled, got %#v", args)
	}
	for _, forbidden := range []string{"OPENAI_API_KEY", "RAIL_CODEX_AUTH_HOME", "RAIL_ACTOR_AUTH_FILE"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("expected shell env include list to omit auth variable %s, got %#v", forbidden, args)
		}
	}
}

func TestBuildCodexCLIArgsDisablesConfiguredCapabilities(t *testing.T) {
	workingDirectory := t.TempDir()
	args := buildCodexCLIArgs(defaultTestActorBackend(), ActorCommandSpec{
		ActorName:        "planner",
		Profile:          ActorProfile{Model: "gpt-5.4-mini", Reasoning: "high"},
		WorkingDirectory: workingDirectory,
		Prompt:           "prompt",
		LastMessagePath:  filepath.Join(workingDirectory, "response.json"),
		SchemaPath:       filepath.Join(workingDirectory, "schema.json"),
	})
	joined := strings.Join(args, "\n")
	for _, want := range []string{"--disable\nplugins", "--disable\ncodex_hooks"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected disabled capability flag %q, got %#v", want, args)
		}
	}
}

func TestRunCommandRequiresBackendSpecSignature(t *testing.T) {
	runCommandType := reflect.TypeOf(runCommand)
	if runCommandType.IsVariadic() {
		t.Fatalf("runCommand must not accept legacy variadic arguments")
	}
	if got, want := runCommandType.NumIn(), 2; got != want {
		t.Fatalf("runCommand input count: got %d want %d", got, want)
	}
	if runCommandType.In(0) != reflect.TypeOf(ActorBackendConfig{}) {
		t.Fatalf("runCommand first input: got %v want ActorBackendConfig", runCommandType.In(0))
	}
	if runCommandType.In(1) != reflect.TypeOf(ActorCommandSpec{}) {
		t.Fatalf("runCommand second input: got %v want ActorCommandSpec", runCommandType.In(1))
	}
}

func TestRunCommandUsesBackendPolicyForCodexInvocation(t *testing.T) {
	workingDirectory := t.TempDir()
	fakeBin := t.TempDir()
	invocationPath := filepath.Join(workingDirectory, "invocation.json")
	logPath := filepath.Join(workingDirectory, "response.json")
	schemaPath := filepath.Join(workingDirectory, "schema.json")
	eventsPath := filepath.Join(workingDirectory, "events.jsonl")
	fakeCodexPath := filepath.Join(fakeBin, "codex")

	script := `#!/usr/bin/env python3
import json
import os
import sys

invocation_path = os.environ["RAIL_TEST_INVOCATION_PATH"]
output_path = None

for index, value in enumerate(sys.argv):
    if value == "--output-last-message" and index + 1 < len(sys.argv):
        output_path = sys.argv[index + 1]

with open(invocation_path, "w", encoding="utf-8") as handle:
    json.dump(sys.argv[1:], handle)

print('{"event":"started"}')

with open(output_path, "w", encoding="utf-8") as handle:
    json.dump({"summary": "ok"}, handle)
`
	if err := os.WriteFile(fakeCodexPath, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake codex: %v", err)
	}

	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	allowFakeCodexForTest(t, fakeCodexPath)
	t.Setenv("RAIL_CODEX_AUTH_HOME", testRailCodexAuthHome(t))
	t.Setenv("RAIL_TEST_INVOCATION_PATH", invocationPath)

	backend := defaultTestActorBackend()
	backend.CaptureJSONEvents = true
	response, err := runCommand(backend, ActorCommandSpec{
		ActorName:         "planner",
		Profile:           ActorProfile{Model: "gpt-5.4-mini", Reasoning: "high"},
		WorkingDirectory:  workingDirectory,
		Prompt:            "prompt",
		LastMessagePath:   logPath,
		SchemaPath:        schemaPath,
		EventsPath:        eventsPath,
		ArtifactDirectory: workingDirectory,
		ActorRunID:        "01_planner",
	})
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}
	if got, want := response["summary"], "ok"; got != want {
		t.Fatalf("unexpected actor response summary: got %#v want %#v", got, want)
	}

	data, err := os.ReadFile(invocationPath)
	if err != nil {
		t.Fatalf("failed to read invocation log: %v", err)
	}

	var invocation []string
	if err := json.Unmarshal(data, &invocation); err != nil {
		t.Fatalf("failed to decode invocation log: %v", err)
	}

	want := []string{
		"exec",
		"-m", "gpt-5.4-mini",
		"--cd", workingDirectory,
		"--ephemeral",
		"--color", "never",
		"-s", "workspace-write",
		"--skip-git-repo-check",
		"--ignore-user-config",
		"--ignore-rules",
		"-c", `model_reasoning_effort="high"`,
		"-c", `approval_policy="never"`,
		"-c", `shell_environment_policy.inherit="none"`,
		"-c", `shell_environment_policy.include_only=["PATH","HOME","TMPDIR","TMP","TEMP","XDG_CONFIG_HOME","XDG_DATA_HOME","XDG_CACHE_HOME"]`,
		"--disable", "plugins",
		"--disable", "codex_hooks",
		"--output-schema", schemaPath,
		"--output-last-message", logPath,
		"--json",
		"prompt",
	}
	if !reflect.DeepEqual(invocation, want) {
		t.Fatalf("unexpected codex invocation:\ngot  %#v\nwant %#v", invocation, want)
	}

	eventsData, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("failed to read events log: %v", err)
	}
	if got, want := strings.TrimSpace(string(eventsData)), `{"event":"started"}`; got != want {
		t.Fatalf("unexpected events log: got %q want %q", got, want)
	}
}

func TestRunCommandAuditsJSONEventsBeforeReturningOutput(t *testing.T) {
	workingDirectory := t.TempDir()
	fakeBin := t.TempDir()
	logPath := filepath.Join(workingDirectory, "response.json")
	schemaPath := filepath.Join(workingDirectory, "schema.json")
	eventsPath := filepath.Join(workingDirectory, "events.jsonl")
	fakeCodexPath := filepath.Join(fakeBin, "codex")

	script := `#!/usr/bin/env python3
import json
import os
import sys

output_path = None
for index, value in enumerate(sys.argv):
    if value == "--output-last-message" and index + 1 < len(sys.argv):
        output_path = sys.argv[index + 1]
        break

print(json.dumps({"type": "item.started", "item": {"type": "command_execution", "command": "sed -n '1,40p' /tmp/.codex/superpowers/skills/using-superpowers/SKILL.md"}}))
with open(output_path, "w", encoding="utf-8") as handle:
    json.dump({"summary": "should not be trusted"}, handle)
`
	if err := os.WriteFile(fakeCodexPath, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake codex: %v", err)
	}

	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	allowFakeCodexForTest(t, fakeCodexPath)
	t.Setenv("RAIL_CODEX_AUTH_HOME", testRailCodexAuthHome(t))

	backend := defaultTestActorBackend()
	backend.CaptureJSONEvents = true
	_, err := runCommand(backend, ActorCommandSpec{
		ActorName:         "planner",
		Profile:           ActorProfile{Model: "gpt-5.4-mini", Reasoning: "high"},
		WorkingDirectory:  workingDirectory,
		Prompt:            "prompt",
		LastMessagePath:   logPath,
		SchemaPath:        schemaPath,
		EventsPath:        eventsPath,
		ArtifactDirectory: workingDirectory,
		ActorRunID:        "01_planner",
	})
	if err == nil {
		t.Fatalf("expected runCommand to reject audited skill injection before returning output")
	}
	if !strings.Contains(err.Error(), "unexpected_skill_injection") {
		t.Fatalf("expected unexpected_skill_injection violation, got %v", err)
	}
}

func TestRunCommandRejectsSealedCodexHomeSkillCreation(t *testing.T) {
	workingDirectory := t.TempDir()
	fakeBin := t.TempDir()
	logPath := filepath.Join(workingDirectory, "response.json")
	schemaPath := filepath.Join(workingDirectory, "schema.json")
	fakeCodexPath := filepath.Join(fakeBin, "codex")

	script := `#!/usr/bin/env python3
import json
import os
import sys

output_path = None
for index, value in enumerate(sys.argv):
    if value == "--output-last-message" and index + 1 < len(sys.argv):
        output_path = sys.argv[index + 1]
        break

skill_dir = os.path.join(os.environ["CODEX_HOME"], "skills", "injected")
os.makedirs(skill_dir, exist_ok=True)
with open(os.path.join(skill_dir, "SKILL.md"), "w", encoding="utf-8") as handle:
    handle.write("unexpected")

with open(output_path, "w", encoding="utf-8") as handle:
    json.dump({"summary": "should not be trusted"}, handle)
`
	if err := os.WriteFile(fakeCodexPath, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake codex: %v", err)
	}

	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	allowFakeCodexForTest(t, fakeCodexPath)
	t.Setenv("RAIL_CODEX_AUTH_HOME", testRailCodexAuthHome(t))

	_, err := runCommand(defaultTestActorBackend(), ActorCommandSpec{
		ActorName:         "planner",
		Profile:           ActorProfile{Model: "gpt-5.4-mini", Reasoning: "high"},
		WorkingDirectory:  workingDirectory,
		Prompt:            "prompt",
		LastMessagePath:   logPath,
		SchemaPath:        schemaPath,
		ArtifactDirectory: workingDirectory,
		ActorRunID:        "01_planner",
	})
	if err == nil {
		t.Fatalf("expected runCommand to reject skill materialization inside sealed CODEX_HOME")
	}
	if !strings.Contains(err.Error(), "unexpected_skill_materialization") {
		t.Fatalf("expected unexpected_skill_materialization violation, got %v", err)
	}
}

func TestRunCommandAllowsMarkedSystemSkillMaterialization(t *testing.T) {
	for _, actorName := range []string{"planner", "context_builder", "critic", "generator", "executor", "evaluator"} {
		t.Run(actorName, func(t *testing.T) {
			workingDirectory := t.TempDir()
			fakeBin := t.TempDir()
			logPath := filepath.Join(workingDirectory, "response.json")
			schemaPath := filepath.Join(workingDirectory, "schema.json")
			fakeCodexPath := filepath.Join(fakeBin, "codex")

			script := `#!/usr/bin/env python3
import json
import os
import sys

output_path = None
for index, value in enumerate(sys.argv):
    if value == "--output-last-message" and index + 1 < len(sys.argv):
        output_path = sys.argv[index + 1]
        break

system_dir = os.path.join(os.environ["CODEX_HOME"], "skills", ".system")
skill_dir = os.path.join(system_dir, "openai-docs")
os.makedirs(skill_dir, exist_ok=True)
with open(os.path.join(skill_dir, "SKILL.md"), "w", encoding="utf-8") as handle:
    handle.write("system skill")

with open(output_path, "w", encoding="utf-8") as handle:
    json.dump({"summary": "ok"}, handle)
`
			if err := os.WriteFile(fakeCodexPath, []byte(script), 0o755); err != nil {
				t.Fatalf("failed to write fake codex: %v", err)
			}

			t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
			allowFakeCodexForTest(t, fakeCodexPath)
			t.Setenv("RAIL_CODEX_AUTH_HOME", testRailCodexAuthHome(t))

			response, err := runCommand(defaultTestActorBackend(), ActorCommandSpec{
				ActorName:         actorName,
				Profile:           ActorProfile{Model: "gpt-5.4-mini", Reasoning: "high"},
				WorkingDirectory:  workingDirectory,
				Prompt:            "prompt",
				LastMessagePath:   logPath,
				SchemaPath:        schemaPath,
				ArtifactDirectory: workingDirectory,
				ActorRunID:        "01_" + actorName,
			})
			if err != nil {
				t.Fatalf("runCommand returned error: %v", err)
			}
			if got, want := response["summary"], "ok"; got != want {
				t.Fatalf("unexpected actor response summary: got %#v want %#v", got, want)
			}
		})
	}
}

func TestRunCommandRejectsUnmarkedSystemSkillMaterialization(t *testing.T) {
	workingDirectory := t.TempDir()
	fakeBin := t.TempDir()
	logPath := filepath.Join(workingDirectory, "response.json")
	schemaPath := filepath.Join(workingDirectory, "schema.json")
	fakeCodexPath := filepath.Join(fakeBin, "codex")

	script := `#!/usr/bin/env python3
import json
import os
import sys

output_path = None
for index, value in enumerate(sys.argv):
    if value == "--output-last-message" and index + 1 < len(sys.argv):
        output_path = sys.argv[index + 1]
        break

system_dir = os.path.join(os.environ["CODEX_HOME"], "skills", ".system")
try:
    os.remove(os.path.join(system_dir, ".rail-system-skills.marker"))
except FileNotFoundError:
    pass
skill_dir = os.path.join(system_dir, "openai-docs")
os.makedirs(skill_dir, exist_ok=True)
with open(os.path.join(skill_dir, "SKILL.md"), "w", encoding="utf-8") as handle:
    handle.write("system skill without marker")

with open(output_path, "w", encoding="utf-8") as handle:
    json.dump({"summary": "should not be trusted"}, handle)
`
	if err := os.WriteFile(fakeCodexPath, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake codex: %v", err)
	}

	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	allowFakeCodexForTest(t, fakeCodexPath)
	t.Setenv("RAIL_CODEX_AUTH_HOME", testRailCodexAuthHome(t))

	_, err := runCommand(defaultTestActorBackend(), ActorCommandSpec{
		ActorName:         "planner",
		Profile:           ActorProfile{Model: "gpt-5.4-mini", Reasoning: "high"},
		WorkingDirectory:  workingDirectory,
		Prompt:            "prompt",
		LastMessagePath:   logPath,
		SchemaPath:        schemaPath,
		ArtifactDirectory: workingDirectory,
		ActorRunID:        "01_planner",
	})
	if err == nil {
		t.Fatalf("expected runCommand to reject unmarked system skill materialization")
	}
	if !strings.Contains(err.Error(), "unexpected_skill_materialization") {
		t.Fatalf("expected unexpected_skill_materialization violation, got %v", err)
	}
}

func TestRunCommandRejectsSealedCodexHomePluginCreation(t *testing.T) {
	workingDirectory := t.TempDir()
	fakeBin := t.TempDir()
	logPath := filepath.Join(workingDirectory, "response.json")
	schemaPath := filepath.Join(workingDirectory, "schema.json")
	fakeCodexPath := filepath.Join(fakeBin, "codex")

	script := `#!/usr/bin/env python3
import json
import os
import sys

output_path = None
for index, value in enumerate(sys.argv):
    if value == "--output-last-message" and index + 1 < len(sys.argv):
        output_path = sys.argv[index + 1]
        break

plugin_dir = os.path.join(os.environ["CODEX_HOME"], ".tmp", "plugins", "plugins", "github")
os.makedirs(plugin_dir, exist_ok=True)
with open(os.path.join(plugin_dir, "plugin.json"), "w", encoding="utf-8") as handle:
    handle.write("{}")

with open(output_path, "w", encoding="utf-8") as handle:
    json.dump({"summary": "should not be trusted"}, handle)
`
	if err := os.WriteFile(fakeCodexPath, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake codex: %v", err)
	}

	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	allowFakeCodexForTest(t, fakeCodexPath)
	t.Setenv("RAIL_CODEX_AUTH_HOME", testRailCodexAuthHome(t))

	_, err := runCommand(defaultTestActorBackend(), ActorCommandSpec{
		ActorName:         "planner",
		Profile:           ActorProfile{Model: "gpt-5.4-mini", Reasoning: "high"},
		WorkingDirectory:  workingDirectory,
		Prompt:            "prompt",
		LastMessagePath:   logPath,
		SchemaPath:        schemaPath,
		ArtifactDirectory: workingDirectory,
		ActorRunID:        "01_planner",
	})
	if err == nil {
		t.Fatalf("expected runCommand to reject plugin materialization inside sealed CODEX_HOME")
	}
	if !strings.Contains(err.Error(), "unexpected_plugin_materialization") {
		t.Fatalf("expected unexpected_plugin_materialization violation, got %v", err)
	}
}

func TestRunCommandPreventsPluginMaterializationWhenBackendDisablesPlugins(t *testing.T) {
	workingDirectory := t.TempDir()
	fakeBin := t.TempDir()
	logPath := filepath.Join(workingDirectory, "response.json")
	schemaPath := filepath.Join(workingDirectory, "schema.json")
	fakeCodexPath := filepath.Join(fakeBin, "codex")

	script := `#!/usr/bin/env python3
import json
import os
import sys

output_path = None
for index, value in enumerate(sys.argv):
    if value == "--output-last-message" and index + 1 < len(sys.argv):
        output_path = sys.argv[index + 1]
        break

plugins_disabled = any(
    value == "--disable" and index + 1 < len(sys.argv) and sys.argv[index + 1] == "plugins"
    for index, value in enumerate(sys.argv)
)
if not plugins_disabled:
    plugin_dir = os.path.join(os.environ["CODEX_HOME"], ".tmp", "plugins", "plugins", "github")
    os.makedirs(plugin_dir, exist_ok=True)
    with open(os.path.join(plugin_dir, "plugin.json"), "w", encoding="utf-8") as handle:
        handle.write("{}")

with open(output_path, "w", encoding="utf-8") as handle:
    json.dump({"summary": "ok"}, handle)
`
	if err := os.WriteFile(fakeCodexPath, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake codex: %v", err)
	}

	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	allowFakeCodexForTest(t, fakeCodexPath)
	t.Setenv("RAIL_CODEX_AUTH_HOME", testRailCodexAuthHome(t))

	response, err := runCommand(defaultTestActorBackend(), ActorCommandSpec{
		ActorName:         "planner",
		Profile:           ActorProfile{Model: "gpt-5.4-mini", Reasoning: "high"},
		WorkingDirectory:  workingDirectory,
		Prompt:            "prompt",
		LastMessagePath:   logPath,
		SchemaPath:        schemaPath,
		ArtifactDirectory: workingDirectory,
		ActorRunID:        "01_planner",
	})
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}
	if got, want := response["summary"], "ok"; got != want {
		t.Fatalf("unexpected actor response summary: got %#v want %#v", got, want)
	}
}

func TestRunCommandRejectsForbiddenSealedRuntimeSurfaces(t *testing.T) {
	cases := []struct {
		name      string
		code      string
		wantError string
	}{
		{
			name: "user_home_skill",
			code: `target_dir = os.path.join(os.environ["HOME"], ".codex", "skills", "injected")
os.makedirs(target_dir, exist_ok=True)
with open(os.path.join(target_dir, "SKILL.md"), "w", encoding="utf-8") as handle:
    handle.write("user skill")`,
			wantError: "unexpected_skill_materialization",
		},
		{
			name: "user_home_plugin",
			code: `target_dir = os.path.join(os.environ["HOME"], ".codex", "plugins", "github")
os.makedirs(target_dir, exist_ok=True)
with open(os.path.join(target_dir, "plugin.json"), "w", encoding="utf-8") as handle:
    handle.write("{}")`,
			wantError: "unexpected_plugin_materialization",
		},
		{
			name: "actor_local_hook",
			code: `target_dir = os.path.join(os.environ["CODEX_HOME"], "hooks")
os.makedirs(target_dir, exist_ok=True)
with open(os.path.join(target_dir, "hook.json"), "w", encoding="utf-8") as handle:
    handle.write("{}")`,
			wantError: "unexpected_hook_materialization",
		},
		{
			name: "actor_local_mcp",
			code: `target_dir = os.path.join(os.environ["CODEX_HOME"], "mcp")
os.makedirs(target_dir, exist_ok=True)
with open(os.path.join(target_dir, "config.json"), "w", encoding="utf-8") as handle:
    handle.write("{}")`,
			wantError: "unexpected_mcp_materialization",
		},
		{
			name: "actor_local_superpowers",
			code: `target_dir = os.path.join(os.environ["CODEX_HOME"], "superpowers", "skills")
os.makedirs(target_dir, exist_ok=True)
with open(os.path.join(target_dir, "using-superpowers.md"), "w", encoding="utf-8") as handle:
    handle.write("skill")`,
			wantError: "unexpected_skill_materialization",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			workingDirectory := t.TempDir()
			fakeBin := t.TempDir()
			logPath := filepath.Join(workingDirectory, "response.json")
			schemaPath := filepath.Join(workingDirectory, "schema.json")
			fakeCodexPath := filepath.Join(fakeBin, "codex")

			script := `#!/usr/bin/env python3
import json
import os
import sys

output_path = None
for index, value in enumerate(sys.argv):
    if value == "--output-last-message" and index + 1 < len(sys.argv):
        output_path = sys.argv[index + 1]
        break

` + tc.code + `

with open(output_path, "w", encoding="utf-8") as handle:
    json.dump({"summary": "should not be trusted"}, handle)
`
			if err := os.WriteFile(fakeCodexPath, []byte(script), 0o755); err != nil {
				t.Fatalf("failed to write fake codex: %v", err)
			}

			t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
			allowFakeCodexForTest(t, fakeCodexPath)
			t.Setenv("RAIL_CODEX_AUTH_HOME", testRailCodexAuthHome(t))

			_, err := runCommand(defaultTestActorBackend(), ActorCommandSpec{
				ActorName:         "planner",
				Profile:           ActorProfile{Model: "gpt-5.4-mini", Reasoning: "high"},
				WorkingDirectory:  workingDirectory,
				Prompt:            "prompt",
				LastMessagePath:   logPath,
				SchemaPath:        schemaPath,
				ArtifactDirectory: workingDirectory,
				ActorRunID:        "01_planner",
			})
			if err == nil {
				t.Fatalf("expected runCommand to reject forbidden sealed runtime surface")
			}
			if !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("expected %s violation, got %v", tc.wantError, err)
			}
		})
	}
}

func TestRunCommandRedactsAuthSecretFromActorFailure(t *testing.T) {
	workingDirectory := t.TempDir()
	fakeBin := t.TempDir()
	logPath := filepath.Join(workingDirectory, "response.json")
	schemaPath := filepath.Join(workingDirectory, "schema.json")
	fakeCodexPath := filepath.Join(fakeBin, "codex")

	script := `#!/usr/bin/env bash
set -euo pipefail
printf 'leaked proxy: %s\n' "${HTTPS_PROXY}" >&2
printf 'leaked base: %s\n' "${OPENAI_BASE_URL}" >&2
exit 42
`
	if err := os.WriteFile(fakeCodexPath, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake codex: %v", err)
	}

	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	allowFakeCodexForTest(t, fakeCodexPath)
	t.Setenv("RAIL_CODEX_AUTH_HOME", testRailCodexAuthHome(t))
	t.Setenv("HTTPS_PROXY", "https://user:pass@proxy.example")
	t.Setenv("OPENAI_BASE_URL", "https://token@example.test/v1")

	_, err := runCommand(defaultTestActorBackend(), ActorCommandSpec{
		ActorName:         "planner",
		Profile:           ActorProfile{Model: "gpt-5.4-mini", Reasoning: "high"},
		WorkingDirectory:  workingDirectory,
		Prompt:            "prompt",
		LastMessagePath:   logPath,
		SchemaPath:        schemaPath,
		ArtifactDirectory: workingDirectory,
		ActorRunID:        "01_planner",
	})
	if err == nil {
		t.Fatalf("expected runCommand to fail")
	}
	for _, leaked := range []string{"https://user:pass@proxy.example", "https://token@example.test/v1"} {
		if strings.Contains(err.Error(), leaked) {
			t.Fatalf("expected actor failure output to redact secret-bearing value %q, got %v", leaked, err)
		}
	}
	if !strings.Contains(err.Error(), "[REDACTED]") {
		t.Fatalf("expected redacted marker in actor failure, got %v", err)
	}
}

func TestRunCommandRequiresEventsPathWhenCapturingJSONEvents(t *testing.T) {
	backend := defaultTestActorBackend()
	backend.CaptureJSONEvents = true

	_, err := runCommand(backend, ActorCommandSpec{
		ActorName:        "planner",
		Profile:          ActorProfile{Model: "gpt-5.4-mini", Reasoning: "high"},
		WorkingDirectory: t.TempDir(),
		Prompt:           "prompt",
		LastMessagePath:  "response.json",
		SchemaPath:       "schema.json",
	})
	if err == nil {
		t.Fatalf("expected runCommand to fail when JSON event capture has no events path")
	}
	if !strings.Contains(err.Error(), "events path") {
		t.Fatalf("expected events path error, got %v", err)
	}
}

func defaultTestActorBackend() ActorBackendConfig {
	return ActorBackendConfig{
		Command:           "codex",
		Subcommand:        "exec",
		Sandbox:           "workspace-write",
		ApprovalPolicy:    "never",
		SessionMode:       "per_actor",
		Ephemeral:         true,
		CaptureJSONEvents: false,
		SkipGitRepoCheck:  true,
		IgnoreUserConfig:  true,
		IgnoreRules:       true,
		Capabilities: ActorBackendCapabilities{
			UserSkills:  "disabled",
			UserRules:   "disabled",
			Plugins:     "disabled",
			MCP:         "disabled",
			Hooks:       "disabled",
			Shell:       "allowed",
			FileEditing: "allowed",
		},
	}
}

func TestActorOutputJSONSchemaPlanRequiresAssumptions(t *testing.T) {
	schema, err := actorOutputJSONSchema("plan")
	if err != nil {
		t.Fatalf("actorOutputJSONSchema returned error: %v", err)
	}

	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected properties map, got %T", schema["properties"])
	}
	if _, ok := properties["assumptions"]; !ok {
		t.Fatalf("expected plan schema to expose assumptions property")
	}

	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatalf("expected required array, got %T", schema["required"])
	}
	requiredSet := map[string]struct{}{}
	for _, name := range required {
		requiredSet[name] = struct{}{}
	}
	for name := range properties {
		if _, ok := requiredSet[name]; !ok {
			t.Fatalf("expected required list to include property %q", name)
		}
	}
}

func TestActorOutputJSONSchemaCriticReport(t *testing.T) {
	schema, err := actorOutputJSONSchema("critic_report")
	if err != nil {
		t.Fatalf("actorOutputJSONSchema returned error: %v", err)
	}

	if got, ok := schema["additionalProperties"].(bool); !ok || got {
		t.Fatalf("expected critic_report schema to be closed, got %v", schema["additionalProperties"])
	}

	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected properties map, got %T", schema["properties"])
	}

	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatalf("expected required array, got %T", schema["required"])
	}
	requiredSet := map[string]struct{}{}
	for _, name := range required {
		requiredSet[name] = struct{}{}
	}

	expectedFields := []string{
		"priority_focus",
		"missing_requirements",
		"risk_hypotheses",
		"validation_expectations",
		"generator_guardrails",
		"blocked_assumptions",
	}
	for _, field := range expectedFields {
		if _, ok := properties[field]; !ok {
			t.Fatalf("expected critic_report schema to expose %q", field)
		}
		if _, ok := requiredSet[field]; !ok {
			t.Fatalf("expected critic_report schema to require %q", field)
		}
	}
	if len(required) != len(expectedFields) {
		t.Fatalf("unexpected required field count: got %d want %d", len(required), len(expectedFields))
	}

	maxItemsByField := map[string]int{
		"priority_focus":          6,
		"missing_requirements":    8,
		"risk_hypotheses":         8,
		"validation_expectations": 8,
		"generator_guardrails":    8,
		"blocked_assumptions":     8,
	}
	for field, wantMaxItems := range maxItemsByField {
		property, ok := properties[field].(map[string]any)
		if !ok {
			t.Fatalf("expected %q property schema to be a map, got %T", field, properties[field])
		}
		if got, ok := property["type"].(string); !ok || got != "array" {
			t.Fatalf("expected %q to be an array schema, got %v", field, property["type"])
		}
		if got, ok := property["maxItems"].(int); !ok || got != wantMaxItems {
			t.Fatalf("unexpected maxItems for %q: got %v want %v", field, property["maxItems"], wantMaxItems)
		}
		items, ok := property["items"].(map[string]any)
		if !ok {
			t.Fatalf("expected %q items schema to be a map, got %T", field, property["items"])
		}
		if got, ok := items["type"].(string); !ok || got != "string" {
			t.Fatalf("expected %q items to be strings, got %v", field, items["type"])
		}
		if got, ok := items["maxLength"].(int); !ok || got != 240 {
			t.Fatalf("unexpected item maxLength for %q: got %v want %d", field, items["maxLength"], 240)
		}
	}
}

func TestActorOutputJSONSchemaCriticReportMatchesTemplate(t *testing.T) {
	schema, err := actorOutputJSONSchema("critic_report")
	if err != nil {
		t.Fatalf("actorOutputJSONSchema returned error: %v", err)
	}

	repoRoot := testRepoRoot(t)
	repoTemplate := loadCriticReportSchemaTemplate(t, repoRoot)
	embeddedTemplate := loadCriticReportSchemaTemplate(t, t.TempDir())
	assertNormalizedSchemaEqual(t, normalizeSchemaForParity(schema), normalizeSchemaForParity(repoTemplate), "runtime schema", "repo template")
	assertNormalizedSchemaEqual(t, normalizeSchemaForParity(repoTemplate), normalizeSchemaForParity(embeddedTemplate), "repo template", "embedded default template")
	assertNormalizedSchemaEqual(t, normalizeSchemaForParity(schema), normalizeSchemaForParity(embeddedTemplate), "runtime schema", "embedded default template")
}

func TestActorOutputJSONSchemaEvaluationResultMatchesTemplate(t *testing.T) {
	schema, err := actorOutputJSONSchema("evaluation_result")
	if err != nil {
		t.Fatalf("actorOutputJSONSchema returned error: %v", err)
	}

	repoRoot := testRepoRoot(t)
	repoTemplate := loadSchemaTemplate(t, repoRoot, ".harness/templates/evaluation_result.schema.yaml")
	embeddedTemplate := loadSchemaTemplate(t, t.TempDir(), ".harness/templates/evaluation_result.schema.yaml")
	assertNormalizedSchemaEqual(t, normalizeSchemaForParity(schema), normalizeSchemaForParity(repoTemplate), "runtime schema", "repo template")
	assertNormalizedSchemaEqual(t, normalizeSchemaForParity(repoTemplate), normalizeSchemaForParity(embeddedTemplate), "repo template", "embedded default template")
	assertNormalizedSchemaEqual(t, normalizeSchemaForParity(schema), normalizeSchemaForParity(embeddedTemplate), "runtime schema", "embedded default template")
}

func TestNormalizeActorResponseDropsNullNextActionForTerminalEvaluation(t *testing.T) {
	normalized := normalizeActorResponse("evaluation_result", map[string]any{
		"decision":           "pass",
		"next_action":        nil,
		"quality_confidence": "high",
	})

	if _, exists := normalized["next_action"]; exists {
		t.Fatalf("expected next_action to be removed for terminal evaluation decisions")
	}
}

func loadCriticReportSchemaTemplate(t *testing.T, projectRoot string) map[string]any {
	t.Helper()

	return loadSchemaTemplate(t, projectRoot, ".harness/templates/critic_report.schema.yaml")
}

func loadSchemaTemplate(t *testing.T, projectRoot string, relativePath string) map[string]any {
	t.Helper()

	data, _, err := assets.Resolve(projectRoot, relativePath)
	if err != nil {
		t.Fatalf("failed to load schema template %s: %v", relativePath, err)
	}

	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to decode schema template %s: %v", relativePath, err)
	}

	template, ok := normalizeSchemaTestValue(raw).(map[string]any)
	if !ok {
		t.Fatalf("expected template schema to decode to a map, got %T", raw)
	}
	return template
}

func assertNormalizedSchemaEqual(t *testing.T, actual map[string]any, expected map[string]any, actualLabel string, expectedLabel string) {
	t.Helper()

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("schema mismatch for %s vs %s:\nactual=%#v\nexpected=%#v", actualLabel, expectedLabel, actual, expected)
	}
}

func normalizeSchemaTestValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		normalized := make(map[string]any, len(typed))
		for key, nested := range typed {
			normalized[key] = normalizeSchemaTestValue(nested)
		}
		return normalized
	case map[any]any:
		normalized := make(map[string]any, len(typed))
		for key, nested := range typed {
			normalized[key.(string)] = normalizeSchemaTestValue(nested)
		}
		return normalized
	case []any:
		normalized := make([]any, len(typed))
		for index, nested := range typed {
			normalized[index] = normalizeSchemaTestValue(nested)
		}
		return normalized
	default:
		return value
	}
}

func normalizeSchemaForParity(value any) map[string]any {
	normalized, ok := normalizeSchemaParityValue(value).(map[string]any)
	if !ok {
		return nil
	}
	return normalized
}

func normalizeSchemaParityValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		normalized := make(map[string]any, len(typed))
		for key, nested := range typed {
			normalized[key] = normalizeSchemaParityValue(nested)
		}
		return normalized
	case []string:
		normalized := append([]string{}, typed...)
		sort.Strings(normalized)
		items := make([]any, len(normalized))
		for i, item := range normalized {
			items[i] = item
		}
		return items
	case []any:
		normalized := make([]any, len(typed))
		for i, nested := range typed {
			normalized[i] = normalizeSchemaParityValue(nested)
		}
		if allStrings(normalized) {
			sort.Slice(normalized, func(i, j int) bool {
				return normalized[i].(string) < normalized[j].(string)
			})
		}
		return normalized
	case int:
		return float64(typed)
	default:
		return value
	}
}

func allStrings(values []any) bool {
	for _, value := range values {
		if _, ok := value.(string); !ok {
			return false
		}
	}
	return true
}
