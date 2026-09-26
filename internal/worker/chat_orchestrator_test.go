package worker

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"uuid"

	"github.com/jwebster45206/d20"
	"github.com/jwebster45206/story-engine/internal/llm"
	"github.com/jwebster45206/story-engine/pkg/character"
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/conditionals"
	"github.com/jwebster45206/story-engine/pkg/queue"
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

	processor := &ChatOrchestrator{logger: logger}
	applier := state.NewApplier(gs, delta, s, logger)

	// Execute
	processor.applyConditionalsCascade(applier, gs.ID)

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

	processor := &ChatOrchestrator{logger: logger}
	applier := state.NewApplier(gs, delta, s, logger)

	// Execute
	processor.applyConditionalsCascade(applier, gs.ID)

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

	processor := &ChatOrchestrator{logger: logger}
	applier := state.NewApplier(gs, delta, s, logger)

	// Execute
	processor.applyConditionalsCascade(applier, gs.ID)

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

// stubLLMService captures the messages slice passed to ChatStream() and no-ops everything else.
type stubResolver struct{ llm llm.LLMService }

func (s stubResolver) Get(_ string) (llm.LLMService, error) { return s.llm, nil }

type stubLLMService struct {
	capturedMessages []chat.ChatMessage
	capturedTemp     float64
	calls            []string
	rulingMessages   []chat.ChatMessage
	ruling           *chat.Ruling
	rulingErr        error
	rulingUsage      llm.Usage
	delta            *conditionals.GameStateDelta
	deltaErr         error
}

func (s *stubLLMService) ChatStream(_ context.Context, messages []chat.ChatMessage, temperature float64) (<-chan llm.StreamChunk, error) {
	s.calls = append(s.calls, "stream")
	s.capturedMessages = messages
	s.capturedTemp = temperature
	ch := make(chan llm.StreamChunk)
	close(ch)
	return ch, nil
}
func (s *stubLLMService) DeltaUpdate(_ context.Context, _ []chat.ChatMessage) (*conditionals.GameStateDelta, llm.Usage, error) {
	s.calls = append(s.calls, "delta")
	if s.deltaErr != nil {
		return nil, llm.Usage{}, s.deltaErr
	}
	if s.delta != nil {
		return s.delta, llm.Usage{Model: "stub-reducer"}, nil
	}
	return nil, llm.Usage{}, nil
}
func (s *stubLLMService) GetRuling(_ context.Context, messages []chat.ChatMessage) (*chat.Ruling, llm.Usage, error) {
	s.calls = append(s.calls, "ruling")
	s.rulingMessages = messages
	if s.rulingErr != nil {
		return nil, s.rulingUsage, s.rulingErr
	}
	if s.ruling != nil || s.rulingUsage.InputTokens > 0 {
		return s.ruling, s.rulingUsage, nil
	}
	return &chat.Ruling{Allowed: true, Reasoning: "allowed"}, llm.Usage{InputTokens: 2, OutputTokens: 1, Model: "stub-adj"}, nil
}

// stubStorage returns a preset GameState and Scenario; all writes are no-ops.
type stubStorage struct {
	gs *state.GameState
	sc *scenario.Scenario
}

func (s *stubStorage) Ping(_ context.Context) error { return nil }
func (s *stubStorage) Close() error                 { return nil }
func (s *stubStorage) CreateGameState(_ context.Context, _ uuid.UUID, _ *state.GameState) error {
	return nil
}
func (s *stubStorage) UpdateGameState(_ context.Context, _ uuid.UUID, _ *state.GameState) error {
	return nil
}
func (s *stubStorage) LoadGameState(_ context.Context, _ uuid.UUID) (*state.GameState, error) {
	return s.gs, nil
}
func (s *stubStorage) DeleteGameState(_ context.Context, _ uuid.UUID) error { return nil }
func (s *stubStorage) GetOwner(_ context.Context, _ uuid.UUID) (uuid.UUID, error) {
	return uuid.Nil(), nil
}
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
func (s *stubStorage) GetPC(_ context.Context, _ string) (*character.PC, error) {
	return nil, nil
}
func (s *stubStorage) ListPCs(_ context.Context) ([]string, error) { return nil, nil }
func (s *stubStorage) GetMonster(_ context.Context, _ string) (*character.Monster, error) {
	return nil, nil
}
func (s *stubStorage) ListMonsters(_ context.Context) (map[string]string, error) {
	return nil, nil
}
func (s *stubStorage) GetNPC(_ context.Context, _ string) (*character.NPC, error) {
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

// lastUserContent returns the content of the last user-role message.
func lastUserContent(msgs []chat.ChatMessage) string {
	for _, msg := range slices.Backward(msgs) {
		if msg.Role == chat.ChatRoleUser {
			return msg.Content
		}
	}
	return ""
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

func newTestSetup(historyCount, historyLimit int) (*ChatOrchestrator, *stubLLMService, chat.ChatRequest) {
	gsID := uuid.New()
	gs := &state.GameState{
		ID:          gsID,
		Scenario:    "test.json",
		Rules:       state.RulesStrict,
		Temperature: state.DefaultTemperature,
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
	processor := NewChatOrchestrator(stor, stubResolver{llm}, nil, slog.Default(), historyLimit)
	req := chat.ChatRequest{GameStateID: gsID, Message: "hello"}
	return processor, llm, req
}

// TestProcessChatStream_HistoryLimitRespected verifies that when ChatHistory contains
// more messages than the configured limit, only the limited number are sent to the LLM.
func TestProcessChatStream_HistoryLimitRespected(t *testing.T) {
	const historyInState = 10 // messages stored in game state
	const limit = 4           // only last 4 should be forwarded

	processor, llm, req := newTestSetup(historyInState, limit)

	_, err := processor.ProcessChatStream(context.Background(), req)
	if err != nil {
		t.Fatalf("ProcessChatStream returned error: %v", err)
	}

	// Non-system messages = limit history messages + 1 current user message
	want := limit + 1
	got := countNonSystem(llm.capturedMessages)
	if got != want {
		t.Errorf("expected %d non-system messages sent to LLM (limit %d + current user), got %d", want, limit, got)
	}
}

// TestProcessChatStream_HistoryLimitZeroUsesDefault verifies that a zero limit
// falls back to PromptHistoryLimit.
func TestProcessChatStream_HistoryLimitZeroUsesDefault(t *testing.T) {
	const historyInState = 20 // more than the default limit of 16

	processor, llm, req := newTestSetup(historyInState, 0) // 0 → default

	_, err := processor.ProcessChatStream(context.Background(), req)
	if err != nil {
		t.Fatalf("ProcessChatStream returned error: %v", err)
	}

	want := PromptHistoryLimit + 1
	got := countNonSystem(llm.capturedMessages)
	if got != want {
		t.Errorf("expected %d non-system messages sent to LLM (default limit %d + current user), got %d", want, PromptHistoryLimit, got)
	}
}

// ---------------------------------------------------------------------------
// Temperature via GameState (verifies processor passes gs temperature to LLM)
// ---------------------------------------------------------------------------

func TestProcessChatStream_UsesDefaultTemperature(t *testing.T) {
	processor, stub, req := newTestSetup(2, 10)
	_, err := processor.ProcessChatStream(context.Background(), req)
	if err != nil {
		t.Fatalf("ProcessChatStream returned error: %v", err)
	}
	if stub.capturedTemp != state.DefaultTemperature {
		t.Errorf("expected default temperature %f, got %f", state.DefaultTemperature, stub.capturedTemp)
	}
}

func TestProcessChatStream_UsesGameStateTemperature(t *testing.T) {
	gsID := uuid.New()
	wantTemp := 0.9
	gs := &state.GameState{
		ID:          gsID,
		Scenario:    "test.json",
		Temperature: wantTemp,
		ChatHistory: makeHistory(2),
		IsEnded:     true,
		Vars:        make(map[string]string),
	}
	sc := &scenario.Scenario{
		Name:   "Test",
		Story:  "A test story",
		Rating: scenario.RatingPG,
	}
	llm := &stubLLMService{}
	stor := &stubStorage{gs: gs, sc: sc}
	processor := NewChatOrchestrator(stor, stubResolver{llm}, nil, slog.Default(), 10)
	req := chat.ChatRequest{GameStateID: gsID, Message: "hello"}

	_, err := processor.ProcessChatStream(context.Background(), req)
	if err != nil {
		t.Fatalf("ProcessChatStream returned error: %v", err)
	}
	if llm.capturedTemp != wantTemp {
		t.Errorf("expected gamestate temperature %f, got %f", wantTemp, llm.capturedTemp)
	}
}

func TestProcessChatStream_RefereeInjectedAfterRules(t *testing.T) {
	gsID := uuid.New()
	gs := &state.GameState{
		ID:          gsID,
		Scenario:    "test.json",
		Rules:       state.RulesStrict,
		Temperature: state.DefaultTemperature,
		ChatHistory: makeHistory(6),
		IsEnded:     true,
		Vars:        make(map[string]string),
		Location:    "tavern",
		WorldLocations: map[string]scenario.Location{
			"tavern": {Name: "The Tavern", Description: "A smoky taproom."},
		},
	}
	sc := &scenario.Scenario{Name: "Test", Story: "A test story", Rating: scenario.RatingPG}
	stub := &stubLLMService{
		ruling: &chat.Ruling{
			Allowed:   false,
			Reasoning: "There is no north exit from The Tavern.",
		},
		rulingUsage: llm.Usage{InputTokens: 4, OutputTokens: 2, Model: "stub-adj", Vendor: "stub"},
	}
	stor := &stubStorage{gs: gs, sc: sc}
	processor := NewChatOrchestrator(stor, stubResolver{stub}, nil, slog.Default(), 10)
	req := chat.ChatRequest{GameStateID: gsID, Message: "I walk north", UseReferee: true}

	_, err := processor.ProcessChatStream(context.Background(), req)
	if err != nil {
		t.Fatalf("ProcessChatStream: %v", err)
	}
	if len(stub.calls) < 2 || stub.calls[0] != "ruling" || stub.calls[1] != "stream" {
		t.Fatalf("call order = %v, want ruling then stream", stub.calls)
	}
	user := lastUserContent(stub.capturedMessages)
	rulesIdx := strings.Index(user, "<rules>")
	refIdx := strings.Index(user, "<referee>")
	if rulesIdx < 0 || refIdx < 0 {
		t.Fatalf("expected rules and referee on narrator user turn, got %q", user)
	}
	if refIdx < rulesIdx {
		t.Fatal("referee block should follow <rules>")
	}
	wantText := stub.ruling.NarratorText()
	if !strings.Contains(user, wantText) {
		t.Errorf("narrator user turn missing ruling: %q", user)
	}
	if len(stub.rulingMessages) == 0 {
		t.Fatal("expected GetRuling messages")
	}
	refUser := stub.rulingMessages[len(stub.rulingMessages)-1].Content
	if strings.Contains(refUser, "<rules>") {
		t.Error("referee user turn should not include <rules>")
	}
	refSys := stub.rulingMessages[0].Content
	if !strings.Contains(refSys, "<world_state>") {
		t.Error("referee system prompt should include world_state")
	}
}

func TestProcessChatStream_RefereeFailOpenOmitsBlock(t *testing.T) {
	gsID := uuid.New()
	gs := &state.GameState{
		ID:          gsID,
		Scenario:    "test.json",
		Temperature: state.DefaultTemperature,
		IsEnded:     true,
		Vars:        make(map[string]string),
	}
	sc := &scenario.Scenario{Name: "Test", Story: "A test story", Rating: scenario.RatingPG}
	stub := &stubLLMService{rulingErr: fmt.Errorf("referee down")}
	processor := NewChatOrchestrator(&stubStorage{gs: gs, sc: sc}, stubResolver{stub}, nil, slog.Default(), 10)
	req := chat.ChatRequest{GameStateID: gsID, Message: "hello", UseReferee: true}

	_, err := processor.ProcessChatStream(context.Background(), req)
	if err != nil {
		t.Fatalf("ProcessChatStream: %v", err)
	}
	user := lastUserContent(stub.capturedMessages)
	if strings.Contains(user, "<referee>") {
		t.Errorf("fail-open should omit referee block, got %q", user)
	}
	if !strings.Contains(user, "<rules>") {
		t.Error("rules block should remain")
	}
}

func TestProcessChatStream_StoryEventSkipsReferee(t *testing.T) {
	processor, stub, req := newTestSetup(2, 10)

	_, err := processor.ProcessChatStream(context.Background(), req)
	if err != nil {
		t.Fatalf("ProcessChatStream: %v", err)
	}
	for _, c := range stub.calls {
		if c == "ruling" {
			t.Fatal("turns without UseReferee should not call GetRuling")
		}
	}
	user := lastUserContent(stub.capturedMessages)
	if strings.Contains(user, "<referee>") {
		t.Errorf("story event should not inject referee block, got %q", user)
	}
}

func TestProcessChatStream_CombatAppendsStrikeLines(t *testing.T) {
	gsID := uuid.New()
	gs := &state.GameState{
		ID:          gsID,
		Scenario:    "test.json",
		Rules:       state.RulesStrict,
		Temperature: state.DefaultTemperature,
		IsEnded:     true,
		Vars:        make(map[string]string),
		Location:    "tavern",
		PC:          &character.PC{Name: "Felix"},
		WorldLocations: map[string]scenario.Location{
			"tavern": {Name: "The Tavern", Description: "A smoky taproom."},
		},
	}
	sc := &scenario.Scenario{Name: "Test", Story: "A test story", Rating: scenario.RatingPG}
	stub := &stubLLMService{
		ruling: &chat.Ruling{
			Allowed:   true,
			Reasoning: "The PC can strike the Giant Rat.",
			Scope:     chat.RulingScopeCombat,
			Object:    "Giant Rat",
		},
	}
	const seed int64 = 1
	processor := NewChatOrchestrator(&stubStorage{gs: gs, sc: sc}, stubResolver{stub}, nil, slog.Default(), 10)
	processor.roller = d20.NewRoller(seed)
	req := chat.ChatRequest{GameStateID: gsID, Message: "I attack the giant rat", UseReferee: true}

	out, err := processor.ProcessChatStream(context.Background(), req)
	if err != nil {
		t.Fatalf("ProcessChatStream: %v", err)
	}
	attempts := out.Attempts
	if len(attempts) != 1 || !strings.HasPrefix(attempts[0].Content, "Felix rolled ") {
		t.Fatalf("expected named roll content, got %+v", attempts)
	}
	if strings.Contains(strings.ToLower(attempts[0].NarratorText), "rolled") || strings.Contains(attempts[0].NarratorText, "DC") {
		t.Errorf("narrator line must not include dice mechanics, got %q", attempts[0].NarratorText)
	}
	user := lastUserContent(stub.capturedMessages)
	wantLines := expectedStrikes(t, seed, "Felix", "Giant Rat")
	if !strings.Contains(user, stub.ruling.NarratorText()) {
		t.Errorf("missing ruling prose, got %q", user)
	}
	for _, line := range wantLines {
		if !strings.Contains(user, line) {
			t.Errorf("missing strike line %q in %q", line, user)
		}
	}
	if strings.Contains(user, "Giant Rat strikes") {
		t.Errorf("should not include a reaction strike, got %q", user)
	}
	if !strings.Contains(user, "strikes") {
		t.Errorf("expected strike prose, got %q", user)
	}
	if strings.Contains(strings.ToLower(user), "rolled") || strings.Contains(user, "DC") {
		t.Errorf("narrator must not include dice mechanics, got %q", user)
	}
}

func TestProcessChatStream_DeniedCombatOmitsStrikes(t *testing.T) {
	gsID := uuid.New()
	gs := &state.GameState{
		ID:          gsID,
		Scenario:    "test.json",
		Temperature: state.DefaultTemperature,
		IsEnded:     true,
		Vars:        make(map[string]string),
		PC:          &character.PC{Name: "Felix"},
	}
	sc := &scenario.Scenario{Name: "Test", Story: "A test story", Rating: scenario.RatingPG}
	stub := &stubLLMService{
		ruling: &chat.Ruling{
			Allowed:   false,
			Reasoning: "There is no one to fight.",
			Scope:     chat.RulingScopeCombat,
			Object:    "Giant Rat",
		},
	}
	processor := NewChatOrchestrator(&stubStorage{gs: gs, sc: sc}, stubResolver{stub}, nil, slog.Default(), 10)
	req := chat.ChatRequest{GameStateID: gsID, Message: "I attack", UseReferee: true}

	out, err := processor.ProcessChatStream(context.Background(), req)
	if err != nil {
		t.Fatalf("ProcessChatStream: %v", err)
	}
	attempts := out.Attempts
	if len(attempts) != 1 || attempts[0].Content != "There is no one to fight." || attempts[0].Success || attempts[0].Scope != attemptScopeAll || attempts[0].NarratorText != "" {
		t.Errorf("deny attempt = %+v", attempts)
	}
	user := lastUserContent(stub.capturedMessages)
	if strings.Contains(user, "strikes") {
		t.Errorf("denied combat should not add strike lines, got %q", user)
	}
	if !strings.Contains(user, stub.ruling.NarratorText()) {
		t.Errorf("missing ruling prose, got %q", user)
	}
}

func TestProcessChatStream_AttachedRulingSkipsReferee(t *testing.T) {
	processor, stub, req := newTestSetup(2, 10)
	processor.roller = d20.NewRoller(1)
	req.Message = "Giant Rat strikes Felix."
	req.Ruling = &chat.Ruling{
		Allowed: true,
		Scope:   chat.RulingScopeCombat,
		Subject: "Giant Rat",
		Object:  "Felix",
	}

	out, err := processor.ProcessChatStream(context.Background(), req)
	if err != nil {
		t.Fatalf("ProcessChatStream: %v", err)
	}
	for _, c := range stub.calls {
		if c == "ruling" {
			t.Fatal("attached ruling should skip GetRuling")
		}
	}
	ruling := out.Ruling
	if ruling == nil || ruling.Subject != "Giant Rat" || ruling.Object != "Felix" {
		t.Fatalf("ruling = %#v", ruling)
	}
	want := expectedStrikes(t, 1, "Giant Rat", "Felix")
	if !slices.Equal(narratorTexts(out.Attempts), want) {
		t.Errorf("got %q, want %q", narratorTexts(out.Attempts), want)
	}
	user := lastUserContent(stub.capturedMessages)
	if !strings.Contains(user, want[0]) {
		t.Errorf("missing reaction strike %q in %q", want[0], user)
	}
	if pendingCombatReaction(&state.GameState{PC: &character.PC{Name: "Felix"}}, ruling) != nil {
		t.Error("reaction should not chain another event")
	}
}

type recordingQueue struct {
	mu   sync.Mutex
	reqs []*queue.Request
}

func (q *recordingQueue) GetFormattedEvents(context.Context, uuid.UUID) (string, error) {
	return "", nil
}
func (q *recordingQueue) Clear(context.Context, uuid.UUID) error { return nil }
func (q *recordingQueue) EnqueueRequest(_ context.Context, req *queue.Request) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	copied := *req
	if req.Ruling != nil {
		r := *req.Ruling
		copied.Ruling = &r
	}
	q.reqs = append(q.reqs, &copied)
	return nil
}
func (q *recordingQueue) all() []*queue.Request {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]*queue.Request, len(q.reqs))
	copy(out, q.reqs)
	return out
}

func tavernCombatFixture() (*state.GameState, *scenario.Scenario, *character.Monster) {
	rat := &character.Monster{ID: "rat_1", Name: "Giant Rat", HP: 5, MaxHP: 5}
	gs := &state.GameState{
		ID:       uuid.New(),
		Scenario: "test.json",
		Location: "tavern",
		PC:       &character.PC{Name: "Felix"},
		WorldLocations: map[string]scenario.Location{
			"tavern": {Name: "The Tavern", Monsters: map[string]*character.Monster{"rat_1": rat}},
		},
		Vars: make(map[string]string),
	}
	sc := &scenario.Scenario{Name: "Test", Story: "A test story", Rating: scenario.RatingPG}
	return gs, sc, rat
}

func TestEnqueueReaction_AfterSync(t *testing.T) {
	gs, sc, _ := tavernCombatFixture()
	q := &recordingQueue{}
	stub := &stubLLMService{delta: &conditionals.GameStateDelta{}}
	p := NewChatOrchestrator(&stubStorage{gs: gs, sc: sc}, stubResolver{stub}, q, slog.Default(), 10)
	player := &chat.Ruling{Allowed: true, Scope: chat.RulingScopeCombat, Object: "Giant Rat"}
	latest := p.syncGameState(context.Background(), gs, "Felix strikes the rat.")
	if err := p.enqueueReaction(context.Background(), latest, pendingCombatReaction(gs, player)); err != nil {
		t.Fatalf("enqueueReaction: %v", err)
	}
	got := q.all()
	if len(got) != 1 {
		t.Fatalf("enqueued %d, want 1", len(got))
	}
	req := got[0]
	if req.Type != queue.RequestTypeStoryEvent || req.EventPrompt != "Giant Rat strikes Felix." {
		t.Errorf("request = %#v", req)
	}
	if req.Ruling == nil || req.Ruling.Subject != "Giant Rat" || req.Ruling.Object != "Felix" || !req.Ruling.Allowed || req.Ruling.Scope != chat.RulingScopeCombat {
		t.Errorf("ruling = %#v", req.Ruling)
	}
}

func TestEnqueueReaction_NamesAction(t *testing.T) {
	gs, sc, rat := tavernCombatFixture()
	rat.Actions = map[string]character.Action{
		"shortbow": {Name: "Shortbow", Type: "attack", Attempt: "1d20+4"},
		"bite":     {Name: "Bite", Type: "attack", Attempt: "1d20+2"},
	}
	q := &recordingQueue{}
	stub := &stubLLMService{delta: &conditionals.GameStateDelta{}}
	p := NewChatOrchestrator(&stubStorage{gs: gs, sc: sc}, stubResolver{stub}, q, slog.Default(), 10)
	player := &chat.Ruling{Allowed: true, Scope: chat.RulingScopeCombat, Object: "Giant Rat"}
	latest := p.syncGameState(context.Background(), gs, "Felix strikes the rat.")
	if err := p.enqueueReaction(context.Background(), latest, pendingCombatReaction(gs, player)); err != nil {
		t.Fatalf("enqueueReaction: %v", err)
	}
	got := q.all()
	if len(got) != 1 {
		t.Fatalf("enqueued %d, want 1", len(got))
	}
	req := got[0]
	if req.EventPrompt != "Giant Rat strikes Felix with Bite." {
		t.Errorf("EventPrompt = %q", req.EventPrompt)
	}
	if req.Ruling == nil || req.Ruling.ActionID != "bite" {
		t.Errorf("ruling = %#v", req.Ruling)
	}
}

func TestEnqueueReaction_Skips(t *testing.T) {
	player := &chat.Ruling{Allowed: true, Scope: chat.RulingScopeCombat, Object: "Giant Rat"}

	t.Run("reducer failure", func(t *testing.T) {
		gs, sc, _ := tavernCombatFixture()
		q := &recordingQueue{}
		stub := &stubLLMService{deltaErr: fmt.Errorf("reducer down")}
		p := NewChatOrchestrator(&stubStorage{gs: gs, sc: sc}, stubResolver{stub}, q, slog.Default(), 10)
		latest := p.syncGameState(context.Background(), gs, "strike")
		if err := p.enqueueReaction(context.Background(), latest, pendingCombatReaction(gs, player)); err != nil {
			t.Fatalf("enqueueReaction: %v", err)
		}
		if n := len(q.all()); n != 0 {
			t.Fatalf("enqueued %d, want 0", n)
		}
	})
	t.Run("canceled", func(t *testing.T) {
		gs, sc, _ := tavernCombatFixture()
		q := &recordingQueue{}
		stub := &stubLLMService{delta: &conditionals.GameStateDelta{}}
		p := NewChatOrchestrator(&stubStorage{gs: gs, sc: sc}, stubResolver{stub}, q, slog.Default(), 10)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		latest := p.syncGameState(ctx, gs, "strike")
		if err := p.enqueueReaction(ctx, latest, pendingCombatReaction(gs, player)); err != nil && ctx.Err() == nil {
			t.Fatalf("enqueueReaction: %v", err)
		}
		if n := len(q.all()); n != 0 {
			t.Fatalf("enqueued %d, want 0", n)
		}
	})
	t.Run("actor gone", func(t *testing.T) {
		gs, sc, _ := tavernCombatFixture()
		gs.WorldLocations["tavern"] = scenario.Location{Name: "The Tavern"}
		q := &recordingQueue{}
		stub := &stubLLMService{delta: &conditionals.GameStateDelta{}}
		p := NewChatOrchestrator(&stubStorage{gs: gs, sc: sc}, stubResolver{stub}, q, slog.Default(), 10)
		latest := p.syncGameState(context.Background(), gs, "strike")
		if err := p.enqueueReaction(context.Background(), latest, pendingCombatReaction(gs, player)); err != nil {
			t.Fatalf("enqueueReaction: %v", err)
		}
		if n := len(q.all()); n != 0 {
			t.Fatalf("enqueued %d, want 0", n)
		}
	})
	t.Run("game ended", func(t *testing.T) {
		gs, sc, _ := tavernCombatFixture()
		gs.IsEnded = true
		q := &recordingQueue{}
		stub := &stubLLMService{delta: &conditionals.GameStateDelta{}}
		p := NewChatOrchestrator(&stubStorage{gs: gs, sc: sc}, stubResolver{stub}, q, slog.Default(), 10)
		latest := p.syncGameState(context.Background(), gs, "strike")
		if err := p.enqueueReaction(context.Background(), latest, pendingCombatReaction(gs, player)); err != nil {
			t.Fatalf("enqueueReaction: %v", err)
		}
		if n := len(q.all()); n != 0 {
			t.Fatalf("enqueued %d, want 0", n)
		}
	})
	t.Run("non-PC subject", func(t *testing.T) {
		gs, sc, _ := tavernCombatFixture()
		q := &recordingQueue{}
		stub := &stubLLMService{delta: &conditionals.GameStateDelta{}}
		p := NewChatOrchestrator(&stubStorage{gs: gs, sc: sc}, stubResolver{stub}, q, slog.Default(), 10)
		reaction := &chat.Ruling{Allowed: true, Scope: chat.RulingScopeCombat, Subject: "Giant Rat", Object: "Felix"}
		latest := p.syncGameState(context.Background(), gs, "strike")
		if err := p.enqueueReaction(context.Background(), latest, pendingCombatReaction(gs, reaction)); err != nil {
			t.Fatalf("enqueueReaction: %v", err)
		}
		if n := len(q.all()); n != 0 {
			t.Fatalf("enqueued %d, want 0", n)
		}
	})
	t.Run("story event passes nil ruling", func(t *testing.T) {
		gs, sc, _ := tavernCombatFixture()
		q := &recordingQueue{}
		stub := &stubLLMService{delta: &conditionals.GameStateDelta{}}
		p := NewChatOrchestrator(&stubStorage{gs: gs, sc: sc}, stubResolver{stub}, q, slog.Default(), 10)
		latest := p.syncGameState(context.Background(), gs, "strike")
		if err := p.enqueueReaction(context.Background(), latest, pendingCombatReaction(gs, nil)); err != nil {
			t.Fatalf("enqueueReaction: %v", err)
		}
		if n := len(q.all()); n != 0 {
			t.Fatalf("enqueued %d, want 0", n)
		}
	})
}
