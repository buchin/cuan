#!/usr/bin/env python3
"""Automated one-step host migration tool for Cuan.

Migrates a running Cuan instance from a source VPS to a target VPS:
1. Pre-flight connectivity, specs, and environment checks
2. Target OS package provisioning (PostgreSQL, Nginx, Node, Go, Bubblewrap, etc.)
3. Target database user & pgcrypto extension setup
4. Streamed live database dump & restore (pg_dump | psql)
5. Application directories, .env, and maintenance synchronization
6. Agent runtime tools sync (agy, gemini runtime, chat browser, playwright)
7. Systemd service units transfer & RAM-scaled limits.conf tuning
8. Nginx reverse proxy & SSL certificates transfer
9. Release deployment / activation & health verification
10. Optional Cloudflare DNS record update and cache purge (--switch-dns)
11. Optional source host decommissioning (--stop-source)
"""

import argparse
import json
import os
from pathlib import Path
import re
import shlex
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request


def log(msg, stage=None):
    if stage:
        print(f"\n::cuan-migrate-stage::{stage}")
    print(f"==> {msg}", flush=True)


def log_success(msg):
    print(f"✓ {msg}", flush=True)


def log_warn(msg):
    print(f"⚠️  WARNING: {msg}", flush=True)


def log_error(msg):
    print(f"❌ ERROR: {msg}", file=sys.stderr, flush=True)


def load_env_file(path):
    """Load key-value pairs from an .env file."""
    env = {}
    p = Path(path)
    if not p.is_file():
        return env
    for line in p.read_text().splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        if "=" in line:
            k, v = line.split("=", 1)
            env[k.strip()] = v.strip().strip('"').strip("'")
    return env


def parse_database_url(url):
    """Parse a PostgreSQL connection string into components."""
    if not url:
        return {"user": "cuan", "password": "", "host": "localhost", "port": 5432, "dbname": "cuan"}
    parsed = urllib.parse.urlparse(url)
    user = urllib.parse.unquote(parsed.username or "cuan")
    password = urllib.parse.unquote(parsed.password or "")
    host = parsed.hostname or "localhost"
    port = parsed.port or 5432
    dbname = parsed.path.lstrip("/") or "cuan"
    return {
        "user": user,
        "password": password,
        "host": host,
        "port": port,
        "dbname": dbname,
    }


def calculate_memory_limits(total_ram_mb):
    """Calculate systemd resource limits based on total host RAM."""
    if total_ram_mb >= 7000:
        max_mb = int(total_ram_mb * 0.5)
        high_mb = int(max_mb * 0.85)
    elif total_ram_mb >= 3500:
        max_mb = int(total_ram_mb * 0.5)
        high_mb = int(max_mb * 0.85)
    elif total_ram_mb >= 1800:
        max_mb = 1000
        high_mb = 850
    else:
        max_mb = 700
        high_mb = 550
    return {
        "MemoryMax": f"{max_mb}M",
        "MemoryHigh": f"{high_mb}M",
        "TasksMax": "8192",
    }


class SSHRunner:
    """Manages remote command execution and streaming via SSH."""

    def __init__(self, host, user="ubuntu", password=None, key_path=None, port=22, name="host"):
        self.host = host
        self.user = user
        self.password = password
        self.key_path = key_path
        self.port = port
        self.name = name

    def _base_cmd(self):
        cmd = []
        if self.password:
            cmd.extend(["sshpass", "-p", self.password])
        cmd.extend([
            "ssh",
            "-o", "StrictHostKeyChecking=no",
            "-o", "UserKnownHostsFile=/dev/null",
            "-o", "LogLevel=ERROR",
            "-o", "ConnectTimeout=15",
            "-p", str(self.port),
        ])
        if self.key_path:
            cmd.extend(["-i", str(self.key_path)])
        cmd.append(f"{self.user}@{self.host}")
        return cmd

    def run(self, remote_cmd, sudo=False, timeout=None, check=True, verbose=False):
        """Execute a remote command over SSH."""
        if sudo:
            if self.password:
                escaped_pwd = shlex.quote(self.password)
                full_cmd = f"echo {escaped_pwd} | sudo -S -p '' bash -c {shlex.quote(remote_cmd)}"
            else:
                full_cmd = f"sudo -n bash -c {shlex.quote(remote_cmd)}"
        else:
            full_cmd = remote_cmd

        args = self._base_cmd() + [full_cmd]
        if verbose:
            print(f"[{self.name}] $ {remote_cmd}")

        res = subprocess.run(
            args,
            capture_output=True,
            text=True,
            timeout=timeout,
        )
        if check and res.returncode != 0:
            err_msg = res.stderr.strip() or res.stdout.strip()
            raise RuntimeError(f"[{self.name}] Command failed ({res.returncode}): {remote_cmd}\nError: {err_msg}")
        return res.stdout.strip(), res.stderr.strip(), res.returncode

    def test_connection(self):
        """Verify SSH connectivity and sudo capability."""
        stdout, _, rc = self.run("id -un", check=False)
        if rc != 0 or not stdout:
            raise RuntimeError(f"[{self.name}] Failed to establish SSH connection to {self.user}@{self.host}")
        # Test sudo
        _, _, sudo_rc = self.run("true", sudo=True, check=False)
        if sudo_rc != 0:
            raise RuntimeError(f"[{self.name}] User {self.user} does not have required sudo privileges on {self.host}")
        return stdout

    def get_specs(self):
        """Retrieve total RAM, CPU count, and disk space."""
        mem_out, _, _ = self.run("grep MemTotal /proc/meminfo", check=False)
        mem_mb = 4096
        if mem_out and "MemTotal:" in mem_out:
            parts = mem_out.split()
            if len(parts) >= 2 and parts[1].isdigit():
                mem_mb = int(parts[1]) // 1024

        cpu_out, _, _ = self.run("nproc", check=False)
        cpus = int(cpu_out) if cpu_out.isdigit() else 2

        df_out, _, _ = self.run("df -h / | tail -n 1 | awk '{print $4}'", check=False)
        free_disk = df_out or "unknown"
        return {"ram_mb": mem_mb, "cpus": cpus, "free_disk": free_disk}

    def pipe_to_remote(self, src_cmd, dst_runner, dst_cmd, sudo_src=False, sudo_dst=False, timeout=None):
        """Stream output from src_cmd over SSH directly into dst_cmd on dst_runner."""
        if sudo_src:
            if self.password:
                src_full = f"echo {shlex.quote(self.password)} | sudo -S -p '' bash -c {shlex.quote(src_cmd)}"
            else:
                src_full = f"sudo -n bash -c {shlex.quote(src_cmd)}"
        else:
            src_full = src_cmd

        if sudo_dst:
            if dst_runner.password:
                dst_full = f"echo {shlex.quote(dst_runner.password)} | sudo -S -p '' bash -c {shlex.quote(dst_cmd)}"
            else:
                dst_full = f"sudo -n bash -c {shlex.quote(dst_cmd)}"
        else:
            dst_full = dst_cmd

        src_args = self._base_cmd() + [src_full]
        dst_args = dst_runner._base_cmd() + [dst_full]

        p_src = subprocess.Popen(src_args, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        p_dst = subprocess.Popen(dst_args, stdin=p_src.stdout, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        p_src.stdout.close()

        dst_stdout, dst_stderr = p_dst.communicate(timeout=timeout)
        src_stderr = p_src.stderr.read()
        p_src.wait()

        if p_src.returncode != 0:
            raise RuntimeError(f"Source stream failed ({p_src.returncode}): {src_cmd}\nError: {src_stderr.decode('utf-8', 'replace').strip()}")
        if p_dst.returncode != 0:
            raise RuntimeError(f"Target stream consumer failed ({p_dst.returncode}): {dst_cmd}\nError: {dst_stderr.decode('utf-8', 'replace').strip()}")

        return dst_stdout.decode("utf-8", "replace").strip()


class CloudflareDNS:
    """Manages Cloudflare DNS updates and cache purges."""

    def __init__(self, api_token):
        self.api_token = api_token
        self.base_url = "https://api.cloudflare.com/client/v4"
        self.headers = {
            "Authorization": f"Bearer {self.api_token}",
            "Content-Type": "application/json",
        }

    def _request(self, endpoint, method="GET", body=None):
        url = f"{self.base_url}{endpoint}"
        data = json.dumps(body).encode("utf-8") if body else None
        req = urllib.request.Request(url, data=data, headers=self.headers, method=method)
        try:
            with urllib.request.urlopen(req, timeout=15) as resp:
                return json.load(resp)
        except urllib.error.HTTPError as e:
            err_body = e.read().decode("utf-8", "replace")
            raise RuntimeError(f"Cloudflare API HTTP {e.code}: {err_body}")

    def list_zones(self):
        res = self._request("/zones?per_page=50")
        return res.get("result", [])

    def find_zone_for_domain(self, domain):
        zones = self.list_zones()
        matching = []
        for z in zones:
            zname = z["name"]
            if domain == zname or domain.endswith("." + zname):
                matching.append(z)
        if not matching:
            raise RuntimeError(f"No matching Cloudflare zone found for {domain}")
        matching.sort(key=lambda x: len(x["name"]), reverse=True)
        return matching[0]

    def update_a_record(self, domain, ip):
        zone = self.find_zone_for_domain(domain)
        zone_id = zone["id"]
        records = self._request(f"/zones/{zone_id}/dns_records?type=A&name={domain}").get("result", [])
        if records:
            rec = records[0]
            rec_id = rec["id"]
            if rec.get("content") == ip:
                return {"status": "unchanged", "domain": domain, "ip": ip, "zone_id": zone_id}
            self._request(f"/zones/{zone_id}/dns_records/{rec_id}", method="PATCH", body={"content": ip})
            return {"status": "updated", "domain": domain, "ip": ip, "zone_id": zone_id}
        else:
            self._request(
                f"/zones/{zone_id}/dns_records",
                method="POST",
                body={"type": "A", "name": domain, "content": ip, "proxied": True, "ttl": 1},
            )
            return {"status": "created", "domain": domain, "ip": ip, "zone_id": zone_id}

    def purge_cache(self, zone_id):
        return self._request(f"/zones/{zone_id}/purge_cache", method="POST", body={"purge_everything": True})


class HostMigrator:
    """Orchestrates end-to-end host migration."""

    def __init__(self, src_runner, dst_runner, options):
        self.src = src_runner
        self.dst = dst_runner
        self.opts = options
        self.db_info = {}
        self.target_specs = {}

    def run_all(self):
        log("Initiating Cuan Host Migration", stage="preflight")
        self.step_preflight()

        if self.opts.dry_run:
            log("DRY-RUN mode enabled. Pre-flight complete. No mutating steps will be executed.")
            return

        if not self.opts.skip_packages:
            self.step_packages()

        if not self.opts.skip_db:
            self.step_database()

        if not self.opts.skip_data:
            self.step_data_sync()

        if not self.opts.skip_runtime:
            self.step_runtime_sync()

        if not self.opts.skip_systemd:
            self.step_systemd()

        if not self.opts.skip_nginx:
            self.step_nginx()

        if not self.opts.skip_deploy:
            self.step_deploy_and_verify()

        if self.opts.switch_dns:
            self.step_switch_dns()

        if self.opts.stop_source:
            self.step_stop_source()

        log("Host migration completed successfully!", stage="completed")

    def step_preflight(self):
        log("Validating SSH connectivity and sudo privileges...")
        src_user = self.src.test_connection()
        dst_user = self.dst.test_connection()
        log_success(f"Source connected ({src_user}@{self.src.host})")
        log_success(f"Target connected ({dst_user}@{self.dst.host})")

        self.target_specs = self.dst.get_specs()
        log(f"Target host specs: {self.target_specs['ram_mb']} MB RAM, {self.target_specs['cpus']} vCPUs, {self.target_specs['free_disk']} free disk")

        # Inspect source environment
        log("Reading source Cuan configuration...")
        src_env_str, _, rc = self.src.run("cat /opt/cuan/.env 2>/dev/null || true", check=False)
        if rc == 0 and src_env_str:
            lines = src_env_str.splitlines()
            for line in lines:
                if line.startswith("DATABASE_URL="):
                    db_url = line.split("=", 1)[1].strip().strip('"').strip("'")
                    self.db_info = parse_database_url(db_url)
                    break
        if not self.db_info:
            self.db_info = {"user": "cuan", "password": "", "host": "localhost", "port": 5432, "dbname": "cuan"}
        log_success(f"Database target identified: db='{self.db_info['dbname']}', user='{self.db_info['user']}'")

    def step_packages(self):
        log("Provisioning target OS packages (PostgreSQL, Nginx, Node, Go, Bubblewrap)...", stage="packages")
        packages = (
            "postgresql postgresql-contrib nginx nodejs npm golang-go "
            "bubblewrap jq rsync curl git sshpass tar gzip"
        )
        self.dst.run(
            f"DEBIAN_FRONTEND=noninteractive apt-get update -y && "
            f"DEBIAN_FRONTEND=noninteractive apt-get install -y {packages}",
            sudo=True,
            timeout=600,
        )
        self.dst.run("systemctl enable --now postgresql nginx", sudo=True)
        log_success("Target OS packages installed and services enabled.")

    def step_database(self):
        log("Preparing target PostgreSQL database and user...", stage="database")
        db_user = self.db_info.get("user") or "cuan"
        db_pass = self.db_info.get("password") or ""
        db_name = self.db_info.get("dbname") or "cuan"

        # Create or update user
        if db_pass:
            escaped_pass = db_pass.replace("'", "''")
            user_sql = (
                f"DO $$ BEGIN "
                f"IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = '{db_user}') THEN "
                f"CREATE ROLE {db_user} WITH LOGIN PASSWORD '{escaped_pass}'; "
                f"ELSE "
                f"ALTER ROLE {db_user} WITH PASSWORD '{escaped_pass}'; "
                f"END IF; "
                f"END $$;"
            )
        else:
            user_sql = (
                f"DO $$ BEGIN "
                f"IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = '{db_user}') THEN "
                f"CREATE ROLE {db_user} WITH LOGIN; "
                f"END IF; "
                f"END $$;"
            )
        self.dst.run(f"sudo -u postgres psql -c \"{user_sql}\"", sudo=True)

        # Create database if missing
        db_check, _, _ = self.dst.run(
            f"sudo -u postgres psql -tAc \"SELECT 1 FROM pg_database WHERE datname='{db_name}'\"",
            sudo=True,
        )
        if "1" not in db_check:
            self.dst.run(f"sudo -u postgres psql -c \"CREATE DATABASE {db_name} OWNER {db_user};\"", sudo=True)

        # Enable pgcrypto extension
        self.dst.run(
            f"sudo -u postgres psql -d {db_name} -c \"CREATE EXTENSION IF NOT EXISTS pgcrypto;\"",
            sudo=True,
        )
        log_success(f"Target database '{db_name}' ready with pgcrypto extension.")

        log("Streaming database dump from source to target (live stream)...")
        src_dump_cmd = f"sudo -u postgres pg_dump -d {db_name} --clean --if-exists"
        dst_restore_cmd = f"sudo -u postgres psql -d {db_name}"
        self.src.pipe_to_remote(src_dump_cmd, self.dst, dst_restore_cmd, sudo_src=True, sudo_dst=True, timeout=900)

        # Verify restoration
        table_count, _, _ = self.dst.run(
            f"sudo -u postgres psql -d {db_name} -tAc \"SELECT count(*) FROM information_schema.tables WHERE table_schema='public';\"",
            sudo=True,
        )
        task_count, _, _ = self.dst.run(
            f"sudo -u postgres psql -d {db_name} -tAc \"SELECT count(*) FROM tasks;\" 2>/dev/null || echo 0",
            sudo=True,
        )
        log_success(f"Database restored successfully ({table_count} tables, {task_count} tasks).")

    def step_data_sync(self):
        log("Synchronizing application state, environment, and maintenance...", stage="data")
        # Ensure directories
        target_dirs = (
            "/opt/cuan/releases /opt/cuan/data /opt/cuan/maintenance/activation-policy "
            f"/home/{self.dst.user}/.local/share/cuan "
            f"/home/{self.dst.user}/.local/share/cuan-releases "
            f"/home/{self.dst.user}/.local/share/cuan-s25 "
            f"/home/{self.dst.user}/.local/bin"
        )
        self.dst.run(f"mkdir -p {target_dirs}", sudo=True)
        self.dst.run(f"chown -R {self.dst.user}:{self.dst.user} /opt/cuan /home/{self.dst.user}/.local", sudo=True)

        # Sync /opt/cuan/.env
        log("Transferring /opt/cuan/.env...")
        env_content, _, rc = self.src.run("cat /opt/cuan/.env 2>/dev/null", sudo=True, check=False)
        if rc == 0 and env_content:
            write_cmd = f"cat << 'EOF' > /opt/cuan/.env\n{env_content}\nEOF\nchmod 0600 /opt/cuan/.env && chown {self.dst.user}:{self.dst.user} /opt/cuan/.env"
            self.dst.run(write_cmd, sudo=True)
            log_success("Saved /opt/cuan/.env on target (mode 0600).")

        # Sync /opt/cuan/data (excluding unix sockets and git pack temporary buffers)
        log("Transferring /opt/cuan/data...")
        data_check, _, _ = self.src.run("test -d /opt/cuan/data && ls -A /opt/cuan/data 2>/dev/null || true", sudo=True)
        if data_check:
            self.src.pipe_to_remote(
                "tar -C /opt/cuan/data --exclude='*.sock' --exclude='tmp_pack*' -czf - .",
                self.dst,
                "tar -C /opt/cuan/data -xzf -",
                sudo_src=True,
                sudo_dst=True,
                timeout=900,
            )
            self.dst.run(f"chown -R {self.dst.user}:{self.dst.user} /opt/cuan/data", sudo=True)
            log_success("Synchronized /opt/cuan/data.")

        # Sync ~/.local/share/cuan (accounts & provider profiles)
        log("Transferring ~/.local/share/cuan...")
        share_check, _, _ = self.src.run(f"test -d /home/{self.src.user}/.local/share/cuan && ls -A /home/{self.src.user}/.local/share/cuan 2>/dev/null || true")
        if share_check:
            self.src.pipe_to_remote(
                f"tar -C /home/{self.src.user}/.local/share/cuan --exclude='conversations' --exclude='brain' --exclude='log' --exclude='*.db*' --exclude='.cache' --exclude='go' --exclude='.npm' -czf - .",
                self.dst,
                f"tar -C /home/{self.dst.user}/.local/share/cuan -xzf -",
                timeout=300,
            )
            log_success("Synchronized ~/.local/share/cuan.")

        # Sync /opt/cuan/maintenance
        log("Transferring /opt/cuan/maintenance...")
        maint_check, _, _ = self.src.run("test -d /opt/cuan/maintenance && ls -A /opt/cuan/maintenance 2>/dev/null || true", sudo=True)
        if maint_check:
            self.src.pipe_to_remote(
                "tar -C /opt/cuan/maintenance -czf - .",
                self.dst,
                "tar -C /opt/cuan/maintenance -xzf -",
                sudo_src=True,
                sudo_dst=True,
                timeout=300,
            )
            log_success("Synchronized /opt/cuan/maintenance.")

    def step_runtime_sync(self):
        log("Synchronizing agent runtime tools (agy, gemini, chat browser)...", stage="runtime")
        # 1. Antigravity CLI binary
        agy_check, _, rc = self.src.run(f"test -f /home/{self.src.user}/.local/bin/agy", check=False)
        if rc == 0:
            log("Transferring Antigravity CLI binary (~/.local/bin/agy)...")
            self.src.pipe_to_remote(
                f"cat /home/{self.src.user}/.local/bin/agy",
                self.dst,
                f"cat > /home/{self.dst.user}/.local/bin/agy && chmod +x /home/{self.dst.user}/.local/bin/agy",
                timeout=300,
            )
            log_success("Antigravity CLI transferred and made executable.")

        # 2. Antigravity runtime (~/.gemini)
        gemini_check, _, rc = self.src.run(f"test -d /home/{self.src.user}/.gemini", check=False)
        if rc == 0:
            log("Transferring Antigravity runtime (~/.gemini)...")
            self.dst.run(f"mkdir -p /home/{self.dst.user}/.gemini")
            self.src.pipe_to_remote(
                f"tar -C /home/{self.src.user}/.gemini -czf - .",
                self.dst,
                f"tar -C /home/{self.dst.user}/.gemini -xzf -",
                timeout=600,
            )
            log_success("Antigravity runtime synchronized.")

        # 3. Obscura chat browser binary (~/.local/bin/cuan-chat-browser)
        browser_check, _, rc = self.src.run(f"test -f /home/{self.src.user}/.local/bin/cuan-chat-browser", check=False)
        if rc == 0:
            log("Transferring cuan-chat-browser binary...")
            self.src.pipe_to_remote(
                f"cat /home/{self.src.user}/.local/bin/cuan-chat-browser",
                self.dst,
                f"cat > /home/{self.dst.user}/.local/bin/cuan-chat-browser && chmod +x /home/{self.dst.user}/.local/bin/cuan-chat-browser",
                timeout=300,
            )
            log_success("cuan-chat-browser transferred.")

        # 4. Playwright cache (~/.cache/ms-playwright)
        pw_check, _, rc = self.src.run(f"test -d /home/{self.src.user}/.cache/ms-playwright", check=False)
        if rc == 0:
            log("Transferring Playwright browser cache (~/.cache/ms-playwright)...")
            self.dst.run(f"mkdir -p /home/{self.dst.user}/.cache/ms-playwright")
            self.src.pipe_to_remote(
                f"tar -C /home/{self.src.user}/.cache/ms-playwright -czf - .",
                self.dst,
                f"tar -C /home/{self.dst.user}/.cache/ms-playwright -xzf -",
                timeout=600,
            )
            log_success("Playwright browser cache synchronized.")

        # Ensure PATH in ~/.bashrc
        self.dst.run(
            f"grep -q 'export PATH=\"$HOME/.local/bin:$PATH\"' /home/{self.dst.user}/.bashrc 2>/dev/null || "
            f"echo 'export PATH=\"$HOME/.local/bin:$PATH\"' >> /home/{self.dst.user}/.bashrc"
        )

    def step_systemd(self):
        log("Transferring systemd units and auto-tuning resource limits...", stage="systemd")
        # Copy systemd units
        self.src.pipe_to_remote(
            "tar -C /etc/systemd/system -czf - $(cd /etc/systemd/system && ls -d cuan* 2>/dev/null)",
            self.dst,
            "tar -C /etc/systemd/system -xzf -",
            sudo_src=True,
            sudo_dst=True,
            timeout=120,
        )

        # Scale limits.conf based on target RAM
        ram_mb = self.target_specs.get("ram_mb", 4096)
        limits = calculate_memory_limits(ram_mb)
        log(f"Auto-tuning limits.conf for {ram_mb} MB RAM: MemoryMax={limits['MemoryMax']}, MemoryHigh={limits['MemoryHigh']}")

        limits_content = (
            f"[Service]\n"
            f"MemoryMax={limits['MemoryMax']}\n"
            f"MemoryHigh={limits['MemoryHigh']}\n"
            f"TasksMax={limits['TasksMax']}\n"
        )

        for svc in ["cuan.service.d", "cuan-runner.service.d"]:
            self.dst.run(f"mkdir -p /etc/systemd/system/{svc}", sudo=True)
            self.dst.run(
                f"cat << 'EOF' > /etc/systemd/system/{svc}/limits.conf\n{limits_content}EOF",
                sudo=True,
            )

        self.dst.run("systemctl daemon-reload", sudo=True)
        log_success("Systemd units and auto-tuned limits.conf installed.")

    def step_nginx(self):
        log("Transferring SSL certificates and Nginx reverse proxy configurations...", stage="nginx")
        # 1. SSL certificates
        ssl_check, _, rc = self.src.run("test -d /etc/letsencrypt && ls -A /etc/letsencrypt 2>/dev/null || true", sudo=True)
        if ssl_check:
            self.dst.run("mkdir -p /etc/letsencrypt", sudo=True)
            self.src.pipe_to_remote(
                "tar -C /etc/letsencrypt -czf - .",
                self.dst,
                "tar -C /etc/letsencrypt -xzf -",
                sudo_src=True,
                sudo_dst=True,
                timeout=300,
            )
            log_success("SSL certificates transferred.")

        # 2. Nginx configuration
        self.src.pipe_to_remote(
            "tar -C /etc/nginx/sites-available -czf - $(cd /etc/nginx/sites-available && ls cuan* 2>/dev/null || true)",
            self.dst,
            "tar -C /etc/nginx/sites-available -xzf -",
            sudo_src=True,
            sudo_dst=True,
            timeout=120,
        )

        # Enable site and remove default
        self.dst.run("rm -f /etc/nginx/sites-enabled/default", sudo=True)
        # Find transferred cuan config
        cuan_conf_out, _, _ = self.dst.run("ls -1 /etc/nginx/sites-available/cuan* 2>/dev/null || true", sudo=True)
        for conf_path in cuan_conf_out.splitlines():
            conf_file = Path(conf_path.strip()).name
            if conf_file:
                self.dst.run(f"ln -sf /etc/nginx/sites-available/{conf_file} /etc/nginx/sites-enabled/{conf_file}", sudo=True)

        # Test and reload
        self.dst.run("nginx -t", sudo=True)
        self.dst.run("systemctl reload nginx", sudo=True)
        log_success("Nginx configured and reloaded.")

    def step_deploy_and_verify(self):
        log("Deploying release payload and activating services on target...", stage="deploy")
        deploy_script = Path(__file__).resolve().parent / "deploy.sh"

        deployed = False
        if deploy_script.is_file() and not self.opts.sync_release_from_source:
            log("Running ./scripts/deploy.sh targeting new host...")
            deploy_env = os.environ.copy()
            deploy_env["DOJO_TERMAPI_VPS_PUBLIC_IP"] = self.dst.host
            deploy_env["DOJO_TERMAPI_VPS_USERNAME"] = self.dst.user
            if self.dst.password:
                deploy_env["DOJO_TERMAPI_VPS_PASSWORD"] = self.dst.password
            if self.opts.deploy_branch:
                deploy_env["CUAN_DEPLOY_BRANCH"] = self.opts.deploy_branch

            res = subprocess.run([str(deploy_script)], env=deploy_env)
            if res.returncode == 0:
                deployed = True
            else:
                log_warn(f"deploy.sh exited with {res.returncode}. Falling back to direct release sync from source.")

        if not deployed:
            # Sync active release directly from source
            log("Syncing active release directly from source...")
            curr_link, _, rc = self.src.run("readlink -f /opt/cuan/current 2>/dev/null", sudo=True, check=False)
            if rc != 0 or not curr_link:
                raise RuntimeError("Could not determine current active release from source /opt/cuan/current")
            release_sha = Path(curr_link).name
            log(f"Source active release SHA: {release_sha}")

            self.dst.run(f"mkdir -p /opt/cuan/releases/{release_sha}", sudo=True)
            self.src.pipe_to_remote(
                f"tar -C /opt/cuan/releases/{release_sha} -czf - .",
                self.dst,
                f"tar -C /opt/cuan/releases/{release_sha} -xzf -",
                sudo_src=True,
                sudo_dst=True,
                timeout=600,
            )

            # Authorize release in authorized.json
            auth_py = (
                "import json, pathlib, sys; "
                "sha = sys.argv[1]; "
                "p = pathlib.Path('/opt/cuan/maintenance/activation-policy/authorized.json'); "
                "d = json.loads(p.read_text()) if p.exists() else {'releases': {}}; "
                "d.setdefault('releases', {})[sha] = {'authorized_at': 'migration'}; "
                "p.parent.mkdir(parents=True, exist_ok=True); "
                "p.write_text(json.dumps(d, indent=2) + '\\n')"
            )
            auth_update_cmd = f"python3 -c {shlex.quote(auth_py)} {shlex.quote(release_sha)}"
            self.dst.run(auth_update_cmd, sudo=True)

            # Activate release
            act_cmd = f"python3 /opt/cuan/maintenance/activation-policy/activate-release.py --release {release_sha}"
            self.dst.run(act_cmd, sudo=True, timeout=180)
            log_success(f"Release {release_sha} activated on target.")

        # Health verification
        log("Verifying target /healthz...")
        healthy = False
        for attempt in range(1, 25):
            h_out, _, rc = self.dst.run("curl -s http://127.0.0.1:8080/healthz", check=False)
            if rc == 0 and h_out:
                try:
                    data = json.loads(h_out)
                    if data.get("status") == "healthy":
                        log_success(f"Target healthz verified: {data}")
                        healthy = True
                        break
                except Exception:
                    pass
            time.sleep(2)

        if not healthy:
            raise RuntimeError("Target host failed healthz verification after activation.")

    def step_switch_dns(self):
        log("Switching Cloudflare DNS records to new host IP...", stage="dns")
        token = self.opts.cloudflare_token
        if not token:
            raise RuntimeError("Cloudflare API token required for --switch-dns (pass --cloudflare-token or set BACKUP_CF_API_TOKEN)")

        cf = CloudflareDNS(token)
        domains = [d.strip() for d in self.opts.domains.split(",") if d.strip()]
        purged_zones = set()

        for domain in domains:
            log(f"Updating DNS A record for '{domain}' -> {self.dst.host}...")
            res = cf.update_a_record(domain, self.dst.host)
            log_success(f"DNS {res['status']}: {domain} -> {self.dst.host}")
            purged_zones.add(res["zone_id"])

        for zone_id in purged_zones:
            log(f"Purging Cloudflare cache for zone {zone_id}...")
            cf.purge_cache(zone_id)
            log_success(f"Purged Cloudflare cache for zone {zone_id}.")

    def step_stop_source(self):
        log("Decommissioning source host services...", stage="decommission")
        log_warn(f"Stopping and disabling Cuan services on source host {self.src.host}...")
        self.src.run("systemctl stop cuan cuan-runner cuan-chat@*", sudo=True, check=False)
        self.src.run("systemctl disable cuan cuan-runner cuan-chat@*", sudo=True, check=False)
        log_success(f"Services stopped and disabled on source host {self.src.host}.")


def parse_args(argv=None):
    parser = argparse.ArgumentParser(
        description=__doc__,
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    env = load_env_file(".env")

    # Source Host Options
    src_group = parser.add_argument_group("Source Host")
    src_group.add_argument(
        "--from-host",
        default=os.environ.get("CUAN_SOURCE_VPS_IP") or env.get("DOJO_TERMAPI_VPS_PUBLIC_IP", "43.173.33.11"),
        help="Source VPS IP or hostname (default: from .env DOJO_TERMAPI_VPS_PUBLIC_IP or 43.173.33.11)",
    )
    src_group.add_argument(
        "--from-user",
        default=os.environ.get("CUAN_SOURCE_VPS_USER") or env.get("DOJO_TERMAPI_VPS_USERNAME", "ubuntu"),
        help="Source VPS SSH username (default: ubuntu)",
    )
    src_group.add_argument(
        "--from-password",
        default=os.environ.get("CUAN_SOURCE_VPS_PASSWORD") or env.get("DOJO_TERMAPI_VPS_PASSWORD"),
        help="Source VPS SSH password",
    )
    src_group.add_argument(
        "--from-key",
        default=None,
        help="Source VPS SSH private key path",
    )
    src_group.add_argument(
        "--from-port",
        type=int,
        default=22,
        help="Source VPS SSH port (default: 22)",
    )

    # Target Host Options
    dst_group = parser.add_argument_group("Target Host")
    dst_group.add_argument(
        "--to-host",
        default=os.environ.get("CUAN_TARGET_VPS_IP") or env.get("CUAN_NEW_VPS_IP"),
        required=not (os.environ.get("CUAN_TARGET_VPS_IP") or env.get("CUAN_NEW_VPS_IP")),
        help="Target VPS IP or hostname (required)",
    )
    dst_group.add_argument(
        "--to-user",
        default=os.environ.get("CUAN_TARGET_VPS_USER") or env.get("CUAN_NEW_VPS_USERNAME", "ubuntu"),
        help="Target VPS SSH username (default: ubuntu)",
    )
    dst_group.add_argument(
        "--to-password",
        default=os.environ.get("CUAN_TARGET_VPS_PASSWORD") or env.get("CUAN_NEW_VPS_PASSWORD"),
        help="Target VPS SSH password",
    )
    dst_group.add_argument(
        "--to-key",
        default=None,
        help="Target VPS SSH private key path",
    )
    dst_group.add_argument(
        "--to-port",
        type=int,
        default=22,
        help="Target VPS SSH port (default: 22)",
    )

    # Migration Flow Controls
    flow_group = parser.add_argument_group("Migration Flow Controls")
    flow_group.add_argument(
        "--deploy-branch",
        default="dev",
        help="Git branch to deploy on target (default: dev)",
    )
    flow_group.add_argument(
        "--sync-release-from-source",
        action="store_true",
        help="Skip local build and sync active release directly from source host",
    )
    flow_group.add_argument(
        "--switch-dns",
        action="store_true",
        help="Update Cloudflare DNS records to point to target host and purge cache",
    )
    flow_group.add_argument(
        "--cloudflare-token",
        default=os.environ.get("BACKUP_CF_API_TOKEN") or env.get("BACKUP_CF_API_TOKEN") or env.get("CF_API_TOKEN"),
        help="Cloudflare API token for DNS update",
    )
    flow_group.add_argument(
        "--domains",
        default="app.cuan.work,cuan-app.dojo.cc",
        help="Comma-separated domains for Cloudflare DNS update (default: app.cuan.work,cuan-app.dojo.cc)",
    )
    flow_group.add_argument(
        "--stop-source",
        action="store_true",
        help="Stop and disable Cuan services on source host after verified migration",
    )
    flow_group.add_argument(
        "--dry-run",
        action="store_true",
        help="Run pre-flight checks and display plan without executing mutating actions",
    )
    flow_group.add_argument(
        "--verbose",
        action="store_true",
        help="Enable verbose command tracing",
    )

    # Skip Flags
    skip_group = parser.add_argument_group("Skip Stages (Advanced)")
    skip_group.add_argument("--skip-packages", action="store_true", help="Skip target OS package installation")
    skip_group.add_argument("--skip-db", action="store_true", help="Skip database setup and streaming")
    skip_group.add_argument("--skip-data", action="store_true", help="Skip application data sync")
    skip_group.add_argument("--skip-runtime", action="store_true", help="Skip agent runtime tools sync")
    skip_group.add_argument("--skip-systemd", action="store_true", help="Skip systemd units sync and limit scaling")
    skip_group.add_argument("--skip-nginx", action="store_true", help="Skip Nginx and SSL certificate sync")
    skip_group.add_argument("--skip-deploy", action="store_true", help="Skip release deployment step")

    return parser.parse_args(argv)


def main():
    args = parse_args()

    src = SSHRunner(
        host=args.from_host,
        user=args.from_user,
        password=args.from_password,
        key_path=args.from_key,
        port=args.from_port,
        name="source",
    )

    dst = SSHRunner(
        host=args.to_host,
        user=args.to_user,
        password=args.to_password,
        key_path=args.to_key,
        port=args.to_port,
        name="target",
    )

    migrator = HostMigrator(src, dst, args)
    try:
        migrator.run_all()
    except Exception as e:
        log_error(str(e))
        sys.exit(1)


if __name__ == "__main__":
    main()
