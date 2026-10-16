# Cuan — Product Vision & Positioning

## 1. Brand Identity & Kategori

* **Nama Produk:** **CUAN**
* **Domain Resmi:** **`https://cuan.co`**
* **Kategori Industri (Category King):** **Digital Project Studio**
* **Brand Statement:**
  > **Cuan: Digital Project Studio for Entrepreneurs**
* **Positioning untuk Pengguna:** Mengelola **aset digital (digital assets)** — website, WordPress, aplikasi, dan web tools — dalam satu tempat. Istilah **Digital Project Studio** tetap menjadi kategori produk; istilah project pada fitur merujuk pada unit pengelolaan aset.
* **Headline (English):**
  > **Manage your digital assets under one roof.**
* **Headline (Bahasa Indonesia):**
  > **Kelola aset digital ente di bawah satu atap.**
* **Sub-deskripsi (Hero Copywriting):**
  > *"Deploy scripts you’ve purchased, manage your websites, and make changes with AI—without the technical headaches."*

---

## 2. Target Market & Karakteristik Pengguna

* **Target Utama:** **Non-Tech Entrepreneurs & Digital Teams** (Solo founders, internet marketers, digital creators, affiliate teams, dan pemilik bisnis online).
* **Beachhead Market (Validasi Awal):** Komunitas member ekosistem Dojo (pengguna nyata yang membeli software/script digital dan membutuhkan zero-friction deployment).
* **Skala Target Jangka Panjang:** Global (English-first interface & docs, siap untuk jutaan pembeli script di CodeCanyon, Gumroad, GitHub, Envato, dan SaaS boilerplates).

---

## 3. Masalah Utama yang Diselesaikan (The Core Pain Point)

1. **Purchased Code Sitting Idle:** Pengguna membeli script/software/boilerplate ($30–$300), tetapi file `.zip` hanya mengendap di laptop karena tidak paham instalasi server, terminal SSH, Docker, dan database.
2. **Ketergantungan pada Freelancer Mahal:** Membayar $50–$100/jam hanya untuk instalasi script sederhana atau mengubah satu fitur kecil (ganti nomor WhatsApp, ubah tombol, pasang payment gateway).
3. **Portofolio Tercecer (Overwhelmed):** Solopreneur mengelola banyak project sekaligus (Web AdSense, Web Adsterra/Arbitrage, APK WebView, tools affiliate, dsb.) tanpa dashboard terpadu yang memantau status hidup/matinya aset mereka.

---

## 4. Nilai Inti Produk: "Project-Based with Clear Progress"

Cuan memperlakukan setiap aset digital sebagai **1 Digital Project** mandiri:
* **Universal Input (GitHub BUKAN Keharusan):**
  - Pengguna non-tech cukup **Upload ZIP** (file script/theme dari CodeCanyon/Dojo/Envato) atau pilih **1-Click Blueprint**.
  - GitHub bersifat **opsional**; pengguna dapat menghubungkan repo yang sudah dimiliki atau memilih menyimpan salinan kode ke akun GitHub pribadi melalui onboarding yang dipandu.
  - Di belakang layar, Cuan tetap membuat repository Git lokal secara otomatis untuk tracking versi dan rollback aman tanpa pengguna perlu tahu apa itu Git.
* **First-Class WordPress & Script Support:**
  - Mendukung proyek **WordPress 1-Click Deploy** (container WordPress + MySQL/MariaDB + Caddy auto-SSL).
  - Upload theme/plugin via ZIP secara instan tanpa perlu akses cPanel/FTP.
  - Asisten AI bertindak sebagai *AI Webmaster* untuk inject tracking script, edit `functions.php`, tweak konfigurasi, dan optimasi performa.
* **Smart Stack Detection:** Deteksi otomatis teknologi (WordPress/PHP, Laravel, Node.js, Next.js, Python, Dockerfile).
* **Visual Config Wizard:** Mengubah kerumitan file `.env` dan konfigurasi database menjadi formulir visual yang ramah manusia.
* **1-Click Turnkey Cloud:** Auto-provisioning VPS/Cloud dengan Docker, persistent database, dan reverse proxy HTTPS (Caddy) otomatis.
* **Agentic Co-Pilot (Chat-to-Modify):** Modifikasi kode dan tambah fitur cukup dengan prompt bahasa manusia biasa, diuji pada staging preview sebelum di-publish ke live.

### Prinsip Kepemilikan & Portabilitas Aset

Prinsip produk: **pengguna dapat meninggalkan Cuan dengan membawa kode, data, dan panduan untuk menjalankan asetnya di luar Cuan.** Ketentuan berikut adalah arah desain untuk planning sprint berikutnya; bukan klaim fitur yang sudah tersedia dan tidak menambah scope sprint yang sedang running.

* **Kepemilikan pengguna:** Kode, data, dan konfigurasi aset dapat diekspor dalam format standar, dengan tetap mengikuti lisensi software pihak ketiga. Ekspor dilengkapi panduan deployment, kebutuhan lingkungan, serta prosedur pemulihan agar dapat digunakan tanpa AgentSession internal.
* **Git otomatis, GitHub opsional:** Setiap project software yang kodenya dikelola Cuan memiliki repo Git lokal otomatis untuk riwayat perubahan. Koneksi GitHub ditawarkan sebagai pilihan saat onboarding tanpa menghalangi pengguna memulai. Repo eksternal berada di akun pengguna; Cuan hanya diberi akses ke repo yang dipilih. GitHub membantu kepemilikan salinan dan portabilitas, tetapi bukan layanan terdesentralisasi.
* **Pengembangan software:** Pengguna dapat memodifikasi software dan menambah fitur melalui AI. Perubahan kode dilacak di Git, diuji di staging preview, dan dipublikasikan setelah persetujuan pengguna. Pemulihan kode tidak otomatis memulihkan database atau file unggahan.
* **WordPress sebagai satu aset:** Satu instalasi WordPress dikelola sebagai satu aset. Kode kustom, seperti child theme dan plugin khusus, dilacak di Git. Artikel, halaman, pengaturan, dan akun berada dalam backup database; media dan file unggahan berada dalam backup uploads. Pemulihan website menggabungkan versi kode, database, dan uploads yang konsisten, beserta versi dependensi yang diperlukan.
* **Backup milik pengguna:** Sediakan ekspor dan pilihan tujuan backup di penyimpanan milik pengguna, misalnya S3/R2. GitHub menjadi salinan kode dan tidak menggantikan backup database atau uploads. Detail integrasi dan verifikasi restore ditentukan saat sprint planning.
* **Secrets di luar repo:** Password, token, kredensial, dan konfigurasi rahasia disimpan melalui mekanisme secrets, tidak di-commit ke Git atau GitHub. Panduan migrasi menjelaskan cara mengonfigurasi ulang secrets secara aman.
* **UX nonteknis:** Pengguna mengakses Website/Aplikasi, Riwayat Perubahan, Preview, Backup, Pulihkan, dan Ekspor melalui antarmuka yang jelas. Pemahaman Git tidak menjadi syarat penggunaan.

---

## 5. Hubungan dengan Fondasi Sprint 1 (S1) & Roadmap Masa Depan

* **Sprint 1 (Fondasi Mesin / Core Engine):**
  Membangun arsitektur Go backend, runner agent headless yang cancelable, persistent queue/lease, integrasi Git/GitHub, Docker compose, dan Task Board. Sprint 1 adalah fondasi teknis yang wajib ada agar engine Cuan stabil dan aman.
* **Sprint 2+ (Lapisan Studio & Non-Tech UX):**
  Membangun lapisan antarmuka *Digital Project Studio*:
  - Fitur New Project: Upload ZIP pihak ketiga & template blueprint (Web Tool, Content/AdSense, dsb.).
  - Visual environment editor (pengganti edit `.env` manual).
  - Multi-project portfolio overview & status health monitor.
  - Conversational customizer interface.

---

## 6. Model Bisnis: Commercial Open Source (COSS)

* **Community Edition (Free Self-Hosted):**
  - Lisensi open-source (Apache-2.0).
  - Siapa pun bebas memasang Cuan di VPS pribadi mereka via Docker Compose tanpa biaya lisensi.
  - Berfungsi sebagai motor pertumbuhan viral, membangun kepercayaan komunitas, dan adopsi luas.
* **Commercial Tier (Cuan Paid Managed Cloud):**
  - Layanan hosted cloud bagi pengguna yang tidak ingin mengelola VPS sendiri ($7–$15/bulan atau paket setara lokal).
  - Menghasilkan recurring revenue (MRR) utama yang stabil dan terprediksi.
* **Community Sponsorship (Transfer Bank / QRIS & GitHub Sponsors):**
  - **Model Logo Berjenjang & Kuota Terbatas di Homepage (Tiered & Dynamic Top 30 Grid):**
    - Homepage dibatasi hanya menampilkan **Top 30 Sponsor Terbesar** agar estetika desain tetap bersih, prestisius, dan tidak penuh sesak (*crowded*).
    - **Sistem Leaderboard Dinamis:** Posisi di Homepage ditentukan berdasarkan kontribusi sponsor terbesar. Di masa depan, jika ada sponsor baru yang memberikan kontribusi lebih besar, posisi sponsor di urutan bawah dapat tergeser dari Homepage.
    - **Jaminan Retensi Abadi:** Sponsor yang tergeser dari Homepage **TIDAK DIHAPUS**, melainkan tetap tercatat permanen selamanya di repositori `README.md` GitHub dan direktori lengkap `/sponsors`.
    - 🥇 *Platinum Tier (Top 1–5):* Logo Paling Besar di baris teratas + 1 baris slogan bisnis + Dofollow link.
    - 🥈 *Gold Tier (Top 6–15):* Logo Ukuran Sedang di baris kedua.
    - 🥉 *Silver Tier (Top 16–30):* Logo Ukuran Kompak di baris ketiga.
    - ☕ *Community Backers & Sponsor Tergeser (Tanpa Batas Kuota):* Nama/Logo tercatat abadi di `README.md` repositori GitHub dan halaman direktori lengkap `/sponsors`.
  - **Strategi Lean GTM (Sprint 2):**
    - Pada fase awal, domain `cuan.co` langsung me-redirect pengunjung ke Landing Page Sponsorship Bahasa Indonesia (`/sponsor`).
    - Menghindari kerja ganda (tidak perlu bikin 2 bahasa di awal), fokus 100% menyerap konversi sponsorship dari email list lokal via Transfer Bank (BCA/Mandiri) dan QRIS.
    - Begitu dana sponsor terkumpul dan Cuan siap rilis publik/OSS global, redirect dimatikan dan Homepage English diaktifkan dengan logo sponsor Indonesia yang sudah berjejer rapi sebagai bukti *social proof*.
  - **Kebijakan Etika & Legalitas Mutlak:** DILARANG KERAS menerima sponsorship atau menautkan link dari industri terlarang (Judi Online/Judol, Pornografi/Dewasa, Pinjaman Online Ilegal, Scam/Phishing, atau aktivitas melanggar hukum lainnya). Seluruh submission sponsor wajib lolos kurasi etika; pelanggar akan ditolak dan dananya dikembalikan secara sepihak.
* **Strategi AI: BYO AI Subscription (Zero Platform AI Burden):**
  - Tidak menjual token/kredit AI di awal guna menghindari beban biaya komputasi, resiko chargeback, dan margin tipis.
  - Memanfaatkan langganan AI yang sudah dimiliki pengguna (ChatGPT Plus, Claude Pro, Gemini, atau API Key pribadi).
  - Nilai jual unik bagi pengguna: *"Gunakan kuota langganan AI yang sudah Anda bayar tiap bulan — tanpa biaya token tambahan di Cuan."*




