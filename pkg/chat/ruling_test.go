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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.NarratorText(); got != tt.want {
				t.Errorf("NarratorText() = %q, want %q", got, tt.want)
			}
		})
	}
}
