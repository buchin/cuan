# Cuan OSS Extensibility & Automation: MCP & Skills Integration Architecture

## 1. Filosofi untuk Pengguna Non-Developer

Cuan dibangun untuk pengguna awam (pemilik bisnis, kreator, dan tim non-teknis) yang ingin pekerjaan mereka diselesaikan secara otomatis tanpa harus berurusan dengan perkakas developer seperti terminal, Docker, kode JSON mentah, atau cron syntax.

Ketika Cuan dirilis sebagai Open Source Software (OSS) self-hosted:
1. **Otomatisasi & Update Berjalan Mandiri**: Cuan memiliki scheduler in-process dan mekanisme auto-update malam hari (*nightly auto-update*) tanpa butuh intervensi manual pengembang.
2. **Ekstensibilitas Berstandar Terbuka (MCP & Skills)**: Alih-alih tim inti harus membangun integrasi tertutup ke ribuan SaaS, Cuan mengadopsi standar **Model Context Protocol (MCP)** dan **Modular Skills**.

---

## 2. Arsitektur Model Context Protocol (MCP) Client

### Mengapa MCP?
MCP adalah protokol standar terbuka dari Anthropic yang menghubungkan agen AI dengan data source, tools eksternal, dan API pihak ketiga. Dengan bertindak sebagai **MCP Client**, Cuan dapat langsung memanfaatkan ribuan MCP server yang sudah ada di komunitas open source.

### Contoh Kasus: Integrasi `tebar.app`
`tebar.app` adalah layanan gateway pengiriman pesan, WhatsApp blast, dan CRM order. Melalui MCP:
- Cuan bertindak sebagai Client.
- `tebar.app` menyediakan MCP server (HTTP/SSE).
- Agen Cuan langsung mengenali kemampuan `tebar.app` tanpa perubahan kode inti (*zero custom backend code*).

```
┌────────────────────────────────┐                 ┌─────────────────────────────┐
│          CUAN AGENT            │                 │     TEBAR.APP MCP SERVER    │
│  (Chat / Tasks / Automations)  │                 │                             │
│               │                │                 │  • tebar_broadcast_whatsapp │
│               ▼                │                 │  • tebar_check_order_status │
│       Cuan MCP Client          │  POST /mcp/call │  • tebar_send_receipt       │
│  (internal/mcp/client.go)      ├────────────────►│                             │
└────────────────────────────────┘                 └─────────────────────────────┘
```

### Pengalaman Antarmuka (UX) Ramah Non-Developer
- Pengguna cukup membuka menu **Settings > Integrations > Tambah Layanan MCP**.
- Masukkan URL endpoint (misal: `https://api.tebar.app`) dan API Key jika diperlukan.
- Cuan secara otomatis melakukan discovery tool (`/mcp/tools`) dan menyajikan daftar kapabilitas yang langsung aktif untuk agen dalam bahasa Indonesia yang mudah dipahami.

---

## 3. Sistem Skills Cuan (Prompt & Domain Knowledge)

### Perbedaan Skills vs Extensions
- **Skills**: Kumpulan instruksi domain dan prompt spesialisasi (misal: *Skill Copywriter Shopee*, *Skill Analisis Laporan Keuangan UKM*, *Skill Customer Service Ramah*).
- **Format Skill**: Berkas Markdown modular (`SKILL.md`) dengan frontmatter YAML sederhana:
  ```yaml
  name: wa-followup-expert
  description: Ahli follow up prospek penjualan WhatsApp dengan nada ramah dan persuasif.
  tools:
    - tebar_broadcast_message
  ```
- **Distribusi Community**: Komunitas OSS dapat membagikan folder skill di GitHub tanpa perlu kompilasi ulang kode Cuan.

---

## 4. Batasan Keamanan & Isolasi (Security Boundaries)

Sesuai dengan [Integration Principles](integration-principles.md):
1. **Enkripsi Kredensial**: Seluruh API Key dan token MCP server disimpan terenkripsi di database menggunakan KeyVault Cuan (`AES-GCM`).
2. **Tidak Menggunakan Ambient Credentials**: Kredensial developer lokal tidak pernah diwariskan ke sesi pengguna Cuan.
3. **Timeout & Failure Circuit Breaker**: Pemanggilan tool MCP eksternal dibatasi dengan context timeout (maksimal 30-60 detik) dan penanganan graceful degradation jika server pihak ketiga offline.
