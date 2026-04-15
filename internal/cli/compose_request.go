package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"rail/internal/request"

	"gopkg.in/yaml.v3"
)

func RunComposeRequest(args []string, stdin io.Reader, stdout io.Writer) error {
	var (
		readFromStdin bool
		inputPath     string
	)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--stdin":
			readFromStdin = true
		case "--input":
			if i+1 >= len(args) {
				return fmt.Errorf("missing value for --input")
			}
			inputPath = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown compose-request flag: %s", args[i])
		}
	}

	if readFromStdin == (inputPath != "") {
		return fmt.Errorf("compose-request requires exactly one of --stdin or --input")
	}

	reader := stdin
	if inputPath != "" {
		file, err := os.Open(inputPath)
		if err != nil {
			return fmt.Errorf("open draft input: %w", err)
		}
		defer file.Close()
		reader = file
	}

	draft, err := request.DecodeDraft(reader)
	if err != nil {
		return err
	}

	materialized, err := request.NormalizeDraft(draft)
	if err != nil {
		return err
	}
	if info, err := os.Stat(materialized.ProjectRoot); err != nil {
		return fmt.Errorf("project_root must point to an existing directory: %w", err)
	} else if !info.IsDir() {
		return fmt.Errorf("project_root must point to an existing directory")
	}

	outputPath := request.RequestPath(materialized.ProjectRoot)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create request directory: %w", err)
	}

	payload, err := yaml.Marshal(materialized.Request)
	if err != nil {
		return fmt.Errorf("marshal normalized request: %w", err)
	}

	if err := os.WriteFile(outputPath, payload, 0o644); err != nil {
		return fmt.Errorf("write normalized request: %w", err)
	}

	if _, err := fmt.Fprintln(stdout, outputPath); err != nil {
		return fmt.Errorf("write compose-request output: %w", err)
	}

	return nil
}
