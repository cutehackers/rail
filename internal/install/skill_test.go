package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallLayoutIncludesPrefixLocalRailSkillAssets(t *testing.T) {
	prefix := "/opt/homebrew/Cellar/rail/0.1.0"

	layout, err := InstallLayout(prefix)
	if err != nil {
		t.Fatalf("InstallLayout returned error: %v", err)
	}

	if got, want := layout.PackageSkillDir, filepath.Join(prefix, "share", "rail", "skill", "Rail"); got != want {
		t.Fatalf("unexpected packaged skill dir: got %q want %q", got, want)
	}
	if got, want := layout.CodexSkillDir, filepath.Join(prefix, "share", "codex", "skills", "rail"); got != want {
		t.Fatalf("unexpected codex skill dir: got %q want %q", got, want)
	}
}

func TestMaterializeBundledSkillCreatesPackagedAndCodexFacingLayouts(t *testing.T) {
	layout, err := InstallLayout(t.TempDir())
	if err != nil {
		t.Fatalf("InstallLayout returned error: %v", err)
	}

	stalePackageFile := filepath.Join(layout.PackageSkillDir, "stale.txt")
	if err := os.MkdirAll(filepath.Dir(stalePackageFile), 0o755); err != nil {
		t.Fatalf("create stale package dir: %v", err)
	}
	if err := os.WriteFile(stalePackageFile, []byte("stale"), 0o644); err != nil {
		t.Fatalf("write stale package file: %v", err)
	}

	staleCodexFile := filepath.Join(layout.CodexSkillDir, "old.txt")
	if err := os.MkdirAll(filepath.Dir(staleCodexFile), 0o755); err != nil {
		t.Fatalf("create stale codex dir: %v", err)
	}
	if err := os.WriteFile(staleCodexFile, []byte("stale"), 0o644); err != nil {
		t.Fatalf("write stale codex file: %v", err)
	}

	files, err := BundledSkillFiles()
	if err != nil {
		t.Fatalf("BundledSkillFiles returned error: %v", err)
	}
	indexed := indexBundledSkillFiles(t, files)

	if err := MaterializeBundledSkill(layout); err != nil {
		t.Fatalf("MaterializeBundledSkill returned error: %v", err)
	}

	if _, err := os.Stat(stalePackageFile); !os.IsNotExist(err) {
		t.Fatalf("expected stale packaged file to be removed, got %v", err)
	}
	if _, err := os.Stat(staleCodexFile); !os.IsNotExist(err) {
		t.Fatalf("expected stale codex file to be removed, got %v", err)
	}

	assertMaterializedFile(t, layout.PackageSkillDir, "SKILL.md", indexed["SKILL.md"])
	assertMaterializedFile(t, layout.PackageSkillDir, filepath.Join("references", "examples.md"), indexed["references/examples.md"])
	assertMaterializedFile(t, layout.CodexSkillDir, "SKILL.md", indexed["SKILL.md"])
	assertMaterializedFile(t, layout.CodexSkillDir, filepath.Join("references", "examples.md"), indexed["references/examples.md"])
}

func TestBundledSkillMatchesCurrentCLIWorkflow(t *testing.T) {
	files, err := BundledSkillFiles()
	if err != nil {
		t.Fatalf("BundledSkillFiles returned error: %v", err)
	}

	indexed := indexBundledSkillFiles(t, files)

	skillDoc := indexed["SKILL.md"]
	if !strings.Contains(skillDoc, "Use this skill through the installed `rail` binary.") {
		t.Fatalf("expected skill doc to reference the installed rail binary, got %q", skillDoc)
	}
	if !strings.Contains(skillDoc, "`rail validate-request` and `rail run`") {
		t.Fatalf("expected skill doc to acknowledge validate-request and run, got %q", skillDoc)
	}
	if strings.Contains(skillDoc, "unless later Go workflow tasks have implemented them") {
		t.Fatalf("expected stale command gating to be removed, got %q", skillDoc)
	}
	if strings.Contains(skillDoc, "local Rail checkout") {
		t.Fatalf("expected skill doc to avoid checkout-era runtime guidance, got %q", skillDoc)
	}

	examples := indexed["references/examples.md"]
	if !strings.Contains(examples, "rail compose-request --stdin") {
		t.Fatalf("expected bundled examples to use the installed rail command, got %q", examples)
	}
	if strings.Contains(examples, "/Users/") || strings.Contains(examples, "~/.codex") {
		t.Fatalf("expected bundled examples to avoid checkout-specific paths, got %q", examples)
	}
}

func TestBundledSkillMatchesRepoOwnedSkillFiles(t *testing.T) {
	files, err := BundledSkillFiles()
	if err != nil {
		t.Fatalf("BundledSkillFiles returned error: %v", err)
	}

	repoRoot := repoRoot(t)
	repoFiles := make(map[string]string)
	err = filepath.Walk(filepath.Join(repoRoot, "skills", "rail"), func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}

		relativePath, err := filepath.Rel(filepath.Join(repoRoot, "skills", "rail"), path)
		if err != nil {
			return err
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		repoFiles[filepath.ToSlash(relativePath)] = string(contents)
		return nil
	})
	if err != nil {
		t.Fatalf("walk repo-owned skill files: %v", err)
	}

	if got, want := len(files), len(repoFiles); got != want {
		t.Fatalf("repo-owned skill file set drifted from bundled asset set: got %d files, want %d", got, want)
	}

	for _, file := range files {
		repoContents, ok := repoFiles[file.RelativePath]
		if !ok {
			t.Fatalf("repo-owned skill file %q missing from repo-owned set", file.RelativePath)
		}
		if repoContents != string(file.Contents) {
			t.Fatalf("repo-owned skill file %q drifted from bundled asset", file.RelativePath)
		}
		delete(repoFiles, file.RelativePath)
	}

	for relativePath := range repoFiles {
		t.Fatalf("repo-owned skill file %q has no bundled counterpart", relativePath)
	}
}

func TestHomebrewFormulaMatchesInstallLayout(t *testing.T) {
	layout, err := InstallLayout("/opt/homebrew/Cellar/rail/0.1.0")
	if err != nil {
		t.Fatalf("InstallLayout returned error: %v", err)
	}

	formulaPath := filepath.Join(repoRoot(t), "packaging", "homebrew", "rail.rb")
	formula, err := os.ReadFile(formulaPath)
	if err != nil {
		t.Fatalf("read formula: %v", err)
	}
	formulaText := string(formula)

	for _, want := range []string{
		`pkgshare.install "assets/skill"`,
		`cp_r (buildpath/"assets/skill/Rail").children, codex_skill_dir`,
		`prefix/"share/codex/skills/rail"`,
		`#{opt_pkgshare}/skill/Rail`,
		`#{opt_prefix}/share/codex/skills/rail`,
		filepath.ToSlash(filepath.Join("share", "codex", "skills", "rail")),
	} {
		if !strings.Contains(formulaText, want) {
			t.Fatalf("expected formula to contain %q", want)
		}
	}

	if got, want := layout.PackageSkillDir, filepath.Join(layout.PackageRoot, "skill", "Rail"); got != want {
		t.Fatalf("packaged skill dir drifted from package root: got %q want %q", got, want)
	}
	if !strings.Contains(formulaText, filepath.ToSlash(strings.TrimPrefix(layout.CodexSkillDir, layout.Prefix+string(filepath.Separator)))) {
		t.Fatalf("formula drifted from codex skill dir layout %q", layout.CodexSkillDir)
	}
}

func TestInstallLayoutRejectsEmptyOrRelativePrefix(t *testing.T) {
	for _, prefix := range []string{"", ".", "relative/path"} {
		t.Run(fmt.Sprintf("prefix=%q", prefix), func(t *testing.T) {
			if _, err := InstallLayout(prefix); err == nil {
				t.Fatalf("expected InstallLayout to reject %q", prefix)
			}
		})
	}
}

func TestInstallLayoutRejectsWhitespaceOnlyPrefix(t *testing.T) {
	for _, prefix := range []string{" ", "\t\n"} {
		t.Run(fmt.Sprintf("prefix=%q", prefix), func(t *testing.T) {
			if _, err := InstallLayout(prefix); err == nil {
				t.Fatalf("expected InstallLayout to reject whitespace-only prefix %q", prefix)
			}
		})
	}
}

func TestMaterializeBundledSkillRejectsRelativeTargets(t *testing.T) {
	err := MaterializeBundledSkill(Layout{
		PackageSkillDir: filepath.Join("relative", "pkg"),
		CodexSkillDir:   filepath.Join("relative", "codex"),
	})
	if err == nil {
		t.Fatal("expected MaterializeBundledSkill to reject relative target dirs")
	}
	if !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("expected absolute path validation error, got %v", err)
	}
}

func TestMaterializeSkillDirRejectsEmptyTargetDir(t *testing.T) {
	files, err := BundledSkillFiles()
	if err != nil {
		t.Fatalf("BundledSkillFiles returned error: %v", err)
	}

	if err := materializeSkillDir("", files); err == nil {
		t.Fatal("expected materializeSkillDir to reject an empty target dir")
	}
}

func indexBundledSkillFiles(t *testing.T, files []SkillFile) map[string]string {
	t.Helper()

	indexed := make(map[string]string, len(files))
	for _, file := range files {
		indexed[file.RelativePath] = string(file.Contents)
	}
	return indexed
}

func assertMaterializedFile(t *testing.T, root, relativePath, want string) {
	t.Helper()

	got, err := os.ReadFile(filepath.Join(root, relativePath))
	if err != nil {
		t.Fatalf("read materialized file %q: %v", relativePath, err)
	}
	if string(got) != want {
		t.Fatalf("unexpected content for %q", relativePath)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("resolve working directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		} else if !os.IsNotExist(err) {
			t.Fatalf("check repo root %q: %v", dir, err)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate repo root from working directory")
		}
		dir = parent
	}
}
