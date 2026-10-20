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
