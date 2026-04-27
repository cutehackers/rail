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

func TestDefaultCodexAuthHomePathNormalizesRelativeXDGConfigHome(t *testing.T) {
	base := t.TempDir()
	t.Chdir(base)
	t.Setenv("XDG_CONFIG_HOME", "relative-config")

	path, err := DefaultCodexAuthHomePath()
	if err != nil {
		t.Fatalf("DefaultCodexAuthHomePath returned error: %v", err)
	}
	want := filepath.Join(base, "relative-config", "rail", "codex-auth-home")
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

func TestCodexAuthHomePathFromEnvUsesEnvMapXDGConfigHome(t *testing.T) {
	base := t.TempDir()
	processConfigHome := filepath.Join(base, "process-config")
	mapConfigHome := "map-config"
	t.Chdir(base)
	t.Setenv("XDG_CONFIG_HOME", processConfigHome)

	path, err := CodexAuthHomePathFromEnv(map[string]string{
		"XDG_CONFIG_HOME": mapConfigHome,
	})
	if err != nil {
		t.Fatalf("CodexAuthHomePathFromEnv returned error: %v", err)
	}
	want := filepath.Join(base, mapConfigHome, "rail", "codex-auth-home")
	if path != want {
		t.Fatalf("unexpected auth home: got %q want %q", path, want)
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

func TestEnsureCodexAuthHomeRejectsExistingNonEmptyUnmarkedDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rail-auth")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("mkdir auth home: %v", err)
	}
	if err := os.WriteFile(filepath.Join(path, "unrelated.txt"), []byte("keep"), 0o600); err != nil {
		t.Fatalf("write unrelated file: %v", err)
	}

	err := EnsureCodexAuthHome(path)
	if err == nil {
		t.Fatalf("expected non-empty unmarked auth home to be rejected")
	}
	if !strings.Contains(err.Error(), "existing unmarked codex auth home must be empty") {
		t.Fatalf("expected non-empty unmarked error, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(path, ".rail-auth-home")); !os.IsNotExist(err) {
		t.Fatalf("expected marker not to be created, stat error: %v", err)
	}
}

func TestEnsureCodexAuthHomeAcceptsExistingMarkedDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rail-auth")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("mkdir auth home: %v", err)
	}
	if err := os.WriteFile(filepath.Join(path, ".rail-auth-home"), []byte("version: 1\n"), 0o600); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	if err := os.WriteFile(filepath.Join(path, "auth.json"), []byte(`{"tokens":"secret"}`), 0o600); err != nil {
		t.Fatalf("write auth.json: %v", err)
	}

	if err := EnsureCodexAuthHome(path); err != nil {
		t.Fatalf("EnsureCodexAuthHome returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(path, "auth.json")); err != nil {
		t.Fatalf("expected existing auth.json to remain: %v", err)
	}
}

func TestEnsureCodexAuthHomeRejectsMarkerSymlink(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rail-auth")
	target := filepath.Join(dir, "marker-target")
	marker := filepath.Join(path, ".rail-auth-home")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("mkdir auth home: %v", err)
	}
	if err := os.WriteFile(target, []byte("version: 1\n"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	if err := os.Symlink(target, marker); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	err := EnsureCodexAuthHome(path)
	if err == nil {
		t.Fatalf("expected marker symlink to be rejected")
	}
	if !strings.Contains(err.Error(), "marker must not be a symlink") {
		t.Fatalf("expected marker symlink error, got %v", err)
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
	if err := os.Chmod(source, 0o700); err != nil {
		t.Fatalf("chmod source: %v", err)
	}
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
	destinationFile := filepath.Join(destination, "auth.json")
	if result.AuthFile != destinationFile {
		t.Fatalf("unexpected auth file: got %q want %q", result.AuthFile, destinationFile)
	}
	if len(result.CopiedFiles) != 1 || result.CopiedFiles[0] != "auth.json" {
		t.Fatalf("unexpected copied files: %#v", result.CopiedFiles)
	}
	info, err := os.Stat(destinationFile)
	if err != nil {
		t.Fatalf("expected auth.json to be copied: %v", err)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0o600); got != want {
		t.Fatalf("unexpected auth.json permission: got %v want %v", got, want)
	}
	content, err := os.ReadFile(destinationFile)
	if err != nil {
		t.Fatalf("read copied auth.json: %v", err)
	}
	if string(content) != `{"tokens":"secret"}` {
		t.Fatalf("unexpected copied auth.json content: %q", content)
	}
	if _, err := os.Stat(filepath.Join(destination, "skills")); !os.IsNotExist(err) {
		t.Fatalf("expected skills directory not to be copied, stat error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destination, "config.toml")); !os.IsNotExist(err) {
		t.Fatalf("expected config.toml not to be copied, stat error: %v", err)
	}
}

func TestMaterializeCodexAuthForActorRejectsEmptyDestinationHome(t *testing.T) {
	source := t.TempDir()
	if err := os.Chmod(source, 0o700); err != nil {
		t.Fatalf("chmod source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "auth.json"), []byte(`{"tokens":"secret"}`), 0o600); err != nil {
		t.Fatalf("write auth.json: %v", err)
	}

	_, err := MaterializeCodexAuthForActor(source, "   ")
	if err == nil {
		t.Fatalf("expected empty destination home to be rejected")
	}
	if !strings.Contains(err.Error(), "actor codex home is required") {
		t.Fatalf("expected required destination error, got %v", err)
	}
}

func TestMaterializeCodexAuthForActorRejectsDestinationAuthSymlink(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	destination := filepath.Join(dir, "destination")
	target := filepath.Join(dir, "target-auth.json")
	link := filepath.Join(destination, "auth.json")
	if err := os.Mkdir(source, 0o700); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	if err := os.Mkdir(destination, 0o700); err != nil {
		t.Fatalf("mkdir destination: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "auth.json"), []byte(`{"tokens":"secret"}`), 0o600); err != nil {
		t.Fatalf("write source auth.json: %v", err)
	}
	if err := os.WriteFile(target, []byte(`{"tokens":"old"}`), 0o600); err != nil {
		t.Fatalf("write target auth.json: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	_, err := MaterializeCodexAuthForActor(source, destination)
	if err == nil {
		t.Fatalf("expected destination auth symlink to be rejected")
	}
	if !strings.Contains(err.Error(), "destination auth material must not be a symlink") {
		t.Fatalf("expected destination symlink error, got %v", err)
	}
}

func TestMaterializeCodexAuthForActorDoesNotCreateMissingSourceHome(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "missing-source")
	destination := filepath.Join(dir, "destination")

	_, err := MaterializeCodexAuthForActor(source, destination)
	if err == nil {
		t.Fatalf("expected missing source home to be rejected")
	}
	if _, statErr := os.Stat(source); !os.IsNotExist(statErr) {
		t.Fatalf("expected missing source not to be created, stat error: %v", statErr)
	}
}

func TestRemoveCodexAuthHomeRejectsDirectoryWithoutMarker(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rail-auth")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("mkdir auth home: %v", err)
	}

	err := RemoveCodexAuthHome(path)
	if err == nil {
		t.Fatalf("expected auth home without marker to be rejected")
	}
	if !strings.Contains(err.Error(), "missing rail auth marker") {
		t.Fatalf("expected missing marker error, got %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected auth home not to be removed: %v", err)
	}
}

func TestRemoveCodexAuthHomeRemovesMarkedDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rail-auth")
	if err := EnsureCodexAuthHome(path); err != nil {
		t.Fatalf("EnsureCodexAuthHome returned error: %v", err)
	}

	if err := RemoveCodexAuthHome(path); err != nil {
		t.Fatalf("RemoveCodexAuthHome returned error: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected auth home to be removed, stat error: %v", err)
	}
}
