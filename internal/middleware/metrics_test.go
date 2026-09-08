package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestInstrument_RecordsChat(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := newHTTPMetrics(reg)
	h := m.instrument("chat", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/chat", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rr.Code)
	}

	got := testutil.ToFloat64(m.requests.With(prometheus.Labels{
		"code": "202", "method": "post", "handler": "chat",
	}))
	if got != 1 {
		t.Fatalf("http_requests_total 202/post/chat = %v, want 1", got)
	}
	if n := testutil.CollectAndCount(m.duration); n != 1 {
		t.Fatalf("duration samples = %d, want 1", n)
	}
	if got := testutil.ToFloat64(m.inFlight.WithLabelValues("chat")); got != 0 {
		t.Fatalf("in_flight after return = %v, want 0", got)
	}
}

func TestInstrumentSSE_InFlightNoDuration(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := newHTTPMetrics(reg)

	started := make(chan struct{})
	h := m.instrumentSSE(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	if got := testutil.ToFloat64(m.inFlight.WithLabelValues("events")); got != 1 {
		t.Fatalf("in_flight during SSE = %v, want 1", got)
	}
	if n := testutil.CollectAndCount(m.duration); n != 0 {
		t.Fatalf("duration samples during SSE = %d, want 0", n)
	}

	cancel()
	<-done

	if got := testutil.ToFloat64(m.inFlight.WithLabelValues("events")); got != 0 {
		t.Fatalf("in_flight after SSE = %v, want 0", got)
	}
	if n := testutil.CollectAndCount(m.duration); n != 0 {
		t.Fatalf("duration samples after SSE = %d, want 0", n)
	}
	got := testutil.ToFloat64(m.requests.With(prometheus.Labels{
		"code": "200", "method": "get", "handler": "events",
	}))
	if got != 1 {
		t.Fatalf("http_requests_total 200/get/events = %v, want 1", got)
	}
}

func TestInstrument_RecordsUnauthorized(t *testing.T) {
	_, pub := testKeyPair(t)
	reg := prometheus.NewRegistry()
	m := newHTTPMetrics(reg)
	h := m.instrument("chat", JWT(pub, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next should not run")
	})))

	req := httptest.NewRequest(http.MethodPost, "/v1/chat", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}

	got := testutil.ToFloat64(m.requests.With(prometheus.Labels{
		"code": "401", "method": "post", "handler": "chat",
	}))
	if got != 1 {
		t.Fatalf("http_requests_total 401/post/chat = %v, want 1", got)
	}
}
