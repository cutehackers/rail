package reporting

import (
	"encoding/json"
	"fmt"
	"os"
)

type State struct {
	TaskID                         string   `json:"taskId"`
	Status                         string   `json:"status"`
	CurrentActor                   *string  `json:"currentActor"`
	CompletedActors                []string `json:"completedActors"`
	LastDecision                   *string  `json:"lastDecision"`
	LastReasonCodes                []string `json:"lastReasonCodes"`
	ActionHistory                  []string `json:"actionHistory"`
	GeneratorRevisionsUsed         int      `json:"generatorRevisionsUsed"`
	ContextRefreshCount            int      `json:"contextRefreshCount"`
	ValidationTighteningsUsed      int      `json:"validationTighteningsUsed"`
	ContextRebuildsRemaining       int      `json:"contextRebuildsRemaining"`
	GeneratorRetriesRemaining      int      `json:"generatorRetriesRemaining"`
	ValidationTighteningsRemaining int      `json:"validationTighteningsRemaining"`
}

func LoadState(path string) (State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return State{}, fmt.Errorf("read state %s: %w", path, err)
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, fmt.Errorf("decode state %s: %w", path, err)
	}
	return state, nil
}

func WriteState(path string, state State) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write state %s: %w", path, err)
	}
	return nil
}
