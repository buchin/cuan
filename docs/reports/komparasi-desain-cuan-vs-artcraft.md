# Analisis Komparasi Desain: Cuan (Calm Software) vs ArtCraft (Craft Anything)

**Tanggal:** 2 November 2026  
**Penyusun:** Scrum Master & Development Team (AgentSession)  
**Subjek:** Evaluasi Keselarasan Desain UI/UX Cuan saat ini (Things 3 & Bear Notes) vs Pendekatan ArtCraft (`getartcraft.com/apps/craft-anything`)  
**Target Persona:** Solopreneurs, UMKM/small business owners, creator businesses, dan non-technical entrepreneurs  
**Dokumen Terkait:** `docs/design/design_guidelines.md`, `AGENTS.md`  

---

## 1. Ringkasan Eksekutif (*Executive Summary*)

Menanggapi pertanyaan evaluasi Product Owner: *"Apakah Cuan lebih cocok dengan desain sekarang atau memakai desain seperti getartcraft.com/apps/craft-anything?"*, tim telah melakukan audit visual, analisis persona pengguna, dan evaluasi kepatuhan terhadap **Design Guidelines & Quality Gate (Zero Cognitive Fatigue)**.

### Kesimpulan Utama:
> **DESAIN SAAT INI (CALM SOFTWARE / THINGS 3 & BEAR NOTES) JAUH LEBIH COCOK DAN WAJIB DIPERTAHANKAN UNTUK UI APLIKASI CUAN.**
>
> 1. **Kesesuaian Target Pasar (Target Audience Fit):** Cuan dirancang spesifik untuk **non-developer dan solopreneur**. Mereka membutuhkan perangkat lunak yang memberikan ketenangan operasional (*peace of mind*), bukan tampilan teknis yang membingungkan.
> 2. **Kepatuhan Terhadap Definition of Done (DoD):** Desain aplikasi Craft Anything mengadopsi pola *developer-dense workbench* yang secara langsung melanggar standar Cuan: *Card Diet* (kotak di dalam kotak), tombol pil putih solid (`#ffffff`), baris tugas berjejal tombol aksi, dan ekspos log teknis pada kanvas utama.
> 3. **Peluang Adopsi Terpilih (*Cross-Pollination*):** Elemen ArtCraft yang sangat baik untuk diadopsi terbatas pada:
>    - **Halaman Pemasaran/Landing Page:** Tipografi editorial kontras tinggi, showcase tangkapan layar dramatis, dan pemilih arsitektur download yang rapi.
>    - **Pola Otorisasi Agen (*Human-in-the-Loop*):** Dialog konfirmasi 3 opsi granular (*Allow once / Always for this project / Don't allow*), namun wajib dibalut dalam styling *Calm Inset Group* tanpa tombol pil putih mencolok.

---

## 2. Diagram Arsitektur Keputusan Desain (D2 + TALA)

```d2
direction: right

cuan: "Cuan (Calm Software)" {
  target_user: "Target: Non-Developer & Solopreneur"
  philosophy: "Things 3 & Bear Notes (Zero Fatigue)"
  layout_system: "Flat Canvas, HairlineList, SlideDrawer"
}

artcraft: "ArtCraft (Craft Anything)" {
  target_user: "Target: Developer & Reverse Engineer"
  philosophy: "Dense Workbench & Multi-Pane IDE"
  layout_system: "Nested Cards, Hex/Disassembly, Solid White Pills"
}

fit: "Kesesuaian Penggunaan" {
  app_ui: "UI Aplikasi Harian -> Cuan Calm Design"
  landing: "Landing Page & Marketing -> Inspirasi ArtCraft"
  permission: "Human-in-the-Loop -> Pola Granular ArtCraft"
}

cuan -> fit.app_ui: "Sesuai DoD & Target Pasar"
artcraft -> fit.landing: "Adopsi Tipografi & Showcase"
artcraft -> fit.permission: "Adaptasi ke Styling Calm"
```

---

## 3. Komparasi Mendalam: Cuan vs ArtCraft (Craft Anything)

| Dimensi Evaluasi | Cuan (Desain Saat Ini) | ArtCraft (Craft Anything) | Penilaian untuk Cuan |
| :--- | :--- | :--- | :--- |
| **Filosofi Inti** | **Calm Software** (Things 3 & Bear Notes). Menghilangkan kecemasan visual pengguna. | **Software Reverse Engineering & Hacker IDE**. Menampilkan kontrol inspeksi teknis mendalam. | **Cuan Unggul.** Solopreneur butuh kejelasan bisnis, bukan menonton eksekusi kode. |
| **Target Pengguna** | Non-developer, pebisnis mandiri, agensi kecil, pemilik toko online/kreator. | Developer, security researcher, AI agent power-users, prompt hackers. | **Cuan Unggul.** Mengubah Cuan menjadi IDE akan memicu *churn* masif bagi non-developer. |
| **Arsitektur Tata Letak (Layout)** | **Kanvas Datar (Card Diet).** Baris daftar murni (`HairlineList`) dengan pemisah rambut halus (`border-border/40`). | **Nested Box Cards.** Kotak bertingkat di dalam kotak dengan stroke border tebal dan drop shadow. | **Cuan Unggul.** ArtCraft melanggar Aturan 2.1 (Larangan Tegas Nested Cards). |
| **Information Layering** | **2 Lapisan Tegas:** Glanceable (permukaan 2 detik) dan Deep Layer (on-demand via `SlideDrawer`). | **Flat Exposure:** Terminal command, check progress, skor kemiripan, dan open question dijejalkan bersamaan. | **Cuan Unggul.** Mencegah *cognitive overload* saat memantau tugas. |
| **Palet & Visual Noise** | **Monokromatik Teredam:** Gelap hangat (`#141416`), aksen minim, 0 pil putih solid, micro-indicator `StatusDot`. | **High-Contrast Dark Mode:** Pil putih solid `#ffffff`, border kuning/oranye kontras tinggi, badge persentase. | **Cuan Unggul.** ArtCraft melanggar Quality Gate Cuan (Zero Visual Noise). |
| **Landing Page / Web** | Minimalis fungsional, fokus pada login dan onboarding cepat. | Sangat memikat, tipografi berani (Serif + Mono), showcase screenshot interaktif yang kuat. | **ArtCraft Unggul.** Dapat diadopsi untuk halaman pemasaran publik Cuan. |

---

## 4. Evaluasi terhadap Quality Gate DoD Cuan (docs/design/design_guidelines.md §7)

### 4.1. Eliminasi Visual Noise (Zero Visual Noise)
- **DoD Cuan:** Dilarang menggunakan pil putih solid `#ffffff`, dilarang border kawat 1px menyala, aksen warna hanya untuk *Status-by-Exception*.
- **Kondisi ArtCraft:** Tombol aksi utama "Allow once" adalah pil putih solid `#ffffff` yang menyilaukan mata di dark mode. Kotak izin menggunakan border kuning/oranye tebal yang terus berteriak di layar pengguna.

### 4.2. Whitespace & Ruang Bernapas (Breathing Room)
- **DoD Cuan:** Kolom baca desktop dibatasi `max-w-[75ch]` (~680–760px). Whitespace bersih memberikan ruang bernapas mata.
- **Kondisi ArtCraft:** Tata letak 3-kolom multi-pane penuh dari tepi ke tepi layar, dengan panel kiri (proyek), panel tengah (kartu-kartu aksi bertumpuk), dan panel kanan (stepper progres, progress bar, open questions). Sangat sesak bagi pengguna awam.

### 4.3. Hierarki Tipografi Alami (Effortless Readability)
- **DoD Cuan:** Kejelasan hierarki melalui bobot font Geist Sans/Mono dan teks netral teredam tanpa perlu dibungkus bingkai kotak.
- **Kondisi ArtCraft:** Sangat bergantung pada kotak-kotak pembungkus (*box containers*) untuk memisahkan setiap jenis informasi.

### 4.4. Usability & Keutuhan Konteks Alur Kerja (Predictable Flow)
- **DoD Cuan:** Pengguna berinteraksi melalui percakapan alami atau daftar tugas bersih. Informasi teknis dibuka melalui laci samping (`SlideDrawer`) tanpa menginterupsi layar baca utama.
- **Kondisi ArtCraft:** Aliran pesan diselingi oleh interupsi kartu konfirmasi perintah bash mentah (`curl ...`), inspeksi pseudo-C, dan hex memory address.

---

## 5. Komponen ArtCraft yang Layak Diadopsi (Cross-Pollination)

Meskipun UI aplikasi ArtCraft tidak cocok untuk Cuan, ada dua inovasi brilian pada ArtCraft yang dapat diadaptasi ke Cuan:

### 1. Pola Otorisasi Agen Granular (Human-in-the-Loop)
Pada tangkapan layar ArtCraft, ketika agen memerlukan izin untuk aksi berisiko, ia menampilkan kartu dialog terstruktur:
- *Opsi 1: Allow once* (Izinkan sekali ini saja)
- *Opsi 2: Always for this project* (Izinkan selalu untuk workspace ini)
- *Opsi 3: Don't allow* (Tolak aksi)

**Adaptasi Cuan:** Cuan dapat mengadopsi struktur perizinan 3 opsi ini saat agen hendak melakukan aksi eksternal berbayar atau sensitif (misal: kirim SMS/WA massal, charge kartu kredit, ubah DNS domain). Namun penampilannya wajib menggunakan *Calm Inset Group* (`GroupedSection`) atau *Hairline Notification* yang tenang tanpa pil putih mencolok.

### 2. Estetika Halaman Pemasaran (Marketing / Landing Page)
Website `getartcraft.com/apps/craft-anything` memiliki daya pikat visual yang sangat tinggi untuk calon pengguna baru:
- Judul editorial besar dengan perpaduan font serif dan monospace yang modern.
- Tangkapan layar aplikasi yang di-framing rapi dan tajam.
- Bagian unduhan / onboarding yang memisahkan pilihan platform dengan sangat intuitif.
- Ini dapat dijadikan inspirasi bagi pembuatan landing page publik Cuan (`cuan.id`).

---

## 6. Poin Keputusan untuk Product Owner (PO Decision Point)

Sesuai aturan Agentic Scrum (Prinsip: Dampak bisnis terbesar, effort terkecil):

* **Rekomendasi:**
  Pertahankan desain Calm Software (Things 3 & Bear Notes) untuk antarmuka operasional Cuan. Batasi adopsi elemen ArtCraft hanya pada **(a)** gaya presentasi editorial untuk landing page/marketing, dan **(b)** pola perizinan 3 level (*Allow once / Always / Deny*) dengan styling tenang di `SlideDrawer`/`GroupedSection`.
  * **Dampak Bisnis:** Retensi tinggi pengguna solopreneur non-teknis, nol kebingungan visual (*Zero Cognitive Fatigue*), konsistensi dengan DoD yang sudah dibangun di Sprint 62–64.
  * **Effort:** Sangat kecil (0 jam perombakan UI inti).
  * **Urgensi:** Tinggi (memastikan tim tidak keluar jalur dari visi produk).

* **Alternatif:**
  Merombak UI Cuan mengikuti gaya developer workbench ArtCraft (3 kolom padat dengan nested cards).
  * **Dampak Bisnis:** Negatif/merusak (*churn* tinggi dari target pasar UMKM/solopreneur karena tampilan terlalu rumit mirip VS Code/Ghidra).
  * **Effort:** Sangat besar (merombak ulang ratusan jam komponen `AppShell`, `InboxView`, `HairlineList`, dan `SlideDrawer`).
  * **Urgensi:** Tidak direkomendasikan.

* **Alasan:**
  Visi utama Cuan adalah *“Cuan is for nondeveloper users”*. Keunggulan kompetitif Cuan terletak pada kemudahan dan ketenangan yang ditawarkannya. Memasukkan visual noise dan layout developer workbench akan menghancurkan diferensiasi utama Cuan di pasar.
