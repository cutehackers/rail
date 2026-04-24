package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditCodexEventsRejectsUnexpectedSkillInjection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(path, []byte(`{"type":"item.started","item":{"type":"command_execution","command":"sed -n '1,220p' /tmp/.codex/superpowers/skills/using-superpowers/SKILL.md"}}`+"\n"), 0o644); err != nil {
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

func TestAuditCodexEventsAllowsTargetSkillsDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(path, []byte(`{"type":"item.started","item":{"type":"command_execution","command":"sed -n '1,40p' /repo/skills/rail/SKILL.md"}}`+"\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := auditCodexEvents(path); err != nil {
		t.Fatalf("expected target-local skills directory to pass, got %v", err)
	}
}

func TestAuditCodexEventsRejectsMCPAndHookEvents(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "mcp",
			body: `{"type":"mcp.tool_call.started","server":"filesystem"}`,
			want: "unexpected_mcp_usage",
		},
		{
			name: "hook",
			body: `{"type":"hook.started","hook":"Stop"}`,
			want: "unexpected_hook_usage",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "events.jsonl")
			if err := os.WriteFile(path, []byte(tc.body+"\n"), 0o644); err != nil {
				t.Fatalf("write fixture: %v", err)
			}
			err := auditCodexEvents(path)
			if err == nil {
				t.Fatalf("expected event audit to reject %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("unexpected error: got %v want %s", err, tc.want)
			}
		})
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
