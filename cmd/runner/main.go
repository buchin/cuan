package main

import (
	"context"
	"encoding/json"
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
	"github.com/buchin/cuan/internal/taskcli"
	"github.com/buchin/cuan/internal/terminal"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "task" {
		os.Exit(taskcli.Run(os.Args[2:], os.Stdout, os.Stderr))
	}
	// Detached project terminal worker execution (Survives deploy via SocketBridge)
	if len(os.Args) > 1 && os.Args[1] == "worker-project" {
		os.Exit(terminal.RunProjectWorkerCLI(os.Args[2:]))
	}
	// Detached worker process execution (Pola #1: Detached Worker Process)
	if len(os.Args) > 1 && os.Args[1] == "worker-task" {
		workerFlags := flag.NewFlagSet("worker-task", flag.ExitOnError)
		claimFile := workerFlags.String("claim-file", "", "Path to claim JSON file")
		registryDir := workerFlags.String("registry-dir", "", "Path to worker registry directory")
		serverURL := workerFlags.String("server-url", "", "Cuan server URL")
		runnerID := workerFlags.String("runner-id", "", "Runner unique ID")
		runnerToken := workerFlags.String("runner-token", "", "Runner authentication token")
		agentBin := workerFlags.String("agent-bin", "", "Custom path to agent binary")
		_ = workerFlags.Parse(os.Args[2:])

		if *claimFile == "" {
			log.Fatalf("cuan-runner worker-task requires --claim-file")
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
		if *registryDir != "" {
			cfg.RunnerWorkerRegistryDir = *registryDir
		}

		claimData, err := os.ReadFile(*claimFile)
		if err != nil {
			log.Fatalf("Failed to read claim file %s: %v", *claimFile, err)
		}

		var claim runner.ClaimTaskResponse
		if err := json.Unmarshal(claimData, &claim); err != nil {
			log.Fatalf("Failed to parse claim JSON: %v", err)
		}

		adapter, err := agent.NewAGYAdapter(*agentBin)
		if err != nil {
			log.Fatalf("Failed to initialize agent adapter in detached worker: %v", err)
		}

		r := runner.NewRunner(cfg, adapter)
		if err := r.RunDetachedAttempt(context.Background(), claim); err != nil {
			log.Fatalf("Detached worker attempt failed: %v", err)
		}
		os.Exit(0)
	}

	showHelp := flag.Bool("help", false, "Show help message")
	showVersion := flag.Bool("version", false, "Show cuan-runner version")
	agentBin := flag.String("agent-bin", "", "Custom path to agent binary (default: searches PATH for 'agy')")
	serverURL := flag.String("server-url", "", "Cuan server URL (default: CUAN_SERVER_URL or http://localhost:8080)")
	runnerID := flag.String("runner-id", "", "Runner unique ID (default: CUAN_RUNNER_ID or runner-01)")
	runnerToken := flag.String("runner-token", "", "Runner authentication token (default: CUAN_RUNNER_TOKEN)")
	drainFile := flag.String("drain-file", "", "Path to drain flag file (default: CUAN_DRAIN_FILE or /tmp/cuan-drain)")
	targetBin := flag.String("binary-path", "", "Target binary path for self-update verification (default: CUAN_RUNNER_BIN or /opt/cuan/current/bin/cuan-runner)")
	flag.Parse()

	if *showVersion {
		cfg := config.Load()
		fmt.Printf("cuan-runner version %s\n", cfg.AppVersion)
		os.Exit(0)
	}

	if *showHelp {
		fmt.Println(`cuan-runner — Decoupled Cuan Task Worker Daemon

Usage:
  cuan-runner [flags]

Flags:
  --agent-bin string      Custom path to coding agent binary (default: searches PATH for 'agy')
  --server-url string     Cuan API server URL (default: http://localhost:8080)
  --runner-id string      Runner identifier (default: runner-01)
  --runner-token string   Runner authentication token
  --drain-file string     Path to drain flag file (default: /tmp/cuan-drain)
  --binary-path string    Target binary path for self-update (default: /opt/cuan/current/bin/cuan-runner)
  --version               Show cuan-runner version
  --help                  Show this help message

Environment variables:
  CUAN_RUNNER_ID            Unique identifier for this runner (default: runner-01)
  CUAN_RUNNER_TOKEN         Authentication token shared with Cuan server
  CUAN_SERVER_URL           Cuan API server URL (default: http://localhost:8080)
  CUAN_RUNNER_INTERVAL_SEC  Queue polling interval in seconds (default: 5)
  CUAN_DRAIN_FILE           Graceful drain flag file path (default: /tmp/cuan-drain)
  CUAN_RUNNER_BIN           Target binary path for disk self-update
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

	releaseLock, lockErr := runner.AcquireRunnerInstanceLock(cfg.RunnerWorkerRegistryDir, cfg.RunnerID)
	if lockErr != nil {
		log.Fatalf("Cannot start cuan-runner: %v", lockErr)
	}
	defer releaseLock()

	if *drainFile != "" {
		r.SetDrainCoordinator(runner.NewDrainCoordinator(*drainFile))
	}
	if *targetBin != "" {
		r.SetUpdateManager(runner.NewUpdateManager(*targetBin))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// SIGUSR1 triggers graceful drain without exiting immediately
	drainSig := make(chan os.Signal, 1)
	signal.Notify(drainSig, syscall.SIGUSR1)
	go func() {
		for range drainSig {
			log.Println("Received SIGUSR1 signal: activating graceful drain on worker...")
			r.RequestDrain("SIGUSR1 signal received")
		}
	}()

	// SIGUSR2 cancels graceful drain and resumes accepting tasks
	undrainSig := make(chan os.Signal, 1)
	signal.Notify(undrainSig, syscall.SIGUSR2)
	go func() {
		for range undrainSig {
			log.Println("Received SIGUSR2 signal: resuming worker task claims...")
			r.CancelDrain()
		}
	}()

	// SIGTERM / SIGINT initiates graceful drain and waits for in-flight tasks
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-shutdown
		log.Printf("Received signal %v, initiating graceful drain (press again to force kill)...", sig)
		r.RequestDrain(fmt.Sprintf("signal %v received", sig))
		cancel()

		// If a second signal arrives before drain finishes, force terminate
		<-shutdown
		log.Println("Received second termination signal, force terminating worker daemon.")
		os.Exit(130)
	}()

	if err := r.Start(ctx); err != nil {
		log.Fatalf("Runner exited with error: %v", err)
	}

	// Graceful wait
	time.Sleep(100 * time.Millisecond)
	log.Println("Cuan runner stopped cleanly.")
}
