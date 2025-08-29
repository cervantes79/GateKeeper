package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoggingMiddleware(t *testing.T) {
	middleware := NewLogging()

	handler := middleware.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	req, err := http.NewRequest("GET", "/test", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	if body := rr.Body.String(); body != "OK" {
		t.Errorf("Handler returned unexpected body: got %v want %v", body, "OK")
	}
}

func TestMetricsMiddleware(t *testing.T) {
	middleware := NewMetrics()

	handler := middleware.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	req, err := http.NewRequest("GET", "/test", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	// Very restrictive rate limiting for testing
	middleware := NewRateLimiter(1, 1) // 1 request per minute, burst of 1

	handler := middleware.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	// First request should succeed
	req, err := http.NewRequest("GET", "/test", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("First request should succeed: got %v want %v", status, http.StatusOK)
	}

	// Second request should be rate limited
	req, err = http.NewRequest("GET", "/test", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusTooManyRequests {
		t.Errorf("Second request should be rate limited: got %v want %v", status, http.StatusTooManyRequests)
	}

	// Check Retry-After header
	if retryAfter := rr.Header().Get("Retry-After"); retryAfter != "60" {
		t.Errorf("Expected Retry-After header to be 60, got %v", retryAfter)
	}
}

func TestRateLimitHealthEndpointBypass(t *testing.T) {
	middleware := NewRateLimiter(1, 1)

	handler := middleware.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	// Multiple requests to /health should all succeed
	for i := 0; i < 5; i++ {
		req, err := http.NewRequest("GET", "/health", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("Health endpoint request %d should succeed: got %v want %v", i, status, http.StatusOK)
		}
	}
}

func TestCORSMiddleware(t *testing.T) {
	middleware := NewCORS(
		[]string{"https://example.com", "https://test.com"},
		[]string{"GET", "POST", "PUT", "DELETE"},
		[]string{"Content-Type", "Authorization"},
	)

	handler := middleware.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	// Test allowed origin
	req, err := http.NewRequest("GET", "/test", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", "https://example.com")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if origin := rr.Header().Get("Access-Control-Allow-Origin"); origin != "https://example.com" {
		t.Errorf("Expected Access-Control-Allow-Origin to be https://example.com, got %v", origin)
	}

	if methods := rr.Header().Get("Access-Control-Allow-Methods"); methods != "GET, POST, PUT, DELETE" {
		t.Errorf("Expected Access-Control-Allow-Methods to be set correctly, got %v", methods)
	}
}

func TestCORSPreflightRequest(t *testing.T) {
	middleware := NewCORS(
		[]string{"*"},
		[]string{"GET", "POST"},
		[]string{"Content-Type"},
	)

	handler := middleware.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called for OPTIONS request")
	}))

	req, err := http.NewRequest("OPTIONS", "/test", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Preflight request should return OK: got %v want %v", status, http.StatusOK)
	}

	if origin := rr.Header().Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Errorf("Expected Access-Control-Allow-Origin to be *, got %v", origin)
	}
}

func TestGetClientIP(t *testing.T) {
	testCases := []struct {
		name           string
		headers        map[string]string
		remoteAddr     string
		expectedIP     string
	}{
		{
			name:           "X-Forwarded-For header",
			headers:        map[string]string{"X-Forwarded-For": "192.168.1.100"},
			remoteAddr:     "10.0.0.1:12345",
			expectedIP:     "192.168.1.100",
		},
		{
			name:           "X-Real-IP header",
			headers:        map[string]string{"X-Real-IP": "192.168.1.200"},
			remoteAddr:     "10.0.0.1:12345",
			expectedIP:     "192.168.1.200",
		},
		{
			name:           "RemoteAddr fallback",
			headers:        map[string]string{},
			remoteAddr:     "10.0.0.1:12345",
			expectedIP:     "10.0.0.1:12345",
		},
		{
			name: "X-Forwarded-For takes precedence",
			headers: map[string]string{
				"X-Forwarded-For": "192.168.1.100",
				"X-Real-IP":       "192.168.1.200",
			},
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "192.168.1.100",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", "/test", nil)
			if err != nil {
				t.Fatal(err)
			}

			for key, value := range tc.headers {
				req.Header.Set(key, value)
			}
			req.RemoteAddr = tc.remoteAddr

			ip := getClientIP(req)
			if ip != tc.expectedIP {
				t.Errorf("Expected IP %v, got %v", tc.expectedIP, ip)
			}
		})
	}
}

func TestContains(t *testing.T) {
	slice := []string{"apple", "banana", "cherry"}

	if !contains(slice, "banana") {
		t.Error("Expected contains to return true for existing item")
	}

	if contains(slice, "grape") {
		t.Error("Expected contains to return false for non-existing item")
	}
}

func TestJoinStrings(t *testing.T) {
	testCases := []struct {
		slice     []string
		separator string
		expected  string
	}{
		{[]string{"a", "b", "c"}, ", ", "a, b, c"},
		{[]string{"x"}, "-", "x"},
		{[]string{}, ", ", ""},
	}

	for _, tc := range testCases {
		result := joinStrings(tc.slice, tc.separator)
		if result != tc.expected {
			t.Errorf("Expected %v, got %v", tc.expected, result)
		}
	}
}

// Benchmark tests
func BenchmarkLoggingMiddleware(b *testing.B) {
	middleware := NewLogging()
	handler := middleware.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req, _ := http.NewRequest("GET", "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
}

func BenchmarkRateLimitMiddleware(b *testing.B) {
	middleware := NewRateLimiter(10000, 100) // High limits for benchmarking
	handler := middleware.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req, _ := http.NewRequest("GET", "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
}