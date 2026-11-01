# Panduan Arsitektur Micro-Packages (`pkg/taskfsm` & `pkg/dodeval`)

## Latar Belakang & Filosofi Desain

Dalam ekosistem aplikasi enterprise modern berbasis Go, pemecahan modul kompleks menjadi unit-unit atomik (*micro-packages*) adalah pendekatan arsitektural terbaik.

### 1. Prinsip Tanggung Jawab Tunggal (Single Responsibility Principle)
Logika status mesin siklus sprint/task (`taskfsm`) dan logika evaluasi kriteria kelulusan Definition of Done (`dodeval`) dipisahkan secara tegas:
- `pkg/taskfsm`: Bertanggung jawab penuh atas keabsahan transisi status, guardrail transisi ilegal, dan riwayat audit pergerakan status.
- `pkg/dodeval`: Bertanggung jawab atas pemindaian kriteria markdown, normalisasi teks, dan pembandingan bukti laporan (*evidence gap analysis*).

### 2. Zero Context Overload pada Runner
Dengan memecah logika ini ke dalam direktori mandiri di `pkg/`:
- Agen pengembang / *runner session* tidak perlu memuat seluruh basis kode Cuan yang besar.
- Waktu booting dan kompilasi unit test terarah (*targeted unit tests*) berjalan seketika (< 1 detik).
- Risiko kesalahan efek samping (*side-effects*) pada modul lain ditekan menjadi nol.

### 3. Reusabilitas Maksimal & Zero External Dependencies
Kedua modul dibangun 100% menggunakan pustaka standar Go (`net/http`, `sync`, `time`, `regexp`, `bufio`, `encoding/json`).
Dapat diimpor dan digunakan secara bersih oleh komponen lain (misalnya internal server, CLI worker, atau proyek tooling lain) tanpa bloat dependensi pihak ketiga.

---

## Spesifikasi Modul

### Modul 1: `pkg/taskfsm`
- **State Lifecycle:** `planning`, `approved`, `running`, `review`, `completed`, `blocked`, `canceled`.
- **Events:** `approve`, `start`, `submit_review`, `complete`, `block`, `resume`, `return_to_doing`, `cancel`.
- **Guardrails:** Menolak transisi ilegal seperti langsung beralih ke `running` dari `planning` tanpa persetujuan PO (§3).
- **Hooks & Observers:** Mendukung `AddGuard`, `OnTransition`, dan `OnState`.

### Modul 2: `pkg/dodeval`
- **Parser Checklist:** Membaca teks Markdown dengan checkbox `- [ ]` / `- [x]`, numbered list, serta bullet list.
- **Evaluator Evidence:** Membandingkan korpus bukti laporan terhadap kriteria dengan perhitungan *keyword match threshold*.
- **Autonomous Decision:** Menghasilkan keputusan otomatis (`mark_done` bila 100% lulus, atau `auto_requeue` jika masih terdapat kesenjangan bukti).
