package worker

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/internal/services"
	"github.com/jwebster45206/story-engine/pkg/actor"
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/conditionals"
	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/state"
)

func TestApplyConditionalsCascade_NoConditionals(t *testing.T) {
	// Setup
	logger := slog.Default()
	gs := &state.GameState{
		ID:   uuid.New(),
		Vars: make(map[string]string),
	}
	delta := &conditionals.GameStateDelta{}
	s := &scenario.Scenario{
		Scenes: map[string]scenario.Scene{
			"start": {
				Conditionals: map[string]scenario.Conditional{},
			},
		},
	}

	processor := &ChatProcessor{logger: logger}
	worker := state.NewDeltaWorker(gs, delta, s, logger)

	// Execute
	processor.applyConditionalsCascade(worker, gs.ID)

	// No conditionals should trigger, function should return cleanly
	// (This is mainly testing that it doesn't panic or error)
}

func TestApplyConditionalsCascade_OneIteration(t *testing.T) {
	// Setup
	logger := slog.Default()
	gs := &state.GameState{
		ID:        uuid.New(),
		SceneName: "start",
		Vars: map[string]string{
			"player_score": "100",
		},
	}

	winFlag := true
	delta := &conditionals.GameStateDelta{}
	s := &scenario.Scenario{
		Scenes: map[string]scenario.Scene{
			"start": {
				Conditionals: map[string]scenario.Conditional{
					"high_score_win": {
						When: conditionals.ConditionalWhen{
							Vars: map[string]string{
								"player_score": "100",
							},
						},
						Then: conditionals.GameStateDelta{
							GameEnded: &winFlag,
							SetVars: map[string]string{
								"victory": "true",
							},
						},
					},
				},
			},
		},
	}

	processor := &ChatProcessor{logger: logger}
	worker := state.NewDeltaWorker(gs, delta, s, logger)

	// Execute
	processor.applyConditionalsCascade(worker, gs.ID)

	// Verify the conditional triggered and applied
	if gs.IsEnded != true {
		t.Errorf("Expected game to be ended, but IsEnded = %v", gs.IsEnded)
	}
	if victory := gs.Vars["victory"]; victory != "true" {
		t.Errorf("Expected victory var to be 'true', got %v", victory)
	}
}

func TestApplyConditionalsCascade_TwoIterations(t *testing.T) {
	// Setup
	logger := slog.Default()
	gs := &state.GameState{
		ID:        uuid.New(),
		SceneName: "start",
		Vars: map[string]string{
			"player_score": "100",
		},
	}

	endGame := true
	delta := &conditionals.GameStateDelta{}
	s := &scenario.Scenario{
		Scenes: map[string]scenario.Scene{
			"start": {
				Conditionals: map[string]scenario.Conditional{
					// First iteration: high score sets achievement
					"high_score_achievement": {
						When: conditionals.ConditionalWhen{
							Vars: map[string]string{
								"player_score": "100",
							},
						},
						Then: conditionals.GameStateDelta{
							SetVars: map[string]string{
								"achievement_unlocked": "true",
							},
						},
					},
					// Second iteration: achievement ends game
					"achievement_win": {
						When: conditionals.ConditionalWhen{
							Vars: map[string]string{
								"achievement_unlocked": "true",
							},
						},
						Then: conditionals.GameStateDelta{
							GameEnded: &endGame,
							SetVars: map[string]string{
								"victory": "true",
							},
						},
					},
				},
			},
		},
	}

	processor := &ChatProcessor{logger: logger}
	worker := state.NewDeltaWorker(gs, delta, s, logger)

	// Execute
	processor.applyConditionalsCascade(worker, gs.ID)

	// Verify both conditionals triggered in cascade
	if achievement := gs.Vars["achievement_unlocked"]; achievement != "true" {
		t.Errorf("Expected achievement_unlocked to be 'true', got %v", achievement)
	}
	if victory := gs.Vars["victory"]; victory != "true" {
		t.Errorf("Expected victory var to be 'true', got %v", victory)
	}
	if gs.IsEnded != true {
		t.Errorf("Expected game to be ended, but IsEnded = %v", gs.IsEnded)
	}
}

// ---------------------------------------------------------------------------
// Stubs
// ---------------------------------------------------------------------------

// stubLLMService captures the messages slice passed to Chat() and no-ops everything else.
type stubLLMService struct {
	capturedMessages []chat.ChatMessage
	capturedTemp     float64
}

func (s *stubLLMService) InitModel(_ context.Context, _ string) error { return nil }
func (s *stubLLMService) Chat(_ context.Context, messages []chat.ChatMessage, temperature float64) (*chat.ChatResponse, error) {
	s.capturedMessages = messages
	s.capturedTemp = temperature
	return &chat.ChatResponse{Message: "ok"}, nil
}
func (s *stubLLMService) ChatStream(_ context.Context, _ []chat.ChatMessage, _ float64) (<-chan services.StreamChunk, error) {
	return nil, nil
}
func (s *stubLLMService) DeltaUpdate(_ context.Context, _ []chat.ChatMessage) (*conditionals.GameStateDelta, string, error) {
	return nil, "", nil
}

// stubStorage returns a preset GameState and Scenario; all writes are no-ops.
type stubStorage struct {
	gs *state.GameState
	sc *scenario.Scenario
}

func (s *stubStorage) Ping(_ context.Context) error { return nil }
func (s *stubStorage) Close() error                 { return nil }
func (s *stubStorage) SaveGameState(_ context.Context, _ uuid.UUID, _ *state.GameState) error {
	return nil
}
func (s *stubStorage) LoadGameState(_ context.Context, _ uuid.UUID) (*state.GameState, error) {
	return s.gs, nil
}
func (s *stubStorage) DeleteGameState(_ context.Context, _ uuid.UUID) error { return nil }
func (s *stubStorage) ListScenarios(_ context.Context) (map[string]string, error) {
	return nil, nil
}
func (s *stubStorage) GetScenario(_ context.Context, _ string) (*scenario.Scenario, error) {
	return s.sc, nil
}
func (s *stubStorage) GetNarrator(_ context.Context, _ string) (*scenario.Narrator, error) {
	return nil, nil
}
func (s *stubStorage) ListNarrators(_ context.Context) ([]string, error) { return nil, nil }
func (s *stubStorage) GetPCSpec(_ context.Context, _ string) (*actor.PCSpec, error) {
	return nil, nil
}
func (s *stubStorage) ListPCs(_ context.Context) ([]string, error) { return nil, nil }
func (s *stubStorage) GetMonster(_ context.Context, _ string) (*actor.Monster, error) {
	return nil, nil
}
func (s *stubStorage) ListMonsters(_ context.Context) (map[string]string, error) {
	return nil, nil
}
func (s *stubStorage) GetNPC(_ context.Context, _ string) (*actor.NPC, error) {
	return nil, nil
}
func (s *stubStorage) ListNPCs(_ context.Context) (map[string]string, error) {
	return nil, nil
}

// makeHistory returns n alternating user/assistant ChatMessages.
func makeHistory(n int) []chat.ChatMessage {
	msgs := make([]chat.ChatMessage, n)
	for i := range msgs {
		if i%2 == 0 {
			msgs[i] = chat.ChatMessage{Role: chat.ChatRoleUser, Content: fmt.Sprintf("user msg %d", i)}
		} else {
			msgs[i] = chat.ChatMessage{Role: chat.ChatRoleAgent, Content: fmt.Sprintf("assistant msg %d", i)}
		}
	}
	return msgs
}

// countNonSystem counts messages whose role is not ChatRoleSystem.
func countNonSystem(msgs []chat.ChatMessage) int {
	n := 0
	for _, m := range msgs {
		if m.Role != chat.ChatRoleSystem {
			n++
		}
	}
	return n
}

func newTestSetup(historyCount, historyLimit int) (*ChatProcessor, *stubLLMService, chat.ChatRequest) {
	gsID := uuid.New()
	gs := &state.GameState{
		ID:          gsID,
		Scenario:    "test.json",
		ChatHistory: makeHistory(historyCount),
		IsEnded:     true, // skip background syncGameState goroutine
		Vars:        make(map[string]string),
	}
	sc := &scenario.Scenario{
		Name:   "Test",
		Story:  "A test story",
		Rating: scenario.RatingPG,
	}
	llm := &stubLLMService{}
	stor := &stubStorage{gs: gs, sc: sc}
	processor := NewChatProcessor(stor, llm, nil, slog.Default(), historyLimit)
	req := chat.ChatRequest{GameStateID: gsID, Message: "hello"}
	return processor, llm, req
}

// TestProcessChatRequest_HistoryLimitRespected verifies that when ChatHistory contains
// more messages than the configured limit, only the limited number are sent to the LLM.
func TestProcessChatRequest_HistoryLimitRespected(t *testing.T) {
	const historyInState = 10 // messages stored in game state
	const limit = 4           // only last 4 should be forwarded

	processor, llm, req := newTestSetup(historyInState, limit)

	_, err := processor.ProcessChatRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("ProcessChatRequest returned error: %v", err)
	}

	// Non-system messages = limit history messages + 1 current user message
	want := limit + 1
	got := countNonSystem(llm.capturedMessages)
	if got != want {
		t.Errorf("expected %d non-system messages sent to LLM (limit %d + current user), got %d", want, limit, got)
	}
}

// TestProcessChatRequest_HistoryLimitZeroUsesDefault verifies that a zero limit
// falls back to PromptHistoryLimit.
func TestProcessChatRequest_HistoryLimitZeroUsesDefault(t *testing.T) {
	const historyInState = 20 // more than the default limit of 16

	processor, llm, req := newTestSetup(historyInState, 0) // 0 → default

	_, err := processor.ProcessChatRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("ProcessChatRequest returned error: %v", err)
	}

	want := PromptHistoryLimit + 1
	got := countNonSystem(llm.capturedMessages)
	if got != want {
		t.Errorf("expected %d non-system messages sent to LLM (default limit %d + current user), got %d", want, PromptHistoryLimit, got)
	}
}

// ---------------------------------------------------------------------------
// resolveTemperature unit tests
// ---------------------------------------------------------------------------

func TestResolveTemperature_DefaultFallback(t *testing.T) {
	gs := &state.GameState{}
	s := &scenario.Scenario{}
	temp := resolveTemperature(gs, s)
	if temp != services.DefaultTemperature {
		t.Errorf("expected default temperature %f, got %f", services.DefaultTemperature, temp)
	}
}

func TestResolveTemperature_ScenarioLevel(t *testing.T) {
	want := 0.9
	gs := &state.GameState{}
	s := &scenario.Scenario{Temperature: &want}
	temp := resolveTemperature(gs, s)
	if temp != want {
		t.Errorf("expected scenario temperature %f, got %f", want, temp)
	}
}

func TestResolveTemperature_SceneOverridesScenario(t *testing.T) {
	scenarioTemp := 0.9
	sceneTemp := 0.3
	gs := &state.GameState{SceneName: "act1"}
	s := &scenario.Scenario{
		Temperature: &scenarioTemp,
		Scenes: map[string]scenario.Scene{
			"act1": {Story: "Act 1", Temperature: &sceneTemp},
		},
	}
	temp := resolveTemperature(gs, s)
	if temp != sceneTemp {
		t.Errorf("expected scene temperature %f, got %f", sceneTemp, temp)
	}
}

func TestResolveTemperature_ScenarioUsedWhenSceneHasNoTemp(t *testing.T) {
	scenarioTemp := 0.9
	gs := &state.GameState{SceneName: "act1"}
	s := &scenario.Scenario{
		Temperature: &scenarioTemp,
		Scenes: map[string]scenario.Scene{
			"act1": {Story: "Act 1"},
		},
	}
	temp := resolveTemperature(gs, s)
	if temp != scenarioTemp {
		t.Errorf("expected scenario temperature %f, got %f", scenarioTemp, temp)
	}
}

// ---------------------------------------------------------------------------
// Temperature integration tests (verifies processor passes correct temp to LLM)
// ---------------------------------------------------------------------------

func TestProcessChatRequest_UsesDefaultTemperature(t *testing.T) {
	processor, llm, req := newTestSetup(2, 10)
	_, err := processor.ProcessChatRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("ProcessChatRequest returned error: %v", err)
	}
	if llm.capturedTemp != services.DefaultTemperature {
		t.Errorf("expected default temperature %f, got %f", services.DefaultTemperature, llm.capturedTemp)
	}
}

func TestProcessChatRequest_UsesScenarioTemperature(t *testing.T) {
	gsID := uuid.New()
	wantTemp := 0.9
	gs := &state.GameState{
		ID:          gsID,
		Scenario:    "test.json",
		ChatHistory: makeHistory(2),
		IsEnded:     true,
		Vars:        make(map[string]string),
	}
	sc := &scenario.Scenario{
		Name:        "Test",
		Story:       "A test story",
		Rating:      scenario.RatingPG,
		Temperature: &wantTemp,
	}
	llm := &stubLLMService{}
	stor := &stubStorage{gs: gs, sc: sc}
	processor := NewChatProcessor(stor, llm, nil, slog.Default(), 10)
	req := chat.ChatRequest{GameStateID: gsID, Message: "hello"}

	_, err := processor.ProcessChatRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("ProcessChatRequest returned error: %v", err)
	}
	if llm.capturedTemp != wantTemp {
		t.Errorf("expected scenario temperature %f, got %f", wantTemp, llm.capturedTemp)
	}
}

func TestProcessChatRequest_UsesSceneTemperature(t *testing.T) {
	gsID := uuid.New()
	scenarioTemp := 0.9
	sceneTemp := 0.3
	gs := &state.GameState{
		ID:          gsID,
		Scenario:    "test.json",
		SceneName:   "act1",
		ChatHistory: makeHistory(2),
		IsEnded:     true,
		Vars:        make(map[string]string),
	}
	sc := &scenario.Scenario{
		Name:        "Test",
		Story:       "A test story",
		Rating:      scenario.RatingPG,
		Temperature: &scenarioTemp,
		Scenes: map[string]scenario.Scene{
			"act1": {Story: "Act 1", Temperature: &sceneTemp},
		},
	}
	llm := &stubLLMService{}
	stor := &stubStorage{gs: gs, sc: sc}
	processor := NewChatProcessor(stor, llm, nil, slog.Default(), 10)
	req := chat.ChatRequest{GameStateID: gsID, Message: "hello"}

	_, err := processor.ProcessChatRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("ProcessChatRequest returned error: %v", err)
	}
	if llm.capturedTemp != sceneTemp {
		t.Errorf("expected scene temperature %f, got %f", sceneTemp, llm.capturedTemp)
	}
}
