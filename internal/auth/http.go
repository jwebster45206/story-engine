package auth

import (
	"errors"
	"net/http"
	"uuid"

	"github.com/jwebster45206/story-engine/pkg/storage"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
)

// RequirePrincipal returns the caller Principal.
func RequirePrincipal(r *http.Request) (Principal, error) {
	p, ok := PrincipalFrom(r.Context())
	if !ok {
		return Principal{}, ErrUnauthorized
	}
	return p, nil
}

// AuthorizeGame reports whether the caller may access the gamestate.
func AuthorizeGame(r *http.Request, store storage.Storage, id uuid.UUID) error {
	p, err := RequirePrincipal(r)
	if err != nil {
		return err
	}
	ownerID, err := store.GetOwner(r.Context(), id)
	if err != nil {
		return err
	}
	if p.ID != ownerID {
		return ErrForbidden
	}
	return nil
}
