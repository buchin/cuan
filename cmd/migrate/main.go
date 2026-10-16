package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/buchin/cuan/internal/db"
)

func main() {
	var (
		dbURL     string
		direction string
		dryRun    bool
		showHelp  bool
	)

	flag.StringVar(&dbURL, "db-url", os.Getenv("DATABASE_URL"), "PostgreSQL connection URL (or set DATABASE_URL)")
	flag.StringVar(&direction, "direction", "up", "Migration direction: up or down")
	flag.BoolVar(&dryRun, "dry-run", false, "Print SQL statements without applying to database")
	flag.BoolVar(&showHelp, "help", false, "Show help and usage")
	flag.Parse()

	if showHelp {
		printUsage()
		os.Exit(0)
	}

	if dryRun {
		plan, err := db.DryRun(direction)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating migration dry-run: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(plan)
		fmt.Printf("Dry-run completed successfully for direction: %s\n", direction)
		os.Exit(0)
	}

	if dbURL == "" {
		fmt.Fprintln(os.Stderr, "Error: DATABASE_URL is not set and --db-url was not provided.")
		fmt.Fprintln(os.Stderr, "Tip: To preview migration SQL statements without a database, use: cuan-migrate --dry-run")
		os.Exit(1)
	}

	database, err := sql.Open("pgx", dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := database.PingContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error reaching database at %s: %v\n", dbURL, err)
		fmt.Fprintln(os.Stderr, "Tip: Verify that PostgreSQL is running, or run 'cuan-migrate --dry-run' to inspect SQL.")
		os.Exit(1)
	}

	if err := db.Run(ctx, database, direction); err != nil {
		fmt.Fprintf(os.Stderr, "Error executing migration (%s): %v\n", direction, err)
		os.Exit(1)
	}

	fmt.Printf("Cuan database migrations applied successfully (%s).\n", direction)
}

func printUsage() {
	fmt.Println(`cuan-migrate — Cuan Database Migration CLI

Usage:
  cuan-migrate [flags]

Flags:
  --db-url string     PostgreSQL connection URL (default: $DATABASE_URL)
  --direction string  Migration direction: 'up' or 'down' (default: "up")
  --dry-run           Print SQL statements without applying to database
  --help              Show this help message

Examples:
  # Inspect baseline migration SQL without database:
  cuan-migrate --dry-run

  # Apply pending migrations to local Postgres:
  cuan-migrate --db-url "postgres://postgres:postgres@localhost:5432/cuan?sslmode=disable"

  # Rollback last migration:
  cuan-migrate --direction down`)
}
