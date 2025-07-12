package services

import (
	"context"
	"errors"

	"github.com/jwebster45206/roleplay-agent/pkg/state"
)

// MockStorage is a mock implementation of Storage for testing
type MockStorage struct {
	gamestates map[string]*state.GameState
	pingError  error
}

// Ensure MockStorage implements Storage interface
var _ Storage = (*MockStorage)(nil)

// NewMockStorage creates a new mock storage
func NewMockStorage() *MockStorage {
	return &MockStorage{
		gamestates: make(map[string]*state.GameState),
	}
}

// SetPingSuccess configures the mock to succeed on ping
func (m *MockStorage) SetPingSuccess() {
	m.pingError = nil
}

// SetPingError configures the mock to fail on ping with the given error
func (m *MockStorage) SetPingError(err error) {
	m.pingError = err
}

// Ping mocks storage ping
func (m *MockStorage) Ping(ctx context.Context) error {
	if m.pingError != nil {
		return m.pingError
	}
	return nil
}

// Close mocks storage close
func (m *MockStorage) Close() error {
	// Mock close doesn't need to do anything
	return nil
}

// SaveGameState mocks saving a gamestate
func (m *MockStorage) SaveGameState(ctx context.Context, uuid string, gamestate *state.GameState) error {
	if gamestate == nil {
		return errors.New("gamestate cannot be nil")
	}
	m.gamestates[uuid] = gamestate
	return nil
}

// LoadGameState mocks loading a gamestate
func (m *MockStorage) LoadGameState(ctx context.Context, uuid string) (*state.GameState, error) {
	gamestate, exists := m.gamestates[uuid]
	if !exists {
		return nil, nil // Return nil for not found
	}
	return gamestate, nil
}

// DeleteGameState mocks deleting a gamestate
func (m *MockStorage) DeleteGameState(ctx context.Context, uuid string) error {
	delete(m.gamestates, uuid)
	return nil
}
