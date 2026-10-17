# Panduan Operasional Bring Your Own Domain (BYOD), Custom Domains & On-Demand TLS

Dokumen ini merupakan panduan operasional komprehensif bagi pemilik instance (*Single-Owner*), pengembang mandiri (*Solo Developer*), agensi digital, dan Indie Hacker mengenai tata cara menghubungkan domain dan subdomain kustom (*Bring Your Own Domain*), mengonfigurasi DNS (CNAME, TXT challenge, dan A-record fallback), mekanisme keamanan Caddy On-Demand TLS via ask-endpoint, serta penanganan kendala propagasi (*troubleshooting*) pada platform Cuan.

---

## 1. Filosofi & Gambaran Umum

Fitur Bring Your Own Domain (BYOD) pada Cuan memungkinkan setiap aplikasi yang di-deploy (baik di *Local Sandbox* maupun *Remote BYOS VPS*) memiliki identitas brand kustom seperti `app.clientbrand.com`, `myproject.io`, atau `toko.umkm.id` menggantikan subdomain default platform (`*.cuan.dojo.cc`).

### Prinsip Utama BYOD Cuan
1. **Zero SSL Management (Automated On-Demand TLS):** Pengguna tidak perlu menjalankan Certbot, mengonfigurasi Cronjob renew SSL, atau mengunggah sertifikat TLS manual. Sertifikat HTTPS Let's Encrypt / ZeroSSL diterbitkan secara instan saat kunjungan web pertama dan diperbarui otomatis di background oleh Caddy Server.
2. **Anti-Abuse Ask Endpoint Protection:** Untuk mencegah serangan *Denial-of-Wallet* atau *ACME Rate-Limit Exhaustion* dari hostname liar yang diarahkan ke IP server, Caddy On-Demand TLS dilindungi oleh gatekeeper endpoint backend Cuan (`/api/caddy/ask`). Sertifikat hanya diterbitkan untuk domain yang terdaftar dan telah lulus verifikasi DNS.
3. **Multi-Tier DNS Verification Engine:** Mendukung fleksibilitas registrasi domain:
   - **CNAME Record:** Ideal untuk subdomain (`app`, `dashboard`, `api`).
   - **TXT Challenge Token:** Ideal untuk apex/root domain dan verifikasi kepemilikan anti-takeover sebelum rute diaktifkan (`_cuan-challenge.<domain>`).
   - **A-Record Fallback:** Mendukung CNAME flattening dari penyedia seperti Cloudflare dan AWS Route53.
4. **Dynamic Routing Zero-Downtime:** Penambahan dan penghapusan domain dieksekusi secara dinamis melalui Caddy Admin API tanpa reload daemon dan tanpa memutus koneksi trafik aktif.
5. **100% Data Retention Guarantee:** Skema database PostgreSQL diperluas secara murni aditif (`000010_custom_domains.up.sql`) dengan integritas foreign key `ON DELETE CASCADE`, memastikan seluruh data Sprint 1 (akun owner, task board), Sprint 2 (sponsorships, leads, orders), Sprint 3 (local deployments), dan Sprint 4 (BYOS servers) tetap 100% utuh tanpa alterasi.

---

## 2. Diagram Arsitektur E2E (E2E Architecture)

Berikut adalah diagram alur kerja siklus hidup Custom Domain dan On-Demand TLS pada Cuan:

```d2
direction: right

client: "Browser / Pengguna (Studio Dashboard & Pengunjung)" {
  shape: person
}

cuan_control_plane: "Cuan Control Plane (Go Backend)" {
  api: "Chi HTTP Router\n(/api/deployments/{id}/domains\n& /api/domains/{id}/verify)"
  domain_service: "Custom Domain Service\n(RFC 1123 Validator & Token Generator)"
  dns_engine: "DNS Verification Engine\n(CNAME, TXT Challenge, A-Record Fallback)"
  ask_gatekeeper: "Security Gatekeeper\n(/api/caddy/ask)"
}

caddy_proxy: "Caddy Web Server (Dynamic Reverse Proxy)" {
  tls_engine: "On-Demand TLS Engine\n(Let's Encrypt / ZeroSSL ACME)"
  vhost_router: "Dynamic SNI Router"
}

authoritative_dns: "Authoritative Public DNS\n(Cloudflare / Route53 / Namecheap / Provider)"

database: "PostgreSQL Database" {
  table_deployments: "deployments"
  table_custom_domains: "custom_domains (000010)"
}

runner: "Application Upstream\n(Local Port / Remote BYOS Host:Port)"

client -> cuan_control_plane.api: "1. Bind Domain (e.g. app.client.com)"
cuan_control_plane.api -> cuan_control_plane.domain_service: "Validasi RFC 1123 & Generate Token"
cuan_control_plane.domain_service -> database.table_custom_domains: "Simpan domain (status: pending)"
cuan_control_plane.api -> client: "Instruksi DNS (CNAME, TXT, A)"

client -> authoritative_dns: "2. Konfigurasi DNS Record di Registrar"
client -> cuan_control_plane.api: "3. Klik 'Verify DNS Now'"
cuan_control_plane.api -> cuan_control_plane.dns_engine: "Jalankan Query Resolusi DNS"
cuan_control_plane.dns_engine -> authoritative_dns: "Lookup CNAME / TXT / IP"
cuan_control_plane.dns_engine -> database.table_custom_domains: "Update Status (verified)"
cuan_control_plane.dns_engine -> caddy_proxy.vhost_router: "Registrasi Dynamic Route Caddy"

client -> caddy_proxy.vhost_router: "4. Kunjungan Pertama HTTPS https://app.client.com"
caddy_proxy.tls_engine -> cuan_control_plane.ask_gatekeeper: "Probe: /api/caddy/ask?domain=app.client.com"
cuan_control_plane.ask_gatekeeper -> database.table_custom_domains: "Validasi Status Domain"
cuan_control_plane.ask_gatekeeper -> caddy_proxy.tls_engine: "HTTP 200 OK (Authorized)"
caddy_proxy.tls_engine -> client: "Handshake TLS & Terbitkan Sertifikat"
caddy_proxy.vhost_router -> runner: "Reverse Proxy ke Internal Runner"
runner -> client: "Aplikasi Terbuka Aman (Gembok Hijau HTTPS)"
```

---

## 3. Persyaratan & Validasi Format Domain

Setiap nama domain yang didaftarkan ke Cuan harus memenuhi standar nama host internet (RFC 1123 / RFC 952):
- **Panjang:** Total panjang maksimum 253 karakter; panjang tiap label (*sub-label*) antara 1 hingga 63 karakter.
- **Karakter:** Hanya mengizinkan huruf alfabet (`a-z`), angka (`0-9`), dan tanda hubung (`-`). Tanda hubung tidak boleh berada di awal atau akhir label.
- **Case Sensitivity:** Otomatis dikonversi ke huruf kecil (*lowercase*) sebelum disimpan atau divalidasi.
- **Larangan Format:** Dilarang menyertakan skema protokol (`http://` atau `https://`), nomor port (`:8080`), garis miring / path (`/app`), atau spasi.
- **Apex vs Subdomain:**
  - *Subdomain* (contoh: `app.bisnis.id`, `shop.brand.com`): Memiliki minimal 3 segmen label. Sangat disarankan menggunakan konfigurasi **CNAME**.
  - *Apex / Root Domain* (contoh: `bisnis.id`, `brand.com`): Hanya memiliki 2 segmen label. Karena standar DNS RFC tidak mengizinkan CNAME pada root zone tanpa flattening, gunakan **TXT Record Challenge** + **A Record**.

---

## 4. Panduan Langkah demi Langkah Konfigurasi DNS

Setelah menambahkan custom domain di Studio Dashboard Cuan, sistem akan menyediakan 3 pilihan konfigurasi DNS yang interaktif:

### Opsi A: CNAME Record (Sangat Direkomendasikan untuk Subdomain)
Gunakan opsi ini jika Anda menggunakan subdomain seperti `app.domainanda.com`.

| Field / Tipe | Host / Name | Target / Points To | TTL |
| --- | --- | --- | --- |
| **CNAME** | `app` (atau `app.domainanda.com`) | `cuan.dojo.cc` (atau Base Domain Cuan) | `300` detik (Auto / 5 menit) |

> [!TIP]
> Pada penyedia seperti Cloudflare, pastikan status proxy diatur ke **DNS Only** (awan abu-abu) saat pertama kali melakukan verifikasi dan penerbitan sertifikat SSL Caddy.

---

### Opsi B: TXT Record Challenge (Ownership Verification untuk Apex & Subdomain)
Gunakan opsi ini untuk memverifikasi kepemilikan domain secara eksklusif menggunakan token acak 32-karakter unik yang diterbitkan Cuan.

| Field / Tipe | Host / Name | Value / Text Content | TTL |
| --- | --- | --- | --- |
| **TXT** | `_cuan-challenge.domainanda.com` | `tok_67a9b0c2...` (Token dari UI) | `300` detik (Auto / 5 menit) |

Alternatif: Anda juga dapat memasukkan token di root TXT record domain dengan nilai:
`cuan-verification=tok_67a9b0c2...`

---

### Opsi C: A-Record Fallback (Khusus Root / Apex Domain atau CNAME Flattening)
Gunakan opsi ini jika DNS provider Anda tidak mengizinkan CNAME pada apex domain dan tidak mendukung CNAME flattening otomatis.

| Field / Tipe | Host / Name | Target IP Address | TTL |
| --- | --- | --- | --- |
| **A** | `@` (atau `domainanda.com`) | `127.0.0.1` (IP Publik Host Cuan Server) | `300` detik (Auto / 5 menit) |

---

### Panduan Berdasarkan DNS Provider Populer

#### 1. Cloudflare
1. Buka dashboard Cloudflare dan pilih domain Anda.
2. Masuk ke menu **DNS** -> **Records** -> klik **Add record**.
3. Pilih Type: **CNAME**, Name: `app`, Target: `cuan.dojo.cc`.
4. **PENTING:** Ubah **Proxy status** menjadi **DNS only** (ikon awan abu-abu). Jika Proxy aktif (awan oranye), Cloudflare akan mencegat koneksi HTTPS sebelum mencapai Caddy, sehingga On-Demand TLS ACME HTTP-01 challenge dapat gagal.
5. Klik **Save**.
6. Tambahkan TXT Record: Type: **TXT**, Name: `_cuan-challenge.app`, Content: `<verification_token>`, TTL: Auto.

#### 2. Namecheap
1. Login ke akun Namecheap, pilih **Domain List** -> klik **Manage** pada domain Anda.
2. Buka tab **Advanced DNS** -> klik **Add New Record**.
3. Tambahkan CNAME:
   - Type: `CNAME Record`
   - Host: `app`
   - Target: `cuan.dojo.cc`
   - TTL: `Automatic` atau `5 min`
4. Tambahkan TXT:
   - Type: `TXT Record`
   - Host: `_cuan-challenge.app`
   - Value: `<verification_token>`
   - TTL: `Automatic`

#### 3. GoDaddy
1. Buka **DNS Management** domain di GoDaddy.
2. Pada bagian **DNS Records**, klik **Add**.
3. Type: `CNAME`, Name: `app`, Value: `cuan.dojo.cc`, TTL: `1/2 Hour`.
4. Klik **Add** lagi untuk TXT: Type: `TXT`, Name: `_cuan-challenge.app`, Value: `<verification_token>`, TTL: `1/2 Hour`.

#### 4. AWS Route 53
1. Buka Hosted Zone domain Anda di Route 53.
2. Klik **Create record**.
3. Record name: `app`, Record type: `CNAME`, Value: `cuan.dojo.cc`, TTL: `300`.
4. Untuk verifikasi token: Buat record baru Type `TXT`, Name: `_cuan-challenge.app`, Value: `"<verification_token>"`.

---

## 5. Mekanisme On-Demand TLS Caddy & Security Gatekeeper

Caddy Web Server dilengkapi fitur **On-Demand TLS**. Fitur ini memungkinkan penerbitan sertifikat SSL secara dinamis tanpa perlu menuliskan nama domain di Caddyfile terlebih dahulu dan tanpa reload daemon server.

### Proteksi Ask Endpoint (`/api/caddy/ask`)
Tanpa filter pengaman, siapa saja dapat mengarahkan domain sembarang ke IP server Cuan dan memicu penerbitan sertifikat SSL massal yang menghabiskan kuota rate limit Let's Encrypt. Cuan memitigasi risiko ini melalui endpoint gatekeeper:

```
GET /api/caddy/ask?domain={domain}
(Alias: /api/domains/check-ask?domain={domain})
```

#### Logika Evaluasi Gatekeeper:
1. Normalisasi nama domain (lowercase, trim).
2. Validasi format RFC 1123. Jika tidak valid, mengembalikan status **HTTP 403 Forbidden**.
3. Pencarian domain di tabel `custom_domains` PostgreSQL. Jika tidak terdaftar, mengembalikan status **HTTP 403 Forbidden**.
4. Pengecekan status verifikasi: Domain harus memiliki status `dns_status = 'verified'` atau `ssl_status = 'active'`.
5. Jika memenuhi syarat, mengembalikan status **HTTP 200 OK** (`{"status": "authorized", ...}`). Caddy melanjutkan pertukaran kunci ACME dan menerbitkan sertifikat TLS.
6. Jika domain masih berstatus `pending` atau `failed`, mengembalikan **HTTP 403 Forbidden**.

---

## 6. Manajemen Custom Domain di Studio Dashboard

### Menghubungkan Domain Baru
1. Masuk ke Studio Dashboard (`/studio`), pilih deployment aktif pada tab **Projects** atau **Deployments**.
2. Klik tombol **"Custom Domains"** atau ikon bola dunia (🌐).
3. Masukkan nama domain Anda (misalnya `app.startupku.id`) pada kolom isian.
4. Klik **"Add Domain"**. Sistem akan membuat entitas custom domain dengan status `pending` dan menampilkan token verifikasi DNS.

### Melakukan Verifikasi DNS
1. Setelah mengonfigurasi DNS di registrar, tunggu 1-2 menit untuk propagasi awal.
2. Klik tombol **"Verify DNS"** di sebelah domain terkait.
3. Backend Cuan akan menjalankan query CNAME, TXT challenge, dan A-record fallback secara instan.
4. Jika salah satu metode resolusi cocok:
   - Badge status berubah dari **Pending** (kuning) menjadi **Verified** (hijau).
   - Rute reverse proxy Caddy didaftarkan secara otomatis ke internal port deployment aplikasi.
   - Domain siap diakses via protokol HTTPS.

### Menghapus Domain (Unbind & SSL Route Teardown)
1. Pada modal Custom Domains, klik ikon tempat sampah (🗑️) pada domain yang ingin dihapus.
2. Konfirmasi penghapusan:
   - Caddy Router mencabut konfigurasi reverse proxy tanpa downtime.
   - Record dihapus dari database.
   - Endpoint ask secara instan menolak permintaan sertifikat baru untuk domain tersebut.

---

## 7. Troubleshooting Matrix & Kendala Umum

| Gejala / Error | Kemungkinan Penyebab | Tindakan Solusi |
| --- | --- | --- |
| **Status DNS tetap `Pending` atau `Failed` setelah klik Verify** | Propagasi DNS publik belum selesai di resolver lokal | Tunggu 3–5 menit. Pastikan TTL di DNS provider diatur rendah (300 detik). Cek propagasi publik via `dig CNAME app.domain.com @8.8.8.8` atau tool online seperti `dnschecker.org`. |
| **Error: `CNAME target is not an authorized canonical host`** | Target CNAME salah ketik atau mengarah ke host lain | Pastikan target CNAME diarahkan tepat ke `cuan.dojo.cc` (atau hostname Cuan Anda), bukan ke IP atau domain lain. |
| **Error: `TXT records found but verification token did not match`** | Nilai token di TXT record tidak persis sama dengan yang ada di UI | Salin ulang token dari Studio Dashboard menggunakan tombol **Copy Token** (1-click copy). Pastikan nama record adalah `_cuan-challenge.<subdomain>`. |
| **Browser menampilkan `SSL_ERROR_NO_CYPHER_OVERLAP` atau `ERR_SSL_VERSION_OR_CIPHER_MISMATCH`** | Handshake On-Demand TLS gagal atau domain belum verified | Pastikan domain sudah berstatus **Verified** di Studio. Cek apakah endpoint `/api/caddy/ask?domain=...` merespons status `200 OK`. Periksa log Caddy untuk memastikan tidak ada rate-limit dari Let's Encrypt. |
| **Koneksi timeout saat Cloudflare aktif** | Status Proxy Cloudflare (awan oranye) mencegat challenge ACME | Ubah status proxy Cloudflare menjadi **DNS Only** (awan abu-abu). Alternatif: Jika ingin tetap menggunakan awan oranye, atur SSL/TLS mode di Cloudflare menjadi **Full (Strict)** setelah On-Demand TLS Caddy aktif. |
| **Sertifikat Let's Encrypt ditolak karena CAA Record** | Domain memiliki DNS CAA record yang hanya mengizinkan CA tertentu (e.g. DigiCert) | Tambahkan CAA record yang mengizinkan Let's Encrypt dan ZeroSSL: `0 issue "letsencrypt.org"` dan `0 issue "zerossl.com"`. |
| **HTTP 503 Service Unavailable saat membuka domain** | Internal runner aplikasi sedang berhenti atau port crash | Buka Studio Dashboard, periksa status deployment aplikasi. Jika status stopped, klik **Restart** atau **Start**. |

---

## 8. FAQ (Frequently Asked Questions)

**Q: Berapa banyak custom domain yang dapat dihubungkan ke satu deployment?**
A: Anda dapat menghubungkan beberapa custom domain ke satu deployment aplikasi (misalnya domain utama `brand.com` dan alias `www.brand.com` serta `app.brand.com`). Masing-masing domain memiliki siklus verifikasi dan On-Demand TLS mandiri.

**Q: Apakah Cuan melayani pembelian atau pendaftaran domain baru?**
A: Tidak. Cuan mengadopsi model **Bring Your Own Domain (BYOD)** murni. Pengguna bebas membeli domain dari registrar mana saja (Namecheap, Niagahoster, GoDaddy, Cloudflare Registrar, dll.) dan menghubungkannya ke Cuan.

**Q: Apakah On-Demand TLS memerlukan pembukaan port khusus?**
A: Caddy reverse proxy menggunakan port standar `80` (HTTP untuk ACME challenge dan redirect HTTPS) dan port `443` (HTTPS TLS termination). Pastikan firewall VPS Anda membuka port 80 dan 443 TCP ke publik.

**Q: Apakah data Sprint 1–4 terdampak oleh fitur Custom Domain ini?**
A: Sama sekali tidak. Migrasi PostgreSQL `000010_custom_domains` bersifat 100% aditif. Data akun owner, task board, sponsorship leads, order pembayaran, local deployments, dan server BYOS tersimpan utuh dan beroperasi normal.
