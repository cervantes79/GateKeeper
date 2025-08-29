package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/barisgenc/gatekeeper/internal/config"
)

func TestNew(t *testing.T) {
	cfg := &config.Config{
		Backends: []config.Backend{
			{Name: "test", URL: "http://localhost:3000", Weight: 100, Health: "/health"},
		},
		RateLimit: config.RateLimitConfig{
			RequestsPerMinute: 60,
			BurstSize:         10,
		},
	}

	gw := New(cfg)
	if gw == nil {
		t.Fatal("Expected gateway to be created, got nil")
	}

	if gw.config != cfg {
		t.Error("Expected gateway config to match input config")
	}

	if gw.loadBalancer == nil {
		t.Error("Expected load balancer to be initialized")
	}

	if gw.router == nil {
		t.Error("Expected router to be initialized")
	}
}

func TestHealthHandler(t *testing.T) {
	cfg := &config.Config{
		Backends: []config.Backend{
			{Name: "test", URL: "http://localhost:3000", Weight: 100, Health: "/health"},
		},
		RateLimit: config.RateLimitConfig{RequestsPerMinute: 60, BurstSize: 10},
	}

	gw := New(cfg)
	
	req, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(gw.healthHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	expected := `{"status":"healthy","healthy_backends":1}`
	if rr.Body.String() != expected {
		t.Errorf("Handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
	}
}

func TestGatewayHandler(t *testing.T) {
	// Create test backend server
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello from backend"))
	}))
	defer backendServer.Close()

	cfg := &config.Config{
		Backends: []config.Backend{
			{Name: "test", URL: backendServer.URL, Weight: 100, Health: "/health"},
		},
		RateLimit: config.RateLimitConfig{RequestsPerMinute: 60, BurstSize: 10},
	}

	gw := New(cfg)
	handler := gw.Handler()

	// Test health endpoint
	req, _ := http.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Health check failed: got %v want %v", rr.Code, http.StatusOK)
	}

	// Test metrics endpoint
	req, _ = http.NewRequest("GET", "/metrics", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Metrics endpoint failed: got %v want %v", rr.Code, http.StatusOK)
	}
}

func TestRateLimiting(t *testing.T) {
	cfg := &config.Config{
		Backends: []config.Backend{
			{Name: "test", URL: "http://localhost:3000", Weight: 100, Health: "/health"},
		},
		RateLimit: config.RateLimitConfig{RequestsPerMinute: 1, BurstSize: 1}, // Very low limits for testing
	}

	gw := New(cfg)
	handler := gw.Handler()

	// First request should succeed
	req, _ := http.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Second request should be rate limited
	req, _ = http.NewRequest("GET", "/test", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("Expected rate limiting: got %v want %v", rr.Code, http.StatusTooManyRequests)
	}
}

// Benchmark tests
func BenchmarkGatewayHandler(b *testing.B) {
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer backendServer.Close()

	cfg := &config.Config{
		Backends: []config.Backend{
			{Name: "test", URL: backendServer.URL, Weight: 100, Health: "/health"},
		},
		RateLimit: config.RateLimitConfig{RequestsPerMinute: 10000, BurstSize: 100},
	}

	gw := New(cfg)
	handler := gw.Handler()

	req, _ := http.NewRequest("GET", "/benchmark", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
}

func BenchmarkHealthCheck(b *testing.B) {
	cfg := &config.Config{
		Backends: []config.Backend{
			{Name: "test", URL: "http://localhost:3000", Weight: 100, Health: "/health"},
		},
		RateLimit: config.RateLimitConfig{RequestsPerMinute: 10000, BurstSize: 100},
	}

	gw := New(cfg)
	handler := gw.Handler()

	req, _ := http.NewRequest("GET", "/health", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
}