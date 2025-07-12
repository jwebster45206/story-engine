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
	"testing"

	"github.com/jwebster45206/roleplay-agent/internal/services"
	"github.com/jwebster45206/roleplay-agent/pkg/chat"
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
			expectedError:  "Method not allowed. Only POST is supported.",
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
			expectedError:  "Message cannot be empty.",
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

			// Create chat handler
			handler := NewChatHandler(mockLLM, logger)

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
			req := httptest.NewRequest(tt.method, "/chat", bytes.NewBuffer(body))
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

			// Parse response
			var response chat.ChatResponse
			if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			// Check expected error
			if tt.expectedError != "" {
				if response.Error != tt.expectedError {
					t.Errorf("Expected error '%s', got '%s'", tt.expectedError, response.Error)
				}
			}

			// Check expected message
			if tt.expectedMsg != "" {
				if response.Message != tt.expectedMsg {
					t.Errorf("Expected message '%s', got '%s'", tt.expectedMsg, response.Message)
				}
			}

			// Verify mock calls for successful requests
			if tt.expectedStatus == http.StatusOK {
				if len(mockLLM.GenerateResponseCalls) != 1 {
					t.Errorf("Expected 1 GenerateResponse call, got %d", len(mockLLM.GenerateResponseCalls))
				} else {
					call := mockLLM.GenerateResponseCalls[0]
					if len(call.Messages) != 3 {
						t.Errorf("Expected 3 messages in call, got %d", len(call.Messages))
					} else {
						// Check system message
						if call.Messages[0].Role != chat.ChatRoleSystem {
							t.Errorf("Expected first message role %s, got %s", chat.ChatRoleSystem, call.Messages[0].Role)
						}
						// Check user message
						userMsg := call.Messages[1]
						if userMsg.Role != chat.ChatRoleUser {
							t.Errorf("Expected second message role %s, got %s", chat.ChatRoleUser, userMsg.Role)
						}
						if reqBody, ok := tt.body.(chat.ChatRequest); ok {
							if userMsg.Content != reqBody.Message {
								t.Errorf("Expected user message content '%s', got '%s'", reqBody.Message, userMsg.Content)
							}
						}
						// Check second system message
						if call.Messages[2].Role != chat.ChatRoleSystem {
							t.Errorf("Expected third message role %s, got %s", chat.ChatRoleSystem, call.Messages[2].Role)
						}
					}
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
	var capturedMessages []chat.ChatMessage

	mockLLM.GenerateResponseFunc = func(ctx context.Context, messages []chat.ChatMessage) (*chat.ChatResponse, error) {
		capturedMessages = messages
		return &chat.ChatResponse{Message: "Response"}, nil
	}

	handler := NewChatHandler(mockLLM, logger)

	requestBody := chat.ChatRequest{Message: "Test message with special chars: !@#$%"}
	body, _ := json.Marshal(requestBody)

	req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rr.Code)
	}

	if len(capturedMessages) != 3 {
		t.Fatalf("Expected 3 messages, got %d", len(capturedMessages))
	}

	// Check that the user message (second message) is correct
	userMsg := capturedMessages[1]
	if userMsg.Role != chat.ChatRoleUser {
		t.Errorf("Expected user message role %s, got %s", chat.ChatRoleUser, userMsg.Role)
	}

	if userMsg.Content != requestBody.Message {
		t.Errorf("Expected user message content '%s', got '%s'", requestBody.Message, userMsg.Content)
	}

	// Check that we have system messages
	if capturedMessages[0].Role != chat.ChatRoleSystem {
		t.Errorf("Expected first message to be system message, got %s", capturedMessages[0].Role)
	}
	if capturedMessages[2].Role != chat.ChatRoleSystem {
		t.Errorf("Expected third message to be system message, got %s", capturedMessages[2].Role)
	}
}

func TestChatHandler_ContentTypeHandling(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	mockLLM := services.NewMockLLMAPI()
	handler := NewChatHandler(mockLLM, logger)

	// Test with missing Content-Type
	requestBody := chat.ChatRequest{Message: "Hello"}
	body, _ := json.Marshal(requestBody)

	req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewBuffer(body))
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
