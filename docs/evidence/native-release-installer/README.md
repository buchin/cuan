## Native application release installer contracts

Six targeted disposable-file contracts pass via python3 scripts/test-native-installer.py:
private config/modes, retained config/data/previous release on upgrade, failed migration
retains active release while diagnostics remain private, malformed settings remain
untouched, concurrent install lock, dry-run without writes, and launcher path/env
handling without shell evaluation.

The contract fixtures use prebuilt fake executable files. No real database,
production writes or provider calls occur. This does not establish an actual fresh
binary/database installation or service-manager supervision. Those remain explicit
follow-up verification; AI setup continues to use the existing separate native
runtime installer.

## Actual native application installation

The actual installed application server and a freshly built migrator now pass the
checked-in test-native-installer-live.py harness on a native PostgreSQL host.
A random empty cuan_fresh_install_* database and temporary non-root HOME are used.
The installer migrates to25, starts via its installed launcher, serves its static
assets and admits initial owner setup with neutral Default. Reinstallation creates
a new immutable release while preserving config, prior release, workspace, owner
session and renamed project across process restart. The readiness command passes.
The test database, temporary HOME and transfer binary are removed.

This supersedes the earlier fixture-only evidence for application installation.
No AI authentication/inference or systemd supervision is exercised. The broader
AI runtime/container support and unattended service setup work remain open.
