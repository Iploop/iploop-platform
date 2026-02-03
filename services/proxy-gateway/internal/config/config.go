package config

import (
	"os"
)

type Config struct {
	HTTPPort      string
	SOCKSPort     string
	DatabaseURL   string
	RedisURL      string
	LogLevel      string
	NodeRegURL    string
}

func Load() *Config {
	return &Config{
		HTTPPort:    getEnv("HTTP_PORT", "7777"),
		SOCKSPort:   getEnv("SOCKS_PORT", "1080"),
		DatabaseURL: getEnv("DATABASE_URL", "postgresql://iploop:securepassword123@localhost:5432/iploop"),
		RedisURL:    getEnv("REDIS_URL", "redis://localhost:6379"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
		NodeRegURL:  getEnv("NODE_REGISTRATION_URL", "http://localhost:8001"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}