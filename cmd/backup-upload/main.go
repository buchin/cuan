// Command backup-upload sends a backup file (and its .sha256 / .meta.json
// companions when present) to S3-compatible storage. Configuration comes from
// environment variables so secrets never appear in process arguments:
// CUAN_BACKUP_S3_ENDPOINT, _REGION, _BUCKET, _PREFIX, _ACCESS_KEY, _SECRET_KEY.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/offsite"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: backup-upload <backup.sql.gz>")
		os.Exit(2)
	}
	env := func(k string) string { return os.Getenv("CUAN_BACKUP_S3_" + k) }
	cli, err := offsite.New(offsite.Config{
		Endpoint: env("ENDPOINT"), Region: env("REGION"), Bucket: env("BUCKET"),
		Prefix: env("PREFIX"), AccessKey: env("ACCESS_KEY"), SecretKey: env("SECRET_KEY"),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "off-site config:", err)
		os.Exit(2)
	}
	main := os.Args[1]
	files := []string{main, main + ".sha256", strings.TrimSuffix(main, ".sql.gz") + ".meta.json"}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()
	for i, f := range files {
		if _, err := os.Stat(f); err != nil {
			if i == 0 {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			continue
		}
		key, err := cli.Put(ctx, filepath.Base(f), f)
		if err != nil {
			fmt.Fprintln(os.Stderr, "FAILED:", filepath.Base(f), err)
			os.Exit(1)
		}
		fmt.Println("uploaded", key)
	}
}
