package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Request metrics
	requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gatekeeper_requests_total",
			Help: "Total number of HTTP requests processed",
		},
		[]string{"method", "status", "backend"},
	)

	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gatekeeper_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "backend"},
	)

	// Backend metrics
	backendRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gatekeeper_backend_requests_total",
			Help: "Total number of requests sent to backends",
		},
		[]string{"backend", "status"},
	)

	backendUp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gatekeeper_backend_up",
			Help: "Backend health status (1 = up, 0 = down)",
		},
		[]string{"backend"},
	)

	// Rate limiting metrics
	rateLimitedRequests = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "gatekeeper_rate_limited_requests_total",
			Help: "Total number of rate limited requests",
		},
	)

	// Gateway metrics
	gatewayInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gatekeeper_info",
			Help: "Information about the GateKeeper instance",
		},
		[]string{"version", "go_version"},
	)
)

func Init() {
	// Register all metrics
	prometheus.MustRegister(
		requestsTotal,
		requestDuration,
		backendRequestsTotal,
		backendUp,
		rateLimitedRequests,
		gatewayInfo,
	)

	// Set gateway info
	gatewayInfo.WithLabelValues("1.0.0", "1.21").Set(1)
}

// RecordRequest records metrics for an HTTP request
func RecordRequest(method, status, backend string, duration time.Duration) {
	requestsTotal.WithLabelValues(method, status, backend).Inc()
	requestDuration.WithLabelValues(method, backend).Observe(duration.Seconds())
}

// RecordBackendRequest records metrics for backend requests
func RecordBackendRequest(backend, status string) {
	backendRequestsTotal.WithLabelValues(backend, status).Inc()
}

// SetBackendStatus sets the health status of a backend
func SetBackendStatus(backend string, up bool) {
	value := 0.0
	if up {
		value = 1.0
	}
	backendUp.WithLabelValues(backend).Set(value)
}

// RecordRateLimit records a rate limited request
func RecordRateLimit() {
	rateLimitedRequests.Inc()
}

// Handler returns the Prometheus metrics handler
func Handler() http.Handler {
	return promhttp.Handler()
}

// ResponseWriter wraps http.ResponseWriter to capture status codes
type ResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func NewResponseWriter(w http.ResponseWriter) *ResponseWriter {
	return &ResponseWriter{w, http.StatusOK}
}

func (rw *ResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *ResponseWriter) StatusCode() string {
	return strconv.Itoa(rw.statusCode)
}