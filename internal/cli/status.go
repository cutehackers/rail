package cli

import (
	"fmt"
	"io"
	"strings"

	"rail/internal/runtime"
)

func RunStatus(args []string, stdout io.Writer) error {
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
			return fmt.Errorf("unknown status flag: %s", args[i])
		}
	}

	if strings.TrimSpace(artifactPath) == "" {
		return fmt.Errorf("status requires --artifact")
	}

	workspace, err := discoverWorkspaceFromPath(artifactPath)
	if err != nil {
		return err
	}
	resolvedArtifactPath, err := resolveWorkspaceInputPath(workspace.Root, artifactPath)
	if err != nil {
		return err
	}
	status, err := runtime.ReadRunStatusForArtifact(workspace.Root, resolvedArtifactPath)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(stdout, runtime.FormatRunStatusSummary(status))
	return err
}
