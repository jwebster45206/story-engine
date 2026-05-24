package prompts

import (
	"strings"
	"testing"

	"github.com/jwebster45206/story-engine/pkg/actor"
	"github.com/jwebster45206/story-engine/pkg/scenario"
)

// requireContains is a small helper that fails the test with the full
// rendered output if the substring is missing, which is much easier to debug
// than a plain Contains() check.
func requireContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Errorf("missing substring %q\n--- rendered output ---\n%s\n--- end ---", want, got)
	}
}

func requireNotContains(t *testing.T, got, unwanted string) {
	t.Helper()
	if strings.Contains(got, unwanted) {
		t.Errorf("unexpected substring %q\n--- rendered output ---\n%s\n--- end ---", unwanted, got)
	}
}

func TestPromptState_ToString_BasicLocation(t *testing.T) {
	ps := &PromptState{
		Location: "tavern",
		WorldLocations: map[string]scenario.Location{
			"tavern": {
				Name:        "The Rusty Anchor Tavern",
				Description: "A dimly lit tavern filled with the smell of ale and sea salt.",
				Exits: map[string]string{
					"north": "street",
				},
			},
			"street": {
				Name:        "Harbor Street",
				Description: "A busy cobblestone street near the docks.",
				Preview:     "A busy street near the docks.",
			},
		},
	}

	result := ps.ToString()

	requireContains(t, result, "<world_state>")
	requireContains(t, result, "</world_state>")
	requireContains(t, result, "<just_entered>false</just_entered>")
	requireContains(t, result, "<current_location>")
	requireContains(t, result, "The Rusty Anchor Tavern")
	requireContains(t, result, "A dimly lit tavern filled with the smell of ale and sea salt.")
	requireContains(t, result, "Exits (the ONLY directions reachable this turn):")
	requireContains(t, result, "- north -> Harbor Street")
	requireContains(t, result, "<adjacent_previews>")
	requireContains(t, result, "- north: Harbor Street - A busy street near the docks.")
	// Preview must be used, NOT the full description, for adjacent rooms.
	requireNotContains(t, result, "A busy cobblestone street near the docks.")
	// World state rules block must enumerate the literal destination.
	requireContains(t, result, "<world_state_rules>")
	requireContains(t, result, "Movement: the player may only choose one of: north (Harbor Street).")
	requireContains(t, result, `From The Rusty Anchor Tavern you can go north to Harbor Street.`)
}

func TestPromptState_ToString_JustEnteredToggle(t *testing.T) {
	ps := &PromptState{
		Location:    "room",
		JustEntered: true,
		WorldLocations: map[string]scenario.Location{
			"room": {Name: "Room"},
		},
	}
	requireContains(t, ps.ToString(), "<just_entered>true</just_entered>")

	ps.JustEntered = false
	requireContains(t, ps.ToString(), "<just_entered>false</just_entered>")
}

func TestPromptState_ToString_BlockedExits(t *testing.T) {
	ps := &PromptState{
		Location: "hallway",
		WorldLocations: map[string]scenario.Location{
			"hallway": {
				Name:        "Castle Hallway",
				Description: "A long stone corridor.",
				Exits: map[string]string{
					"north": "great_hall",
				},
				BlockedExits: map[string]string{
					"north": "the door is locked",
					"east":  "the passage has collapsed",
				},
			},
			"great_hall": {
				Name:    "Great Hall",
				Preview: "A grand room with high ceilings.",
			},
		},
	}

	result := ps.ToString()

	requireContains(t, result, "<current_location>")
	requireContains(t, result, "Castle Hallway")
	requireContains(t, result, "A long stone corridor.")
	requireContains(t, result, "- north -> Great Hall but is blocked (the door is locked)")
	// East has only a blocked entry, no exit - should render as a bare blocked line.
	requireContains(t, result, "- east is blocked (the passage has collapsed)")
}

func TestPromptState_ToString_WithNPCsHere(t *testing.T) {
	ps := &PromptState{
		Location: "market",
		WorldLocations: map[string]scenario.Location{
			"market": {
				Name:        "Town Market",
				Description: "A bustling marketplace.",
			},
		},
		NPCs: map[string]actor.NPC{
			"merchant": {
				Name:        "Greedy Merchant",
				Disposition: "neutral",
				Description: "A rotund man with a calculating look in his eyes.",
				Location:    "market",
				Items:       []string{"healing potion", "rope", "torch"},
			},
		},
	}

	result := ps.ToString()

	requireContains(t, result, "<current_location>")
	requireContains(t, result, "Town Market")
	requireContains(t, result, "NPCs here: Greedy Merchant")
	// We deliberately do NOT dump the NPC's full description or items in the
	// world state; that information lives in scenario context. Present NPCs
	// must not appear in <npcs_elsewhere>.
	requireNotContains(t, result, "<npcs_elsewhere>")
}

func TestPromptState_ToString_NPCsElsewhere(t *testing.T) {
	ps := &PromptState{
		Location: "tomb",
		WorldLocations: map[string]scenario.Location{
			"tomb": {Name: "Tomb"},
			"sleepy_mermaid": {
				Name: "Sleepy Mermaid",
			},
		},
		NPCs: map[string]actor.NPC{
			"calypso": {
				Name:        "Calypso",
				Description: "A bartender known for her enchanting stories.",
				Location:    "sleepy_mermaid",
				IsImportant: true,
			},
		},
	}

	result := ps.ToString()

	requireContains(t, result, "<npcs_elsewhere>")
	requireContains(t, result, "- Calypso: Sleepy Mermaid")
	// Description and items for remote NPCs must NOT leak in.
	requireNotContains(t, result, "A bartender known for her enchanting stories.")
	requireNotContains(t, result, "NPCs here:")
}

func TestPromptState_ToString_WithInventory(t *testing.T) {
	ps := &PromptState{
		Location: "room",
		WorldLocations: map[string]scenario.Location{
			"room": {Name: "Small Room"},
		},
		Inventory: []string{"sword", "shield", "health potion"},
	}

	result := ps.ToString()

	requireContains(t, result, "<user_inventory>")
	requireContains(t, result, "sword, shield, health potion")
	requireContains(t, result, "</user_inventory>")
}

func TestPromptState_ToString_Comprehensive(t *testing.T) {
	ps := &PromptState{
		Location:    "deck",
		JustEntered: true,
		WorldLocations: map[string]scenario.Location{
			"deck": {
				Name:        "Main Deck",
				Description: "The weathered deck of a pirate ship.",
				Items:       []string{"coiled rope"},
				Exits: map[string]string{
					"down": "hold",
				},
				BlockedExits: map[string]string{
					"south": "the plank has been removed",
				},
			},
			"hold": {
				Name:        "Ship's Hold",
				Description: "Dark and musty cargo area.",
				Preview:     "The cargo hold below decks.",
			},
		},
		NPCs: map[string]actor.NPC{
			"captain": {
				Name:        "Captain Blackbeard",
				Disposition: "hostile",
				Description: "A fearsome pirate captain.",
				Location:    "deck",
				Items:       []string{"cutlass", "pistol"},
			},
		},
		Monsters: map[string]actor.Monster{
			"rat1": {
				ID: "rat1", Name: "Giant Rat", AC: 12, HP: 7, MaxHP: 7,
				Description: "A massive rat with matted fur.",
			},
		},
		Inventory: []string{"rope", "compass"},
	}

	result := ps.ToString()

	requireContains(t, result, "<world_state>")
	requireContains(t, result, "<just_entered>true</just_entered>")
	requireContains(t, result, "<current_location>")
	requireContains(t, result, "Main Deck")
	requireContains(t, result, "The weathered deck of a pirate ship.")
	requireContains(t, result, "Items here: coiled rope")
	requireContains(t, result, "NPCs here: Captain Blackbeard")
	requireContains(t, result, "Monsters here:")
	requireContains(t, result, "- Giant Rat (AC: 12, HP: 7/7): A massive rat with matted fur.")
	requireContains(t, result, "Exits (the ONLY directions reachable this turn):")
	requireContains(t, result, "- down -> Ship's Hold")
	requireContains(t, result, "- south is blocked (the plank has been removed)")
	requireContains(t, result, "<adjacent_previews>")
	requireContains(t, result, "- down: Ship's Hold - The cargo hold below decks.")
	// Adjacent room must NOT include its full description.
	requireNotContains(t, result, "Dark and musty cargo area.")
	requireContains(t, result, "<user_inventory>")
	requireContains(t, result, "rope, compass")
	requireContains(t, result, "<world_state_rules>")
	requireContains(t, result, "Movement: the player may only choose one of: down (Ship's Hold).")
}

func TestPromptState_ToString_EmptyState(t *testing.T) {
	ps := &PromptState{}
	result := ps.ToString()

	requireContains(t, result, "<world_state>")
	requireContains(t, result, "<just_entered>false</just_entered>")
	requireContains(t, result, "<current_location>")
	requireContains(t, result, "Unknown location:")
	// With no current location, no movement rule should be emitted.
	requireNotContains(t, result, "Movement: the player may only choose one of:")
	// Rules block still present.
	requireContains(t, result, "<world_state_rules>")
}

func TestPromptState_ToString_NoExitsHidesMovementRule(t *testing.T) {
	ps := &PromptState{
		Location: "sealed_room",
		WorldLocations: map[string]scenario.Location{
			"sealed_room": {
				Name:        "Sealed Room",
				Description: "A windowless chamber with no visible exits.",
			},
		},
	}

	result := ps.ToString()

	requireContains(t, result, "Sealed Room")
	// No exits, so no exits header and no movement rule.
	requireNotContains(t, result, "Exits (the ONLY directions reachable this turn):")
	requireNotContains(t, result, "Movement: the player may only choose")
	// But the other rules still apply.
	requireContains(t, result, "Narrate ONLY current_location.")
}

func TestPromptState_ToString_AdjacentPreviewFallbackToName(t *testing.T) {
	ps := &PromptState{
		Location: "tavern",
		WorldLocations: map[string]scenario.Location{
			"tavern": {
				Name: "The Rusty Anchor",
				Exits: map[string]string{
					"north": "street",
				},
			},
			"street": {
				Name: "Harbor Street",
				// no Preview, no Description
			},
		},
	}

	result := ps.ToString()

	// When no preview is set, we still emit direction + name, but with no
	// dash-separated tail.
	requireContains(t, result, "- north: Harbor Street\n")
	requireNotContains(t, result, "- north: Harbor Street -")
}

func TestPromptState_ToString_NoLeakBetweenAdjacentRooms(t *testing.T) {
	// Regression: the previous branch rendered adjacent rooms with full
	// Description + their own per-room exits, which caused leakage. Verify
	// that adjacent rooms only contribute a preview line.
	ps := &PromptState{
		Location: "entry_hall",
		WorldLocations: map[string]scenario.Location{
			"entry_hall": {
				Name:        "Entry Hall",
				Description: "A wide ceremonial corridor.",
				Exits: map[string]string{
					"west": "antechamber",
				},
			},
			"antechamber": {
				Name:        "Antechamber",
				Description: "A broad vaulted chamber held up by cat-pillar columns with unblinking obsidian eyes.",
				Preview:     "A vaulted entry chamber with cat-pillar columns.",
				Exits: map[string]string{
					"north": "tomb_entrance", // 2-hop away from player
				},
			},
		},
	}

	result := ps.ToString()

	// Preview must appear.
	requireContains(t, result, "- west: Antechamber - A vaulted entry chamber with cat-pillar columns.")
	// Full description of adjacent room must NOT appear.
	requireNotContains(t, result, "unblinking obsidian eyes")
	// 2-hop exits of adjacent rooms must NOT be exposed.
	requireNotContains(t, result, "tomb_entrance")
}

func TestPromptState_ToString_ExitOrderingIsStable(t *testing.T) {
	// Run several times to assert deterministic output despite map iteration.
	ps := &PromptState{
		Location: "crossroads",
		WorldLocations: map[string]scenario.Location{
			"crossroads": {
				Name: "Crossroads",
				Exits: map[string]string{
					"north": "n_loc",
					"east":  "e_loc",
					"south": "s_loc",
					"west":  "w_loc",
				},
			},
			"n_loc": {Name: "N"},
			"e_loc": {Name: "E"},
			"s_loc": {Name: "S"},
			"w_loc": {Name: "W"},
		},
	}

	first := ps.ToString()
	for i := 0; i < 10; i++ {
		if ps.ToString() != first {
			t.Fatalf("ToString output is not deterministic across calls")
		}
	}
	// Verify exits appear in alphabetical order.
	idxEast := strings.Index(first, "- east ->")
	idxNorth := strings.Index(first, "- north ->")
	idxSouth := strings.Index(first, "- south ->")
	idxWest := strings.Index(first, "- west ->")
	if idxEast >= idxNorth || idxNorth >= idxSouth || idxSouth >= idxWest {
		t.Errorf("exits should be sorted alphabetically; got order: east=%d north=%d south=%d west=%d",
			idxEast, idxNorth, idxSouth, idxWest)
	}
}

func TestPromptState_ToString_WithMonstersHere(t *testing.T) {
	ps := &PromptState{
		Location: "dungeon",
		WorldLocations: map[string]scenario.Location{
			"dungeon": {Name: "Dark Dungeon", Description: "A dank chamber."},
		},
		Monsters: map[string]actor.Monster{
			"rat1": {
				ID: "rat1", Name: "Giant Rat", AC: 12, HP: 9, MaxHP: 9,
				Description: "A filthy, red-eyed rodent the size of a dog.",
			},
			"skeleton1": {
				ID: "skeleton1", Name: "Skeleton Warrior", AC: 13, HP: 15, MaxHP: 20,
				Description: "An animated skeleton wielding a rusty sword.",
			},
		},
	}

	result := ps.ToString()

	requireContains(t, result, "Monsters here:")
	requireContains(t, result, "- Giant Rat (AC: 12, HP: 9/9): A filthy, red-eyed rodent the size of a dog.")
	requireContains(t, result, "- Skeleton Warrior (AC: 13, HP: 15/20): An animated skeleton wielding a rusty sword.")
}

func TestPromptState_ToString_ImportantElsewhereWithoutDirection(t *testing.T) {
	// Important locations not reachable via an exit from current should be
	// shown under <adjacent_previews> but without a direction prefix, so the
	// narrator knows they exist but does not treat them as one-step reachable.
	ps := &PromptState{
		Location: "tavern",
		WorldLocations: map[string]scenario.Location{
			"tavern": {
				Name: "Tavern",
				Exits: map[string]string{
					"north": "street",
				},
			},
			"street": {Name: "Street", Preview: "A cobblestone street."},
			"castle": {
				Name:        "Castle",
				Preview:     "A distant castle on the hill.",
				IsImportant: true,
			},
		},
	}

	result := ps.ToString()

	requireContains(t, result, "- north: Street - A cobblestone street.")
	requireContains(t, result, "- Castle (elsewhere) - A distant castle on the hill.")
	// Castle must NOT appear in the movement rule.
	requireNotContains(t, result, "Castle).")
}

func TestPromptState_ToString_RedirectTemplateMultiExit(t *testing.T) {
	ps := &PromptState{
		Location: "hub",
		WorldLocations: map[string]scenario.Location{
			"hub": {
				Name: "Hub",
				Exits: map[string]string{
					"east":  "east_room",
					"north": "north_room",
					"south": "south_room",
				},
			},
			"east_room":  {Name: "East Room"},
			"north_room": {Name: "North Room"},
			"south_room": {Name: "South Room"},
		},
	}

	result := ps.ToString()

	// Comma-separated list with final "or".
	requireContains(t, result, `From Hub you can go east to East Room, north to North Room, or south to South Room.`)
	// Movement options use parenthesized form, sorted alphabetically by direction.
	requireContains(t, result, "Movement: the player may only choose one of: east (East Room), north (North Room), south (South Room).")
}
