package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"rail/internal/runtime"
)

func RunResult(args []string, stdout io.Writer) error {
	var artifactPath string
	var projectRoot string
	latest := false
	jsonOutput := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--artifact":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
				return fmt.Errorf("result requires --artifact")
			}
			artifactPath = args[i+1]
			i++
		case "--latest":
			latest = true
		case "--project-root":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
				return fmt.Errorf("result requires --project-root")
			}
			projectRoot = args[i+1]
			i++
		case "--json":
			jsonOutput = true
		default:
			return fmt.Errorf("unknown result flag: %s", args[i])
		}
	}

	if latest && strings.TrimSpace(artifactPath) != "" {
		return fmt.Errorf("--artifact and --latest are mutually exclusive")
	}
	if latest && strings.TrimSpace(projectRoot) == "" {
		return fmt.Errorf("result --latest requires --project-root")
	}
	if !latest && strings.TrimSpace(projectRoot) != "" {
		return fmt.Errorf("--project-root requires --latest")
	}
	if !latest && strings.TrimSpace(artifactPath) == "" {
		return fmt.Errorf("result requires --artifact")
	}

	result, err := projectHarnessResult(artifactPath, projectRoot, latest)
	if err != nil {
		return err
	}
	if jsonOutput {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, string(data))
		return err
	}
	_, err = fmt.Fprint(stdout, runtime.FormatHarnessResult(result))
	return err
}

func projectHarnessResult(artifactPath string, projectRoot string, latest bool) (runtime.HarnessResult, error) {
	if latest {
		return runtime.ProjectLatestHarnessResult(projectRoot)
	}
	workspace, err := discoverWorkspaceFromPath(artifactPath)
	if err != nil {
		return runtime.HarnessResult{}, err
	}
	resolvedArtifactPath, err := resolveWorkspaceInputPath(workspace.Root, artifactPath)
	if err != nil {
		return runtime.HarnessResult{}, err
	}
	return runtime.ProjectHarnessResultForArtifact(workspace.Root, resolvedArtifactPath)
}
