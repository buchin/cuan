# Cuan — Product Vision & Positioning

## 1. Brand Identity & Kategori

* **Nama Produk:** **CUAN**
* **Domain Resmi:** **`https://cuan.co`**
* **Kategori Industri (Category King):** **Agentic Project Management for Creators**
* **Brand Statement:**
  > **Cuan: Agentic Project Management for Creators**
* **Positioning untuk Pengguna:** Platform *Agentic Project Management* yang dirancang khusus untuk para kreator — menghubungkan repositori, blueprint proyek, pipeline produksi, dan eksekusi tugas langsung oleh AI Agent dalam satu studio terpadu tanpa kerumitan teknis. Di Cuan, tugas pada Kanban bukan sekadar catatan to-do list mati, melainkan langsung dieksekusi oleh AI Agent untuk menghasilkan draft konten, skrip, dan copywriting siap pakai.
* **Headline (English):**
  > **Agentic Project Management for Creators.**
* **Headline (Bahasa Indonesia):**
  > **Manajemen Proyek Agentik untuk Kreator.**
* **Sub-deskripsi (Hero Copywriting):**
  > *"Plan your creator projects, assign work to AI agents on your server, and turn backlog tasks into finished content—without the technical headaches."*
* **Dasar Copywriting:** Mengatasi kelemahan to-do list tradisional yang pasif. Cuan menghadirkan blueprint alur kerja kreator dan agen AI mandiri yang langsung menyelesaikan tugas di dalam workspace proyek.
* **Istilah Dashboard:** Gunakan **New Project** untuk membuat proyek kreator baru (Blog, YouTube, Olshop, Affiliate). Setiap proyek dilengkapi room diskusi studio dan Kanban task yang dapat dijalankan langsung dengan AI Agent (*In-Task Agent Runner*).

---

## 2. Target Market & Karakteristik Pengguna

* **Target Utama:** **Creators & Digital Solopreneurs**:
  1. **Blogger & Web Publisher:** Membutuhkan blueprint riset kata kunci/SEO, perancangan topik pilar, dan drafting artikel otomatis oleh agent tanpa merancang prompt dari nol.
  2. **YouTube & Video Creators:** Membutuhkan pipeline produksi video terstruktur (*Ideasi → Hook/Title → Full Scriptwriting → Description & Tagging*) dengan agent scriptwriter siap pakai.
  3. **Online Shop & Digital Product Creators:** Membutuhkan funnel peluncuran produk digital (*Sales Page Copywriting → 3-Part Email Nurture Sequence → FAQ & Objection Handling*).
  4. **Affiliate Marketers:** Membutuhkan workflow komparasi produk, ulasan fitur mendalam, dan drafting review konversi tinggi secara terstruktur.
* **Beachhead Market (Validasi Awal):** Komunitas creator & marketer di ekosistem Dojo (pengguna aktif yang mengelola portofolio website AdSense, channel YouTube, dan produk digital).
* **Skala Target Jangka Panjang:** Global (English-first interface & docs, siap untuk jutaan solopreneur kreator konten di Substack, YouTube, Gumroad, dan WordPress).

---

## 3. Masalah Utama yang Diselesaikan (The Core Pain Point)

1. **Passive To-Do Lists (To-Do List Mati):** Aplikasi manajemen tugas konvensional (Trello, Asana, Notion) hanya mencatat daftar pekerjaan tanpa kemampuan mengeksekusi isinya. Kreator tetap kehabisan waktu dan overwhelmed mengerjakan semuanya sendiri.
2. **Tools Terpecah & Context Switching:** Berpindah-pindah antara spreadsheet catatan ide, chatbox AI eksternal, Google Docs, dan CMS membuat fokus kreator terdisrupsi dan banyak draf tercecer.
3. **Portofolio Tercecer (Overwhelmed):** Solopreneur mengelola banyak project sekaligus (beberapa blog, channel YouTube, produk digital) tanpa pipeline otomasi terpadu yang memantau eksekusi dari ide hingga deliverable.

---

## 4. Nilai Inti Produk: "Project-Based with Clear Progress"

Cuan memperlakukan setiap website/aplikasi yang dikelola sebagai **1 Project** mandiri pada antarmuka pengguna, konsisten dengan entitas teknis `project`. Project tersebut merupakan aset bisnis milik pengguna:
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
* **Sprint 2–10 (Lapisan Studio, Blueprints & In-Task Agent Execution):**
  Membangun lapisan antarmuka *Digital Project Studio*, blueprint proyek kreator (Blog, YouTube, Olshop, Affiliate), dan in-task agent execution via Gemini Key Vault.
* **Sprint 11 (Modular Workspace OS, AI Script Onboarding & Creator Publishing Hub):**
  Mengembangkan Cuan menjadi Modular Workspace OS dengan dynamic module navigation (`enabled_modules`), AI Script Onboarding (ZIP upload dropzone, stack detection, setup pipeline auto-generation), serta Creator Publishing Hub (WordPress REST API 1-click publishing & Teleprompter).

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




