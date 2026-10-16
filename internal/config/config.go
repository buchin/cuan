package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Environment       string
	ServerPort        string
	DatabaseURL       string
	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnMaxLifetime time.Duration
	DBConnMaxIdleTime time.Duration
	AutoMigrate       bool
	StaticDir         string
	RunnerInterval    time.Duration
	RunnerID          string
	AuthSecret        string
}

func Load() *Config {
	env := getEnv("CUAN_ENV", "development")
	port := getEnv("PORT", "8080")
	dbURL := getEnv("DATABASE_URL", "postgres://cuan:cuan@localhost:5432/cuan?sslmode=disable")
	runnerID := getEnv("CUAN_RUNNER_ID", "runner-01")
	authSecret := getEnv("CUAN_AUTH_SECRET", "dev-secret-change-in-production")
	staticDir := getEnv("CUAN_STATIC_DIR", "web/dist")

	intervalSec, _ := strconv.Atoi(getEnv("CUAN_RUNNER_INTERVAL_SEC", "5"))
	if intervalSec < 1 {
		intervalSec = 5
	}

	maxOpenConns, _ := strconv.Atoi(getEnv("CUAN_DB_MAX_OPEN_CONNS", "25"))
	if maxOpenConns < 1 {
		maxOpenConns = 25
	}

	maxIdleConns, _ := strconv.Atoi(getEnv("CUAN_DB_MAX_IDLE_CONNS", "10"))
	if maxIdleConns < 1 {
		maxIdleConns = 10
	}

	connMaxLifetime, err := time.ParseDuration(getEnv("CUAN_DB_CONN_MAX_LIFETIME", "5m"))
	if err != nil {
		connMaxLifetime = 5 * time.Minute
	}

	connMaxIdleTime, err := time.ParseDuration(getEnv("CUAN_DB_CONN_MAX_IDLE_TIME", "1m"))
	if err != nil {
		connMaxIdleTime = 1 * time.Minute
	}

	autoMigrate := getEnv("CUAN_AUTO_MIGRATE", "true") != "false"

	return &Config{
		Environment:       env,
		ServerPort:        port,
		DatabaseURL:       dbURL,
		DBMaxOpenConns:    maxOpenConns,
		DBMaxIdleConns:    maxIdleConns,
		DBConnMaxLifetime: connMaxLifetime,
		DBConnMaxIdleTime: connMaxIdleTime,
		AutoMigrate:       autoMigrate,
		StaticDir:         staticDir,
		RunnerInterval:    time.Duration(intervalSec) * time.Second,
		RunnerID:          runnerID,
		AuthSecret:        authSecret,
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
