package chat

import "testing"

func TestRuling_Normalize_DropsReactionWhenAllowed(t *testing.T) {
	r := Ruling{
		Allowed:   true,
		Reasoning: "  the exit is listed  ",
		Reaction:  "  would be blocked  ",
	}
	r.Normalize()
	if r.Reasoning != "the exit is listed" {
		t.Errorf("Reasoning = %q", r.Reasoning)
	}
	if r.Reaction != "" {
		t.Errorf("Reaction = %q, want empty when allowed", r.Reaction)
	}
}

func TestRuling_Normalize_KeepsReactionWhenDisallowed(t *testing.T) {
	r := Ruling{
		Allowed:  false,
		Reaction: "  The guard bars the way.  ",
	}
	r.Normalize()
	if r.Reaction != "The guard bars the way." {
		t.Errorf("Reaction = %q", r.Reaction)
	}
}

func TestRuling_Normalize_ScopeAndLists(t *testing.T) {
	r := Ruling{
		Scope: "  DIALOGUE  ",
		Focus: RulingFocus{
			NPCs:      []string{"  Guard  ", "", "Guard", "Bartender"},
			Locations: []string{"  Cave  ", "", "Cave"},
		},
	}
	r.Normalize()
	if r.Scope != RulingScopeDialogue {
		t.Errorf("Scope = %q, want %q", r.Scope, RulingScopeDialogue)
	}
	if len(r.Focus.NPCs) != 2 || r.Focus.NPCs[0] != "Guard" || r.Focus.NPCs[1] != "Bartender" {
		t.Errorf("Focus.NPCs = %#v", r.Focus.NPCs)
	}
	if len(r.Focus.Locations) != 1 || r.Focus.Locations[0] != "Cave" {
		t.Errorf("Focus.Locations = %#v", r.Focus.Locations)
	}

	unknown := Ruling{Scope: "teleport"}
	unknown.Normalize()
	if unknown.Scope != RulingScopeOther {
		t.Errorf("unknown scope = %q, want %q", unknown.Scope, RulingScopeOther)
	}

	empty := Ruling{}
	empty.Normalize()
	if empty.Scope != RulingScopeOther {
		t.Errorf("empty scope = %q, want %q", empty.Scope, RulingScopeOther)
	}
}

func TestRuling_NarratorText(t *testing.T) {
	tests := []struct {
		name string
		r    *Ruling
		want string
	}{
		{name: "nil", want: ""},
		{name: "allowed only", r: &Ruling{Allowed: true}, want: "Allowed."},
		{
			name: "allowed with reasoning",
			r:    &Ruling{Allowed: true, Reasoning: "The PC can move north."},
			want: "Allowed. The PC can move north.",
		},
		{
			name: "not allowed with reasoning",
			r:    &Ruling{Allowed: false, Reasoning: "There is no north exit."},
			want: "Not allowed. There is no north exit.",
		},
		{
			name: "not allowed with reaction",
			r:    &Ruling{Allowed: false, Reasoning: "The exit is blocked.", Reaction: "The guard bars the way."},
			want: "Not allowed. The exit is blocked.\nThe guard bars the way.",
		},
		{
			name: "scope fields do not leak into narrator text",
			r: &Ruling{
				Allowed:   true,
				Reasoning: "The PC greets the bartender.",
				Scope:     RulingScopeDialogue,
				Focus:     RulingFocus{NPCs: []string{"Bartender"}, Locations: []string{"tavern"}},
			},
			want: "Allowed. The PC greets the bartender.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.NarratorText(); got != tt.want {
				t.Errorf("NarratorText() = %q, want %q", got, tt.want)
			}
		})
	}
}
