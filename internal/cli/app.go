package cli

type App struct {
	commands []string
}

func NewApp() *App {
	return &App{
		commands: []string{
			"compose-request",
			"validate-request",
			"init",
			"run",
			"execute",
			"route-evaluation",
		},
	}
}

func (a *App) CommandNames() []string {
	return append([]string(nil), a.commands...)
}

func (a *App) Run(args []string) int {
	return 0
}
