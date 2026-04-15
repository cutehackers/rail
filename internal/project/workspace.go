package project

import (
	"fmt"
	"os"
	"path/filepath"
)

type Workspace struct {
	Root string
}

func DiscoverProject(start string) (Workspace, error) {
	root, err := filepath.Abs(start)
	if err != nil {
		return Workspace{}, err
	}

	if projectRoot, ok, err := findAncestorWithPath(root, filepath.Join(".harness", "project.yaml")); err != nil {
		return Workspace{}, err
	} else if ok {
		return Workspace{Root: projectRoot}, nil
	}

	if gitRoot, ok, err := findAncestorWithGitRoot(root); err != nil {
		return Workspace{}, err
	} else if ok {
		return Workspace{Root: gitRoot}, nil
	}

	return Workspace{}, fmt.Errorf("no Rail project or git repository found from %s", start)
}

func findAncestorWithPath(start, relPath string) (string, bool, error) {
	current := start
	for {
		info, err := os.Stat(filepath.Join(current, relPath))
		if err == nil && !info.IsDir() {
			return current, true, nil
		}
		if err != nil && !os.IsNotExist(err) {
			return "", false, err
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", false, nil
		}
		current = parent
	}
}

func findAncestorWithGitRoot(start string) (string, bool, error) {
	current := start
	for {
		_, err := os.Stat(filepath.Join(current, ".git"))
		if err == nil {
			return current, true, nil
		}
		if err != nil && !os.IsNotExist(err) {
			return "", false, err
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", false, nil
		}
		current = parent
	}
}
