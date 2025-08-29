package loadbalancer

import (
	"testing"

	"github.com/barisgenc/gatekeeper/internal/config"
)

func TestNew(t *testing.T) {
	backends := []config.Backend{
		{Name: "backend1", URL: "http://localhost:3001", Weight: 50},
		{Name: "backend2", URL: "http://localhost:3002", Weight: 50},
	}

	lb := New(backends)

	if lb == nil {
		t.Fatal("Expected LoadBalancer to be created, got nil")
	}

	if len(lb.backends) != 2 {
		t.Errorf("Expected 2 backends, got %d", len(lb.backends))
	}

	for _, backend := range lb.backends {
		if !backend.Healthy {
			t.Error("Expected backends to be healthy initially")
		}
	}
}

func TestRoundRobinBalancing(t *testing.T) {
	backends := []config.Backend{
		{Name: "backend1", URL: "http://localhost:3001", Weight: 50},
		{Name: "backend2", URL: "http://localhost:3002", Weight: 50},
	}

	lb := New(backends)

	// Test round-robin distribution
	first := lb.NextBackend()
	second := lb.NextBackend()
	third := lb.NextBackend()

	if first == nil || second == nil || third == nil {
		t.Fatal("Expected backends to be returned, got nil")
	}

	if first.Name == second.Name {
		t.Error("Expected different backends in round-robin")
	}

	if first.Name != third.Name {
		t.Error("Expected round-robin to cycle back to first backend")
	}
}

func TestSetBackendHealth(t *testing.T) {
	backends := []config.Backend{
		{Name: "backend1", URL: "http://localhost:3001", Weight: 50},
		{Name: "backend2", URL: "http://localhost:3002", Weight: 50},
	}

	lb := New(backends)

	// Mark first backend as unhealthy
	lb.SetBackendHealth("backend1", false)

	// All requests should go to backend2
	for i := 0; i < 5; i++ {
		backend := lb.NextBackend()
		if backend == nil {
			t.Fatal("Expected backend to be returned")
		}
		if backend.Name != "backend2" {
			t.Errorf("Expected backend2, got %s", backend.Name)
		}
	}

	// Mark backend2 as unhealthy too
	lb.SetBackendHealth("backend2", false)

	// Should return nil as no backends are healthy
	backend := lb.NextBackend()
	if backend != nil {
		t.Error("Expected nil when no backends are healthy")
	}
}

func TestWeightedRoundRobin(t *testing.T) {
	backends := []config.Backend{
		{Name: "backend1", URL: "http://localhost:3001", Weight: 75},
		{Name: "backend2", URL: "http://localhost:3002", Weight: 25},
	}

	lb := New(backends)
	lb.SetAlgorithm("weighted_round_robin")

	// Count backend selections over many requests
	backend1Count := 0
	backend2Count := 0
	totalRequests := 1000

	for i := 0; i < totalRequests; i++ {
		backend := lb.NextBackend()
		if backend == nil {
			t.Fatal("Expected backend to be returned")
		}

		switch backend.Name {
		case "backend1":
			backend1Count++
		case "backend2":
			backend2Count++
		}
	}

	// Check that distribution roughly matches weights (75% vs 25%)
	backend1Percentage := float64(backend1Count) / float64(totalRequests) * 100
	backend2Percentage := float64(backend2Count) / float64(totalRequests) * 100

	if backend1Percentage < 65 || backend1Percentage > 85 {
		t.Errorf("Expected backend1 to get ~75%% of requests, got %.2f%%", backend1Percentage)
	}

	if backend2Percentage < 15 || backend2Percentage > 35 {
		t.Errorf("Expected backend2 to get ~25%% of requests, got %.2f%%", backend2Percentage)
	}
}

func TestRandomAlgorithm(t *testing.T) {
	backends := []config.Backend{
		{Name: "backend1", URL: "http://localhost:3001", Weight: 50},
		{Name: "backend2", URL: "http://localhost:3002", Weight: 50},
	}

	lb := New(backends)
	lb.SetAlgorithm("random")

	// Test that we get both backends over many requests
	seenBackends := make(map[string]bool)
	for i := 0; i < 100; i++ {
		backend := lb.NextBackend()
		if backend != nil {
			seenBackends[backend.Name] = true
		}
	}

	if len(seenBackends) != 2 {
		t.Errorf("Expected to see both backends with random algorithm, saw %d", len(seenBackends))
	}
}

func TestGetHealthyBackends(t *testing.T) {
	backends := []config.Backend{
		{Name: "backend1", URL: "http://localhost:3001", Weight: 50},
		{Name: "backend2", URL: "http://localhost:3002", Weight: 50},
		{Name: "backend3", URL: "http://localhost:3003", Weight: 50},
	}

	lb := New(backends)

	// Initially all backends should be healthy
	healthy := lb.GetHealthyBackends()
	if len(healthy) != 3 {
		t.Errorf("Expected 3 healthy backends, got %d", len(healthy))
	}

	// Mark one as unhealthy
	lb.SetBackendHealth("backend2", false)

	healthy = lb.GetHealthyBackends()
	if len(healthy) != 2 {
		t.Errorf("Expected 2 healthy backends after marking one unhealthy, got %d", len(healthy))
	}
}

func TestGetStats(t *testing.T) {
	backends := []config.Backend{
		{Name: "backend1", URL: "http://localhost:3001", Weight: 50},
		{Name: "backend2", URL: "http://localhost:3002", Weight: 50},
	}

	lb := New(backends)
	lb.SetBackendHealth("backend1", false)

	stats := lb.GetStats()

	if stats["total_backends"] != 2 {
		t.Errorf("Expected 2 total backends, got %v", stats["total_backends"])
	}

	if stats["healthy_backends"] != 1 {
		t.Errorf("Expected 1 healthy backend, got %v", stats["healthy_backends"])
	}

	if stats["unhealthy_backends"] != 1 {
		t.Errorf("Expected 1 unhealthy backend, got %v", stats["unhealthy_backends"])
	}

	if stats["algorithm"] != "round_robin" {
		t.Errorf("Expected round_robin algorithm, got %v", stats["algorithm"])
	}
}

func TestSetInvalidAlgorithm(t *testing.T) {
	backends := []config.Backend{
		{Name: "backend1", URL: "http://localhost:3001", Weight: 50},
	}

	lb := New(backends)
	lb.SetAlgorithm("invalid_algorithm")

	stats := lb.GetStats()
	if stats["algorithm"] != "round_robin" {
		t.Error("Expected algorithm to default to round_robin for invalid algorithm")
	}
}

// Benchmark tests
func BenchmarkNextBackendRoundRobin(b *testing.B) {
	backends := []config.Backend{
		{Name: "backend1", URL: "http://localhost:3001", Weight: 50},
		{Name: "backend2", URL: "http://localhost:3002", Weight: 50},
		{Name: "backend3", URL: "http://localhost:3003", Weight: 50},
	}

	lb := New(backends)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lb.NextBackend()
	}
}

func BenchmarkNextBackendWeighted(b *testing.B) {
	backends := []config.Backend{
		{Name: "backend1", URL: "http://localhost:3001", Weight: 75},
		{Name: "backend2", URL: "http://localhost:3002", Weight: 25},
	}

	lb := New(backends)
	lb.SetAlgorithm("weighted_round_robin")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lb.NextBackend()
	}
}