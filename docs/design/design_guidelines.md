# Cuan Calm Design System — Single Source of Truth

**Topic:** Calm Software Philosophy (Things 3 & Bear Notes Inspired) for Agentic Project Management
**Target Audience:** Solopreneurs, small teams, creator businesses, and non-technical entrepreneurs
**Repository:** `cuan` (`RPcd484fc4229ae192d17a0f01`)
**Status:** Living Design System Guideline
**Version:** 1.2.0 (Updated in Sprint 61: Things 3 & Bear Notes Total Overhaul — Apple Mail Inset List, Calm Low-Contrast Chat Bubbles & SlideDrawer Task Integration)
**Author:** Buchin <buchin@dropsugar.com>  

---

## 1. Filosofi & Visi: Calm Software

Dalam era kecerdasan buatan (*Agentic Era*), perangkat lunak sering kali terjebak dalam perangkap "kelebihan visual" (*visual clutter*): dashboard dengan puluhan kartu bertumpuk, badge warna-warni yang berkedip, log teknis yang membanjiri layar, dan tombol aksi yang membingungkan.

Bagi solopreneur, pemilik usaha kreatif, dan tim kecil, tujuan utama menggunakan agen AI bukanlah untuk menonton kode berjalan, melainkan untuk **mencapai ketenangan pikiran (*peace of mind*) dan kejelasan operasional (*operational clarity*)**.

Sistem desain Cuan berakar pada prinsip **Calm Software**, yang diilhami oleh dua tolok ukur desain perangkat lunak paling elegan di dunia:
- **Things 3 (Cultured Code):** Keindahan kanvas datar, ritme visual yang lapang, hierarki tanpa kartu tebal (*card diet*), baris tugas murni tanpa tombol aksi berisik, serta interaksi yang mengalir tanpa friksi.
- **Bear Notes (Shiny Frog):** Kesederhanaan tipografis, estetika monokromatik yang hangat, ketajaman teks Geist, gelembung percakapan mengalir tanpa bingkai tebal berlapis, dan fokus mendalam pada konten tanpa ornamen artifisial.

---

## 2. Enam Prinsip Inti (Core Principles)

### 2.1. Card Diet: Larangan Tegas Nested Cards & Banners
- **Aturan Utama:** Dilarang keras menempatkan kartu di dalam kartu (*zero nested cards / no boxes inside boxes*), serta dilarang memasang banner edukasi SaaS yang berisik di dalam ruang kerja operasional.
- **Mekanisme Alternatif:**
  - Gunakan kanvas latar belakang yang bersih dan datar (`bg-background` atau `bg-card/50`).
  - Pemisahan antar-item dilakukan menggunakan garis batas setipis helai rambut (*hairline divider*): `border-b border-border/40` atau `divide-y divide-border/40`.
  - Hindari drop-shadow tebal (`shadow-lg`, `shadow-xl`) pada daftar item berulang. Gunakan transisi hover latar belakang lembut (`hover:bg-muted/30`) untuk menunjukkan keterbukaan interaksi.
  - Untuk formulir pengaturan dan integrasi, gunakan *Apple-style Inset Grouped Rows* (`GroupedSection`).
- **Komponen Penopang:** `HairlineList`, `HairlineListItem`, dan `GroupedSection`.

### 2.2. Things 3 Pure Task Rows (Zero Action Buttons on Rows)
- **Aturan Utama:** Baris daftar tugas di desktop maupun mobile murni menyajikan tiga elemen esensial:
  1. **Kiri:** Indikator status mikro (`StatusDot`) berbasis *Status-by-Exception*.
  2. **Tengah:** Judul tugas bernapas lega dengan tipografi Geist tanpa badge bertumpuk.
  3. **Kanan:** Timestamp relatif abu-abu teredam (`text-muted-foreground` dengan `tabular-nums`).
- **Larangan Keras:** Dilarang menempatkan tombol aksi eksekusi berwarna mencolok (`action-primary`, *View result*, *Retry*, *Live Terminal*) pada baris daftar tugas. Seluruh baris adalah satu kesatuan *touch target* yang membuka laci samping (`SlideDrawer` / `TaskSidePeek`) di mana aksi eksekusi dipusatkan secara teratur.

### 2.3. Information Layering & Progressive Disclosure
- **Aturan Utama:** Pisahkan pengalaman pengguna menjadi dua lapisan yang tegas:
  1. **Glanceable Layer (Permukaan):** Menampilkan informasi esensial dan hasil ringkas yang langsung dapat dipahami dalam 2 detik (judul tugas, waktu relatif, status terkini, hasil utama).
  2. **Deep Layer (Kedalaman On-Demand):** Menyembunyikan seluruh parameter teknis (prompt mentah, log stdout/stderr, token usage, hash commit, branch git, konfigurasi API model) hingga pengguna secara sadar meminta untuk melihatnya.
- **Mekanisme Interaksi:**
  - Klik pada baris atau tombol kelola membuka panel samping kanan yang mulus (`SlideDrawer` / `SideDrawer`) pada desktop, atau *Bottom Sheet* pada mobile.
  - Jangan pernah membanjiri aliran percakapan utama dengan ratusan baris log teknis yang merusak ritme membaca.
- **Komponen Penopang:** `SlideDrawer` (SideDrawer) dan *inline expansion*.

### 2.4. Status-by-Exception (Ketenangan Tanpa Alarm Palsu)
- **Aturan Utama:** Warna kontras tinggi (merah/amber/destructive) dicadangkan **HANYA** untuk status luar biasa (*exceptions*), galat (*errors*), kegagalan sistem, atau tindakan kritis yang mutlak membutuhkan keputusan manusia.
- **Aturan Status Normal:**
  - Status sehat, sukses, atau selesai ditampilkan secara tenang dan bersahaja: gunakan teks netral (`text-muted-foreground`) atau micro-indicator berukuran 6–8px (`StatusDot status="healthy"`).
  - Status menunggu, antre, atau tidak aktif (*idle*) ditampilkan dengan warna abu-abu netral teredam (`bg-muted-foreground/40`).
  - Status agen sedang bekerja menggunakan denyut lembut (*calm pulse*) warna biru/sky (`StatusDot status="running"`), bukan animasi berputar yang agresif.
- **Komponen Penopang:** `StatusDot`.

### 2.5. Desktop Column Constraints (Batas Kolom 65–80ch)
- **Aturan Utama:** Konten membaca, aliran percakapan, dan form instruksi pada layar desktop **WAJIB** dibatasi pada lebar optimal 65 hingga 80 karakter (*typographic measure* ~680px–760px): `max-w-[75ch]` atau `max-w-3xl mx-auto`.
- **Rasional:**
  - Membaca baris teks yang membentang dari ujung kiri ke ujung kanan pada monitor ultrawide menyebabkan mata lelah (*eye fatigue*) dan kehilangan fokus.
  - Ruang kosong di sisi kiri dan kanan (*aesthetic whitespace*) bukanlah pemborosan ruang, melainkan ruang bernapas visual (*visual breathing room*) yang menciptakan ketenangan.

### 2.6. Tipografi Geist yang Tajam & Spacing Tokens Terpadu
- **Aturan Utama:**
  - Font Sans: `@fontsource-variable/geist` (`font-sans`, `'Geist Variable'`). Karakter font netral, modern, dan sangat mudah dibaca pada ukuran kecil (11px–13px) hingga judul besar.
  - Font Mono: `@fontsource-variable/geist-mono` (`font-mono`, `'Geist Mono Variable'`) untuk ID tugas (`#GQATE`), durasi, hash git, dan angka monospaced.
  - Hierarki Spacing: Gunakan standard spacing Tailwind (skala 4px): `gap-1` (4px), `gap-2` (8px), `gap-3` (12px), `gap-4` (16px), `gap-6` (24px).
  - Touch Target Accessibility: Seluruh elemen interaktif memiliki area sentuh minimum 44x44px pada viewport layar sentuh/mobile (`min-h-[44px]`).

### 2.7. Apple Mail & Bear Inset List (Format 3 Tingkat Bebas Bar Redundan)
- **Aturan Utama:** Daftar percakapan di subpanel Inbox murni berformat baris Apple Mail & Bear Inset List 3 tingkat:
  1. **Tingkat 1 (Pengirim & Waktu):** Nama kontak/agen di sisi kiri (`font-semibold/medium text-foreground`) dan timestamp abu-abu teredam di sisi kanan dengan angka tabular (`tabular-nums`).
  2. **Tingkat 2 (Subjek & Indikator Mikro):** Subjek/judul percakapan berbobot Geist bersih, didampingi indikator mikro `StatusDot` jika terdapat tugas latar belakang yang aktif (`data-testid="task-count-badge"`).
  3. **Tingkat 3 (Cuplikan 2 Baris):** Cuplikan ringkasan pesan maksimal 2 baris (`line-clamp-2`) dengan warna teredam (`text-zinc-400`).
- **Larangan Keras:** Dilarang memasang bar selebar layar `[+ New Idea / Task]` di atas daftar kontak yang memicu *call-to-action fatigue*. Pembuatan ide baru dipusatkan di header utama atau Studio Canvas.
- **Pemisah Baris:** Menggunakan garis batas setipis helai rambut (`border-b border-border/40`), bebas dari kotak bertingkat (*nested box cards*).

### 2.8. Calm Low-Contrast Chat Bubbles & Quiet Date Dividers
- **Aturan Utama:**
  - Gelembung percakapan membedakan lawan bicara secara intuitif (kiri untuk incoming/agen, kanan untuk outgoing/owner).
  - Kontras latar belakang menggunakan mikro-tint monokromatik (`#1b1c1f` vs `#25272c`) yang menyatu lembut dengan kanvas dark mode (`#141416`), tanpa garis border stroke putih kaku (`1px solid border`).
  - Pembatas tanggal (`Today`, `Yesterday`) menggunakan teks abu-abu teredam (`calm-day-label text-xs font-medium text-muted-foreground`) tanpa pil putih solid (`#ffffff`) yang menyilaukan mata di dark mode.
  - Kartu status dan kartu tugas di dalam stream pesan mengalir borderless dengan tipografi elegan dan badge mikro.

### 2.9. Progressive Disclosure SlideDrawer untuk Referensi Tugas di Chat
- **Aturan Utama:** Mengklik referensi tugas dari gelembung percakapan (atau kartu aksi tugas) meluncurkan laci samping kanan (`SlideDrawer` / `TaskSidePeek`), bukan mengganti kanvas obrolan (*zero canvas disruption*).
- **Hasil Pengalaman Pengguna:** Aliran percakapan di sebelah kiri tetap utuh, terlihat, dan aktif, sehingga pengguna dapat memeriksa detail teknis tugas tanpa kehilangan konteks diskusi dengan asisten AI.

---

## 3. Arsitektur Pelapisan Informasi & Primitives

Diagram berikut memvisualisasikan bagaimana informasi dipisahkan antara lapisan pandang sekilas (*Glanceable Layer*) dan lapisan data teknis mendalam (*Deep Layer*):

```d2
direction: right

glanceable_layer: "1. Glanceable Layer (Surface)\nKanvas Tenang & Minimalis\nBatas Kolom 65-80ch" {
  hairline_list: "HairlineList & Items\n(Pemisah border-border/40, Zero Nested Cards)"
  status_dot: "StatusDot Indicator\n(Status-by-Exception, Micro-indicators)"
  grouped_section: "GroupedSection / InsetGroup\n(Pengelompokan Form ala Apple/Things)"
}

deep_layer: "2. Deep Layer (On-Demand)\nProgressive Disclosure\nSlideDrawer / SideDrawer" {
  technical_details: "Log Streaming & Raw Data"
  config_panel: "Konfigurasi Lanjutan & Model"
  actions: "Aksi Sekunder / Terminal Shell"
}

glanceable_layer.hairline_list -> deep_layer.technical_details: "Klik Baris / Expand"
glanceable_layer.status_dot -> deep_layer.config_panel: "Inspeksi Status / Error"
glanceable_layer.grouped_section -> deep_layer.actions: "Detail Pengaturan"
```

---

## 4. Calm Layout Primitives Reference

### 4.1. `StatusDot`
Micro-indicator visual 6–8px untuk status tenang tanpa badge berisik.

```tsx
import { StatusDot } from '@/components/ui/StatusDot'

// Status Sehat / Selesai (Emerald Halus)
<StatusDot status="healthy" />

// Status Netral / Idle / Antre (Muted Gray)
<StatusDot status="neutral" />

// Status Sedang Berjalan (Calm Pulse Sky Blue)
<StatusDot status="running" pulse />

// Status Luar Biasa / Error (Rose - Status-by-Exception)
<StatusDot status="exception" title="Koneksi terputus" />

// Dengan Label Teks Pendukung
<StatusDot status="healthy" label="Workspace Aktif" showLabel />
```

### 4.2. `HairlineList` & `HairlineListItem`
Kanvas daftar datar menggantikan kartu-kartu tebal bertumpuk.

```tsx
import { HairlineList, HairlineListItem } from '@/components/layout/HairlineList'
import { StatusDot } from '@/components/ui/StatusDot'

<HairlineList>
  <HairlineListItem
    interactive
    onClick={() => openDetail(project.id)}
    leading={<StatusDot status={project.status} />}
    title={project.name}
    description={project.description}
    trailing={<span className="text-xs text-muted-foreground">Aktif 5m lalu</span>}
  />
</HairlineList>
```

### 4.3. `SlideDrawer` (SideDrawer)
Laci geser samping kanan yang mulus untuk penyingkapan data teknis (*progressive disclosure*).

```tsx
import { SlideDrawer } from '@/components/layout/SlideDrawer'

<SlideDrawer
  isOpen={isOpen}
  onClose={() => setIsOpen(false)}
  title="Rincian Teknis & Riwayat Eksekusi"
  description="Log streaming dan konsumsi token agen AI"
  size="lg" // sm | md | lg | xl | full
  footer={
    <button className="btn-primary" onClick={handleSave}>Simpan Perubahan</button>
  }
>
  <div className="space-y-4">
    {/* Deep Layer content: logs, git branch, tokens */}
  </div>
</SlideDrawer>
```

### 4.4. `GroupedSection` & `InsetGroup`
Pengelompokan baris form dan preferensi terstruktur ala Things 3 dan iOS/macOS Preferences.

```tsx
import { GroupedSection, GroupedRow } from '@/components/layout/GroupedSection'

<GroupedSection
  title="Notifikasi & Otomasi"
  description="Pilih bagaimana agen memberitahukan status tugas workspace Anda."
  footerNote="Pembaruan disimpan secara otomatis ke profil Anda."
>
  <GroupedRow
    label="Pemberitahuan Suara Web Audio"
    description="Mainkan nada lembut saat tugas selesai"
    control={<Switch checked={audioEnabled} onCheckedChange={setAudioEnabled} />}
  />
  <GroupedRow
    label="Kirim Laporan Ringkasan ke WhatsApp"
    description="Ringkasan penjualan harian setiap pukul 21:00"
    control={<Switch checked={waSummary} onCheckedChange={setWaSummary} />}
  />
</GroupedSection>
```

---

## 5. Token Desain & Nilai Referensi CSS

| Kategori | Token / Class Tailwind | Nilai Nyata / Keterangan |
|----------|------------------------|--------------------------|
| **Font Family (Sans)** | `font-sans` | `'Geist Variable', ui-sans-serif, system-ui, sans-serif` |
| **Font Family (Mono)** | `font-mono` | `'Geist Mono Variable', ui-monospace, monospace` |
| **Batas Kolom Baca** | `max-w-[75ch]` | ~680px – 760px lebar maksimal untuk paragraf & chat |
| **Garis Pembatas (Hairline)** | `border-border/40` | Warna border dengan transparansi 40% (sangat halus) |
| **Garis Pemisah (Divider)** | `divide-border/40` | Pembatas baris vertikal antar-elemen daftar |
| **Micro-indicator Size** | `h-2 w-2` (8px) | Standar ukuran `StatusDot` |
| **Touch Target Minimal** | `min-h-[44px]` | Standar aksesibilitas WCAG 2.1 AA |
| **Radius Inset Group** | `rounded-xl` | 12px sudut melengkung halus untuk grup formulir |
| **Backdrop Drawer** | `backdrop:bg-black/40` | Latar belakang semi-transparan dengan `backdrop-blur-sm` |

---

## 6. Daftar Larangan Desain (Design Anti-Patterns)

1. **DILARANG:** Menumpuk `Card` di dalam `Card` (*Nested Cards*). Jika data perlu dikelompokkan, gunakan `GroupedSection` atau `HairlineList`.
2. **DILARANG:** Memasang tombol aksi mencolok (`action-primary`, *View result*, *Live Terminal*, *Retry*) di setiap baris daftar tugas. Seluruh baris harus bersih ala Things 3.
3. **DILARANG:** Menggunakan warna merah/kuning untuk status yang normal atau berhasil. Warna merah dicadangkan khusus untuk error kritis (*Status-by-Exception*).
4. **DILARANG:** Membiarkan kolom baca teks membentang hingga 100% lebar layar pada monitor desktop lebar tanpa batas `max-w-[75ch]`.
5. **DILARANG:** Menampilkan raw JSON, log streaming panjang, atau prompt mentah langsung di aliran pesan utama (*Glanceable Layer*). Buka di `SlideDrawer`.
6. **DILARANG:** Menggunakan animasi berputar (*spinning spinner*) berukuran besar yang membuat cemas. Gunakan denyut lembut `StatusDot` (`animate-pulse`) atau skeleton tenang.
7. **DILARANG:** Menggunakan peninggalan kata "Toko" pada teks antarmuka atau kamus locale. Gunakan terminologi netral dan inklusif: "Workspace", "Ruang Kerja", atau "Bisnis".

---

## 7. Standar Evaluasi Visual & Kualitas Pengalaman Pengguna (Usability & Low Visual Noise Quality Gate)

Inspeksi hasil pengerjaan antarmuka **TIDAK BOLEH** hanya terpaku pada verifikasi teknis sempit (seperti sekadar memeriksa hilangnya garis stroke 1px atau kelulusan regex pada unit test).

**Tujuan Hakiki (The Real Goal):** Mencapai **Standar Kenyamanan & Ketenangan Pengguna (Zero Cognitive Fatigue)** yang setara dengan Things 3 dan Bear Notes.

Scrum Master dan Development Team **WAJIB** mengevaluasi hasil render secara menyeluruh (*holistic assessment*) berdasarkan 4 pilar kenyamanan berikut:

### 7.1. Eliminasi Visual Noise & Ketenangan Monokromatik (Zero Visual Noise)
- Seluruh antarmuka tidak boleh memiliki elemen yang saling berebut perhatian pengguna (*no screaming elements*).
- Dilarang ada warna jenuh/neon di dark mode. Aksen warna harus tenang, teredam, dan monokromatik.
- Penanda tanggal atau status tidak boleh menggunakan latar putih solid (`#ffffff`) atau kontras menyilaukan.

### 7.2. Whitespace & Breathing Room (Ruang Bernapas yang Cukup)
- Tata letak tidak boleh sesak (*cramped*) seperti dashboard SaaS / DevOps analitik konvensional.
- Padding, margin, dan pembatas harus memberikan ruang bernapas lega sehingga mata pengguna rileks saat membaca dalam durasi panjang.

### 7.3. Hierarki Tipografi Alami (Effortless Readability)
- Pengguna harus dapat membedakan konteks (siapa pengirim, judul pesan, isi ringkasan) secara instan hanya dari bobot font (`font-medium` vs `text-muted-foreground`), tanpa memerlukan pembungkus kotak/wireframe berlapis-lapis.
- Gelembung percakapan mengalir alami tanpa stroke border ganda yang kaku.

### 7.4. Usability & Keutuhan Konteks Alur Kerja (Predictable Interaction Flow)
- Aliran komunikasi utama tidak boleh disela atau diputus oleh widget birokrasi, banner duplikasi, atau pergantian halaman yang membingungkan.
- Tindakan sekunder (seperti detail tugas atau log teknis) harus diakses secara tenang melalui *Progressive Disclosure* (`SlideDrawer`).

### 7.5. Protokol Evaluasi Nyata SM & Tim:
1. Render aplikasi dan ambil tangkapan layar nyata (*real screenshot*) pada mobile viewport (390px) dan desktop.
2. Ajukan pertanyaan evaluasi kunci: *"Apakah layar ini terasa tenang, bersahabat, dan memanjakan mata saat dipakai berjam-jam seperti Bear Notes & Things 3, atau masih menyisakan kebisingan visual?"*
3. **Jika belum mencapai standar kenyamanan ini, tim WAJIB merevisi dan memperbaiki desainnya kembali hingga benar-benar memuaskan sebelum dinyatakan selesai atau di-deploy.**
