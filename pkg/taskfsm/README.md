# `taskfsm` — Pure Go State Machine Engine

`taskfsm` adalah *micro-package* mandiri (zero external dependencies) untuk mengelola siklus hidup status tugas dan sprint (*task & sprint lifecycle*) di Cuan, sesuai dengan aturan Agentic Scrum (§1–§13).

## Fitur Utama

- **Zero External Dependencies**: Dibangun murni dengan Go standard library (`sync`, `time`, `encoding/json`).
- **Prinsip Tanggung Jawab Tunggal (Single Responsibility Principle)**: Murni berfokus pada transisi status dan validasi *guardrail* tanpa tercampur dengan logika database atau transport layer.
- **Strict Guardrails**: Mencegah transisi ilegal (misalnya melompati persetujuan PO langsung ke status `running`).
- **Audit Logging**: Mencatat riwayat lengkap setiap transisi status (`from`, `to`, `event`, `reason`, `actor`, `timestamp`).
- **Thread-Safe**: Dilindungi `sync.RWMutex` untuk akses bersamaan tanpa race condition.
- **Serialisasi JSON**: Mendukung `MarshalJSON` dan `UnmarshalJSON` secara native untuk persistensi status.

## Siklus Status (Lifecycle States)

Sesuai urutan Agentic Scrum:
`planning → approved → running → review → completed`
Dengan status interupsi & terminasi: `blocked`, `canceled`.

```
[planning]
   │ (approve)
   ▼
[approved]
   │ (start)
   ▼
[running] <─────────────────┐
   │ ├── (block) ──> [blocked]
   │ │                  │ (resume)
   │ └──────────────────┘
   ▼ (submit_review)
[review] ─── (return_to_doing) ──> [running]
   │ (complete)
   ▼
[completed] (terminal)
```

## Cara Penggunaan

```go
package main

import (
    "fmt"
    "github.com/buchin/cuan/pkg/taskfsm"
)

func main() {
    // Inisialisasi FSM di state planning
    sm := taskfsm.New(taskfsm.StatePlanning)

    // Periksa apakah event diizinkan
    if sm.Can(taskfsm.EventApprove) {
        _ = sm.Transition(taskfsm.EventApprove, 
            taskfsm.WithReason("Scope disetujui PO"),
            taskfsm.WithActor("PO"),
        )
    }

    fmt.Printf("Current state: %s\n", sm.Current()) // approved
}
```
