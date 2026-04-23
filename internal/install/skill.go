package install

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	skillassets "rail/assets/skill"
)

const (
	packagedSkillDirName = "Rail"
	codexSkillDirName    = "rail"
	manifestFileName     = ".rail-skill-install.json"
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

type CodexSkillInstallResult struct {
	CodexHome    string
	SkillDir     string
	ManifestPath string
	FilesWritten int
}

type CodexSkillStatus struct {
	CodexHome string
	SkillDir  string
	Healthy   bool
	Problem   string
}

type codexSkillManifest struct {
	SkillName   string            `json:"skill_name"`
	RailVersion string            `json:"rail_version"`
	InstalledAt string            `json:"installed_at"`
	FilesSHA256 map[string]string `json:"files_sha256"`
}

func InstallLayout(prefix string) (Layout, error) {
	trimmedPrefix := strings.TrimSpace(prefix)
	if trimmedPrefix == "" {
		return Layout{}, fmt.Errorf("install prefix is required")
	}

	cleanPrefix := filepath.Clean(trimmedPrefix)
	if !filepath.IsAbs(cleanPrefix) {
		return Layout{}, fmt.Errorf("install prefix must be absolute: %q", prefix)
	}

	return Layout{
		Prefix:          cleanPrefix,
		RailBinary:      filepath.Join(cleanPrefix, "bin", "rail"),
		PackageRoot:     filepath.Join(cleanPrefix, "share", "rail"),
		PackageSkillDir: filepath.Join(cleanPrefix, "share", "rail", "skill", packagedSkillDirName),
		CodexSkillDir:   filepath.Join(cleanPrefix, "share", "codex", "skills", codexSkillDirName),
	}, nil
}

func DefaultCodexHome() (string, error) {
	if value := strings.TrimSpace(os.Getenv("CODEX_HOME")); value != "" {
		return filepath.Abs(value)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home: %w", err)
	}
	return filepath.Join(home, ".codex"), nil
}

func CodexUserSkillDir(codexHome string) (string, string, error) {
	home := strings.TrimSpace(codexHome)
	var err error
	if home == "" {
		home, err = DefaultCodexHome()
		if err != nil {
			return "", "", err
		}
	} else {
		home, err = filepath.Abs(home)
		if err != nil {
			return "", "", err
		}
	}

	return home, filepath.Join(home, "skills", codexSkillDirName), nil
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

func MaterializeCodexUserSkill(codexHome, railVersion string) (CodexSkillInstallResult, error) {
	home, targetDir, err := CodexUserSkillDir(codexHome)
	if err != nil {
		return CodexSkillInstallResult{}, err
	}

	files, err := BundledSkillFiles()
	if err != nil {
		return CodexSkillInstallResult{}, err
	}
	if err := materializeSkillDir(targetDir, files); err != nil {
		return CodexSkillInstallResult{}, fmt.Errorf("materialize Codex user skill: %w", err)
	}

	manifestPath := filepath.Join(targetDir, manifestFileName)
	if err := writeManifest(manifestPath, railVersion, files); err != nil {
		return CodexSkillInstallResult{}, err
	}

	return CodexSkillInstallResult{
		CodexHome:    home,
		SkillDir:     targetDir,
		ManifestPath: manifestPath,
		FilesWritten: len(files) + 1,
	}, nil
}

func CheckCodexUserSkill(codexHome string) (CodexSkillStatus, error) {
	home, targetDir, err := CodexUserSkillDir(codexHome)
	if err != nil {
		return CodexSkillStatus{}, err
	}

	status := CodexSkillStatus{
		CodexHome: home,
		SkillDir:  targetDir,
	}

	dirInfo, err := os.Lstat(targetDir)
	if os.IsNotExist(err) {
		status.Problem = "missing skill directory"
		return status, nil
	}
	if err != nil {
		return status, err
	}
	if dirInfo.Mode()&os.ModeSymlink != 0 {
		status.Problem = "skill directory is a symlink; Codex expects regular files under its user skill root"
		return status, nil
	}
	if !dirInfo.IsDir() {
		status.Problem = "skill path is not a directory"
		return status, nil
	}

	files, err := BundledSkillFiles()
	if err != nil {
		return status, err
	}
	for _, file := range files {
		targetPath := filepath.Join(targetDir, filepath.FromSlash(file.RelativePath))
		info, err := os.Lstat(targetPath)
		if os.IsNotExist(err) {
			status.Problem = fmt.Sprintf("missing %s", file.RelativePath)
			return status, nil
		}
		if err != nil {
			return status, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			status.Problem = fmt.Sprintf("%s is a symlink; Codex expects a regular file", file.RelativePath)
			return status, nil
		}
		if info.IsDir() {
			status.Problem = fmt.Sprintf("%s is a directory", file.RelativePath)
			return status, nil
		}

		got, err := os.ReadFile(targetPath)
		if err != nil {
			return status, err
		}
		if string(got) != string(file.Contents) {
			status.Problem = fmt.Sprintf("%s differs from the bundled Rail skill", file.RelativePath)
			return status, nil
		}
	}

	status.Healthy = true
	return status, nil
}

func materializeSkillDir(targetDir string, files []SkillFile) error {
	if targetDir == "" {
		return fmt.Errorf("target directory is required")
	}
	cleanTargetDir := filepath.Clean(targetDir)
	if !filepath.IsAbs(cleanTargetDir) {
		return fmt.Errorf("target directory must be absolute: %q", targetDir)
	}
	if err := os.RemoveAll(cleanTargetDir); err != nil {
		return err
	}
	for _, file := range files {
		targetPath := filepath.Join(cleanTargetDir, filepath.FromSlash(file.RelativePath))
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(targetPath, file.Contents, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func writeManifest(path, railVersion string, files []SkillFile) error {
	fileHashes := make(map[string]string, len(files))
	for _, file := range files {
		sum := sha256.Sum256(file.Contents)
		fileHashes[file.RelativePath] = hex.EncodeToString(sum[:])
	}

	manifest := codexSkillManifest{
		SkillName:   packagedSkillDirName,
		RailVersion: railVersion,
		InstalledAt: time.Now().UTC().Format(time.RFC3339),
		FilesSHA256: fileHashes,
	}
	payload, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')

	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return fmt.Errorf("write Codex skill install manifest: %w", err)
	}
	return nil
}
