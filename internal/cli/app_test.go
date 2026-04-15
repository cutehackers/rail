package cli

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestAppRegistersCoreCommands(t *testing.T) {
	app := NewApp()
	got := app.CommandNames()
	want := []string{"compose-request", "validate-request", "init", "run", "execute", "route-evaluation"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("unexpected commands (-want +got):\n%s", diff)
	}
}
