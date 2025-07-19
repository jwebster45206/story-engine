package scenario

// These structs are not implemented yet, but are sketched out for future use.

// Flag represents a boolean condition in the scenario that can be set or checked.
type Flag struct {
	Value  bool   `json:"value"`  // True if the flag is active.
	Prompt string `json:"prompt"` // LLM instructions for using the flag.
}

// Trigger represents an action that occurs when a specific condition is met in the game.
// As sketched out here, a LLM would need to handle both conditions and actions.
// TODO: Might be nice to have non-LLM triggers.
// Consider modularity, and keeping scenarios easy to write and read.
type Trigger struct {
	// Prompt is the LLM instruction for the trigger.
	// Example: "When the user first enters the dragon's lair, set the 'dragon_appeared' flag."

	// When the user acquires both halves of the map, remove both map halves from inventory, and add the complete map to inventory. Also set the map_complete flag.""

	Condition string `json:"condition"` // Condition that triggers the action.
	Action    string `json:"action"`    // Action to perform when triggered.
}
