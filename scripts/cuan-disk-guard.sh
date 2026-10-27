#!/usr/bin/env bash
# Automated disk space watchdog for Cuan VPS
# Checks disk usage hourly and cleans unrotated third-party logs, build caches, and test artifacts
set -euo pipefail

USAGE=$(df / | awk 'NR==2 {print $5}' | tr -d '%')
LOG="/var/log/cuan-disk-guard.log"

if [ "$USAGE" -ge 75 ]; then
    echo "$(date -u --iso-8601=seconds) [WARN] Disk usage is at ${USAGE}%. Starting automated cleanup..." >> "$LOG"
    
    # 1. Truncate oversized unrotated external Laravel logs (>50M)
    find /srv/dracin-git/shared/storage/logs/ -type f -name "*.log" -size +50M -exec truncate -s 1M {} + 2>/dev/null || true
    
    # 2. Clean Go build cache inside agent account homes
    find /home/ubuntu/.local/share/cuan/accounts/ -type d -name "go-build" -prune -exec rm -rf {} + 2>/dev/null || true
    
    # 3. Clean stale test binaries and old release build artifacts
    find /home/ubuntu/.local/share/cuan-releases/ -type f \( -name "*.test" -o -name "*-server" \) -delete 2>/dev/null || true
    
    # 4. Clean system journal older than 7 days
    journalctl --vacuum-time=7d 2>/dev/null || true
    
    NEW_USAGE=$(df / | awk 'NR==2 {print $5}' | tr -d '%')
    echo "$(date -u --iso-8601=seconds) [INFO] Cleanup complete. Disk usage reduced from ${USAGE}% to ${NEW_USAGE}%." >> "$LOG"
fi
