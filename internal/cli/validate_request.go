package cli

import (
	"fmt"
	"io"

	"rail/internal/contracts"
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

	workspace, err := discoverWorkspaceFromPath(requestPath)
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
