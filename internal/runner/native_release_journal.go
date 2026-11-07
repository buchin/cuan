package runner

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/buchin/cuan/internal/release"
)

var errNativeReleaseActive = errors.New("native release process is already owned by another worker")

type nativeProcessRecord struct {
	Version int             `json:"version"`
	Binding string          `json:"binding"`
	Result  *release.Result `json:"result,omitempty"`
}

// A durable start marker forbids replay after an unknown external outcome. A
// finished process result may be re-observed and delivered without rerunning it.
func (r *Runner) executeNativeReleaseOnce(ctx context.Context, claim ClaimTaskResponse) (release.Result, error) {
	unknown := release.Result{ExitCode: -1}
	if claim.Attempt == nil || claim.Task == nil || claim.Attempt.TaskID != claim.Task.ID || claim.NativeRelease == nil || claim.NativeReleaseExecution == nil || !completionID.MatchString(claim.Attempt.ID) {
		return unknown, errors.New("native execution identity unavailable")
	}
	root, err := r.openCompletionOutbox()
	if err != nil {
		return unknown, err
	}
	defer root.Close()
	lock, err := lockCompletion(root, "native-"+claim.Attempt.ID)
	if err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return unknown, errNativeReleaseActive
		}
		return unknown, err
	}
	defer lock.Close()
	if err := root.Mkdir("native-process", 0700); err != nil && !os.IsExist(err) {
		return unknown, err
	}
	info, err := root.Lstat("native-process")
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
		return unknown, errors.New("native process journal must be private")
	}
	// Only a digest of selected environment is retained, never credential values.
	binding := nativeExecutionBinding(claim, r.cfg.RunnerID)
	name := filepath.Join("native-process", claim.Attempt.ID+".json")
	f, readErr := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if readErr == nil {
		defer f.Close()
		info, err := f.Stat()
		if err != nil || !info.Mode().IsRegular() || info.Size() > 16384 {
			return unknown, errors.New("invalid native process journal")
		}
		var record nativeProcessRecord
		decoder := json.NewDecoder(io.LimitReader(f, 16385))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&record) != nil || decoder.Decode(&struct{}{}) != io.EOF || record.Version != 1 || record.Binding != binding {
			return unknown, errors.New("native process journal differs from preparation")
		}
		if record.Result == nil {
			return unknown, errors.New("native process outcome unknown; no execution replay permitted")
		}
		return *record.Result, nil
	} else if !os.IsNotExist(readErr) {
		return unknown, readErr
	}
	record := nativeProcessRecord{Version: 1, Binding: binding}
	if err := writeNativeProcessRecord(root, name, record, true); err != nil {
		return unknown, err
	}
	execution := claim.NativeReleaseExecution
	result, executeErr := executePreparedNativeRelease(ctx, claim.NativeRelease, execution.Workspace, execution.Environment)
	if executeErr != nil {
		result.Error = "Native release preparation could not be verified; no script execution confirmed"
	}
	record.Result = &result
	if err := writeNativeProcessRecord(root, name, record, false); err != nil {
		return unknown, err
	}
	return result, nil
}

func writeNativeProcessRecord(root *os.Root, name string, record nativeProcessRecord, exclusive bool) error {
	raw, err := json.Marshal(record)
	if err != nil || len(raw) > 16384 {
		return errors.New("native process record exceeds bound")
	}
	destination := name
	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL | syscall.O_NOFOLLOW
	if !exclusive {
		destination = "temporary/native-process-" + rand.Text() + ".tmp"
		defer root.Remove(destination)
	}
	f, err := root.OpenFile(destination, flags, 0600)
	if err != nil {
		return err
	}
	if _, err = f.Write(raw); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if !exclusive {
		if err := root.Rename(destination, name); err != nil {
			return err
		}
	}
	directory, err := root.Open("native-process")
	if err != nil {
		return err
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return err
	}
	parent, err := root.Open(".")
	if err != nil {
		return err
	}
	defer parent.Close()
	return parent.Sync()
}

func nativeExecutionBinding(claim ClaimTaskResponse, runnerID string) string {
	raw, _ := json.Marshal(struct {
		TaskID, RunnerID string
		Intent           any
		Execution        *NativeReleaseExecution
	}{claim.Task.ID, runnerID, claim.NativeRelease, claim.NativeReleaseExecution})
	return completionHash(raw)
}
