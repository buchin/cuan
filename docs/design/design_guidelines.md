# Cuan Calm Design System — Single Source of Truth

**Topic:** Calm Software Philosophy (Things 3 & Bear Notes Inspired) for Agentic Project Management  
**Target Audience:** Solopreneurs, small teams, online store owners, and non-technical entrepreneurs  
**Repository:** `cuan` (`RPcd484fc4229ae192d17a0f01`)  
**Status:** Living Design System Guideline  
**Version:** 1.0.0 (Established in Sprint 59)  
**Author:** Buchin <buchin@dropsugar.com>  

---

## 1. Filosofi & Visi: Calm Software

Dalam era kecerdasan buatan (*Agentic Era*), perangkat lunak sering kali terjebak dalam perangkap "kelebihan visual" (*visual clutter*): dashboard dengan puluhan kartu bertumpuk, badge warna-warni yang berkedip, log teknis yang membanjiri layar, dan tombol aksi yang membingungkan.

Bagi solopreneur, pemilik toko online, dan tim kecil, tujuan utama menggunakan agen AI bukanlah untuk menonton kode berjalan, melainkan untuk **mencapai ketenangan pikiran (*peace of mind*) dan kejelasan operasional (*operational clarity*)**.

Sistem desain Cuan berakar pada prinsip **Calm Software**, yang diilhami oleh dua tolok ukur desain perangkat lunak paling elegan di dunia:
- **Things 3 (Cultured Code):** Keindahan kanvas datar, ritme visual yang lapang, hierarki tanpa kartu tebal (*card diet*), serta interaksi yang mengalir tanpa friksi.
- **Bear Notes (Shiny Frog):** Kesederhanaan tipografis, estetika monokromatik yang hangat, ketajaman teks, dan fokus mendalam pada konten tanpa ornamen artifisial.

---

## 2. Lima Prinsip Inti (Core Principles)

### 2.1. Card Diet: Larangan Tegas Nested Cards
- **Aturan Utama:** Dilarang keras menempatkan kartu di dalam kartu (*zero nested cards / no boxes inside boxes*).
- **Mekanisme Alternatif:**
  - Gunakan kanvas latar belakang yang bersih dan datar (`bg-background` atau `bg-card/50`).
  - Pemisahan antar-item dilakukan menggunakan garis batas setipis helai rambut (*hairline divider*): `border-b border-border/40` atau `divide-y divide-border/40`.
  - Hindari drop-shadow tebal (`shadow-lg`, `shadow-xl`) pada daftar item berulang. Gunakan transisi hover latar belakang lembut (`hover:bg-muted/30`) untuk menunjukkan keterbukaan interaksi.
- **Komponen Penopang:** `HairlineList` dan `HairlineListItem`.

### 2.2. Information Layering & Progressive Disclosure
- **Aturan Utama:** Pisahkan pengalaman pengguna menjadi dua lapisan yang tegas:
  1. **Glanceable Layer (Permukaan):** Menampilkan informasi esensial dan hasil ringkas yang langsung dapat dipahami dalam 2 detik (judul tugas, waktu relatif, status terkini, hasil utama).
  2. **Deep Layer (Kedalaman On-Demand):** Menyembunyikan seluruh parameter teknis (prompt mentah, log stdout/stderr, token usage, hash commit, branch git, JSON payload) hingga pengguna secara sadar meminta untuk melihatnya.
- **Mekanisme Interaksi:**
  - Klik pada baris `HairlineListItem` atau tombol detail membuka panel samping kanan yang mulus (`SlideDrawer` / `SideDrawer`) pada desktop, atau *Bottom Sheet* pada mobile.
  - Jangan pernah membanjiri aliran percakapan utama dengan ratusan baris log teknis yang merusak ritme membaca.
- **Komponen Penopang:** `SlideDrawer` (SideDrawer) dan *inline expansion*.

### 2.3. Status-by-Exception (Ketenangan Tanpa Alarm Palsu)
- **Aturan Utama:** Warna kontras tinggi (merah/amber/destructive) dicadangkan **HANYA** untuk status luar biasa (*exceptions*), galat (*errors*), kegagalan sistem, atau tindakan kritis yang mutlak membutuhkan keputusan manusia.
- **Aturan Status Normal:**
  - Status sehat, sukses, atau selesai ditampilkan secara tenang dan bersahaja: gunakan teks netral (`text-muted-foreground`) atau micro-indicator berukuran 6–8px (`StatusDot status="healthy"`).
  - Status menunggu, antre, atau tidak aktif (*idle*) ditampilkan dengan warna abu-abu netral teredam (`bg-muted-foreground/40`).
  - Status agen sedang bekerja menggunakan denyut lembut (*calm pulse*) warna biru/sky (`StatusDot status="running"`), bukan animasi berputar yang agresif.
- **Komponen Penopang:** `StatusDot`.

### 2.4. Desktop Column Constraints (Batas Kolom 65–80ch)
- **Aturan Utama:** Konten membaca, aliran percakapan, dan form instruksi pada layar desktop **WAJIB** dibatasi pada lebar optimal 65 hingga 80 karakter (*typographic measure* ~680px–760px): `max-w-[75ch]` atau `max-w-3xl mx-auto`.
- **Rasional:**
  - Membaca baris teks yang membentang dari ujung kiri ke ujung kanan pada monitor ultrawide menyebabkan mata lelah (*eye fatigue*) dan kehilangan fokus.
  - Ruang kosong di sisi kiri dan kanan (*aesthetic whitespace*) bukanlah pemborosan ruang, melainkan ruang bernapas visual (*visual breathing room*) yang menciptakan ketenangan.

### 2.5. Tipografi Geist yang Tajam & Spacing Tokens Terpadu
- **Aturan Utama:**
  - Font Sans: `@fontsource-variable/geist` (`font-sans`, `'Geist Variable'`). Karakter font netral, modern, dan sangat mudah dibaca pada ukuran kecil (11px–13px) hingga judul besar.
  - Font Mono: `@fontsource-variable/geist-mono` (`font-mono`, `'Geist Mono Variable'`) untuk ID tugas (`#GQATE`), durasi, hash git, dan angka monospaced.
  - Hierarki Spacing: Gunakan standard spacing Tailwind (skala 4px): `gap-1` (4px), `gap-2` (8px), `gap-3` (12px), `gap-4` (16px), `gap-6` (24px).
  - Touch Target Accessibility: Seluruh elemen interaktif memiliki area sentuh minimum 44x44px pada viewport layar sentuh/mobile (`min-h-[44px]`).

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
<StatusDot status="healthy" label="Toko Online Aktif" showLabel />
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
  description="Pilih bagaimana agen memberitahukan status tugas toko Anda."
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
2. **DILARANG:** Menggunakan warna merah/kuning untuk status yang normal atau berhasil. Warna merah dicadangkan khusus untuk error kritis (*Status-by-Exception*).
3. **DILARANG:** Membiarkan kolom baca teks membentang hingga 100% lebar layar pada monitor desktop lebar tanpa batas `max-w-[75ch]`.
4. **DILARANG:** Menampilkan raw JSON, log streaming panjang, atau prompt mentah langsung di aliran pesan utama (*Glanceable Layer*). Buka di `SlideDrawer`.
5. **DILARANG:** Menggunakan animasi berputar (*spinning spinner*) berukuran besar yang membuat cemas. Gunakan denyut lembut `StatusDot` (`animate-pulse`) atau skeleton tenang.
