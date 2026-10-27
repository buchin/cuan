## Combined native AI prerequisite setup

The native CLI and Bubblewrap setup are now one command:
`sudo python3 scripts/setup-agent-runtime.py --user cuan` (use the actual
non-root Linux service user). The sandbox gate runs before downloading the CLI.
The existing installer verifies the official release SHA-512. This command does
not create accounts or start inference; Settings → Agents remains the sign-in flow.

The read-only `--check` path was run as root on the native production host for
service user `ubuntu` on2026-10-20. It exited0 in2.6s: the actual service-user
Bubblewrap boundary check passed, and the actual installed AGY CLI successfully
ran `--version` inside a fresh network-disabled sandbox with disposable HOME.
No provider profile was mounted; no inference or token refresh was requested.
This verifies the existing native runtime, not a new CLI download/install.

Three targeted contracts in `scripts/test-agent-runtime.py` pass: success requires
sandboxed CLI startup, missing CLI never reports ready, and startup failure never
reports ready or exposes captured provider output. `--check` delegates only
read-only checks, not installation. Help describes service-user and check options.

The command deliberately rejects installing inside the stock Docker image.
Container app/database persistence is verified separately in `../docker-install/`;
turnkey container AI execution remains unresolved. A ready native runtime still
requires authenticated account, project access and execution evidence. Do not
interpret the local version check as a successful provider response.
