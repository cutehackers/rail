package auth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteReadActorAuthFileStoresSecretWithPrivatePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "actor_auth.yaml")
	if err := WriteActorAuthFile(path, "test-api-key"); err != nil {
		t.Fatalf("WriteActorAuthFile returned error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("failed to stat auth file: %v", err)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0o600); got != want {
		t.Fatalf("unexpected auth file permission: got %v want %v", got, want)
	}

	bundle, err := ReadActorAuthFile(path)
	if err != nil {
		t.Fatalf("ReadActorAuthFile returned error: %v", err)
	}
	if bundle.OpenAIAPIKey != "test-api-key" {
		t.Fatalf("unexpected API key: got %q", bundle.OpenAIAPIKey)
	}
	if bundle.Provider != "openai_api_key" {
		t.Fatalf("unexpected provider: got %q", bundle.Provider)
	}
}

func TestReadActorAuthFileRejectsLoosePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "actor_auth.yaml")
	if err := os.WriteFile(path, []byte("version: 1\nprovider: openai_api_key\nopenai_api_key: test-api-key\n"), 0o644); err != nil {
		t.Fatalf("failed to write auth fixture: %v", err)
	}

	_, err := ReadActorAuthFile(path)
	if err == nil {
		t.Fatalf("expected loose auth file permissions to be rejected")
	}
	if !strings.Contains(err.Error(), "permissions") {
		t.Fatalf("expected permissions error, got %v", err)
	}
}

func TestActorAuthFileRejectsSymlinkPath(t *testing.T) {
	dir := t.TempDir()
	targetPath := filepath.Join(dir, "target.yaml")
	linkPath := filepath.Join(dir, "actor_auth.yaml")
	if err := os.WriteFile(targetPath, []byte("version: 1\nprovider: openai_api_key\nopenai_api_key: test-api-key\n"), 0o600); err != nil {
		t.Fatalf("failed to write target file: %v", err)
	}
	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	if err := WriteActorAuthFile(linkPath, "new-key"); err == nil {
		t.Fatalf("expected WriteActorAuthFile to reject symlink auth path")
	}
	if _, err := ReadActorAuthFile(linkPath); err == nil {
		t.Fatalf("expected ReadActorAuthFile to reject symlink auth path")
	}
}

func TestResolveOpenAIAPIKeyPrefersEnvironmentOverAuthFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "actor_auth.yaml")
	if err := WriteActorAuthFile(path, "file-key"); err != nil {
		t.Fatalf("WriteActorAuthFile returned error: %v", err)
	}

	value, source, err := ResolveOpenAIAPIKey(map[string]string{
		"OPENAI_API_KEY":       "env-key",
		"RAIL_ACTOR_AUTH_FILE": path,
	})
	if err != nil {
		t.Fatalf("ResolveOpenAIAPIKey returned error: %v", err)
	}
	if value != "env-key" || source != "env" {
		t.Fatalf("unexpected resolved auth: value=%q source=%q", value, source)
	}
}

func TestResolveOpenAIAPIKeyUsesRailActorAuthFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "actor_auth.yaml")
	if err := WriteActorAuthFile(path, "file-key"); err != nil {
		t.Fatalf("WriteActorAuthFile returned error: %v", err)
	}

	value, source, err := ResolveOpenAIAPIKey(map[string]string{
		"RAIL_ACTOR_AUTH_FILE": path,
	})
	if err != nil {
		t.Fatalf("ResolveOpenAIAPIKey returned error: %v", err)
	}
	if value != "file-key" || source != "auth_file" {
		t.Fatalf("unexpected resolved auth: value=%q source=%q", value, source)
	}
}
