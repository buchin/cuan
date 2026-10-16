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
	RunnerInterval      time.Duration
	RunnerID            string
	RunnerToken         string
	RunnerServerURL     string
	RunnerLeaseDuration time.Duration
	RunnerMaxLogLines   int
	AuthSecret          string
	GitAuthorName       string
	GitAuthorEmail      string
	WorkspacesDir       string
	AppVersion          string
	DojoWebhookSecret         string
	DojoWaitingListWebhookURL string
	DojoCheckoutURL           string
	SponsorLogosDir           string
}

func Load() *Config {
	env := getEnv("CUAN_ENV", "development")
	port := getEnv("PORT", "8080")
	dbURL := getEnv("DATABASE_URL", "postgres://cuan:cuan@localhost:5432/cuan?sslmode=disable")
	runnerID := getEnv("CUAN_RUNNER_ID", "runner-01")
	authSecret := getEnv("CUAN_AUTH_SECRET", "dev-secret-change-in-production")
	runnerToken := getEnv("CUAN_RUNNER_TOKEN", authSecret)
	serverURL := getEnv("CUAN_SERVER_URL", "http://localhost:"+port)
	staticDir := getEnv("CUAN_STATIC_DIR", "web/dist")
	gitAuthorName := getEnv("GIT_AUTHOR_NAME", "Buchin")
	gitAuthorEmail := getEnv("GIT_AUTHOR_EMAIL", "buchin@dropsugar.com")
	workspacesDir := getEnv("CUAN_WORKSPACES_DIR", "data/workspaces")
	appVersion := getEnv("CUAN_APP_VERSION", "0.1.0")
	dojoWebhookSecret := getEnv("DOJO_WEBHOOK_SECRET", "dev-dojo-secret")
	dojoWaitingListWebhookURL := getEnv("DOJO_WAITING_LIST_WEBHOOK_URL", "")
	dojoCheckoutURL := getEnv("DOJO_CHECKOUT_URL", "https://checkout.dojo.id/products/cuan-founding-backer")
	sponsorLogosDir := getEnv("CUAN_SPONSOR_LOGOS_DIR", "data/sponsors/logos")

	intervalSec, _ := strconv.Atoi(getEnv("CUAN_RUNNER_INTERVAL_SEC", "5"))
	if intervalSec < 1 {
		intervalSec = 5
	}

	leaseSec, _ := strconv.Atoi(getEnv("CUAN_RUNNER_LEASE_DURATION_SEC", "30"))
	if leaseSec < 5 {
		leaseSec = 30
	}

	maxLogLines, _ := strconv.Atoi(getEnv("CUAN_RUNNER_MAX_LOG_LINES", "5000"))
	if maxLogLines < 100 {
		maxLogLines = 5000
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
		Environment:         env,
		ServerPort:          port,
		DatabaseURL:         dbURL,
		DBMaxOpenConns:      maxOpenConns,
		DBMaxIdleConns:      maxIdleConns,
		DBConnMaxLifetime:   connMaxLifetime,
		DBConnMaxIdleTime:   connMaxIdleTime,
		AutoMigrate:         autoMigrate,
		StaticDir:           staticDir,
		RunnerInterval:      time.Duration(intervalSec) * time.Second,
		RunnerID:            runnerID,
		RunnerToken:         runnerToken,
		RunnerServerURL:     serverURL,
		RunnerLeaseDuration: time.Duration(leaseSec) * time.Second,
		RunnerMaxLogLines:   maxLogLines,
		AuthSecret:          authSecret,
		GitAuthorName:       gitAuthorName,
		GitAuthorEmail:      gitAuthorEmail,
		WorkspacesDir:       workspacesDir,
		AppVersion:                appVersion,
		DojoWebhookSecret:         dojoWebhookSecret,
		DojoWaitingListWebhookURL: dojoWaitingListWebhookURL,
		DojoCheckoutURL:           dojoCheckoutURL,
		SponsorLogosDir:           sponsorLogosDir,
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
