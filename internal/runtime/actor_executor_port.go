package runtime

import "context"

type ActorExecutor interface {
	RunActor(context.Context, ActorInvocation) (ActorResult, error)
}

type ActorInvocation struct {
	ActorName         string
	ActorRunID        string
	WorkingDirectory  string
	ArtifactDirectory string
	Prompt            string
	OutputSchemaPath  string
	LastMessagePath   string
	EventsPath        string
	Profile           ActorProfile
	Policy            ActorBackendConfig
}

type ActorResult struct {
	StructuredOutput    map[string]any
	LastMessagePath     string
	EventsPath          string
	ProvenancePath      string
	RuntimeEvidencePath string
}
