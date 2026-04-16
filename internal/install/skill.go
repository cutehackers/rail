package install

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const (
	packagedSkillDirName = "Rail"
	codexSkillDirName    = "rail"
)

var bundledSkillPaths = []string{
	"SKILL.md",
	"references/examples.md",
}

type Layout struct {
	Prefix          string
	RailBinary      string
	PackageRoot     string
	PackageSkillDir string
	CodexHome       string
	CodexSkillDir   string
}

type SkillFile struct {
	RelativePath string
	Contents     []byte
}

func InstallLayout(prefix, codexHome string) Layout {
	cleanPrefix := filepath.Clean(prefix)
	cleanCodexHome := filepath.Clean(codexHome)

	return Layout{
		Prefix:          cleanPrefix,
		RailBinary:      filepath.Join(cleanPrefix, "bin", "rail"),
		PackageRoot:     filepath.Join(cleanPrefix, "share", "rail"),
		PackageSkillDir: filepath.Join(cleanPrefix, "share", "rail", "skill", packagedSkillDirName),
		CodexHome:       cleanCodexHome,
		CodexSkillDir:   filepath.Join(cleanCodexHome, "skills", codexSkillDirName),
	}
}

func BundledSkillFiles() ([]SkillFile, error) {
	sourceDir, err := bundledSkillSourceDir()
	if err != nil {
		return nil, err
	}

	files := make([]SkillFile, 0, len(bundledSkillPaths))
	for _, relPath := range bundledSkillPaths {
		contents, err := os.ReadFile(filepath.Join(sourceDir, filepath.FromSlash(relPath)))
		if err != nil {
			return nil, fmt.Errorf("read bundled skill asset %q: %w", relPath, err)
		}
		files = append(files, SkillFile{
			RelativePath: relPath,
			Contents:     contents,
		})
	}

	return files, nil
}

func bundledSkillSourceDir() (string, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("resolve bundled skill source: runtime caller unavailable")
	}

	return filepath.Join(filepath.Dir(filename), "..", "..", "assets", "skill", packagedSkillDirName), nil
}
