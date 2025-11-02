package conditionals

// GameStateDelta is a compact, structured representation of changes to game state.
// It is intentionally smaller and stricter than the full game state.
type GameStateDelta struct {
	UserLocation string `json:"user_location"`
	SceneChange  *struct {
		To     string `json:"to"`
		Reason string `json:"reason"`
	} `json:"scene_change,omitempty"`

	ItemEvents []struct {
		Item   string `json:"item"`
		Action string `json:"action"` // enum "acquire" | "give" | "drop" | "move" | "use"
		From   *struct {
			Type string `json:"type"` // enum "player" | "npc" | "location"
			Name string `json:"name,omitempty"`
		} `json:"from,omitempty"`
		To *struct {
			Type string `json:"type"` // enum "player" | "npc" | "location"
			Name string `json:"name,omitempty"`
		} `json:"to,omitempty"`
		Consumed *bool `json:"consumed,omitempty"`
	} `json:"item_events,omitempty"`

	NPCEvents []NPCEvent `json:"npc_events,omitempty"`

	MonsterEvents []MonsterEvent `json:"monster_events,omitempty"`

	// TODO: Add LocationEvents structure to track stateful elements of locations:
	// such as exits being blocked/unblocked, conditions changing, etc.

	SetVars   map[string]string `json:"set_vars,omitempty"`
	GameEnded *bool             `json:"game_ended,omitempty"`
	Prompt    *string           `json:"prompt,omitempty"` // Narrative prompt to inject (use "STORY EVENT: " prefix for story events)
}

type MonsterEventAction string

const (
	MonsterEventSpawn   MonsterEventAction = "spawn"
	MonsterEventDespawn MonsterEventAction = "despawn"
)

// MonsterEvent represents a change to a monster's state (spawn or despawn)
type MonsterEvent struct {
	Action     MonsterEventAction `json:"action"`      // enum "spawn" | "despawn"
	InstanceID string             `json:"instance_id"` // Unique ID for this monster instance

	// Required for spawn
	Template string `json:"template,omitempty"` // Template ID to load from data/monsters/
	Location string `json:"location,omitempty"` // Location key where monster should spawn

	// Optional overrides for spawn (override template values)
	Name              string         `json:"name,omitempty"`
	Description       string         `json:"description,omitempty"`
	AC                int            `json:"ac,omitempty"`
	HP                int            `json:"hp,omitempty"`
	MaxHP             int            `json:"max_hp,omitempty"`
	Attributes        map[string]int `json:"attributes,omitempty"`
	CombatMods        map[string]int `json:"combat_modifiers,omitempty"`
	Items             []string       `json:"items,omitempty"`
	DropItemsOnDefeat *bool          `json:"drop_items_on_defeat,omitempty"`
}

// NPCEvent represents a change to an NPC's state
type NPCEvent struct {
	NPCID        string  `json:"npc_id"`
	SetLocation  *string `json:"set_location,omitempty"`  // Set NPC to specific location
	SetFollowing *string `json:"set_following,omitempty"` // Set following target ("pc", npc_id, or "" to clear).
}
