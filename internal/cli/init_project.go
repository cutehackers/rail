package cli

import (
	"fmt"
	"os"

	"rail/internal/install"
	"rail/internal/project"
)

func RunInit(args []string) error {
	targetRoot, err := os.Getwd()
	if err != nil {
		return err
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project-root":
			if i+1 >= len(args) {
				return fmt.Errorf("missing value for --project-root")
			}
			targetRoot = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown init flag: %s", args[i])
		}
	}

	if err := project.Init(targetRoot); err != nil {
		return err
	}

	_, err = install.MaterializeCodexUserSkill("", appVersion)
	return err
}
