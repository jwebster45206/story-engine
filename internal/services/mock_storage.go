package services

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/state"
)

// MockStorage is a mock implementation of Storage for testing
type MockStorage struct {
	gamestates map[uuid.UUID]*state.GameState
	scenarios  map[string]*scenario.Scenario
	pingError  error
}

// Ensure MockStorage implements Storage interface
var _ Storage = (*MockStorage)(nil)

// NewMockStorage creates a new mock storage
func NewMockStorage() *MockStorage {
	return &MockStorage{
		gamestates: make(map[uuid.UUID]*state.GameState),
		scenarios:  make(map[string]*scenario.Scenario),
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
func (m *MockStorage) SaveGameState(ctx context.Context, uuid uuid.UUID, gamestate *state.GameState) error {
	if gamestate == nil {
		return errors.New("gamestate cannot be nil")
	}
	m.gamestates[uuid] = gamestate
	return nil
}

// LoadGameState mocks loading a gamestate
func (m *MockStorage) LoadGameState(ctx context.Context, uuid uuid.UUID) (*state.GameState, error) {
	gamestate, exists := m.gamestates[uuid]
	if !exists {
		return nil, nil // Return nil for not found
	}
	return gamestate, nil
}

// DeleteGameState mocks deleting a gamestate
func (m *MockStorage) DeleteGameState(ctx context.Context, uuid uuid.UUID) error {
	delete(m.gamestates, uuid)
	return nil
}

// ListScenarios mocks listing scenarios
func (m *MockStorage) ListScenarios(ctx context.Context) (map[string]string, error) {
	// Return a mock list of scenarios
	scenarios := map[string]string{
		"Pirate Adventure": "pirate_scenario.json",
	}
	return scenarios, nil
}

// GetScenario mocks getting a scenario by filename
func (m *MockStorage) GetScenario(ctx context.Context, filename string) (*scenario.Scenario, error) {
	// Return a mock scenario based on filename
	switch filename {
	case "foo_scenario.json":
		return &scenario.Scenario{
			Name:  "FooScenario",
			Story: "A test scenario story",
			Locations: map[string]scenario.Location{
				"TestLocation": {
					Name:         "TestLocation",
					Description:  "A location for testing",
					Exits:        map[string]string{"north": "OtherLocation"},
					BlockedExits: map[string]string{"south": "The way is blocked by a locked door."},
				},
				"OtherLocation": {
					Name:        "OtherLocation",
					Description: "Another test location",
				},
			},
			Inventory: []string{"test_item1", "test_item2"},
			NPCs:      map[string]scenario.NPC{"TestNPC": {Name: "TestNPC", Type: "human", Disposition: "neutral"}},
			// Triggers:      []string{"test_trigger"},
			OpeningPrompt:   "Welcome to the FooScenario!",
			OpeningLocation: "TestLocation",
		}, nil
	default:
		return nil, errors.New("scenario not found")
	}
}
