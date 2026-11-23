<#
.SYNOPSIS
    Cuan Universal Installer for Windows (PowerShell)
.DESCRIPTION
    Installs and runs Cuan in a local Docker Desktop environment on Windows.
    Stores workspace and configuration in $env:USERPROFILE\.cuan.
.EXAMPLE
    irm https://cuan.work/install.ps1 | iex
#>

[CmdletBinding()]
param(
    [string]$InstallDir = $(if ($env:CUAN_INSTALL_DIR) { $env:CUAN_INSTALL_DIR } else { Join-Path $env:USERPROFILE ".cuan" }),
    [int]$Port = 8080,
    [switch]$DryRun,
    [switch]$SkipPreflight
)

$ErrorActionPreference = 'Stop'

# Ensure TLS 1.2 is enabled for older PowerShell versions on Windows 10
try {
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]'Tls12,Tls13'
} catch {
    try {
        [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]'Tls12'
    } catch {
        # Retain existing system default if explicit assignment fails
    }
}

Write-Host "================================================================================" -ForegroundColor Cyan
Write-Host " Cuan Installation Wizard - Windows (PowerShell)" -ForegroundColor Cyan
Write-Host "================================================================================" -ForegroundColor Cyan

# ------------------------------------------------------------------------------
# 1. PREFLIGHT CHECKS
# ------------------------------------------------------------------------------
Write-Host "==> Menjalankan preflight checks untuk Windows..." -ForegroundColor Cyan

# 1.1 Docker CLI Check
$dockerCmd = Get-Command docker -ErrorAction SilentlyContinue
if (-not $dockerCmd) {
    Write-Host "[ERROR] Docker Desktop belum terinstal di komputer Windows Anda." -ForegroundColor Red
    Write-Host "        Silakan unduh dan pasang Docker Desktop untuk Windows dari:" -ForegroundColor Yellow
    Write-Host "        👉 https://www.docker.com/products/docker-desktop/" -ForegroundColor Cyan
    Write-Host "        Pastikan opsi 'Use WSL 2 instead of Hyper-V' dicentang saat instalasi." -ForegroundColor Yellow
    Write-Host "        Setelah instalasi selesai, buka Docker Desktop lalu jalankan kembali script ini:" -ForegroundColor Yellow
    Write-Host "        irm https://cuan.work/install.ps1 | iex" -ForegroundColor White
    exit 1
}
Write-Host "    [OK] Docker CLI terdeteksi." -ForegroundColor Green

# 1.2 Docker Daemon Status Check
$dockerRunning = $false
try {
    $null = & docker info 2>&1
    if ($LASTEXITCODE -eq 0) {
        $dockerRunning = $true
    }
} catch {
    $dockerRunning = $false
}

if (-not $dockerRunning) {
    Write-Host "[ERROR] Docker Desktop terdeteksi tetapi belum aktif / berjalan." -ForegroundColor Red
    Write-Host "        Silakan buka aplikasi Docker Desktop di Windows Anda dan tunggu sampai statusnya 'Engine running'." -ForegroundColor Yellow
    Write-Host "        Setelah status Docker Desktop aktif, jalankan kembali perintah ini:" -ForegroundColor Yellow
    Write-Host "        irm https://cuan.work/install.ps1 | iex" -ForegroundColor White
    exit 1
}
Write-Host "    [OK] Docker Desktop aktif (Engine running)." -ForegroundColor Green

# 1.3 Docker Compose Check
$composeCmd = $null
try {
    $null = & docker compose version 2>&1
    if ($LASTEXITCODE -eq 0) {
        $composeCmd = "docker compose"
    }
} catch {}

if (-not $composeCmd) {
    $legacy = Get-Command docker-compose -ErrorAction SilentlyContinue
    if ($legacy) {
        $composeCmd = "docker-compose"
    } else {
        Write-Host "[ERROR] Docker Compose tidak ditemukan. Pastikan komponen Docker Compose terpasang di Docker Desktop." -ForegroundColor Red
        exit 1
    }
}
Write-Host "    [OK] Docker Compose terdeteksi ($composeCmd)." -ForegroundColor Green

# 1.4 Port Check
if (-not $SkipPreflight) {
    $portActive = $false
    try {
        $tcpProps = [System.Net.NetworkInformation.IPGlobalProperties]::GetIPGlobalProperties()
        $endpoints = $tcpProps.GetActiveTcpListeners()
        foreach ($ep in $endpoints) {
            if ($ep.Port -eq $Port) {
                $portActive = $true
                break
            }
        }
    } catch {
        $portActive = $false
    }

    if ($portActive) {
        Write-Host "    [WARN] Port $Port sedang aktif digunakan. Jika ini update, Cuan akan me-restart container." -ForegroundColor Yellow
    } else {
        Write-Host "    [OK] Port $Port tersedia." -ForegroundColor Green
    }
}

# ------------------------------------------------------------------------------
# 2. DRY RUN / PLAN OUTPUT
# ------------------------------------------------------------------------------
if ($DryRun) {
    Write-Host ""
    Write-Host "--------------------------------------------------------------------------------" -ForegroundColor Cyan
    Write-Host " DRY-RUN EXECUTION PLAN (No changes made)" -ForegroundColor Cyan
    Write-Host "--------------------------------------------------------------------------------" -ForegroundColor Cyan
    Write-Host " Environment:         local (Windows)"
    Write-Host " Root Directory:      $InstallDir"
    Write-Host " Compose Command:     $composeCmd"
    Write-Host " Application Port:    $Port"
    Write-Host " Target URL:          http://localhost:$Port"
    Write-Host "--------------------------------------------------------------------------------" -ForegroundColor Cyan
    Write-Host "Dry-run verification completed successfully." -ForegroundColor Green
    exit 0
}

# ------------------------------------------------------------------------------
# 3. DIRECTORY SETUP & BOOTSTRAP
# ------------------------------------------------------------------------------
Write-Host "==> Menyiapkan direktori kerja Cuan di $InstallDir..." -ForegroundColor Cyan
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

$composeFile = Join-Path $InstallDir "docker-compose.yml"
if (-not (Test-Path $composeFile)) {
    Write-Host "==> Mengunduh file proyek Cuan..." -ForegroundColor Cyan
    $bootstrapDone = $false

    # Attempt git clone if git is installed
    $gitCmd = Get-Command git -ErrorAction SilentlyContinue
    if ($gitCmd) {
        try {
            & git clone --depth 1 https://github.com/buchin/cuan.git $InstallDir 2>&1 | Out-Null
            if ($LASTEXITCODE -eq 0 -and (Test-Path $composeFile)) {
                $bootstrapDone = $true
            }
        } catch {
            $bootstrapDone = $false
        }
    }

    # Fallback to downloading zip archive via PowerShell
    if (-not $bootstrapDone) {
        $zipUrl = "https://github.com/buchin/cuan/archive/refs/heads/main.zip"
        $guid = [System.Guid]::NewGuid().ToString("N")
        $tempZip = Join-Path $env:TEMP "cuan-bootstrap-$guid.zip"
        $tempExtract = Join-Path $env:TEMP "cuan-bootstrap-$guid"

        try {
            Write-Host "    Mengunduh arsip Cuan dari GitHub..." -ForegroundColor Cyan
            Invoke-WebRequest -Uri $zipUrl -OutFile $tempZip -UseBasicParsing
            Expand-Archive -Path $tempZip -DestinationPath $tempExtract -Force
            $sourceDir = Join-Path $tempExtract "cuan-main"
            if (Test-Path $sourceDir) {
                Get-ChildItem -Path $sourceDir -Force | ForEach-Object {
                    Copy-Item -Path $_.FullName -Destination $InstallDir -Recurse -Force
                }
                $bootstrapDone = $true
            }
        } catch {
            Write-Host "[ERROR] Gagal mengunduh dan mengekstrak repositori Cuan: $_" -ForegroundColor Red
            exit 1
        } finally {
            if (Test-Path $tempZip) { Remove-Item -Path $tempZip -Force -ErrorAction SilentlyContinue }
            if (Test-Path $tempExtract) { Remove-Item -Path $tempExtract -Recurse -Force -ErrorAction SilentlyContinue }
        }
    }
}

# ------------------------------------------------------------------------------
# 4. CONFIGURATION & ENVIRONMENT SETUP (.env)
# ------------------------------------------------------------------------------
$envFile = Join-Path $InstallDir ".env"
$envExample = Join-Path $InstallDir ".env.example"

function New-CuanSecret {
    param([int]$Length = 24)
    $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
    $bytes = New-Object byte[] $Length
    $rng.GetBytes($bytes)
    $sb = New-Object System.Text.StringBuilder
    foreach ($b in $bytes) {
        $null = $sb.Append($b.ToString("x2"))
    }
    return $sb.ToString()
}

if (-not (Test-Path $envFile)) {
    Write-Host "==> Mengonfigurasi environment (.env) dengan secret baru..." -ForegroundColor Cyan
    if (-not (Test-Path $envExample)) {
        Write-Host "[ERROR] File .env.example tidak ditemukan di $InstallDir" -ForegroundColor Red
        exit 1
    }

    $dbSecret = New-CuanSecret 24
    $authSecret = New-CuanSecret 32

    $envContent = Get-Content -Path $envExample -Raw
    $envContent = $envContent -replace "replace_with_strong_password_min_16_chars", $dbSecret
    $envContent = $envContent -replace "replace_with_random_32_character_auth_secret_key", $authSecret

    if ($Port -ne 8080) {
        if ($envContent -match "(?m)^PORT=") {
            $envContent = $envContent -replace "(?m)^PORT=.*", "PORT=$Port"
        } else {
            $envContent += "`r`nPORT=$Port"
        }
    }

    [System.IO.File]::WriteAllText($envFile, $envContent, [System.Text.Encoding]::UTF8)
    Write-Host "    [OK] File .env berhasil dibuat." -ForegroundColor Green
} else {
    Write-Host "    [INFO] File .env lama ditemukan. Mempertahankan konfigurasi dan secret yang ada." -ForegroundColor Green
}

# ------------------------------------------------------------------------------
# 5. CONTAINER DEPLOYMENT
# ------------------------------------------------------------------------------
Write-Host "==> Menjalankan service Cuan dengan $composeCmd..." -ForegroundColor Cyan
Set-Location $InstallDir

if ($composeCmd -eq "docker compose") {
    & docker compose -f docker-compose.yml --env-file .env up -d --build
} else {
    & docker-compose -f docker-compose.yml --env-file .env up -d --build
}

if ($LASTEXITCODE -ne 0) {
    Write-Host "[ERROR] Gagal menjalankan container Cuan via Docker Compose." -ForegroundColor Red
    Write-Host "        Periksa error di atas atau jalankan '$composeCmd logs app db'." -ForegroundColor Yellow
    exit 1
}

# ------------------------------------------------------------------------------
# 6. READINESS VERIFICATION & BROWSER LAUNCH
# ------------------------------------------------------------------------------
Write-Host "==> Menunggu aplikasi siap (healthcheck & database readiness)..." -ForegroundColor Cyan
$ready = $false
for ($attempt = 1; $attempt -le 30; $attempt++) {
    if ($composeCmd -eq "docker compose") {
        $null = & docker compose -f docker-compose.yml --env-file .env exec -T app /app/cuan-server --healthcheck 2>&1
        $hCode = $LASTEXITCODE
        $null = & docker compose -f docker-compose.yml --env-file .env exec -T app /app/cuan-server --readycheck 2>&1
        $rCode = $LASTEXITCODE
    } else {
        $null = & docker-compose -f docker-compose.yml --env-file .env exec -T app /app/cuan-server --healthcheck 2>&1
        $hCode = $LASTEXITCODE
        $null = & docker-compose -f docker-compose.yml --env-file .env exec -T app /app/cuan-server --readycheck 2>&1
        $rCode = $LASTEXITCODE
    }

    if ($hCode -eq 0 -and $rCode -eq 0) {
        $ready = $true
        break
    }
    Start-Sleep -Seconds 1
}

if (-not $ready) {
    Write-Host "[ERROR] Aplikasi belum siap setelah 30 detik. Data dan konfigurasi tersimpan aman." -ForegroundColor Red
    Write-Host "        Periksa status log dengan: $composeCmd logs app db" -ForegroundColor Yellow
    exit 1
}

Write-Host "    [OK] Aplikasi Cuan dan database PostgreSQL siap digunakan." -ForegroundColor Green
$appUrl = "http://localhost:$Port"
Write-Host "==> Membuka Cuan di browser: $appUrl ..." -ForegroundColor Green
try {
    Start-Process $appUrl
} catch {
    Write-Host "    Silakan buka browser Anda secara manual ke: $appUrl" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "================================================================================" -ForegroundColor Cyan
Write-Host " Cuan berhasil dipasang dan berjalan di Windows Anda!" -ForegroundColor Green
Write-Host " URL Aplikasi:    $appUrl" -ForegroundColor White
Write-Host " Direktori Kerja: $InstallDir" -ForegroundColor White
Write-Host " Health Checks:   /healthz (liveness), /readyz (readiness)" -ForegroundColor White
Write-Host " Dokumentasi:     https://cuan.work/docs" -ForegroundColor White
Write-Host "================================================================================" -ForegroundColor Cyan
