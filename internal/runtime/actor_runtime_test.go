package runtime

import "testing"

func TestActorOutputJSONSchemaPlanRequiresAssumptions(t *testing.T) {
	schema, err := actorOutputJSONSchema("plan")
	if err != nil {
		t.Fatalf("actorOutputJSONSchema returned error: %v", err)
	}

	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected properties map, got %T", schema["properties"])
	}
	if _, ok := properties["assumptions"]; !ok {
		t.Fatalf("expected plan schema to expose assumptions property")
	}

	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatalf("expected required array, got %T", schema["required"])
	}
	requiredSet := map[string]struct{}{}
	for _, name := range required {
		requiredSet[name] = struct{}{}
	}
	for name := range properties {
		if _, ok := requiredSet[name]; !ok {
			t.Fatalf("expected required list to include property %q", name)
		}
	}
}

func TestNormalizeActorResponseDropsNullNextActionForTerminalEvaluation(t *testing.T) {
	normalized := normalizeActorResponse("evaluation_result", map[string]any{
		"decision":           "pass",
		"next_action":        nil,
		"quality_confidence": "high",
	})

	if _, exists := normalized["next_action"]; exists {
		t.Fatalf("expected next_action to be removed for terminal evaluation decisions")
	}
}
