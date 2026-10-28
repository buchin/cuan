# Laporan Verifikasi Faktual Ketersediaan GitHub Organization dan Domain Cuan

**Tanggal Pemeriksaan:** 28 Oktober 2026  
**Metode:** Automated Live Verification via GitHub API (HTTPS GET), Web Endpoint, ICANN Registry RDAP, dan DNS Query (Dig/Socket).

---

## 1. Candidate GitHub Organization

Pemeriksaan dilakukan secara live terhadap endpoint resmi GitHub REST API (`https://api.github.com/users/<candidate>`) dan web status (`https://github.com/<candidate>`). Di GitHub, namespace user dan organization berada di lingkup yang sama. Jika endpoint mengembalikan HTTP 200 (baik user maupun org), nama tersebut tidak dapat didaftarkan sebagai organisasi baru.

| No | Candidate GitHub Org | API Status | Web Status | Tipe Akun Terdaftar | Waktu Pendaftaran (Created At) | Ketersediaan (Status) |
|---|---|---|---|---|---|---|
| 1 | `cuanstudio` | 200 OK | 200 OK | User (`/CuanStudio`) | 2026-08-25T18:42:06Z | ❌ **TAKEN** |
| 2 | `cuan-studio` | 404 Not Found | 404 Not Found | - | - | ✅ **AVAILABLE** |
| 3 | `cuanlabs` | 200 OK | 200 OK | User (`/CuanLabs`) | 2026-08-15T11:17:23Z | ❌ **TAKEN** |
| 4 | `cuan-labs` | 404 Not Found | 404 Not Found | - | - | ✅ **AVAILABLE** |
| 5 | `cuanwork` | 200 OK | 200 OK | User (`/cuanwork`) | 2017-11-06T22:37:52Z | ❌ **TAKEN** |
| 6 | `cuan-work` | 404 Not Found | 404 Not Found | - | - | ✅ **AVAILABLE** |
| 7 | `cuanengine` | 404 Not Found | 404 Not Found | - | - | ✅ **AVAILABLE** |
| 8 | `cuan-engine` | 404 Not Found | 404 Not Found | - | - | ✅ **AVAILABLE** |

### Ringkasan GitHub Organization:
- **Tersedia (AVAILABLE):** `cuan-studio`, `cuan-labs`, `cuan-work`, `cuanengine`, `cuan-engine`.
- **Tidak Tersedia (TAKEN):** `cuanstudio`, `cuanlabs`, `cuanwork` (ketiganya telah terdaftar sebagai profil user perorangan).

---

## 2. Candidate Domain Utama

Pemeriksaan dilakukan melalui protokol resmi ICANN RDAP (*Registration Data Access Protocol*) dari masing-masing authoritative registry TLD (`pubapi.registry.google`, `rdap.identitydigital.services`, `rdap.nic.work`), serta live DNS query (A record dan NS record).

| No | Candidate Domain | RDAP Endpoint Status | DNS Status | Registrar / Registry Detail | Masa Aktif / Expired | Ketersediaan (Status) |
|---|---|---|---|---|---|---|
| 1 | `cuanstudio.dev` | 404 Not Found | NXDOMAIN (No DNS) | Google Registry (`.dev`) | - | ✅ **AVAILABLE** |
| 2 | `cuan.studio` | 200 OK | ACTIVE (Parking) | HOSTINGER operations, UAB | Exp: 2027-08-25 (Reg: 2026-08-25) | ❌ **TAKEN** |
| 3 | `cuan.work` | 404 Not Found | NXDOMAIN (No DNS) | Registry NIC `.work` | - | ✅ **AVAILABLE** |
| 4 | `cuanlabs.dev` | 404 Not Found | NXDOMAIN (No DNS) | Google Registry (`.dev`) | - | ✅ **AVAILABLE** |

### Detail Teknis Domain Terdaftar (`cuan.studio`):
- **Registrar:** HOSTINGER operations, UAB
- **Nameservers:** `atlas.dns-parking.com.`, `hyperion.dns-parking.com.`
- **A Records:** `91.108.119.40`, `88.223.91.212` (Parking page)
- **Status Flags:** `client transfer prohibited`

---

## 3. Rekomendasi Branding & Pairing

Berdasarkan hasil faktual live di atas, pasangan nama organisasi GitHub dan nama domain yang **keduanya tersedia dan sinkron** adalah:
1. **Pilihan Utama 1:** GitHub Org `cuan-studio` + Domain `cuanstudio.dev`
2. **Pilihan Utama 2:** GitHub Org `cuan-labs` + Domain `cuanlabs.dev`
3. **Pilihan Alternatif:** GitHub Org `cuan-work` + Domain `cuan.work`
4. **Pilihan Clean (Org Tanpa Tanda Hubung):** GitHub Org `cuanengine` (tersedia secara utuh).
