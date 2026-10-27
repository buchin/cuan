# Panduan Arsitektur & Operasional: Mobile Responsive Overhaul & Minimalist Task Creation Form (Model Minimalis)

## 1. Ikhtisar & Mental Model Arsitektur

Sprint 19 mentransformasi antarmuka Cuan agar **100% responsif, ergonomis, dan intuitif di perangkat mobile (< md / 360px–414px)** sekaligus menghadirkan **Minimalist Task Creation Form (Model Minimalis)** yang memungkinkan pencatatan ide dan tugas tuntas dalam waktu kurang dari 3 detik.

### Latar Belakang Masalah (Pre-Sprint 19)
Pada versi sebelumnya, layout Cuan dirancang dengan paradigma desktop 3-tier:
1. **Tier 1:** Primary Icon Rail selebar 48px di paling kiri.
2. **Tier 2:** Contextual Sub-Panel selebar 240px di tengah.
3. **Tier 3:** Main Viewport di kanan.

Ketika dibuka pada layar smartphone (misalnya iPhone SE 375px atau Android standar 360px), kombinasi Tier 1 (48px) dan Tier 2 (240px) menghabiskan 288px dari total lebar layar. Akibatnya, Main Viewport chat hanya menyisakan **72px** ruang horizontal! Balon pesan terpotong, teks terjepit parah, dan tombol-tombol antarmuka tidak dapat digunakan secara wajar. Selain itu, formulir pembuatan task lama memiliki 6 kolom bertingkat dan builder DoD rumit yang memerlukan waktu pengisian lama serta scrolling vertikal melelahkan di layar HP.

### Solusi Arsitektur Sprint 19
1. **Mobile Responsive Shell (S19.1):**
   - Rail samping 48px disembunyikan sepenuhnya pada mobile (`hidden md:flex`).
   - Bottom Navigation Bar (`md:hidden`) hadir di bagian bawah viewport dengan 4 tab utama: **Inbox**, **Projects**, **Agents**, dan **Team**.
   - Container utama diberi padding bawah `pb-16 md:pb-0` agar seluruh konten aman dari tumpukan bottom bar.
   - Top bar header mobile menyediakan akses instan ke tombol **Theme Toggle** (Light/Dark) dan **User Profile Avatar** (dengan popover Ganti Password & Logout).
2. **WhatsApp/Telegram Mobile Single-View Navigation (S19.2):**
   - Mengadopsi standar perpesanan modern smartphone:
     * **Mode Kontak (Contacts List):** Daftar kontak tampil **100% lebar layar HP** lengkap dengan filter (All, Agents, Customers) dan pencarian instan.
     * **Mode Chat (Chat Stream):** Saat kontak diklik, layar berganti menampilkan obrolan **100% lebar layar HP** dengan tombol Back (`<`) di kiri atas header.
     * Mengetuk tombol Back (`<`) mengembalikan tampilan ke daftar kontak.
   - Pada desktop (`>= md`), sistem tetap mempertahankan tata letak split view berdampingan (subpanel 240px di kiri + chat stream di kanan).
3. **Minimalist Task Creation Form — Model Minimalis (S19.3):**
   - Input judul tugas langsung mendapatkan fokus otomatis (`autoFocus`) dengan placeholder `"Apa yang ingin dikerjakan?..."`.
   - Textarea deskripsi opsional.
   - Dropdown project otomatis terisi project yang sedang aktif.
   - Shortcut keyboard instan: Tekan **Enter** pada judul langsung menyimpan tugas; tekan **Ctrl+Enter** atau **Cmd+Enter** pada deskripsi langsung submit.
   - Ukuran modal sangat kompak (~240px) sehingga pas di layar ponsel tanpa scroll vertikal.

---

## 2. Diagram Alur Kerja & Arsitektur Mobile (D2 TALA)

Diagram berikut mengilustrasikan alur navigasi responsif dan tata letak single-view Cuan di perangkat mobile dan desktop:

```d2
vars: {
  d2-config: {
    layout-engine: tala
  }
}

direction: right

user: "Pengguna (Mobile / Desktop)"

mobile_flow: "Arsitektur Mobile Viewport (< md)" {
  top_header: "Mobile Top Header (h-12)" {
    brand_badge: "Cuan Logo & Tab Title"
    theme_profile: "Theme Toggle & User Menu"
  }

  single_view: "WhatsApp Single-View Container" {
    contacts_view: "Contacts List View (100% Width)" {
      search_box: "Real-time Search"
      filter_tabs: "Filter Tabs (All / Agents / Customers)"
      contact_items: "Contact Items & Unread Badges"
    }

    chat_view: "Chat Stream View (100% Width)" {
      chat_header: "Chat Header + Back Button (<)"
      chat_bubbles: "Full-width Bubble Stream"
      composer: "Auto-expanding Composer Textarea"
    }
  }

  quick_task: "Minimalist Task Modal (Model Minimalis)" {
    title_input: "Autofocus Title ('Apa yang ingin dikerjakan?...')"
    project_select: "Active Project Dropdown"
    shortcuts: "Enter or Ctrl+Enter to Submit (< 3s)"
  }

  bottom_nav: "Bottom Navigation Bar (h-16, fixed)" {
    tab_inbox: "Inbox Tab"
    tab_projects: "Projects Tab"
    tab_agents: "Agents Tab"
    tab_team: "Team Tab"
  }
}

desktop_flow: "Arsitektur Desktop (>= md)" {
  rail: "Primary Icon Rail (48px)"
  subpanel: "Contextual Sub-panel (240px)"
  main_viewport: "Main Chat Viewport (flex-1)"
}

user -> mobile_flow: "Akses via Smartphone (360px-414px)"
mobile_flow.contacts_view -> mobile_flow.chat_view: "Klik Kontak: Buka Chat 100% Width"
mobile_flow.chat_view -> mobile_flow.contacts_view: "Klik Back (<): Kembali ke Daftar Kontak"
mobile_flow.bottom_nav -> mobile_flow.single_view: "Switch Tab Modul"
mobile_flow.top_header -> mobile_flow.quick_task: "Pemicu Form Tugas Kilat"

user -> desktop_flow: "Akses via Desktop (>= 768px)"
desktop_flow.rail -> desktop_flow.subpanel: "Navigasi Modul"
desktop_flow.subpanel -> desktop_flow.main_viewport: "Split View Berdampingan"
```

---

## 3. Mobile Responsive Shell & Bottom Navigation Bar

### 3.1 Eliminasi Primary Rail 48px di Mobile
Komponen `AppShell.tsx` menerapkan kelas utilitas Tailwind responsif:
```tsx
{/* ── TIER 1: Primary Icon Rail (48px) — Hidden on mobile ── */}
<nav className="hidden md:flex w-12 shrink-0 flex-col border-r border-border bg-sidebar">
  {/* Logo, nav buttons, theme toggle, nav user */}
</nav>
```
Dengan kelas `hidden md:flex`, elemen rail samping 48px tereliminasi sepenuhnya pada layar `< md`, mengembalikan 48px ruang horizontal berharga ke konten utama.

### 3.2 Mobile Bottom Navigation Bar (`md:hidden`)
Sebagai pengganti rail samping, dihadirkan bilah navigasi bawah tetap (*fixed bottom bar*) setinggi 64px (`h-16`):
```tsx
{/* ── Mobile Bottom Navigation Bar (md:hidden) ── */}
<nav className="md:hidden fixed bottom-0 left-0 right-0 h-16 bg-sidebar border-t border-border z-30 flex items-center justify-around px-2 shadow-lg">
  {NAV_ITEMS.map(({ id, label, icon: Icon }) => (
    <button
      key={id}
      onClick={() => onSelectTab(id)}
      className={cn(
        'flex flex-col items-center justify-center flex-1 py-1 gap-1 transition-colors relative',
        currentTab === id
          ? 'text-primary font-bold'
          : 'text-muted-foreground hover:text-foreground'
      )}
    >
      <Icon className="h-5 w-5" />
      <span className="text-[10px] tracking-tight">{label}</span>
    </button>
  ))}
</nav>
```
Semua tombol dirancang dengan area sentuh yang nyaman untuk jempol tangan (*thumb-friendly interaction*).

### 3.3 Padding Bawah Konten (`pb-16 md:pb-0`)
Agar elemen terbawah pada halaman (seperti textarea chat, tombol form, atau footer list) tidak tertutup oleh bottom bar, container konten utama menerapkan kelas dinamis:
```tsx
<div
  className={cn(
    'flex-1 overflow-hidden pb-16 md:pb-0',
    currentTab === 'inbox' ? 'p-0' : 'overflow-auto p-4 md:p-6'
  )}
>
  {children}
</div>
```

### 3.4 Top Bar Header Mobile
Header atas (`h-12`) menyediakan akses cepat:
* **Kiri:** Logo badge Cuan dan breadcrumb modul aktif (`Cuan / Inbox`).
* **Kanan (`md:hidden`):**
  - Tombol **Theme Toggle** (Sun/Moon) untuk berganti tema Light/Dark seketika.
  - Avatar pengguna dengan dropdown popover `NavUser` yang menampilkan identitas, badge OWNER, tombol *Profil & Ganti Password*, serta tombol *Keluar (Logout)*.

---

## 4. WhatsApp/Telegram Mobile Single-View Navigation

### 4.1 Logika Toggle Single-View
Pada `InboxView.tsx`, antarmuka dikelola dengan state navigasi internal:
```tsx
const [mobileView, setMobileView] = useState<'contacts' | 'chat'>('contacts')
```
* **State `contacts`:** Menampilkan `InboxSubPanel` selebar 100% viewport HP (`w-full h-full md:hidden flex-col`).
* **State `chat`:** Menampilkan chat stream selebar 100% viewport HP (`flex-1 flex flex-col h-full bg-background`).

### 4.2 Alur Interaksi
1. Pengguna membuka tab **Inbox**:
   - Di mobile: Tampil daftar kontak 100% lebar layar HP dengan kolom pencarian, filter tab All/Agents/Customers, avatar kontak, status online, dan badge pesan unread.
2. Pengguna memilih kontak (misalnya Scrum Master atau Visitor Live Chat):
   - `handleSelectContactInternal` terpanggil.
   - Layar mobile seketika beralih ke Chat Stream 100% lebar layar HP.
3. Di dalam Chat Stream mobile:
   - Header menampilkan tombol Back (`<`) di kiri avatar (`md:hidden`).
   - Balon pesan membentang hingga 85% lebar layar ponsel dengan margin yang proporsional.
   - Textarea composer membesar otomatis (*auto-expanding*) saat mengetik pesan panjang.
4. Pengguna mengetuk tombol Back (`<`):
   - `handleBackToContacts` terpanggil.
   - Layar kembali secara mulus ke Daftar Kontak 100% lebar layar.

### 4.3 Zero Regression pada Desktop
Di layar desktop (`>= md`), kelas `hidden md:flex` dan `md:hidden` memastikan:
* Sub-panel kontak 240px tetap tampil di sebelah kiri.
* Chat stream tetap tampil di sebelah kanan.
* Tombol Back (`<`) disembunyikan otomatis.
* Tata letak split view berdampingan berfungsi 100% identik dengan desktop standard.

---

## 5. Minimalist Task Creation Form (Model Minimalis)

### 5.1 Desain Zero-Friction
Formulir pembuatan task pada `web/src/features/tasks/CreateTaskModal.tsx` dirombak total dari model lama yang kaku menjadi formulir minimalis modern minimalis modern:
* **Autofocus Title:** Segera setelah modal dibuka, kursor input otomatis berada di input judul dengan placeholder ramah: `"Apa yang ingin dikerjakan?..."`.
* **Description Opsional:** Textarea ringkas 3 baris untuk catatan atau prompt instruksi tambahan tanpa kewajiban validasi.
* **Auto Project Selector:** Dropdown otomatis memilih project aktif saat ini (`activeProjectId`).
* **Priority Selector:** Pilihan ringkas (Low, Normal, High, Urgent) dengan default *Normal*.

### 5.2 Keyboard Shortcuts (< 3 Detik Pembuatan Tugas)
Pengguna dapat membuat tugas tanpa menyentuh mouse atau tombol on-screen:
* **Enter di Title:** Langsung menyimpan task dan menutup modal seketika.
* **Ctrl+Enter / Cmd+Enter di Description:** Langsung submit task saat selesai menulis catatan panjang.
* **Escape (Esc):** Langsung membatalkan dan menutup modal.

### 5.3 Ergonomi Mobile
Modal memiliki tinggi total sekitar ~240px dengan rounded corner `rounded-xl` dan bayangan lembut `shadow-2xl`. Pada viewport mobile 360px–414px, modal berada persis di tengah layar tanpa menimbulkan scrolling vertikal yang mengganggu.

---

## 6. Prosedur Verifikasi & Runbook Deployment

### 6.1 Uji Staging Otomatis (Port 8081)
Jalankan test suite verifikasi staging end-to-end:
```bash
go test -v ./internal/server -run TestVerify_S19
```
Uji ini memverifikasi:
- Auth login & gatekeeper status.
- Kompatibilitas REST API kontak direct messenger & visitor live chat.
- Integritas live chat widget Shadow DOM (`/chat/widget.js`) dengan CORS terbuka.
- Keutuhan 19 migrasi database PostgreSQL.
- Verifikasi kelas responsif mobile pada bundle frontend.

### 6.2 Build Bundle
Kompilasi binary server Go dan bundle frontend:
```bash
# Build Go server & migrate binary
mkdir -p bin
go build -o bin/cuan-server ./cmd/server
go build -o bin/cuan-migrate ./cmd/migrate

# Build Frontend
cd web && npm run build && cd ..
```

### 6.3 Deployment ke Production VPS (`cuan.dojo.cc`)
```bash
# 1. Upload Binary
sshpass -p "$DOJO_TERMAPI_VPS_PASSWORD" scp -o StrictHostKeyChecking=no \
  bin/cuan-server bin/cuan-migrate \
  $DOJO_TERMAPI_VPS_USERNAME@$DOJO_TERMAPI_VPS_PUBLIC_IP:/opt/cuan/bin/

# 2. Upload Web Assets
sshpass -p "$DOJO_TERMAPI_VPS_PASSWORD" rsync -avz --delete -e "ssh -o StrictHostKeyChecking=no" \
  web/dist/ \
  $DOJO_TERMAPI_VPS_USERNAME@$DOJO_TERMAPI_VPS_PUBLIC_IP:/opt/cuan/web/dist/

# 3. Restart Service
sshpass -p "$DOJO_TERMAPI_VPS_PASSWORD" ssh -o StrictHostKeyChecking=no \
  $DOJO_TERMAPI_VPS_USERNAME@$DOJO_TERMAPI_VPS_PUBLIC_IP \
  "sudo systemctl restart cuan && sudo systemctl is-active cuan"

# 4. Smoke Test
curl -fsS https://cuan.dojo.cc/healthz
curl -fsS https://cuan.dojo.cc/readyz
curl -fsS https://cuan.dojo.cc/api/info
curl -fsS https://cuan.dojo.cc/chat/widget.js | head -n 5
```

---

## 7. Kesimpulan & Penjaminan Mutu

Sistem arsitektur mobile dan formulir pembuatan task Sprint 19 telah terverifikasi memenuhi seluruh kriteria Definition of Done (DoD):
- **User-Centric & Ergonomis:** Pengguna mobile dapat mengoperasikan Cuan dengan satu tangan melalui Bottom Navigation Bar dan alur perpesanan WhatsApp/Telegram yang sangat natural.
- **Zero-Friction Productivity:** Pembuatan tugas baru selesai dalam hitungan detik tanpa hambatan formulir bertingkat.
- **100% Backward-Compatibility:** 19 migrasi database PostgreSQL, endpoint internal chat, dan Dojo drop-in widget `/chat/widget.js` tetap beroperasi prima tanpa degradasi.

---
*Dokumen ini merupakan bagian dari Definition of Done (DoD) Sprint 19.*
