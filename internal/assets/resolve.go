package assets

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var ErrNoFallback = errors.New("no embedded fallback for stateful harness paths")

var statefulPrefixes = []string{
	".harness/artifacts/",
	".harness/learning/",
	".harness/requests/",
	".harness/fixtures/",
}

func Resolve(projectRoot, relPath string) ([]byte, string, error) {
	localPath := filepath.Join(projectRoot, filepath.FromSlash(relPath))
	if info, err := os.Stat(localPath); err == nil && !info.IsDir() {
		data, readErr := os.ReadFile(localPath)
		if readErr != nil {
			return nil, "", readErr
		}
		return data, "local", nil
	}

	if isStateful(relPath) {
		return nil, "none", fmt.Errorf("%w: %s", ErrNoFallback, relPath)
	}

	embeddedPath, err := toEmbeddedPath(relPath)
	if err != nil {
		return nil, "", err
	}

	data, readErr := fs.ReadFile(defaultAssets, embeddedPath)
	if readErr != nil {
		return nil, "", readErr
	}
	return data, "embedded", nil
}

func isStateful(relPath string) bool {
	for _, prefix := range statefulPrefixes {
		if strings.HasPrefix(relPath, prefix) {
			return true
		}
	}
	return false
}

func toEmbeddedPath(relPath string) (string, error) {
	const harnessPrefix = ".harness/"
	if !strings.HasPrefix(relPath, harnessPrefix) {
		return "", fmt.Errorf("unsupported harness path: %s", relPath)
	}
	return filepath.ToSlash(filepath.Join("defaults", strings.TrimPrefix(relPath, harnessPrefix))), nil
}
