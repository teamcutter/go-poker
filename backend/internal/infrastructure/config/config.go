package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	Port             string
	Environment      string
	PostgresDSN      string
	RedisAddr        string
	RedisPassword    string
	TelegramBotToken string
	TelegramAuthTTL  time.Duration
	SessionSecret    string
	PublicIDSecret   string
	SessionTTL       time.Duration
	JWTIssuer        string
	AppURL           string
	CORSOrigins      []string
}

func getListEnv(key string) []string {
	raw := os.Getenv(key)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:             getEnv("PORT", "8080"),
		Environment:      getEnv("ENV", "development"),
		PostgresDSN:      getEnv("DATABASE_URL", "postgres://petly:petly@localhost:5432/petly?sslmode=disable"),
		RedisAddr:        getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:    getEnv("REDIS_PASSWORD", ""),
		TelegramBotToken: getEnv("TELEGRAM_BOT_TOKEN", ""),
		TelegramAuthTTL:  getDurationEnv("TELEGRAM_AUTH_TTL", 24*time.Hour),
		SessionSecret:    getEnv("SESSION_SECRET", "change-me-in-production"),
		PublicIDSecret:   getEnv("PUBLIC_ID_SECRET", ""),
		SessionTTL:       getDurationEnv("SESSION_TTL", 7*24*time.Hour),
		JWTIssuer:        getEnv("JWT_ISSUER", "gopoker"),
		AppURL:           getEnv("APP_URL", "http://localhost:5173"),
		CORSOrigins:      getListEnv("CORS_ORIGINS"),
	}

	// Falling back keeps existing deployments working, at the cost of re-coupling
	// the two lifecycles: rotating SESSION_SECRET would then change player ids.
	if cfg.PublicIDSecret == "" {
		cfg.PublicIDSecret = cfg.SessionSecret
	}

	if cfg.SessionSecret == "change-me-in-production" && cfg.Environment != "development" {
		return nil, fmt.Errorf("SESSION_SECRET must be set in non-development environments")
	}
	if cfg.TelegramBotToken == "" && cfg.Environment != "development" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	if d, err := time.ParseDuration(v); err == nil {
		return d
	}
	return fallback
}
