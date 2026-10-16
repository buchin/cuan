package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/buchin/cuan/internal/config"
	dbPkg "github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/server"
)

func main() {
	showHelp := flag.Bool("help", false, "Show help message")
	showVersion := flag.Bool("version", false, "Show Cuan server version")
	healthCheck := flag.Bool("healthcheck", false, "Perform local health check against running server")
	readyCheck := flag.Bool("readycheck", false, "Perform local readiness check against running server (/readyz)")
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

	if *healthCheck {
		runHealthCheck("/healthz")
		return
	}

	if *readyCheck {
		runHealthCheck("/readyz")
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

	addr := ":" + cfg.ServerPort
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      srv.Router(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
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
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

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
	fmt.Println(`cuan-server — Cuan API Server

Usage:
  cuan-server [flags]

Flags:
  --healthcheck   Perform local liveness health check (/healthz) (exits 0 on 200 OK)
  --readycheck    Perform local readiness check (/readyz) (exits 0 on 200 OK)
  --version       Show Cuan server version
  --help          Show this help message

Environment variables:
  PORT                      Port to listen on (default: 8080)
  DATABASE_URL              PostgreSQL connection string
  CUAN_ENV                  Environment mode (development/production)
  CUAN_APP_VERSION          Application version string (default: 0.1.0)
  CUAN_AUTH_SECRET          Secret for single-owner session authentication
  CUAN_STATIC_DIR           Path to built web dist files (default: web/dist)
  CUAN_AUTO_MIGRATE         Auto-run migrations on boot (default: true)
  CUAN_DB_MAX_OPEN_CONNS    Max open database connections (default: 25)
  CUAN_DB_MAX_IDLE_CONNS    Max idle database connections (default: 10)`)
}
