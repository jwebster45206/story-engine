package worker

import (
	"log/slog"
	"slices"
	"strings"
	"testing"

	"github.com/jwebster45206/d20"
	"github.com/jwebster45206/story-engine/pkg/character"
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/state"
)

func TestStrikeLine(t *testing.T) {
	tests := []struct {
		attacker, target string
		hit              bool
		want             string
	}{
		{"Felix", "Giant Rat", true, "Felix strikes Giant Rat and hits."},
		{"Felix", "Giant Rat", false, "Felix strikes Giant Rat and misses."},
		{"Felix", "", true, "Felix strikes  and hits."},
		{"Felix", "", false, "Felix strikes  and misses."},
	}
	for _, tt := range tests {
		if got := strikeLine(tt.attacker, tt.target, tt.hit); got != tt.want {
			t.Errorf("strikeLine(%q, %q, %v) = %q, want %q", tt.attacker, tt.target, tt.hit, got, tt.want)
		}
	}
}

func TestAttempt_SeededHitAndMiss(t *testing.T) {
	probe := d20.NewRoller(1)
	wantHit := mustRoll(t, probe).Value >= defaultDC
	p := &ChatOrchestrator{roller: d20.NewRoller(1), logger: slog.Default()}
	got := p.attempt("Felix", "Giant Rat")
	want := strikeLine("Felix", "Giant Rat", wantHit)
	if got.NarratorText != want {
		t.Errorf("attempt = %q, want %q", got.NarratorText, want)
	}
	if strings.Contains(got.NarratorText, "DC") || strings.Contains(strings.ToLower(got.NarratorText), "rolled") {
		t.Errorf("narrator line must not include dice mechanics, got %q", got.NarratorText)
	}
	if !strings.HasPrefix(got.Content, "Felix rolled ") {
		t.Errorf("content should name the actor, got %q", got.Content)
	}

	if got.Success != wantHit {
		t.Errorf("Success = %v, want %v", got.Success, wantHit)
	}

	missSeed := seedForMiss(t)
	p = &ChatOrchestrator{roller: d20.NewRoller(missSeed), logger: slog.Default()}
	got = p.attempt("Felix", "Giant Rat")
	if !strings.HasSuffix(got.NarratorText, "misses.") {
		t.Errorf("expected a miss line, got %q", got.NarratorText)
	}
	if got.Success {
		t.Error("expected Success=false on miss")
	}
	hitSeed := seedForHit(t)
	p = &ChatOrchestrator{roller: d20.NewRoller(hitSeed), logger: slog.Default()}
	got = p.attempt("Felix", "Giant Rat")
	if !strings.HasSuffix(got.NarratorText, "hits.") {
		t.Errorf("expected a hit line, got %q", got.NarratorText)
	}
	if !got.Success {
		t.Error("expected Success=true on hit")
	}
}

func TestResolveAttempts(t *testing.T) {
	gs := &state.GameState{PC: &character.PC{Name: "Felix"}}
	p := &ChatOrchestrator{roller: d20.NewRoller(1), logger: slog.Default()}

	t.Run("allowed combat with actor", func(t *testing.T) {
		ruling := &chat.Ruling{
			Allowed: true,
			Scope:   chat.RulingScopeCombat,
			Focus:   chat.RulingFocus{Actors: []string{"Giant Rat"}},
		}
		got := p.resolveAttempts(gs, ruling)
		want := expectedStrikes(t, 1, "Felix", "Giant Rat")
		if !slices.Equal(narratorTexts(got), want) {
			t.Errorf("got %q, want %q", narratorTexts(got), want)
		}
		if len(got) != 1 {
			t.Fatalf("want 1 line, got %d: %q", len(got), narratorTexts(got))
		}
		if got[0].Content == "" {
			t.Fatal("expected roll content")
		}
		if got[0].Scope != chat.RulingScopeCombat {
			t.Errorf("Scope = %q, want %q", got[0].Scope, chat.RulingScopeCombat)
		}
	})

	t.Run("allowed combat without actors", func(t *testing.T) {
		p := &ChatOrchestrator{roller: d20.NewRoller(1), logger: slog.Default()}
		ruling := &chat.Ruling{Allowed: true, Scope: chat.RulingScopeCombat}
		got := p.resolveAttempts(gs, ruling)
		want := expectedStrikes(t, 1, "Felix", "")
		if !slices.Equal(narratorTexts(got), want) {
			t.Errorf("got %q, want %q", narratorTexts(got), want)
		}
		if len(got) != 1 {
			t.Errorf("want one line, got %q", narratorTexts(got))
		}
	})

	t.Run("unnamed PC", func(t *testing.T) {
		p := &ChatOrchestrator{roller: d20.NewRoller(1), logger: slog.Default()}
		got := p.resolveAttempts(&state.GameState{}, &chat.Ruling{Allowed: true, Scope: chat.RulingScopeCombat})
		if len(got) != 1 || !strings.HasPrefix(got[0].NarratorText, "PC strikes") {
			t.Errorf("got %q, want PC prefix", narratorTexts(got))
		}
	})

	for _, ruling := range []*chat.Ruling{
		nil,
		{Allowed: false, Scope: chat.RulingScopeCombat, Focus: chat.RulingFocus{Actors: []string{"Giant Rat"}}},
		{Allowed: true, Scope: chat.RulingScopeDialogue, Focus: chat.RulingFocus{Actors: []string{"Pip"}}},
		{Allowed: true, Scope: chat.RulingScopeMovement},
	} {
		if got := p.resolveAttempts(gs, ruling); len(got) != 0 {
			t.Errorf("ruling %+v: got %q, want empty", ruling, narratorTexts(got))
		}
	}
}

func expectedStrikes(t *testing.T, seed int64, pc, target string) []string {
	t.Helper()
	r := d20.NewRoller(seed)
	pcHit := mustRoll(t, r).Value >= defaultDC
	return []string{strikeLine(pc, target, pcHit)}
}

func mustRoll(t *testing.T, r *d20.Roller) d20.RollOutcome {
	t.Helper()
	o, err := r.Roll(d20Die)
	if err != nil {
		t.Fatalf("Roll: %v", err)
	}
	return o
}

func seedForHit(t *testing.T) int64 {
	t.Helper()
	for seed := int64(1); seed < 1000; seed++ {
		o, err := d20.NewRoller(seed).Roll(d20Die)
		if err != nil {
			t.Fatalf("Roll: %v", err)
		}
		if o.Value >= defaultDC {
			return seed
		}
	}
	t.Fatal("no hit seed")
	return 0
}

func seedForMiss(t *testing.T) int64 {
	t.Helper()
	for seed := int64(1); seed < 1000; seed++ {
		o, err := d20.NewRoller(seed).Roll(d20Die)
		if err != nil {
			t.Fatalf("Roll: %v", err)
		}
		if o.Value < defaultDC {
			return seed
		}
	}
	t.Fatal("no miss seed")
	return 0
}

func TestRollContent(t *testing.T) {
	tests := []struct {
		actor, detail, want string
	}{
		{"Jack", "Rolled 1d20... 12; *Result: 12*", "Jack rolled 1d20... 12; *Result: 12*"},
		{"Jack", "rolled 1d20... 12; *Result: 12*", "Jack rolled 1d20... 12; *Result: 12*"},
	}
	for _, tt := range tests {
		if got := rollContent(tt.actor, tt.detail); got != tt.want {
			t.Errorf("rollContent(%q, %q) = %q, want %q", tt.actor, tt.detail, got, tt.want)
		}
	}
}
