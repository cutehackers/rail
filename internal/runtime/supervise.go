package runtime

import (
	"fmt"
	"path/filepath"
	"strings"
)

type SuperviseOptions struct {
	RetryBudget int
}

func (r *Runner) Supervise(artifactPath string, options SuperviseOptions) (string, error) {
	artifactDirectory, err := r.router.resolveArtifactDirectory(artifactPath)
	if err != nil {
		return "", err
	}

	retryBudget := options.RetryBudget
	if retryBudget < 0 {
		retryBudget = 0
	}

	attempts := 0
	retries := 0
	for {
		attempts++
		summary, executeErr := r.Execute(artifactDirectory)
		status, statusErr := ReadRunStatus(artifactDirectory)
		if executeErr == nil {
			if statusErr == nil {
				if blockedActor, retryable := r.retryableBlockedActor(artifactDirectory, status); retryable && retries < retryBudget {
					retries++
					if err := appendWorkLedgerEntry(
						filepath.Join(artifactDirectory, workLedgerFileName),
						"Supervise retry scheduled",
						[]string{
							"retry: " + fmt.Sprintf("%d/%d", retries, retryBudget),
							"phase: " + status.Phase,
							"actor: " + blockedActor,
							"interruption: " + fallbackString(status.InterruptionKind, "unknown"),
						},
					); err != nil {
						return "", fmt.Errorf("failed to append supervise retry entry: %w", err)
					}
					if err := writeYAML(
						filepath.Join(artifactDirectory, nextActionFileName),
						buildNextActionForBlockedActorRetry(blockedActor),
					); err != nil {
						return "", fmt.Errorf("failed to write retry next action: %w", err)
					}
					if err := writeRunStatus(artifactDirectory, RunStatus{
						Status:           "retrying",
						Phase:            "blocked_actor_retry",
						CurrentActor:     blockedActor,
						InterruptionKind: "blocked_actor_retry",
						Message:          "Rail supervise is retrying the blocked actor with current sealed runtime isolation.",
						ArtifactDir:      artifactDirectory,
						Evidence: []string{
							"state.json",
							nextActionFileName,
							workLedgerFileName,
							runStatusFileName,
							"runs/",
						},
						NextStep: "Rail supervise is retrying the blocked actor with current sealed runtime isolation.",
					}); err != nil {
						return "", fmt.Errorf("failed to write retry status: %w", err)
					}
					continue
				}
			}
			if statusErr == nil && !isTerminalRunStatus(status) {
				return formatSuperviseSummary(summary, attempts, retries), nil
			}
			return formatSuperviseSummary(summary, attempts, retries), nil
		}
		if statusErr != nil {
			return "", executeErr
		}
		if !isRetryableRunStatus(status) || retries >= retryBudget {
			return "", executeErr
		}

		retries++
		if err := appendWorkLedgerEntry(
			filepath.Join(artifactDirectory, workLedgerFileName),
			"Supervise retry scheduled",
			[]string{
				"retry: " + fmt.Sprintf("%d/%d", retries, retryBudget),
				"phase: " + status.Phase,
				"actor: " + fallbackString(status.CurrentActor, "unknown"),
				"interruption: " + fallbackString(status.InterruptionKind, "unknown"),
			},
		); err != nil {
			return "", fmt.Errorf("%w; additionally failed to append supervise retry entry: %v", executeErr, err)
		}
		status.Status = "retrying"
		status.NextStep = "Rail supervise is retrying the interrupted actor loop."
		if err := writeRunStatus(artifactDirectory, status); err != nil {
			return "", fmt.Errorf("%w; additionally failed to write retry status: %v", executeErr, err)
		}
	}
}

func (r *Runner) retryableBlockedActor(artifactDirectory string, status RunStatus) (string, bool) {
	if status.Status != "blocked_environment" && status.Status != "retrying" {
		return "", false
	}
	state, err := readState(filepath.Join(artifactDirectory, "state.json"))
	if err != nil {
		return "", false
	}
	if state.Status != "blocked_environment" || !state.BlockedRetryable || state.BlockedActor == nil {
		return "", false
	}
	actor := strings.TrimSpace(*state.BlockedActor)
	if actor == "" {
		return "", false
	}
	workflow, err := readWorkflow(filepath.Join(artifactDirectory, "workflow.json"))
	if err != nil {
		return "", false
	}
	if !validBlockedActorForWorkflow(state, workflow) {
		return "", false
	}
	return actor, true
}

func isRetryableRunStatus(status RunStatus) bool {
	if status.Status != "interrupted" {
		return false
	}
	switch status.InterruptionKind {
	case "actor_failed", "actor_watchdog_expired":
		return status.Phase == "actor_execution"
	case "artifact_validation_failed":
		return status.Phase == "artifact_validation"
	default:
		return false
	}
}

func isTerminalRunStatus(status RunStatus) bool {
	switch status.Status {
	case "passed", "rejected", "revise_exhausted", "evolution_exhausted", "blocked_environment", "split_required":
		return true
	default:
		return false
	}
}

func formatSuperviseSummary(summary string, attempts int, retries int) string {
	return fmt.Sprintf(
		"Harness supervised execution finished after %d attempt(s), %d retry(s): %s",
		attempts,
		retries,
		summary,
	)
}
