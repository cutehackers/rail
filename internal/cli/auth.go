package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"rail/internal/auth"
)

const (
	actorAuthNotConfiguredError = "rail actor auth not configured"
	actorAuthCheckUnsafeError   = "rail actor auth cannot be checked because it is not a Rail-owned auth home"
	actorAuthRemoveUnsafeError  = "rail actor auth cannot be removed because it is not a Rail-owned auth home"
)

func RunAuth(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("auth subcommand is required: login, status, logout, or doctor")
	}
	subcommand := args[0]
	options, err := parseAuthOptions(args[1:])
	if err != nil {
		return err
	}
	switch subcommand {
	case "login":
		return runAuthLogin(options, stdin, stdout)
	case "status":
		return runAuthStatus(options, stdout, false)
	case "doctor":
		return runAuthStatus(options, stdout, true)
	case "logout":
		return runAuthLogout(options, stdout)
	default:
		return fmt.Errorf("unknown auth subcommand")
	}
}

type authOptions struct {
	codexCommand string
}

func parseAuthOptions(args []string) (authOptions, error) {
	var options authOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--codex-command":
			i++
			if i >= len(args) || strings.TrimSpace(args[i]) == "" {
				return authOptions{}, fmt.Errorf("--codex-command requires a command")
			}
			options.codexCommand = args[i]
		default:
			return authOptions{}, fmt.Errorf("unknown auth flag")
		}
	}
	return options, nil
}

func runAuthLogin(options authOptions, stdin io.Reader, stdout io.Writer) error {
	authHome, err := railCodexAuthHomeForProcess()
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintln(stdout, "Opening Codex browser login for Rail actor auth...")
	if err := auth.RunCodexLogin(authCommand(options), authHome, stdin, stdout, os.Stderr); err != nil {
		return err
	}
	if err := auth.RunCodexLoginStatus(authCommand(options), authHome, io.Discard, io.Discard); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(stdout, "Rail actor auth configured.")
	_, _ = fmt.Fprintln(stdout, "Secret values are not printed.")
	return nil
}

func runAuthStatus(options authOptions, stdout io.Writer, doctor bool) error {
	authHome, err := railCodexAuthHomeForProcess()
	if err != nil {
		return err
	}
	err = auth.RunCodexLoginStatus(authCommand(options), authHome, io.Discard, io.Discard)
	if err != nil {
		if doctor {
			_, _ = fmt.Fprintln(stdout, "Rail actor auth not configured.")
			_, _ = fmt.Fprintln(stdout, "Run `rail auth login` before standard actor execution.")
			return fmt.Errorf(actorAuthNotConfiguredError)
		}
		if !isActorAuthNotConfigured(err) {
			return fmt.Errorf(actorAuthCheckUnsafeError)
		}
		_, _ = fmt.Fprintln(stdout, "Rail actor auth not configured")
		return nil
	}
	if doctor {
		_, _ = fmt.Fprintln(stdout, "Rail actor auth ready (source=rail_codex_login)")
		_, _ = fmt.Fprintln(stdout, "Secret values are not printed.")
		return nil
	}
	_, _ = fmt.Fprintln(stdout, "Rail actor auth configured (source=rail_codex_login)")
	return nil
}

func runAuthLogout(options authOptions, stdout io.Writer) error {
	authHome, err := railCodexAuthHomeForProcess()
	if err != nil {
		return err
	}
	if err := auth.RunCodexLogout(authCommand(options), authHome, stdout, os.Stderr); err != nil {
		return fmt.Errorf(actorAuthRemoveUnsafeError)
	}
	_, _ = fmt.Fprintln(stdout, "Rail actor auth removed.")
	return nil
}

func railCodexAuthHomeForProcess() (string, error) {
	env := map[string]string{}
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			env[key] = value
		}
	}
	return auth.CodexAuthHomePathFromEnv(env)
}

func authCommand(options authOptions) string {
	if strings.TrimSpace(options.codexCommand) != "" {
		return options.codexCommand
	}
	return "codex"
}

func isActorAuthNotConfigured(err error) bool {
	return err != nil && err.Error() == actorAuthNotConfiguredError
}
