package llm

import (
	"strings"
	"testing"
)

func TestParseDeltaUpdateResponse_Empty(t *testing.T) {
	t.Parallel()
	delta, repaired, err := parseDeltaUpdateResponse("")
	if err != nil || repaired || delta != nil {
		t.Fatalf("delta=%v repaired=%v err=%v", delta, repaired, err)
	}
}

func TestParseDeltaUpdateResponse_MarkdownAndRepair(t *testing.T) {
	t.Parallel()
	input := "```json\n{\"user_location\": \"dock\", \"game_ended\": false,\n```"
	delta, repaired, err := parseDeltaUpdateResponse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repaired {
		t.Fatal("expected repaired=true")
	}
	if delta == nil || delta.UserLocation != "dock" {
		t.Fatalf("unexpected delta: %+v", delta)
	}
}

func TestParseDeltaUpdateResponse_TruncatedAcquire(t *testing.T) {
	t.Parallel()
	input := `{
  "game_ended": false,
  "item_events": [
    {
      "action": "acquire",
      "consumed": false,
      "from": { "name": "black_pearl", "type": "location" },
      "item": "ship repair ledger"` + strings.Repeat("\n", 20)
	delta, repaired, err := parseDeltaUpdateResponse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repaired {
		t.Fatal("expected repaired=true")
	}
	if delta == nil || len(delta.ItemEvents) != 1 || delta.ItemEvents[0].Item != "ship repair ledger" {
		t.Fatalf("unexpected delta: %+v", delta)
	}
}
