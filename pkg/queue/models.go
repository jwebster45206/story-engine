package queue

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// RequestType identifies the type of request in the queue
type RequestType string

const (
	// RequestTypeChat is a user-initiated chat message
	RequestTypeChat RequestType = "chat"

	// RequestTypeStoryEvent is a system-generated story event
	RequestTypeStoryEvent RequestType = "story_event"
)

// Request represents a unified request in the queue
type Request struct {
	RequestID   string      `json:"request_id"`
	Type        RequestType `json:"type"`
	GameStateID uuid.UUID   `json:"game_state_id"`

	// Chat-specific fields
	Message string `json:"message,omitempty"`
	Actor   string `json:"actor,omitempty"`

	// Story event-specific fields
	EventPrompt string `json:"event_prompt,omitempty"`

	EnqueuedAt time.Time `json:"enqueued_at"`
}

// MarshalJSON serializes the request to JSON for Redis storage
func (r *Request) MarshalJSON() ([]byte, error) {
	type Alias Request
	return json.Marshal(&struct {
		GameStateID string `json:"game_state_id"`
		*Alias
	}{
		GameStateID: r.GameStateID.String(),
		Alias:       (*Alias)(r),
	})
}

// UnmarshalJSON deserializes the request from JSON in Redis
func (r *Request) UnmarshalJSON(data []byte) error {
	type Alias Request
	aux := &struct {
		GameStateID string `json:"game_state_id"`
		*Alias
	}{
		Alias: (*Alias)(r),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	gameStateID, err := uuid.Parse(aux.GameStateID)
	if err != nil {
		return err
	}

	r.GameStateID = gameStateID
	return nil
}

// ToJSON converts the request to JSON bytes for Redis
func (r *Request) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

// FromJSON parses a request from JSON bytes
func FromJSON(data []byte) (*Request, error) {
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, err
	}
	return &req, nil
}
