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

func TestAuditCodexEventsRejectsRecursiveCodexExec(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "shell command",
			body: `{"type":"item.started","item":{"type":"command_execution","command":"codex exec 'continue the task'"}}`,
		},
		{
			name: "extra whitespace",
			body: `{"type":"item.started","item":{"type":"command_execution","command":"codex    exec 'continue the task'"}}`,
		},
		{
			name: "separate args",
			body: `{"type":"item.started","item":{"type":"command_execution","command":"codex","args":["exec","continue the task"]}}`,
		},
		{
			name: "argv path",
			body: `{"type":"item.started","item":{"type":"command_execution","argv":["/usr/local/bin/codex","exec","continue the task"]}}`,
		},
		{
			name: "shell quote concatenated exec",
			body: `{"type":"item.started","item":{"type":"command_execution","command":"codex ex''ec 'continue the task'"}}`,
		},
		{
			name: "shell wrapped",
			body: `{"type":"item.started","item":{"type":"command_execution","command":"sh -c 'codex exec continue the task'"}}`,
		},
		{
			name: "bash login shell wrapped",
			body: `{"type":"item.started","item":{"type":"command_execution","command":"bash -lc 'codex exec continue the task'"}}`,
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
				t.Fatal("expected event audit to reject recursive codex exec")
			}
			if !strings.Contains(err.Error(), "unexpected_recursive_codex_exec") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestAuditCodexEventsAllowsNonRecursiveCodexExecText(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "documentation search command",
			body: `{"type":"item.started","item":{"type":"command_execution","command":"rg \"codex exec\" docs"}}`,
		},
		{
			name: "plain message",
			body: `{"type":"thread.message","message":"Operators can run codex exec outside Rail."}`,
		},
		{
			name: "non codex command args",
			body: `{"type":"item.started","item":{"type":"command_execution","command":"rg","args":["codex exec","docs"]}}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "events.jsonl")
			if err := os.WriteFile(path, []byte(tc.body+"\n"), 0o644); err != nil {
				t.Fatalf("write fixture: %v", err)
			}
			if err := auditCodexEvents(path); err != nil {
				t.Fatalf("expected non-recursive codex exec text to pass, got %v", err)
			}
		})
	}
}

func TestAuditCodexEventsAllowsLargeValidEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	body := `{"type":"thread.started","message":"` + strings.Repeat("x", 128*1024) + `"}`
	if err := os.WriteFile(path, []byte(body+"\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := auditCodexEvents(path); err != nil {
		t.Fatalf("expected large valid event stream to pass, got %v", err)
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
