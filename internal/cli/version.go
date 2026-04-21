package cli

import (
	"fmt"
	"os"
)

var appVersion = "development"

func RunVersion() error {
	_, err := fmt.Fprintln(os.Stdout, appVersion)
	return err
}
