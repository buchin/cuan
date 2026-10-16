package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/server"
)

func main() {
	showHelp := flag.Bool("help", false, "Show help message")
	flag.Parse()

	if *showHelp {
		fmt.Println(`cuan-server — Cuan API Server

Usage:
  cuan-server [flags]

Flags:
  --help    Show this help message

Environment variables:
  PORT            Port to listen on (default: 8080)
  DATABASE_URL    PostgreSQL connection string
  CUAN_ENV        Environment mode (development/production)`)
		os.Exit(0)
	}

	cfg := config.Load()
	srv := server.NewServer(cfg)

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
		log.Printf("Cuan API server listening on %s (env=%s)", addr, cfg.Environment)
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
		log.Println("Server stopped cleanly.")
	}
}
