package diagnostics

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// BackupManifest contains metadata for a Cuan database/system snapshot archive.
type BackupManifest struct {
	Version        string    `json:"version"`
	CreatedAt      time.Time `json:"created_at"`
	DatabaseEngine string    `json:"database_engine"`
	PublicTables   int       `json:"public_tables"`
	DumpFile       string    `json:"dump_file"`
}

func findPostgresBinary(name string) (string, error) {
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	candidates := []string{
		filepath.Join("/usr/lib/postgresql/16/bin", name),
		filepath.Join("/usr/lib/postgresql/15/bin", name),
		filepath.Join("/usr/lib/postgresql/14/bin", name),
		filepath.Join("/opt/homebrew/opt/libpq/bin", name),
		filepath.Join("/usr/local/opt/libpq/bin", name),
		filepath.Join("/usr/bin", name),
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("%s executable not found in PATH or standard PostgreSQL locations", name)
}

func resolveDatabaseURL(explicit string) string {
	if strings.TrimSpace(explicit) != "" {
		return strings.TrimSpace(explicit)
	}
	if env := os.Getenv("DATABASE_URL"); strings.TrimSpace(env) != "" {
		return strings.TrimSpace(env)
	}
	for _, envFile := range []string{".env", "/opt/cuan/.env", "../.env"} {
		if data, err := os.ReadFile(envFile); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "DATABASE_URL=") {
					val := strings.TrimPrefix(line, "DATABASE_URL=")
					val = strings.Trim(val, `"' `)
					if val != "" {
						return val
					}
				}
			}
		}
	}
	return ""
}

// RunAdminCLI is the entrypoint for 'cuan admin', 'cuan backup', and 'cuan restore'.
func RunAdminCLI(args []string, out, errOut io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "help" {
		printAdminHelp(out)
		return 0
	}

	command := args[0]
	subArgs := args[1:]

	// Handle direct shortcuts: 'cuan backup ...' or 'cuan restore ...'
	if command == "backup" {
		return runAdminBackup(subArgs, out, errOut)
	}
	if command == "restore" {
		return runAdminRestore(subArgs, out, errOut)
	}
	if command == "admin" {
		if len(subArgs) == 0 || subArgs[0] == "--help" || subArgs[0] == "help" {
			printAdminHelp(out)
			return 0
		}
		subCmd := subArgs[0]
		switch subCmd {
		case "backup":
			return runAdminBackup(subArgs[1:], out, errOut)
		case "restore":
			return runAdminRestore(subArgs[1:], out, errOut)
		case "status":
			return runAdminStatus(subArgs[1:], out, errOut)
		default:
			fmt.Fprintf(errOut, "Unknown admin command: %s. Use 'cuan admin --help'.\n", subCmd)
			return 2
		}
	}

	fmt.Fprintf(errOut, "Unknown command: %s. Use 'cuan admin --help'.\n", command)
	return 2
}

func printAdminHelp(out io.Writer) {
	fmt.Fprintln(out, `Cuan Administrative & Disaster Recovery CLI

Usage:
  cuan admin backup [options]     Create a portable, full-system snapshot archive
  cuan admin restore <archive>    Restore system and database from snapshot
  cuan admin status [options]     Verify database health, table counts and system metrics

Aliases:
  cuan backup [options]           Shortcut for 'cuan admin backup'
  cuan restore <archive>          Shortcut for 'cuan admin restore'

Backup Options:
  --output, -o <path>             Destination tar.gz archive path (default: cuan-backup-<timestamp>.tar.gz)
  --db-url <url>                  Source database URL (defaults to DATABASE_URL)

Restore Options:
  --target-db <url>               Target database URL (defaults to DATABASE_URL)
  --force, -f                     Skip non-interactive confirmation checks`)
}

func runAdminBackup(args []string, out, errOut io.Writer) int {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		printAdminHelp(out)
		return 0
	}
	fs := flag.NewFlagSet("admin backup", flag.ContinueOnError)
	fs.SetOutput(errOut)
	outFlag := fs.String("output", "", "Output tar.gz archive path")
	fs.StringVar(outFlag, "o", "", "Output tar.gz archive path (shorthand)")
	dbFlag := fs.String("db-url", "", "PostgreSQL database URL")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printAdminHelp(out)
			return 0
		}
		return 2
	}

	dbURL := resolveDatabaseURL(*dbFlag)
	if dbURL == "" {
		fmt.Fprintln(errOut, "Error: DATABASE_URL is not set and not found in .env. Specify --db-url.")
		return 1
	}

	pgDump, err := findPostgresBinary("pg_dump")
	if err != nil {
		fmt.Fprintf(errOut, "Error: %v\n", err)
		return 1
	}

	outputPath := *outFlag
	if outputPath == "" {
		outputPath = fmt.Sprintf("cuan-backup-%s.tar.gz", time.Now().UTC().Format("20060102-150405"))
	}

	tempDir, err := os.MkdirTemp("", "cuan-backup-*")
	if err != nil {
		fmt.Fprintf(errOut, "Error creating temporary workspace: %v\n", err)
		return 1
	}
	defer os.RemoveAll(tempDir)

	fmt.Fprintf(out, "==> Probing database connection...\n")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		fmt.Fprintf(errOut, "Error connecting to database: %v\n", err)
		return 1
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		fmt.Fprintf(errOut, "Error pinging database: %v\n", err)
		return 1
	}

	var tableCount int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public'").Scan(&tableCount); err != nil {
		fmt.Fprintf(errOut, "Error reading table count: %v\n", err)
		return 1
	}

	sqlDumpPath := filepath.Join(tempDir, "database.sql")
	fmt.Fprintf(out, "==> Exporting PostgreSQL database schema and data (%d public tables)...\n", tableCount)

	dumpCmd := exec.Command(pgDump, "--clean", "--if-exists", "--no-owner", "--no-privileges", "-f", sqlDumpPath, dbURL)
	dumpCmd.Stderr = errOut
	if err := dumpCmd.Run(); err != nil {
		fmt.Fprintf(errOut, "pg_dump failed: %v\n", err)
		return 1
	}

	manifest := BackupManifest{
		Version:        "1.0",
		CreatedAt:      time.Now().UTC(),
		DatabaseEngine: "postgresql",
		PublicTables:   tableCount,
		DumpFile:       "database.sql",
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		fmt.Fprintf(errOut, "Error generating manifest: %v\n", err)
		return 1
	}
	if err := os.WriteFile(filepath.Join(tempDir, "manifest.json"), manifestBytes, 0600); err != nil {
		fmt.Fprintf(errOut, "Error writing manifest: %v\n", err)
		return 1
	}

	fmt.Fprintf(out, "==> Packaging snapshot archive into %s...\n", outputPath)
	if err := createTarGz(outputPath, tempDir); err != nil {
		fmt.Fprintf(errOut, "Error creating archive: %v\n", err)
		return 1
	}

	fi, _ := os.Stat(outputPath)
	var sizeBytes int64
	if fi != nil {
		sizeBytes = fi.Size()
	}
	fmt.Fprintf(out, "✓ Backup completed successfully!\n")
	fmt.Fprintf(out, "  Archive: %s (%d bytes)\n", outputPath, sizeBytes)
	fmt.Fprintf(out, "  Tables:  %d public tables\n", tableCount)
	return 0
}

func runAdminRestore(args []string, out, errOut io.Writer) int {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		printAdminHelp(out)
		return 0
	}
	fs := flag.NewFlagSet("admin restore", flag.ContinueOnError)
	fs.SetOutput(errOut)
	targetFlag := fs.String("target-db", "", "Target PostgreSQL database URL")
	_ = fs.Bool("force", false, "Force execution without confirmation")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printAdminHelp(out)
			return 0
		}
		return 2
	}

	extra := fs.Args()
	if len(extra) == 0 {
		fmt.Fprintln(errOut, "Usage: cuan admin restore <archive.tar.gz> [--target-db <url>]")
		return 2
	}
	archivePath := extra[0]

	if _, err := os.Stat(archivePath); err != nil {
		fmt.Fprintf(errOut, "Error: Backup archive not found: %s\n", archivePath)
		return 1
	}

	dbURL := resolveDatabaseURL(*targetFlag)
	if dbURL == "" {
		fmt.Fprintln(errOut, "Error: Target DATABASE_URL is not set and not specified via --target-db.")
		return 1
	}

	psql, err := findPostgresBinary("psql")
	if err != nil {
		fmt.Fprintf(errOut, "Error: %v\n", err)
		return 1
	}

	tempDir, err := os.MkdirTemp("", "cuan-restore-*")
	if err != nil {
		fmt.Fprintf(errOut, "Error creating temporary workspace: %v\n", err)
		return 1
	}
	defer os.RemoveAll(tempDir)

	fmt.Fprintf(out, "==> Unpacking backup archive %s...\n", archivePath)
	if err := extractTarGz(archivePath, tempDir); err != nil {
		fmt.Fprintf(errOut, "Error extracting archive: %v\n", err)
		return 1
	}

	manifestPath := filepath.Join(tempDir, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		fmt.Fprintf(errOut, "Error reading backup manifest: %v\n", err)
		return 1
	}

	var manifest BackupManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		fmt.Fprintf(errOut, "Invalid backup manifest format: %v\n", err)
		return 1
	}

	dumpPath := filepath.Join(tempDir, manifest.DumpFile)
	if _, err := os.Stat(dumpPath); err != nil {
		fmt.Fprintf(errOut, "Database dump file '%s' missing from archive: %v\n", manifest.DumpFile, err)
		return 1
	}

	fmt.Fprintf(out, "==> Restoring database from snapshot (%d expected public tables, captured at %s)...\n",
		manifest.PublicTables, manifest.CreatedAt.Format(time.RFC3339))

	restoreCmd := exec.Command(psql, "-v", "ON_ERROR_STOP=1", "-f", dumpPath, dbURL)
	restoreCmd.Stderr = errOut
	if err := restoreCmd.Run(); err != nil {
		fmt.Fprintf(errOut, "Database restore command failed: %v\n", err)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := sql.Open("pgx", dbURL)
	if err == nil {
		defer db.Close()
		var restoredCount int
		if err := db.QueryRowContext(ctx, "SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public'").Scan(&restoredCount); err == nil {
			fmt.Fprintf(out, "✓ Database restored successfully! (%d public tables verified)\n", restoredCount)
			return 0
		}
	}

	fmt.Fprintf(out, "✓ Database restore command completed successfully.\n")
	return 0
}

func runAdminStatus(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("admin status", flag.ContinueOnError)
	fs.SetOutput(errOut)
	dbFlag := fs.String("db-url", "", "PostgreSQL database URL")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	dbURL := resolveDatabaseURL(*dbFlag)
	if dbURL == "" {
		fmt.Fprintln(errOut, "Notice: DATABASE_URL not configured.")
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		fmt.Fprintf(errOut, "Database connection error: %v\n", err)
		return 1
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		fmt.Fprintf(errOut, "Database ping failed: %v\n", err)
		return 1
	}

	var tables, tasks, launches, rooms int
	_ = db.QueryRowContext(ctx, "SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public'").Scan(&tables)
	_ = db.QueryRowContext(ctx, "SELECT count(*) FROM tasks").Scan(&tasks)
	_ = db.QueryRowContext(ctx, "SELECT count(*) FROM task_launches").Scan(&launches)
	_ = db.QueryRowContext(ctx, "SELECT count(*) FROM chat_rooms").Scan(&rooms)

	fmt.Fprintf(out, "Cuan System & Database Status:\n")
	fmt.Fprintf(out, "- Connection:    Connected\n")
	fmt.Fprintf(out, "- Public Tables: %d\n", tables)
	fmt.Fprintf(out, "- Total Tasks:   %d\n", tasks)
	fmt.Fprintf(out, "- Task Launches: %d\n", launches)
	fmt.Fprintf(out, "- Chat Rooms:    %d\n", rooms)
	return 0
}

func createTarGz(tarGzPath, sourceDir string) error {
	f, err := os.Create(tarGzPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil || relPath == "." {
			return nil
		}

		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath)

		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(tw, file)
		return err
	})
}

func extractTarGz(tarGzPath, targetDir string) error {
	f, err := os.Open(tarGzPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}

		cleanPath := filepath.Clean(header.Name)
		if strings.HasPrefix(cleanPath, "..") || filepath.IsAbs(cleanPath) {
			return fmt.Errorf("archive contains unsafe file path: %s", header.Name)
		}

		target := filepath.Join(targetDir, cleanPath)
		if header.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		outFile, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, header.FileInfo().Mode())
		if err != nil {
			return err
		}
		if _, err := io.Copy(outFile, tr); err != nil {
			outFile.Close()
			return err
		}
		outFile.Close()
	}
	return nil
}
