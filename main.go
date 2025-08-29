package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/barisgenc/gatekeeper/internal/config"
	"github.com/barisgenc/gatekeeper/internal/gateway"
	"github.com/barisgenc/gatekeeper/internal/logger"
	"github.com/barisgenc/gatekeeper/internal/metrics"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("Failed to load configuration: %v", err)
	}

	// Initialize logger
	logger.Init(cfg.LogLevel)

	// Initialize metrics
	metrics.Init()

	// Create gateway server
	gw := gateway.New(cfg)

	// Create HTTP server
	srv := &http.Server{
		Addr:         cfg.Server.Address,
		Handler:      gw.Handler(),
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(cfg.Server.IdleTimeout) * time.Second,
	}

	// Start server in goroutine
	go func() {
		logger.Info("Starting GateKeeper on %s", cfg.Server.Address)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown: %v", err)
	}

	logger.Info("Server exited")
}