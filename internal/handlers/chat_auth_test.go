package handlers

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/jwebster45206/story-engine/pkg/queue"
	"github.com/jwebster45206/story-engine/pkg/state"
	"github.com/jwebster45206/story-engine/pkg/storage"
)

type stubChatQueue struct {
	enqueued int
}

func (s *stubChatQueue) GetFormattedEvents(context.Context, uuid.UUID) (string, error) {
	return "", nil
}
func (s *stubChatQueue) Clear(context.Context, uuid.UUID) error { return nil }
func (s *stubChatQueue) EnqueueRequest(context.Context, *queue.Request) error {
	s.enqueued++
	return nil
}

func TestChatHandler_Ownership(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mockStorage := storage.NewMockStorage()
	gs := state.NewGameState("foo_scenario.json", nil, "foo", "foo_model")
	saveOwned(t, mockStorage, gs, testPrincipalA)

	q := &stubChatQueue{}
	handler := NewChatHandler(q, mockStorage, logger)
	priv, pub := testKeyPair(t)
	body := `{"gamestate_id":"` + gs.ID.String() + `","message":"look around"}`

	t.Run("wrong principal 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		serveJWT(t, handler, pub, rr, req, tokenFor(t, priv, testPrincipalB))
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 body=%s", rr.Code, rr.Body.String())
		}
		if q.enqueued != 0 {
			t.Fatal("must not enqueue for unauthorized chat")
		}
	})

	t.Run("owner accepted", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		serveJWT(t, handler, pub, rr, req, tokenFor(t, priv, testPrincipalA))
		if rr.Code != http.StatusAccepted {
			t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
		}
		if q.enqueued != 1 {
			t.Fatalf("enqueued = %d", q.enqueued)
		}
	})
}

func TestEventsHandler_OwnershipBeforeSSE(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mockStorage := storage.NewMockStorage()
	gs := state.NewGameState("foo_scenario.json", nil, "foo", "foo_model")
	saveOwned(t, mockStorage, gs, testPrincipalA)

	handler := NewEventsHandler(nil, mockStorage, logger)
	priv, pub := testKeyPair(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/events/gamestate/"+gs.ID.String(), nil)
	rr := httptest.NewRecorder()
	serveJWT(t, handler, pub, rr, req, tokenFor(t, priv, testPrincipalB))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); strings.Contains(ct, "text/event-stream") {
		t.Fatalf("Content-Type = %q; must not start SSE before authz", ct)
	}
}
