package cli

import (
	"fmt"
	"io"

	"rail/internal/install"
)

func RunInstallCodexSkill(args []string, stdout io.Writer) error {
	var codexHome string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--codex-home":
			if i+1 >= len(args) {
				return fmt.Errorf("missing value for --codex-home")
			}
			codexHome = args[i+1]
			i++
		case "--repair":
		default:
			return fmt.Errorf("unknown install-codex-skill flag: %s", args[i])
		}
	}

	result, err := install.MaterializeCodexUserSkill(codexHome, appVersion)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(stdout, "Codex skill installed: %s\n", result.SkillDir); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "Files written: %d\n", result.FilesWritten)
	return err
}

func RunDoctor(args []string, stdout io.Writer) error {
	var codexHome string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--codex-home":
			if i+1 >= len(args) {
				return fmt.Errorf("missing value for --codex-home")
			}
			codexHome = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown doctor flag: %s", args[i])
		}
	}

	status, err := install.CheckCodexUserSkill(codexHome)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(stdout, "Rail version: %s\n", appVersion); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stdout, "Codex home: %s\n", status.CodexHome); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stdout, "Codex skill path: %s\n", status.SkillDir); err != nil {
		return err
	}
	if status.Healthy {
		_, err := fmt.Fprintln(stdout, "Codex skill: installed")
		return err
	}

	if _, err := fmt.Fprintf(stdout, "Codex skill: needs repair (%s)\n", status.Problem); err != nil {
		return err
	}
	repairCommand := "rail install-codex-skill --repair"
	if codexHome != "" {
		repairCommand = fmt.Sprintf("%s --codex-home %q", repairCommand, codexHome)
	}
	return fmt.Errorf("Codex skill needs repair; run %s", repairCommand)
}
