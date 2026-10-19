package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/buchin/cuan/internal/agent"
)

// boundedInboxOutput prevents noisy subprocesses exhausting server memory.
// Live activity contains only application-owned labels, never stderr/reasoning.
type boundedInboxOutput struct {
	mu       sync.Mutex
	data     []byte
	first    func()
	once     sync.Once
	overflow bool
}

func (b *boundedInboxOutput) Write(p []byte) (int, error) {
	b.mu.Lock()
	remaining := 1024*1024 - len(b.data)
	if len(p) > remaining {
		b.overflow = true
	}
	if remaining > 0 {
		n := len(p)
		if n > remaining {
			n = remaining
		}
		b.data = append(b.data, p[:n]...)
	}
	b.mu.Unlock()
	if len(p) > 0 && b.first != nil {
		b.once.Do(b.first)
	}
	return len(p), nil
}
func (b *boundedInboxOutput) String() string { b.mu.Lock(); defer b.mu.Unlock(); return string(b.data) }

func (s *Server) executeInboxCLI(ctx context.Context, name, harness, prompt, home, workspace string, step func(string, string)) (string, string) {
	if s.cfg != nil && s.cfg.Environment == "testing" {
		return "", "Provider tidak tersedia. Pesan tersimpan; periksa instalasi CLI dan koneksi agen ini."
	}
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	system := fmt.Sprintf("Anda adalah %s, rekan kerja di Cuan. Jawab sesuai kebutuhan pesan: sapaan singkat, pertanyaan substantif dengan jawaban yang berguna, atau minta klarifikasi bila ambigu. Gunakan bahasa pengguna. Jangan memaksa semua pesan menjadi sapaan. Jangan mengklaim aksi, perubahan file, task, atau deployment yang belum dilakukan. Anda memiliki konteks project dan boleh menggunakan tools baca untuk analisis repo, backlog, dan rekomendasi sprint tanpa membuat task. Gunakan hanya workspace project yang diberikan; jangan membaca credential, profil login atau project lain. Jangan mengubah file, commit, menjalankan tes/build, atau deployment saat analisis chat. Untuk implementasi yang perlu dilacak, tawarkan task. Jika snapshot menyebut repo belum terhubung atau project ambigu, jelaskan kondisi tersebut dan minta project yang benar; jangan mengaku tidak punya tools secara umum. Jangan tampilkan reasoning privat, token, credential atau diagnostik runtime.\n\n%s", name, prompt)
	binary := "agy"
	var args []string
	switch harness {
	case "claude":
		binary = "claude"
		args = []string{"-p", "--permission-mode", "plan", "--allowedTools", "Read,Glob,Grep", "--tools", "Read,Glob,Grep", "--output-format", "text", system}
	case "codex":
		binary = "codex"
		args = []string{"exec", "--ephemeral", "--sandbox", "read-only", "--skip-git-repo-check", system}
	case "", "agy", "antigravity":
		args = []string{"-p", system, "--print-timeout", "170s", "--mode", "plan", "--sandbox", "--output-format", "stream-json"}
	default:
		return "", "Jenis CLI agen belum didukung untuk percakapan. Gunakan agen agy, Claude, atau Codex yang terdaftar."
	}
	path, err := exec.LookPath(binary)
	if err != nil && binary == "agy" {
		candidate := "/home/dev/.local/bin/agy"
		if _, e := os.Stat(candidate); e == nil {
			path = candidate
			err = nil
		}
	}
	if err != nil {
		return "", "Provider tidak tersedia. Pesan tersimpan; periksa instalasi CLI dan koneksi agen ini."
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir = home
	if workspace != "" {
		cmd.Dir = workspace
	}
	cmd.Env = agent.AntigravityProfileEnvironment(home)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if err == syscall.ESRCH {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = 2 * time.Second
	stdout := &boundedInboxOutput{}
	stderr := &boundedInboxOutput{}
	if step != nil {
		stdout.first = func() { step("output", "Menerima hasil proses provider.") }
	}
	var runtime *inboxRuntimeStream
	if binary == "agy" {
		runtime = &inboxRuntimeStream{step: step}
		cmd.Stdout = runtime
	} else {
		cmd.Stdout = stdout
	}
	cmd.Stderr = stderr
	started := time.Now()
	if err := cmd.Start(); err != nil {
		if step != nil {
			step("provider_result", "Proses provider tidak dapat dimulai.")
		}
		return "", safeInboxProviderFailure(ctx.Err(), "")
	}
	if step != nil {
		step("provider", "Proses agen berjalan; menunggu respons provider.")
	}
	runErr := cmd.Wait()
	if step != nil {
		step("provider_result", fmt.Sprintf("Proses provider selesai: exit %d, durasi %.1f dtk.", cmd.ProcessState.ExitCode(), time.Since(started).Seconds()))
	}
	if runtime != nil {
		raw, failure, reason := runtime.finish()

		if failure == "" {
			var reply string
			reply, failure, reason = decodeInboxAgyResult(raw, stderr.String())
			if failure == "" && runErr != nil {
				failure = safeInboxProviderFailure(ctx.Err(), stderr.String())
				reason = "runtime_process_failure"
			}
			if failure == "" {
				if step != nil {
					step("result_validation", "Balasan provider tervalidasi.")
				}
				return reply, ""
			}
			if reason == "empty_response" && runtime.toolFailures > 0 {
				failure = "Agen tidak menghasilkan balasan setelah aktivitas tool gagal. Pesan tersimpan; buka detail respons. Permintaan tidak dijalankan ulang otomatis."
			}
		}
		lowerDiagnostic := strings.ToLower(stderr.String())
		if reason == "runtime_missing_result" && (strings.Contains(lowerDiagnostic, "quota") || strings.Contains(lowerDiagnostic, "429") || strings.Contains(lowerDiagnostic, "resource_exhausted") || strings.Contains(lowerDiagnostic, "unauthorized") || strings.Contains(lowerDiagnostic, "401") || strings.Contains(lowerDiagnostic, "invalid_grant")) {
			failure = safeInboxProviderFailure(nil, stderr.String())
			reason = "provider_failure"
		}
		if ctx.Err() != nil {
			failure = safeInboxProviderFailure(ctx.Err(), "")
			reason = "runtime_interrupted"
		}
		labels := map[string]string{"runtime_missing_result": "Runtime berhenti tanpa hasil akhir.", "runtime_invalid": "Event runtime tidak dapat divalidasi.", "runtime_overflow": "Batas ukuran aktivitas runtime tercapai.", "runtime_process_failure": "Proses runtime gagal.", "runtime_interrupted": "Proses terputus atau melewati batas waktu.", "provider_failure": "Provider melaporkan kegagalan.", "provider_timeout": "Provider melewati batas waktu.", "empty_response": "Provider tidak menghasilkan teks balasan.", "invalid_result": "Hasil akhir runtime tidak valid.", "empty_output": "Runtime tidak mengembalikan hasil akhir."}
		if step != nil {
			step("result_validation", labels[reason])
		}
		log.Printf("inbox runtime result rejected reason=%s bytes=%d events=%d tool_failures=%d", reason, runtime.bytes, runtime.events, runtime.toolFailures)
		return "", failure
	}
	if runErr != nil {
		return "", safeInboxProviderFailure(ctx.Err(), stdout.String()+"\n"+stderr.String())
	}

	if stdout.overflow {
		return "", "Balasan melewati batas ukuran. Pesan tersimpan; pecah permintaan menjadi bagian lebih kecil."
	}
	output := strings.TrimSpace(stdout.String())

	if output == "" {
		return "", safeInboxProviderFailure(nil, stderr.String())
	}

	return output, ""
}

// Only the final response of a successful CLI result is user-visible. Never
// publish JSON envelopes, stderr, reasoning, authentication material or partial
// responses from failed turns. Exit zero alone does not establish success.
func decodeInboxAgyResult(raw, stderr string) (string, string, string) {
	var result struct {
		Status   string          `json:"status"`
		Response string          `json:"response"`
		Error    json.RawMessage `json:"error"`
		Message  string          `json:"message"`
	}
	if strings.TrimSpace(raw) == "" {
		return "", safeInboxProviderFailure(nil, stderr), "empty_output"
	}
	if json.Unmarshal([]byte(raw), &result) != nil {
		return "", "Hasil agen belum dapat dibaca. Pesan tersimpan; coba periksa koneksi agen. Pesan tidak dikirim ulang otomatis.", "invalid_result"
	}
	if strings.ToUpper(strings.TrimSpace(result.Status)) != "SUCCESS" {
		diagnostic := result.Status + "\n" + string(result.Error) + "\n" + result.Message + "\n" + stderr
		if strings.Contains(strings.ToLower(result.Status), "timeout") {
			return "", safeInboxProviderFailure(context.DeadlineExceeded, ""), "provider_timeout"
		}
		return "", safeInboxProviderFailure(nil, diagnostic), "provider_failure"
	}
	reply := strings.TrimSpace(result.Response)
	if reply == "" {
		if strings.TrimSpace(stderr) != "" {
			return "", safeInboxProviderFailure(nil, stderr), "empty_response"
		}
		return "", "Agen belum menghasilkan balasan. Pesan tersimpan; Anda dapat mencoba lagi dengan pesan baru. Pesan ini tidak dikirim ulang otomatis.", "empty_response"
	}
	return reply, "", "success"
}
