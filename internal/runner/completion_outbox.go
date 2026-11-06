package runner

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/buchin/cuan/internal/task"
)

var completionID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

const completionRecordLimit = 1 << 20

type completionRecord struct {
	Version           int                    `json:"version"`
	AttemptID         string                 `json:"attempt_id"`
	TaskID            string                 `json:"task_id"`
	ServerHash        string                 `json:"server_hash"`
	CreatedAt         time.Time              `json:"created_at"`
	Payload           CompleteAttemptRequest `json:"payload"`
	PayloadHash       string                 `json:"payload_hash"`
	DeliveryAttempts  int                    `json:"delivery_attempts"`
	NextAttemptAt     time.Time              `json:"next_attempt_at"`
	AcknowledgedState string                 `json:"acknowledged_state,omitempty"`
	HTTPStatus        int                    `json:"http_status,omitempty"`
	Ignored           bool                   `json:"ignored,omitempty"`
}

func completionHash(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}
func (r *Runner) completionDirectory() string {
	if r.cfg.WorkspacesDir != "" {
		return filepath.Join(filepath.Dir(filepath.Clean(r.cfg.WorkspacesDir)), "runner-completions", completionHash([]byte(r.cfg.RunnerID))[:16], completionHash([]byte(r.baseURL))[:16])
	}
	return filepath.Join(r.registry.Dir(), "completion-outbox", completionHash([]byte(r.baseURL))[:16])
}
func (r *Runner) openCompletionOutbox() (*os.Root, error) {
	directory := r.completionDirectory()
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("completion outbox must be a private directory")
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	for _, name := range []string{"locks", "acknowledged", "rejected", "temporary"} {
		if err := root.Mkdir(name, 0700); err != nil && !os.IsExist(err) {
			root.Close()
			return nil, err
		}
		info, err := root.Lstat(name)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
			root.Close()
			return nil, errors.New("completion archive must be private")
		}
	}
	return root, nil
}
func writeCompletion(root *os.Root, name string, record completionRecord) error {
	data, err := json.Marshal(record)
	if err != nil || len(data) > completionRecordLimit {
		return errors.New("completion record exceeds limit")
	}
	temporary := "temporary/" + name + "." + rand.Text() + ".tmp"
	f, err := root.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	defer root.Remove(temporary)
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = root.Rename(temporary, name); err != nil {
		return err
	}
	directory, err := root.Open(".")
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
func readCompletion(root *os.Root, name string) (completionRecord, error) {
	var record completionRecord
	f, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return record, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > completionRecordLimit || info.Mode().Perm()&0077 != 0 {
		return record, errors.New("invalid private completion record")
	}
	decoder := json.NewDecoder(io.LimitReader(f, completionRecordLimit+1))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&record) != nil || decoder.Decode(new(any)) != io.EOF {
		return record, errors.New("invalid completion record")
	}
	data, err := json.Marshal(record.Payload)
	if err != nil || record.Version != 1 || !completionID.MatchString(record.AttemptID) || record.TaskID == "" || record.PayloadHash != completionHash(data) || record.CreatedAt.IsZero() {
		return record, errors.New("completion identity or payload changed")
	}
	return record, nil
}
func lockCompletion(root *os.Root, attempt string) (*os.File, error) {
	f, err := root.OpenFile("locks/"+attempt+".lock", os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
	if err != nil {
		return nil, err
	}
	info, statErr := f.Stat()
	if statErr != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		f.Close()
		return nil, errors.New("completion lock is not private")
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

// Save the exact native result before its first network handoff. This never
// queues or executes an agent, changes the task, or accepts the reported result.
func (r *Runner) deliverCompletion(ctx context.Context, attempt, taskID string, payload CompleteAttemptRequest) error {
	if !completionID.MatchString(attempt) || taskID == "" || payload.RunnerID != r.cfg.RunnerID || !terminalCompletionState(payload.Status) {
		return errors.New("invalid completion identity")
	}
	root, err := r.openCompletionOutbox()
	if err != nil {
		return err
	}
	defer root.Close()
	lock, err := lockCompletion(root, attempt)
	if err != nil {
		return err
	}
	defer lock.Close()
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	record := completionRecord{Version: 1, AttemptID: attempt, TaskID: taskID, ServerHash: completionHash([]byte(r.baseURL)), CreatedAt: time.Now().UTC(), Payload: payload, PayloadHash: completionHash(data)}
	name := attempt + ".json"
	for _, archive := range []string{"acknowledged", "rejected"} {
		if old, err := readCompletion(root, archive+"/"+name); err == nil {
			if old.PayloadHash != record.PayloadHash || old.TaskID != taskID || old.ServerHash != record.ServerHash {
				return errors.New("different completion already archived; original result preserved")
			}
			if archive == "acknowledged" {
				return nil
			}
			return errors.New("completion remains rejected; native diagnosis required")
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	if old, readErr := readCompletion(root, name); readErr == nil {
		if old.PayloadHash != record.PayloadHash || old.TaskID != taskID || old.ServerHash != record.ServerHash {
			return errors.New("different completion already retained; existing result preserved")
		}
		record = old
	} else if !os.IsNotExist(readErr) {
		return readErr
	} else if err = writeCompletion(root, name, record); err != nil {
		return err
	}
	return r.submitCompletionRecord(ctx, root, name, record)
}
func (r *Runner) submitCompletionRecord(parent context.Context, root *os.Root, name string, record completionRecord) error {
	if record.ServerHash != completionHash([]byte(r.baseURL)) || record.Payload.RunnerID != r.cfg.RunnerID || name != record.AttemptID+".json" {
		return errors.New("completion belongs to a different native connection")
	}
	for _, archive := range []string{"acknowledged", "rejected"} {
		if old, readErr := readCompletion(root, archive+"/"+name); readErr == nil {
			if old.PayloadHash != record.PayloadHash || old.TaskID != record.TaskID || old.ServerHash != record.ServerHash {
				return errors.New("completion archive conflicts with pending result; both preserved")
			}
			return root.Remove(name)
		} else if !os.IsNotExist(readErr) {
			return readErr
		}
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	response, err := r.doRequest(ctx, http.MethodPost, "/api/v1/runner/attempts/"+record.AttemptID+"/complete", record.Payload)
	record.DeliveryAttempts++
	if response != nil {
		defer response.Body.Close()
		record.HTTPStatus = response.StatusCode
	}
	if err == nil && response.StatusCode >= 200 && response.StatusCode < 300 {
		// A lost response can be replayed after the server committed it. An ignored
		// terminal receipt is settled, not proof that this payload was accepted.
		var receipt struct {
			task.TaskAttempt
			Ignored bool              `json:"ignored"`
			Attempt *task.TaskAttempt `json:"attempt"`
		}
		if json.NewDecoder(io.LimitReader(response.Body, 128<<10)).Decode(&receipt) == nil {
			saved := &receipt.TaskAttempt
			if receipt.Ignored {
				saved = receipt.Attempt
			}
			if saved != nil && saved.ID == record.AttemptID && saved.TaskID == record.TaskID && saved.RunnerID == r.cfg.RunnerID && terminalCompletionState(saved.Status) {
				record.AcknowledgedState = saved.Status
				record.Ignored = receipt.Ignored
				if err = writeCompletion(root, name, record); err != nil {
					return err
				}
				return archiveCompletion(root, name, "acknowledged", record.AttemptID)
			}
		}
		err = errors.New("server did not acknowledge the exact execution")
	}
	if response != nil && response.StatusCode >= 400 && response.StatusCode < 500 {
		if writeErr := writeCompletion(root, name, record); writeErr != nil {
			return writeErr
		}
		if renameErr := archiveCompletion(root, name, "rejected", record.AttemptID); renameErr != nil {
			return renameErr
		}
		return fmt.Errorf("completion retained for diagnosis after HTTP %d", response.StatusCode)
	}
	delay := 2 * time.Second
	for i := 1; i < record.DeliveryAttempts && delay < 5*time.Minute; i++ {
		delay *= 2
	}
	if delay > 5*time.Minute {
		delay = 5 * time.Minute
	}
	record.NextAttemptAt = time.Now().UTC().Add(delay)
	if writeErr := writeCompletion(root, name, record); writeErr != nil {
		return writeErr
	}
	return errors.New("completion retained for transport retry")
}

// The supervisor retries only saved native payloads. Per-file OS locks prevent
// overlap with a surviving detached worker or a second supervisor.
func (r *Runner) replayCompletions(ctx context.Context) {
	if !r.completionMu.TryLock() {
		return
	}
	defer r.completionMu.Unlock()
	root, err := r.openCompletionOutbox()
	if err != nil {
		return
	}
	defer root.Close()
	directory, err := root.Open(".")
	if err != nil {
		return
	}
	defer directory.Close()
	entries, err := directory.ReadDir(64)
	if err != nil && err != io.EOF {
		return
	}
	processed := 0
	for _, entry := range entries {
		name := entry.Name()
		attempt := strings.TrimSuffix(name, ".json")
		if !strings.HasSuffix(name, ".json") || !completionID.MatchString(attempt) || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		lock, err := lockCompletion(root, attempt)
		if err != nil {
			continue
		}
		record, err := readCompletion(root, name)
		if err == nil && time.Now().After(record.NextAttemptAt) {
			_ = r.submitCompletionRecord(ctx, root, name, record)
			processed++
		}
		lock.Close()
		if processed >= 8 || ctx.Err() != nil {
			return
		}
	}
}

func terminalCompletionState(status string) bool {
	switch status {
	case task.AttemptStatusSucceeded, task.AttemptStatusFailed, task.AttemptStatusCanceled, task.AttemptStatusInterrupted:
		return true
	}
	return false
}

// Lease renewal is bounded to the result handoff, not an imaginary live agent.
// Terminal/cancelled/superseded attempts are still fenced by the server.
func (r *Runner) pendingCompletionIDs() []string {
	root, err := r.openCompletionOutbox()
	if err != nil {
		return nil
	}
	defer root.Close()
	directory, err := root.Open(".")
	if err != nil {
		return nil
	}
	defer directory.Close()
	entries, err := directory.ReadDir(64)
	if err != nil && err != io.EOF {
		return nil
	}
	var ids []string
	now := time.Now()
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") || !completionID.MatchString(strings.TrimSuffix(entry.Name(), ".json")) {
			continue
		}
		record, err := readCompletion(root, entry.Name())
		if err == nil && record.ServerHash == completionHash([]byte(r.baseURL)) && record.Payload.RunnerID == r.cfg.RunnerID && !record.CreatedAt.After(now.Add(5*time.Second)) && now.Before(record.CreatedAt.Add(2*time.Minute)) {
			ids = append(ids, record.AttemptID)
		}
		if len(ids) >= 16 {
			break
		}
	}
	return ids
}

func archiveCompletion(root *os.Root, name, archive, attempt string) error {
	destination := archive + "/" + attempt + ".json"
	if _, err := root.Lstat(destination); err == nil {
		return errors.New("completion archive already exists; original retained")
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := root.Rename(name, destination); err != nil {
		return err
	}
	for _, dir := range []string{archive, "."} {
		f, err := root.Open(dir)
		if err != nil {
			return err
		}
		err = f.Sync()
		f.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
