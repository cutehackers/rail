package runtime

import (
	"fmt"
	"sort"
	"strings"

	"rail/internal/assets"

	"gopkg.in/yaml.v3"
)

type ActorProfile struct {
	Model     string `yaml:"model"`
	Reasoning string `yaml:"reasoning"`
}

type ActorProfiles struct {
	Version int                     `yaml:"version"`
	Actors  map[string]ActorProfile `yaml:"actors"`
}

var supportedActorReasoningEfforts = map[string]struct{}{
	"none":     {},
	"minimal":  {},
	"low":      {},
	"medium":   {},
	"high":     {},
	"xhigh":    {},
}

func loadActorProfiles(projectRoot string, requiredActors []string) (ActorProfiles, error) {
	data, source, err := assets.Resolve(projectRoot, ".harness/supervisor/actor_profiles.yaml")
	if err != nil {
		return ActorProfiles{}, fmt.Errorf("resolve actor profiles: %w", err)
	}

	var profiles ActorProfiles
	if err := yaml.Unmarshal(data, &profiles); err != nil {
		return ActorProfiles{}, fmt.Errorf("decode actor profiles from %s policy: %w", source, err)
	}
	if profiles.Version != 1 {
		return ActorProfiles{}, fmt.Errorf("actor profiles version must be 1, got %d", profiles.Version)
	}
	if profiles.Actors == nil {
		return ActorProfiles{}, fmt.Errorf("actor profiles must define actors")
	}

	requiredActorSet := map[string]struct{}{}
	missingActors := make([]string, 0, len(requiredActors))
	for _, actorName := range requiredActors {
		actorName = strings.TrimSpace(actorName)
		if actorName == "" {
			continue
		}
		if _, seen := requiredActorSet[actorName]; seen {
			continue
		}
		requiredActorSet[actorName] = struct{}{}
		profile, ok := profiles.Actors[actorName]
		if !ok {
			missingActors = append(missingActors, actorName)
			continue
		}
		profile.Model = strings.TrimSpace(profile.Model)
		profile.Reasoning = strings.TrimSpace(profile.Reasoning)
		if profile.Model == "" {
			return ActorProfiles{}, fmt.Errorf("actor profile %q must define model", actorName)
		}
		if _, ok := supportedActorReasoningEfforts[profile.Reasoning]; !ok {
			return ActorProfiles{}, fmt.Errorf("actor profile %q has unsupported reasoning %q", actorName, profile.Reasoning)
		}

		profiles.Actors[actorName] = profile
	}
	if len(missingActors) > 0 {
		sort.Strings(missingActors)
		return ActorProfiles{}, fmt.Errorf("actor profiles missing required actors: %s", strings.Join(missingActors, ", "))
	}

	return profiles, nil
}

func (p ActorProfiles) ProfileFor(actorName string) (ActorProfile, error) {
	profile, ok := p.Actors[actorName]
	if !ok {
		return ActorProfile{}, fmt.Errorf("actor profile %q is not defined", actorName)
	}
	return profile, nil
}
