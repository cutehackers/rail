package cli

import (
	"fmt"
	"io"
	"os"

	"rail/internal/contracts"
	"rail/internal/project"
)

func RunValidateRequest(args []string, stdout io.Writer) error {
	var requestPath string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--request":
			if i+1 >= len(args) {
				return fmt.Errorf("missing value for --request")
			}
			requestPath = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown validate-request flag: %s", args[i])
		}
	}
	if requestPath == "" {
		return fmt.Errorf("validate-request requires --request")
	}

	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}
	workspace, err := project.DiscoverProject(currentDir)
	if err != nil {
		return err
	}
	validator, err := contracts.NewValidator(workspace.Root)
	if err != nil {
		return err
	}
	if _, err := validator.ValidateRequestFile(requestPath); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "Request is valid: %s\n", requestPath)
	return err
}
