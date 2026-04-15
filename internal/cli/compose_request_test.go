package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const composeRequestSmokeProjectRoot = "/tmp/rail-compose-request-smoke-target"

func TestRunComposeRequestUsesExplicitProjectRootFromInputDraft(t *testing.T) {
	fixturePath, err := filepath.Abs(filepath.Join("..", "..", "testdata", "request_draft.json"))
	if err != nil {
		t.Fatalf("failed to resolve fixture path: %v", err)
	}

	projectRoot := composeRequestSmokeProjectRoot
	requestPath := filepath.Join(projectRoot, ".harness", "requests", "request.yaml")
	_ = os.RemoveAll(projectRoot)
	t.Cleanup(func() {
		_ = os.RemoveAll(projectRoot)
	})

	var stdout bytes.Buffer
	if err := RunComposeRequest([]string{"--input", fixturePath}, strings.NewReader(""), &stdout); err != nil {
		t.Fatalf("RunComposeRequest returned error: %v", err)
	}

	if got := strings.TrimSpace(stdout.String()); got != requestPath {
		t.Fatalf("expected request path %q, got %q", requestPath, got)
	}

	payload, err := os.ReadFile(requestPath)
	if err != nil {
		t.Fatalf("expected normalized request at %q: %v", requestPath, err)
	}

	if !strings.Contains(string(payload), "project_root: "+projectRoot) {
		t.Fatalf("expected normalized request to include explicit project_root, got:\n%s", payload)
	}
}

func TestRunComposeRequestUsesExplicitProjectRootFromStdinDraft(t *testing.T) {
	projectRoot := t.TempDir()
	requestPath := filepath.Join(projectRoot, ".harness", "requests", "request.yaml")

	draft := `{
	  "request_version": "1",
	  "project_root": "` + projectRoot + `",
	  "task_type": "bug_fix",
	  "goal": "Write the normalized request into the target project",
	  "definition_of_done": [
	    "request file lands in the target repo"
	  ]
	}`

	var stdout bytes.Buffer
	if err := RunComposeRequest([]string{"--stdin"}, strings.NewReader(draft), &stdout); err != nil {
		t.Fatalf("RunComposeRequest returned error: %v", err)
	}

	if got := strings.TrimSpace(stdout.String()); got != requestPath {
		t.Fatalf("expected request path %q, got %q", requestPath, got)
	}

	if _, err := os.Stat(requestPath); err != nil {
		t.Fatalf("expected normalized request at %q: %v", requestPath, err)
	}
}
