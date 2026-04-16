package cli

import (
	"fmt"
	"io"
	"strings"

	"rail/internal/contracts"
)

func RunValidateArtifact(args []string, stdout io.Writer) error {
	var filePath string
	var schemaName string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--file":
			if i+1 >= len(args) {
				return fmt.Errorf("missing value for --file")
			}
			filePath = args[i+1]
			i++
		case "--schema":
			if i+1 >= len(args) {
				return fmt.Errorf("missing value for --schema")
			}
			schemaName = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown validate-artifact flag: %s", args[i])
		}
	}

	if strings.TrimSpace(filePath) == "" {
		return fmt.Errorf("validate-artifact requires --file")
	}
	if strings.TrimSpace(schemaName) == "" {
		return fmt.Errorf("validate-artifact requires --schema")
	}

	workspace, err := discoverWorkspaceFromPath(filePath)
	if err != nil {
		return err
	}
	validator, err := contracts.NewValidator(workspace.Root)
	if err != nil {
		return err
	}
	if _, err := validator.ValidateArtifactFile(filePath, schemaName); err != nil {
		return err
	}

	_, err = fmt.Fprintf(stdout, "Artifact is valid for `%s`: %s\n", schemaName, filePath)
	return err
}
