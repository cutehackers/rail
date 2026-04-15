package reporting

import (
	"encoding/json"
	"fmt"
	"os"
)

type State struct {
	TaskID                             string   `json:"taskId"`
	TaskFamily                         string   `json:"taskFamily"`
	TaskFamilySource                   string   `json:"taskFamilySource"`
	Status                             string   `json:"status"`
	CurrentActor                       *string  `json:"currentActor"`
	CompletedActors                    []string `json:"completedActors"`
	GeneratorRetriesRemaining          int      `json:"generatorRetriesRemaining"`
	ContextRebuildsRemaining           int      `json:"contextRebuildsRemaining"`
	ValidationTighteningsRemaining     int      `json:"validationTighteningsRemaining"`
	LastDecision                       *string  `json:"lastDecision"`
	LastReasonCodes                    []string `json:"lastReasonCodes"`
	ActionHistory                      []string `json:"actionHistory"`
	GeneratorRevisionsUsed             int      `json:"generatorRevisionsUsed"`
	ContextRefreshCount                int      `json:"contextRefreshCount"`
	LastContextRefreshTrigger          *string  `json:"lastContextRefreshTrigger"`
	LastContextRefreshReasonFamily     *string  `json:"lastContextRefreshReasonFamily"`
	LastInterventionTriggerReasonCodes []string `json:"lastInterventionTriggerReasonCodes"`
	LastInterventionTriggerCategory    *string  `json:"lastInterventionTriggerCategory"`
	PendingContextRefreshTrigger       *string  `json:"pendingContextRefreshTrigger"`
	PendingContextRefreshReasonFamily  *string  `json:"pendingContextRefreshReasonFamily"`
	ValidationTighteningsUsed          int      `json:"validationTighteningsUsed"`
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
