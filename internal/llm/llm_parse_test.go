package llm

import (
	"reflect"
	"strings"
	"testing"

	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/conditionals"
)

func TestParseDeltaUpdateResponse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		wantNil  bool
		repaired bool
		wantErr  bool
		check    func(*testing.T, *conditionals.GameStateDelta)
	}{
		{
			name:    "empty",
			wantNil: true,
		},
		{
			name:     "repairs truncated markdown json",
			input:    "```json\n{\"user_location\": \"dock\", \"game_ended\": false,\n```",
			repaired: true,
			check: func(t *testing.T, delta *conditionals.GameStateDelta) {
				if delta.UserLocation != "dock" {
					t.Errorf("UserLocation = %q, want dock", delta.UserLocation)
				}
			},
		},
		{
			name: "salvages truncated item_events",
			input: `{
  "game_ended": false,
  "item_events": [
    {
      "action": "acquire",
      "consumed": false,
      "from": { "name": "black_pearl", "type": "location" },
      "item": "ship repair ledger"` + strings.Repeat("\n", 20),
			repaired: true,
			check: func(t *testing.T, delta *conditionals.GameStateDelta) {
				if len(delta.ItemEvents) != 1 || delta.ItemEvents[0].Item != "ship repair ledger" {
					t.Errorf("ItemEvents = %+v", delta.ItemEvents)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			delta, repaired, err := parseDeltaUpdateResponse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if repaired != tt.repaired {
				t.Errorf("repaired = %v, want %v", repaired, tt.repaired)
			}
			if tt.wantNil {
				if delta != nil {
					t.Fatalf("delta = %+v, want nil", delta)
				}
				return
			}
			if delta == nil {
				t.Fatal("delta is nil")
			}
			if tt.check != nil {
				tt.check(t, delta)
			}
		})
	}
}

func TestParseRulingResponse(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("x", 300)
	tests := []struct {
		name    string
		input   string
		want    *chat.Ruling
		wantErr bool
	}{
		{
			name: "empty",
		},
		{
			name:  "strips markdown fence",
			input: "```json\n{\"allowed\": false, \"reasoning\": \"No such exit.\", \"reaction\": \"The wall stops the PC.\"}\n```",
			want:  &chat.Ruling{Allowed: false, Reasoning: "No such exit.", Reaction: "The wall stops the PC.", Scope: chat.RulingScopeOther},
		},
		{
			name:  "null optionals become empty",
			input: `{"allowed": true, "reasoning": null, "reaction": null}`,
			want:  &chat.Ruling{Allowed: true, Scope: chat.RulingScopeOther},
		},
		{
			name:  "drops reaction when allowed",
			input: `{"allowed": true, "reasoning": "Listed exit.", "reaction": "Should be ignored."}`,
			want:  &chat.Ruling{Allowed: true, Reasoning: "Listed exit.", Scope: chat.RulingScopeOther},
		},
		{
			name:  "keeps over-length reasoning",
			input: `{"allowed": true, "reasoning": "` + long + `", "reaction": null}`,
			want:  &chat.Ruling{Allowed: true, Reasoning: long, Scope: chat.RulingScopeOther},
		},
		{
			name:  "parses subject null and object actor",
			input: `{"allowed":true,"reasoning":"Talk to the bartender.","reaction":null,"scope":"dialogue","subject":null,"object":"Bartender"}`,
			want: &chat.Ruling{
				Allowed:   true,
				Reasoning: "Talk to the bartender.",
				Scope:     chat.RulingScopeDialogue,
				Object:    "Bartender",
			},
		},
		{
			name:  "parses combat object monster name",
			input: `{"allowed":true,"reasoning":"You attack the giant rat.","reaction":null,"scope":"combat","subject":null,"object":"Giant Rat"}`,
			want: &chat.Ruling{
				Allowed:   true,
				Reasoning: "You attack the giant rat.",
				Scope:     chat.RulingScopeCombat,
				Object:    "Giant Rat",
			},
		},
		{
			name:  "parses movement object destination",
			input: `{"allowed":true,"reasoning":"You can move to the drawbridge.","reaction":null,"scope":"movement","subject":null,"object":"drawbridge"}`,
			want: &chat.Ruling{
				Allowed:   true,
				Reasoning: "You can move to the drawbridge.",
				Scope:     chat.RulingScopeMovement,
				Object:    "drawbridge",
			},
		},
		{
			name:  "null action_id",
			input: `{"allowed":true,"reasoning":null,"reaction":null,"scope":"dialogue","subject":null,"object":null,"action_id":null}`,
			want:  &chat.Ruling{Allowed: true, Scope: chat.RulingScopeDialogue},
		},
		{
			name:  "action_id survives normalize",
			input: `{"allowed":true,"reasoning":"You attack.","reaction":null,"scope":"combat","subject":null,"object":"Giant Rat","action_id":"  bite  "}`,
			want: &chat.Ruling{
				Allowed:   true,
				Reasoning: "You attack.",
				Scope:     chat.RulingScopeCombat,
				Object:    "Giant Rat",
				ActionID:  "bite",
			},
		},
		{
			name:  "unknown scope becomes other",
			input: `{"allowed":true,"reasoning":null,"reaction":null,"scope":"teleport","subject":null,"object":null}`,
			want:  &chat.Ruling{Allowed: true, Scope: chat.RulingScopeOther},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseRulingResponse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
