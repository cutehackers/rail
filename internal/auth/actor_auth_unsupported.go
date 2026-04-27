//go:build !darwin && !linux && !freebsd && !netbsd && !openbsd

package auth

import (
	"fmt"
	"os"
)

func platformCurrentUID() (uint32, error) {
	return 0, fmt.Errorf("rail codex auth home ownership validation is unsupported on this platform")
}

func ensurePlatformSupported() error {
	return fmt.Errorf("rail codex auth home is unsupported on this platform")
}

func ownerUID(info os.FileInfo) (uint32, bool) {
	return 0, false
}

func copyPrivateRegularFile(sourceHome string, destinationHome string, name string) error {
	return fmt.Errorf("rail codex auth materialization is unsupported on this platform")
}
