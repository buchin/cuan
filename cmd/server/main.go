package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"github.com/buchin/cuan/internal/taskprocess"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/config"
	dbPkg "github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/diagnostics"
	"github.com/buchin/cuan/internal/server"
	"github.com/buchin/cuan/internal/sponsorship"
	"github.com/buchin/cuan/internal/taskcli"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == taskprocess.Command {
		os.Exit(taskprocess.Run(os.Args[2:], os.Stdin, os.Stdout, os.Stderr))
	}
	if len(os.Args) > 1 && os.Args[1] == "task" {
		hasCapability := os.Getenv("CUAN_TASK_CAPABILITY_FILE") != "" || os.Getenv("CUAN_TASK_CAPABILITY_JSON") != ""
		isRunnerAction := len(os.Args) > 2 && (os.Args[2] == "report" || os.Args[2] == "push" || (os.Args[2] == "get" && len(os.Args) == 3))
		if hasCapability && isRunnerAction {
			os.Exit(taskcli.Run(os.Args[2:], os.Stdout, os.Stderr))
		}
		os.Exit(diagnostics.RunTaskCLI(os.Args[2:], os.Stdout, os.Stderr))
	}
	if len(os.Args) > 1 && os.Args[1] == "chat" {
		os.Exit(diagnostics.RunChatCLI(os.Args[2:], os.Stdout, os.Stderr))
	}
	if len(os.Args) > 1 && os.Args[1] == "config" {
		os.Exit(diagnostics.RunConfigCLI(os.Args[2:], os.Stdout, os.Stderr))
	}
	if len(os.Args) > 1 && os.Args[1] == "repo" {
		os.Exit(diagnostics.RunRepoCLI(os.Args[2:], os.Stdout, os.Stderr))
	}
	if len(os.Args) > 1 && (os.Args[1] == "diag" || os.Args[1] == "health" || os.Args[1] == "diagnostics") {
		os.Exit(diagnostics.RunDiagCLI(os.Args[2:], os.Stdout, os.Stderr))
	}
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		os.Exit(diagnostics.RunMCP(os.Stdin, os.Stdout, os.Stderr))
	}
	showHelp := flag.Bool("help", false, "Show help message")
	showVersion := flag.Bool("version", false, "Show Cuan server version")
	showInfo := flag.Bool("info", false, "Show Cuan server metadata and release information")
	healthCheck := flag.Bool("healthcheck", false, "Perform local health check against running server")
	readyCheck := flag.Bool("readycheck", false, "Perform local readiness check against running server (/readyz)")
	syncSponsors := flag.Bool("sync-sponsors", false, "Synchronize README.md and sponsors permanent archive directory")
	detectAgents := flag.Bool("detect-agents", false, "Detect installed agent binaries (agy, claude, shell) on the host")
	pairAgent := flag.Bool("pair-agent", false, "Pair an agent CLI account using a device code")
	pairCode := flag.String("code", "", "Device pairing code (e.g. CUAN-8892)")
	agentIDFlag := flag.String("agent-id", "", "Agent ID to pair")
	memberIDFlag := flag.String("member-id", "buchin", "Member ID authorizing the agent")
	providerFlag := flag.String("provider", "antigravity", "Agent provider (antigravity, claude, shell)")
	flag.Parse()

	if *showHelp {
		printUsage()
		os.Exit(0)
	}

	if *showVersion {
		cfg := config.Load()
		fmt.Printf("cuan-server version %s\n", cfg.AppVersion)
		os.Exit(0)
	}

	if *showInfo {
		cfg := config.Load()
		fmt.Printf("================================================================================\n")
		fmt.Printf("Cuan Server — Agentic Project Management for Self-Hosted Teams\n")
		fmt.Printf("================================================================================\n")
		fmt.Printf("Version:        %s\n", cfg.AppVersion)
		fmt.Printf("License:        Apache-2.0\n")
		fmt.Printf("Repository:     https://github.com/buchin/cuan\n")
		fmt.Printf("Documentation:  https://github.com/buchin/cuan#readme\n")
		fmt.Printf("Author:         Buchin <buchin@dropsugar.com>\n")
		fmt.Printf("Release Stage:  Release Candidate (Sprint 1)\n")
		fmt.Printf("================================================================================\n")
		os.Exit(0)
	}

	if *healthCheck {
		runHealthCheck("/healthz")
		return
	}

	if *readyCheck {
		runHealthCheck("/readyz")
		return
	}

	if *detectAgents {
		bins := agent.DetectAgentBinaries()
		fmt.Printf("Detected host agent executables (%d candidates):\n", len(bins))
		for _, b := range bins {
			status := "NOT FOUND"
			if b.Available {
				status = fmt.Sprintf("AVAILABLE (%s)", b.Path)
				if b.Version != "" {
					status += fmt.Sprintf(" - %s", b.Version)
				}
			}
			fmt.Printf("  • %-10s : %s\n", b.Name, status)
		}
		os.Exit(0)
	}

	if *pairAgent {
		if *pairCode == "" {
			fmt.Fprintln(os.Stderr, "Error: --code is required for agent pairing (e.g. --code CUAN-8892)")
			os.Exit(1)
		}
		sessionStore := agent.NewMemorySessionStore()
		sess, err := sessionStore.AuthorizeDevicePairing(context.Background(), agent.AuthorizePairingRequest{
			DeviceCode: *pairCode,
			MemberID:   *memberIDFlag,
			AgentID:    *agentIDFlag,
			AgentName:  fmt.Sprintf("Agent %s (%s)", *agentIDFlag, *providerFlag),
			Provider:   *providerFlag,
			BinaryType: *providerFlag,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to authorize agent pairing: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Agent paired successfully!\n")
		fmt.Printf("  Session ID:  %s\n", sess.ID)
		fmt.Printf("  Member ID:   %s\n", sess.MemberID)
		fmt.Printf("  Agent ID:    %s\n", sess.AgentID)
		fmt.Printf("  Binary Type: %s\n", sess.BinaryType)
		fmt.Printf("  Device Code: %s\n", sess.DeviceCode)
		fmt.Printf("  Status:      %s\n", sess.Status)
		os.Exit(0)
	}

	if *syncSponsors {
		cfg := config.Load()
		var spStore sponsorship.Store
		if cfg.DatabaseURL != "" {
			if dbConn, err := sql.Open("pgx", cfg.DatabaseURL); err == nil {
				pingCtx, pingCancel := context.WithTimeout(context.Background(), 1*time.Second)
				if err := dbConn.PingContext(pingCtx); err == nil {
					spStore = sponsorship.NewDBStore(dbConn)
				}
				pingCancel()
			}
		}
		if spStore == nil {
			spStore = sponsorship.NewMemoryStore()
		}

		syncer := sponsorship.NewSyncer(spStore, sponsorship.SyncerConfig{
			ReadmePath:         cfg.ReadmePath,
			SponsorsArchiveDir: cfg.SponsorsArchiveDir,
			AutoGitCommit:      cfg.SyncGitCommit,
			AuthorName:         cfg.GitAuthorName,
			AuthorEmail:        cfg.GitAuthorEmail,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		res, err := syncer.Sync(ctx, "v1.0")
		if err != nil {
			log.Fatalf("Gagal sinkronisasi sponsor: %v", err)
		}
		fmt.Printf("✓ Sinkronisasi selesai: %s (sponsors: %d, files modified: %d)\n",
			res.Message, res.SponsorsCount, len(res.FilesModified))
		return
	}

	cfg := config.Load()

	// Initialize database connection and pool if configured
	var dbConn *sql.DB
	if cfg.DatabaseURL != "" {
		var err error
		dbConn, err = sql.Open("pgx", cfg.DatabaseURL)
		if err != nil {
			log.Printf("Warning: failed to initialize database driver for %s: %v", cfg.DatabaseURL, err)
		} else {
			dbConn.SetMaxOpenConns(cfg.DBMaxOpenConns)
			dbConn.SetMaxIdleConns(cfg.DBMaxIdleConns)
			dbConn.SetConnMaxLifetime(cfg.DBConnMaxLifetime)
			dbConn.SetConnMaxIdleTime(cfg.DBConnMaxIdleTime)

			if cfg.AutoMigrate {
				runAutoMigration(dbConn)
			}
		}
	}

	srv := server.NewServer(cfg, dbConn)

	// Start background remote worker health poller if configured
	if srv.HealthPoller() != nil {
		_ = srv.HealthPoller().Start(context.Background())
	}

	// Start background DNS health poller if configured
	if srv.DNSPoller() != nil {
		_ = srv.DNSPoller().Start(context.Background())
	}

	inboxCtx, stopInbox := context.WithCancel(context.Background())
	defer stopInbox()
	srv.StartInboxDispatcher(inboxCtx)
	srv.StartZombieReaper(inboxCtx)
	srv.StartNotificationDispatcher(inboxCtx)
	srv.StartWorkspaceRetention(inboxCtx)
	srv.StartGitHubRecovery(inboxCtx)

	addr := ":" + cfg.ServerPort
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      srv.Router(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 90 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("Cuan API server listening on %s (env=%s, static=%s)", addr, cfg.Environment, cfg.StaticDir)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		log.Fatalf("Server error: %v", err)
	case sig := <-shutdown:
		log.Printf("Received signal %v, initiating graceful shutdown...", sig)
		srv.PrepareGracefulShutdown(10 * time.Second)
		stopInbox()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if srv.HealthPoller() != nil {
			srv.HealthPoller().Stop()
		}

		if srv.DNSPoller() != nil {
			srv.DNSPoller().Stop()
		}

		if err := httpServer.Shutdown(ctx); err != nil {
			log.Printf("Graceful shutdown failed: %v", err)
			_ = httpServer.Close()
		}

		if dbConn != nil {
			_ = dbConn.Close()
		}
		log.Println("Server stopped cleanly.")
	}
}

func runAutoMigration(database *sql.DB) {
	log.Println("Checking database connectivity for auto-migration...")
	var connected bool

	// Retry loop for DB startup (e.g. docker-compose container boot)
	for i := 1; i <= 5; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		err := database.PingContext(ctx)
		cancel()

		if err == nil {
			connected = true
			break
		}

		log.Printf("Waiting for database to become available (attempt %d/5)...", i)
		time.Sleep(2 * time.Second)
	}

	if !connected {
		log.Println("Warning: database is unreachable. Server will start in degraded mode (readyz will report disconnected).")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := dbPkg.Run(ctx, database, "up"); err != nil {
		log.Printf("Warning: auto-migration failed: %v", err)
	} else {
		log.Println("PostgreSQL connected and schema migrations applied successfully.")
	}
}

func runHealthCheck(endpoint string) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}
	url := fmt.Sprintf("http://127.0.0.1:%s%s", port, endpoint)

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil || resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Check failed for %s: err=%v, status=%v\n", url, err, resp)
		os.Exit(1)
	}
	os.Exit(0)
}

func printUsage() {
	fmt.Println(`cuan-server / cuan — Agentic Project Management & Server Daemon

Usage:
  cuan-server [flags]
  cuan chat <subcommand> [flags]
  cuan task <subcommand> [flags]
  cuan config <subcommand> [flags]
  cuan repo <subcommand> [flags]
  cuan diag <subcommand> [flags]
  cuan mcp

Subcommands:
  chat              Agent chat & inbox conversations (new, send, list, messages)
  task              Task management (list, get, update, create) & runner reporting
  config            CLI and MCP connection settings (show, set)
  repo              Read-only git repository inspection (status, log, diff-summary, branches)
  diag              Host system health & static diagnostics (health, df, free, tsc, deps)
  mcp               Model Context Protocol (MCP) stdio server for Agent function calling

Flags:
  --healthcheck     Perform local liveness health check (/healthz) (exits 0 on 200 OK)
  --readycheck      Perform local readiness check (/readyz) (exits 0 on 200 OK)
  --sync-sponsors   Synchronize README.md and permanent sponsors archive directory
  --version         Show Cuan server version
  --info            Show Cuan server metadata and release information
  --help            Show this help message

Environment variables:
  PORT                      Port to listen on (default: 8080)
  DATABASE_URL              PostgreSQL connection string
  CUAN_ENV                  Environment mode (development/production)
  CUAN_APP_VERSION          Application version string (default: 0.1.0)
  CUAN_AUTH_SECRET          Secret for single-owner session authentication
  CUAN_STATIC_DIR           Path to built web dist files (default: web/dist)
  CUAN_AUTO_MIGRATE         Auto-run migrations on boot (default: true)
  CUAN_DB_MAX_OPEN_CONNS    Max open database connections (default: 25)
  CUAN_DB_MAX_IDLE_CONNS    Max idle database connections (default: 10)
  APP_MODE                  Application mode (marketing/studio) (default: marketing)`)
}
