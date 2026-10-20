## Native Linux setup

AGY Inbox analysis uses Bubblewrap outside the provider CLI. AGY tool permissions
are automatically approved; the OS mount namespace enforces the filesystem boundary.
Approved task execution keeps its existing separate workflow; this release does
not sandbox task execution or change the Claude/Codex adapters.

On Debian/Ubuntu, run from a trusted Cuan checkout (replace `cuan` with the actual
Linux service user):

```sh
sudo python3 scripts/setup-inbox-sandbox.py --user cuan
sudo python3 scripts/setup-inbox-sandbox.py --user cuan --check
```

The installer extracts the distribution Bubblewrap package into a root-owned
`/opt/cuan/runtime` directory. It installs no system packages and enables no setuid
bits. On Ubuntu with restricted user namespaces, it loads a narrowly attached
AppArmor profile allowing user namespaces for this exact executable. It does not
disable AppArmor or alter global sysctls. Repeating the command is supported.
The service PATH must include its `$HOME/.local/bin`.

A passing check starts a real sandbox under the service user without contacting
an AI provider. Missing/blocked sandbox support fails closed: chat will show an
actionable setup error, with no unrestricted fallback.

## Filesystem and network boundaries

The sandbox exposes system tools and certificates read-only, the selected project
read-only, and explicitly supplied attachment files read-only. Host root,
`/proc`, service environment files, SSH keys, and unrelated account homes are not
mounted. Its temporary filesystem is disposable.

Only the selected account's `.gemini/antigravity-cli` directory is writable and
persistent, so token refresh and provider state continue to work. The provider
and its tools can access that selected authentication directory; this design
does not separate tool processes from the provider's own credentials.

Provider networking remains available, including host-reachable network services.
This is filesystem/process isolation, not a network egress firewall or an
untrusted-code hosting service. Do not treat it as a boundary against a malicious
authenticated provider with access to its own credentials.

## Containers and fresh installs

The existing Docker installer is not yet a verified turnkey AGY runtime. In
addition to an installed/authenticated AGY CLI, nested Bubblewrap needs user
namespace and mount support from the host/container security policy. Merely
adding the Bubblewrap package does not establish compatibility. Native Linux is
the verified deployment path for this change. Do not add `privileged: true`,
disable the host's global protections, or bypass the sandbox to make a container
appear compatible.

Before declaring a fresh install chat-ready, run the sandbox check as the actual
service user, verify CLI startup inside it, and configure an AI account in
Settings → Agents. UI/database health alone does not establish provider readiness.

## Deployment and rollback

Back up the database and account profiles, verify no Inbox/task work is active,
and pass the sandbox check before activating the application release. No schema
migration is introduced. Use the standard atomic release/health rollback procedure.
If the sandbox setup fails, keep the previous application release active.

To roll back the prerequisite after rolling back the application, remove the
service user's `.local/bin/bwrap` symlink only if it points to
`/opt/cuan/runtime/bwrap`, unload the exact AppArmor profile with
`sudo apparmor_parser -R /etc/apparmor.d/cuan-inbox-bwrap`, and remove that profile
and the dedicated runtime binary. Do not delete account profiles or modify
user-namespace sysctls.
