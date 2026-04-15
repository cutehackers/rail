package project

import (
	"errors"
	"os"
	"path/filepath"

	"rail/assets/scaffold"
)

var scaffoldDirectories = []string{
	".harness/requests",
	".harness/artifacts",
	".harness/learning/feedback",
	".harness/learning/reviews",
	".harness/learning/hardening-reviews",
	".harness/learning/approved",
}

var scaffoldFiles = []string{
	".harness/learning/review_queue.yaml",
	".harness/learning/hardening_queue.yaml",
	".harness/learning/family_evidence_index.yaml",
}

func Init(projectRoot string) error {
	if projectRoot == "" {
		return errors.New("project root is required")
	}

	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return err
	}

	projectFile := filepath.Join(root, ".harness", "project.yaml")
	if _, err := os.Stat(projectFile); err == nil {
		return nil
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	if err := os.MkdirAll(filepath.Join(root, ".harness"), 0o755); err != nil {
		return err
	}

	for _, relPath := range scaffoldDirectories {
		if err := os.MkdirAll(filepath.Join(root, relPath), 0o755); err != nil {
			return err
		}
	}

	projectName := filepath.Base(root)
	projectYAML, err := scaffold.RenderProjectYAML(projectName)
	if err != nil {
		return err
	}
	if err := writeFileIfMissing(projectFile, projectYAML, 0o644); err != nil {
		return err
	}

	for _, relPath := range scaffoldFiles {
		if err := writeFileIfMissing(filepath.Join(root, relPath), []byte{}, 0o644); err != nil {
			return err
		}
	}

	return nil
}

func writeFileIfMissing(path string, contents []byte, mode os.FileMode) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	return os.WriteFile(path, contents, mode)
}
