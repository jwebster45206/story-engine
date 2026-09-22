package main

import (
	"strings"
	"testing"

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
