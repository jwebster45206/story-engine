package chat

// ChatRequest represents a chat message request
type ChatRequest struct {
	Message string `json:"message"`
}

// ChatResponse represents a chat message response
type ChatResponse struct {
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}

const (
	ChatRoleUser     = "user"      // Use
	ChatRoleAgent    = "assistant" // NPC
	ChatRoleNarrator = "system"    // Narrator or system
)

// ChatMessage represents a single chat message in the conversation
// This interface is defined by Ollama's API and is used to structure messages
// sent to the LLM.
type ChatMessage struct {
	Role    string `json:"role"` // "user", "assistant", "system"
	Content string `json:"content"`
}
