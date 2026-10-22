## Isolated native Cuan installation on Dracin

Test URL: https://cuan-goal-test-dojo-dojo.fural.space

The native app runs on the Dracin host under its own `cuan-goal-test` systemd
service/Linux user, private HOME `/var/lib/cuan-goal-test`, database/role
`cuan_goal_test`, and port 18086. It uses production-tested release `98f8811`,
with schema25, explicit migrations and `CUAN_ENV=staging`. No production database,
owner password, AI profile or Fural account was copied. A new test owner was
configured privately before publishing the URL. Login instructions are private,
outside this repository; they must never be committed.

A separate Fural custom sandbox `CS55b9e290b3891291c4fa4ea2` publishes an SSH
forward on port8086. Its dedicated key is restricted to forwarding to
127.0.0.1:18086, with no command/shell/PTY access. No Cloudflare/Coolify config
or production proxy change was required. The sandbox boot supervises the
forward; systemd independently supervises the app.

Resource limits: CPU75%, MemoryHigh512MiB, MemoryMax768MiB, TasksMax192. The host
had 10.69GiB free disk/854MiB available RAM before setup and 10.07GiB free after.
Dracin worker, MySQL, Nginx and PHP-FPM remain active. This is capacity for a
bounded test, not proof of concurrent provider capacity under real load.

The supported native installer and service setup completed; the official CLI
installer validated its download, and native Bubblewrap readiness passed.
Browser login, empty Agents list and New agent → Connect Google form pass at
390/1440px with no overflow/page errors. Google consent/inference is not yet
verified and requires the owner's next action. No agent was created by the
browser check. See browser.json, infrastructure.json and screenshots.

## Operator checks and recovery

On Dracin, inspect `systemctl status cuan-goal-test` and its journal. Check the
application with `sudo -u cuan-goal-test /var/lib/cuan-goal-test/app/cuan check`.
Persistent config is app/config.json (0600), never the repository. Native
installation source is /opt/cuan-goal-test-source. Do not restart the app or
proxy while the owner has a pending Google consent flow. Backup the dedicated
PostgreSQL DB plus this HOME before upgrades; app readiness does not establish
AI readiness.

If the public URL fails, inspect the custom sandbox's existing boot logs and
process rather than publishing a duplicate. Stopping only this test service and
its proxy leaves Dracin and production Cuan unchanged. Retain HOME/DB until the
owner's testing is complete; no automatic destructive teardown is scheduled.

## Cuan production storage cleanup

Only 16 disposable upload copies in ~/.local/share/cuan-releases were removed.
Each candidate had a matching immutable release manifest, exact file set and
verified checksums in /opt/cuan/releases; no symlinks were followed. A release
lock, current/previous/newest protection, age guard and live process path check
were applied. Validated regular files were individually unlinked, then empty
directories removed. No recursive force deletion was used.

597,987,328 bytes were reclaimed; free disk rose from 1,609,854,976 to
2,207,842,304 bytes. App releases, rollback backups, AI profiles and workspaces
were retained. Production remains98f8811; readiness and all37 retained application
tables,930 profile files,7 item states and11 read flags verify unchanged.
