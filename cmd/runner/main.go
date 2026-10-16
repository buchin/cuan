package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/runner"
)

func main() {
	showHelp := flag.Bool("help", false, "Show help message")
	agentBin := flag.String("agent-bin", "", "Custom path to agent binary (default: searches PATH for 'agy')")
	serverURL := flag.String("server-url", "", "Cuan server URL (default: CUAN_SERVER_URL or http://localhost:8080)")
	runnerID := flag.String("runner-id", "", "Runner unique ID (default: CUAN_RUNNER_ID or runner-01)")
	runnerToken := flag.String("runner-token", "", "Runner authentication token (default: CUAN_RUNNER_TOKEN)")
	flag.Parse()

	if *showHelp {
		fmt.Println(`cuan-runner — Cuan Agent Execution Runner Daemon

Usage:
  cuan-runner [flags]

Flags:
  --agent-bin string      Custom path to coding agent binary (default: searches PATH for 'agy')
  --server-url string     Cuan API server URL (default: http://localhost:8080)
  --runner-id string      Runner identifier (default: runner-01)
  --runner-token string   Runner authentication token
  --help                  Show this help message

Environment variables:
  CUAN_RUNNER_ID            Unique identifier for this runner (default: runner-01)
  CUAN_RUNNER_TOKEN         Authentication token shared with Cuan server
  CUAN_SERVER_URL           Cuan API server URL (default: http://localhost:8080)
  CUAN_RUNNER_INTERVAL_SEC  Queue polling interval in seconds (default: 5)
  DATABASE_URL              PostgreSQL connection string`)
		os.Exit(0)
	}

	cfg := config.Load()
	if *serverURL != "" {
		cfg.RunnerServerURL = *serverURL
	}
	if *runnerID != "" {
		cfg.RunnerID = *runnerID
	}
	if *runnerToken != "" {
		cfg.RunnerToken = *runnerToken
	}

	adapter, err := agent.NewAGYAdapter(*agentBin)
	if err != nil {
		log.Fatalf("Failed to initialize agent adapter: %v", err)
	}

	ver, err := adapter.Version()
	if err != nil {
		log.Fatalf("Failed to verify agent version: %v", err)
	}
	log.Printf("Initialized agent adapter '%s' version: %s", adapter.Name(), ver)

	r := runner.NewRunner(cfg, adapter)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-shutdown
		log.Printf("Received signal %v, terminating runner...", sig)
		cancel()
	}()

	if err := r.Start(ctx); err != nil {
		log.Fatalf("Runner exited with error: %v", err)
	}

	// Graceful wait
	time.Sleep(100 * time.Millisecond)
	log.Println("Cuan runner stopped cleanly.")
}
