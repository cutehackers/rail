package auth

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	ActorAuthProviderOpenAIAPIKey = "openai_api_key"
	ActorAuthFileEnv              = "RAIL_ACTOR_AUTH_FILE"
)

type ActorAuthFile struct {
	Version      int    `yaml:"version"`
	Provider     string `yaml:"provider"`
	OpenAIAPIKey string `yaml:"openai_api_key"`
	CreatedAt    string `yaml:"created_at"`
}

func WriteActorAuthFile(path string, apiKey string) error {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return fmt.Errorf("API key is required")
	}
	resolvedPath, err := resolveAuthFilePath(path)
	if err != nil {
		return err
	}
	authDir := filepath.Dir(resolvedPath)
	createdDir := false
	if info, err := os.Lstat(authDir); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("inspect auth directory: %w", err)
		}
		createdDir = true
	} else if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("actor auth directory must not be a symlink: %s", authDir)
	}
	if err := os.MkdirAll(authDir, 0o700); err != nil {
		return fmt.Errorf("create auth directory: %w", err)
	}
	if createdDir {
		if err := os.Chmod(authDir, 0o700); err != nil {
			return fmt.Errorf("chmod auth directory: %w", err)
		}
	} else if info, err := os.Stat(authDir); err != nil {
		return fmt.Errorf("inspect auth directory: %w", err)
	} else if info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("auth directory permissions must not be group/world writable: %s", authDir)
	}
	if info, err := os.Lstat(resolvedPath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("actor auth file must not be a symlink: %s", resolvedPath)
		}
		if info.IsDir() {
			return fmt.Errorf("actor auth file is a directory: %s", resolvedPath)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect actor auth file: %w", err)
	}
	value := ActorAuthFile{
		Version:      1,
		Provider:     ActorAuthProviderOpenAIAPIKey,
		OpenAIAPIKey: apiKey,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	data, err := yaml.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal actor auth file: %w", err)
	}
	if err := os.WriteFile(resolvedPath, data, 0o600); err != nil {
		return fmt.Errorf("write actor auth file: %w", err)
	}
	if err := os.Chmod(resolvedPath, 0o600); err != nil {
		return fmt.Errorf("chmod actor auth file: %w", err)
	}
	return nil
}

func ReadActorAuthFile(path string) (ActorAuthFile, error) {
	resolvedPath, err := resolveAuthFilePath(path)
	if err != nil {
		return ActorAuthFile{}, err
	}
	lstatInfo, err := os.Lstat(resolvedPath)
	if err != nil {
		return ActorAuthFile{}, err
	}
	if lstatInfo.Mode()&os.ModeSymlink != 0 {
		return ActorAuthFile{}, fmt.Errorf("actor auth file must not be a symlink: %s", resolvedPath)
	}
	info, err := os.Stat(resolvedPath)
	if err != nil {
		return ActorAuthFile{}, err
	}
	if info.IsDir() {
		return ActorAuthFile{}, fmt.Errorf("actor auth file is a directory: %s", resolvedPath)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return ActorAuthFile{}, fmt.Errorf("actor auth file permissions must be 0600 or stricter: %s", resolvedPath)
	}
	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return ActorAuthFile{}, fmt.Errorf("read actor auth file: %w", err)
	}
	var value ActorAuthFile
	if err := yaml.Unmarshal(data, &value); err != nil {
		return ActorAuthFile{}, fmt.Errorf("decode actor auth file: %w", err)
	}
	if value.Version != 1 {
		return ActorAuthFile{}, fmt.Errorf("actor auth file version must be 1, got %d", value.Version)
	}
	if value.Provider != ActorAuthProviderOpenAIAPIKey {
		return ActorAuthFile{}, fmt.Errorf("unsupported actor auth provider %q", value.Provider)
	}
	if strings.TrimSpace(value.OpenAIAPIKey) == "" {
		return ActorAuthFile{}, fmt.Errorf("actor auth file is missing openai_api_key")
	}
	return value, nil
}

func RemoveActorAuthFile(path string) error {
	resolvedPath, err := resolveAuthFilePath(path)
	if err != nil {
		return err
	}
	if err := os.Remove(resolvedPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("remove actor auth file: %w", err)
	}
	return nil
}

func ResolveOpenAIAPIKey(env map[string]string) (string, string, error) {
	if value := strings.TrimSpace(env["OPENAI_API_KEY"]); value != "" {
		return value, "env", nil
	}
	path, err := ActorAuthFilePathFromEnv(env)
	if err != nil {
		return "", "", err
	}
	bundle, err := ReadActorAuthFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", "", nil
		}
		return "", "", err
	}
	return strings.TrimSpace(bundle.OpenAIAPIKey), "auth_file", nil
}

func ActorAuthFilePathFromEnv(env map[string]string) (string, error) {
	if value := strings.TrimSpace(env[ActorAuthFileEnv]); value != "" {
		return resolveAuthFilePath(value)
	}
	return DefaultActorAuthFilePath()
}

func DefaultActorAuthFilePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(configDir, "rail", "actor_auth.yaml"), nil
}

func resolveAuthFilePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return DefaultActorAuthFilePath()
	}
	resolved, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve actor auth file path: %w", err)
	}
	return resolved, nil
}
