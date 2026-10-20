# Troubleshooting

## Installer fails

Read the reported error before rerunning. `--skip-preflight` does not bypass Docker
Engine, Compose, or readiness requirements. Check `docker info` and
`docker compose version`. On failed startup, inspect:

```sh
docker compose --env-file .env logs --tail 100 app db
```

For staging add `-f docker-compose.staging.yml --env-file .env.staging` instead.
Keep logs private and redact credentials before sharing. Replace placeholder
secrets in manually copied configuration; never replace an existing instance's
auth secret casually, because stored credentials depend on it.

## Web application works but agents do not

`/healthz` checks the application; `/readyz` checks database readiness. Neither
proves provider authentication, quota, project access, or sandbox readiness.
Check **Settings → Agents** and the message's **Detail**. Verify CLI availability
and authentication under the service user, HOME, and PATH. For AGY, run the
[sandbox check](chat-sandbox.md). Stock Docker AGY execution is not verified;
do not enable privileged containers to hide that limitation.

## Work is waiting or interrupted

Account capacity differs from model quota. Waiting authorized work is persisted
and retried automatically for up to 24 hours. Use **Cancel waiting work** if it
is no longer wanted. A request with unknown effects cannot safely be blindly
replayed. Use **Check status** and inspect the project/session before continuing.
See the [lifecycle guide](message_task_lifecycle.md).

## Data disappears after container recreation

Retain database, app-data, and app-HOME volumes. Avoid `down -v`. An installation
created before persistent app mounts needs its existing container files exported
before recreation. Restore a consistent database/profile/data backup rather than
creating new credentials over an existing instance.

## Reverse proxy or streaming problems

Match the configured application host port. Compose publishes that port on host
interfaces; use host firewall rules where needed. Enable streamed responses and
appropriate timeouts in the proxy. Ensure HTTPS and correct domain configuration
for remote use. Staging Caddy defaults to 8082/8443, production to 80/443.

## Upgrade or rollback

Stop accepting new work, confirm active/pending execution is clear, and back up
the database, persistent data, account profiles, and private environment file.
Use the [deployment recovery procedure](deployment_recovery_guide.md). An old
binary may not support a new schema: rollback compatibility must be verified for
the specific release. Do not delete locks or reset work state while an execution
process may still be alive.
