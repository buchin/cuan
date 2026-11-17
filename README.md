# Cuan

> **AI untuk Kelola Aset Digital dari Ide sampai Cuan.**  
> The self-hosted agentic workspace to run digital projects on your own server with zero token markup.

[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Latest Release](https://img.shields.io/github/v/release/buchin/cuan?color=green)](https://github.com/buchin/cuan/releases)
[![Docs](https://img.shields.io/badge/docs-cuan.work-blue)](https://cuan.work/docs)

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

### v1.0 Founding Backers

Terima kasih kepada seluruh founding backer yang mendukung independensi dan keberlanjutan pengembangan Cuan open-source:

| Peringkat | Brand & Tautan | Slogan | Kontribusi | Tier |
| :---: | :--- | :--- | :---: | :--- |
| 🥇 #1 | **[Jeon.Id](https://jeon.id)** | Satu platform untuk membangun identitas digital, menciptakan konten dengan AI, menjual produk, dan mengembangkan bisnis Anda. | `Rp 2.499.115` | 🌟 Platinum Founding Backer |
| 🥈 #2 | **[Matematika.id](https://www.matematika.id/)** | Les Privat & Kursus Matematika Online Lengkap | `Rp 299.768` | 🥇 Gold Founding Backer |
| 🥉 #3 | **[Printjavasport](https://printjavasport.net/)** | We're your team in difficult times | `Rp 299.757` | 🥈 Silver Founding Backer |
| ⭐ #4 | **[KedaiMu](https://kedaimu.com)** | Belanja lebih dekat. | `Rp 299.556` | 🥉 Bronze Founding Backer |
| ⭐ #5 | **[DigitalMarket.id](https://digitalmarket.id)** | Marketplace Produk Digital, Ebook & Tools Bisnis Online | `Rp 299.034` | 🥉 Bronze Founding Backer |
| #6 | **Hendy Mahardhika** | - | `Rp 99.923` | 🥉 Bronze Backer |
| #7 | **tedjo2021** | - | `Rp 99.891` | 🥉 Bronze Backer |
| #8 | **Anjrah Susanto** | - | `Rp 99.824` | 🥉 Bronze Backer |
| #9 | **rizky05** | - | `Rp 99.807` | 🥉 Bronze Backer |
| #10 | **sdrfakri** | - | `Rp 99.725` | 🥉 Bronze Backer |
| #11 | **[Fasitel](https://fasitel.co.id)** | Solusi Teknologi Digital, Produk SaaS & Platform AI | `Rp 99.716` | 🥉 Bronze Backer |
| #12 | **Lia Novita** | - | `Rp 99.716` | 🥉 Bronze Backer |
| #13 | **Zayinul Muharom** | - | `Rp 99.693` | 🥉 Bronze Backer |
| #14 | **arief Widodo** | - | `Rp 99.691` | 🥉 Bronze Backer |
| #15 | **Sam Priatna** | - | `Rp 99.678` | 🥉 Bronze Backer |
| #16 | **[Pasti Hebat Teknologi](https://pastihebat.com/)** | Platform Layanan Edukasi & Teknologi Digital | `Rp 99.668` | 🥉 Bronze Backer |
| #17 | **pwanto98** | - | `Rp 99.634` | 🥉 Bronze Backer |
| #18 | **[Websites.id](https://www.websites.id)** | Jasa Pembuatan Website Profesional | `Rp 99.625` | 🥉 Bronze Backer |
| #19 | **rafki_** | - | `Rp 99.610` | 🥉 Bronze Backer |
| #20 | **** | - | `Rp 99.606` | 🥉 Bronze Backer |
| #21 | **Santo Gunawan** | - | `Rp 99.588` | 🥉 Bronze Backer |
| #22 | **Kornelius Agustinus** | - | `Rp 99.564` | 🥉 Bronze Backer |
| #23 | **fajarvalentine14** | - | `Rp 99.564` | 🥉 Bronze Backer |
| #24 | **Yuli Aji Prabowo** | - | `Rp 99.559` | 🥉 Bronze Backer |
| #25 | **Tri Sunarso** | - | `Rp 99.525` | 🥉 Bronze Backer |
| #26 | **arygaseboai** | - | `Rp 99.523` | 🥉 Bronze Backer |
| #27 | **papajosiasean** | - | `Rp 99.520` | 🥉 Bronze Backer |
| #28 | **Muji burrahman** | - | `Rp 99.502` | 🥉 Bronze Backer |
| #29 | **angga dwi s** | - | `Rp 99.442` | 🥉 Bronze Backer |
| #30 | **[APK Extractor](https://extractor.termoradev.com)** | Extract, Save & Share Android Apps | `Rp 99.417` | 🥉 Bronze Backer |
| #31 | **Muchlisin** | - | `Rp 99.415` | 🥉 Bronze Backer |
| #32 | **Muhammad Ihsan** | - | `Rp 99.411` | 🥉 Bronze Backer |
| #33 | **[TataKarya](https://tatakarya.id)** | Kerja rumit, jadi tertata. | `Rp 99.384` | 🥉 Bronze Backer |
| #34 | **pediadigitalniaga** | - | `Rp 99.368` | 🥉 Bronze Backer |
| #35 | **Jersey Mason** | - | `Rp 99.339` | 🥉 Bronze Backer |
| #36 | **Rachmat Ajis** | - | `Rp 99.310` | 🥉 Bronze Backer |
| #37 | **Abd Azis** | - | `Rp 99.288` | 🥉 Bronze Backer |
| #38 | **Fazrie Said** | - | `Rp 99.286` | 🥉 Bronze Backer |
| #39 | **rachman8** | - | `Rp 99.264` | 🥉 Bronze Backer |
| #40 | **Verta Ferry** | - | `Rp 99.257` | 🥉 Bronze Backer |
| #41 | **serbacuanclass** | - | `Rp 99.242` | 🥉 Bronze Backer |
| #42 | **[Penerbit Uwais](https://penerbituwais.id/)** | Jasa Penerbitan dan Cetak Buku Murah | `Rp 99.238` | 🥉 Bronze Backer |
| #43 | **Imam HA** | - | `Rp 99.223` | 🥉 Bronze Backer |
| #44 | **taurida Adinda** | - | `Rp 99.201` | 🥉 Bronze Backer |
| #45 | **Aulia Ridwan** | - | `Rp 99.173` | 🥉 Bronze Backer |
| #46 | **agus yulian** | - | `Rp 99.136` | 🥉 Bronze Backer |
| #47 | **[mbesjisp](https://github.com/mbesjisp)** | Web Programing | `Rp 99.088` | 🥉 Bronze Backer |
| #48 | **Bamzdepok** | - | `Rp 99.051` | 🥉 Bronze Backer |
| #49 | **budi mulyana mulyana** | - | `Rp 99.041` | 🥉 Bronze Backer |

<!-- SPONSORS:END -->

Ingin brand atau nama Anda tercantum permanen di README resmi GitHub dan website cuan.work?  
Dukung pengembangan Cuan di **[cuan.work/sponsor](https://cuan.work/sponsor)**.

---

## 📄 Lisensi

Cuan adalah perangkat lunak open-source berlisensi [Apache License 2.0](LICENSE).  
Hak Cipta © 2026 Buchin dan kontributor Cuan.
