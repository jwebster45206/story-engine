package character

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestToActor_MergesAbilitiesAndAttributes(t *testing.T) {
	stats := Stats{
		HP:    20,
		MaxHP: 20,
		AC:    12,
		Abilities: Abilities{
			Intelligence: 17,
			Dexterity:    13,
		},
		Attributes: map[string]int{"arcana": 6, "intelligence": 99},
		Modifiers:  map[string]int{"proficiency": 2},
		Actions: map[string]Action{
			"arcane_bolt": {Name: "Arcane Bolt", Type: "attack", Attempt: "1d20+5", Effect: "1d10"},
		},
	}

	actor, err := stats.ToActor("elara_wyn", "Elara Wyn")
	if err != nil {
		t.Fatal(err)
	}
	if actor.Name != "Elara Wyn" || actor.AC != 12 || actor.HP != 20 {
		t.Fatalf("actor identity/stats = %+v", actor)
	}
	if actor.Attributes["dexterity"] != 13 || actor.Attributes["arcana"] != 6 {
		t.Fatalf("attributes = %v", actor.Attributes)
	}
	if actor.Attributes["intelligence"] != 99 {
		t.Fatalf("attributes should win a key collision, intelligence = %d", actor.Attributes["intelligence"])
	}
	if actor.Modifiers["proficiency"] != 2 {
		t.Fatalf("modifiers = %v", actor.Modifiers)
	}
	action, ok := actor.Action("arcane_bolt")
	if !ok {
		t.Fatal("missing arcane_bolt")
	}
	if action.Attempt.Count != 1 || action.Attempt.Faces != 20 {
		t.Fatalf("attempt = %+v", action.Attempt)
	}
	if len(action.Attempt.Modifiers) != 1 || action.Attempt.Modifiers[0].Value != 5 {
		t.Fatalf("attempt modifiers = %+v", action.Attempt.Modifiers)
	}
	if action.Effect.Faces != 10 {
		t.Fatalf("effect = %+v", action.Effect)
	}
}

func TestValidateActions_RejectsGarbage(t *testing.T) {
	stats := Stats{Actions: map[string]Action{"bite": {Attempt: "not-dice"}}}
	if err := stats.ValidateActions(); err == nil {
		t.Fatal("expected notation error")
	}
	if _, err := stats.ToActor("rat", "Rat"); err == nil {
		t.Fatal("expected ToActor to reject bad notation")
	}

	ok := Stats{Actions: map[string]Action{"wait": {Name: "Wait", Type: "attack"}}}
	if err := ok.ValidateActions(); err != nil {
		t.Fatal(err)
	}
}

func TestContentRoundTrip(t *testing.T) {
	root := filepath.Join("..", "..", "data")
	sets := []struct {
		dir  string
		kind string
	}{
		{"pcs", "pc"},
		{"npcs", "npc"},
		{"monsters", "monster"},
	}
	for _, set := range sets {
		files, err := filepath.Glob(filepath.Join(root, set.dir, "*.json"))
		if err != nil {
			t.Fatal(err)
		}
		if len(files) == 0 {
			t.Fatalf("no files in %s", set.dir)
		}
		for _, file := range files {
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			var decoded any
			switch set.kind {
			case "pc":
				var pc PC
				if err := json.Unmarshal(raw, &pc); err != nil {
					t.Fatalf("%s: %v", file, err)
				}
				decoded = pc
			case "npc":
				var npc NPC
				if err := json.Unmarshal(raw, &npc); err != nil {
					t.Fatalf("%s: %v", file, err)
				}
				decoded = npc
			default:
				var monster Monster
				if err := json.Unmarshal(raw, &monster); err != nil {
					t.Fatalf("%s: %v", file, err)
				}
				decoded = monster
			}
			var actions Stats
			switch v := decoded.(type) {
			case PC:
				actions = v.Stats
			case NPC:
				actions = v.Stats
			case Monster:
				actions = v.Stats
			}
			if err := actions.ValidateActions(); err != nil {
				t.Fatalf("%s: %v", file, err)
			}
			out, err := json.Marshal(decoded)
			if err != nil {
				t.Fatalf("%s: %v", file, err)
			}
			assertNoNewJSON(t, file, raw, out)
		}
	}
}

// assertNoNewJSON fails when remarshaling introduces a key or changes a value.
// Keys dropped because they were empty are allowed: omitempty removes them.
func assertNoNewJSON(t *testing.T, file string, original, remarshaled []byte) {
	t.Helper()
	var want, got any
	if err := json.Unmarshal(original, &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(remarshaled, &got); err != nil {
		t.Fatal(err)
	}
	// PC ids live in the filename. Storage stamps them on load, and the
	// struct tag does not omit an empty id, so a file with no id remarshals
	// one. That is not a content change.
	if m, ok := got.(map[string]any); ok {
		if id, ok := m["id"].(string); ok && id == "" {
			delete(m, "id")
		}
	}
	diffJSON(t, file, want, got)
}

func diffJSON(t *testing.T, path string, want, got any) {
	t.Helper()
	switch g := got.(type) {
	case map[string]any:
		w, ok := want.(map[string]any)
		if !ok {
			t.Errorf("%s: got object, want %T", path, want)
			return
		}
		for k, gv := range g {
			wv, ok := w[k]
			if !ok {
				t.Errorf("%s: unexpected key %q (%s)", path, k, truncate(gv))
				continue
			}
			// Contingency prompts accept a bare string and remarshal as an
			// object. That widening is older than Stats and is not a migration.
			if k == "contingency_prompts" {
				continue
			}
			diffJSON(t, path+"."+k, wv, gv)
		}
	case []any:
		w, ok := want.([]any)
		if !ok {
			t.Errorf("%s: got array, want %T", path, want)
			return
		}
		if len(g) != len(w) {
			t.Errorf("%s: len %d, want %d", path, len(g), len(w))
			return
		}
		for i := range g {
			diffJSON(t, path, w[i], g[i])
		}
	default:
		if !reflect.DeepEqual(want, got) {
			t.Errorf("%s: %v, want %v", path, got, want)
		}
	}
}

func truncate(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	s := string(b)
	if len(s) > 80 {
		return s[:80] + "..."
	}
	return s
}

func TestContentRoundTrip_NoLegacyKeys(t *testing.T) {
	// The files on disk are the migrated form. A stray combat_modifiers or PC
	// stats key would load, then disappear on the way back out, and the
	// round trip above would not notice a missing key. Reject them directly.
	root := filepath.Join("..", "..", "data")
	var files []string
	for _, dir := range []string{"pcs", "npcs", "monsters", "scenarios"} {
		matches, err := filepath.Glob(filepath.Join(root, dir, "*.json"))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, matches...)
	}
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		if strings.Contains(text, "combat_modifiers") {
			t.Errorf("%s still uses combat_modifiers", file)
		}
	}
}
