package cli

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	codexHome, err := os.MkdirTemp("", "rail-cli-codex-home-*")
	if err != nil {
		panic(err)
	}
	originalCodexHome, hadCodexHome := os.LookupEnv("CODEX_HOME")
	if err := os.Setenv("CODEX_HOME", codexHome); err != nil {
		panic(err)
	}

	code := m.Run()

	if hadCodexHome {
		_ = os.Setenv("CODEX_HOME", originalCodexHome)
	} else {
		_ = os.Unsetenv("CODEX_HOME")
	}
	_ = os.RemoveAll(codexHome)
	os.Exit(code)
}
