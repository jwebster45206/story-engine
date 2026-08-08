package chat

import (
	"encoding/json"
	"testing"
)

func TestToLLMMessagesOmitsStoryEventMetadata(t *testing.T) {
	msgs := []ChatMessage{
		{Role: ChatRoleUser, Content: "story prompt", IsStoryEvent: true},
		{Role: ChatRoleAgent, Content: "narrator reply"},
	}

	llmMsgs := ToLLMMessages(msgs)
	if len(llmMsgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(llmMsgs))
	}
	if llmMsgs[0].Role != ChatRoleUser || llmMsgs[0].Content != "story prompt" {
		t.Errorf("unexpected first message: %#v", llmMsgs[0])
	}

	data, err := json.Marshal(llmMsgs)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if string(data) != `[{"role":"user","content":"story prompt"},{"role":"assistant","content":"narrator reply"}]` {
		t.Errorf("unexpected JSON: %s", data)
	}
}
