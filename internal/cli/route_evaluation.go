package cli

import (
	"fmt"
	"io"
	"os"

	"rail/internal/project"
	"rail/internal/runtime"
)

func RunRouteEvaluation(args []string, stdout io.Writer) error {
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
			return fmt.Errorf("unknown route-evaluation flag: %s", args[i])
		}
	}
	if artifactPath == "" {
		return fmt.Errorf("route-evaluation requires --artifact")
	}

	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}
	workspace, err := project.DiscoverProject(currentDir)
	if err != nil {
		return err
	}
	router, err := runtime.NewRouter(workspace.Root)
	if err != nil {
		return err
	}
	summary, err := router.RouteEvaluation(artifactPath)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, summary)
	return err
}
