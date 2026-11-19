package runner

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/buchin/cuan/internal/task"
)

// The registered project recipe is selected by convention, never task text.
// Its exact bytes are bound by the server before any model starts.
func VisualRecipeAt(directory, commit string) (*task.VisualRecipe, error) {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	for _, name := range []string{"scripts", task.VisualVerificationScript} {
		info, err := root.Lstat(name)
		if os.IsNotExist(err) {
			return nil, nil
		}
		if err != nil || info.Mode()&os.ModeSymlink != 0 || (name == "scripts" && !info.IsDir()) || (name == task.VisualVerificationScript && (!info.Mode().IsRegular() || info.Size() > 64<<10 || info.Mode()&0111 == 0)) {
			return nil, errors.New("visual verification recipe path is not private")
		}
	}
	file, err := root.OpenFile(task.VisualVerificationScript, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 64<<10 || info.Mode()&0111 == 0 {
		return nil, errors.New("visual verification recipe must be a small executable regular file")
	}
	data, err := io.ReadAll(io.LimitReader(file, 64<<10+1))
	if err != nil || len(data) > 64<<10 {
		return nil, errors.New("visual verification recipe exceeds its limit")
	}
	hash := sha256.Sum256(data)
	recipe := &task.VisualRecipe{Script: task.VisualVerificationScript, ScriptHash: hex.EncodeToString(hash[:]), SourceCommit: commit}
	if !recipe.Valid() {
		return nil, errors.New("visual verification recipe identity is invalid")
	}
	return recipe, nil
}

func runNativeVisualVerification(ctx context.Context, root, artifacts, baseline string, recipe *task.VisualRecipe, environment map[string]string) (*task.VisualVerification, error) {
	if !recipe.Valid() {
		return nil, errors.New("invalid bound visual recipe")
	}
	actual, err := VisualRecipeAt(root, recipe.SourceCommit)
	if err != nil || actual == nil || *actual != *recipe {
		return nil, errors.New("bound visual verification recipe changed")
	}
	if artifacts == "" {
		return nil, errors.New("native visual verification requires private session artifacts")
	}
	destination := filepath.Join(artifacts, "native-visual")
	if err := os.Mkdir(destination, 0700); err != nil {
		return nil, errors.New("native visual output already exists or is unavailable; saved files preserved")
	}
	privateHome, err := os.MkdirTemp("", "cuan-visual-home-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(privateHome)
	env := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + privateHome, "XDG_CONFIG_HOME=" + filepath.Join(privateHome, "config"), "XDG_CACHE_HOME=" + filepath.Join(privateHome, "cache"), "CI=1", "NO_COLOR=1"}
	for _, key := range []string{"CUAN_BROWSER_MODULE", "CUAN_CHROMIUM_EXECUTABLE", "CUAN_BROWSER_NO_SANDBOX", "PLAYWRIGHT_BROWSERS_PATH"} {
		if value := environment[key]; value != "" {
			env = append(env, key+"="+value)
		}
	}
	captureCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	checked := runProjectVerifierCommandEnvironment(captureCtx, root, env, filepath.Join(root, recipe.Script), destination)
	info, statErr := os.Lstat(destination)
	if statErr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("native visual output directory changed")
	}
	output, err := os.OpenRoot(destination)
	if err != nil {
		return nil, err
	}
	defer output.Close()
	logFile, logErr := output.OpenFile("check-output.log", os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if logErr != nil {
		return nil, errors.New("native visual diagnostic could not be retained")
	}
	_, logErr = logFile.WriteString(checked.Summary)
	closeErr := logFile.Close()
	if logErr != nil || closeErr != nil {
		return nil, errors.New("native visual diagnostic could not be retained")
	}
	if !checked.Passed {
		return nil, fmt.Errorf("native visual verification incomplete: %s; output retained in session:native-visual/check-output.log", checked.Error)
	}

	file, err := output.OpenFile("visual-evidence.json", os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, errors.New("native visual manifest is missing")
	}
	defer file.Close()
	info, err = file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 64<<10 {
		return nil, errors.New("native visual manifest is invalid")
	}
	var manifest struct {
		Components []string `json:"components"`
		Captures   []string `json:"captures"`
	}
	decoder := json.NewDecoder(io.LimitReader(file, 64<<10+1))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&manifest) != nil || decoder.Decode(new(any)) != io.EOF || len(manifest.Captures) == 0 || len(manifest.Captures) > 32 {
		return nil, errors.New("native visual manifest could not be verified")
	}
	receipt := &task.VisualVerification{Recipe: *recipe, Baseline: baseline, Components: manifest.Components}
	total := 0
	for _, path := range manifest.Captures {
		capture := task.VisualCapture{Path: "native-visual/" + path, SHA256: strings.Repeat("0", 64), Width: 1, Height: 1}
		receipt.Captures = append(receipt.Captures, capture)
		if !receipt.Valid() {
			return nil, errors.New("native visual manifest paths or components are invalid")
		}
		data, err := readNativeCapture(output, path)
		if err != nil {
			return nil, err
		}
		total += len(data)
		if total > 25<<20 {
			return nil, errors.New("native visual output exceeds delivery budget")
		}
		image, err := png.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return nil, errors.New("native visual output must be a PNG screenshot")
		}
		hash := sha256.Sum256(data)
		receipt.Captures[len(receipt.Captures)-1] = task.VisualCapture{Path: capture.Path, SHA256: hex.EncodeToString(hash[:]), Width: image.Width, Height: image.Height}
	}
	if !receipt.Valid() {
		return nil, errors.New("native visual screenshot metadata is invalid")
	}
	for _, component := range receipt.Components {
		repository, err := os.OpenRoot(root)
		if err != nil {
			return nil, err
		}
		info, err := repository.Stat(component)
		repository.Close()
		if err != nil || !info.Mode().IsRegular() {
			return nil, errors.New("native visual coverage names an unavailable component")
		}
	}
	return receipt, nil
}
func readNativeCapture(root *os.Root, path string) ([]byte, error) {
	info, err := root.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("native visual output is not a regular file")
	}
	f, err := root.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 10<<20 {
		return nil, errors.New("native screenshot exceeds its limit")
	}
	data, err := io.ReadAll(io.LimitReader(f, 10<<20+1))
	if err != nil || len(data) > 10<<20 {
		return nil, errors.New("native screenshot is unreadable")
	}
	return data, nil
}
func unchangedNativeVisualArtifacts(artifacts string, receipt *task.VisualVerification) bool {
	if !receipt.Valid() {
		return false
	}
	root, err := os.OpenRoot(artifacts)
	if err != nil {
		return false
	}
	defer root.Close()
	for _, capture := range receipt.Captures {
		data, err := readNativeCapture(root, capture.Path)
		if err != nil {
			return false
		}
		hash := sha256.Sum256(data)
		if hex.EncodeToString(hash[:]) != capture.SHA256 {
			return false
		}
	}
	return true
}

func (r *Runner) publishNativeVisualArtifacts(ctx context.Context, claim ClaimTaskResponse, receipt *task.VisualVerification) error {
	if !receipt.Valid() || claim.TaskAgent == nil {
		return errors.New("native capture is unavailable")
	}
	root, err := os.OpenRoot(claim.TaskAgent.ArtifactsDir)
	if err != nil {
		return err
	}
	defer root.Close()
	request := VisualVerificationRequest{RunnerID: r.cfg.RunnerID, Verification: receipt}
	for _, capture := range receipt.Captures {
		data, err := readNativeCapture(root, capture.Path)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(data)
		if hex.EncodeToString(hash[:]) != capture.SHA256 {
			return errors.New("native capture changed before delivery")
		}
		request.Files = append(request.Files, data)
	}
	delivery, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	response, err := r.doRequest(delivery, "POST", "/api/v1/runner/attempts/"+claim.Attempt.ID+"/visual-verification", request)
	uncertain := err != nil
	if err == nil {
		var ack struct {
			Recorded bool `json:"recorded"`
		}
		decodeErr := json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&ack)
		response.Body.Close()
		if response.StatusCode == 200 && decodeErr == nil && ack.Recorded {
			return nil
		}
		uncertain = response.StatusCode >= 500 || response.StatusCode == 200
	}
	// Inspect the same durable receipt after an ambiguous response. Never
	// blindly resend uploads or claim an unrecorded delivery succeeded.
	if uncertain && r.nativeVisualDeliveryRecorded(ctx, claim, receipt) {
		return nil
	}
	return errors.New("native visual delivery was not confirmed; saved files preserved")
}

func (r *Runner) nativeVisualDeliveryRecorded(ctx context.Context, claim ClaimTaskResponse, receipt *task.VisualVerification) bool {
	check, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	query := url.Values{"task_id": {claim.Task.ID}, "runner_id": {r.cfg.RunnerID}}
	response, err := r.doRequest(check, "GET", "/api/v1/runner/attempts/"+claim.Attempt.ID+"/report?"+query.Encode(), nil)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return false
	}
	var observed struct {
		TaskID    string                   `json:"task_id"`
		AttemptID string                   `json:"attempt_id"`
		Receipt   *task.VisualVerification `json:"native_visual_verification"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(&observed) != nil || observed.TaskID != claim.Task.ID || observed.AttemptID != claim.Attempt.ID || !observed.Receipt.Valid() {
		return false
	}
	saved := observed.Receipt
	actual, _ := json.Marshal(saved)
	expected, _ := json.Marshal(receipt)
	return saved != nil && bytes.Equal(actual, expected)
}
