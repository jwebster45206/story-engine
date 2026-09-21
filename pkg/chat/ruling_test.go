package chat

import (
	"encoding/json"
	"testing"
)

func TestRuling_Normalize_DropsReactionWhenAllowed(t *testing.T) {
	r := Ruling{
		Allowed:   true,
		Reasoning: "  the exit is listed  ",
		Reaction:  "  would be blocked  ",
		Subject:   "  Felix  ",
		Object:    "  Giant Rat  ",
	}
	r.Normalize()
	if r.Reasoning != "the exit is listed" {
		t.Errorf("Reasoning = %q", r.Reasoning)
	}
	if r.Reaction != "" {
		t.Errorf("Reaction = %q, want empty when allowed", r.Reaction)
	}
	if r.Subject != "Felix" || r.Object != "Giant Rat" {
		t.Errorf("Subject=%q Object=%q", r.Subject, r.Object)
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

func TestRuling_Normalize_Scope(t *testing.T) {
	r := Ruling{Scope: "  DIALOGUE  "}
	r.Normalize()
	if r.Scope != RulingScopeDialogue {
		t.Errorf("Scope = %q, want %q", r.Scope, RulingScopeDialogue)
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

func TestRuling_IsPCSubject(t *testing.T) {
	if !((*Ruling)(nil)).IsPCSubject("Felix") {
		t.Error("nil ruling should be PC subject")
	}
	if !(&Ruling{}).IsPCSubject("Felix") {
		t.Error("empty subject should be PC")
	}
	if !(&Ruling{Subject: "PC"}).IsPCSubject("Felix") {
		t.Error("PC should be PC subject")
	}
	if !(&Ruling{Subject: "Felix"}).IsPCSubject("Felix") {
		t.Error("PC name should be PC subject")
	}
	if (&Ruling{Subject: "Giant Rat"}).IsPCSubject("Felix") {
		t.Error("Giant Rat should not be PC subject")
	}
}

func TestRuling_ActorAndTargetNames(t *testing.T) {
	pc := &Ruling{Object: "Giant Rat"}
	if got := pc.ActorName("Felix"); got != "Felix" {
		t.Errorf("PC actor = %q, want Felix", got)
	}
	if got := pc.TargetName("Felix"); got != "Giant Rat" {
		t.Errorf("PC target = %q, want Giant Rat", got)
	}
	reaction := &Ruling{Subject: "Giant Rat", Object: "Felix"}
	if got := reaction.ActorName("Felix"); got != "Giant Rat" {
		t.Errorf("reaction actor = %q", got)
	}
	if got := reaction.TargetName("Felix"); got != "Felix" {
		t.Errorf("reaction target = %q", got)
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
				Object:    "Bartender",
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

func TestRuling_UnmarshalJSON_NullOptionals(t *testing.T) {
	var r Ruling
	if err := json.Unmarshal([]byte(`{"allowed":true,"reasoning":null,"reaction":null,"scope":"dialogue","subject":null,"object":null}`), &r); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !r.Allowed {
		t.Error("Allowed = false")
	}
	if r.Reasoning != "" || r.Reaction != "" || r.Subject != "" || r.Object != "" {
		t.Errorf("optional strings not empty: %+v", r)
	}
	if r.Scope != RulingScopeDialogue {
		t.Errorf("Scope = %q, want %q", r.Scope, RulingScopeDialogue)
	}
}

func TestRuling_FocusedNames(t *testing.T) {
	pc := &Ruling{Object: "Bartender"}
	if got := pc.FocusedNames("Felix"); len(got) != 1 || got[0] != "Bartender" {
		t.Errorf("PC focus = %#v", got)
	}
	reaction := &Ruling{Subject: "Giant Rat", Object: "Felix"}
	if got := reaction.FocusedNames("Felix"); len(got) != 1 || got[0] != "Giant Rat" {
		t.Errorf("reaction focus = %#v", got)
	}
}
