package chat

import (
	"testing"

	"github.com/google/uuid"
)

func TestFormatWithPCName(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		pcName   string
		expected string
	}{
		{
			name:     "adds PC name prefix to plain message",
			message:  "I swing my sword at the tree.",
			pcName:   "Korga",
			expected: "Korga: I swing my sword at the tree.",
		},
		{
			name:     "preserves existing speaker prefix",
			message:  "Narrator: The tree falls.",
			pcName:   "Korga",
			expected: "Narrator: The tree falls.",
		},
		{
			name:     "preserves PC's own name prefix",
			message:  "Korga: I examine the chest.",
			pcName:   "Korga",
			expected: "Korga: I examine the chest.",
		},
		{
			name:     "preserves different speaker prefix",
			message:  "Gandalf: You shall not pass!",
			pcName:   "Frodo",
			expected: "Gandalf: You shall not pass!",
		},
		{
			name:     "preserves colon in sentence (acceptable false positive)",
			message:  "I look at the map: it shows a path.",
			pcName:   "Aragorn",
			expected: "I look at the map: it shows a path.",
		},
		{
			name:     "handles empty message",
			message:  "",
			pcName:   "Legolas",
			expected: "Legolas: ",
		},
		{
			name:     "handles very long potential speaker name (over 50 chars)",
			message:  "This is a really really really really really long name: message",
			pcName:   "Gimli",
			expected: "Gimli: This is a really really really really really long name: message",
		},
		{
			name:     "adds prefix if speaker name has spaces",
			message:  "Captain Jack: Set sail!",
			pcName:   "Will",
			expected: "Captain Jack: Set sail!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatWithPCName(tt.message, tt.pcName)
			if result != tt.expected {
				t.Errorf("FormatWithPCName(%q, %q) = %q; want %q",
					tt.message, tt.pcName, result, tt.expected)
			}
		})
	}
}

func TestChatRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     ChatRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid short message",
			req: ChatRequest{
				Message:     "I attack the goblin.",
				GameStateID: mustParseUUID("550e8400-e29b-41d4-a716-446655440000"),
			},
			wantErr: false,
		},
		{
			name: "valid message at max length",
			req: ChatRequest{
				Message:     string(make([]byte, MaxMessageLength)),
				GameStateID: mustParseUUID("550e8400-e29b-41d4-a716-446655440000"),
			},
			wantErr: false,
		},
		{
			name: "message too long",
			req: ChatRequest{
				Message:     string(make([]byte, MaxMessageLength+1)),
				GameStateID: mustParseUUID("550e8400-e29b-41d4-a716-446655440000"),
			},
			wantErr: true,
			errMsg:  "exceeds maximum length",
		},
		{
			name: "empty message",
			req: ChatRequest{
				Message:     "",
				GameStateID: mustParseUUID("550e8400-e29b-41d4-a716-446655440000"),
			},
			wantErr: true,
			errMsg:  "cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func mustParseUUID(s string) uuid.UUID {
	u, err := uuid.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
