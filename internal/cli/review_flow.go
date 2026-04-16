package cli

import (
	"fmt"
	"io"
	"strings"

	"rail/internal/runtime"
)

func RunInitUserOutcomeFeedback(args []string, stdout io.Writer) error {
	var artifactPath string
	var outputPath string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--artifact":
			if i+1 >= len(args) {
				return fmt.Errorf("missing value for --artifact")
			}
			artifactPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return fmt.Errorf("missing value for --output")
			}
			outputPath = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown init-user-outcome-feedback flag: %s", args[i])
		}
	}
	if strings.TrimSpace(artifactPath) == "" {
		return fmt.Errorf("init-user-outcome-feedback requires --artifact")
	}

	workspace, err := discoverWorkspaceFromPath(artifactPath)
	if err != nil {
		return err
	}
	runner, err := runtime.NewRunner(workspace.Root)
	if err != nil {
		return err
	}
	output, err := runner.InitUserOutcomeFeedback(artifactPath, outputPath)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, output)
	return err
}

func RunInitLearningReview(args []string, stdout io.Writer) error {
	var candidatePath string
	var outputPath string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--candidate":
			if i+1 >= len(args) {
				return fmt.Errorf("missing value for --candidate")
			}
			candidatePath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return fmt.Errorf("missing value for --output")
			}
			outputPath = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown init-learning-review flag: %s", args[i])
		}
	}
	if strings.TrimSpace(candidatePath) == "" {
		return fmt.Errorf("init-learning-review requires --candidate")
	}

	workspace, err := discoverWorkspaceFromPath(candidatePath)
	if err != nil {
		return err
	}
	runner, err := runtime.NewRunner(workspace.Root)
	if err != nil {
		return err
	}
	output, err := runner.InitLearningReview(candidatePath, outputPath)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, output)
	return err
}

func RunInitHardeningReview(args []string, stdout io.Writer) error {
	var candidatePath string
	var outputPath string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--candidate":
			if i+1 >= len(args) {
				return fmt.Errorf("missing value for --candidate")
			}
			candidatePath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return fmt.Errorf("missing value for --output")
			}
			outputPath = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown init-hardening-review flag: %s", args[i])
		}
	}
	if strings.TrimSpace(candidatePath) == "" {
		return fmt.Errorf("init-hardening-review requires --candidate")
	}

	workspace, err := discoverWorkspaceFromPath(candidatePath)
	if err != nil {
		return err
	}
	runner, err := runtime.NewRunner(workspace.Root)
	if err != nil {
		return err
	}
	output, err := runner.InitHardeningReview(candidatePath, outputPath)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, output)
	return err
}

func RunApplyUserOutcomeFeedback(args []string, stdout io.Writer) error {
	filePath, err := parseApplyReviewFileArg("apply-user-outcome-feedback", args)
	if err != nil {
		return err
	}
	workspace, err := discoverWorkspaceFromPath(filePath)
	if err != nil {
		return err
	}
	runner, err := runtime.NewRunner(workspace.Root)
	if err != nil {
		return err
	}
	summary, err := runner.ApplyUserOutcomeFeedback(filePath)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, summary)
	return err
}

func RunApplyLearningReview(args []string, stdout io.Writer) error {
	filePath, err := parseApplyReviewFileArg("apply-learning-review", args)
	if err != nil {
		return err
	}
	workspace, err := discoverWorkspaceFromPath(filePath)
	if err != nil {
		return err
	}
	runner, err := runtime.NewRunner(workspace.Root)
	if err != nil {
		return err
	}
	summary, err := runner.ApplyLearningReview(filePath)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, summary)
	return err
}

func RunApplyHardeningReview(args []string, stdout io.Writer) error {
	filePath, err := parseApplyReviewFileArg("apply-hardening-review", args)
	if err != nil {
		return err
	}
	workspace, err := discoverWorkspaceFromPath(filePath)
	if err != nil {
		return err
	}
	runner, err := runtime.NewRunner(workspace.Root)
	if err != nil {
		return err
	}
	summary, err := runner.ApplyHardeningReview(filePath)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, summary)
	return err
}

func parseApplyReviewFileArg(command string, args []string) (string, error) {
	var filePath string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--file":
			if i+1 >= len(args) {
				return "", fmt.Errorf("missing value for --file")
			}
			filePath = args[i+1]
			i++
		default:
			return "", fmt.Errorf("unknown %s flag: %s", command, args[i])
		}
	}
	if strings.TrimSpace(filePath) == "" {
		return "", fmt.Errorf("%s requires --file", command)
	}
	return filePath, nil
}
