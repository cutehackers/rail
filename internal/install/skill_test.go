package install

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallLayoutIncludesRailSkillAssets(t *testing.T) {
	prefix := "/opt/homebrew/Cellar/rail/0.1.0"
	codexHome := "/tmp/codex-home"

	layout := InstallLayout(prefix, codexHome)

	if got, want := layout.PackageSkillDir, filepath.Join(prefix, "share", "rail", "skill", "Rail"); got != want {
		t.Fatalf("unexpected packaged skill dir: got %q want %q", got, want)
	}
	if got, want := layout.CodexSkillDir, filepath.Join(codexHome, "skills", "rail"); got != want {
		t.Fatalf("unexpected codex skill dir: got %q want %q", got, want)
	}

	files, err := BundledSkillFiles()
	if err != nil {
		t.Fatalf("BundledSkillFiles returned error: %v", err)
	}

	indexed := indexBundledSkillFiles(t, files)
	for _, relPath := range []string{
		"SKILL.md",
		"references/examples.md",
	} {
		if _, ok := indexed[relPath]; !ok {
			t.Fatalf("expected bundled skill asset %q to be present", relPath)
		}
	}
}

func TestBundledSkillReferencesInstalledRailBinary(t *testing.T) {
	files, err := BundledSkillFiles()
	if err != nil {
		t.Fatalf("BundledSkillFiles returned error: %v", err)
	}

	indexed := indexBundledSkillFiles(t, files)

	skillDoc := indexed["SKILL.md"]
	if !strings.Contains(skillDoc, "Use this skill through the installed `rail` binary.") {
		t.Fatalf("expected skill doc to reference the installed rail binary, got %q", skillDoc)
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

func indexBundledSkillFiles(t *testing.T, files []SkillFile) map[string]string {
	t.Helper()

	indexed := make(map[string]string, len(files))
	for _, file := range files {
		indexed[file.RelativePath] = string(file.Contents)
	}
	return indexed
}
