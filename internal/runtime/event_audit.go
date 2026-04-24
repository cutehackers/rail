package runtime

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func auditCodexEvents(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open Codex events audit log: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return fmt.Errorf("backend_policy_violation: malformed_codex_event in %s", path)
		}
		if violationCode := auditCodexEventValue(event); violationCode != "" {
			return fmt.Errorf("backend_policy_violation: %s in %s", violationCode, path)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan Codex events audit log: %w", err)
	}
	return nil
}

func auditCodexEventValue(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		if eventType, ok := typed["type"].(string); ok {
			normalizedType := strings.ToLower(eventType)
			switch {
			case strings.HasPrefix(normalizedType, "mcp") || strings.Contains(normalizedType, ".mcp"):
				return "unexpected_mcp_usage"
			case strings.HasPrefix(normalizedType, "hook") || strings.Contains(normalizedType, ".hook"):
				return "unexpected_hook_usage"
			}
		}
		for _, nested := range typed {
			if code := auditCodexEventValue(nested); code != "" {
				return code
			}
		}
	case []any:
		for _, nested := range typed {
			if code := auditCodexEventValue(nested); code != "" {
				return code
			}
		}
	case string:
		return auditCodexEventString(typed)
	}
	return ""
}

func auditCodexEventString(value string) string {
	normalized := filepathLikeSlash(strings.ToLower(value))
	switch {
	case strings.Contains(normalized, "/.codex/skills/"),
		strings.Contains(normalized, "/.codex/superpowers/skills/"):
		return "unexpected_skill_injection"
	case strings.Contains(normalized, "/.codex/.tmp/plugins/"),
		strings.Contains(normalized, "/.codex/plugins/"):
		return "unexpected_plugin_load"
	default:
		return ""
	}
}

func filepathLikeSlash(value string) string {
	return strings.ReplaceAll(value, "\\", "/")
}
