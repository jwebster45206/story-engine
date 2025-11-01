package storage

import (
	"context"
	"errors"
	"sync"

	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/pkg/actor"
	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/state"
)

// MockStorage is a mock implementation of Storage for testing
type MockStorage struct {
	mu         sync.RWMutex
	gamestates map[uuid.UUID]*state.GameState
	scenarios  map[string]*scenario.Scenario
	narrators  map[string]*scenario.Narrator
	pcSpecs    map[string]*actor.PCSpec
	monsters   map[string]*actor.Monster
	pingError  error
}

// Ensure MockStorage implements Storage interface
var _ Storage = (*MockStorage)(nil)

// NewMockStorage creates a new mock storage
func NewMockStorage() *MockStorage {
	return &MockStorage{
		gamestates: make(map[uuid.UUID]*state.GameState),
		scenarios:  make(map[string]*scenario.Scenario),
		narrators:  make(map[string]*scenario.Narrator),
		pcSpecs:    make(map[string]*actor.PCSpec),
		monsters:   make(map[string]*actor.Monster),
	}
}

// SetPingSuccess configures the mock to succeed on ping
func (m *MockStorage) SetPingSuccess() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pingError = nil
}

// SetPingError configures the mock to fail on ping with the given error
func (m *MockStorage) SetPingError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pingError = err
}

// Ping mocks storage ping
func (m *MockStorage) Ping(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
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
func (m *MockStorage) SaveGameState(ctx context.Context, id uuid.UUID, gamestate *state.GameState) error {
	if gamestate == nil {
		return errors.New("gamestate cannot be nil")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gamestates[id] = gamestate
	return nil
}

// LoadGameState mocks loading a gamestate
func (m *MockStorage) LoadGameState(ctx context.Context, id uuid.UUID) (*state.GameState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	gamestate, exists := m.gamestates[id]
	if !exists {
		return nil, nil // Return nil for not found
	}
	return gamestate, nil
}

// DeleteGameState mocks deleting a gamestate
func (m *MockStorage) DeleteGameState(ctx context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.gamestates, id)
	return nil
}

// ListScenarios mocks listing scenarios
func (m *MockStorage) ListScenarios(ctx context.Context) (map[string]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Build map of scenario names to filenames
	result := make(map[string]string)
	for filename, s := range m.scenarios {
		result[s.Name] = filename
	}
	return result, nil
}

// GetScenario mocks getting a scenario by filename
func (m *MockStorage) GetScenario(ctx context.Context, filename string) (*scenario.Scenario, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, exists := m.scenarios[filename]
	if !exists {
		return nil, errors.New("scenario not found")
	}
	return s, nil
}

// AddScenario adds a scenario to the mock storage (for testing)
func (m *MockStorage) AddScenario(filename string, s *scenario.Scenario) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scenarios[filename] = s
}

// GetNarrator mocks getting a narrator by ID
func (m *MockStorage) GetNarrator(ctx context.Context, narratorID string) (*scenario.Narrator, error) {
	if narratorID == "" {
		return nil, nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	n, exists := m.narrators[narratorID]
	if !exists {
		return nil, errors.New("narrator not found")
	}
	return n, nil
}

// ListNarrators mocks listing narrators
func (m *MockStorage) ListNarrators(ctx context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]string, 0, len(m.narrators))
	for id := range m.narrators {
		result = append(result, id)
	}
	return result, nil
}

// AddNarrator adds a narrator to the mock storage (for testing)
func (m *MockStorage) AddNarrator(narratorID string, n *scenario.Narrator) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.narrators[narratorID] = n
}

// GetPCSpec mocks getting a PC spec by ID
func (m *MockStorage) GetPCSpec(ctx context.Context, pcID string) (*actor.PCSpec, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	spec, exists := m.pcSpecs[pcID]
	if !exists {
		return nil, errors.New("PC spec not found")
	}
	return spec, nil
}

// ListPCs mocks listing PCs
func (m *MockStorage) ListPCs(ctx context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]string, 0, len(m.pcSpecs))
	for id := range m.pcSpecs {
		result = append(result, id)
	}
	return result, nil
}

// AddPCSpec adds a PC spec to the mock storage (for testing)
func (m *MockStorage) AddPCSpec(pcID string, spec *actor.PCSpec) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pcSpecs[pcID] = spec
}

// GetMonster mocks getting a monster template by ID
func (m *MockStorage) GetMonster(ctx context.Context, templateID string) (*actor.Monster, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	monster, exists := m.monsters[templateID]
	if !exists {
		return nil, errors.New("monster template not found")
	}
	return monster, nil
}

// ListMonsters mocks listing monsters
func (m *MockStorage) ListMonsters(ctx context.Context) (map[string]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]string)
	for templateID, monster := range m.monsters {
		result[monster.Name] = templateID
	}
	return result, nil
}

// AddMonster adds a monster template to the mock storage (for testing)
func (m *MockStorage) AddMonster(templateID string, monster *actor.Monster) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.monsters[templateID] = monster
}
