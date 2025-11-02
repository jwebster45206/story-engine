package prompts

import (
	"strings"
	"testing"

	"github.com/jwebster45206/story-engine/pkg/actor"
	"github.com/jwebster45206/story-engine/pkg/scenario"
)

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
			},
		},
	}

	result := ps.ToString()

	// Check required sections exist
	if !strings.Contains(result, "CURRENT LOCATION:") {
		t.Error("Missing CURRENT LOCATION section")
	}
	if !strings.Contains(result, "The Rusty Anchor Tavern") {
		t.Error("Missing current location name")
	}
	if !strings.Contains(result, "A dimly lit tavern filled with the smell of ale and sea salt.") {
		t.Error("Missing current location description")
	}
	if !strings.Contains(result, "Exits:") {
		t.Error("Missing Exits section")
	}
	if !strings.Contains(result, "- north leads to Harbor Street") {
		t.Error("Missing exit information")
	}
	if !strings.Contains(result, "NEARBY LOCATIONS:") {
		t.Error("Missing NEARBY LOCATIONS section")
	}
	if !strings.Contains(result, "Harbor Street: A busy cobblestone street near the docks.") {
		t.Error("Missing nearby location")
	}
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
				},
			},
			"great_hall": {
				Name:        "Great Hall",
				Description: "A grand room with high ceilings and ornate decorations.",
				Exits: map[string]string{
					"south": "hallway",
				},
				BlockedExits: map[string]string{
					"south": "the door is locked",
				},
			},
		},
	}

	result := ps.ToString()

	if !strings.Contains(result, "CURRENT LOCATION:") {
		t.Error("Missing CURRENT LOCATION section")
	}
	if !strings.Contains(result, "Castle Hallway") {
		t.Error("Missing location name")
	}
	if !strings.Contains(result, "A long stone corridor.") {
		t.Error("Missing location description")
	}
	if !strings.Contains(result, "Exits:") {
		t.Error("Missing Exits section")
	}
	if !strings.Contains(result, "- north leads to Great Hall but is blocked (the door is locked)") {
		t.Errorf("Missing blocked exit information; got: %s", result)
	}
}

func TestPromptState_ToString_WithNPCs(t *testing.T) {
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
				Items:       []string{"healing potion", "rope", "torch"},
			},
		},
	}

	result := ps.ToString()

	if !strings.Contains(result, "CURRENT LOCATION:") {
		t.Error("Missing CURRENT LOCATION section")
	}
	if !strings.Contains(result, "Town Market") {
		t.Error("Missing location name")
	}
	if !strings.Contains(result, "NPCs:") {
		t.Error("Missing NPCs section")
	}
	if !strings.Contains(result, "Greedy Merchant (neutral)") {
		t.Error("Missing NPC name and disposition")
	}
	if !strings.Contains(result, "A rotund man with a calculating look in his eyes.") {
		t.Error("Missing NPC description")
	}
	if !strings.Contains(result, "Items: healing potion, rope, torch") {
		t.Error("Missing NPC items")
	}
}

func TestPromptState_ToString_WithInventory(t *testing.T) {
	ps := &PromptState{
		Location: "room",
		WorldLocations: map[string]scenario.Location{
			"room": {
				Name: "Small Room",
			},
		},
		Inventory: []string{"sword", "shield", "health potion"},
	}

	result := ps.ToString()

	if !strings.Contains(result, "CURRENT LOCATION:") {
		t.Error("Missing CURRENT LOCATION section")
	}
	if !strings.Contains(result, "Small Room") {
		t.Error("Missing location name")
	}
	if !strings.Contains(result, "USER'S INVENTORY:") {
		t.Error("Missing USER'S INVENTORY section")
	}
	if !strings.Contains(result, "sword, shield, health potion") {
		t.Error("Missing inventory items")
	}
}

func TestPromptState_ToString_Comprehensive(t *testing.T) {
	ps := &PromptState{
		Location: "deck",
		WorldLocations: map[string]scenario.Location{
			"deck": {
				Name:        "Main Deck",
				Description: "The weathered deck of a pirate ship.",
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
			},
		},
		NPCs: map[string]actor.NPC{
			"captain": {
				Name:        "Captain Blackbeard",
				Disposition: "hostile",
				Description: "A fearsome pirate captain.",
				Items:       []string{"cutlass", "pistol"},
			},
		},
		Inventory: []string{"rope", "compass"},
	}

	result := ps.ToString()

	// Check all major sections
	if !strings.Contains(result, "CURRENT LOCATION:") {
		t.Error("Missing CURRENT LOCATION section")
	}
	if !strings.Contains(result, "Main Deck") {
		t.Error("Missing current location name")
	}
	if !strings.Contains(result, "The weathered deck of a pirate ship.") {
		t.Error("Missing current location description")
	}
	if !strings.Contains(result, "Exits:") {
		t.Error("Missing Exits section")
	}
	if !strings.Contains(result, "- down leads to Ship's Hold") {
		t.Error("Missing exit to hold")
	}
	if !strings.Contains(result, "- south is blocked (the plank has been removed)") {
		t.Error("Missing blocked exit")
	}
	if !strings.Contains(result, "NEARBY LOCATIONS:") {
		t.Error("Missing NEARBY LOCATIONS section")
	}
	if !strings.Contains(result, "Ship's Hold: Dark and musty cargo area.") {
		t.Error("Missing nearby location")
	}
	if !strings.Contains(result, "NPCs:") {
		t.Error("Missing NPCs section")
	}
	if !strings.Contains(result, "Captain Blackbeard (hostile)") {
		t.Error("Missing NPC name and disposition")
	}
	if !strings.Contains(result, "A fearsome pirate captain.") {
		t.Error("Missing NPC description")
	}
	if !strings.Contains(result, "Items: cutlass, pistol") {
		t.Error("Missing NPC items")
	}
	if !strings.Contains(result, "USER'S INVENTORY:") {
		t.Error("Missing USER'S INVENTORY section")
	}
	if !strings.Contains(result, "rope, compass") {
		t.Error("Missing inventory items")
	}
}

func TestPromptState_ToString_EmptyState(t *testing.T) {
	ps := &PromptState{}
	result := ps.ToString()

	if !strings.Contains(result, "CURRENT LOCATION:") {
		t.Error("Missing CURRENT LOCATION section")
	}
	if !strings.Contains(result, "Unknown location:") {
		t.Error("Missing unknown location message")
	}
}

func TestPromptState_ToString_NPCWithoutDisposition(t *testing.T) {
	ps := &PromptState{
		Location: "room",
		WorldLocations: map[string]scenario.Location{
			"room": {Name: "Room"},
		},
		NPCs: map[string]actor.NPC{
			"stranger": {
				Name:        "Mysterious Stranger",
				Description: "A cloaked figure.",
			},
		},
	}

	result := ps.ToString()

	if !strings.Contains(result, "NPCs:") {
		t.Error("Missing NPCs section")
	}
	if !strings.Contains(result, "Mysterious Stranger") {
		t.Error("Missing NPC name")
	}
	if !strings.Contains(result, "A cloaked figure.") {
		t.Error("Missing NPC description")
	}
	// Should NOT have empty parentheses
	if strings.Contains(result, "Mysterious Stranger ()") {
		t.Error("Should not have empty parentheses for NPC without disposition")
	}
}

func TestPromptState_ToString_WithMonsters(t *testing.T) {
	ps := &PromptState{
		Location: "dungeon",
		WorldLocations: map[string]scenario.Location{
			"dungeon": {
				Name:        "Dark Dungeon",
				Description: "A dank underground chamber.",
			},
		},
		Monsters: map[string]actor.Monster{
			"rat1": {
				ID:          "rat1",
				Name:        "Giant Rat",
				Description: "A filthy, red-eyed rodent the size of a dog.",
				AC:          12,
				HP:          9,
				MaxHP:       9,
				Location:    "dungeon",
			},
			"skeleton1": {
				ID:          "skeleton1",
				Name:        "Skeleton Warrior",
				Description: "An animated skeleton wielding a rusty sword.",
				AC:          13,
				HP:          15,
				MaxHP:       20,
				Location:    "dungeon",
			},
		},
	}

	result := ps.ToString()

	// Check that MONSTERS section exists
	if !strings.Contains(result, "MONSTERS:") {
		t.Error("Missing MONSTERS section")
	}

	// Check for monster details
	if !strings.Contains(result, "Giant Rat (AC: 12, HP: 9/9)") {
		t.Error("Missing Giant Rat with stats")
	}
	if !strings.Contains(result, "A filthy, red-eyed rodent the size of a dog.") {
		t.Error("Missing Giant Rat description")
	}

	if !strings.Contains(result, "Skeleton Warrior (AC: 13, HP: 15/20)") {
		t.Error("Missing Skeleton Warrior with stats")
	}
	if !strings.Contains(result, "An animated skeleton wielding a rusty sword.") {
		t.Error("Missing Skeleton Warrior description")
	}
}
