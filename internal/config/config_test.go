package config

import (
	"os"
	"testing"
)

func TestLoadDefaultConfig(t *testing.T) {
	// Clear environment variables
	os.Clearenv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Expected no error loading default config, got: %v", err)
	}

	// Test default values
	if cfg.Server.Address != ":8080" {
		t.Errorf("Expected default address :8080, got %v", cfg.Server.Address)
	}

	if cfg.Server.ReadTimeout != 30 {
		t.Errorf("Expected default read timeout 30, got %v", cfg.Server.ReadTimeout)
	}

	if cfg.RateLimit.RequestsPerMinute != 100 {
		t.Errorf("Expected default rate limit 100, got %v", cfg.RateLimit.RequestsPerMinute)
	}

	if cfg.LogLevel != "info" {
		t.Errorf("Expected default log level info, got %v", cfg.LogLevel)
	}

	// Test default backend
	if len(cfg.Backends) != 1 {
		t.Errorf("Expected 1 default backend, got %d", len(cfg.Backends))
	}

	if cfg.Backends[0].Name != "default" {
		t.Errorf("Expected default backend name 'default', got %v", cfg.Backends[0].Name)
	}
}

func TestLoadConfigFromEnvironment(t *testing.T) {
	// Set environment variables
	os.Setenv("GATEKEEPER_ADDRESS", ":9090")
	os.Setenv("GATEKEEPER_READ_TIMEOUT", "60")
	os.Setenv("GATEKEEPER_RATE_LIMIT", "200")
	os.Setenv("GATEKEEPER_LOG_LEVEL", "debug")
	os.Setenv("GATEKEEPER_DEFAULT_BACKEND", "http://localhost:4000")

	defer func() {
		os.Unsetenv("GATEKEEPER_ADDRESS")
		os.Unsetenv("GATEKEEPER_READ_TIMEOUT")
		os.Unsetenv("GATEKEEPER_RATE_LIMIT")
		os.Unsetenv("GATEKEEPER_LOG_LEVEL")
		os.Unsetenv("GATEKEEPER_DEFAULT_BACKEND")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Expected no error loading config from environment, got: %v", err)
	}

	if cfg.Server.Address != ":9090" {
		t.Errorf("Expected address :9090 from env, got %v", cfg.Server.Address)
	}

	if cfg.Server.ReadTimeout != 60 {
		t.Errorf("Expected read timeout 60 from env, got %v", cfg.Server.ReadTimeout)
	}

	if cfg.RateLimit.RequestsPerMinute != 200 {
		t.Errorf("Expected rate limit 200 from env, got %v", cfg.RateLimit.RequestsPerMinute)
	}

	if cfg.LogLevel != "debug" {
		t.Errorf("Expected log level debug from env, got %v", cfg.LogLevel)
	}

	if cfg.Backends[0].URL != "http://localhost:4000" {
		t.Errorf("Expected backend URL http://localhost:4000 from env, got %v", cfg.Backends[0].URL)
	}
}

func TestLoadConfigFromFile(t *testing.T) {
	// Create temporary config file
	configContent := `
server:
  address: ":8888"
  readTimeout: 45
  writeTimeout: 45
  idleTimeout: 180

backends:
  - name: "api1"
    url: "http://localhost:3001"
    weight: 70
    health: "/api/health"
  - name: "api2"
    url: "http://localhost:3002"
    weight: 30
    health: "/api/health"

rateLimit:
  requestsPerMinute: 150
  burstSize: 20

logLevel: "warn"
`

	tmpFile, err := os.CreateTemp("", "config*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(configContent)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Set config file path
	os.Setenv("GATEKEEPER_CONFIG", tmpFile.Name())
	defer os.Unsetenv("GATEKEEPER_CONFIG")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Expected no error loading config from file, got: %v", err)
	}

	if cfg.Server.Address != ":8888" {
		t.Errorf("Expected address :8888 from file, got %v", cfg.Server.Address)
	}

	if cfg.Server.ReadTimeout != 45 {
		t.Errorf("Expected read timeout 45 from file, got %v", cfg.Server.ReadTimeout)
	}

	if len(cfg.Backends) != 2 {
		t.Errorf("Expected 2 backends from file, got %d", len(cfg.Backends))
	}

	if cfg.Backends[0].Name != "api1" {
		t.Errorf("Expected first backend name 'api1', got %v", cfg.Backends[0].Name)
	}

	if cfg.Backends[0].Weight != 70 {
		t.Errorf("Expected first backend weight 70, got %v", cfg.Backends[0].Weight)
	}

	if cfg.RateLimit.RequestsPerMinute != 150 {
		t.Errorf("Expected rate limit 150 from file, got %v", cfg.RateLimit.RequestsPerMinute)
	}

	if cfg.LogLevel != "warn" {
		t.Errorf("Expected log level warn from file, got %v", cfg.LogLevel)
	}
}

func TestGetEnv(t *testing.T) {
	// Test with set environment variable
	os.Setenv("TEST_VAR", "test_value")
	defer os.Unsetenv("TEST_VAR")

	value := getEnv("TEST_VAR", "default")
	if value != "test_value" {
		t.Errorf("Expected test_value, got %v", value)
	}

	// Test with unset environment variable
	value = getEnv("UNSET_VAR", "default")
	if value != "default" {
		t.Errorf("Expected default, got %v", value)
	}
}

func TestGetEnvInt(t *testing.T) {
	// Test with valid integer environment variable
	os.Setenv("TEST_INT", "42")
	defer os.Unsetenv("TEST_INT")

	value := getEnvInt("TEST_INT", 10)
	if value != 42 {
		t.Errorf("Expected 42, got %v", value)
	}

	// Test with invalid integer environment variable
	os.Setenv("TEST_INVALID_INT", "not_a_number")
	defer os.Unsetenv("TEST_INVALID_INT")

	value = getEnvInt("TEST_INVALID_INT", 10)
	if value != 10 {
		t.Errorf("Expected default value 10 for invalid int, got %v", value)
	}

	// Test with unset environment variable
	value = getEnvInt("UNSET_INT_VAR", 20)
	if value != 20 {
		t.Errorf("Expected default value 20, got %v", value)
	}
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	// Create temporary file with invalid YAML
	invalidYAML := `
server:
  address: ":8080"
  invalid yaml syntax here
backends:
  - name: "test"
    url
`

	tmpFile, err := os.CreateTemp("", "invalid_config*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(invalidYAML)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	os.Setenv("GATEKEEPER_CONFIG", tmpFile.Name())
	defer os.Unsetenv("GATEKEEPER_CONFIG")

	_, err = Load()
	if err == nil {
		t.Error("Expected error loading invalid YAML config, got nil")
	}
}

func TestConfigValidation(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test that required fields have sensible defaults
	if cfg.Server.Address == "" {
		t.Error("Server address should not be empty")
	}

	if cfg.Server.ReadTimeout <= 0 {
		t.Error("Read timeout should be positive")
	}

	if cfg.RateLimit.RequestsPerMinute <= 0 {
		t.Error("Rate limit should be positive")
	}

	if len(cfg.Backends) == 0 {
		t.Error("Should have at least one backend configured")
	}

	for _, backend := range cfg.Backends {
		if backend.Name == "" {
			t.Error("Backend name should not be empty")
		}
		if backend.URL == "" {
			t.Error("Backend URL should not be empty")
		}
		if backend.Weight < 0 {
			t.Error("Backend weight should not be negative")
		}
	}
}