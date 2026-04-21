package runtime

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadActorBackendPolicy(t *testing.T) {
	t.Run("loads embedded defaults and returns default backend", func(t *testing.T) {
		policy, err := loadActorBackendPolicy(t.TempDir())
		if err != nil {
			t.Fatalf("loadActorBackendPolicy returned error: %v", err)
		}

		if got, want := policy.Version, 1; got != want {
			t.Fatalf("unexpected version: got %d want %d", got, want)
		}

		backend, err := policy.DefaultBackend()
		if err != nil {
			t.Fatalf("DefaultBackend returned error: %v", err)
		}
		if backend.Sandbox != "workspace-write" {
			t.Fatalf("unexpected sandbox: got %q want %q", backend.Sandbox, "workspace-write")
		}
		if backend.ApprovalPolicy != "never" {
			t.Fatalf("unexpected approval policy: got %q want %q", backend.ApprovalPolicy, "never")
		}
		if !backend.CaptureJSONEvents {
			t.Fatalf("expected capture_json_events to be true")
		}
	})

	t.Run("project-local policy overrides embedded defaults", func(t *testing.T) {
		projectRoot := writeActorBackendFixture(t, `
version: 1
execution_environment: local
default_backend: codex_cli

backends:
  codex_cli:
    command: codex
    subcommand: exec
    sandbox: read-only
    approval_policy: on-request
    session_mode: per_actor
    ephemeral: false
    capture_json_events: false
    skip_git_repo_check: false

execution_environments:
  local:
    allowed_sandboxes:
      - read-only
`)

		policy, err := loadActorBackendPolicy(projectRoot)
		if err != nil {
			t.Fatalf("loadActorBackendPolicy returned error: %v", err)
		}

		backend, err := policy.DefaultBackend()
		if err != nil {
			t.Fatalf("DefaultBackend returned error: %v", err)
		}
		if backend.Sandbox != "read-only" {
			t.Fatalf("unexpected sandbox: got %q want %q", backend.Sandbox, "read-only")
		}
		if backend.ApprovalPolicy != "on-request" {
			t.Fatalf("unexpected approval policy: got %q want %q", backend.ApprovalPolicy, "on-request")
		}
		if backend.CaptureJSONEvents {
			t.Fatalf("expected capture_json_events to be false")
		}
	})

	t.Run("rejects non-local execution environment even when policy allows full access", func(t *testing.T) {
		projectRoot := writeActorBackendFixture(t, `
version: 1
execution_environment: isolated_ci
default_backend: codex_cli

backends:
  codex_cli:
    command: codex
    subcommand: exec
    sandbox: danger-full-access
    approval_policy: on-request
    session_mode: per_actor
    ephemeral: false
    capture_json_events: false
    skip_git_repo_check: false

execution_environments:
  isolated_ci:
    allowed_sandboxes:
      - workspace-write
      - danger-full-access
`)

		_, err := loadActorBackendPolicy(projectRoot)
		if err == nil {
			t.Fatalf("expected loadActorBackendPolicy to reject non-local execution environment")
		}
		if !strings.Contains(err.Error(), `execution_environment "isolated_ci" is not supported`) {
			t.Fatalf("expected non-local execution environment validation error, got %v", err)
		}
	})

	t.Run("rejects unsafe sandbox when environment does not allow it", func(t *testing.T) {
		projectRoot := writeActorBackendFixture(t, `
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
`)

		_, err := loadActorBackendPolicy(projectRoot)
		if err == nil {
			t.Fatalf("expected loadActorBackendPolicy to reject unsafe sandbox")
		}
		if !strings.Contains(err.Error(), "sandbox danger-full-access is not allowed") {
			t.Fatalf("expected sandbox validation error, got %v", err)
		}
	})

	t.Run("rejects unsupported version", func(t *testing.T) {
		projectRoot := writeActorBackendFixture(t, `
version: 2
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
`)

		_, err := loadActorBackendPolicy(projectRoot)
		if err == nil {
			t.Fatalf("expected loadActorBackendPolicy to reject unsupported version")
		}
		if !strings.Contains(err.Error(), "version must be 1") {
			t.Fatalf("expected version validation error, got %v", err)
		}
	})

	t.Run("rejects missing default backend", func(t *testing.T) {
		projectRoot := writeActorBackendFixture(t, `
version: 1
execution_environment: local

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
`)

		_, err := loadActorBackendPolicy(projectRoot)
		if err == nil {
			t.Fatalf("expected loadActorBackendPolicy to reject missing default backend")
		}
		if !strings.Contains(err.Error(), "default_backend") {
			t.Fatalf("expected default backend validation error, got %v", err)
		}
	})
}

func TestLoadActorBackendPolicyParity(t *testing.T) {
	repoPolicy, err := loadActorBackendPolicy(testRepoRoot(t))
	if err != nil {
		t.Fatalf("loadActorBackendPolicy(repo) returned error: %v", err)
	}

	embeddedPolicy, err := loadActorBackendPolicy(t.TempDir())
	if err != nil {
		t.Fatalf("loadActorBackendPolicy(embedded) returned error: %v", err)
	}

	if !reflect.DeepEqual(repoPolicy, embeddedPolicy) {
		t.Fatalf("expected checked-in and embedded backend policies to match: repo=%+v embedded=%+v", repoPolicy, embeddedPolicy)
	}
}

func writeActorBackendFixture(t *testing.T, body string) string {
	t.Helper()

	projectRoot := t.TempDir()
	backendPath := filepath.Join(projectRoot, ".harness", "supervisor", "actor_backend.yaml")
	if err := os.MkdirAll(filepath.Dir(backendPath), 0o755); err != nil {
		t.Fatalf("failed to create actor backend fixture directory: %v", err)
	}
	if err := os.WriteFile(backendPath, []byte(strings.TrimSpace(body)+"\n"), 0o644); err != nil {
		t.Fatalf("failed to write actor backend fixture: %v", err)
	}
	return projectRoot
}
