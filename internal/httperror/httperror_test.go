package httperror

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	Write(rr, slog.Default(), http.StatusNotFound, "Game state not found")

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := rr.Header().Get("WWW-Authenticate"); got != "" {
		t.Fatalf("WWW-Authenticate = %q, want empty", got)
	}
	var body Response
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Error != "Game state not found" {
		t.Fatalf("error = %q", body.Error)
	}
}

func TestWriteUnauthorized(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	Write(rr, slog.Default(), http.StatusUnauthorized, "unauthorized")

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	if got := rr.Header().Get("WWW-Authenticate"); got != "Bearer" {
		t.Fatalf("WWW-Authenticate = %q", got)
	}
	var body Response
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Error != "unauthorized" {
		t.Fatalf("error = %q", body.Error)
	}
}

func TestWriteNilLogger(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	Write(rr, nil, http.StatusBadRequest, "bad request")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	b, err := io.ReadAll(rr.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) == "" {
		t.Fatal("expected JSON body")
	}
}
