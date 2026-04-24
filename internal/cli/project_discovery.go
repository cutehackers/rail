package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"rail/internal/project"
)

func discoverWorkspaceFromPath(path string) (project.Workspace, error) {
	if strings.TrimSpace(path) == "" {
		return project.Workspace{}, fmt.Errorf("path is required")
	}

	candidate := path
	if !filepath.IsAbs(candidate) {
		currentDir, err := os.Getwd()
		if err != nil {
			return project.Workspace{}, fmt.Errorf("resolve working directory: %w", err)
		}
		candidate = filepath.Join(currentDir, candidate)
	}
	candidate = filepath.Clean(candidate)

	start := candidate
	info, err := os.Stat(candidate)
	switch {
	case err == nil && !info.IsDir():
		start = filepath.Dir(candidate)
	case err == nil:
		start = candidate
	case os.IsNotExist(err):
		start = filepath.Dir(candidate)
	case err != nil:
		return project.Workspace{}, fmt.Errorf("stat input path: %w", err)
	}

	return project.DiscoverProject(start)
}

func resolveWorkspaceInputPath(workspaceRoot string, path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("path is required")
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	cleanPath := filepath.Clean(path)
	if cleanPath == ".harness" || strings.HasPrefix(cleanPath, ".harness"+string(os.PathSeparator)) {
		return filepath.Clean(filepath.Join(workspaceRoot, cleanPath)), nil
	}
	currentDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}
	return filepath.Clean(filepath.Join(currentDir, cleanPath)), nil
}
