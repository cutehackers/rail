package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFakeCodex(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "codex")
	script := `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >>"${RAIL_FAKE_CODEX_LOG}"
printf '%s\n' "${CODEX_HOME:-}" >>"${RAIL_FAKE_CODEX_HOME_LOG}"
printf '%s\n' "${RAIL_CODEX_AUTH_HOME:-}" >>"${RAIL_FAKE_RAIL_CODEX_AUTH_HOME_LOG}"
if [[ "$1" == "login" && "${2:-}" == "status" ]]; then
  if [[ -f "${CODEX_HOME}/auth.json" ]]; then
    printf 'Logged in\n'
    exit 0
  fi
  printf 'Not logged in\n' >&2
  exit 1
fi
if [[ "$1" == "login" ]]; then
  mkdir -p "${CODEX_HOME}"
  printf '{"fake":"auth"}\n' >"${CODEX_HOME}/auth.json"
  chmod 600 "${CODEX_HOME}/auth.json"
  printf 'Successfully logged in\n'
  exit 0
fi
if [[ "$1" == "logout" ]]; then
  rm -f "${CODEX_HOME}/auth.json"
  printf 'Logged out\n'
  exit 0
fi
printf 'unexpected args: %s\n' "$*" >&2
exit 64
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake codex: %v", err)
	}
	return path
}

func stubActorRuntimeReadiness(t *testing.T, fn func(string) error) {
	t.Helper()
	previous := actorRuntimeReadinessCheck
	actorRuntimeReadinessCheck = fn
	t.Cleanup(func() {
		actorRuntimeReadinessCheck = previous
	})
}

func TestRunAuthLoginStatusDoctorAndLogoutUseRailCodexHome(t *testing.T) {
	var readinessProjectRoot string
	stubActorRuntimeReadiness(t, func(projectRoot string) error {
		readinessProjectRoot = projectRoot
		return nil
	})
	fakeBin := t.TempDir()
	writeFakeCodex(t, fakeBin)
	authHome := filepath.Join(t.TempDir(), "rail-codex-auth")
	argsLog := filepath.Join(t.TempDir(), "args.log")
	homeLog := filepath.Join(t.TempDir(), "home.log")
	railAuthHomeLog := filepath.Join(t.TempDir(), "rail-auth-home.log")
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("RAIL_CODEX_AUTH_HOME", authHome)
	t.Setenv("RAIL_FAKE_CODEX_LOG", argsLog)
	t.Setenv("RAIL_FAKE_CODEX_HOME_LOG", homeLog)
	t.Setenv("RAIL_FAKE_RAIL_CODEX_AUTH_HOME_LOG", railAuthHomeLog)

	var loginOut bytes.Buffer
	if err := RunAuth([]string{"login"}, strings.NewReader(""), &loginOut); err != nil {
		t.Fatalf("RunAuth login returned error: %v", err)
	}
	if strings.Contains(loginOut.String(), authHome) {
		t.Fatalf("login output exposed concrete auth home: %q", loginOut.String())
	}

	var doctorOut bytes.Buffer
	if err := RunAuth([]string{"doctor"}, strings.NewReader(""), &doctorOut); err != nil {
		t.Fatalf("RunAuth doctor returned error: %v", err)
	}
	if !strings.Contains(doctorOut.String(), "ready") {
		t.Fatalf("expected ready doctor output, got %q", doctorOut.String())
	}
	if readinessProjectRoot == "" {
		t.Fatalf("expected doctor to check actor runtime readiness")
	}

	var statusOut bytes.Buffer
	if err := RunAuth([]string{"status"}, strings.NewReader(""), &statusOut); err != nil {
		t.Fatalf("RunAuth status returned error: %v", err)
	}
	if !strings.Contains(statusOut.String(), "configured") {
		t.Fatalf("expected configured status, got %q", statusOut.String())
	}

	var logoutOut bytes.Buffer
	if err := RunAuth([]string{"logout"}, strings.NewReader(""), &logoutOut); err != nil {
		t.Fatalf("RunAuth logout returned error: %v", err)
	}
	if _, err := os.Stat(authHome); !os.IsNotExist(err) {
		t.Fatalf("expected auth home to be removed after logout, stat error: %v", err)
	}

	homeData, err := os.ReadFile(homeLog)
	if err != nil {
		t.Fatalf("read fake codex home log: %v", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(homeData)), "\n") {
		if line != authHome {
			t.Fatalf("expected fake codex to use Rail auth home %q, got log %q", authHome, string(homeData))
		}
	}
	railAuthHomeData, err := os.ReadFile(railAuthHomeLog)
	if err != nil {
		t.Fatalf("read fake codex Rail auth home log: %v", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(railAuthHomeData)), "\n") {
		if line != "" {
			t.Fatalf("expected fake codex not to inherit RAIL_CODEX_AUTH_HOME, got log %q", string(railAuthHomeData))
		}
	}
}

func TestRunAuthDoctorChecksProjectRootRuntimeReadiness(t *testing.T) {
	fakeBin := t.TempDir()
	writeFakeCodex(t, fakeBin)
	authHome := filepath.Join(t.TempDir(), "rail-codex-auth")
	projectRoot := t.TempDir()
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("RAIL_CODEX_AUTH_HOME", authHome)
	t.Setenv("RAIL_FAKE_CODEX_LOG", filepath.Join(t.TempDir(), "args.log"))
	t.Setenv("RAIL_FAKE_CODEX_HOME_LOG", filepath.Join(t.TempDir(), "home.log"))
	t.Setenv("RAIL_FAKE_RAIL_CODEX_AUTH_HOME_LOG", filepath.Join(t.TempDir(), "rail-auth-home.log"))
	stubActorRuntimeReadiness(t, func(actual string) error {
		if filepath.Clean(actual) != filepath.Clean(projectRoot) {
			t.Fatalf("expected project root %q, got %q", projectRoot, actual)
		}
		return nil
	})

	var loginOut bytes.Buffer
	if err := RunAuth([]string{"login"}, strings.NewReader(""), &loginOut); err != nil {
		t.Fatalf("RunAuth login returned error: %v", err)
	}

	var doctorOut bytes.Buffer
	if err := RunAuth([]string{"doctor", "--project-root", projectRoot}, strings.NewReader(""), &doctorOut); err != nil {
		t.Fatalf("RunAuth doctor returned error: %v", err)
	}
	for _, want := range []string{
		"Rail actor auth ready",
		"Rail actor runtime ready",
		"Secret values are not printed.",
	} {
		if !strings.Contains(doctorOut.String(), want) {
			t.Fatalf("expected doctor output to contain %q, got %q", want, doctorOut.String())
		}
	}
}

func TestRunAuthDoctorSanitizesRuntimeReadinessFailure(t *testing.T) {
	fakeBin := t.TempDir()
	writeFakeCodex(t, fakeBin)
	authHome := filepath.Join(t.TempDir(), "rail-codex-auth")
	projectRoot := t.TempDir()
	rawPath := filepath.Join(t.TempDir(), "raw-path")
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("RAIL_CODEX_AUTH_HOME", authHome)
	t.Setenv("RAIL_FAKE_CODEX_LOG", filepath.Join(t.TempDir(), "args.log"))
	t.Setenv("RAIL_FAKE_CODEX_HOME_LOG", filepath.Join(t.TempDir(), "home.log"))
	t.Setenv("RAIL_FAKE_RAIL_CODEX_AUTH_HOME_LOG", filepath.Join(t.TempDir(), "rail-auth-home.log"))
	stubActorRuntimeReadiness(t, func(string) error {
		return fmt.Errorf("backend_policy_violation: unsafe_codex_path: %s %s %s", authHome, projectRoot, rawPath)
	})

	var loginOut bytes.Buffer
	if err := RunAuth([]string{"login"}, strings.NewReader(""), &loginOut); err != nil {
		t.Fatalf("RunAuth login returned error: %v", err)
	}

	var doctorOut bytes.Buffer
	err := RunAuth([]string{"doctor", "--project-root", projectRoot}, strings.NewReader(""), &doctorOut)
	if err == nil {
		t.Fatalf("expected doctor to fail when actor runtime readiness fails")
	}
	if got, want := err.Error(), actorRuntimeNotReadyError; got != want {
		t.Fatalf("unexpected sanitized doctor error: got %q want %q", got, want)
	}
	for _, leaked := range []string{authHome, projectRoot, rawPath} {
		if strings.Contains(err.Error(), leaked) || strings.Contains(doctorOut.String(), leaked) {
			t.Fatalf("doctor exposed sensitive path %q: err=%v stdout=%q", leaked, err, doctorOut.String())
		}
	}
	if !strings.Contains(doctorOut.String(), "Rail actor runtime not ready") {
		t.Fatalf("expected runtime readiness failure output, got %q", doctorOut.String())
	}
}

func TestRunAuthDoctorRejectsTargetLocalUnsafeBackendPolicy(t *testing.T) {
	fakeBin := t.TempDir()
	writeFakeCodex(t, fakeBin)
	authHome := filepath.Join(t.TempDir(), "rail-codex-auth")
	projectRoot := t.TempDir()
	supervisorDir := filepath.Join(projectRoot, ".harness", "supervisor")
	if err := os.MkdirAll(supervisorDir, 0o755); err != nil {
		t.Fatalf("create supervisor dir: %v", err)
	}
	policy := []byte(`version: 1
execution_environment: local
default_backend: codex_cli
backends:
  codex_cli:
    command: /absolute/path/to/codex
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
	if err := os.WriteFile(filepath.Join(supervisorDir, "actor_backend.yaml"), policy, 0o644); err != nil {
		t.Fatalf("write actor backend policy: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("RAIL_CODEX_AUTH_HOME", authHome)
	t.Setenv("RAIL_FAKE_CODEX_LOG", filepath.Join(t.TempDir(), "args.log"))
	t.Setenv("RAIL_FAKE_CODEX_HOME_LOG", filepath.Join(t.TempDir(), "home.log"))
	t.Setenv("RAIL_FAKE_RAIL_CODEX_AUTH_HOME_LOG", filepath.Join(t.TempDir(), "rail-auth-home.log"))

	var loginOut bytes.Buffer
	if err := RunAuth([]string{"login"}, strings.NewReader(""), &loginOut); err != nil {
		t.Fatalf("RunAuth login returned error: %v", err)
	}

	var doctorOut bytes.Buffer
	err := RunAuth([]string{"doctor", "--project-root", projectRoot}, strings.NewReader(""), &doctorOut)
	if err == nil {
		t.Fatalf("expected doctor to reject target-local unsafe actor backend policy")
	}
	if got, want := err.Error(), actorRuntimeNotReadyError; got != want {
		t.Fatalf("unexpected sanitized doctor error: got %q want %q", got, want)
	}
	if strings.Contains(doctorOut.String(), projectRoot) || strings.Contains(err.Error(), projectRoot) {
		t.Fatalf("doctor exposed project root in unsafe backend failure: err=%v stdout=%q", err, doctorOut.String())
	}
	if !strings.Contains(doctorOut.String(), "Rail actor runtime not ready") {
		t.Fatalf("expected runtime readiness failure output, got %q", doctorOut.String())
	}
}

func TestRunAuthRejectsProjectRootOutsideDoctor(t *testing.T) {
	var stdout bytes.Buffer
	for _, subcommand := range []string{"login", "status", "logout"} {
		err := RunAuth([]string{subcommand, "--project-root", t.TempDir()}, strings.NewReader(""), &stdout)
		if err == nil {
			t.Fatalf("expected %s to reject --project-root", subcommand)
		}
		if !strings.Contains(err.Error(), "only supported for auth doctor") {
			t.Fatalf("expected project-root rejection for %s, got %v", subcommand, err)
		}
	}
}

func TestRunAuthDoctorFailsClosedWhenAuthMissing(t *testing.T) {
	fakeBin := t.TempDir()
	writeFakeCodex(t, fakeBin)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	argsLog := filepath.Join(t.TempDir(), "args.log")
	t.Setenv("RAIL_CODEX_AUTH_HOME", filepath.Join(t.TempDir(), "missing-auth-home"))
	t.Setenv("RAIL_FAKE_CODEX_LOG", argsLog)
	t.Setenv("RAIL_FAKE_CODEX_HOME_LOG", filepath.Join(t.TempDir(), "home.log"))
	t.Setenv("RAIL_FAKE_RAIL_CODEX_AUTH_HOME_LOG", filepath.Join(t.TempDir(), "rail-auth-home.log"))

	var stdout bytes.Buffer
	err := RunAuth([]string{"doctor"}, strings.NewReader(""), &stdout)
	if err == nil {
		t.Fatalf("expected RunAuth doctor to fail when auth is missing")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("expected not configured error, got %v", err)
	}
	if !strings.Contains(stdout.String(), "rail auth login") {
		t.Fatalf("expected doctor output to explain login next step, got %q", stdout.String())
	}
	if data, err := os.ReadFile(argsLog); err == nil && len(data) != 0 {
		t.Fatalf("expected missing auth home not to invoke fake codex, got log %q", data)
	} else if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read fake codex args log: %v", err)
	}
}

func TestRunAuthLoginRejectsUnsafeAuthHomeWithoutExposingPath(t *testing.T) {
	fakeBin := t.TempDir()
	writeFakeCodex(t, fakeBin)
	authHome := filepath.Join(t.TempDir(), "rail-codex-auth")
	argsLog := filepath.Join(t.TempDir(), "args.log")
	if err := os.Mkdir(authHome, 0o700); err != nil {
		t.Fatalf("mkdir auth home: %v", err)
	}
	if err := os.WriteFile(filepath.Join(authHome, "unrelated.txt"), []byte("keep"), 0o600); err != nil {
		t.Fatalf("write unrelated file: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("RAIL_CODEX_AUTH_HOME", authHome)
	t.Setenv("RAIL_FAKE_CODEX_LOG", argsLog)
	t.Setenv("RAIL_FAKE_CODEX_HOME_LOG", filepath.Join(t.TempDir(), "home.log"))
	t.Setenv("RAIL_FAKE_RAIL_CODEX_AUTH_HOME_LOG", filepath.Join(t.TempDir(), "rail-auth-home.log"))

	var stdout bytes.Buffer
	err := RunAuth([]string{"login"}, strings.NewReader(""), &stdout)
	if err == nil {
		t.Fatalf("expected RunAuth login to reject unsafe auth home")
	}
	if got, want := err.Error(), "rail actor auth cannot be configured because the auth home is unsafe"; got != want {
		t.Fatalf("unexpected sanitized login error: got %q want %q", got, want)
	}
	if strings.Contains(err.Error(), authHome) || strings.Contains(stdout.String(), authHome) {
		t.Fatalf("login exposed concrete auth home: err=%v stdout=%q", err, stdout.String())
	}
	if data, err := os.ReadFile(argsLog); err == nil && len(data) != 0 {
		t.Fatalf("expected unsafe auth home not to invoke fake codex, got log %q", data)
	} else if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read fake codex args log: %v", err)
	}
}

func TestRunAuthLoginPreservesCodexExecutionFailure(t *testing.T) {
	authHome := filepath.Join(t.TempDir(), "rail-codex-auth")
	t.Setenv("RAIL_CODEX_AUTH_HOME", authHome)

	var stdout bytes.Buffer
	err := RunAuth([]string{"login", "--codex-command", filepath.Join(t.TempDir(), "missing-codex")}, strings.NewReader(""), &stdout)
	if err == nil {
		t.Fatalf("expected RunAuth login to return codex execution failure")
	}
	if err.Error() == "rail actor auth cannot be configured because the auth home is unsafe" {
		t.Fatalf("expected codex execution failure not to be reported as unsafe auth home")
	}
	if strings.Contains(err.Error(), authHome) || strings.Contains(stdout.String(), authHome) {
		t.Fatalf("login exposed concrete auth home: err=%v stdout=%q", err, stdout.String())
	}
}

func TestRunAuthStatusRejectsUnmarkedPrivateAuthHomeWithoutInvokingCodex(t *testing.T) {
	fakeBin := t.TempDir()
	writeFakeCodex(t, fakeBin)
	authHome := filepath.Join(t.TempDir(), "rail-codex-auth")
	argsLog := filepath.Join(t.TempDir(), "args.log")
	if err := os.Mkdir(authHome, 0o700); err != nil {
		t.Fatalf("mkdir auth home: %v", err)
	}
	if err := os.WriteFile(filepath.Join(authHome, "auth.json"), []byte(`{"fake":"auth"}`), 0o600); err != nil {
		t.Fatalf("write auth.json: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("RAIL_CODEX_AUTH_HOME", authHome)
	t.Setenv("RAIL_FAKE_CODEX_LOG", argsLog)
	t.Setenv("RAIL_FAKE_CODEX_HOME_LOG", filepath.Join(t.TempDir(), "home.log"))
	t.Setenv("RAIL_FAKE_RAIL_CODEX_AUTH_HOME_LOG", filepath.Join(t.TempDir(), "rail-auth-home.log"))

	var stdout bytes.Buffer
	err := RunAuth([]string{"status"}, strings.NewReader(""), &stdout)
	if err == nil {
		t.Fatalf("expected RunAuth status to reject unmarked auth home")
	}
	if got, want := err.Error(), "rail actor auth cannot be checked because it is not a Rail-owned auth home"; got != want {
		t.Fatalf("unexpected sanitized status error: got %q want %q", got, want)
	}
	if strings.Contains(err.Error(), authHome) {
		t.Fatalf("status error exposed concrete auth home: %v", err)
	}
	if data, err := os.ReadFile(argsLog); err == nil && len(data) != 0 {
		t.Fatalf("expected unmarked auth home not to invoke fake codex, got log %q", data)
	} else if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read fake codex args log: %v", err)
	}
}

func TestRunAuthDoctorRejectsUnmarkedPrivateAuthHomeWithoutInvokingCodex(t *testing.T) {
	fakeBin := t.TempDir()
	writeFakeCodex(t, fakeBin)
	authHome := filepath.Join(t.TempDir(), "rail-codex-auth")
	argsLog := filepath.Join(t.TempDir(), "args.log")
	if err := os.Mkdir(authHome, 0o700); err != nil {
		t.Fatalf("mkdir auth home: %v", err)
	}
	if err := os.WriteFile(filepath.Join(authHome, "auth.json"), []byte(`{"fake":"auth"}`), 0o600); err != nil {
		t.Fatalf("write auth.json: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("RAIL_CODEX_AUTH_HOME", authHome)
	t.Setenv("RAIL_FAKE_CODEX_LOG", argsLog)
	t.Setenv("RAIL_FAKE_CODEX_HOME_LOG", filepath.Join(t.TempDir(), "home.log"))
	t.Setenv("RAIL_FAKE_RAIL_CODEX_AUTH_HOME_LOG", filepath.Join(t.TempDir(), "rail-auth-home.log"))

	var stdout bytes.Buffer
	err := RunAuth([]string{"doctor"}, strings.NewReader(""), &stdout)
	if err == nil {
		t.Fatalf("expected RunAuth doctor to reject unmarked auth home")
	}
	if got, want := err.Error(), "rail actor auth not configured"; got != want {
		t.Fatalf("unexpected sanitized doctor error: got %q want %q", got, want)
	}
	if strings.Contains(err.Error(), authHome) || strings.Contains(stdout.String(), authHome) {
		t.Fatalf("doctor exposed concrete auth home: err=%v stdout=%q", err, stdout.String())
	}
	if !strings.Contains(stdout.String(), "rail auth login") {
		t.Fatalf("expected doctor output to explain login next step, got %q", stdout.String())
	}
	if data, err := os.ReadFile(argsLog); err == nil && len(data) != 0 {
		t.Fatalf("expected unmarked auth home not to invoke fake codex, got log %q", data)
	} else if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read fake codex args log: %v", err)
	}
}

func TestRunAuthLogoutSkipsCodexForUnmarkedAuthHome(t *testing.T) {
	fakeBin := t.TempDir()
	writeFakeCodex(t, fakeBin)
	authHome := filepath.Join(t.TempDir(), "rail-codex-auth")
	argsLog := filepath.Join(t.TempDir(), "args.log")
	if err := os.Mkdir(authHome, 0o700); err != nil {
		t.Fatalf("mkdir auth home: %v", err)
	}
	if err := os.WriteFile(filepath.Join(authHome, "auth.json"), []byte(`{"fake":"auth"}`), 0o600); err != nil {
		t.Fatalf("write auth.json: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("RAIL_CODEX_AUTH_HOME", authHome)
	t.Setenv("RAIL_FAKE_CODEX_LOG", argsLog)
	t.Setenv("RAIL_FAKE_CODEX_HOME_LOG", filepath.Join(t.TempDir(), "home.log"))
	t.Setenv("RAIL_FAKE_RAIL_CODEX_AUTH_HOME_LOG", filepath.Join(t.TempDir(), "rail-auth-home.log"))

	var stdout bytes.Buffer
	err := RunAuth([]string{"logout"}, strings.NewReader(""), &stdout)
	if err == nil {
		t.Fatalf("expected RunAuth logout to reject unmarked auth home")
	}
	if got, want := err.Error(), "rail actor auth cannot be removed because it is not a Rail-owned auth home"; got != want {
		t.Fatalf("unexpected sanitized logout error: got %q want %q", got, want)
	}
	if strings.Contains(err.Error(), authHome) {
		t.Fatalf("logout error exposed concrete auth home: %v", err)
	}
	if data, err := os.ReadFile(argsLog); err == nil && len(data) != 0 {
		t.Fatalf("expected unmarked auth home not to invoke fake codex, got log %q", data)
	} else if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read fake codex args log: %v", err)
	}
	if _, err := os.Stat(authHome); err != nil {
		t.Fatalf("expected unmarked auth home to remain: %v", err)
	}
}

func TestRunAuthLogoutSkipsCodexForMissingAuthHome(t *testing.T) {
	fakeBin := t.TempDir()
	writeFakeCodex(t, fakeBin)
	authHome := filepath.Join(t.TempDir(), "missing-auth-home")
	argsLog := filepath.Join(t.TempDir(), "args.log")
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("RAIL_CODEX_AUTH_HOME", authHome)
	t.Setenv("RAIL_FAKE_CODEX_LOG", argsLog)
	t.Setenv("RAIL_FAKE_CODEX_HOME_LOG", filepath.Join(t.TempDir(), "home.log"))
	t.Setenv("RAIL_FAKE_RAIL_CODEX_AUTH_HOME_LOG", filepath.Join(t.TempDir(), "rail-auth-home.log"))

	var stdout bytes.Buffer
	if err := RunAuth([]string{"logout"}, strings.NewReader(""), &stdout); err != nil {
		t.Fatalf("RunAuth logout returned error: %v", err)
	}
	if data, err := os.ReadFile(argsLog); err == nil && len(data) != 0 {
		t.Fatalf("expected missing auth home not to invoke fake codex, got log %q", data)
	} else if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read fake codex args log: %v", err)
	}
}

func TestRunAuthRejectsUnknownSecretLookingArgumentWithoutEchoingIt(t *testing.T) {
	var stdout bytes.Buffer
	err := RunAuth([]string{"login", "sk-secret-value"}, strings.NewReader(""), &stdout)
	if err == nil {
		t.Fatalf("expected RunAuth to fail")
	}
	if strings.Contains(err.Error(), "sk-secret-value") {
		t.Fatalf("auth error leaked secret-looking argument: %v", err)
	}
}
