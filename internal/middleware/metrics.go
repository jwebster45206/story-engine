package middleware

import (
	"crypto/ecdsa"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type httpMetrics struct {
	inFlight *prometheus.GaugeVec
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

func newHTTPMetrics(reg prometheus.Registerer) *httpMetrics {
	factory := promauto.With(reg)
	return new(httpMetrics{
		inFlight: factory.NewGaugeVec(prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "HTTP requests currently being processed, including SSE.",
		}, []string{"handler"}),
		requests: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests completed.",
		}, []string{"code", "method", "handler"}),
		duration: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"code", "method", "handler"}),
	})
}

var defaultHTTPMetrics = newHTTPMetrics(prometheus.DefaultRegisterer)

// Instrument records request, error, and duration metrics for next. The
// handler label is the name given at registration, not the request path.
func Instrument(name string, next http.Handler) http.Handler {
	return defaultHTTPMetrics.instrument(name, next)
}

// Protected is Instrument around JWT so 401s are labeled with name.
func Protected(name string, pub *ecdsa.PublicKey, next http.Handler) http.Handler {
	return Instrument(name, JWT(pub, next))
}

// ProtectedSSE is JWT plus in-flight and request count for the events
// handler. Duration is omitted so SSE session length does not poison API p99.
func ProtectedSSE(pub *ecdsa.PublicKey, next http.Handler) http.Handler {
	return defaultHTTPMetrics.instrumentSSE(JWT(pub, next))
}

func (m *httpMetrics) instrument(name string, next http.Handler) http.Handler {
	labels := prometheus.Labels{"handler": name}
	return promhttp.InstrumentHandlerDuration(
		m.duration.MustCurryWith(labels),
		promhttp.InstrumentHandlerCounter(
			m.requests.MustCurryWith(labels),
			promhttp.InstrumentHandlerInFlight(m.inFlight.With(labels), next),
		),
	)
}

func (m *httpMetrics) instrumentSSE(next http.Handler) http.Handler {
	labels := prometheus.Labels{"handler": "events"}
	return promhttp.InstrumentHandlerCounter(
		m.requests.MustCurryWith(labels),
		promhttp.InstrumentHandlerInFlight(m.inFlight.With(labels), next),
	)
}
