package request

import (
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

type DraftContext struct {
	Feature           string   `json:"feature" yaml:"feature"`
	SuspectedFiles    []string `json:"suspected_files" yaml:"suspected_files"`
	RelatedFiles      []string `json:"related_files" yaml:"related_files"`
	ValidationRoots   []string `json:"validation_roots" yaml:"validation_roots"`
	ValidationTargets []string `json:"validation_targets" yaml:"validation_targets"`
}

type Draft struct {
	RequestVersion    string       `json:"request_version" yaml:"request_version"`
	ProjectRoot       string       `json:"project_root" yaml:"project_root"`
	TaskType          string       `json:"task_type" yaml:"task_type"`
	Goal              string       `json:"goal" yaml:"goal"`
	Context           DraftContext `json:"context" yaml:"context"`
	Constraints       []string     `json:"constraints" yaml:"constraints"`
	DefinitionOfDone  []string     `json:"definition_of_done" yaml:"definition_of_done"`
	RiskTolerance     string       `json:"risk_tolerance" yaml:"risk_tolerance"`
	ValidationProfile string       `json:"validation_profile" yaml:"validation_profile"`
}

type RequestContext struct {
	Feature           string   `yaml:"feature"`
	SuspectedFiles    []string `yaml:"suspected_files"`
	RelatedFiles      []string `yaml:"related_files"`
	ValidationRoots   []string `yaml:"validation_roots"`
	ValidationTargets []string `yaml:"validation_targets"`
}

type CanonicalRequest struct {
	TaskType          string         `yaml:"task_type"`
	Goal              string         `yaml:"goal"`
	Context           RequestContext `yaml:"context"`
	Constraints       []string       `yaml:"constraints"`
	DefinitionOfDone  []string       `yaml:"definition_of_done"`
	Priority          string         `yaml:"priority"`
	RiskTolerance     string         `yaml:"risk_tolerance"`
	ValidationProfile string         `yaml:"validation_profile"`
}

type MaterializedRequest struct {
	ProjectRoot string
	Request     CanonicalRequest
}

func DecodeDraft(r io.Reader) (Draft, error) {
	decoder := yaml.NewDecoder(r)
	decoder.KnownFields(true)

	var draft Draft
	if err := decoder.Decode(&draft); err != nil {
		return Draft{}, fmt.Errorf("decode draft: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != nil && err != io.EOF {
		return Draft{}, fmt.Errorf("decode draft: %w", err)
	}
	if trailing != nil {
		return Draft{}, fmt.Errorf("decode draft: multiple draft documents are not allowed")
	}

	return draft, nil
}
