package request

import (
	"fmt"
	"strings"
)

var validationTargetCommandPrefixes = []string{
	"./gradlew",
	"bundle ",
	"bun ",
	"deno ",
	"dotnet ",
	"go ",
	"flutter ",
	"dart ",
	"gradle ",
	"make ",
	"mvn ",
	"node ",
	"npx ",
	"npm ",
	"python ",
	"python3 ",
	"yarn ",
	"pnpm ",
	"pytest ",
	"cargo ",
	"swift ",
	"xcodebuild ",
}

var validationTargetCommandWords = map[string]struct{}{
	"bundle":     {},
	"bun":        {},
	"cargo":      {},
	"dart":       {},
	"deno":       {},
	"dotnet":     {},
	"flutter":    {},
	"go":         {},
	"gradle":     {},
	"make":       {},
	"mvn":        {},
	"node":       {},
	"npm":        {},
	"npx":        {},
	"pnpm":       {},
	"python":     {},
	"python3":    {},
	"pytest":     {},
	"swift":      {},
	"xcodebuild": {},
	"yarn":       {},
}

func ValidateValidationTargets(targets []string) error {
	for _, target := range targets {
		trimmed := strings.TrimSpace(target)
		if trimmed == "" {
			continue
		}
		if looksLikeValidationCommand(trimmed) {
			return fmt.Errorf("context.validation_targets must be project-relative file paths, not shell commands: %q", trimmed)
		}
	}
	return nil
}

func looksLikeValidationCommand(target string) bool {
	lower := strings.ToLower(target)
	fields := strings.Fields(lower)
	if len(fields) > 1 {
		return true
	}
	if len(fields) == 1 {
		if _, ok := validationTargetCommandWords[fields[0]]; ok {
			return true
		}
	}
	for _, operator := range []string{"&&", "||", ";", "|", "<", ">"} {
		if strings.Contains(target, operator) {
			return true
		}
	}
	for _, prefix := range validationTargetCommandPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return strings.Contains(target, " --")
}
