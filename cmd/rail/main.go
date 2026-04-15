package main

import (
	"os"

	"rail/internal/cli"
)

func main() {
	os.Exit(cli.NewApp().Run(os.Args[1:]))
}
