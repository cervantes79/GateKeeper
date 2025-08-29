package middleware

import (
	"net/http"
	"time"

	"golang.org/x/time/rate"

	"github.com/barisgenc/gatekeeper/internal/logger"
	"github.com/barisgenc/gatekeeper/internal/metrics"
)

type Middleware interface {
	Wrap(http.Handler) http.Handler
}

// Logging middleware
type LoggingMiddleware struct{}

func NewLogging() *LoggingMiddleware {
	return &LoggingMiddleware{}
}

func (m *LoggingMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Create response writer to capture status
		rw := metrics.NewResponseWriter(w)
		
		// Call next handler
		next.ServeHTTP(rw, r)
		
		duration := time.Since(start)
		
		logger.WithFields(map[string]interface{}{
			"method":     r.Method,
			"path":       r.URL.Path,
			"status":     rw.StatusCode(),
			"duration":   duration.String(),
			"remote_ip":  getClientIP(r),
			"user_agent": r.UserAgent(),
		}).Info("HTTP Request")
	})
}

// Metrics middleware
type MetricsMiddleware struct{}

func NewMetrics() *MetricsMiddleware {
	return &MetricsMiddleware{}
}

func (m *MetricsMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Create response writer to capture status
		rw := metrics.NewResponseWriter(w)
		
		// Call next handler
		next.ServeHTTP(rw, r)
		
		// Skip metrics recording for metrics endpoint itself
		if r.URL.Path != "/metrics" {
			duration := time.Since(start)
			metrics.RecordRequest(r.Method, rw.StatusCode(), "gateway", duration)
		}
	})
}

// Rate limiting middleware
type RateLimitMiddleware struct {
	limiter *rate.Limiter
}

func NewRateLimiter(requestsPerMinute, burstSize int) *RateLimitMiddleware {
	// Convert requests per minute to requests per second
	rps := float64(requestsPerMinute) / 60.0
	limiter := rate.NewLimiter(rate.Limit(rps), burstSize)
	
	logger.Info("Rate limiter initialized: %.2f req/sec, burst: %d", rps, burstSize)
	
	return &RateLimitMiddleware{
		limiter: limiter,
	}
}

func (m *RateLimitMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip rate limiting for health and metrics endpoints
		if r.URL.Path == "/health" || r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		if !m.limiter.Allow() {
			logger.Warn("Rate limit exceeded for %s %s from %s", 
				r.Method, r.URL.Path, getClientIP(r))
			
			metrics.RecordRateLimit()
			
			w.Header().Set("Retry-After", "60")
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// CORS middleware
type CORSMiddleware struct {
	allowedOrigins []string
	allowedMethods []string
	allowedHeaders []string
}

func NewCORS(origins, methods, headers []string) *CORSMiddleware {
	return &CORSMiddleware{
		allowedOrigins: origins,
		allowedMethods: methods,
		allowedHeaders: headers,
	}
}

func (m *CORSMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		
		// Set CORS headers
		if len(m.allowedOrigins) > 0 && contains(m.allowedOrigins, origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		} else if len(m.allowedOrigins) > 0 && contains(m.allowedOrigins, "*") {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		
		if len(m.allowedMethods) > 0 {
			w.Header().Set("Access-Control-Allow-Methods", joinStrings(m.allowedMethods, ", "))
		}
		
		if len(m.allowedHeaders) > 0 {
			w.Header().Set("Access-Control-Allow-Headers", joinStrings(m.allowedHeaders, ", "))
		}
		
		// Handle preflight request
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// Helper functions
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	
	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	
	// Fall back to RemoteAddr
	return r.RemoteAddr
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func joinStrings(slice []string, separator string) string {
	if len(slice) == 0 {
		return ""
	}
	
	result := slice[0]
	for i := 1; i < len(slice); i++ {
		result += separator + slice[i]
	}
	return result
}