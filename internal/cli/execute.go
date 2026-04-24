package cli

import (
	"fmt"
	"io"
	"strings"

	"rail/internal/runtime"
)

func RunExecute(args []string, stdout io.Writer) error {
	var artifactPath string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--artifact":
			if i+1 >= len(args) {
				return fmt.Errorf("missing value for --artifact")
			}
			artifactPath = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown execute flag: %s", args[i])
		}
	}

	if strings.TrimSpace(artifactPath) == "" {
		return fmt.Errorf("execute requires --artifact")
	}

	workspace, err := discoverWorkspaceFromPath(artifactPath)
	if err != nil {
		return err
	}
	runner, err := runtime.NewRunner(workspace.Root)
	if err != nil {
		return err
	}
	summary, err := runner.Execute(artifactPath)
	if err != nil {
		if status, statusErr := runtime.ReadRunStatusForArtifact(workspace.Root, artifactPath); statusErr == nil {
			_, _ = fmt.Fprint(stdout, runtime.FormatRunStatusSummary(status))
		}
		return err
	}
	_, err = fmt.Fprintln(stdout, summary)
	return err
}
