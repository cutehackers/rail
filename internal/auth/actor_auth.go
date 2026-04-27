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
	if value := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); value != "" {
		return filepath.Join(value, "rail", "codex-auth-home"), nil
	}
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
	if err := ensurePrivateDirectory(sourceHome); err != nil {
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
