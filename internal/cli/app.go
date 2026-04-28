package cli

import (
	"fmt"
	"os"
)

type App struct {
	commands   []string
	commandSet map[string]struct{}
}

func NewApp() *App {
	commands := []string{
		"version",
		"init-request",
		"compose-request",
		"validate-request",
		"validate-artifact",
		"init",
		"install-codex-skill",
		"doctor",
		"auth",
		"init-user-outcome-feedback",
		"init-learning-review",
		"init-hardening-review",
		"run",
		"execute",
		"supervise",
		"status",
		"result",
		"route-evaluation",
		"integrate",
		"apply-user-outcome-feedback",
		"apply-learning-review",
		"apply-hardening-review",
		"verify-learning-state",
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

	if args[0] == "version" || args[0] == "--version" || args[0] == "-v" {
		if err := RunVersion(); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}

	if args[0] == "init" {
		if err := RunInit(args[1:]); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}

	if args[0] == "install-codex-skill" {
		if err := RunInstallCodexSkill(args[1:], os.Stdout); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}

	if args[0] == "doctor" {
		if err := RunDoctor(args[1:], os.Stdout); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}

	if args[0] == "auth" {
		if err := RunAuth(args[1:], os.Stdin, os.Stdout); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}

	if args[0] == "init-request" {
		if err := RunInitRequest(args[1:], os.Stdout); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}

	if args[0] == "compose-request" {
		if err := RunComposeRequest(args[1:], os.Stdin, os.Stdout); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}

	if args[0] == "validate-request" {
		if err := RunValidateRequest(args[1:], os.Stdout); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}

	if args[0] == "validate-artifact" {
		if err := RunValidateArtifact(args[1:], os.Stdout); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}

	if args[0] == "init-user-outcome-feedback" {
		if err := RunInitUserOutcomeFeedback(args[1:], os.Stdout); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}

	if args[0] == "init-learning-review" {
		if err := RunInitLearningReview(args[1:], os.Stdout); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}

	if args[0] == "init-hardening-review" {
		if err := RunInitHardeningReview(args[1:], os.Stdout); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}

	if args[0] == "route-evaluation" {
		if err := RunRouteEvaluation(args[1:], os.Stdout); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}

	if args[0] == "run" {
		if err := RunRun(args[1:], os.Stdout); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}

	if args[0] == "execute" {
		if err := RunExecute(args[1:], os.Stdout); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}

	if args[0] == "supervise" {
		if err := RunSupervise(args[1:], os.Stdout); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}

	if args[0] == "status" {
		if err := RunStatus(args[1:], os.Stdout); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}

	if args[0] == "result" {
		if err := RunResult(args[1:], os.Stdout); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}

	if args[0] == "integrate" {
		if err := RunIntegrate(args[1:], os.Stdout); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}

	if args[0] == "apply-user-outcome-feedback" {
		if err := RunApplyUserOutcomeFeedback(args[1:], os.Stdout); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}

	if args[0] == "apply-learning-review" {
		if err := RunApplyLearningReview(args[1:], os.Stdout); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}

	if args[0] == "apply-hardening-review" {
		if err := RunApplyHardeningReview(args[1:], os.Stdout); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}

	if args[0] == "verify-learning-state" {
		if err := RunVerifyLearningState(args[1:], os.Stdout); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}

	if _, ok := a.commandSet[args[0]]; ok {
		_, _ = fmt.Fprintf(os.Stderr, "%s is not yet implemented\n", args[0])
		return 1
	}

	return 1
}
