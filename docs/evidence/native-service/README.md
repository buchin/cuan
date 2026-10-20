# Native service recovery verification

Verified 2026-10-20 with actual production-release binaries in a disposable native
installation, separate PostgreSQL database, non-root account, random systemd unit,
and an installation path containing spaces. No provider was invoked.

- Native install, migration 25, owner setup, assets, and reinstall passed.
- Unit enabled for boot; readiness matched MainPID executable and database.
- SIGKILL of the service process caused automatic restart with a new MainPID.
- Owner session, renamed project, workspace marker and private configuration survived.
- Temporary unit, HOME and database were removed after verification.
- Production release remained unchanged and healthy; production was not restarted.

Six targeted local contracts passed with `python3 scripts/test-native-service.py`.
The actual-host harness is `actual-service.py`; it requires root, systemd,
PostgreSQL tools, a postgres OS account and real server/migrator/static artifacts.
It creates only randomly named fixture resources and cleans them in `finally`.
This proves service recovery and data retention, not provider execution or the
complete goal-to-delivery workflow. Existing durable task recovery has separate
evidence under `docs/evidence/durable-goal-recovery/`.
