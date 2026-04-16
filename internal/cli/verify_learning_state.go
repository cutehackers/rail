package cli

import (
	"fmt"
	"io"
	"os"

	"rail/internal/runtime"
)

func RunVerifyLearningState(args []string, stdout io.Writer) error {
	if len(args) > 0 {
		return fmt.Errorf("verify-learning-state does not accept flags")
	}

	workingDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}
	workspace, err := discoverWorkspaceFromPath(workingDir)
	if err != nil {
		return err
	}
	runner, err := runtime.NewRunner(workspace.Root)
	if err != nil {
		return err
	}
	summary, err := runner.VerifyLearningState()
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, summary)
	return err
}
