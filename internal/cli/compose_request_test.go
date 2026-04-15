package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const composeRequestSmokeProjectRoot = "/tmp/rail-compose-request-smoke-target"

func TestRunComposeRequestMaterializesCanonicalRequestFromInput(t *testing.T) {
	fixturePath, err := filepath.Abs(filepath.Join("..", "..", "testdata", "request_draft.json"))
	if err != nil {
		t.Fatalf("failed to resolve fixture path: %v", err)
	}

	requestPath := filepath.Join(composeRequestSmokeProjectRoot, ".harness", "requests", "request.yaml")
	_ = os.RemoveAll(composeRequestSmokeProjectRoot)
	t.Cleanup(func() {
		_ = os.RemoveAll(composeRequestSmokeProjectRoot)
	})

	var stdout bytes.Buffer
	if err := RunComposeRequest([]string{"--input", fixturePath}, strings.NewReader(""), &stdout); err != nil {
		t.Fatalf("RunComposeRequest returned error: %v", err)
	}

	if got := strings.TrimSpace(stdout.String()); got != requestPath {
		t.Fatalf("expected request path %q, got %q", requestPath, got)
	}

	assertCanonicalRequestFile(t, requestPath)
}

func TestRunComposeRequestMaterializesCanonicalRequestFromStdin(t *testing.T) {
	projectRoot := t.TempDir()
	requestPath := filepath.Join(projectRoot, ".harness", "requests", "request.yaml")

	draft := `{
	  "request_version": "1",
	  "project_root": "` + projectRoot + `",
	  "task_type": "bug_fix",
	  "goal": "Write the normalized request into the target project",
	  "context": {
	    "feature": "profile",
	    "validation_targets": ["packages/app/test/profile_test.dart"]
	  },
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

	assertCanonicalRequestFile(t, requestPath)
}

func assertCanonicalRequestFile(t *testing.T, requestPath string) {
	t.Helper()

	payload, err := os.ReadFile(requestPath)
	if err != nil {
		t.Fatalf("expected normalized request at %q: %v", requestPath, err)
	}

	text := string(payload)
	for _, want := range []string{
		"context:",
		"feature:",
		"suspected_files:",
		"related_files:",
		"validation_roots:",
		"validation_targets:",
		"priority: medium",
		"validation_profile: standard",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected canonical request to contain %q, got:\n%s", want, text)
		}
	}

	for _, forbidden := range []string{"project_root:", "request_version:"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("expected canonical request to omit %q, got:\n%s", forbidden, text)
		}
	}
}
