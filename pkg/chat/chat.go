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
