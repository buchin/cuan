## Guided first use

Studio now guides a workspace with no enabled agent to **Connect your AI**.
The existing account sign-in flow opens directly. Saving the agent returns to
Studio with the goal draft intact. The first-goal view offers **Connect a project
(optional)**; users can instead submit a goal and let Cuan organize the project.
No execution occurs merely by opening or finishing onboarding.

Loading/error states are distinct from an empty roster. A failed agent-list fetch
does not invite a duplicate account. Returning workspaces with existing tasks
retain the normal Studio. Cancelling project setup preserves the goal draft.
English is the default and the new copy includes Indonesian translations.

## First-install correctness

Migration19 no longer inserts a shared owner credential when it is pending on a
new installation. Already-applied migrations remain skipped; no existing owner is
deleted or reset, and no new schema version is introduced. Existing installations
must maintain their own account/password settings. The old command-entrypoint
seed, which forced Default to the Cuan repository/name, is removed. The shared
project service creates a neutral Default and preserves existing configuration.

## First-owner browser flow

An unowned installation now shows **Set up your workspace** at root/login, with
email and a new password. A successful setup enters Studio's AI connection guide.
Configured installations retain normal sign-in; shared default-credential hints
and technical login clutter are removed. Authentication progress stays local to
the form so failures retain inputs and visible error feedback. PostgreSQL owner
creation is serialized in a transaction; six concurrent first-setup requests
create exactly one owner and return403 for the rest, without changing schema.

## Verification

- Real PostgreSQL, empty dedicated database: migrate all25; owner setup required;
  unauthenticated access rejected; six concurrent setup requests create exactly one owner; later setup denied;
  missing-agent guidance; dedicated account connection; Studio submission; one
  task/attempt and verified result; identical-goal replay; server reconstruction
  retains owner session, task, outcome and renamed project. Deterministic provider
  and sandbox fixtures only, no external inference. Latest run passed in3.49s. Database dropped.
- Actual candidate native binary, not a test server: starts against a separate
  empty PostgreSQL database and temporary HOME, migrates to25, requires owner setup,
  creates neutral Default, and preserves session/name/repository across a real
  process stop/start. Exactly one owner/project. No provider invocation or production
  database writes. Repeatable: `sudo python3 scripts/test-native-install.py --binary
  ./bin/cuan-server`. The script removes its dedicated database and HOME afterward.
- Exact built frontend with API fixtures at390/820/1440: connect/sign-in/save returns
  to Studio; draft retained; optional project dialog/cancel; no premature goal
  submission; fetch error retry; existing-workspace behavior; no horizontal overflow.
  See browser.json and screenshots. Fixture OAuth never contacts Google.
- TypeScript/Vite production build passes. Existing CSS syntax and chunk-size
  warnings persist. Targeted project-default and embedded migration checks pass.

## Boundaries

The real PostgreSQL lifecycle uses a deterministic local provider; it does not
prove arbitrary model decisions. Real native AGY filesystem/process sandbox
readiness is established by the separate chat-sandbox evidence. Docker Compose,
image build and nested AGY runtime remain unverified here. These are not claimed
as a supported turnkey AI installation.

Owner setup/sign-in browser checks also pass at390/820/1440: first-run root entry,
no shared credentials, show/hide password, failure input retention, successful
setup into AI onboarding, reload session retention, stale setup conflict and
setup-status retry. See owner-browser.json. OAuth and owner-account creation in
these browser fixtures are simulated; separate SQL/native tests use real local
storage and APIs with temporary credentials.

## Production delivery

Application release49f89ec68d8ca35e8fe61ca247ac251f526336a1 is active at
https://cuan.dojo.cc with schema25 and auto-migration disabled. The checksummed
native release passed health/readiness. A real rollback to0cd57b8 and reactivation
of49f89ec both passed without a down migration. Private pre-release backup:
`/opt/cuan/backups/onboarding-owner-release-20261020`.

Read-only production browser verification passed existing-owner sign-in, Studio,
English default, explicit Indonesian selection, recovery controls and new served
onboarding strings. It created no owner/agent/task/message and invoked no provider.
All37 retained application tables (auth sessions excluded) and878 profile files
match the backup. New-owner UI is tested with fixtures and disposable PostgreSQL,
not by resetting production owner state. See production.json.
