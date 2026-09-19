package llm

import (
	"slices"
	"testing"
)

func TestRulingSchema_IncludesScopeFields(t *testing.T) {
	schema := rulingSchema()
	props, _ := schema["properties"].(map[string]any)
	if props["scope"] == nil || props["focus"] == nil {
		t.Fatalf("missing scope fields in properties: %v", keys(props))
	}
	if props["mentioned"] != nil {
		t.Error("mentioned should be omitted from ruling schema")
	}

	scope, _ := props["scope"].(map[string]any)
	enum, _ := scope["enum"].([]string)
	wantEnum := []string{"dialogue", "movement", "combat", "examine", "ambient", "other"}
	if !slices.Equal(enum, wantEnum) {
		t.Errorf("scope enum = %v, want %v", enum, wantEnum)
	}

	focus, _ := props["focus"].(map[string]any)
	focusProps, _ := focus["properties"].(map[string]any)
	if focusProps["npcs"] == nil || focusProps["locations"] == nil {
		t.Errorf("focus properties = %v", keys(focusProps))
	}
	if focusProps["items"] != nil {
		t.Error("focus.items should be omitted")
	}
	focusReq, _ := focus["required"].([]string)
	if !slices.Contains(focusReq, "npcs") || !slices.Contains(focusReq, "locations") {
		t.Errorf("focus required = %v", focusReq)
	}

	required, _ := schema["required"].([]string)
	for _, field := range []string{"allowed", "reasoning", "reaction", "scope", "focus"} {
		if !slices.Contains(required, field) {
			t.Errorf("required missing %q: %v", field, required)
		}
	}
	if slices.Contains(required, "mentioned") {
		t.Error("mentioned should not be required")
	}
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
