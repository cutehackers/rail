package auth

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	RailCodexAuthHomeEnv = "RAIL_CODEX_AUTH_HOME"
	AuthSourceRailCodex  = "rail_codex_login"
	CodexAuthFileName    = "auth.json"
	authHomeMarkerName   = ".rail-auth-home"
)

type MaterializedCodexAuth struct {
	Source      string
	SourceHome  string
	AuthFile    string
	CopiedFiles []string
}

var currentUID = platformCurrentUID

func DefaultCodexAuthHomePath() (string, error) {
	return defaultCodexAuthHomePath(os.Getenv("XDG_CONFIG_HOME"), true)
}

func CodexAuthHomePathFromEnv(env map[string]string) (string, error) {
	if value := strings.TrimSpace(env[RailCodexAuthHomeEnv]); value != "" {
		return filepath.Abs(value)
	}
	xdgConfigHome, ok := env["XDG_CONFIG_HOME"]
	return defaultCodexAuthHomePath(xdgConfigHome, ok)
}

func EnsureCodexAuthHome(path string) error {
	if err := ensurePlatformSupported(); err != nil {
		return err
	}
	resolved, err := resolveCodexAuthHomePath(path)
	if err != nil {
		return err
	}
	info, err := os.Lstat(resolved)
	existed := err == nil
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("inspect codex auth home: %w", err)
	}
	if existed {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("codex auth home must not be a symlink: %s", resolved)
		}
		if !info.IsDir() {
			return fmt.Errorf("codex auth home must be a directory: %s", resolved)
		}
		marker := filepath.Join(resolved, authHomeMarkerName)
		if markerInfo, err := os.Lstat(marker); err == nil {
			if markerInfo.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("rail auth marker must not be a symlink: %s", marker)
			}
			if !markerInfo.Mode().IsRegular() {
				return fmt.Errorf("rail auth marker must be a regular file: %s", marker)
			}
			if err := validateRailAuthHomeOwnership(resolved); err != nil {
				return err
			}
		} else if os.IsNotExist(err) {
			empty, err := isDirectoryEmpty(resolved)
			if err != nil {
				return err
			}
			if !empty {
				return fmt.Errorf("existing unmarked codex auth home must be empty: %s", resolved)
			}
		} else {
			return fmt.Errorf("inspect rail auth marker: %w", err)
		}
	}
	if err := ensurePrivateDirectory(resolved); err != nil {
		return err
	}
	marker := filepath.Join(resolved, authHomeMarkerName)
	if _, err := os.Lstat(marker); os.IsNotExist(err) {
		content := fmt.Sprintf("version: 1\ncreated_at: %s\n", time.Now().UTC().Format(time.RFC3339))
		if err := os.WriteFile(marker, []byte(content), 0o600); err != nil {
			return fmt.Errorf("write rail auth marker: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("inspect rail auth marker: %w", err)
	}
	return nil
}

func defaultCodexAuthHomePath(xdgConfigHome string, allowAmbientXDG bool) (string, error) {
	if value := strings.TrimSpace(xdgConfigHome); value != "" {
		configDir, err := filepath.Abs(value)
		if err != nil {
			return "", fmt.Errorf("resolve XDG config directory: %w", err)
		}
		return filepath.Join(configDir, "rail", "codex-auth-home"), nil
	}
	if !allowAmbientXDG {
		return userConfigDirWithoutAmbientXDG()
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(configDir, "rail", "codex-auth-home"), nil
}

func userConfigDirWithoutAmbientXDG() (string, error) {
	switch runtime.GOOS {
	case "windows":
		if value := strings.TrimSpace(os.Getenv("AppData")); value != "" {
			return filepath.Join(value, "rail", "codex-auth-home"), nil
		}
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home directory: %w", err)
		}
		return filepath.Join(home, "Library", "Application Support", "rail", "codex-auth-home"), nil
	default:
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home directory: %w", err)
		}
		return filepath.Join(home, ".config", "rail", "codex-auth-home"), nil
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(configDir, "rail", "codex-auth-home"), nil
}

func ValidateCodexAuthHome(path string) error {
	if err := ensurePlatformSupported(); err != nil {
		return err
	}
	resolved, err := resolveCodexAuthHomePath(path)
	if err != nil {
		return err
	}
	return validatePrivateDirectory(resolved)
}

func validateMarkedCodexAuthHome(path string) (string, error) {
	if err := ensurePlatformSupported(); err != nil {
		return "", err
	}
	resolved, err := resolveCodexAuthHomePath(path)
	if err != nil {
		return "", err
	}
	if err := validatePrivateDirectory(resolved); err != nil {
		return "", err
	}
	if err := validateRailAuthHomeMarker(resolved); err != nil {
		return "", err
	}
	if err := validateRailAuthHomeOwnership(resolved); err != nil {
		return "", err
	}
	return resolved, nil
}

func RunCodexLogin(command string, authHome string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	if err := EnsureCodexAuthHome(authHome); err != nil {
		return err
	}
	return runCodexAuthCommand(command, authHome, stdin, stdout, stderr, "login")
}

func RunCodexLoginStatus(command string, authHome string, stdout io.Writer, stderr io.Writer) error {
	resolved, err := validateMarkedCodexAuthHome(authHome)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("rail actor auth not configured")
		}
		return err
	}
	if err := runCodexAuthCommand(command, resolved, nil, stdout, stderr, "login", "status"); err != nil {
		return fmt.Errorf("rail actor auth not configured")
	}
	return nil
}

func RunCodexLogout(command string, authHome string, stdout io.Writer, stderr io.Writer) error {
	resolved, err := resolveCodexAuthHomePath(authHome)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(resolved); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("inspect codex auth home: %w", err)
	}
	resolved, err = validateMarkedCodexAuthHome(resolved)
	if err != nil {
		return err
	}
	if err := runCodexAuthCommand(command, resolved, nil, stdout, stderr, "logout"); err != nil && !os.IsNotExist(err) {
		return err
	}
	return RemoveCodexAuthHome(resolved)
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

func MaterializeCodexAuthForActor(sourceHome string, destinationHome string) (MaterializedCodexAuth, error) {
	if err := ensurePlatformSupported(); err != nil {
		return MaterializedCodexAuth{}, err
	}
	sourceHome, err := resolveCodexAuthHomePath(sourceHome)
	if err != nil {
		return MaterializedCodexAuth{}, err
	}
	destinationHome = strings.TrimSpace(destinationHome)
	if destinationHome == "" {
		return MaterializedCodexAuth{}, fmt.Errorf("actor codex home is required")
	}
	destinationHome, err = filepath.Abs(destinationHome)
	if err != nil {
		return MaterializedCodexAuth{}, fmt.Errorf("resolve actor codex home: %w", err)
	}
	if err := validatePrivateDirectory(sourceHome); err != nil {
		return MaterializedCodexAuth{}, fmt.Errorf("rail_actor_auth_home_unsafe: %w", err)
	}
	if err := ensurePrivateDirectory(destinationHome); err != nil {
		return MaterializedCodexAuth{}, err
	}
	destinationFile := filepath.Join(destinationHome, CodexAuthFileName)
	if err := copyPrivateRegularFile(sourceHome, destinationHome, CodexAuthFileName); err != nil {
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
	if err := ensurePlatformSupported(); err != nil {
		return err
	}
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
	if err := validatePrivateDirectory(resolved); err != nil {
		return err
	}
	if err := validateRailAuthHomeMarker(resolved); err != nil {
		return err
	}
	if err := validateRailAuthHomeOwnership(resolved); err != nil {
		return err
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

func validateRailAuthHomeMarker(path string) error {
	marker := filepath.Join(path, authHomeMarkerName)
	info, err := os.Lstat(marker)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("missing rail auth marker: %s", marker)
		}
		return fmt.Errorf("inspect rail auth marker: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("rail auth marker must not be a symlink: %s", marker)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("rail auth marker must be a regular file: %s", marker)
	}
	return nil
}

func validateRailAuthHomeOwnership(path string) error {
	uid, err := currentUID()
	if err != nil {
		return err
	}
	if err := validatePathOwner(path, uid, "codex auth home"); err != nil {
		return err
	}
	return validatePathOwner(filepath.Join(path, authHomeMarkerName), uid, "rail auth marker")
}

func validatePathOwner(path string, uid uint32, label string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	owner, ok := ownerUID(info)
	if !ok {
		// Conservative fallback: without UID metadata, Rail cannot prove ownership of
		// a marked auth home, so destructive operations and marker trust are denied.
		return fmt.Errorf("%s ownership validation is unsupported on this platform: %s", label, path)
	}
	if owner != uid {
		return fmt.Errorf("%s must be owned by the current user: %s", label, path)
	}
	return nil
}

func isDirectoryEmpty(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, fmt.Errorf("read codex auth home: %w", err)
	}
	return len(entries) == 0, nil
}
