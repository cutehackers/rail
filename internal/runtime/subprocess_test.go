package runtime

import (
	"testing"
	"time"
)

func TestSubprocessRunnerDoesNotRequireZshOnPath(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	result, err := subprocessRunner{}.RunShell("printf portable", t.TempDir(), time.Second)
	if err != nil {
		t.Fatalf("RunShell returned error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("unexpected exit code: got %d want 0", result.ExitCode)
	}
	if result.Stdout != "portable" {
		t.Fatalf("unexpected stdout: got %q want portable", result.Stdout)
	}
}
