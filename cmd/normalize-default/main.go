// normalize-default separates a configured legacy project from neutral Default.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/buchin/cuan/internal/project"
	_ "github.com/jackc/pgx/v5/stdlib"
	"os"
	"path/filepath"
	"time"
)

func main() {
	apply := flag.Bool("apply", false, "Apply the separation; default is read-only preview")
	stopped := flag.Bool("service-stopped", false, "Confirm application service is stopped and a private backup exists")
	flag.Parse()
	if *apply && !*stopped {
		fmt.Fprintln(os.Stderr, "Stop the Cuan service and validate a private backup before using --apply --service-stopped.")
		os.Exit(1)
	}
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		fmt.Fprintln(os.Stderr, "Set DATABASE_URL privately in the environment.")
		os.Exit(1)
	}
	database, err := sql.Open("pgx", url)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Could not open the application database.")
		os.Exit(1)
	}
	defer database.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	result, err := project.NormalizeDefaultProject(ctx, database, *apply)
	if err != nil {
		home, _ := os.UserHomeDir()
		directory := filepath.Join(home, ".local/share/cuan")
		_ = os.MkdirAll(directory, 0700)
		logPath := filepath.Join(directory, "default-maintenance.log")
		if file, e := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600); e == nil {
			_ = file.Chmod(0600)
			fmt.Fprintf(file, "%s: %v\n", time.Now().UTC().Format(time.RFC3339), err)
			_ = file.Close()
		}
		fmt.Fprintln(os.Stderr, "Default separation did not complete; no changes were committed. Check the private default-maintenance.log for database access, active work or configuration conflicts.")
		os.Exit(1)
	}
	_ = json.NewEncoder(os.Stdout).Encode(result)
}
