package cli

import "os"

type App struct {
	commands   []string
	commandSet map[string]struct{}
}

func NewApp() *App {
	commands := []string{
		"compose-request",
		"validate-request",
		"init",
		"run",
		"execute",
		"route-evaluation",
	}
	commandSet := make(map[string]struct{}, len(commands))
	for _, command := range commands {
		commandSet[command] = struct{}{}
	}

	return &App{commands: commands, commandSet: commandSet}
}

func (a *App) CommandNames() []string {
	return append([]string(nil), a.commands...)
}

func (a *App) Run(args []string) int {
	if len(args) == 0 {
		return 1
	}

	if args[0] == "init" {
		if err := RunInit(args[1:]); err != nil {
			return 1
		}
		return 0
	}

	if args[0] == "compose-request" {
		if err := RunComposeRequest(args[1:], os.Stdin, os.Stdout); err != nil {
			return 1
		}
		return 0
	}

	if _, ok := a.commandSet[args[0]]; ok {
		return 0
	}

	return 1
}
