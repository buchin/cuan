# Panduan Reverse Proxy HTTPS & Isolasi Jaringan Cuan

Panduan arsitektur dan operasional untuk mengonfigurasi reverse proxy HTTPS, alokasi port dan domain, serta memastikan database PostgreSQL tidak diekspos ke publik pada Cuan (Sprint 1, Task T2).

---

## 1. Arsitektur Jaringan & Threat Boundaries

Cuan dirancang dengan prinsip *defense-in-depth* dan isolasi jaringan yang ketat:

```d2
direction: right

client: "Browser / Client\n(Public Internet)"

network: "Host / Docker Bridge Network (cuan-network)" {
  proxy: "Reverse Proxy\n(Caddy / Existing Nginx)\nPort 80 / 443"
  app: "Cuan App Server\n(Go API + React SPA)\nPort 8080"
  db: "PostgreSQL Database\n(Persistent Storage)\nPort 5432 (Internal Only)"
}

client -> proxy: "HTTPS (TLS 1.3 / Port 443)"
proxy -> app: "HTTP Reverse Proxy (:8080)"
app -> db: "SQL Connection (:5432 internal)"
```

### Prinsip Keamanan Jaringan:
1. **Database PostgreSQL Terisolasi (Port 5432)**:
   - Port 5432 **hanya** diekspos ke jaringan internal container Docker (`cuan-network`).
   - `docker-compose.yml` menggunakan `expose: ["5432"]` dan **tidak** menggunakan `ports: ["5432:5432"]`.
   - Database tidak dapat diakses secara langsung dari IP publik host maupun dari jaringan luar.
2. **Reverse Proxy Terminasi TLS (Port 80 / 443)**:
   - Akses dari luar hanya diizinkan melalui reverse proxy HTTPS.
   - Menggunakan sertifikat TLS otomatis dari Let's Encrypt / ZeroSSL via ACME (Caddy) atau sertifikat terkelola.
3. **Application Server (Port 8080)**:
   - Menjalankan backend Go dan melayani berkas statis frontend React SPA.
   - Header proxy seperti `X-Forwarded-For` dan `X-Forwarded-Proto` diproses untuk keamanan cookie sesi (`HttpOnly`, `SameSite=Lax`, dan `Secure` flag saat HTTPS aktif).

---

## 2. Mode Pemasangan Reverse Proxy

Cuan mendukung dua mode pemasangan utama:

### Mode A: Caddy Otomatis (Standalone VPS Baru)

Gunakan mode ini jika Anda menginstal Cuan di VPS mandiri yang belum memiliki reverse proxy lain.

1. **Konfigurasi Domain dan Email**:
   Edit file `.env`:
   ```bash
   CUAN_DOMAIN=cuan.example.com
   ACME_EMAIL=admin@example.com
   ```

2. **Pastikan Port 80 dan 443 Tersedia**:
   Pastikan DNS A Record `cuan.example.com` mengarah ke IP publik VPS Anda, serta firewall mengizinkan port 80 dan 443:
   ```bash
   sudo ufw allow 80/tcp
   sudo ufw allow 443/tcp
   ```

3. **Jalankan dengan Profil Proxy**:
   ```bash
   docker compose --profile proxy up -d
   ```
   Caddy akan otomatis memesan sertifikat Let's Encrypt, memperbarui secara berkala, dan mengarahkan seluruh lalu lintas HTTP (port 80) ke HTTPS (port 443).

---

### Mode B: Integrasi Reverse Proxy Existing (Nginx / Traefik / Cloudflare)

Gunakan mode ini jika VPS Anda sudah menjalankan Nginx, Traefik, Apache, atau Cloudflare Tunnel.

1. **Jalankan Cuan tanpa Caddy container**:
   ```bash
   docker compose up -d
   ```
   Layanan `app` akan mendengarkan di `http://127.0.0.1:8080`.

2. **Konfigurasi Nginx Host**:
   Tambahkan konfigurasi virtual host Nginx (misalnya `/etc/nginx/sites-available/cuan.conf`):

   ```nginx
   server {
       listen 80;
       server_name cuan.example.com;
       return 301 https://$host$request_uri;
   }

   server {
       listen 443 ssl http2;
       server_name cuan.example.com;

       ssl_certificate /etc/letsencrypt/live/cuan.example.com/fullchain.pem;
       ssl_certificate_key /etc/letsencrypt/live/cuan.example.com/privkey.pem;
       ssl_protocols TLSv1.2 TLSv1.3;
       ssl_ciphers HIGH:!aNULL:!MD5;

       # Security Headers
       add_header X-Content-Type-Options "nosniff" always;
       add_header X-Frame-Options "DENY" always;
       add_header Referrer-Policy "strict-origin-when-cross-origin" always;
       add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;

       location / {
           proxy_pass http://127.0.0.1:8080;
           proxy_http_version 1.1;
           proxy_set_header Host $host;
           proxy_set_header X-Real-IP $remote_addr;
           proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
           proxy_set_header X-Forwarded-Proto $scheme;

           # WebSocket / SSE streaming support
           proxy_set_header Upgrade $http_upgrade;
           proxy_set_header Connection "upgrade";
           proxy_read_timeout 86400s;
       }
   }
   ```

3. **Uji dan Terapkan Konfigurasi**:
   ```bash
   sudo nginx -t && sudo systemctl reload nginx
   ```

---

## 3. Matriks Alokasi Port & Domain

| Port | Protokol | Visibilitas | Komponen | Deskripsi |
| :--- | :--- | :--- | :--- | :--- |
| **80** | HTTP | Publik | Caddy / Nginx | Redirect otomatis ke HTTPS (port 443) |
| **443** | HTTPS | Publik | Caddy / Nginx | Titik masuk utama terenkripsi TLS 1.3 |
| **8080** | HTTP | Lokal / Host | Cuan App Container | Backend Go API + Web SPA Shell |
| **5432** | PostgreSQL | **Internal Only** | DB Container | Penyimpanan persisten (dilarang publik) |

---

## 4. Perlindungan Autentikasi Pemilik Tunggal

Cuan menerapkan single-owner lockdown:
- Akun pertama dibuat melalui endpoint `/api/v1/auth/setup`.
- Begitu akun dibuat, flag `is_setup` disetel `true` dan database mengunci registrasi.
- Setiap percobaan setup berikutnya ditolak seketika dengan status **403 Forbidden**.
- Cookie sesi `cuan_session` menggunakan atribut `HttpOnly`, `SameSite=Lax`, dan `Path=/` sehingga aman dari pencurian melalui XSS browser.
