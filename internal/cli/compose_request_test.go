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
	if err := os.MkdirAll(composeRequestSmokeProjectRoot, 0o755); err != nil {
		t.Fatalf("failed to create project root: %v", err)
	}

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
	    "validation_targets": ["internal/profile/service_test.go"]
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

func TestRunComposeRequestPreservesSmokeValidationProfile(t *testing.T) {
	projectRoot := t.TempDir()
	requestPath := filepath.Join(projectRoot, ".harness", "requests", "request.yaml")

	draft := `{
	  "request_version": "1",
	  "project_root": "` + projectRoot + `",
	  "task_type": "test_repair",
	  "goal": "Verify the smoke control-plane path",
	  "validation_profile": "smoke",
	  "definition_of_done": [
	    "smoke request is materialized"
	  ]
	}`

	var stdout bytes.Buffer
	if err := RunComposeRequest([]string{"--stdin"}, strings.NewReader(draft), &stdout); err != nil {
		t.Fatalf("RunComposeRequest returned error: %v", err)
	}

	if got := strings.TrimSpace(stdout.String()); got != requestPath {
		t.Fatalf("expected request path %q, got %q", requestPath, got)
	}

	payload, err := os.ReadFile(requestPath)
	if err != nil {
		t.Fatalf("expected normalized request at %q: %v", requestPath, err)
	}
	if !strings.Contains(string(payload), "validation_profile: smoke") {
		t.Fatalf("expected smoke validation_profile, got:\n%s", string(payload))
	}
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
