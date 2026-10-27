# Cuan Database Backup, Restore, and VPS Migration Guide

## 1. Overview

Cuan provides built-in, production-tested bash scripts to automate database backups and restorations with cryptographic integrity checks:
- [`scripts/backup.sh`](file:///home/dev/repos/cuan/scripts/backup.sh)
- [`scripts/restore.sh`](file:///home/dev/repos/cuan/scripts/restore.sh)

These tools eliminate the need for manual, error-prone `pg_dump` and `pg_restore` commands and ensure safe database migration between host environments.

---

## 2. Backup Tool (`scripts/backup.sh`)

### Capabilities
- **Connection Flexibility**: Connects via `DATABASE_URL`, direct host `pg_dump`, or auto-detected Docker container (`cuan-db`).
- **Compression**: Streams and compresses PostgreSQL dumps directly to `.sql.gz`.
- **Integrity Guarantee**: Automatically computes a SHA-256 checksum stored in `<backup>.sha256`.
- **Structured Metadata**: Generates a companion `<backup>.meta.json` recording timestamp, tags, table inventory, file size, and status.
- **Automated Retention**: Keeps the latest $N$ backups (default: 7) and automatically rotates out older archives.

### Usage Examples
```bash
# Standard backup (uses DATABASE_URL or auto-detected Docker DB):
./scripts/backup.sh

# Tagged pre-migration backup:
./scripts/backup.sh --tag vps-migration

# Custom connection URL:
./scripts/backup.sh --db-url "postgres://cuan:secret@localhost:5432/cuan?sslmode=disable"

# Retain last 14 backups:
./scripts/backup.sh --keep 14
```

Backup artifacts are stored in `backups/`:
- `cuan_backup_<timestamp>_<tag>.sql.gz` (compressed database snapshot)
- `cuan_backup_<timestamp>_<tag>.sql.gz.sha256` (SHA-256 hash)
- `cuan_backup_<timestamp>_<tag>.meta.json` (metadata manifest)

---

## 3. Restore Tool (`scripts/restore.sh`)

### Capabilities
- **Pre-flight Integrity Check**: Verifies the SHA-256 checksum and tests gzip stream integrity before touching the database.
- **Safety Gate**: Requires interactive confirmation by default (can be bypassed with `--force` or `-f` for automated pipelines).
- **Auto-detection**: Automatically finds the latest valid backup archive in `backups/` when `--latest` is specified.
- **Post-restore Verification**: Validates database schema migrations and core entity tables post-execution.

### Usage Examples
```bash
# Restore latest backup interactively:
./scripts/restore.sh --latest

# Restore specific backup file non-interactively:
./scripts/restore.sh --file backups/cuan_backup_20261026_120000Z_vps-migration.sql.gz --force

# Restore to specific database target:
./scripts/restore.sh --file backups/cuan_backup_latest.sql.gz --db-url "$DATABASE_URL" --force
```

---

## 4. End-to-End VPS Migration Procedure

Moving a Cuan installation from an old VPS to a new VPS involves three primary components:
1. **PostgreSQL Database**: Handled via `backup.sh` and `restore.sh`.
2. **Persistent Storage & File Attachments**:
   - `/opt/cuan/data/` (project workspaces and task repositories)
   - `/home/ubuntu/.local/share/cuan/` (user attachments, agent tokens, and chat transcripts)
3. **Environment & Service Configuration**:
   - `/opt/cuan/.env` (database credentials, application secrets, ports)
   - Systemd service (`/etc/systemd/system/cuan.service`)
   - Reverse proxy (Nginx or Caddy)

---

### Step-by-Step Migration Checklist

#### Phase A: VPS Lama (Source)
1. **Buat Snapshot Database Terbaru**:
   ```bash
   cd /opt/cuan/current
   ./scripts/backup.sh --tag migration-final
   ```

2. **Hentikan Sementara Service (Opsional, untuk mencegah delta data)**:
   ```bash
   sudo systemctl stop cuan
   ```

#### Phase B: Salin Data ke VPS Baru (Target)
Gunakan `rsync` untuk memindahkan database backup dan persistent data:
```bash
# 1. Salin file backup
rsync -avz /opt/cuan/current/backups/ ubuntu@<IP_VPS_BARU>:/opt/cuan/backups/

# 2. Salin workspace proyek & repository data
rsync -avz /opt/cuan/data/ ubuntu@<IP_VPS_BARU>:/opt/cuan/data/

# 3. Salin attachments, chat logs, dan konfigurasi akun provider
rsync -avz /home/ubuntu/.local/share/cuan/ ubuntu@<IP_VPS_BARU>:/home/ubuntu/.local/share/cuan/

# 4. Salin environment configuration
rsync -avz /opt/cuan/.env ubuntu@<IP_VPS_BARU>:/opt/cuan/.env
```

#### Phase C: VPS Baru (Target)
1. **Setup Prasyarat di VPS Baru**:
   - Pasang PostgreSQL, Git, CLI agent (`agy` di `/home/ubuntu/.local/bin/agy`).
   - Buat user & database PostgreSQL:
     ```bash
     sudo -u postgres psql -c "CREATE USER cuan WITH PASSWORD 'password_rahasia';"
     sudo -u postgres psql -c "CREATE DATABASE cuan OWNER cuan;"
     ```

2. **Restore Database via Script**:
   ```bash
   cd /opt/cuan/current
   ./scripts/restore.sh --latest --force
   ```

3. **Verifikasi Izin File (Permissions)**:
   ```bash
   sudo chown -R ubuntu:ubuntu /opt/cuan /home/ubuntu/.local/share/cuan
   ```

4. **Start Service & Health Check**:
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable --now cuan
   
   # Cek liveness dan readiness
   curl -s http://localhost:8080/healthz
   curl -s http://localhost:8080/readyz
   ```

5. **Switch DNS**:
   - Arahkan DNS A record domain (misalnya `cuan.dojo.cc`) ke alamat IP VPS baru.
