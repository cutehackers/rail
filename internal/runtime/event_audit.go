package runtime

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"unicode"
)

func auditCodexEvents(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open Codex events audit log: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)
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
		return fmt.Errorf("backend_policy_violation: malformed_codex_event in %s: %w", path, err)
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
		if auditCodexEventMapForRecursiveExec(typed) {
			return "unexpected_recursive_codex_exec"
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

func auditCodexEventMapForRecursiveExec(value map[string]any) bool {
	if !looksLikeCommandExecutionMap(value) {
		return false
	}
	hasCommandKey := false
	for _, commandKey := range []string{"command", "cmd", "program", "executable"} {
		commandValue, ok := value[commandKey].(string)
		if !ok {
			continue
		}
		hasCommandKey = true
		if commandStringInvokesRecursiveCodexExec(commandValue) {
			return true
		}
		if !commandStringNamesCodex(commandValue) {
			continue
		}
		for _, argsKey := range []string{"args", "argv", "arguments"} {
			if args, ok := value[argsKey].([]any); ok && arrayContainsExecToken(args) {
				return true
			}
		}
	}
	if hasCommandKey {
		return false
	}
	for _, key := range []string{"args", "argv", "arguments"} {
		if args, ok := value[key].([]any); ok && argvInvokesRecursiveCodexExec(args) {
			return true
		}
	}
	return false
}

func looksLikeCommandExecutionMap(value map[string]any) bool {
	if eventType, ok := value["type"].(string); ok {
		normalized := strings.ToLower(eventType)
		if strings.Contains(normalized, "command_execution") {
			return true
		}
	}
	for _, key := range []string{"command", "cmd", "program", "executable", "argv"} {
		if _, ok := value[key]; ok {
			return true
		}
	}
	return false
}

func argvInvokesRecursiveCodexExec(values []any) bool {
	tokens := []string{}
	for _, value := range values {
		text, ok := value.(string)
		if !ok {
			continue
		}
		tokens = append(tokens, shellCommandTokens(text)...)
	}
	return argvTokensInvokeRecursiveCodexExec(tokens)
}

func arrayContainsExecToken(values []any) bool {
	for _, value := range values {
		text, ok := value.(string)
		if !ok {
			continue
		}
		for _, token := range shellCommandTokens(text) {
			if normalizeCommandToken(token) == "exec" {
				return true
			}
		}
	}
	return false
}

func commandStringNamesCodex(value string) bool {
	tokens := normalizeCommandInvocationTokens(shellCommandTokens(value))
	return len(tokens) > 0 && commandTokenBase(tokens[0]) == "codex"
}

func commandStringInvokesRecursiveCodexExec(value string) bool {
	for _, segment := range splitShellCommandSegments(value) {
		tokens := shellCommandTokens(segment)
		if len(tokens) == 0 {
			continue
		}
		normalized := normalizeCommandInvocationTokens(tokens)
		if argvTokensInvokeRecursiveCodexExec(normalized) {
			return true
		}
		if shellWrapperInvokesRecursiveCodexExec(normalized) {
			return true
		}
	}
	return false
}

func shellWrapperInvokesRecursiveCodexExec(tokens []string) bool {
	if len(tokens) < 4 || !isShellCommandToken(tokens[0]) {
		return false
	}
	for index := 1; index < len(tokens)-1; index++ {
		option := normalizeCommandToken(tokens[index])
		if !strings.HasPrefix(option, "-") || !strings.Contains(option, "c") {
			continue
		}
		return argvTokensInvokeRecursiveCodexExec(normalizeCommandInvocationTokens(tokens[index+1:]))
	}
	return false
}

func isShellCommandToken(token string) bool {
	switch commandTokenBase(token) {
	case "sh", "bash", "zsh", "fish", "dash", "ksh":
		return true
	default:
		return false
	}
}

func argvTokensInvokeRecursiveCodexExec(tokens []string) bool {
	if len(tokens) < 2 {
		return false
	}
	return commandTokenBase(tokens[0]) == "codex" && normalizeCommandToken(tokens[1]) == "exec"
}

func normalizeCommandInvocationTokens(tokens []string) []string {
	normalized := []string{}
	beforeCommand := true
	for index := 0; index < len(tokens); index++ {
		token := normalizeCommandToken(tokens[index])
		if token == "" {
			continue
		}
		if beforeCommand && token == "env" {
			continue
		}
		if beforeCommand && (token == "command" || token == "builtin" || token == "exec") {
			continue
		}
		if beforeCommand && strings.Contains(token, "=") && !strings.HasPrefix(token, "/") {
			continue
		}
		beforeCommand = false
		normalized = append(normalized, token)
	}
	return normalized
}

func splitShellCommandSegments(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool {
		return r == '\n' || r == ';' || r == '|' || r == '&'
	})
}

func shellCommandTokens(value string) []string {
	unquoted := strings.NewReplacer("'", "", `"`, "", `\`, "").Replace(value)
	rawTokens := strings.FieldsFunc(unquoted, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune(",()[]{}<>", r)
	})
	tokens := []string{}
	for _, raw := range rawTokens {
		token := normalizeCommandToken(raw)
		if token != "" {
			tokens = append(tokens, token)
		}
	}
	return tokens
}

func commandTokenBase(token string) string {
	token = normalizeCommandToken(token)
	if index := strings.LastIndex(token, "/"); index >= 0 {
		return token[index+1:]
	}
	return token
}

func normalizeCommandToken(token string) string {
	token = filepathLikeSlash(strings.ToLower(strings.TrimSpace(token)))
	return strings.Trim(token, "`:$")
}

func filepathLikeSlash(value string) string {
	return strings.ReplaceAll(value, "\\", "/")
}
