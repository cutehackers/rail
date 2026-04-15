package assets

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	defaultassets "rail/assets/defaults"
)

var ErrNoFallback = errors.New("no embedded fallback for stateful harness paths")
var ErrInvalidPath = errors.New("invalid harness path")

var statPath = os.Stat

var statefulPrefixes = []string{
	".harness/artifacts/",
	".harness/learning/",
	".harness/requests/",
	".harness/fixtures/",
}

func Resolve(projectRoot, relPath string) ([]byte, string, error) {
	normalizedPath, err := normalizeHarnessPath(relPath)
	if err != nil {
		return nil, "none", err
	}

	localPath := filepath.Join(projectRoot, filepath.FromSlash(normalizedPath))
	info, statErr := statPath(localPath)
	switch {
	case statErr == nil && info.IsDir():
		return nil, "none", fmt.Errorf("%s is a directory", relPath)
	case statErr == nil:
		data, readErr := os.ReadFile(localPath)
		if readErr != nil {
			return nil, "none", readErr
		}
		return data, "local", nil
	case errors.Is(statErr, fs.ErrNotExist):
		// fall through to embedded defaults
	default:
		return nil, "none", statErr
	}

	if isStateful(normalizedPath) {
		return nil, "none", fmt.Errorf("%w: %s", ErrNoFallback, relPath)
	}

	embeddedPath, err := toEmbeddedPath(normalizedPath)
	if err != nil {
		return nil, "none", err
	}

	data, readErr := fs.ReadFile(defaultassets.FS, embeddedPath)
	if readErr != nil {
		return nil, "none", readErr
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

func normalizeHarnessPath(relPath string) (string, error) {
	if relPath == "" {
		return "", fmt.Errorf("%w: empty path", ErrInvalidPath)
	}
	if filepath.IsAbs(relPath) {
		return "", fmt.Errorf("%w: %s", ErrInvalidPath, relPath)
	}

	normalized := path.Clean(filepath.ToSlash(relPath))
	if normalized == "." || !strings.HasPrefix(normalized, ".harness/") {
		return "", fmt.Errorf("%w: %s", ErrInvalidPath, relPath)
	}
	return normalized, nil
}

func toEmbeddedPath(relPath string) (string, error) {
	const harnessPrefix = ".harness/"
	if !strings.HasPrefix(relPath, harnessPrefix) {
		return "", fmt.Errorf("unsupported harness path: %s", relPath)
	}
	return filepath.ToSlash(strings.TrimPrefix(relPath, harnessPrefix)), nil
}
