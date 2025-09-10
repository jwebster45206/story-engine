package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/internal/services"
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/state"
	"github.com/stretchr/testify/assert"
)

func TestChatHandler_ServeHTTP(t *testing.T) {
	// Create a logger for testing
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	tests := []struct {
		name           string
		method         string
		body           interface{}
		mockSetup      func(*services.MockLLMAPI)
		expectedStatus int
		expectedError  string
		expectedMsg    string
	}{
		{
			name:   "successful chat request",
			method: http.MethodPost,
			body:   chat.ChatRequest{Message: "Hello, world!"},
			mockSetup: func(m *services.MockLLMAPI) {
				m.GenerateResponseFunc = func(ctx context.Context, messages []chat.ChatMessage) (*chat.ChatResponse, error) {
					// Return valid JSON for meta extraction, otherwise normal test response
					promptPrefix := scenario.PromptStateExtractionInstructions
					if len(promptPrefix) > 50 {
						promptPrefix = promptPrefix[:50]
					}
					if len(messages) > 0 && messages[0].Role == chat.ChatRoleSystem && strings.HasPrefix(messages[0].Content, promptPrefix) {
						return &chat.ChatResponse{
							Message: `{"location":"Test Location","flags":{"test_flag":true},"inventory":["test item"],"npcs":{"TestNPC":{"name":"TestNPC","type":"test","disposition":"neutral","description":"A test NPC.","important":true}}}`,
						}, nil
					}
					return &chat.ChatResponse{Message: "Hello! How can I help you today?"}, nil
				}
			},
			expectedStatus: http.StatusOK,
			expectedMsg:    "Hello! How can I help you today?",
		},
		{
			name:           "method not allowed",
			method:         http.MethodGet,
			body:           nil,
			mockSetup:      func(m *services.MockLLMAPI) {},
			expectedStatus: http.StatusMethodNotAllowed,
			expectedError:  "Method not allowed. Only POST is supported at /v1/chat.",
		},
		{
			name:           "invalid JSON body",
			method:         http.MethodPost,
			body:           "invalid json",
			mockSetup:      func(m *services.MockLLMAPI) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid request body. Expected JSON with 'message' field.",
		},
		{
			name:           "empty message",
			method:         http.MethodPost,
			body:           chat.ChatRequest{Message: ""},
			mockSetup:      func(m *services.MockLLMAPI) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid request: message cannot be empty",
		},
		{
			name:   "LLM service error",
			method: http.MethodPost,
			body:   chat.ChatRequest{Message: "Hello"},
			mockSetup: func(m *services.MockLLMAPI) {
				m.SetGenerateResponseError(errors.New("LLM service unavailable"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "Failed to generate response. Please try again.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock LLM service
			mockLLM := services.NewMockLLMAPI()
			tt.mockSetup(mockLLM)

			mockSto := services.NewMockStorage()

			// For tests that need a valid GameStateID, create one
			var gameStateID uuid.UUID
			if tt.expectedStatus == http.StatusOK || tt.name == "LLM service error" {
				// Create a test game state
				testGS := state.NewGameState("foo_scenario.json", "foo_model")
				gameStateID = testGS.ID
				if err := mockSto.SaveGameState(context.Background(), testGS.ID, testGS); err != nil {
					t.Fatalf("Failed to save test game state: %v", err)
				}

				// Update the request body to include GameStateID
				if reqBody, ok := tt.body.(chat.ChatRequest); ok {
					reqBody.GameStateID = gameStateID
					tt.body = reqBody
				}
			}

			// Create chat handler
			handler := NewChatHandler(mockLLM, logger, mockSto)

			// Prepare request body
			var body []byte
			if tt.body != nil {
				if str, ok := tt.body.(string); ok {
					body = []byte(str)
				} else {
					var err error
					body, err = json.Marshal(tt.body)
					if err != nil {
						t.Fatalf("Failed to marshal request body: %v", err)
					}
				}
			}

			// Create HTTP request
			req := httptest.NewRequest(tt.method, "/v1/chat", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			// Create response recorder
			rr := httptest.NewRecorder()

			// Execute the handler
			handler.ServeHTTP(rr, req)

			// Check status code
			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			// Check content type
			if rr.Header().Get("Content-Type") != "application/json" {
				t.Errorf("Expected Content-Type application/json, got %s", rr.Header().Get("Content-Type"))
			}

			// Parse response based on expected status
			if tt.expectedError != "" {
				// For error cases, expect ErrorResponse
				var errorResponse ErrorResponse
				if err := json.NewDecoder(rr.Body).Decode(&errorResponse); err != nil {
					t.Fatalf("Failed to decode error response: %v", err)
				}

				if errorResponse.Error != tt.expectedError {
					t.Errorf("Expected error '%s', got '%s'", tt.expectedError, errorResponse.Error)
				}
			} else {
				// For success cases, expect ChatResponse
				var response chat.ChatResponse
				if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode chat response: %v", err)
				}

				// Check expected message
				if tt.expectedMsg != "" {
					if response.Message != tt.expectedMsg {
						t.Errorf("Expected message '%s', got '%s'", tt.expectedMsg, response.Message)
					}
				}
			}

			// Verify mock calls for successful requests
			if tt.expectedStatus == http.StatusOK {
				_, generateCalls := mockLLM.GetCalls()

				// Instead of checking for exactly 1 call, only count main chat calls
				mainPromptPrefix := scenario.BaseSystemPrompt
				if len(mainPromptPrefix) > 50 {
					mainPromptPrefix = mainPromptPrefix[:50]
				}
				mainCalls := 0
				for _, call := range generateCalls {
					if len(call.Messages) > 0 && strings.HasPrefix(call.Messages[0].Content, mainPromptPrefix) {
						mainCalls++
					}
				}
				if mainCalls != 1 {
					t.Errorf("Expected 1 main GenerateResponse call, got %d", mainCalls)
				}
			}
		})
	}
}

func TestChatHandler_MessageFormatting(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError, // Reduce noise in tests
	}))

	mockLLM := services.NewMockLLMAPI()
	var capturedMainChatMessages []chat.ChatMessage
	var mu sync.Mutex

	mockLLM.GenerateResponseFunc = func(ctx context.Context, messages []chat.ChatMessage) (*chat.ChatResponse, error) {
		// Return valid JSON for meta extraction, otherwise normal test response
		promptPrefix := scenario.PromptStateExtractionInstructions
		if len(promptPrefix) > 50 {
			promptPrefix = promptPrefix[:50]
		}
		if len(messages) > 0 && messages[0].Role == chat.ChatRoleSystem && strings.HasPrefix(messages[0].Content, promptPrefix) {
			return &chat.ChatResponse{
				Message: `{"location":"Test Location","flags":{"test_flag":true},"inventory":["test item"],"npcs":{"TestNPC":{"name":"TestNPC","type":"test","disposition":"neutral","description":"A test NPC.","important":true}}}`,
			}, nil
		}

		// This is the main chat call - capture its messages
		mu.Lock()
		capturedMainChatMessages = make([]chat.ChatMessage, len(messages))
		copy(capturedMainChatMessages, messages)
		mu.Unlock()

		return &chat.ChatResponse{Message: "Response"}, nil
	}
	mockSto := services.NewMockStorage()

	// Create a test game state
	testGS := state.NewGameState("foo_scenario.json", "foo_model")
	if err := mockSto.SaveGameState(context.Background(), testGS.ID, testGS); err != nil {
		t.Fatalf("Failed to save test game state: %v", err)
	}

	handler := NewChatHandler(mockLLM, logger, mockSto)
	requestBody := chat.ChatRequest{
		GameStateID: testGS.ID,
		Message:     "Test message with special chars: !@#$%",
	}
	body, _ := json.Marshal(requestBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rr.Code)
	}

	mu.Lock()
	capturedMessagesCopy := make([]chat.ChatMessage, len(capturedMainChatMessages))
	copy(capturedMessagesCopy, capturedMainChatMessages)
	mu.Unlock()

	if len(capturedMessagesCopy) != 3 {
		t.Fatalf("Expected 3 messages, got %d", len(capturedMessagesCopy))
	}

	// Check that the user message (second message) is correct
	userMsg := capturedMessagesCopy[1]
	if userMsg.Role != chat.ChatRoleUser {
		t.Errorf("Expected user message role %s, got %s", chat.ChatRoleUser, userMsg.Role)
	}

	if userMsg.Content != requestBody.Message {
		t.Errorf("Expected user message content '%s', got '%s'", requestBody.Message, userMsg.Content)
	}

	// Check that we have a consolidated system message first
	if capturedMessagesCopy[0].Role != chat.ChatRoleSystem {
		t.Errorf("Expected first message to be system message, got %s", capturedMessagesCopy[0].Role)
	}

	// Check that we have a final system message (UserPostPrompt)
	if capturedMessagesCopy[2].Role != chat.ChatRoleSystem {
		t.Errorf("Expected third message to be system message, got %s", capturedMessagesCopy[2].Role)
	}

	// Verify the system message contains all expected components
	systemContent := capturedMessagesCopy[0].Content
	if !strings.Contains(systemContent, "omniscient narrator") {
		t.Errorf("Expected system message to contain base system prompt")
	}
	if !strings.Contains(systemContent, "Game State:") {
		t.Errorf("Expected system message to contain game state JSON")
	}

	// Verify the final system message contains the UserPostPrompt
	finalSystemContent := capturedMessagesCopy[2].Content
	if !strings.Contains(finalSystemContent, "Treat the user's message as a request") {
		t.Errorf("Expected final system message to contain user post prompt")
	}
}

func TestChatHandler_ContentTypeHandling(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	mockLLM := services.NewMockLLMAPI()
	mockSto := services.NewMockStorage()

	// Create a test game state
	testGS := state.NewGameState("foo_scenario.json", "foo_model")
	if err := mockSto.SaveGameState(context.Background(), testGS.ID, testGS); err != nil {
		t.Fatalf("Failed to save test game state: %v", err)
	}

	handler := NewChatHandler(mockLLM, logger, mockSto)

	// Test with missing Content-Type
	requestBody := chat.ChatRequest{
		GameStateID: testGS.ID,
		Message:     "Hello",
	}
	body, _ := json.Marshal(requestBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat", bytes.NewBuffer(body))
	// Intentionally not setting Content-Type

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Should still work since Go's JSON decoder is forgiving
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 even without Content-Type, got %d", rr.Code)
	}

	// Verify response has correct Content-Type
	if rr.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Expected response Content-Type application/json, got %s", rr.Header().Get("Content-Type"))
	}
}

func TestApplyMetaUpdate(t *testing.T) {
	// Prepare initial game state
	loc1 := scenario.Location{Name: "Tavern", Items: []string{"Apple", "Shield"}}
	loc2 := scenario.Location{Name: "Forest", Items: []string{}}
	// npc := scenario.NPC{Name: "Goblin", Items: []string{"Dagger"}}

	gs := &state.GameState{
		ID:             uuid.New(),
		Location:       "Tavern",
		Inventory:      []string{"Gold"},
		WorldLocations: map[string]scenario.Location{"Tavern": loc1, "Forest": loc2},
		// You can include NPCs if you implement the TODO NPC logic later
	}

	s := &scenario.Scenario{
		Name: "Test Scenario",
		Locations: map[string]scenario.Location{
			"Tavern": loc1,
			"Forest": loc2,
		},
		NPCs: map[string]scenario.NPC{
			"TestNPC": {Name: "TestNPC", Type: "human", Disposition: "neutral"},
		},
	}

	meta := &state.GameStateDelta{
		UserLocation: "Forest",
		ItemEvents: []struct {
			Item   string `json:"item"`
			Action string `json:"action"` // enum "acquire" | "give" | "drop" | "move" | "use"
			From   *struct {
				Type string `json:"type"` // enum "player" | "npc" | "location"
				Name string `json:"name,omitempty"`
			} `json:"from,omitempty"`
			To *struct {
				Type string `json:"type"` // enum "player" | "npc" | "location"
				Name string `json:"name,omitempty"`
			} `json:"to,omitempty"`
			Consumed *bool `json:"consumed,omitempty"`
		}{
			{
				Item:   "Apple",
				Action: "acquire",
				From: &struct {
					Type string `json:"type"`
					Name string `json:"name,omitempty"`
				}{
					Type: "location",
					Name: "Tavern",
				},
			},
			{
				Item:   "Gold",
				Action: "drop",
				To: &struct {
					Type string `json:"type"`
					Name string `json:"name,omitempty"`
				}{
					Type: "location",
					Name: "Tavern",
				},
			},
		},
		SetVars: map[string]string{
			"test Var":    "value1",
			"another_var": "value2",
		},
	}

	err := applyGameStateDelta(gs, s, meta)
	if err != nil {
		t.Fatalf("Failed to apply gamestate delta: %v", err)
	}

	// Assertions
	assert.Equal(t, "Forest", gs.Location, "user location should be updated")
	assert.Contains(t, gs.Inventory, "Apple", "new item should be added to inventory")
	assert.NotContains(t, gs.Inventory, "Gold", "removed item should not be in inventory")
	tavern := gs.WorldLocations["Tavern"]
	assert.NotContains(t, tavern.Items, "Apple", "item should be removed from Tavern")
	assert.Contains(t, tavern.Items, "Gold", "new item should be added to Tavern items")

	// SetVars assertions
	if assert.NotNil(t, gs.Vars, "Vars map should be initialized") {
		assert.Equal(t, "value1", gs.Vars["test_var"], "test Var should be upserted as test_var")
		assert.Equal(t, "value2", gs.Vars["another_var"], "another var should be upserted as another_var")
	}
}
