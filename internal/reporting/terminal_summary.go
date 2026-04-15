package reporting

import (
	"fmt"
	"os"
)

func ReadTerminalSummary(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read terminal summary %s: %w", path, err)
	}
	return string(data), nil
}

func WriteTerminalSummary(path, summary string) error {
	if err := os.WriteFile(path, []byte(summary), 0o644); err != nil {
		return fmt.Errorf("write terminal summary %s: %w", path, err)
	}
	return nil
}
