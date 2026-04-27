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
