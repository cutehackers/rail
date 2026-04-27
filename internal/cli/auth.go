package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"rail/internal/auth"
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
	authFile string
	apiKey   string
}

func parseAuthOptions(args []string) (authOptions, error) {
	var options authOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--auth-file":
			i++
			if i >= len(args) || strings.TrimSpace(args[i]) == "" {
				return authOptions{}, fmt.Errorf("--auth-file requires a path")
			}
			options.authFile = args[i]
		case "--api-key":
			i++
			if i >= len(args) || strings.TrimSpace(args[i]) == "" {
				return authOptions{}, fmt.Errorf("--api-key requires a value")
			}
			options.apiKey = args[i]
		default:
			return authOptions{}, fmt.Errorf("unknown auth flag")
		}
	}
	return options, nil
}

func runAuthLogin(options authOptions, stdin io.Reader, stdout io.Writer) error {
	apiKey := strings.TrimSpace(options.apiKey)
	if apiKey == "" {
		_, _ = fmt.Fprint(stdout, "Paste OpenAI API key for Rail actor runs: ")
		scanner := bufio.NewScanner(stdin)
		if scanner.Scan() {
			apiKey = strings.TrimSpace(scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("read API key: %w", err)
		}
	}
	if apiKey == "" {
		return fmt.Errorf("API key is required")
	}
	if err := auth.WriteActorAuthFile(options.authFile, apiKey); err != nil {
		return err
	}
	path, err := actorAuthPathForDisplay(options.authFile)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stdout, "Rail actor auth configured at %s\n", path)
	return nil
}

func runAuthStatus(options authOptions, stdout io.Writer, doctor bool) error {
	env := authEnvFromProcess()
	if options.authFile != "" {
		env[auth.ActorAuthFileEnv] = options.authFile
	}
	_, source, err := auth.ResolveOpenAIAPIKey(env)
	if err != nil {
		return err
	}
	if source == "" {
		if doctor {
			_, _ = fmt.Fprintln(stdout, "Rail actor auth not configured.")
			_, _ = fmt.Fprintln(stdout, "Run `rail auth login` or set OPENAI_API_KEY before standard actor execution.")
			return fmt.Errorf("rail actor auth not configured")
		}
		_, _ = fmt.Fprintln(stdout, "Rail actor auth not configured")
		return nil
	}
	if doctor {
		_, _ = fmt.Fprintf(stdout, "Rail actor auth ready (source=%s)\n", source)
		_, _ = fmt.Fprintln(stdout, "Secret values are not printed.")
		return nil
	}
	_, _ = fmt.Fprintf(stdout, "Rail actor auth configured (source=%s)\n", source)
	return nil
}

func authEnvFromProcess() map[string]string {
	env := map[string]string{}
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			env[key] = value
		}
	}
	return env
}

func runAuthLogout(options authOptions, stdout io.Writer) error {
	if err := auth.RemoveActorAuthFile(options.authFile); err != nil {
		return err
	}
	path, err := actorAuthPathForDisplay(options.authFile)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stdout, "Rail actor auth removed from %s\n", path)
	_, _ = fmt.Fprintln(stdout, "If this key should no longer be usable, revoke it in the OpenAI API dashboard.")
	return nil
}

func actorAuthPathForDisplay(path string) (string, error) {
	if strings.TrimSpace(path) != "" {
		return path, nil
	}
	return auth.DefaultActorAuthFilePath()
}
