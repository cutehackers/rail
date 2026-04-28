package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"rail/internal/runtime"
)

const defaultSuperviseRetryBudget = 2

func RunSupervise(args []string, stdout io.Writer) error {
	var artifactPath string
	retryBudget := defaultSuperviseRetryBudget
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--artifact":
			if i+1 >= len(args) {
				return fmt.Errorf("missing value for --artifact")
			}
			artifactPath = args[i+1]
			i++
		case "--retry-budget":
			if i+1 >= len(args) {
				return fmt.Errorf("missing value for --retry-budget")
			}
			value, err := strconv.Atoi(args[i+1])
			if err != nil || value < 0 {
				return fmt.Errorf("--retry-budget must be a non-negative integer")
			}
			retryBudget = value
			i++
		default:
			return fmt.Errorf("unknown supervise flag: %s", args[i])
		}
	}

	if strings.TrimSpace(artifactPath) == "" {
		return fmt.Errorf("supervise requires --artifact")
	}

	workspace, err := discoverWorkspaceFromPath(artifactPath)
	if err != nil {
		return err
	}
	resolvedArtifactPath, err := resolveWorkspaceInputPath(workspace.Root, artifactPath)
	if err != nil {
		return err
	}
	runner, err := runtime.NewRunner(workspace.Root)
	if err != nil {
		return err
	}
	summary, err := runner.Supervise(resolvedArtifactPath, runtime.SuperviseOptions{RetryBudget: retryBudget})
	if err != nil {
		if result, resultErr := runtime.ProjectHarnessResultForArtifact(workspace.Root, resolvedArtifactPath); resultErr == nil {
			_, _ = fmt.Fprint(stdout, runtime.FormatHarnessResult(result))
		} else if status, statusErr := runtime.ReadRunStatusForArtifact(workspace.Root, resolvedArtifactPath); statusErr == nil {
			_, _ = fmt.Fprint(stdout, runtime.FormatRunStatusSummary(status))
		}
		return err
	}
	if _, err := fmt.Fprintln(stdout, summary); err != nil {
		return err
	}
	result, err := runtime.ProjectHarnessResultForArtifact(workspace.Root, resolvedArtifactPath)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(stdout, runtime.FormatHarnessResult(result))
	return err
}
