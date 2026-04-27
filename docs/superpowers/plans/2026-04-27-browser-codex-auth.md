# Browser Codex Auth Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace Rail's pre-release API-key auth file flow with a user-scoped Rail Codex auth home that is populated by `codex login` and materialized safely into sealed actor runtimes.

**Architecture:** `internal/auth` owns Rail Codex auth-home pathing, safety checks, Codex login/status/logout wrappers, and allowlisted auth materialization. `internal/cli/auth.go` becomes a thin command surface over those auth primitives. `internal/runtime/actor_runtime_sealed.go` stops resolving `OPENAI_API_KEY`, materializes only `auth.json` into the actor-local `CODEX_HOME`, and records `auth_source: rail_codex_login` without secrets.

**Tech Stack:** Go CLI in `cmd/rail` and `internal/cli`, auth helpers in `internal/auth`, sealed runtime in `internal/runtime`, Markdown docs and Rail skill copies, tests with fake `codex` binaries and `go test`.

---

## Scope Decisions

- Remove the pre-release Rail YAML API-key file path. There is no compatibility task for `RAIL_ACTOR_AUTH_FILE`, `--auth-file`, `--api-key`, or `OPENAI_API_KEY` as actor auth.
- Use a user-scoped Rail auth home, resolved from `RAIL_CODEX_AUTH_HOME` when set and otherwise from the platform user config location under `rail/codex-auth-home`.
- Copy only `auth.json` from the Rail auth home into each sealed actor `CODEX_HOME`. Local characterization with Codex CLI `0.124.0` showed `codex login --with-api-key` stores auth in `<CODEX_HOME>/auth.json`; browser login is expected to use the same Codex auth home contract.
- Keep `RAIL_CODEX_AUTH_HOME` as an advanced override for tests and operators. Do not document it as the normal user path.
- `rail auth logout` should invoke `codex logout` with the Rail auth home, then remove the Rail-owned auth home directory to avoid stale auth material.
- `rail auth doctor` classifies expired/revoked login through `codex login status`. Runtime actor prep classifies missing, unsafe, or unmaterializable auth before launching the actor.

## File Structure

- Replace: `internal/auth/actor_auth.go` with Rail Codex auth-home helpers. Keeping the filename is acceptable to reduce churn, but the API-key file types and functions should be removed.
- Replace: `internal/auth/actor_auth_test.go` with tests for auth-home pathing, permission checks, symlink rejection, and `auth.json` materialization.
- Modify: `internal/cli/auth.go` to remove API-key parsing and shell out to `codex login`, `codex login status`, and `codex logout`.
- Modify: `internal/cli/auth_test.go` to use a fake `codex` binary instead of API-key fixtures.
- Modify: `internal/runtime/actor_runtime_sealed.go` to materialize Rail Codex auth into sealed actor `CODEX_HOME`, remove API-key env injection, update provenance, and keep redaction for other secret-bearing env values.
- Modify: `internal/runtime/actor_runtime_test.go`, `internal/runtime/integration_test.go`, `internal/runtime/runner_test.go`, and `internal/cli/app_test.go` where tests currently set `OPENAI_API_KEY`.
- Modify: `README.md`, `docs/ARCHITECTURE.md`, `docs/ARCHITECTURE-kr.md`, `skills/Rail/SKILL.md`, and `assets/skill/Rail/SKILL.md` to describe browser login rather than API-key auth.

## Task 1: Replace API-Key Auth File With Rail Codex Auth Home

**Files:**
- Modify: `internal/auth/actor_auth.go`
- Modify: `internal/auth/actor_auth_test.go`

- [ ] **Step 1: Replace auth package tests with auth-home tests**

Replace `internal/auth/actor_auth_test.go` with tests that describe the new contract:

```go
package auth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultCodexAuthHomePathUsesUserConfigRailDirectory(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	path, err := DefaultCodexAuthHomePath()
	if err != nil {
		t.Fatalf("DefaultCodexAuthHomePath returned error: %v", err)
	}
	want := filepath.Join(configHome, "rail", "codex-auth-home")
	if path != want {
		t.Fatalf("unexpected auth home: got %q want %q", path, want)
	}
}

func TestCodexAuthHomePathFromEnvPrefersOverride(t *testing.T) {
	override := filepath.Join(t.TempDir(), "rail-auth")

	path, err := CodexAuthHomePathFromEnv(map[string]string{
		"RAIL_CODEX_AUTH_HOME": override,
	})
	if err != nil {
		t.Fatalf("CodexAuthHomePathFromEnv returned error: %v", err)
	}
	if path != override {
		t.Fatalf("unexpected auth home: got %q want %q", path, override)
	}
}

func TestEnsureCodexAuthHomeCreatesPrivateDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rail-auth")

	if err := EnsureCodexAuthHome(path); err != nil {
		t.Fatalf("EnsureCodexAuthHome returned error: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat auth home: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected auth home to be a directory")
	}
	if got, want := info.Mode().Perm(), os.FileMode(0o700); got != want {
		t.Fatalf("unexpected auth home permission: got %v want %v", got, want)
	}
}

func TestEnsureCodexAuthHomeRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	link := filepath.Join(dir, "link")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	err := EnsureCodexAuthHome(link)
	if err == nil {
		t.Fatalf("expected symlink auth home to be rejected")
	}
	if !strings.Contains(err.Error(), "must not be a symlink") {
		t.Fatalf("expected symlink error, got %v", err)
	}
}

func TestMaterializeCodexAuthForActorCopiesOnlyAuthJSON(t *testing.T) {
	source := t.TempDir()
	destination := filepath.Join(t.TempDir(), "actor-codex-home")
	if err := os.WriteFile(filepath.Join(source, "auth.json"), []byte(`{"tokens":"secret"}`), 0o600); err != nil {
		t.Fatalf("write auth.json: %v", err)
	}
	if err := os.Mkdir(filepath.Join(source, "skills"), 0o700); err != nil {
		t.Fatalf("mkdir skills: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "config.toml"), []byte("model = \"wrong\"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	result, err := MaterializeCodexAuthForActor(source, destination)
	if err != nil {
		t.Fatalf("MaterializeCodexAuthForActor returned error: %v", err)
	}
	if result.Source != "rail_codex_login" {
		t.Fatalf("unexpected source: %q", result.Source)
	}
	if _, err := os.Stat(filepath.Join(destination, "auth.json")); err != nil {
		t.Fatalf("expected auth.json to be copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destination, "skills")); !os.IsNotExist(err) {
		t.Fatalf("expected skills directory not to be copied, stat error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destination, "config.toml")); !os.IsNotExist(err) {
		t.Fatalf("expected config.toml not to be copied, stat error: %v", err)
	}
}
```

Run:

```bash
go test ./internal/auth -count=1
```

Expected: FAIL because the new functions do not exist and the old API-key functions still exist.

- [ ] **Step 2: Implement auth-home pathing and safety helpers**

Replace the API-key content in `internal/auth/actor_auth.go` with this focused implementation:

```go
package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	RailCodexAuthHomeEnv = "RAIL_CODEX_AUTH_HOME"
	AuthSourceRailCodex  = "rail_codex_login"
	CodexAuthFileName    = "auth.json"
)

type MaterializedCodexAuth struct {
	Source      string
	SourceHome  string
	AuthFile    string
	CopiedFiles []string
}

func DefaultCodexAuthHomePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(configDir, "rail", "codex-auth-home"), nil
}

func CodexAuthHomePathFromEnv(env map[string]string) (string, error) {
	if value := strings.TrimSpace(env[RailCodexAuthHomeEnv]); value != "" {
		return filepath.Abs(value)
	}
	return DefaultCodexAuthHomePath()
}

func EnsureCodexAuthHome(path string) error {
	resolved, err := resolveCodexAuthHomePath(path)
	if err != nil {
		return err
	}
	if err := ensurePrivateDirectory(resolved); err != nil {
		return err
	}
	marker := filepath.Join(resolved, ".rail-auth-home")
	if _, err := os.Stat(marker); os.IsNotExist(err) {
		content := fmt.Sprintf("version: 1\ncreated_at: %s\n", time.Now().UTC().Format(time.RFC3339))
		if err := os.WriteFile(marker, []byte(content), 0o600); err != nil {
			return fmt.Errorf("write rail auth marker: %w", err)
		}
	}
	return nil
}

func ValidateCodexAuthHome(path string) error {
	resolved, err := resolveCodexAuthHomePath(path)
	if err != nil {
		return err
	}
	return validatePrivateDirectory(resolved)
}

func MaterializeCodexAuthForActor(sourceHome string, destinationHome string) (MaterializedCodexAuth, error) {
	sourceHome, err := resolveCodexAuthHomePath(sourceHome)
	if err != nil {
		return MaterializedCodexAuth{}, err
	}
	destinationHome, err = filepath.Abs(strings.TrimSpace(destinationHome))
	if err != nil {
		return MaterializedCodexAuth{}, fmt.Errorf("resolve actor codex home: %w", err)
	}
	if err := validatePrivateDirectory(sourceHome); err != nil {
		return MaterializedCodexAuth{}, fmt.Errorf("rail_actor_auth_home_unsafe: %w", err)
	}
	if err := ensurePrivateDirectory(destinationHome); err != nil {
		return MaterializedCodexAuth{}, err
	}
	sourceFile := filepath.Join(sourceHome, CodexAuthFileName)
	destinationFile := filepath.Join(destinationHome, CodexAuthFileName)
	if err := copyPrivateRegularFile(sourceFile, destinationFile); err != nil {
		if os.IsNotExist(err) {
			return MaterializedCodexAuth{}, fmt.Errorf("rail_actor_auth_not_configured: run `rail auth login` before standard actor execution")
		}
		return MaterializedCodexAuth{}, fmt.Errorf("rail_actor_auth_materialization_failed: %w", err)
	}
	return MaterializedCodexAuth{
		Source:      AuthSourceRailCodex,
		SourceHome:  sourceHome,
		AuthFile:    destinationFile,
		CopiedFiles: []string{CodexAuthFileName},
	}, nil
}

func RemoveCodexAuthHome(path string) error {
	resolved, err := resolveCodexAuthHomePath(path)
	if err != nil {
		return err
	}
	if info, err := os.Lstat(resolved); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("codex auth home must not be a symlink: %s", resolved)
	} else if err != nil && os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect codex auth home: %w", err)
	}
	if err := os.RemoveAll(resolved); err != nil {
		return fmt.Errorf("remove codex auth home: %w", err)
	}
	return nil
}

func resolveCodexAuthHomePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return DefaultCodexAuthHomePath()
	}
	resolved, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve codex auth home path: %w", err)
	}
	return resolved, nil
}

func ensurePrivateDirectory(path string) error {
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("codex auth home must not be a symlink: %s", path)
		}
		if !info.IsDir() {
			return fmt.Errorf("codex auth home must be a directory: %s", path)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect codex auth home: %w", err)
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("create codex auth home: %w", err)
	}
	if err := os.Chmod(path, 0o700); err != nil {
		return fmt.Errorf("chmod codex auth home: %w", err)
	}
	return validatePrivateDirectory(path)
}

func validatePrivateDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("codex auth home must not be a symlink: %s", path)
	}
	info, err = os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("codex auth home must be a directory: %s", path)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("codex auth home permissions must be 0700 or stricter: %s", path)
	}
	return nil
}

func copyPrivateRegularFile(source string, destination string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("auth material must not be a symlink: %s", source)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("auth material must be a regular file: %s", source)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("auth material permissions must be 0600 or stricter: %s", source)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("read auth material: %w", err)
	}
	if err := os.WriteFile(destination, data, 0o600); err != nil {
		return fmt.Errorf("write actor auth material: %w", err)
	}
	if err := os.Chmod(destination, 0o600); err != nil {
		return fmt.Errorf("chmod actor auth material: %w", err)
	}
	return nil
}
```

This file intentionally has no YAML marshaling and no `OPENAI_API_KEY` resolver.

- [ ] **Step 3: Run auth tests**

Run:

```bash
go test ./internal/auth -count=1
```

Expected: PASS.

- [ ] **Step 4: Commit auth-home primitives**

```bash
git add internal/auth/actor_auth.go internal/auth/actor_auth_test.go
git commit -m "feat: add rail codex auth home"
```

## Task 2: Wrap Codex Login, Status, And Logout In `rail auth`

**Files:**
- Modify: `internal/auth/actor_auth.go`
- Modify: `internal/cli/auth.go`
- Modify: `internal/cli/auth_test.go`

- [ ] **Step 1: Add fake Codex helpers in CLI tests**

Replace API-key CLI tests in `internal/cli/auth_test.go` with fake Codex tests. Include this helper:

```go
func writeFakeCodex(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "codex")
	script := `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >>"${RAIL_FAKE_CODEX_LOG}"
printf '%s\n' "${CODEX_HOME:-}" >>"${RAIL_FAKE_CODEX_HOME_LOG}"
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
```

Add this test:

```go
func TestRunAuthLoginStatusDoctorAndLogoutUseRailCodexHome(t *testing.T) {
	fakeBin := t.TempDir()
	writeFakeCodex(t, fakeBin)
	authHome := filepath.Join(t.TempDir(), "rail-codex-auth")
	argsLog := filepath.Join(t.TempDir(), "args.log")
	homeLog := filepath.Join(t.TempDir(), "home.log")
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("RAIL_CODEX_AUTH_HOME", authHome)
	t.Setenv("RAIL_FAKE_CODEX_LOG", argsLog)
	t.Setenv("RAIL_FAKE_CODEX_HOME_LOG", homeLog)

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
}
```

Run:

```bash
go test ./internal/cli -run TestRunAuthLoginStatusDoctorAndLogoutUseRailCodexHome -count=1
```

Expected: FAIL because `RunAuth` still expects API-key auth.

- [ ] **Step 2: Add Codex command wrappers to `internal/auth`**

Append these helpers to `internal/auth/actor_auth.go`:

```go
func RunCodexLogin(command string, authHome string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	if err := EnsureCodexAuthHome(authHome); err != nil {
		return err
	}
	return runCodexAuthCommand(command, authHome, stdin, stdout, stderr, "login")
}

func RunCodexLoginStatus(command string, authHome string, stdout io.Writer, stderr io.Writer) error {
	if err := ValidateCodexAuthHome(authHome); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("rail actor auth not configured")
		}
		return err
	}
	if err := runCodexAuthCommand(command, authHome, nil, stdout, stderr, "login", "status"); err != nil {
		return fmt.Errorf("rail actor auth not configured")
	}
	return nil
}

func RunCodexLogout(command string, authHome string, stdout io.Writer, stderr io.Writer) error {
	if err := runCodexAuthCommand(command, authHome, nil, stdout, stderr, "logout"); err != nil && !os.IsNotExist(err) {
		return err
	}
	return RemoveCodexAuthHome(authHome)
}

func runCodexAuthCommand(command string, authHome string, stdin io.Reader, stdout io.Writer, stderr io.Writer, args ...string) error {
	command = strings.TrimSpace(command)
	if command == "" {
		command = "codex"
	}
	cmd := exec.Command(command, args...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = codexAuthCommandEnv(os.Environ(), authHome)
	return cmd.Run()
}

func codexAuthCommandEnv(parent []string, authHome string) []string {
	env := make([]string, 0, len(parent)+2)
	for _, entry := range parent {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if key == "CODEX_HOME" || key == RailCodexAuthHomeEnv {
			continue
		}
		env = append(env, entry)
	}
	env = append(env, "CODEX_HOME="+authHome)
	return env
}
```

Add `io` and `os/exec` to the import block in `internal/auth/actor_auth.go` when adding these wrappers:

```go
import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)
```

- [ ] **Step 3: Rewrite CLI auth parsing and behavior**

Replace `internal/cli/auth.go` with a browser-login command surface:

```go
package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"rail/internal/auth"
)

func RunAuth(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("auth subcommand is required: login, status, logout, or doctor")
	}
	subcommand := args[0]
	options, err := parseAuthOptions(args[1:])
	if err != nil {
		return err
	}
	switch subcommand {
	case "login":
		return runAuthLogin(options, stdin, stdout)
	case "status":
		return runAuthStatus(options, stdout, false)
	case "doctor":
		return runAuthStatus(options, stdout, true)
	case "logout":
		return runAuthLogout(options, stdout)
	default:
		return fmt.Errorf("unknown auth subcommand")
	}
}

type authOptions struct {
	codexCommand string
}

func parseAuthOptions(args []string) (authOptions, error) {
	var options authOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--codex-command":
			i++
			if i >= len(args) || strings.TrimSpace(args[i]) == "" {
				return authOptions{}, fmt.Errorf("--codex-command requires a command")
			}
			options.codexCommand = args[i]
		default:
			return authOptions{}, fmt.Errorf("unknown auth flag")
		}
	}
	return options, nil
}

func runAuthLogin(options authOptions, stdin io.Reader, stdout io.Writer) error {
	authHome, err := railCodexAuthHomeForProcess()
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintln(stdout, "Opening Codex browser login for Rail actor auth...")
	if err := auth.RunCodexLogin(authCommand(options), authHome, stdin, stdout, os.Stderr); err != nil {
		return err
	}
	if err := auth.RunCodexLoginStatus(authCommand(options), authHome, io.Discard, io.Discard); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(stdout, "Rail actor auth configured.")
	_, _ = fmt.Fprintln(stdout, "Secret values are not printed.")
	return nil
}

func runAuthStatus(options authOptions, stdout io.Writer, doctor bool) error {
	authHome, err := railCodexAuthHomeForProcess()
	if err != nil {
		return err
	}
	err = auth.RunCodexLoginStatus(authCommand(options), authHome, io.Discard, io.Discard)
	if err != nil {
		if doctor {
			_, _ = fmt.Fprintln(stdout, "Rail actor auth not configured.")
			_, _ = fmt.Fprintln(stdout, "Run `rail auth login` before standard actor execution.")
			return fmt.Errorf("rail actor auth not configured")
		}
		_, _ = fmt.Fprintln(stdout, "Rail actor auth not configured")
		return nil
	}
	if doctor {
		_, _ = fmt.Fprintln(stdout, "Rail actor auth ready (source=rail_codex_login)")
		_, _ = fmt.Fprintln(stdout, "Secret values are not printed.")
		return nil
	}
	_, _ = fmt.Fprintln(stdout, "Rail actor auth configured (source=rail_codex_login)")
	return nil
}

func runAuthLogout(options authOptions, stdout io.Writer) error {
	authHome, err := railCodexAuthHomeForProcess()
	if err != nil {
		return err
	}
	if err := auth.RunCodexLogout(authCommand(options), authHome, stdout, os.Stderr); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(stdout, "Rail actor auth removed.")
	return nil
}

func railCodexAuthHomeForProcess() (string, error) {
	env := map[string]string{}
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			env[key] = value
		}
	}
	return auth.CodexAuthHomePathFromEnv(env)
}

func authCommand(options authOptions) string {
	if strings.TrimSpace(options.codexCommand) != "" {
		return options.codexCommand
	}
	return "codex"
}
```

The hidden `--codex-command` flag is for deterministic tests and emergency operator use; normal docs should not feature it.

- [ ] **Step 4: Remove old API-key CLI tests**

Delete tests that assert `--api-key`, stdin API key reading, `--auth-file`, or `OPENAI_API_KEY` fallback. Replace the secret-leak test with:

```go
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
```

- [ ] **Step 5: Run CLI auth tests**

Run:

```bash
go test ./internal/cli -run 'TestRunAuth|TestAppIncludesAuthCommand' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit CLI auth wrapper**

```bash
git add internal/auth/actor_auth.go internal/cli/auth.go internal/cli/auth_test.go
git commit -m "feat: wrap codex browser login for rail auth"
```

## Task 3: Materialize Rail Codex Auth Into Sealed Actor Runtime

**Files:**
- Modify: `internal/runtime/actor_runtime_sealed.go`
- Modify: `internal/runtime/actor_runtime_test.go`

- [ ] **Step 1: Write failing sealed runtime auth tests**

Replace `TestPrepareSealedActorRuntimeRequiresEnvAuth` and `TestPrepareSealedActorRuntimeUsesRailActorAuthFile` with:

```go
func TestPrepareSealedActorRuntimeRequiresRailCodexAuth(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	artifactDirectory := t.TempDir()
	workingDirectory := t.TempDir()
	fakeBin := t.TempDir()
	fakeCodexPath := filepath.Join(fakeBin, "codex")
	if err := os.WriteFile(fakeCodexPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("failed to write fake codex: %v", err)
	}

	_, err := prepareSealedActorRuntime(defaultTestActorBackend(), testActorCommandSpec(t, artifactDirectory, workingDirectory, "planner"), fakeCodexParentEnv(t, fakeBin, fakeCodexPath,
		"CODEX_HOME="+filepath.Join(t.TempDir(), ".codex"),
		"RAIL_CODEX_AUTH_HOME="+filepath.Join(t.TempDir(), "missing-auth-home"),
	))
	if err == nil {
		t.Fatalf("expected sealed runtime setup to reject missing Rail Codex auth")
	}
	if !strings.Contains(err.Error(), "rail_actor_auth_not_configured") {
		t.Fatalf("expected rail_actor_auth_not_configured violation, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(artifactDirectory, "runtime")); !os.IsNotExist(statErr) {
		t.Fatalf("expected missing auth preflight to avoid creating sealed runtime directory, stat error: %v", statErr)
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
	authHome := t.TempDir()
	if err := os.WriteFile(filepath.Join(authHome, "auth.json"), []byte(`{"token":"secret"}`), 0o600); err != nil {
		t.Fatalf("failed to write auth.json: %v", err)
	}
	if err := os.Mkdir(filepath.Join(authHome, "skills"), 0o700); err != nil {
		t.Fatalf("failed to create source skills dir: %v", err)
	}

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
		t.Fatalf("expected sealed actor env to omit RAIL_CODEX_AUTH_HOME, got %v", sealed.Env)
	}
	if _, err := os.Stat(filepath.Join(sealed.CodexHome, "auth.json")); err != nil {
		t.Fatalf("expected auth.json in sealed CODEX_HOME: %v", err)
	}
	if _, err := os.Stat(filepath.Join(sealed.CodexHome, "skills")); !os.IsNotExist(err) {
		t.Fatalf("expected source skills not to be copied, stat error: %v", err)
	}
	data, err := os.ReadFile(sealed.ProvenancePath)
	if err != nil {
		t.Fatalf("failed to read provenance: %v", err)
	}
	provenance := string(data)
	if strings.Contains(provenance, "secret") {
		t.Fatalf("expected provenance to omit auth material, got:\n%s", provenance)
	}
	if !strings.Contains(provenance, "auth_source: rail_codex_login") {
		t.Fatalf("expected provenance to record rail_codex_login source, got:\n%s", provenance)
	}
}
```

Run:

```bash
go test ./internal/runtime -run 'TestPrepareSealedActorRuntimeRequiresRailCodexAuth|TestPrepareSealedActorRuntimeMaterializesRailCodexAuth' -count=1
```

Expected: FAIL because runtime still resolves API-key auth.

- [ ] **Step 2: Update sealed runtime struct**

In `internal/runtime/actor_runtime_sealed.go`, remove `AuthSecret` from `sealedActorRuntime` and add auth material metadata:

```go
type sealedActorRuntime struct {
	Root                string
	CodexHome           string
	Home                string
	XDGConfigHome       string
	XDGDataHome         string
	XDGCacheHome        string
	Temp                string
	ProvenancePath      string
	CommandPath         string
	Env                 []string
	SanitizedPath       string
	AuthSource          string
	AuthMaterialized    bool
	AuthMaterialFiles   []string
}
```

- [ ] **Step 3: Materialize auth after sealed directories exist**

In `prepareSealedActorRuntime`, remove the block that calls `auth.ResolveOpenAIAPIKey` and sets `parentMap["OPENAI_API_KEY"]`. After the loop that creates sealed directories, add:

```go
authHome, err := auth.CodexAuthHomePathFromEnv(parentMap)
if err != nil {
	return sealedActorRuntime{}, backendPolicyViolation("rail_actor_auth_resolution_failed: " + err.Error())
}
materializedAuth, err := auth.MaterializeCodexAuthForActor(authHome, sealed.CodexHome)
if err != nil {
	return sealedActorRuntime{}, backendPolicyViolation(err.Error())
}
sealed.AuthSource = materializedAuth.Source
sealed.AuthMaterialized = true
sealed.AuthMaterialFiles = append([]string(nil), materializedAuth.CopiedFiles...)
```

This keeps actor auth inside the actor-local `CODEX_HOME` and keeps auth paths out of `sealed.Env`.

- [ ] **Step 4: Remove actor auth env keys**

In `allowedSealedActorEnvKeys`, remove:

```go
"OPENAI_API_KEY",
"OPENAI_ORG_ID",
"OPENAI_PROJECT",
```

Keep `OPENAI_BASE_URL`, proxy variables, and certificate variables because they are transport configuration rather than actor credentials. They remain covered by redaction because they can still contain secret-bearing values.

- [ ] **Step 5: Update redaction secrets**

In `sealedActorRedactionSecrets`, remove `add(sealed.AuthSecret)`. Keep the loop that redacts secret-bearing env values such as proxies and base URLs:

```go
func sealedActorRedactionSecrets(sealed sealedActorRuntime) []string {
	seen := map[string]struct{}{}
	secrets := []string{}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, exists := seen[value]; exists {
			return
		}
		seen[value] = struct{}{}
		secrets = append(secrets, value)
	}
	for _, entry := range sealed.Env {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || !isSecretBearingActorEnvKey(key) {
			continue
		}
		add(value)
	}
	sort.Slice(secrets, func(i, j int) bool {
		return len(secrets[i]) > len(secrets[j])
	})
	return secrets
}
```

- [ ] **Step 6: Update provenance**

In `writeSealedActorProvenance`, add auth material fields without paths or tokens:

```go
"auth_source":           sealed.AuthSource,
"auth_materialized":     sealed.AuthMaterialized,
"auth_material_files":   sealed.AuthMaterialFiles,
```

Do not include the Rail auth home path or the actor `auth.json` contents.

- [ ] **Step 7: Run focused runtime auth tests**

Run:

```bash
go test ./internal/runtime -run 'TestPrepareSealedActorRuntimeRequiresRailCodexAuth|TestPrepareSealedActorRuntimeMaterializesRailCodexAuth|TestPrepareSealedActorRuntimeDropsUserCodexSurface' -count=1
```

Expected: PASS after updating `TestPrepareSealedActorRuntimeDropsUserCodexSurface` to create a temp Rail auth home with `auth.json` and pass `RAIL_CODEX_AUTH_HOME=<path>` instead of `OPENAI_API_KEY=test-key`.

- [ ] **Step 8: Commit sealed runtime auth materialization**

```bash
git add internal/runtime/actor_runtime_sealed.go internal/runtime/actor_runtime_test.go
git commit -m "feat: materialize codex auth for sealed actors"
```

## Task 4: Update Runtime Tests Away From API-Key Auth

**Files:**
- Modify: `internal/runtime/actor_runtime_test.go`
- Modify: `internal/runtime/integration_test.go`
- Modify: `internal/runtime/runner_test.go`
- Modify: `internal/cli/app_test.go`

- [ ] **Step 1: Add a runtime test helper for Rail auth home**

In `internal/runtime/actor_runtime_test.go`, add:

```go
func testRailCodexAuthHome(t *testing.T) string {
	t.Helper()
	authHome := t.TempDir()
	if err := os.WriteFile(filepath.Join(authHome, "auth.json"), []byte(`{"fake":"auth"}`), 0o600); err != nil {
		t.Fatalf("write fake auth.json: %v", err)
	}
	return authHome
}
```

Use this helper in runtime tests that call `prepareSealedActorRuntime` or `runCommand`.

- [ ] **Step 2: Replace `OPENAI_API_KEY` test setup in runtime actor tests**

For each `t.Setenv("OPENAI_API_KEY", "test-key")` or parent env entry `"OPENAI_API_KEY=..."` in `internal/runtime/actor_runtime_test.go`, replace it with:

```go
authHome := testRailCodexAuthHome(t)
t.Setenv("RAIL_CODEX_AUTH_HOME", authHome)
```

For tests that pass explicit parent env slices into `prepareSealedActorRuntime`, pass:

```go
"RAIL_CODEX_AUTH_HOME="+testRailCodexAuthHome(t),
```

Update expected redaction tests so fake actor scripts leak `HTTPS_PROXY` and `OPENAI_BASE_URL` rather than `OPENAI_API_KEY`. The expected leaked list should no longer include an API key.

- [ ] **Step 3: Update env omission assertions**

In `TestBuildCodexCLIArgsBlocksSecretEnvFromActorShell`, change the forbidden env assertion from:

```go
if strings.Contains(joined, "OPENAI_API_KEY") || strings.Contains(joined, "RAIL_ACTOR_AUTH_FILE") {
	t.Fatalf("expected shell env include list to omit auth variables, got %#v", args)
}
```

to:

```go
if strings.Contains(joined, "OPENAI_API_KEY") || strings.Contains(joined, "RAIL_CODEX_AUTH_HOME") || strings.Contains(joined, "RAIL_ACTOR_AUTH_FILE") {
	t.Fatalf("expected shell env include list to omit auth variables, got %#v", args)
}
```

- [ ] **Step 4: Update integration and runner tests**

Where `internal/runtime/integration_test.go`, `internal/runtime/runner_test.go`, or `internal/cli/app_test.go` set `OPENAI_API_KEY`, use a temp auth home instead:

```go
authHome := t.TempDir()
if err := os.WriteFile(filepath.Join(authHome, "auth.json"), []byte(`{"fake":"auth"}`), 0o600); err != nil {
	t.Fatalf("write fake auth.json: %v", err)
}
t.Setenv("RAIL_CODEX_AUTH_HOME", authHome)
```

Add `path/filepath` to imports where needed.

- [ ] **Step 5: Run runtime and CLI tests**

Run:

```bash
go test ./internal/auth ./internal/cli ./internal/runtime -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit test migration**

```bash
git add internal/runtime/actor_runtime_test.go internal/runtime/integration_test.go internal/runtime/runner_test.go internal/cli/app_test.go
git commit -m "test: use rail codex auth home in actor tests"
```

## Task 5: Update User-Facing Docs And Rail Skill

**Files:**
- Modify: `README.md`
- Modify: `docs/ARCHITECTURE.md`
- Modify: `docs/ARCHITECTURE-kr.md`
- Modify: `skills/Rail/SKILL.md`
- Modify: `assets/skill/Rail/SKILL.md`

- [ ] **Step 1: Update README Actor Auth section**

Replace API-key language in `README.md` with:

~~~markdown
## Actor Auth

Standard actor execution needs authentication, but it must not inherit the
user's normal Codex home. For local use, configure Rail actor auth once:

```bash
rail auth login
rail auth doctor
```

`rail auth login` uses Codex's browser login flow in a Rail-owned auth home.
That login persists for the local machine account across terminal sessions and
target repositories. Actor runs still use artifact-local sealed `CODEX_HOME`
directories and receive only the minimum Codex auth material needed to run.

To remove the local Rail actor auth state:

```bash
rail auth logout
```

Rail never prints stored secret values, and actor provenance records only the
auth source and materialization status, not tokens.
~~~

Keep the surrounding README structure intact.

- [ ] **Step 2: Update architecture docs**

In `docs/ARCHITECTURE.md`, replace the current actor auth paragraph with:

```markdown
Because user Codex login state is intentionally not read from the normal
Codex home, standard actor runs use explicit Rail actor auth. Local users
configure that once with `rail auth login`, which runs Codex browser login in a
Rail-owned auth home. Each actor run still receives an artifact-local sealed
`CODEX_HOME`; Rail materializes only allowlisted auth material into that home.
```

In `docs/ARCHITECTURE-kr.md`, use:

```markdown
사용자의 일반 Codex login state는 의도적으로 읽지 않기 때문에 standard actor
실행은 명시적인 Rail actor auth를 사용합니다. 로컬 사용자는 `rail auth login`을
한 번 실행해 Rail 전용 auth home에서 Codex 브라우저 로그인을 완료합니다.
각 actor 실행은 여전히 artifact-local sealed `CODEX_HOME`을 사용하며, Rail은
허용된 인증 material만 그 home에 전달합니다.
```

- [ ] **Step 3: Update both Rail skill copies**

In `skills/Rail/SKILL.md` and `assets/skill/Rail/SKILL.md`, replace the auth guidance paragraph with:

```markdown
Before any standard actor execution, run `rail auth doctor`. If it fails because actor auth is not configured, run `rail auth login` once, complete the Codex browser login flow, then retry `rail auth doctor`. Do not run `rail auth login` on every skill trigger. Do not ask users to pass API keys in task prompts; Rail stores Codex login state in a Rail-owned auth home outside the request and does not print secret values. The login persists for the local machine account across target repositories unless the user runs `rail auth logout`, the credential expires, or the Rail auth home is removed.
```

Also replace the later execution guard with:

```markdown
If `rail auth doctor` is not ready, do not start `supervise` or `execute`. Run `rail auth login` first, complete browser login, then report that actor auth is ready before continuing. This prevents the actor loop from stopping later with `blocked_environment` due to missing sealed actor credentials.
```

- [ ] **Step 4: Check documentation security rule**

Run:

```bash
rg -n '/Users/|~/' README.md docs/ skills/Rail/SKILL.md assets/skill/Rail/SKILL.md
```

Expected: no new home-directory path examples from this task. Existing unrelated historical archive hits should not be edited unless they are in the touched sections.

- [ ] **Step 5: Commit docs and skill updates**

```bash
git add README.md docs/ARCHITECTURE.md docs/ARCHITECTURE-kr.md skills/Rail/SKILL.md assets/skill/Rail/SKILL.md
git commit -m "docs: describe browser codex actor auth"
```

## Task 6: Remove Remaining API-Key Auth Surface

**Files:**
- Modify any files found by the searches below.

- [ ] **Step 1: Search for removed auth names**

Run:

```bash
rg -n 'RAIL_ACTOR_AUTH_FILE|ActorAuthFile|ResolveOpenAIAPIKey|WriteActorAuthFile|ReadActorAuthFile|RemoveActorAuthFile|OPENAI_API_KEY|--api-key|--auth-file|API key|api key' README.md docs skills assets internal cmd .harness -g '!**/.harness/artifacts/**'
```

Expected: FAIL-style output showing remaining references to remove or consciously retain.

- [ ] **Step 2: Remove or rewrite remaining references**

Use these replacements:

- `RAIL_ACTOR_AUTH_FILE` -> `RAIL_CODEX_AUTH_HOME` only in tests or advanced auth-home override context.
- `OPENAI_API_KEY` actor auth references -> browser `rail auth login` references.
- `auth_file` provenance expectations -> `rail_codex_login`.
- API-key prompt text -> Codex browser login text.

Do not remove unrelated OpenAI API key references outside Rail actor auth unless the search result is clearly part of this pre-release auth flow.

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

Expected: PASS and `build/rail` exists.

- [ ] **Step 5: Run auth doctor missing-state smoke**

Run with a missing auth home:

```bash
RAIL_CODEX_AUTH_HOME="$(mktemp -d)/missing" ./build/rail auth doctor
```

Expected: non-zero exit, stdout includes `Rail actor auth not configured.` and `rail auth login`, and no secret-looking values.

- [ ] **Step 6: Commit cleanup**

```bash
git add README.md docs skills assets internal cmd .harness
git commit -m "chore: remove api key actor auth surface"
```

## Task 7: Manual Browser Login Validation

**Files:**
- No source changes expected.

- [ ] **Step 1: Build the CLI**

Run:

```bash
go build -o build/rail ./cmd/rail
```

Expected: PASS.

- [ ] **Step 2: Run browser login using an isolated test auth home**

Run:

```bash
RAIL_CODEX_AUTH_HOME="$(mktemp -d)" ./build/rail auth login
```

Expected: Codex browser login starts. After successful login, output includes `Rail actor auth configured.` and does not print token values or the concrete auth home path.

- [ ] **Step 3: Verify doctor in a new shell environment**

Reusing the same auth home path from Step 2, run:

```bash
RAIL_CODEX_AUTH_HOME="/absolute/path/to/test-auth-home" ./build/rail auth doctor
```

Expected: PASS with `Rail actor auth ready (source=rail_codex_login)`.

- [ ] **Step 4: Inspect materialized actor auth with a fake standard run**

Use an existing standard request or the smallest local fixture that reaches a Codex-backed actor. After the run starts, inspect:

```bash
find /absolute/path/to/target-repo/.harness/artifacts/<task-id>/runtime -maxdepth 4 -type f | sort
```

Expected: each actor `codex-home` contains `auth.json` and does not contain `skills`, `plugins`, `hooks`, `mcp`, or user `config.toml`.

- [ ] **Step 5: Verify logout only removes Rail actor auth**

Run:

```bash
RAIL_CODEX_AUTH_HOME="/absolute/path/to/test-auth-home" ./build/rail auth logout
RAIL_CODEX_AUTH_HOME="/absolute/path/to/test-auth-home" ./build/rail auth doctor
```

Expected: logout reports Rail actor auth removed. The following doctor command fails and tells the user to run `rail auth login`.

## Final Verification

Run:

```bash
git status --short
go test ./...
go build -o build/rail ./cmd/rail
rg -n '/Users/|~/' README.md docs/ skills/Rail/SKILL.md assets/skill/Rail/SKILL.md
```

Expected:

- `go test ./...` passes.
- `go build` passes.
- `git status --short` only shows intentional changes before the final commit, and is clean after commits.
- Documentation search shows no new machine-specific home-directory path examples in changed docs.
- `rail auth doctor` no longer suggests `OPENAI_API_KEY`.
