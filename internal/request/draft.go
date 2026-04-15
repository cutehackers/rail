package request

import (
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

type Draft struct {
	RequestVersion   string   `json:"request_version" yaml:"request_version"`
	ProjectRoot      string   `json:"project_root" yaml:"project_root"`
	TaskType         string   `json:"task_type" yaml:"task_type"`
	Goal             string   `json:"goal" yaml:"goal"`
	Context          []string `json:"context" yaml:"context"`
	Constraints      []string `json:"constraints" yaml:"constraints"`
	DefinitionOfDone []string `json:"definition_of_done" yaml:"definition_of_done"`
	RiskTolerance    string   `json:"risk_tolerance" yaml:"risk_tolerance"`
}

func DecodeDraft(r io.Reader) (Draft, error) {
	payload, err := io.ReadAll(r)
	if err != nil {
		return Draft{}, fmt.Errorf("read draft: %w", err)
	}

	var draft Draft
	if err := yaml.Unmarshal(payload, &draft); err != nil {
		return Draft{}, fmt.Errorf("decode draft: %w", err)
	}

	return draft, nil
}
