package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunAuthLoginStatusAndLogout(t *testing.T) {
	authFile := filepath.Join(t.TempDir(), "actor_auth.yaml")

	var loginOut bytes.Buffer
	if err := RunAuth([]string{"login", "--auth-file", authFile, "--api-key", "test-api-key"}, strings.NewReader(""), &loginOut); err != nil {
		t.Fatalf("RunAuth login returned error: %v", err)
	}
	if strings.Contains(loginOut.String(), "test-api-key") {
		t.Fatalf("login output leaked secret: %q", loginOut.String())
	}
	if _, err := os.Stat(authFile); err != nil {
		t.Fatalf("expected auth file to exist after login: %v", err)
	}

	var statusOut bytes.Buffer
	if err := RunAuth([]string{"status", "--auth-file", authFile}, strings.NewReader(""), &statusOut); err != nil {
		t.Fatalf("RunAuth status returned error: %v", err)
	}
	if !strings.Contains(statusOut.String(), "configured") {
		t.Fatalf("expected configured status, got %q", statusOut.String())
	}
	if strings.Contains(statusOut.String(), "test-api-key") {
		t.Fatalf("status output leaked secret: %q", statusOut.String())
	}

	var logoutOut bytes.Buffer
	if err := RunAuth([]string{"logout", "--auth-file", authFile}, strings.NewReader(""), &logoutOut); err != nil {
		t.Fatalf("RunAuth logout returned error: %v", err)
	}
	if _, err := os.Stat(authFile); !os.IsNotExist(err) {
		t.Fatalf("expected auth file to be removed after logout, stat error: %v", err)
	}
}

func TestRunAuthLoginReadsAPIKeyFromStdin(t *testing.T) {
	authFile := filepath.Join(t.TempDir(), "actor_auth.yaml")

	var stdout bytes.Buffer
	if err := RunAuth([]string{"login", "--auth-file", authFile}, strings.NewReader("stdin-api-key\n"), &stdout); err != nil {
		t.Fatalf("RunAuth login returned error: %v", err)
	}
	if strings.Contains(stdout.String(), "stdin-api-key") {
		t.Fatalf("login output leaked secret: %q", stdout.String())
	}

	var statusOut bytes.Buffer
	if err := RunAuth([]string{"doctor", "--auth-file", authFile}, strings.NewReader(""), &statusOut); err != nil {
		t.Fatalf("RunAuth doctor returned error: %v", err)
	}
	if !strings.Contains(statusOut.String(), "ready") {
		t.Fatalf("expected doctor to report ready auth, got %q", statusOut.String())
	}
}

func TestRunAuthDoctorFailsClosedWhenAuthMissing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "")
	missingAuthFile := filepath.Join(t.TempDir(), "missing.yaml")

	var stdout bytes.Buffer
	err := RunAuth([]string{"doctor", "--auth-file", missingAuthFile}, strings.NewReader(""), &stdout)
	if err == nil {
		t.Fatalf("expected RunAuth doctor to fail when auth is missing")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("expected not configured error, got %v", err)
	}
	if !strings.Contains(stdout.String(), "rail auth login") {
		t.Fatalf("expected doctor output to explain login next step, got %q", stdout.String())
	}
}

func TestRunAuthDoctorUsesSameAuthFileEnvAsActors(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("OPENAI_API_KEY", "")
	defaultAuthFile := filepath.Join(configHome, "rail", "actor_auth.yaml")
	if err := os.MkdirAll(filepath.Dir(defaultAuthFile), 0o700); err != nil {
		t.Fatalf("failed to create default auth dir: %v", err)
	}
	if err := os.WriteFile(defaultAuthFile, []byte("version: 1\nprovider: openai_api_key\nopenai_api_key: default-key\ncreated_at: test\n"), 0o600); err != nil {
		t.Fatalf("failed to write default auth file: %v", err)
	}
	t.Setenv("RAIL_ACTOR_AUTH_FILE", filepath.Join(t.TempDir(), "missing.yaml"))

	var stdout bytes.Buffer
	err := RunAuth([]string{"doctor"}, strings.NewReader(""), &stdout)
	if err == nil {
		t.Fatalf("expected doctor to fail when RAIL_ACTOR_AUTH_FILE points at missing auth")
	}
	if strings.Contains(stdout.String(), "ready") {
		t.Fatalf("expected doctor not to use default auth file when RAIL_ACTOR_AUTH_FILE is set, got %q", stdout.String())
	}
}

func TestRunAuthErrorsDoNotEchoSecretLookingArguments(t *testing.T) {
	for _, args := range [][]string{
		{"sk-secret-value"},
		{"login", "sk-secret-value"},
	} {
		var stdout bytes.Buffer
		err := RunAuth(args, strings.NewReader(""), &stdout)
		if err == nil {
			t.Fatalf("expected RunAuth(%v) to fail", args)
		}
		if strings.Contains(err.Error(), "sk-secret-value") {
			t.Fatalf("auth error leaked secret-looking argument: %v", err)
		}
	}
}
