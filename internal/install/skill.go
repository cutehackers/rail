package install

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	skillassets "rail/assets/skill"
)

const (
	packagedSkillDirName = "Rail"
	codexSkillDirName    = "rail"
)

type Layout struct {
	Prefix          string
	RailBinary      string
	PackageRoot     string
	PackageSkillDir string
	CodexSkillDir   string
}

type SkillFile struct {
	RelativePath string
	Contents     []byte
}

func InstallLayout(prefix string) Layout {
	cleanPrefix := filepath.Clean(prefix)

	return Layout{
		Prefix:          cleanPrefix,
		RailBinary:      filepath.Join(cleanPrefix, "bin", "rail"),
		PackageRoot:     filepath.Join(cleanPrefix, "share", "rail"),
		PackageSkillDir: filepath.Join(cleanPrefix, "share", "rail", "skill", packagedSkillDirName),
		CodexSkillDir:   filepath.Join(cleanPrefix, "share", "codex", "skills", codexSkillDirName),
	}
}

func BundledSkillFiles() ([]SkillFile, error) {
	files := make([]SkillFile, 0, 4)
	err := fs.WalkDir(skillassets.FS, packagedSkillDirName, func(assetPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}

		contents, err := fs.ReadFile(skillassets.FS, assetPath)
		if err != nil {
			return fmt.Errorf("read bundled skill asset %q: %w", assetPath, err)
		}

		files = append(files, SkillFile{
			RelativePath: strings.TrimPrefix(assetPath, packagedSkillDirName+"/"),
			Contents:     contents,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].RelativePath < files[j].RelativePath
	})

	return files, nil
}

func MaterializeBundledSkill(layout Layout) error {
	files, err := BundledSkillFiles()
	if err != nil {
		return err
	}

	for _, targetDir := range []string{layout.PackageSkillDir, layout.CodexSkillDir} {
		if err := materializeSkillDir(targetDir, files); err != nil {
			return fmt.Errorf("materialize bundled skill in %q: %w", targetDir, err)
		}
	}

	return nil
}

func materializeSkillDir(targetDir string, files []SkillFile) error {
	if targetDir == "" {
		return fmt.Errorf("target directory is required")
	}
	if err := os.RemoveAll(targetDir); err != nil {
		return err
	}
	for _, file := range files {
		targetPath := filepath.Join(targetDir, filepath.FromSlash(file.RelativePath))
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(targetPath, file.Contents, 0o644); err != nil {
			return err
		}
	}
	return nil
}
