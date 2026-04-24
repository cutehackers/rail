package runtime

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

var codexEventViolationPatterns = []struct {
	Code    string
	Pattern string
}{
	{Code: "unexpected_skill_injection", Pattern: "/skills/"},
	{Code: "unexpected_skill_injection", Pattern: "superpowers/skills"},
	{Code: "unexpected_plugin_load", Pattern: ".codex/.tmp/plugins"},
}

func auditCodexEvents(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open Codex events audit log: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		for _, match := range codexEventViolationPatterns {
			if strings.Contains(line, match.Pattern) {
				return fmt.Errorf("backend_policy_violation: %s in %s", match.Code, path)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan Codex events audit log: %w", err)
	}
	return nil
}
