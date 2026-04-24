package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditCodexEventsRejectsUnexpectedSkillInjection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(path, []byte(`{"type":"item.started","item":{"type":"command_execution","command":"sed -n '1,220p' /tmp/codex/superpowers/skills/using-superpowers/SKILL.md"}}`+"\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	err := auditCodexEvents(path)
	if err == nil {
		t.Fatal("expected event audit to reject unexpected skill injection")
	}
	if !strings.Contains(err.Error(), "unexpected_skill_injection") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAuditCodexEventsAllowsBasicThreadEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(path, []byte(`{"type":"thread.started","thread_id":"test"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := auditCodexEvents(path); err != nil {
		t.Fatalf("expected basic event stream to pass, got %v", err)
	}
}
