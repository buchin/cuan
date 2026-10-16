package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Environment    string
	ServerPort     string
	DatabaseURL    string
	RunnerInterval time.Duration
	RunnerID       string
	AuthSecret     string
}

func Load() *Config {
	env := getEnv("CUAN_ENV", "development")
	port := getEnv("PORT", "8080")
	dbURL := getEnv("DATABASE_URL", "postgres://cuan:cuan@localhost:5432/cuan?sslmode=disable")
	runnerID := getEnv("CUAN_RUNNER_ID", "runner-01")
	authSecret := getEnv("CUAN_AUTH_SECRET", "dev-secret-change-in-production")

	intervalSec, _ := strconv.Atoi(getEnv("CUAN_RUNNER_INTERVAL_SEC", "5"))
	if intervalSec < 1 {
		intervalSec = 5
	}

	return &Config{
		Environment:    env,
		ServerPort:     port,
		DatabaseURL:    dbURL,
		RunnerInterval: time.Duration(intervalSec) * time.Second,
		RunnerID:       runnerID,
		AuthSecret:     authSecret,
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
