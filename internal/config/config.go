package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// GitCommit can be populated at build time using ldflags:
// -ldflags "-X github.com/buchin/cuan/internal/config.GitCommit=abc1234"
var GitCommit = ""

// SourceRepository is embedded by release builders, never project environment.
// An unknown source stays empty and cannot identify a project's live result.
var SourceRepository = ""

var buildSourceIdentity = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]*/[a-z0-9][a-z0-9_.-]*$`)

func (c *Config) SourceRepositoryURL() string {
	if c == nil || !buildSourceIdentity.MatchString(c.SourceRepository) {
		return ""
	}
	return "https://github.com/" + c.SourceRepository
}

type Config struct {
	Environment               string
	ServerPort                string
	DatabaseURL               string
	DBMaxOpenConns            int
	DBMaxIdleConns            int
	DBConnMaxLifetime         time.Duration
	DBConnMaxIdleTime         time.Duration
	AutoMigrate               bool
	StaticDir                 string
	RunnerInterval            time.Duration
	RunnerID                  string
	RunnerToken               string
	RunnerServerURL           string
	RunnerLeaseDuration       time.Duration
	RunnerMaxLogLines         int
	RunnerDetachedWorkers     bool
	RunnerWorkerRegistryDir   string
	RunnerBinPath             string
	AuthSecret                string
	GitAuthorName             string
	GitAuthorEmail            string
	WorkspacesDir             string
	ProjectsDir               string
	AppVersion                string
	CommitSHA                 string
	SourceRepository          string
	DojoWebhookSecret         string
	DojoWaitingListWebhookURL string
	DojoCheckoutURL           string
	SponsorLogosDir           string
	ReadmePath                string
	SponsorsArchiveDir        string
	AutoSyncReadme            bool
	SyncGitCommit             bool
	BaseDomain                string
	CaddyAdminURL             string
	AppMode                   string
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
	gitAuthorName := getEnv("GIT_AUTHOR_NAME", "")
	gitAuthorEmail := getEnv("GIT_AUTHOR_EMAIL", "")
	workspacesDir := getEnv("CUAN_WORKSPACES_DIR", "data/workspaces")
	projectsDir := getEnv("CUAN_PROJECTS_DIR", "")
	if projectsDir == "" && workspacesDir != "" {
		projectsDir = filepath.Join(filepath.Dir(workspacesDir), "projects")
	}
	if projectsDir == "" {
		projectsDir = filepath.Join("data", "projects")
	}
	appVersion := getEnv("CUAN_APP_VERSION", "0.1.0")
	commitSHA := GitCommit
	if commitSHA == "" {
		commitSHA = getEnv("CUAN_COMMIT_SHA", "")
	}
	if commitSHA == "" {
		candidates := []string{
			"release.json",
			filepath.Join("..", "release.json"),
			"/opt/cuan/current/release.json",
		}
		for _, candidate := range candidates {
			if data, err := os.ReadFile(candidate); err == nil && len(data) > 0 {
				var manifest struct {
					SHA string `json:"sha"`
				}
				if json.Unmarshal(data, &manifest) == nil && manifest.SHA != "" {
					commitSHA = manifest.SHA
					break
				}
			}
		}
	}
	dojoWebhookSecret := getEnv("DOJO_WEBHOOK_SECRET", "dev-dojo-secret")
	dojoWaitingListWebhookURL := getEnv("DOJO_WAITING_LIST_WEBHOOK_URL", "")
	dojoCheckoutURL := getEnv("DOJO_CHECKOUT_URL", "https://checkout.dojo.id/products/cuan-founding-backer")
	sponsorLogosDir := getEnv("CUAN_SPONSOR_LOGOS_DIR", "data/sponsors/logos")
	readmePath := getEnv("CUAN_README_PATH", "README.md")
	sponsorsArchiveDir := getEnv("CUAN_SPONSORS_ARCHIVE_DIR", "sponsors")
	autoSyncReadme := getEnv("CUAN_AUTO_SYNC_README", "true") != "false"
	syncGitCommit := getEnv("CUAN_SYNC_GIT_COMMIT", "false") == "true"
	baseDomain := getEnv("BASE_DOMAIN", getEnv("CUAN_BASE_DOMAIN", "cuan.dojo.cc"))
	caddyAdminURL := getEnv("CADDY_ADMIN_URL", "http://localhost:2019")
	appMode := strings.ToLower(strings.TrimSpace(getEnv("APP_MODE", "marketing")))
	if appMode != "studio" {
		appMode = "marketing"
	}

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

	runnerDetachedWorkers := getEnv("CUAN_RUNNER_DETACHED_WORKERS", "true") != "false"
	runnerWorkerRegistryDir := getEnv("CUAN_WORKER_REGISTRY_DIR", "/tmp/cuan-workers")
	runnerBinPath := getEnv("CUAN_RUNNER_BIN", "/opt/cuan/current/bin/cuan-runner")

	return &Config{
		Environment:               env,
		ServerPort:                port,
		DatabaseURL:               dbURL,
		DBMaxOpenConns:            maxOpenConns,
		DBMaxIdleConns:            maxIdleConns,
		DBConnMaxLifetime:         connMaxLifetime,
		DBConnMaxIdleTime:         connMaxIdleTime,
		AutoMigrate:               autoMigrate,
		StaticDir:                 staticDir,
		RunnerInterval:            time.Duration(intervalSec) * time.Second,
		RunnerID:                  runnerID,
		RunnerToken:               runnerToken,
		RunnerServerURL:           serverURL,
		RunnerLeaseDuration:       time.Duration(leaseSec) * time.Second,
		RunnerMaxLogLines:         maxLogLines,
		RunnerDetachedWorkers:     runnerDetachedWorkers,
		RunnerWorkerRegistryDir:   runnerWorkerRegistryDir,
		RunnerBinPath:             runnerBinPath,
		AuthSecret:                authSecret,
		GitAuthorName:             gitAuthorName,
		GitAuthorEmail:            gitAuthorEmail,
		WorkspacesDir:             workspacesDir,
		ProjectsDir:               projectsDir,
		AppVersion:                appVersion,
		CommitSHA:                 commitSHA,
		SourceRepository:          SourceRepository,
		DojoWebhookSecret:         dojoWebhookSecret,
		DojoWaitingListWebhookURL: dojoWaitingListWebhookURL,
		DojoCheckoutURL:           dojoCheckoutURL,
		SponsorLogosDir:           sponsorLogosDir,
		ReadmePath:                readmePath,
		SponsorsArchiveDir:        sponsorsArchiveDir,
		AutoSyncReadme:            autoSyncReadme,
		SyncGitCommit:             syncGitCommit,
		BaseDomain:                baseDomain,
		CaddyAdminURL:             caddyAdminURL,
		AppMode:                   appMode,
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
