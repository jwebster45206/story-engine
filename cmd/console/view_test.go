package main

import (
	"strings"
	"testing"

	"github.com/jwebster45206/story-engine/pkg/character"
	"github.com/jwebster45206/story-engine/pkg/state"
)

func TestFormatNarratorResponse_PlainText(t *testing.T) {
	got := formatNarratorResponse("The door creaks open.", 80)
	if !strings.Contains(got, "The door creaks open.") {
		t.Fatalf("expected original text, got %q", got)
	}
}

func TestFormatNarratorResponse_KeepsSpeakerPrefix(t *testing.T) {
	got := formatNarratorResponse("Guard: Halt!", 80)
	if strings.Contains(got, "Narrator:") {
		t.Fatalf("should not add Narrator when speaker prefix exists, got %q", got)
	}
	if !strings.Contains(got, "Guard:") {
		t.Fatalf("expected Guard speaker, got %q", got)
	}
	if !strings.Contains(got, "Halt!") {
		t.Fatalf("expected rest of line, got %q", got)
	}
}

func TestFormatNarratorResponse_WrapsLongLine(t *testing.T) {
	long := strings.Repeat("word ", 40)
	got := formatNarratorResponse(long, 40)
	if !strings.Contains(got, "\n") {
		t.Fatalf("expected wrapped lines, got single line %q", got)
	}
}

func TestWriteSidebar_Processing(t *testing.T) {
	gs := &state.GameState{Location: "dock"}
	if strings.Contains(writeSidebar(gs, "Test", false), "Processing...") {
		t.Fatal("idle sidebar should not say Processing")
	}
	got := writeSidebar(gs, "Test", true)
	if !strings.Contains(got, "Processing...") {
		t.Fatalf("expected Processing..., got %q", got)
	}
	if strings.Contains(got, "Syncing game state") {
		t.Fatal("old syncing copy should be gone")
	}
}

func TestWriteSidebar_PCAndActions(t *testing.T) {
	gs := &state.GameState{
		Location: "dock",
		PC: &character.PC{
			Name:  "Felix",
			Level: 5,
			Class: "Fighter",
			HP:    10,
			MaxHP: 10,
			AC:    14,
			Actions: map[string]character.Action{
				"cutlass": {Name: "Cutlass"},
				"bite":    {Name: "Bite"},
			},
		},
		Inventory: []string{"torch"},
	}
	got := writeSidebar(gs, "Test", false)
	if !strings.Contains(got, "Felix") || !strings.Contains(got, "Level 5 Fighter") || !strings.Contains(got, "HP 10/10") || !strings.Contains(got, "AC 14") {
		t.Fatalf("sidebar missing PC stats: %q", got)
	}
	actions := strings.Index(got, "Actions")
	inventory := strings.Index(got, "Inventory:")
	bite := strings.Index(got, "Bite")
	cutlass := strings.Index(got, "Cutlass")
	if actions < 0 || bite < actions || cutlass < bite || inventory < cutlass {
		t.Fatalf("want Actions, then Bite, Cutlass, then Inventory, got %q", got)
	}
	if strings.Contains(got, "Commands:") || strings.Contains(got, "Ctrl+C") {
		t.Fatalf("commands section should be hidden, got %q", got)
	}
}
