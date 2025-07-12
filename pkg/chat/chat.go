package chat

// ChatRequest represents a chat message request made by the user
// to the roleplay-agent api.
type ChatRequest struct {
	GameStateID string `json:"gamestate_id"` // Unique ID for the game state
	Message     string `json:"message"`
}

// ChatResponse represents a chat message response returned by the roleplay-agent api.
// It omits GameState currently, but GameState should be updated
// within the chat handler.
type ChatResponse struct {
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}

const (
	ChatRoleUser   = "user"      // Use
	ChatRoleAgent  = "assistant" // NPC
	ChatRoleSystem = "system"    // Narrator or system
)

// ChatMessage represents a single chat message in the conversation
// This interface is defined by Ollama's API and is used to structure messages
// sent to the LLM.
type ChatMessage struct {
	Role    string `json:"role"` // "user", "assistant", "system"
	Content string `json:"content"`
}
