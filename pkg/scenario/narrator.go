package scenario

// Narrator defines the voice and style of the game narrator
type Narrator struct {
	ID          string   `json:"id"`                    // Unique identifier (e.g., "vincent_price", "classic", "comedic")
	Name        string   `json:"name"`                  // Display name
	Description string   `json:"description,omitempty"` // What this narrator style is like (not used in prompts)
	Prompts     []string `json:"prompts"`               // Voice and style instructions injected into the system prompt
	Rules       []string `json:"rules,omitempty"`       // Per-turn constraints injected into the <rules> block after every user message
}

// GetPromptsAsString returns all narrator prompts joined with newlines and bullet points
func (n *Narrator) GetPromptsAsString() string {
	if len(n.Prompts) == 0 {
		return ""
	}

	result := ""
	for _, prompt := range n.Prompts {
		result += "- " + prompt + "\n"
	}
	return result
}
