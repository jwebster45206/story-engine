package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRouteLabel(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/health", "health"},
		{"/metrics", "metrics"},
		{"/v1/chat", "chat"},
		{"/v1/events/gamestate/22222222-2222-4222-8222-222222222222", "events"},
		{"/v1/gamestate", "gamestate"},
		{"/v1/gamestate/22222222-2222-4222-8222-222222222222", "gamestate"},
		{"/v1/providers", "providers"},
		{"/v1/scenarios", "scenarios"},
		{"/v1/pcs", "pcs"},
		{"/v1/narrators", "narrators"},
		{"/v1/monsters", "monsters"},
		{"/v1/unknown", "other"},
		{"/", "other"},
	}
	for _, tt := range tests {
		if got := routeLabel(tt.path); got != tt.want {
			t.Errorf("routeLabel(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestMetrics_RecordsChat(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := newHTTPMetrics(reg)
	h := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/chat", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rr.Code)
	}

	if got := testutil.ToFloat64(m.requests.WithLabelValues("POST", "202", "chat")); got != 1 {
		t.Fatalf("http_requests_total POST/202/chat = %v, want 1", got)
	}
	if n := testutil.CollectAndCount(m.duration); n != 1 {
		t.Fatalf("duration samples = %d, want 1", n)
	}
	if got := testutil.ToFloat64(m.inFlight); got != 0 {
		t.Fatalf("in_flight after return = %v, want 0", got)
	}
}

func TestMetrics_SkipsMetricsPath(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := newHTTPMetrics(reg)
	h := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	if n := testutil.CollectAndCount(m.requests); n != 0 {
		t.Fatalf("requests samples = %d, want 0", n)
	}
	if n := testutil.CollectAndCount(m.duration); n != 0 {
		t.Fatalf("duration samples = %d, want 0", n)
	}
	if got := testutil.ToFloat64(m.inFlight); got != 0 {
		t.Fatalf("in_flight = %v, want 0", got)
	}
}

func TestMetrics_EventsInFlightOnly(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := newHTTPMetrics(reg)

	started := make(chan struct{})
	h := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/v1/events/gamestate/22222222-2222-4222-8222-222222222222", nil)
	req = req.WithContext(ctx)

	done := make(chan struct{})
	go func() {
		h.ServeHTTP(httptest.NewRecorder(), req)
		close(done)
	}()

	<-started
	if got := testutil.ToFloat64(m.inFlight); got != 1 {
		t.Fatalf("in_flight during SSE = %v, want 1", got)
	}
	if n := testutil.CollectAndCount(m.duration); n != 0 {
		t.Fatalf("duration samples during SSE = %d, want 0", n)
	}

	cancel()
	<-done

	if got := testutil.ToFloat64(m.inFlight); got != 0 {
		t.Fatalf("in_flight after SSE = %v, want 0", got)
	}
	if n := testutil.CollectAndCount(m.duration); n != 0 {
		t.Fatalf("duration samples after SSE = %d, want 0", n)
	}
	if n := testutil.CollectAndCount(m.requests); n != 0 {
		t.Fatalf("requests samples after SSE = %d, want 0", n)
	}
}

func TestMetrics_RecordsUnauthorized(t *testing.T) {
	_, pub := testKeyPair(t)
	reg := prometheus.NewRegistry()
	m := newHTTPMetrics(reg)
	h := m.Handler(JWT(pub, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next should not run")
	})))

	req := httptest.NewRequest(http.MethodPost, "/v1/chat", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}

	if got := testutil.ToFloat64(m.requests.WithLabelValues("POST", "401", "chat")); got != 1 {
		t.Fatalf("http_requests_total POST/401/chat = %v, want 1", got)
	}
}
