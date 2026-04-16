package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"rail/internal/assets"
	"rail/internal/contracts"
)

func RunInitRequest(args []string, stdout io.Writer) error {
	outputPath := ".harness/request.template.yaml"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--output":
			if i+1 >= len(args) {
				return fmt.Errorf("missing value for --output")
			}
			outputPath = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown init-request flag: %s", args[i])
		}
	}

	workingDir, err := os.Getwd()
	if err != nil {
		return err
	}
	workspace, err := discoverWorkspaceFromPath(workingDir)
	if err != nil {
		return err
	}
	data, _, err := assets.Resolve(workspace.Root, ".harness/templates/request.template.yaml")
	if err != nil {
		return err
	}
	destination, err := contracts.ResolvePathWithinRoot(workspace.Root, outputPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(destination, data, 0o644); err != nil {
		return fmt.Errorf("write request template: %w", err)
	}
	_, err = fmt.Fprintf(stdout, "Request template written to %s\n", outputPath)
	return err
}
