package taskcli

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var attachmentIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,160}$`)

func runArtifact(args []string, out, errOut io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(errOut, "Usage: task artifact download <attachment-id> [--output FILENAME] | task artifact read <attachment-id> [--raw]")
		return 0
	}

	action := "download"
	subArgs := args
	if args[0] == "artifact" {
		subArgs = args[1:]
	} else if args[0] == "download" || args[0] == "download-artifact" {
		action = "download"
		subArgs = args[1:]
	}

	if len(subArgs) == 0 {
		fmt.Fprintln(errOut, "Usage: task artifact download <attachment-id> [--output FILENAME] | task artifact read <attachment-id> [--raw]")
		return 2
	}

	rawOutput := false
	switch subArgs[0] {
	case "download", "get":
		action = "download"
		subArgs = subArgs[1:]
	case "read":
		action = "read"
		subArgs = subArgs[1:]
	case "cat":
		action = "cat"
		rawOutput = true
		subArgs = subArgs[1:]
	default:
		if strings.HasPrefix(subArgs[0], "-") {
			action = "download"
		} else {
			action = "download"
		}
	}

	fs := flag.NewFlagSet("task artifact "+action, flag.ContinueOnError)
	fs.SetOutput(errOut)
	outputFlag := fs.String("output", "", "Destination filename inside attempt artifact directory")
	rawFlag := fs.Bool("raw", false, "Output raw contents or path instead of JSON")
	stdoutFlag := fs.Bool("stdout", false, "Stream raw content to stdout instead of saving to disk")

	var flagArgs []string
	var posArgs []string
	for i := 0; i < len(subArgs); i++ {
		arg := subArgs[i]
		if strings.HasPrefix(arg, "-") {
			flagArgs = append(flagArgs, arg)
			cleanFlag := strings.TrimLeft(arg, "-")
			if (cleanFlag == "output" || cleanFlag == "o") && !strings.Contains(arg, "=") {
				if i+1 < len(subArgs) {
					i++
					flagArgs = append(flagArgs, subArgs[i])
				}
			}
		} else {
			posArgs = append(posArgs, arg)
		}
	}
	reordered := append(flagArgs, posArgs...)

	if err := fs.Parse(reordered); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(errOut, "Attachment ID is required")
		return 2
	}
	attachmentID := fs.Arg(0)
	if !attachmentIDPattern.MatchString(attachmentID) {
		fmt.Fprintln(errOut, "Invalid attachment ID; must be alphanumeric characters, hyphens or underscores")
		return 2
	}

	c, code := LoadCapability(errOut)
	if code != 0 {
		return code
	}

	artifactsDir := c.ArtifactsDir
	if artifactsDir == "" {
		artifactsDir = os.Getenv("CUAN_TASK_ARTIFACTS_DIR")
	}
	if artifactsDir == "" {
		fmt.Fprintln(errOut, "Attempt artifact directory is unavailable")
		return 1
	}
	artifactsDir = filepath.Clean(artifactsDir)
	canonicalArtifactsDir, err := filepath.EvalSymlinks(artifactsDir)
	if err != nil {
		canonicalArtifactsDir = artifactsDir
	} else {
		canonicalArtifactsDir = filepath.Clean(canonicalArtifactsDir)
	}

	var cleanTarget string
	if action == "download" && *outputFlag != "" {
		var code int
		cleanTarget, code = validateArtifactDestination(artifactsDir, canonicalArtifactsDir, *outputFlag, errOut)
		if code != 0 {
			return code
		}
	}

	endpoint := strings.TrimRight(c.API, "/") + "/artifacts/" + url.PathEscape(attachmentID)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		fmt.Fprintln(errOut, "Invalid task API request")
		return 1
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)

	client := &http.Client{
		Timeout:       60 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintln(errOut, "Task API unavailable; could not download artifact")
		return 1
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusPartialContent || resp.Header.Get("Content-Range") != "" {
		fmt.Fprintln(errOut, "Partial response received; artifact transfer rejected")
		return 1
	}
	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		fmt.Fprintf(errOut, "Artifact download failed (%d): %s\n", resp.StatusCode, bytes.TrimSpace(errBody))
		return 1
	}

	const maxLimit = 25 << 20 // 25 MB

	if action == "read" || action == "cat" || *stdoutFlag {
		data, err := io.ReadAll(io.LimitReader(resp.Body, maxLimit+1))
		if err != nil {
			fmt.Fprintln(errOut, "Failed to read artifact content")
			return 1
		}
		if len(data) > maxLimit {
			fmt.Fprintln(errOut, "Artifact exceeds maximum size limit of 25 MB")
			return 1
		}
		if resp.ContentLength > 0 && int64(len(data)) != resp.ContentLength {
			fmt.Fprintln(errOut, "Incomplete artifact read; partial content received")
			return 1
		}

		if rawOutput || *rawFlag || *stdoutFlag || action == "cat" {
			_, _ = out.Write(data)
			return 0
		}

		result := map[string]any{
			"attachment_id": attachmentID,
			"content":       string(data),
			"media_type":    resp.Header.Get("Content-Type"),
			"size_bytes":    len(data),
		}
		encoded, _ := json.Marshal(result)
		_, _ = out.Write(encoded)
		fmt.Fprintln(out)
		return 0
	}

	// Download mode
	if cleanTarget == "" {
		filename := ""
		if cd := resp.Header.Get("Content-Disposition"); cd != "" {
			if _, params, err := mime.ParseMediaType(cd); err == nil {
				filename = params["filename"]
			}
		}
		filename = filepath.Base(filename)
		if filename == "" || filename == "." || filename == "/" || filename == ".." {
			filename = attachmentID
		}
		var code int
		cleanTarget, code = validateArtifactDestination(artifactsDir, canonicalArtifactsDir, filename, errOut)
		if code != 0 {
			return code
		}
	}

	tmpFile, err := os.CreateTemp(artifactsDir, ".artifact-dl-*")
	if err != nil {
		fmt.Fprintln(errOut, "Could not create temporary file in attempt artifact directory")
		return 1
	}
	tmpName := tmpFile.Name()
	defer os.Remove(tmpName)
	defer tmpFile.Close()

	copied, err := io.Copy(tmpFile, io.LimitReader(resp.Body, maxLimit+1))
	if err != nil {
		fmt.Fprintln(errOut, "Failed to write artifact data")
		return 1
	}
	if copied > maxLimit {
		fmt.Fprintln(errOut, "Artifact exceeds maximum size limit of 25 MB")
		return 1
	}
	if resp.ContentLength > 0 && copied != resp.ContentLength {
		fmt.Fprintln(errOut, "Incomplete artifact download; partial content received")
		return 1
	}
	if err = tmpFile.Sync(); err != nil {
		fmt.Fprintln(errOut, "Failed to flush artifact data to disk")
		return 1
	}
	if err = tmpFile.Close(); err != nil {
		fmt.Fprintln(errOut, "Failed to close temporary file")
		return 1
	}

	if parentDir := filepath.Dir(cleanTarget); parentDir != artifactsDir && parentDir != canonicalArtifactsDir {
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			fmt.Fprintln(errOut, "Failed to create target directory")
			return 1
		}
	}

	if err = os.Link(tmpName, cleanTarget); err != nil {
		if errors.Is(err, os.ErrExist) {
			fmt.Fprintf(errOut, "Output file %q already exists; overwriting existing files is not permitted\n", cleanTarget)
			return 1
		}
		if err = os.Rename(tmpName, cleanTarget); err != nil {
			fmt.Fprintln(errOut, "Failed to move artifact to destination path")
			return 1
		}
	}
	_ = os.Remove(tmpName)

	realTarget, err := filepath.EvalSymlinks(cleanTarget)
	if err != nil {
		_ = os.Remove(cleanTarget)
		fmt.Fprintln(errOut, "Path escape detected: unable to verify real file location")
		return 2
	}
	relTarget, err := filepath.Rel(canonicalArtifactsDir, filepath.Clean(realTarget))
	if err != nil || strings.HasPrefix(relTarget, "..") {
		_ = os.Remove(cleanTarget)
		fmt.Fprintln(errOut, "Path escape detected: output file wrote outside attempt artifact directory")
		return 2
	}

	if *rawFlag {
		fmt.Fprintln(out, cleanTarget)
		return 0
	}

	result := map[string]any{
		"saved_path":    cleanTarget,
		"attachment_id": attachmentID,
		"size_bytes":    copied,
		"media_type":    resp.Header.Get("Content-Type"),
	}
	encoded, _ := json.Marshal(result)
	_, _ = out.Write(encoded)
	fmt.Fprintln(out)
	return 0
}

func validateArtifactDestination(artifactsDir, canonicalArtifactsDir, target string, errOut io.Writer) (string, int) {
	if !filepath.IsAbs(target) {
		target = filepath.Join(artifactsDir, target)
	}
	cleanTarget := filepath.Clean(target)

	rel, err := filepath.Rel(artifactsDir, cleanTarget)
	if err != nil || strings.HasPrefix(rel, "..") || rel == "." {
		fmt.Fprintln(errOut, "Path escape detected: output file must reside inside the attempt artifact directory")
		return "", 2
	}
	if relCanon, errCanon := filepath.Rel(canonicalArtifactsDir, cleanTarget); errCanon == nil {
		if strings.HasPrefix(relCanon, "..") || relCanon == "." {
			fmt.Fprintln(errOut, "Path escape detected: output file must reside inside the attempt artifact directory")
			return "", 2
		}
	}

	fi, lstatErr := os.Lstat(cleanTarget)
	if lstatErr == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			if realDest, err := filepath.EvalSymlinks(cleanTarget); err == nil {
				realDest = filepath.Clean(realDest)
				if relDest, errDest := filepath.Rel(canonicalArtifactsDir, realDest); errDest != nil || strings.HasPrefix(relDest, "..") {
					fmt.Fprintln(errOut, "Path escape detected: output destination symlink resolves outside attempt artifact directory")
					return "", 2
				}
			}
		}
		fmt.Fprintf(errOut, "Output file %q already exists; overwriting existing files is not permitted\n", cleanTarget)
		return "", 1
	} else if !errors.Is(lstatErr, os.ErrNotExist) {
		fmt.Fprintf(errOut, "Output file %q check failed: %v\n", cleanTarget, lstatErr)
		return "", 1
	}

	curr := filepath.Dir(cleanTarget)
	for {
		_, err := os.Lstat(curr)
		if err == nil {
			realCurr, err := filepath.EvalSymlinks(curr)
			if err != nil {
				fmt.Fprintln(errOut, "Path escape detected: unable to resolve parent directory symlink")
				return "", 2
			}
			realCurr = filepath.Clean(realCurr)
			relCurr, errRel := filepath.Rel(canonicalArtifactsDir, realCurr)
			if errRel != nil || strings.HasPrefix(relCurr, "..") {
				fmt.Fprintln(errOut, "Path escape detected: output file parent directory resolves outside attempt artifact directory")
				return "", 2
			}
			break
		}
		parent := filepath.Dir(curr)
		if parent == curr {
			break
		}
		curr = parent
	}

	return cleanTarget, 0
}
