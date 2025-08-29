package loadbalancer

import (
	"math/rand"
	"sync"
	"time"

	"github.com/barisgenc/gatekeeper/internal/config"
	"github.com/barisgenc/gatekeeper/internal/logger"
)

type BackendStatus struct {
	Backend config.Backend
	Healthy bool
	Weight  int
}

type LoadBalancer struct {
	backends      []*BackendStatus
	mu            sync.RWMutex
	currentIndex  int
	randomSource  *rand.Rand
	algorithm     string
}

func New(backends []config.Backend) *LoadBalancer {
	lb := &LoadBalancer{
		backends:     make([]*BackendStatus, len(backends)),
		randomSource: rand.New(rand.NewSource(time.Now().UnixNano())),
		algorithm:    "round_robin", // Default algorithm
	}

	for i, backend := range backends {
		lb.backends[i] = &BackendStatus{
			Backend: backend,
			Healthy: true, // Assume healthy initially
			Weight:  backend.Weight,
		}
	}

	logger.Info("LoadBalancer initialized with %d backends", len(backends))
	return lb
}

// NextBackend returns the next backend using round-robin algorithm
func (lb *LoadBalancer) NextBackend() *config.Backend {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	healthyBackends := lb.getHealthyBackendsLocked()
	if len(healthyBackends) == 0 {
		logger.Warn("No healthy backends available")
		return nil
	}

	switch lb.algorithm {
	case "weighted_round_robin":
		return lb.weightedRoundRobin(healthyBackends)
	case "random":
		return lb.randomBackend(healthyBackends)
	case "least_connections":
		// For now, fall back to round robin
		// In a production system, you'd track active connections
		return lb.roundRobin(healthyBackends)
	default:
		return lb.roundRobin(healthyBackends)
	}
}

func (lb *LoadBalancer) roundRobin(healthyBackends []*BackendStatus) *config.Backend {
	if len(healthyBackends) == 0 {
		return nil
	}

	backend := healthyBackends[lb.currentIndex%len(healthyBackends)]
	lb.currentIndex++
	
	// Prevent overflow
	if lb.currentIndex >= 1000000 {
		lb.currentIndex = 0
	}
	
	return &backend.Backend
}

func (lb *LoadBalancer) weightedRoundRobin(healthyBackends []*BackendStatus) *config.Backend {
	if len(healthyBackends) == 0 {
		return nil
	}

	totalWeight := 0
	for _, backend := range healthyBackends {
		totalWeight += backend.Weight
	}

	if totalWeight == 0 {
		return lb.roundRobin(healthyBackends)
	}

	// Generate random number between 0 and totalWeight
	randomWeight := lb.randomSource.Intn(totalWeight)
	
	currentWeight := 0
	for _, backend := range healthyBackends {
		currentWeight += backend.Weight
		if randomWeight < currentWeight {
			return &backend.Backend
		}
	}

	// Fallback to first backend
	return &healthyBackends[0].Backend
}

func (lb *LoadBalancer) randomBackend(healthyBackends []*BackendStatus) *config.Backend {
	if len(healthyBackends) == 0 {
		return nil
	}

	index := lb.randomSource.Intn(len(healthyBackends))
	return &healthyBackends[index].Backend
}

func (lb *LoadBalancer) getHealthyBackendsLocked() []*BackendStatus {
	var healthy []*BackendStatus
	for _, backend := range lb.backends {
		if backend.Healthy {
			healthy = append(healthy, backend)
		}
	}
	return healthy
}

// GetHealthyBackends returns the list of healthy backends (thread-safe)
func (lb *LoadBalancer) GetHealthyBackends() []*BackendStatus {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	return lb.getHealthyBackendsLocked()
}

// SetBackendHealth updates the health status of a backend
func (lb *LoadBalancer) SetBackendHealth(backendName string, healthy bool) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	for _, backend := range lb.backends {
		if backend.Backend.Name == backendName {
			if backend.Healthy != healthy {
				logger.Info("Backend %s health changed: %v -> %v", backendName, backend.Healthy, healthy)
				backend.Healthy = healthy
			}
			return
		}
	}

	logger.Warn("Backend %s not found when updating health status", backendName)
}

// SetAlgorithm sets the load balancing algorithm
func (lb *LoadBalancer) SetAlgorithm(algorithm string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	
	validAlgorithms := map[string]bool{
		"round_robin":          true,
		"weighted_round_robin": true,
		"random":               true,
		"least_connections":    true,
	}

	if !validAlgorithms[algorithm] {
		logger.Warn("Invalid load balancing algorithm: %s, using round_robin", algorithm)
		lb.algorithm = "round_robin"
		return
	}

	logger.Info("Load balancing algorithm set to: %s", algorithm)
	lb.algorithm = algorithm
}

// GetStats returns statistics about backends
func (lb *LoadBalancer) GetStats() map[string]interface{} {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	stats := make(map[string]interface{})
	totalBackends := len(lb.backends)
	healthyBackends := len(lb.getHealthyBackendsLocked())

	stats["total_backends"] = totalBackends
	stats["healthy_backends"] = healthyBackends
	stats["unhealthy_backends"] = totalBackends - healthyBackends
	stats["algorithm"] = lb.algorithm

	backendStats := make([]map[string]interface{}, 0, len(lb.backends))
	for _, backend := range lb.backends {
		backendStat := map[string]interface{}{
			"name":    backend.Backend.Name,
			"url":     backend.Backend.URL,
			"healthy": backend.Healthy,
			"weight":  backend.Weight,
		}
		backendStats = append(backendStats, backendStat)
	}
	stats["backends"] = backendStats

	return stats
}