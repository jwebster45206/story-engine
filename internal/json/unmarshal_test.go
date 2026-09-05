package json

import (
	"strings"
	"testing"

	"github.com/jwebster45206/story-engine/pkg/conditionals"
)

func TestUnmarshal(t *testing.T) {
	t.Parallel()

	truncatedAcquire := `{
  "game_ended": false,
  "item_events": [
    {
      "action": "acquire",
      "consumed": false,
      "from": { "name": "black_pearl", "type": "location" },
      "item": "ship repair ledger"` + strings.Repeat("\n", 40)

	tests := []struct {
		name         string
		input        string
		wantErr      bool
		wantRepaired bool
		wantLocation string
		wantEnded    *bool
		wantItems    int
		wantItemName string
	}{
		{
			name:         "truncated item_events acquire with newline padding",
			input:        truncatedAcquire,
			wantRepaired: true,
			wantItems:    1,
			wantItemName: "ship repair ledger",
			wantEnded:    new(false),
		},
		{
			name:         "truncated mid-string value",
			input:        `{"user_location": "crash_bea`,
			wantRepaired: true,
			wantLocation: "crash_bea",
		},
		{
			name:         "trailing comma before EOF",
			input:        `{"user_location": "dock", "game_ended": false,`,
			wantRepaired: true,
			wantLocation: "dock",
			wantEnded:    new(false),
		},
		{
			name:         "truncated after key with no value",
			input:        `{"user_location": "dock", "set_vars":`,
			wantRepaired: true,
			wantLocation: "dock",
		},
		{
			name:         "already-valid JSON",
			input:        `{"user_location":"dock","game_ended":false,"item_events":[],"npc_events":[],"set_vars":{}}`,
			wantRepaired: false,
			wantLocation: "dock",
			wantEnded:    new(false),
		},
		{
			name:    "unrepairable garbage",
			input:   `not json at all`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var delta conditionals.GameStateDelta
			repaired, err := Unmarshal([]byte(tt.input), &delta)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got repaired=%v", repaired)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if repaired != tt.wantRepaired {
				t.Fatalf("repaired=%v, want %v", repaired, tt.wantRepaired)
			}
			if tt.wantLocation != "" && delta.UserLocation != tt.wantLocation {
				t.Errorf("user_location=%q, want %q", delta.UserLocation, tt.wantLocation)
			}
			if tt.wantEnded != nil {
				if delta.GameEnded == nil || *delta.GameEnded != *tt.wantEnded {
					t.Errorf("game_ended=%v, want %v", delta.GameEnded, *tt.wantEnded)
				}
			}
			if tt.wantItems > 0 {
				if len(delta.ItemEvents) != tt.wantItems {
					t.Fatalf("item_events len=%d, want %d", len(delta.ItemEvents), tt.wantItems)
				}
				if tt.wantItemName != "" && delta.ItemEvents[0].Item != tt.wantItemName {
					t.Errorf("item=%q, want %q", delta.ItemEvents[0].Item, tt.wantItemName)
				}
				if delta.ItemEvents[0].Action != "acquire" {
					t.Errorf("action=%q, want acquire", delta.ItemEvents[0].Action)
				}
			}
		})
	}
}

func TestRepairTruncated(t *testing.T) {
	t.Parallel()

	got := repairTruncated(`{"a": true,`)
	if got != `{"a": true}` {
		t.Fatalf("got %q", got)
	}

	got = repairTruncated(`{"loc": "ab`)
	if got != `{"loc": "ab"}` {
		t.Fatalf("got %q", got)
	}
}
