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

## Verification

- Real PostgreSQL, empty dedicated database: migrate all25; owner setup required;
  unauthenticated access rejected; initial owner created and second setup denied;
  missing-agent guidance; dedicated account connection; Studio submission; one
  task/attempt and verified result; identical-goal replay; server reconstruction
  retains owner session, task, outcome and renamed project. Deterministic provider
  and sandbox fixtures only, no external inference. Passed in3.09s. Database dropped.
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
