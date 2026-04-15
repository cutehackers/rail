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
