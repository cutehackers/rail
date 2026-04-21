package runtime

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadActorProfiles(t *testing.T) {
	t.Run("loads valid checked-in actor profiles", func(t *testing.T) {
		profiles, err := loadActorProfiles(testRepoRoot(t), []string{"planner", "context_builder", "critic", "generator", "evaluator", "integrator"})
		if err != nil {
			t.Fatalf("loadActorProfiles returned error: %v", err)
		}

		if got, want := profiles.Version, 1; got != want {
			t.Fatalf("unexpected version: got %d want %d", got, want)
		}

		testCases := map[string]ActorProfile{
			"planner":         {Model: "gpt-5.4", Reasoning: "high"},
			"context_builder": {Model: "gpt-5.4-mini", Reasoning: "high"},
			"critic":          {Model: "gpt-5.4", Reasoning: "high"},
			"generator":       {Model: "gpt-5.4", Reasoning: "high"},
			"evaluator":       {Model: "gpt-5.4", Reasoning: "high"},
			"integrator":      {Model: "gpt-5.4", Reasoning: "high"},
		}
		for actorName, want := range testCases {
			got, err := profiles.ProfileFor(actorName)
			if err != nil {
				t.Fatalf("ProfileFor(%q) returned error: %v", actorName, err)
			}
			if got != want {
				t.Fatalf("unexpected profile for %q: got %+v want %+v", actorName, got, want)
			}
		}
	})

	t.Run("checked-in and embedded defaults stay in parity", func(t *testing.T) {
		requiredActors := []string{"planner", "context_builder", "critic", "generator", "evaluator", "integrator"}
		repoProfiles, err := loadActorProfiles(testRepoRoot(t), requiredActors)
		if err != nil {
			t.Fatalf("loadActorProfiles(repo) returned error: %v", err)
		}
		embeddedProfiles, err := loadActorProfiles(t.TempDir(), requiredActors)
		if err != nil {
			t.Fatalf("loadActorProfiles(embedded) returned error: %v", err)
		}
		if !reflect.DeepEqual(repoProfiles, embeddedProfiles) {
			t.Fatalf("expected checked-in and embedded actor profiles to match: repo=%+v embedded=%+v", repoProfiles, embeddedProfiles)
		}
	})

	t.Run("rejects missing required actor entries", func(t *testing.T) {
		projectRoot := writeActorProfilesFixture(t, `
version: 1
actors:
  planner: { model: gpt-5.4, reasoning: high }
  context_builder: { model: gpt-5.4-mini, reasoning: high }
  generator: { model: gpt-5.4, reasoning: high }
  evaluator: { model: gpt-5.4, reasoning: high }
`)

		_, err := loadActorProfiles(projectRoot, []string{"planner", "context_builder", "critic", "generator", "evaluator"})
		if err == nil {
			t.Fatalf("expected loadActorProfiles to reject missing critic profile")
		}
		if !strings.Contains(err.Error(), "critic") {
			t.Fatalf("expected missing actor error to mention critic, got %v", err)
		}
	})

	t.Run("rejects unsupported reasoning values", func(t *testing.T) {
		projectRoot := writeActorProfilesFixture(t, `
version: 1
actors:
  planner: { model: gpt-5.4, reasoning: high }
  context_builder: { model: gpt-5.4-mini, reasoning: medium }
  critic: { model: gpt-5.4, reasoning: critical }
  generator: { model: gpt-5.4, reasoning: high }
  evaluator: { model: gpt-5.4, reasoning: high }
`)

		_, err := loadActorProfiles(projectRoot, []string{"planner", "context_builder", "critic", "generator", "evaluator"})
		if err == nil {
			t.Fatalf("expected loadActorProfiles to reject unsupported reasoning")
		}
		if !strings.Contains(err.Error(), "unsupported reasoning") {
			t.Fatalf("expected unsupported reasoning error, got %v", err)
		}
	})

	t.Run("accepts documented reasoning values", func(t *testing.T) {
		projectRoot := writeActorProfilesFixture(t, `
version: 1
actors:
  planner: { model: gpt-5.4, reasoning: none }
  context_builder: { model: gpt-5, reasoning: minimal }
  critic: { model: gpt-5.3-codex, reasoning: xhigh }
  generator: { model: gpt-5.4, reasoning: medium }
  evaluator: { model: gpt-5, reasoning: low }
`)

		profiles, err := loadActorProfiles(projectRoot, []string{"planner", "context_builder", "critic", "generator", "evaluator"})
		if err != nil {
			t.Fatalf("loadActorProfiles returned error: %v", err)
		}

		for actorName, wantReasoning := range map[string]string{
			"planner":         "none",
			"context_builder": "minimal",
			"critic":          "xhigh",
			"generator":       "medium",
			"evaluator":       "low",
		} {
			profile, err := profiles.ProfileFor(actorName)
			if err != nil {
				t.Fatalf("ProfileFor(%q) returned error: %v", actorName, err)
			}
			if profile.Reasoning != wantReasoning {
				t.Fatalf("unexpected reasoning for %q: got %q want %q", actorName, profile.Reasoning, wantReasoning)
			}
		}
	})
}

func TestLoadActorProfilesRejectsMissingWorkflowActor(t *testing.T) {
	projectRoot := writeActorProfilesFixture(t, `
version: 1
actors:
  planner: { model: gpt-5.4, reasoning: high }
  context_builder: { model: gpt-5.4-mini, reasoning: high }
  critic: { model: gpt-5.4, reasoning: high }
  generator: { model: gpt-5.4, reasoning: high }
  evaluator: { model: gpt-5.4, reasoning: high }
`)

	_, err := loadActorProfiles(projectRoot, []string{"planner", "integrator"})
	if err == nil {
		t.Fatalf("expected loadActorProfiles to reject missing workflow actor")
	}
	if !strings.Contains(err.Error(), "integrator") {
		t.Fatalf("expected missing workflow actor error to mention integrator, got %v", err)
	}
}

func TestLoadActorProfilesRejectsUnsupportedVersion(t *testing.T) {
	projectRoot := writeActorProfilesFixture(t, `
version: 2
actors:
  planner: { model: gpt-5.4, reasoning: high }
`)

	_, err := loadActorProfiles(projectRoot, []string{"planner"})
	if err == nil {
		t.Fatalf("expected loadActorProfiles to reject unsupported version")
	}
	if !strings.Contains(err.Error(), "version must be 1") {
		t.Fatalf("expected unsupported version error, got %v", err)
	}
}

func TestLoadActorProfilesUsesEmbeddedDefaults(t *testing.T) {
	requiredActors := []string{"planner", "context_builder", "critic", "generator", "evaluator", "integrator"}
	profiles, err := loadActorProfiles(t.TempDir(), requiredActors)
	if err != nil {
		t.Fatalf("loadActorProfiles returned error: %v", err)
	}

	testCases := map[string]ActorProfile{
		"planner":         {Model: "gpt-5.4", Reasoning: "high"},
		"context_builder": {Model: "gpt-5.4-mini", Reasoning: "high"},
		"critic":          {Model: "gpt-5.4", Reasoning: "high"},
		"generator":       {Model: "gpt-5.4", Reasoning: "high"},
		"evaluator":       {Model: "gpt-5.4", Reasoning: "high"},
		"integrator":      {Model: "gpt-5.4", Reasoning: "high"},
	}
	for actorName, want := range testCases {
		got, err := profiles.ProfileFor(actorName)
		if err != nil {
			t.Fatalf("ProfileFor(%q) returned error: %v", actorName, err)
		}
		if got != want {
			t.Fatalf("unexpected embedded profile for %q: got %+v want %+v", actorName, got, want)
		}
	}
}

func TestLoadActorProfilesIgnoresUnusedInvalidActor(t *testing.T) {
	projectRoot := writeActorProfilesFixture(t, `
version: 1
actors:
  planner: { model: gpt-5.4, reasoning: high }
  context_builder: { model: gpt-5.4-mini, reasoning: medium }
  critic: { model: gpt-5.4, reasoning: xhigh }
  generator: { model: gpt-5.4, reasoning: low }
  evaluator: { model: gpt-5.4, reasoning: none }
  integrator: { model: gpt-5.4, reasoning: critical }
`)

	profiles, err := loadActorProfiles(projectRoot, []string{"planner", "context_builder", "critic", "generator", "evaluator"})
	if err != nil {
		t.Fatalf("expected unused invalid actor profile to be ignored, got %v", err)
	}
	if _, err := profiles.ProfileFor("planner"); err != nil {
		t.Fatalf("expected required actor profile to remain available, got %v", err)
	}
}

func writeActorProfilesFixture(t *testing.T, body string) string {
	t.Helper()

	projectRoot := t.TempDir()
	profilesPath := filepath.Join(projectRoot, ".harness", "supervisor", "actor_profiles.yaml")
	if err := os.MkdirAll(filepath.Dir(profilesPath), 0o755); err != nil {
		t.Fatalf("failed to create actor profiles fixture directory: %v", err)
	}
	if err := os.WriteFile(profilesPath, []byte(strings.TrimSpace(body)+"\n"), 0o644); err != nil {
		t.Fatalf("failed to write actor profiles fixture: %v", err)
	}
	return projectRoot
}
