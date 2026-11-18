# Cuan

> **AI untuk Kelola Aset Digital dari Ide sampai Cuan.**  
> The self-hosted agentic workspace to run digital projects on your own server with zero token markup.

[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Latest Release](https://img.shields.io/github/v/release/buchin/cuan?color=green)](https://github.com/buchin/cuan/releases)
[![Status: Public Alpha](https://img.shields.io/badge/status-public%20alpha%20%2F%20developer%20preview-orange.svg)](https://github.com/buchin/cuan)
[![Docs](https://img.shields.io/badge/docs-cuan.work-blue)](https://cuan.work/docs)

> [!IMPORTANT]
> ### 🚀 Komitmen Sponsorware Terpenuhi: Rilis Public Alpha (Developer Preview)
> Berkat dukungan luar biasa dari **100 Founding Backers**, repositori Cuan resmi dibuka untuk publik!
>
> ⚠️ **Status Kestabilan: Masih Belum Stabil (Fase Alpha / Early Preview v0.9.x)**  
> Cuan saat ini masih dalam tahap pengembangan aktif dan **belum stabil untuk operasional bisnis utama (*production*)**. Fitur, alur kerja, dan skema database masih dapat mengalami perubahan (*breaking changes*).
>
> 💡 **Tujuan Rilis Ini & Ajakan Kolaborasi Komunitas**:  
> Kode ini dibuka agar Anda dan komunitas perintis dapat ikut mencoba, mengeksplorasi, melaporkan kendala/bug di berbagai setup VPS, dan mematangkan Cuan bersama-sama.  
> - **Gunakan untuk**: Uji coba, eksplorasi fitur, riset otomasi, dan kontribusi komunitas.
> - **Jangan gunakan untuk**: Sistem bisnis kritis yang menuntut kestabilan 100% tanpa backup berkala (`./scripts/backup.sh`).
> - **Menemukan bug atau error?** Bantu laporkan langsung melalui [GitHub Issues](https://github.com/buchin/cuan/issues) agar tim bisa segera mematangkannya.

---

## 🎯 Bukan Sekadar Chatbot. Bukan Tool Rumit Programmer.

Punya segudang ide proyek digital — dari website, toko online, script otomatis, sampai landing page — tapi sering mentok di urusan teknis?

| Yang Biasa Anda Temui | Masalahnya | Solusi Cuan |
| :--- | :--- | :--- |
| **Chatbot Biasa**<br>*(ChatGPT / Claude Web)* | Cuma bisa kasih teks obrolan & saran; tidak bisa login ke server, tidak bisa mengeksekusi file langsung sampai selesai. | **Eksekusi Nyata:** Asisten AI bekerja mandiri di server Anda, menyelesaikan kode, menguji, dan membuktikan hasilnya secara nyata. |
| **Tool Koding Programmer**<br>*(Cursor / Copilot / CLI)* | Sangat rumit, wajib paham terminal, git, dan konfigurasi environment yang bikin pusing non-developer. | **Zero Cognitive Fatigue:** Antarmuka tenang ala Things 3 & Bear Notes. Kelola pekerjaan semudah berkirim pesan, bisa dari laptop maupun HP. |
| **Platform SaaS AI Pihak Ketiga** | Biaya token dimarkup 2x–5x lipat, data tersimpan di server orang lain, dan fitur dibatasi kuota bulanan. | **100% Milik Anda:** Self-hosted di server sendiri. Pakai langganan resmi Anda (Claude Pro, ChatGPT Plus, Antigravity) tanpa markup sepeser pun. |

---

## ⚡ Coba dalam 30 Detik (Quick Start)

Jalankan Cuan di server pribadi (VPS) atau komputer lokal:

### Opsi 1: Docker Compose (Disarankan)

```bash
curl -fsSL https://cuan.work/docker-compose.yml -o docker-compose.yml
docker compose up -d
```
Buka browser ke `http://localhost:8080` untuk masuk ke wizard aktivasi akun Owner pertama.

### Opsi 2: 1 Baris Perintah Linux (VPS + Domain Pribadi)

Arahkan DNS A-Record (misal `cuan.bisnisanda.com`) ke IP VPS Anda, lalu jalankan:

```bash
curl -fsSL https://cuan.work/install.sh | bash -s -- --domain cuan.bisnisanda.com --email anda@bisnisanda.com
```
*(Cuan otomatis mengonfigurasi Caddy reverse proxy & sertifikat HTTPS gratis Let's Encrypt).*

### Opsi 3: Build dari Source (Developer)

```bash
git clone https://github.com/buchin/cuan.git
cd cuan
go run ./cmd/server
```

---

## 📖 Panduan & Dokumentasi Resmi

Panduan lengkap, konfigurasi AI, dan operasional server dipusatkan di **[cuan.work/docs](https://cuan.work/docs)**:

- 🚀 [Panduan Memulai & Instalasi 30 Detik](https://cuan.work/docs/getting-started)
- 🌐 [Menghubungkan Domain Pribadi (cuan.bisnisanda.com) & HTTPS](https://cuan.work/docs/getting-started#domain-setup)
- 🤖 [Menghubungkan AI (Claude Pro, ChatGPT Plus, DeepSeek, OpenAI)](https://cuan.work/docs/ai-providers)
- 📁 [Proyek & Task Otonom (Pendelegasian & Human Approval)](https://cuan.work/docs/projects-tasks)
- 💬 [Customer Inbox & Live Chat Widget Website](https://cuan.work/docs/inbox)
- 👥 [Kolaborasi Tim & Undangan WhatsApp 1-Klik (RBAC)](https://cuan.work/docs/team)
- 🛠️ [Spesifikasi Server VPS, Backup, Restore & Update Versi](https://cuan.work/docs/self-hosting)

---

## 🤝 Founding Backers & Sponsors

<!-- SPONSORS:START -->

### 🌟 Founding Titans (Top Sponsors)

Terima kasih tak terhingga kepada seluruh perintis yang mendanai rilis open-source Cuan v1.0 melalui model *Sponsorware*:

| Peringkat | Brand & Tautan | Slogan / Layanan | Kontribusi | Kategori |
| :---: | :--- | :--- | :---: | :--- |
| 🏆 #1 | **[Jeon.Id](https://jeon.id)** | Satu platform untuk membangun identitas digital, menciptakan konten dengan AI, menjual produk, dan mengembangkan bisnis Anda. | `Rp 2.499.115` | 🌟 Platinum Titan |
| 🏆 #2 | **[Tentaklik](https://tentaklik.com/)** | Sewa akun Whitelist Meta, Google, dan TikTok untuk membantu campaign kamu berjalan lebih lancar. | `Rp 999.844` | 🥇 Gold Titan |
| 🏆 #3 | **[Masterily](https://masterily.com)** | Learn More, Do Better, Grow Faster. | `Rp 999.460` | 🥇 Gold Titan |
| 🏆 #4 | **[Matematika.id](https://www.matematika.id/)** | Les Privat & Kursus Matematika Online Lengkap | `Rp 299.768` | 🥈 Silver Titan |
| 🏆 #5 | **[Printjavasport](https://printjavasport.net/)** | We're your team in difficult times | `Rp 299.757` | 🥈 Silver Titan |
| 🏆 #6 | **[KedaiMu](https://kedaimu.com)** | Belanja lebih dekat. | `Rp 299.556` | 🥈 Silver Titan |
| 🏆 #7 | **[Devid Ekha Hidayat](https://masdev.id/)** | masdev.id | `Rp 299.202` | 🥈 Silver Titan |
| 🏆 #8 | **[DigitalMarket.id](https://digitalmarket.id)** | Marketplace Produk Digital, Ebook & Tools Bisnis Online | `Rp 299.034` | 🥈 Silver Titan |

### 🌐 Mitra Bisnis & Proyek Terverifikasi

| Peringkat | Brand / Bisnis | Slogan | Kontribusi | Tautan Resmi |
| :---: | :--- | :--- | :---: | :--- |
| #9 | **Nurjaya bin Musa** | - | `Rp 99.955` | [legit.co.id](https://legit.co.id) |
| #10 | **Fasitel** | Solusi Teknologi Digital, Produk SaaS & Platform AI | `Rp 99.716` | [fasitel.co.id](https://fasitel.co.id) |
| #11 | **Pasti Hebat Teknologi** | Platform Layanan Edukasi & Teknologi Digital | `Rp 99.668` | [pastihebat.com](https://pastihebat.com/) |
| #12 | **Websites.id** | Jasa Pembuatan Website Profesional | `Rp 99.625` | [websites.id](https://www.websites.id) |
| #13 | **Dian Hadinata** | - | `Rp 99.606` | [web.facebook.com/nata.dinata2](https://web.facebook.com/nata.dinata2/) |
| #14 | **satubeasiswa** | Informasi Beasiswa Terbaru | `Rp 99.520` | [satubeasiswa.com](https://satubeasiswa.com) |
| #15 | **APK Extractor** | Extract, Save & Share Android Apps | `Rp 99.417` | [extractor.termoradev.com](https://extractor.termoradev.com) |
| #16 | **Ronal Egi Ernest** | Medis, Catur, dan Hidup | `Rp 99.395` | [Https://www.Egijournal.com](Https://www.Egijournal.com) |
| #17 | **TataKarya** | Kerja rumit, jadi tertata. | `Rp 99.384` | [tatakarya.id](https://tatakarya.id) |
| #18 | **Penerbit Uwais** | Jasa Penerbitan dan Cetak Buku Murah | `Rp 99.238` | [penerbituwais.id](https://penerbituwais.id/) |
| #19 | **Bani Risset** | Digital and AI Spesialis | `Rp 99.108` | [banirisset.com](https://banirisset.com) |
| #20 | **mbesjisp** | Web Programing | `Rp 99.088` | [github.com/mbesjisp](https://github.com/mbesjisp) |

> 📜 **[Lihat Daftar Lengkap 100 Founding Backers & Komunitas di SPONSORS.md](SPONSORS.md)**

<!-- SPONSORS:END -->

Ingin brand atau nama Anda tercantum permanen di README resmi GitHub dan website cuan.work?  
Dukung pengembangan Cuan di **[cuan.work/sponsor](https://cuan.work/sponsor)**.

---

## 📄 Lisensi

Cuan adalah perangkat lunak open-source berlisensi [Apache License 2.0](LICENSE).  
Hak Cipta © 2026 Buchin dan kontributor Cuan.
