# Panduan Operasional: Real-Time Stdout/Stderr Streaming Pipe & Interactive Steering di cuan-runner

Dokumen ini merupakan panduan teknis dan operasional resmi (berstandar §11) mengenai implementasi arsitektur **Real-Time Stdout/Stderr Streaming Pipe** dan **Interactive Steering** pada adapter agen AI `internal/agent/agy_adapter.go`, daemon `cuan-runner`, dan Cuan API Server (Sprint 58).

---

## 1. Ikhtisar & Arsitektur Sistem

Sebelum Sprint 58, pengumpulan log proses agen mengandalkan buffer statis dalam memori (`bytes.Buffer`) yang menampung seluruh keluaran teks hingga proses terminasi. Hal tersebut memiliki dua kelemahan utama:
1. **Ketiadaan Visibilitas Seketika (*Blind Execution*):** Selama eksekusi berlangsung (hingga batas timeout puluhan menit), antarmuka obrolan dan daemon tidak dapat menampilkan baris demi baris log yang sedang diproses.
2. **Risiko Kebocoran Memori (*Memory Spikes*):** Proses agen yang menghasilkan keluaran teks masif dapat menyebabkan konsumsi memori tak terbatas.

### Solusi Sprint 58
1. **Streaming Pipe & Bounded Ring-Buffer:** Output `stdout` dan `stderr` dialirkan langsung melalui pipa non-blocking (`os/exec` pipes) dengan pemindai baris terarah (`bufio.Reader`), dan ditampung dalam **Bounded Ring-Buffer** (512 KB) untuk efisiensi memori mutlak.
2. **Live Incremental Log Streaming Endpoint:** Endpoint `GET /api/v1/runner/attempts/{id}/logs?offset=N` menyajikan potongan log baru secara bertahap tanpa membaca ulang seluruh riwayat log, dilengkapi indikator penyelesaian status `is_final: true/false`.
3. **Interactive Agent Steering:** Endpoint `POST /api/v1/runner/attempts/{id}/steer` memungkinkan pengguna memberikan instruksi koreksi atau konfirmasi seketika yang disalurkan langsung ke `stdin` proses agen aktif, dengan fallback ramah (*queued follow-up prompt*) bila proses tidak sedang berjalan.

---

## 2. Diagram Alur Kerja Streaming & Steering (D2 TALA)

Diagram berikut menggambarkan alur transmisi streaming log live dan penyaluran kemudi interaktif:

```d2
direction: right

user: "Pemilik Toko UMKM\n(Chat UI Mobile/Desktop)"
server: "Cuan API Server\n(/api/v1/runner/attempts)"
runner: "cuan-runner Daemon\n(Multi-Lane Worker)"
adapter: "AGY Adapter\n(internal/agent/agy_adapter.go)"
agent_proc: "Antigravity CLI Subprocess\n(agy -p ...)"

user -> server: "1. Monitor tugas aktif"
runner -> adapter: "2. Execute attempt (req.OnLog, SteerChan)"
adapter -> agent_proc: "3. Spawn process & buka pipes (stdin/stdout/stderr)"
agent_proc -> adapter: "4. Emisi live line chunks stdout/stderr"
adapter -> runner: "5. Callback OnLog(stream, line)"
runner -> server: "6. POST /api/v1/runner/attempts/{id}/logs"
server -> user: "7. GET /api/v1/runner/attempts/{id}/logs?offset=N\n(is_final: false)"
user -> server: "8. POST /api/v1/runner/attempts/{id}/steer\n{\"prompt\": \"arahan\", \"author\": \"user\"}"
server -> runner: "9. Forward steering directive (Heartbeat / Direct)"
runner -> adapter: "10. Injeksi prompt ke stdin agen"
adapter -> agent_proc: "11. Tulis string ke stdin pipe"
agent_proc -> adapter: "12. Proses selesai / exit 0"
adapter -> runner: "13. Return ExecutionResult & RingBuffer metrics"
server -> user: "14. Poller menerima is_final: true"
```

---

## 3. Spesifikasi Teknis Komponen

### 3.1 Bounded Ring-Buffer (`internal/agent/ring_buffer.go`)
- **Kapasitas Default:** 512 KB per stream (stdout dan stderr terpisah).
- **Perilaku Buffer:** Menggunakan struktur sirkular *thread-safe* (`sync.Mutex`). Ketika data masuk melebihi kapasitas, byte tertua dibuang sehingga memori tetap terikat (*strictly bounded*).
- **Metrik:** Menyediakan `Len()`, `Capacity()`, `TotalBytesWritten()`, dan `Reset()`.

### 3.2 Real-Time Emission Pipe (`internal/agent/agy_adapter.go`)
- Menggunakan `cmd.StdoutPipe()` dan `cmd.StderrPipe()` bersama `bufio.NewReaderSize(pipe, 32*1024)`.
- Pemotongan baris berbasis `ReadString('\n')` memastikan setiap baris teks langsung diproses seketika (*zero-delay*).
- Setiap baris dibersihkan dari *secret* via `MaskSecrets` sebelum dipancarkan ke callback `req.OnLog(stream, safeLine)`.
- Pipa `stdin` (`cmd.StdinPipe()`) dikelola secara aman dengan mutex dan perlindungan penutupan ganda (*double-close protection*).

### 3.3 Endpoint Streaming Logs (`GET /api/v1/runner/attempts/{id}/logs`)
- **Query Parameter:**
  - `offset`: integer offset ID log terakhir yang telah diterima klien (alias `after_id`).
  - `limit`: jumlah baris maksimum yang diminta (default: 500, max: 1000).
- **Skema Respons JSON:**
  ```json
  {
    "attempt_id": "att-123456",
    "status": "running",
    "is_final": false,
    "offset": 45,
    "total": 12,
    "logs": [
      {
        "id": 44,
        "attempt_id": "att-123456",
        "stream": "stdout",
        "line": "Scanning project files...",
        "logged_at": "2026-10-31T12:00:00Z"
      }
    ]
  }
  ```
- **Sinyal Penyelesaian:** `is_final` bernilai `true` secara otomatis saat status attempt berpindah ke `succeeded`, `failed`, `canceled`, atau `interrupted`.

### 3.4 Endpoint Interactive Steering (`POST /api/v1/runner/attempts/{id}/steer`)
- **Payload Request:**
  ```json
  {
    "prompt": "Gunakan diskon 15% untuk varian merah saja.",
    "author": "owner@toko.com"
  }
  ```
- **Respons Sukses (Proses Berjalan):**
  ```json
  {
    "status": "delivered",
    "fallback": false,
    "message": "Steering instruction injected to active agent process stdin."
  }
  ```
- **Respons Graceful Fallback (Proses Tidak Berjalan):**
  ```json
  {
    "status": "queued",
    "fallback": true,
    "message": "Attempt is not running; prompt recorded as queued follow-up on task."
  }
  ```

---

## 4. Konfigurasi Timeout Adaptif & Grace Period

Seluruh alokasi timeout stabil yang telah teruji tetap dipertahankan:
- **Worker Terminal Attempt 1:** 25 menit.
- **Worker Terminal Attempt 2:** 35 menit.
- **Worker Terminal Attempt 3+:** 45 menit (hard cap).
- **Termination Grace Period:** 30 detik (SIGTERM sebelum eskalasi ke SIGKILL).
