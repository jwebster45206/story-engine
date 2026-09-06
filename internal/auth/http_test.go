package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"uuid"

	"github.com/jwebster45206/story-engine/pkg/state"
	"github.com/jwebster45206/story-engine/pkg/storage"
)

func TestRequirePrincipal(t *testing.T) {
	id := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	tests := []struct {
		name    string
		with    bool
		wantErr error
	}{
		{name: "missing", wantErr: ErrUnauthorized},
		{name: "present", with: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/gamestate", nil)
			if tt.with {
				req = req.WithContext(WithPrincipal(req.Context(), Principal{ID: id}))
			}
			p, err := RequirePrincipal(req)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil || p.ID != id {
				t.Fatalf("err=%v id=%v", err, p.ID)
			}
		})
	}
}

func TestAuthorizeGame(t *testing.T) {
	store := storage.NewMockStorage()
	gs := state.NewGameState("foo_scenario.json", nil, "foo", "foo_model")
	owner := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	other := uuid.MustParse("44444444-4444-4444-8444-444444444444")
	if err := store.SaveGameState(t.Context(), gs.ID, gs); err != nil {
		t.Fatal(err)
	}
	if err := store.SetOwner(t.Context(), gs.ID, owner); err != nil {
		t.Fatal(err)
	}
	orphan := state.NewGameState("foo_scenario.json", nil, "foo", "foo_model")
	if err := store.SaveGameState(t.Context(), orphan.ID, orphan); err != nil {
		t.Fatal(err)
	}

	withID := func(id uuid.UUID) *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/v1/gamestate/"+gs.ID.String(), nil)
		return req.WithContext(WithPrincipal(req.Context(), Principal{ID: id}))
	}

	tests := []struct {
		name    string
		req     *http.Request
		id      uuid.UUID
		wantErr error
	}{
		{name: "owner", req: withID(owner), id: gs.ID},
		{name: "wrong owner", req: withID(other), id: gs.ID, wantErr: ErrForbidden},
		{name: "missing owner", req: withID(owner), id: orphan.ID, wantErr: storage.ErrNotFound},
		{name: "no principal", req: httptest.NewRequest(http.MethodGet, "/v1/gamestate/"+gs.ID.String(), nil), id: gs.ID, wantErr: ErrUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := AuthorizeGame(tt.req, store, tt.id)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
