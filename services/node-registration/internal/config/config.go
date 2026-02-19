package config

import (
	"os"
	"time"
)

type Config struct {
	Port               string
	DatabaseURL        string
	RedisURL           string
	RedisAddr          string
	RedisPassword      string
	LogLevel           string
	HeartbeatInterval  time.Duration
	InactiveTimeout    time.Duration
}

func Load() *Config {
	return &Config{
		Port:          getEnv("PORT", "8001"),
		DatabaseURL:   getEnv("DATABASE_URL", "postgresql://iploop:securepassword123@localhost:5432/iploop"),
		RedisURL:      getEnv("REDIS_URL", "redis://localhost:6379"),
		RedisAddr:     getEnv("REDIS_ADDR", ""),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		LogLevel:      getEnv("LOG_LEVEL", "info"),
		HeartbeatInterval: parseDuration(getEnv("NODE_HEARTBEAT_INTERVAL", "30s")),
		InactiveTimeout:   parseDuration(getEnv("NODE_INACTIVE_TIMEOUT", "90s")),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 30 * time.Second // Default fallback
	}
	return d
}