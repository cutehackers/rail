package cli

import (
	"fmt"
	"io"
	"strings"

	"rail/internal/runtime"
)

func RunIntegrate(args []string, stdout io.Writer) error {
	var artifactPath string
	var projectRoot string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--artifact":
			if i+1 >= len(args) {
				return fmt.Errorf("missing value for --artifact")
			}
			artifactPath = args[i+1]
			i++
		case "--project-root":
			if i+1 >= len(args) {
				return fmt.Errorf("missing value for --project-root")
			}
			projectRoot = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown integrate flag: %s", args[i])
		}
	}

	if strings.TrimSpace(artifactPath) == "" {
		return fmt.Errorf("integrate requires --artifact")
	}

	workspace, err := discoverWorkspaceFromPath(artifactPath)
	if err != nil {
		return err
	}
	runner, err := runtime.NewRunner(workspace.Root)
	if err != nil {
		return err
	}
	summary, err := runner.Integrate(artifactPath, projectRoot)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, summary)
	return err
}
