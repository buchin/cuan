## Production storage policy

The production VPS hosts several projects. On 2026-10-21 the40GB root filesystem
reached100%, preventing a release upload. The active application release was not
switched by that failed upload.

### Nginx logs

The overlapping `/etc/logrotate.d/dojo-nginx` rule matched both active `.log`
files and already rotated `.log.[0-9]*` files. This produced repeated `.1.1`
generations and retained7.3GB in `/var/log/nginx`.

The overlapping rule was backed up outside logrotate's configuration directory.
The standard `/etc/logrotate.d/nginx` rule now matches only active `*.log`, rotates
daily, retains seven generations, compresses old logs and checks `maxsize100M`
at each logrotate invocation. This is not a continuous100MB hard cap.
`logrotate -d /etc/logrotate.conf` and an actual Nginx rotation passed.

400 nested-generation archives were moved into a dated legacy directory below
`/var/log/nginx` and compressed:6,099,951,255 bytes became682,709,774 bytes. No log
contents were deleted. The legacy directory is a retained one-time archive;
ordinary rotation does not prune it. Active logs continue under the standard rule.

### Cuan releases

Use `python3 scripts/prune-release-artifacts.py` for a dry run or append `--apply`
for removal. The default installation is `/opt/cuan`; `--base` supports an isolated
installation. Keep five newest valid payloads and remove only candidates older
than24hours. Preserve current/previous symlinks and any payload referenced by
running processes. Unknown directories and invalid manifests are left alone.
The pruner shares `.release.lock` with activation/rollback and fails closed if
rollback references are missing. Application data, workspaces and backups are
outside its scope.

Production runs the installed script at
`/opt/cuan/maintenance/prune-release-artifacts.py`, daily through
`/etc/cron.d/cuan-release-retention`. Its log is overwritten per run at
`/var/log/cuan-release-retention.log`. Existing application backups were retained.
Fourteen redundant upload copies were additionally removed only after verifying
the complete installed counterpart against its manifest, reclaiming525,444,752
bytes; installed releases were retained during that upload-copy cleanup.

### Capacity decision

After log correction and release retention, production has8.6GB free (78% used),
versus458MB before the full cleanup. Nginx and Cuan remain active. Dracin has the
same40GB disk and about5.6GB free, with less available memory at observation time.
Moving Cuan there is not a storage improvement. Fix retention first; consider a
larger volume or dedicated server when real persistent data, rather than retained
artifacts, requires it. Backup retention needs its own restore-tested policy;
this cleanup intentionally did not delete existing backups.
