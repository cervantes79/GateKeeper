package gateway

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/mux"

	"github.com/barisgenc/gatekeeper/internal/config"
	"github.com/barisgenc/gatekeeper/internal/loadbalancer"
	"github.com/barisgenc/gatekeeper/internal/logger"
	"github.com/barisgenc/gatekeeper/internal/metrics"
	"github.com/barisgenc/gatekeeper/internal/middleware"
)

type Gateway struct {
	config       *config.Config
	loadBalancer *loadbalancer.LoadBalancer
	router       *mux.Router
	middlewares  []middleware.Middleware
	mu           sync.RWMutex
}

func New(cfg *config.Config) *Gateway {
	gw := &Gateway{
		config:       cfg,
		loadBalancer: loadbalancer.New(cfg.Backends),
		router:       mux.NewRouter(),
	}

	gw.setupMiddleware()
	gw.setupRoutes()
	gw.startHealthChecks()

	return gw
}

func (gw *Gateway) setupMiddleware() {
	// Rate limiting middleware
	rateLimiter := middleware.NewRateLimiter(
		gw.config.RateLimit.RequestsPerMinute,
		gw.config.RateLimit.BurstSize,
	)

	// Logging middleware
	loggingMiddleware := middleware.NewLogging()

	// Metrics middleware
	metricsMiddleware := middleware.NewMetrics()

	// Add middlewares in order
	gw.middlewares = []middleware.Middleware{
		loggingMiddleware,
		metricsMiddleware,
		rateLimiter,
	}
}

func (gw *Gateway) setupRoutes() {
	// Health check endpoint
	gw.router.HandleFunc("/health", gw.healthHandler).Methods("GET")

	// Metrics endpoint
	gw.router.Handle("/metrics", metrics.Handler()).Methods("GET")

	// All other requests go through the proxy
	gw.router.PathPrefix("/").HandlerFunc(gw.proxyHandler)
}

func (gw *Gateway) Handler() http.Handler {
	handler := http.Handler(gw.router)

	// Apply middlewares in reverse order (last middleware wraps first)
	for i := len(gw.middlewares) - 1; i >= 0; i-- {
		handler = gw.middlewares[i].Wrap(handler)
	}

	return handler
}

func (gw *Gateway) healthHandler(w http.ResponseWriter, r *http.Request) {
	gw.mu.RLock()
	backends := gw.loadBalancer.GetHealthyBackends()
	gw.mu.RUnlock()

	status := "healthy"
	if len(backends) == 0 {
		status = "unhealthy"
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	response := fmt.Sprintf(`{"status":"%s","healthy_backends":%d}`, status, len(backends))
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(response))
}

func (gw *Gateway) proxyHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	backend := gw.loadBalancer.NextBackend()
	if backend == nil {
		logger.Error("No healthy backends available")
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		metrics.RecordRequest(r.Method, "503", "none", time.Since(start))
		return
	}

	// Parse backend URL
	target, err := url.Parse(backend.URL)
	if err != nil {
		logger.Error("Invalid backend URL %s: %v", backend.URL, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		metrics.RecordRequest(r.Method, "500", backend.Name, time.Since(start))
		return
	}

	// Create reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(target)

	// Modify the request
	r.URL.Host = target.Host
	r.URL.Scheme = target.Scheme
	r.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))
	r.Host = target.Host

	// Create response writer to capture status
	rw := metrics.NewResponseWriter(w)

	// Serve the request
	proxy.ServeHTTP(rw, r)

	// Record metrics
	duration := time.Since(start)
	metrics.RecordRequest(r.Method, rw.StatusCode(), backend.Name, duration)
	metrics.RecordBackendRequest(backend.Name, rw.StatusCode())

	logger.Debug("Proxied %s %s to %s (status: %s, duration: %v)",
		r.Method, r.URL.Path, backend.Name, rw.StatusCode(), duration)
}

func (gw *Gateway) startHealthChecks() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				gw.performHealthChecks()
			}
		}
	}()
}

func (gw *Gateway) performHealthChecks() {
	gw.mu.Lock()
	defer gw.mu.Unlock()

	for _, backend := range gw.config.Backends {
		go gw.checkBackendHealth(backend)
	}
}

func (gw *Gateway) checkBackendHealth(backend config.Backend) {
	healthURL := backend.URL + backend.Health
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		logger.Error("Failed to create health check request for %s: %v", backend.Name, err)
		gw.loadBalancer.SetBackendHealth(backend.Name, false)
		metrics.SetBackendStatus(backend.Name, false)
		return
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logger.Warn("Health check failed for backend %s: %v", backend.Name, err)
		gw.loadBalancer.SetBackendHealth(backend.Name, false)
		metrics.SetBackendStatus(backend.Name, false)
		return
	}
	defer resp.Body.Close()

	isHealthy := resp.StatusCode >= 200 && resp.StatusCode < 300
	gw.loadBalancer.SetBackendHealth(backend.Name, isHealthy)
	metrics.SetBackendStatus(backend.Name, isHealthy)

	if isHealthy {
		logger.Debug("Health check passed for backend %s", backend.Name)
	} else {
		logger.Warn("Health check failed for backend %s (status: %d)", backend.Name, resp.StatusCode)
	}
}