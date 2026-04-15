package cli

import (
	"fmt"
	"io"
	"strings"

	"rail/internal/runtime"
)

func RunRun(args []string, stdout io.Writer) error {
	var requestPath string
	var projectRoot string
	var taskID string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--request":
			if i+1 >= len(args) {
				return fmt.Errorf("missing value for --request")
			}
			requestPath = args[i+1]
			i++
		case "--project-root":
			if i+1 >= len(args) {
				return fmt.Errorf("missing value for --project-root")
			}
			projectRoot = args[i+1]
			i++
		case "--task-id":
			if i+1 >= len(args) {
				return fmt.Errorf("missing value for --task-id")
			}
			taskID = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown run flag: %s", args[i])
		}
	}

	if strings.TrimSpace(requestPath) == "" {
		return fmt.Errorf("run requires --request")
	}
	if strings.TrimSpace(projectRoot) == "" {
		return fmt.Errorf("run requires --project-root")
	}

	runner, err := runtime.NewRunner(projectRoot)
	if err != nil {
		return err
	}
	artifactPath, err := runner.Run(requestPath, taskID)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, artifactPath)
	return err
}
