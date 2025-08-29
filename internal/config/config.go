package config

import (
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig   `yaml:"server"`
	Backends  []Backend      `yaml:"backends"`
	RateLimit RateLimitConfig `yaml:"rateLimit"`
	LogLevel  string         `yaml:"logLevel"`
}

type ServerConfig struct {
	Address      string `yaml:"address"`
	ReadTimeout  int    `yaml:"readTimeout"`
	WriteTimeout int    `yaml:"writeTimeout"`
	IdleTimeout  int    `yaml:"idleTimeout"`
}

type Backend struct {
	Name   string `yaml:"name"`
	URL    string `yaml:"url"`
	Weight int    `yaml:"weight"`
	Health string `yaml:"health"`
}

type RateLimitConfig struct {
	RequestsPerMinute int `yaml:"requestsPerMinute"`
	BurstSize         int `yaml:"burstSize"`
}

func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Address:      getEnv("GATEKEEPER_ADDRESS", ":8080"),
			ReadTimeout:  getEnvInt("GATEKEEPER_READ_TIMEOUT", 30),
			WriteTimeout: getEnvInt("GATEKEEPER_WRITE_TIMEOUT", 30),
			IdleTimeout:  getEnvInt("GATEKEEPER_IDLE_TIMEOUT", 120),
		},
		RateLimit: RateLimitConfig{
			RequestsPerMinute: getEnvInt("GATEKEEPER_RATE_LIMIT", 100),
			BurstSize:         getEnvInt("GATEKEEPER_BURST_SIZE", 10),
		},
		LogLevel: getEnv("GATEKEEPER_LOG_LEVEL", "info"),
	}

	// Try to load from config file
	configFile := getEnv("GATEKEEPER_CONFIG", "config.yaml")
	if data, err := os.ReadFile(configFile); err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}

	// Set default backends if none configured
	if len(cfg.Backends) == 0 {
		cfg.Backends = []Backend{
			{
				Name:   "default",
				URL:    getEnv("GATEKEEPER_DEFAULT_BACKEND", "http://localhost:3000"),
				Weight: 100,
				Health: "/health",
			},
		}
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}