package worker

import (
	"log/slog"
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
		{"Felix", "", true, "Felix strikes and hits."},
		{"Felix", "", false, "Felix strikes and misses."},
	}
	for _, tt := range tests {
		if got := strikeLine(tt.attacker, tt.target, tt.hit); got != tt.want {
			t.Errorf("strikeLine(%q, %q, %v) = %q, want %q", tt.attacker, tt.target, tt.hit, got, tt.want)
		}
	}
}

func TestRollStrike_SeededHitAndMiss(t *testing.T) {
	probe := d20.NewRoller(1)
	wantHit := mustRoll(t, probe).Value >= combatDC
	p := &ChatProcessor{roller: d20.NewRoller(1), logger: slog.Default()}
	got := p.rollStrike("Felix", "Giant Rat")
	want := strikeLine("Felix", "Giant Rat", wantHit)
	if got != want {
		t.Errorf("rollStrike = %q, want %q", got, want)
	}
	if strings.Contains(got, "DC") || strings.Contains(got, "Rolled") {
		t.Errorf("narrator line must not include dice mechanics, got %q", got)
	}

	missSeed := seedForMiss(t)
	p = &ChatProcessor{roller: d20.NewRoller(missSeed), logger: slog.Default()}
	got = p.rollStrike("Felix", "Giant Rat")
	if !strings.HasSuffix(got, "misses.") {
		t.Errorf("expected a miss line, got %q", got)
	}
	hitSeed := seedForHit(t)
	p = &ChatProcessor{roller: d20.NewRoller(hitSeed), logger: slog.Default()}
	got = p.rollStrike("Felix", "Giant Rat")
	if !strings.HasSuffix(got, "hits.") {
		t.Errorf("expected a hit line, got %q", got)
	}
}

func TestCombatStrikeLines(t *testing.T) {
	gs := &state.GameState{PC: &character.PC{Name: "Felix"}}
	p := &ChatProcessor{roller: d20.NewRoller(1), logger: slog.Default()}

	t.Run("allowed combat with actor", func(t *testing.T) {
		ruling := &chat.Ruling{
			Allowed: true,
			Scope:   chat.RulingScopeCombat,
			Focus:   chat.RulingFocus{Actors: []string{"Giant Rat"}},
		}
		got := p.combatStrikeLines(gs, ruling)
		want := expectedStrikes(t, 1, "Felix", "Giant Rat")
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		lines := strings.Split(got, "\n")
		if len(lines) != 2 {
			t.Fatalf("want 2 lines, got %d: %q", len(lines), got)
		}
	})

	t.Run("allowed combat without actors", func(t *testing.T) {
		p := &ChatProcessor{roller: d20.NewRoller(1), logger: slog.Default()}
		ruling := &chat.Ruling{Allowed: true, Scope: chat.RulingScopeCombat}
		got := p.combatStrikeLines(gs, ruling)
		want := expectedStrikes(t, 1, "Felix", "")
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		if strings.Contains(got, "\n") {
			t.Errorf("want one line, got %q", got)
		}
	})

	t.Run("unnamed PC", func(t *testing.T) {
		p := &ChatProcessor{roller: d20.NewRoller(1), logger: slog.Default()}
		got := p.combatStrikeLines(&state.GameState{}, &chat.Ruling{Allowed: true, Scope: chat.RulingScopeCombat})
		if !strings.HasPrefix(got, "PC strikes") {
			t.Errorf("got %q, want PC prefix", got)
		}
	})

	for _, ruling := range []*chat.Ruling{
		nil,
		{Allowed: false, Scope: chat.RulingScopeCombat, Focus: chat.RulingFocus{Actors: []string{"Giant Rat"}}},
		{Allowed: true, Scope: chat.RulingScopeDialogue, Focus: chat.RulingFocus{Actors: []string{"Pip"}}},
		{Allowed: true, Scope: chat.RulingScopeMovement},
	} {
		if got := p.combatStrikeLines(gs, ruling); got != "" {
			t.Errorf("ruling %+v: got %q, want empty", ruling, got)
		}
	}
}

func expectedStrikes(t *testing.T, seed int64, pc, target string) string {
	t.Helper()
	r := d20.NewRoller(seed)
	pcHit := mustRoll(t, r).Value >= combatDC
	lines := []string{strikeLine(pc, target, pcHit)}
	if target != "" {
		foeHit := mustRoll(t, r).Value >= combatDC
		lines = append(lines, strikeLine(target, pc, foeHit))
	}
	return strings.Join(lines, "\n")
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
		if o.Value >= combatDC {
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
		if o.Value < combatDC {
			return seed
		}
	}
	t.Fatal("no miss seed")
	return 0
}
