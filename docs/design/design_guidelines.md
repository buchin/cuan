# Cuan Calm Design System — Single Source of Truth

**Topic:** Calm Software Philosophy (Things 3 & Bear Notes Inspired) for Agentic Project Management
**Target Audience:** Solopreneurs, small teams, creator businesses, and non-technical entrepreneurs
**Repository:** `cuan` (`RPcd484fc4229ae192d17a0f01`)
**Status:** Living Design System Guideline
**Version:** 1.3.0 (Updated in Sprint 62: Things 3 & Bear Notes Navigation & Visual Calm — Mobile Nav 4-Pillar with Contacts, Schedules Consolidation, Floating Soft Surface Composer, Universal Inset Groups & Terminal Streaming)
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

### 2.10. Studio, Inbox & Mobile Navigation
- The user-facing entry label is **New work**, with the Plus icon, in both English and Indonesian. Keep `/studio` and internal Studio identifiers stable. It opens the existing writing canvas; submission still starts an Inbox conversation before any task approval.
- Studio (`/studio`) is the quiet entry point for an idea. Preserve the existing heading and writing canvas; keep only text, attachments and Send. No agent/project pickers, preference banners, memory chips or redundant return links.
- A successful submission opens its Inbox conversation. Drafts survive reload and failed delivery; retries reuse the saved request identifier. Explicit `?project=` context remains supported without a picker.
- Desktop sidebar includes Studio, Inbox, Tasks and the existing management links. Mobile bottom navigation is Studio, Inbox, Tasks, Projects and Account; Contacts and Settings remain available through the account menu. Keep touch targets at least 44px.
- Mobile page headers show the current section icon beside the title, using the same icon as navigation. Studio has no extra header; detail views prioritize back navigation and object identity. Desktop headers need no duplicate section icon.

### 2.11. Konsolidasi Rute Otomasi Jadwal ke Settings (/settings/schedules)
- **Aturan Utama:** Menghilangkan link / item menu redundan `Schedules` dari sidebar desktop dan drawer mobile. Seluruh akses manajemen jadwal/rutinitas otomatis dikonsolidasikan ke `SettingsView.tsx` (`/settings/schedules`).
- **Mental Model Clarity:** Otomasi jadwal adalah konfigurasi operasional latar belakang sistem (*system automation setting*), bukan daftar tugas interaktif harian. Deep link dan bookmark lama diarahkan secara mulus via redirect otomatis tanpa broken link.

### 2.12. Kanvas Chat Murni Borderless & Floating Soft Surface Composer
- **Aturan Utama:**
  - Menghilangkan total garis horizontal 1px kaku (`border-b border-border/40` pada header chat dan `border-t border-border/40` di atas composer input).
  - Header chat mobile menampilkan identitas kontak/agen yang ramah dan manusiawi, bebas dari potongan teks ID tiket teknis pengembang.
  - Mengeliminasi total chip memori teknis AI (`TasteGroundingChips`) dari balon percakapan obrolan (ekspos jeroan AI yang menabrak timestamp); konfigurasi preferensi dipusatkan di Settings.
  - Wadah composer bertransformasi menjadi *floating soft surface tint* borderless (#1e2023 / surface elevation) ala Bear Notes dengan tombol kirim sirkular lembut.

### 2.13. Universal Inset Group Audit & Real-Time Terminal Streaming
- **Aturan Utama:**
  - Seluruh halaman dan turunannya (Tasks, Projects, Contacts, Settings) mematuhi audit Things 3: 0 border kawat 1px kaku, 0 pil putih solid `#ffffff`, konsisten menggunakan `GroupedSection`, `HairlineList`, `border-border/40` dan `bg-card/30` - `bg-card/50`.
  - Komponen `TaskTerminal.tsx` dan backend Go `agy_adapter.go` / `runner.go` menggunakan format keluaran `stream-json` seketika. Log mengalir lancar dengan incremental poller (`after_id=N`) tanpa buffering delay dan dilengkapi indikator denyut hijau zamrud (*Streaming live logs…*).

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

### 7.6. Aksen navigasi dan skala baca

Arah yang disetujui: calm tidak harus seluruhnya abu-abu. Gunakan aksen warna
lembut dan konsisten pada ikon navigasi serta indikator status; permukaan card,
background, dan teks utama tetap netral. Token navigasi dan status di `index.css`
memiliki pasangan light/dark dengan chroma rendah. Warna tidak menjadi satu-satunya
penanda: label, ikon, dan status teks tetap tersedia.

Pertahankan Geist untuk UI, dengan monospace untuk terminal, kode dan identifier
teknis. Header halaman bersama menggunakan20px mobile/24px desktop, judul menu dan
task14px medium, isi chat15–16px, metadata12px. Identitas akun: avatar36px pada
trigger/40px pada menu, nama14px medium, email12px muted, target klik minimal56px.

### 7.7. Progres pekerjaan dan ekor bubble

Persetujuan terbaru: gunakan centang `--status-success` hijau lembut untuk tugas
selesai, dengan label dan permukaan netral. Komponen bersama `TaskProgress`
menampilkan pie16px di samping judul Inbox/nama project; hanya `done` mengisi pie,
`review` belum selesai, canceled dikecualikan. Semua selesai menjadi centang;
tanpa tugas/data tidak menampilkan indikator. Jumlah tepat tersedia lewat judul
indikator dan nama aksesibel. Ringkasan server mencakup seluruh task, bukan jumlah
yang kebetulan sedang dimuat pada halaman.

Ekor bubble kecil diperbolehkan sesuai persetujuan user: hanya bubble ucapan
pertama per rangkaian pengirim yang sama; warna mengikuti permukaan, tanpa garis
atau bayangan tambahan. Pergantian hari/pengirim, visibilitas internal, atau jeda
lima menit memulai rangkaian baru. Preview task/file bukan bubble ucapan dan tidak
mendapat ekor. Bentuk umum komponen mengikuti tema yang sudah ada.

Penyempurnaan yang disetujui: sembunyikan pie pada progres0%; Inbox juga tidak
menampilkan pie untuk satu task. Penanda **Perlu perhatian**, working, dan review
aktual tetap terlihat. Tugas dibatalkan tidak dihitung sebagai review hanya karena
status penyimpanan hasil parsial masih review. Semua pekerjaan tersisa selesai
dengan pembatalan memakai lingkaran penuh netral; centang hijau berarti selesai
tanpa pembatalan. Pie dapat diketuk/di-hover untuk rincian angka, dengan target44px,
ikon16px, dan kontrol yang terpisah dari tombol/link pembuka percakapan/project.
Menu titik tiga daftar Inbox tampil on-demand melalui hover, tekan lama, atau
keyboard. Cuplikan laporan sistem menggunakan bahasa singkat dan kondisi terkini;
pesan asli dan rincian laporan tetap utuh di dalam percakapan.

Sesuai persetujuan user terbaru, avatar Inbox dan account kembali bulat,
termasuk menu akun. Trigger account berukuran36px di desktop,28px di mobile,
dan40px di menu. Pie progres dan titik status tetap bulat. Ketentuan ini
menggantikan aturan avatar kotak sebelumnya.

Preview task dan draf balasan merupakan lampiran terpisah yang rata kiri.
Gunakan frame `inbox-attachment` dengan batas lebar92% yang sama seperti
pesan incoming. Jangan membungkus task standalone dengan bubble kosong atau
mengembangkan kartu hingga memenuhi kolom percakapan. Footer tetap ringkas
dengan target aksi44px.

### 7.8. Internal visibility markers

Internal customer-room messages and owner-only task/attachment surfaces use the
shared `InternalVisibility` eye-off indicator in their leading corner/header.
Do not repeat yellow explanatory banners, sender pills or amber border rings on
every bubble. The icon is muted14px with a44px touch target. Tap/keyboard opens
the explanation; desktop hover may reveal it. Popovers support Escape, outside
dismissal and viewport bounds, including native file-preview dialogs.
Only mark surfaces known to be internal: message visibility comes from persisted
`is_internal`; unsent attachments follow the actual composer mode; customer-room
task cards are owner-only surfaces. Public messages/attachments have no marker.
The indicator explains access and never grants or changes it.

### 7.9. Task progress and terminal access

Task details show one current status, the latest progress report from the current
attempt, and summary/result sections only when content exists. Expandable task
settings have a visible chevron and a44px target. The running action says
**Buka terminal / Open terminal**, with the terminal icon, and opens the actual
interactive session. Queued attempts have no active terminal action.

Following the user's explicit request, task list rows may expose one quiet
terminal link for an actual running attempt: visible on mobile, on hover or
keyboard focus on desktop. It is a separate target from opening task details.
This exception supersedes the earlier blanket prohibition of terminal actions
on list rows. Do not infer a live terminal from task status alone.

Task previews use a softly tinted identity header. Content and footer share the
same surface; CardActions separates the footer with a hairline and gives its
buttons a32px muted visual surface within a44px touch target, with no extra
vertical footer padding. Status
labels use icon and text without a button-shaped background.
Reply drafts use the same action treatment.

### 7.10. Shared Inbox and terminal composer

Inbox and terminal use Composer, ComposerInput and ComposerSendButton for layout,
typography, resizing and the send control. Draft state, attachments and transport
remain owned by each caller. The send control occupies the full right edge of the
composer with a48px width, a minimum44px touch height, and a quiet accent surface.
There are no outer top/right/bottom gaps. Multiline input grows within the existing
bounded height. This user-approved edge action supersedes the earlier circular
send-button treatment. Public/internal modes and terminal input routing retain
their existing semantics. Streaming terminal output must never clear typed input.

ComposerAccessory centers attachment/quick-reply icons in a44px accessory row.
For single-line input their centers align with Send; for multiline input the
accessory row remains at the bottom. Inbox and terminal use the same primitive.
Single-line ComposerInput uses a44px minimum height,20px line height and12px
vertical padding so text and accessory/send icons share the same visual center.
The resize helper uses that same minimum on mobile and desktop.

Customer reply modes use tabs flush with the composer's left edge and top edge.
The selected tab shares the input surface; inactive tabs remain transparent and
muted, without an enclosing border. Internal mode retains its lock icon. Tab
selection supports arrow/Home/End keys and preserves the draft without sending.
Attachment previews appear above mode tabs so they never separate tabs from input.
Reply mode tabs use a32px visible surface within their44px touch targets, with
12px medium labels. The active surface occupies the bottom32px so it still joins
the composer; the upper12px remains transparent.
The Inbox composer dock has a transparent outer background and no extra top
padding. Only the active tab, input and action surfaces are tinted. Internal mode
is identified by its lock tab/send icon; do not repeat an eye-off marker beside
the mode tabs. Persisted internal messages and unsent attachments retain their
own visibility markers.

### 7.11. Visible execution and automated review

Task cards and details distinguish active execution from automatic result review.
The read-only `review_state` projection is tied to the latest attempt's live Cuan
review request, never a historical review or a status guess. Running review shows
**Memeriksa hasil / Checking result** with a small round pulse; queued review is
static. Completed, expired and canceled requests do not pulse. Manual result
approval is unavailable while the coordinator review is active. Respect reduced
motion. Display a task result card before its ready-to-use customer reply draft,
including when both are attached to the same message. Do not create extra status
messages just to animate progress.

For multiline composers the send surface still fills the right edge, while the
send icon stays centered in the bottom44px accessory row with attachment/quick
reply icons. This avoids moving the icon upward as the input grows.

### 7.12. Actionable attention states

A task that needs attention states its known obstacle in plain language and
suggests one next step. Detailed native errors, durations and diagnostics remain
in task details. Do not infer OOM or another root cause from a timeout/exit code.
Detail stays at the left of the existing footer. A concrete, saved agent
recommendation is shown before the right-aligned Others and Approve actions.
Approve selects that exact persisted proposal for the existing task; it does not
accept the result or create a replacement task. Bind approval to the current
attempt and saved independent assessment; reject stale, canceled, changed, or
already running work. Without a verified proposal, do not fabricate an Approve
action. Others focuses the internal composer, preserves unsent text, and sends
nothing. Users can also type normally. Running/queued reviews hide intervention
actions and keep their truthful progress state.

Use the shared amber warning token for attention, green for completion, and the
Inbox blue token for primary footer actions. `CardActions` owns the calm primary
action surface via `data-primary-action`; reuse it for Review & Use and task next
steps. Keep 44px targets with the existing 32px visible surface. Do not add a
second nested box, prominent warning banner, or extra vertical footer margins.

### 7.13. Response timeline and shared headers

Use `StatusDot` for response timeline markers so the global square-surface theme
cannot flatten them. Connect consecutive dots with a muted vertical line; stop
the line at the last item. Only the final dot of a running/cancelling response
pulses. Queued and terminal states are static, and reduced motion is respected.

Active response details follow new activity until the user scrolls upward.
Preserve that reading position across live updates. Scrolling back to the bottom
or selecting Latest activity resumes following. Do not force-scroll a user who
is reading older events. Opening a stopped response starts at its summary.

`SurfaceHeader` owns the shared leading/title/description/badge/action layout.
Panel and card variants retain their hierarchy; compact/normal presets control
spacing instead of arbitrary per-screen pixel values. `SlideDrawer` and Inbox
task attachments reuse it. Close controls retain a minimum 44px touch target.

### 7.14. Chat readability

Inbox is a conversation, not a meeting or status report. The shared
`prompt.ConversationStyle` guides coordinator replies and result-review messages.
Adapt depth to the current question and known context instead of enforcing a
character, word, sentence or paragraph quota. Lead with the answer or decision,
keep one idea per paragraph, and use readable sentences and useful whitespace.
Use lists for actual options or steps; headings and diagrams are optional tools
for an explanation that benefits from them, rather than a mandatory template.

Do not repeat an earlier catalogue, force a technical impact matrix into a small
UI question, or read repository files to answer a purely conceptual follow-up.
Current implementation claims still require relevant source evidence. Preserve
the reason, material tradeoff, limitation and next step needed to understand the
answer; brevity must not discard important context.

Review fallbacks publish a plain outcome without copying the full DoD document
or promising an automatic correction before launch admission. Full checklists,
verification notes and diagnostic correction instructions remain on the task.
Only substantive progress, obstacles, outcomes or decisions warrant a new chat
update. Historical messages remain unchanged.

### 7.15. File preview exit and controls

File previews use SurfaceHeader with a persistent,44px Close target beside the
truncated filename. Secondary Preview/Raw, zoom, copy and download controls wrap
below it and cannot push Close outside a narrow viewport. Image controls appear
once. Escape closes the viewer and restores focus to the attachment trigger.

### 7.16. Images in conversations

Persisted Inbox images use a large, bounded, uncropped preview above the message
caption. The image itself opens the file viewer; omit the filename, size and
redundant eye action from a successfully loaded photo. Preserve authentication,
internal visibility, loading/error states and the existing viewer's Close,
Escape and focus restoration. Documents and unsent composer attachments retain
the compact file treatment. Image loading failure must retain an accessible
filename and a way to open the viewer.

Inbox photo-only surfaces place images flush against the top and side edges.
Captions retain ordinary text padding. Without a caption or email delivery label,
place the timestamp on a small translucent dark overlay at the bottom right;
retain readable receipt icons. Whisper speech uses a quiet warm tint in both
themes, alongside the existing privacy marker. Public speech remains neutral.

### 7.17. One work preview and adaptive work panel

A conversation renders one current preview per task. Put its latest saved progress
below the title and before CardActions. Background review updates, response timers,
file-delivery bubbles and attempt histories belong in task Detail. Preserve real
conversation, owner decisions, final outcomes and ready-to-use customer drafts.
Classify native work records by explicit server projection; never hide arbitrary
agent speech merely because it mentions a task. Stored messages remain intact.

Detail and Terminal are independent destinations, not tabs in a miniature drawer.
At1600px and above the selected destination becomes a fourth column, keeping chat
usable. From1200px to1599px it replaces the chat column while the conversation
list stays visible. Below1200px it fills the screen with Back. Browser Back must
work, preserving the chat draft and scroll position. Terminal fills its available
height using the actual attempt and live session; never invent a fallback attempt.
Detail contains no embedded terminal or redundant terminal link.

Use TaskTimeline for chronological saved progress, reviews and delivered files.
Round StatusDots connect with a muted vertical line; only the final confirmed
running execution/review pulses. Successful execution alone is not accepted work.
Reports and technical diagnostics use disclosure. Follow new activity until the
reader scrolls upward; Latest activity resumes following. No repeating generic
Progress heading on each checkpoint, and no decorative nested status boxes.

The work panel header carries the task title instead of a generic Task details
label, without repeating it in the body. Clamp long titles to two lines; tapping
the title expands it. Provider routing and other runner/system notes remain
technical evidence, not user progress or ordinary Note events.

Timeline report bodies use 14px text with 22px line height, medium-weight step
labels and quiet 11px timestamps. Collapse long bodies after four rendered lines
using the shared CollapsibleChatText, with View report to expand the original
Markdown. Determine overflow from rendered height, not a character quota.
