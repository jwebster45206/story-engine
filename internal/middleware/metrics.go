package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type httpMetrics struct {
	inFlight prometheus.Gauge
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

func newHTTPMetrics(reg prometheus.Registerer) *httpMetrics {
	factory := promauto.With(reg)
	return new(httpMetrics{
		inFlight: factory.NewGauge(prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "HTTP requests currently being processed.",
		}),
		requests: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests completed.",
		}, []string{"method", "code", "handler"}),
		duration: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "code", "handler"}),
	})
}

var defaultHTTPMetrics = newHTTPMetrics(prometheus.DefaultRegisterer)

// Metrics records HTTP RED metrics. SSE (handler=events) is counted in-flight
// only so connection lifetime does not poison request duration.
func Metrics(next http.Handler) http.Handler {
	return defaultHTTPMetrics.Handler(next)
}

func (m *httpMetrics) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler := routeLabel(r.URL.Path)
		if handler == "metrics" {
			next.ServeHTTP(w, r)
			return
		}

		m.inFlight.Inc()
		defer m.inFlight.Dec()

		if handler == "events" {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		wrapped := new(responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		})
		next.ServeHTTP(wrapped, r)
		labels := prometheus.Labels{
			"method":  r.Method,
			"code":    strconv.Itoa(wrapped.statusCode),
			"handler": handler,
		}
		m.requests.With(labels).Inc()
		m.duration.With(labels).Observe(time.Since(start).Seconds())
	})
}

func routeLabel(path string) string {
	switch {
	case path == "/health":
		return "health"
	case path == "/metrics":
		return "metrics"
	case path == "/v1/chat" || strings.HasPrefix(path, "/v1/chat/"):
		return "chat"
	case strings.HasPrefix(path, "/v1/events/"):
		return "events"
	case path == "/v1/gamestate" || strings.HasPrefix(path, "/v1/gamestate/"):
		return "gamestate"
	case path == "/v1/providers" || strings.HasPrefix(path, "/v1/providers/"):
		return "providers"
	case path == "/v1/scenarios" || strings.HasPrefix(path, "/v1/scenarios/"):
		return "scenarios"
	case path == "/v1/pcs" || strings.HasPrefix(path, "/v1/pcs/"):
		return "pcs"
	case path == "/v1/narrators" || strings.HasPrefix(path, "/v1/narrators/"):
		return "narrators"
	case path == "/v1/monsters" || strings.HasPrefix(path, "/v1/monsters/"):
		return "monsters"
	default:
		return "other"
	}
}
