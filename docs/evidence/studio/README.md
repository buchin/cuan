# Studio verification

- Targeted Go tests passed: Studio authenticated acceptance, replay and conflict, validation, missing agent; managed project workspace creation/reuse; Default preservation; delegation protocol; existing/topic delegation; approved save/provenance; managed project execution.
- Delegation fixtures use a deterministic fake provider and real PTY. Managed project execution confirms its working directory matches the registered workspace. No paid provider invocations.
- Frontend TypeScript/Vite production build passed. Existing bundle-size and CSS minification warnings remain.
- `browser-check.cjs` serves the production build with isolated API fixtures. `browser.json` records eight viewport/theme combinations (360/390/820/1440, light/dark), default Studio route, accessible goal input, 16px text, draft persistence, no selectors, Inbox New message routing, retry identity and accepted Inbox topic. No browser exceptions or horizontal overflow.
- Screenshots: `studio-light-390.png`, `studio-dark-390.png`, `studio-light-1440.png`, `studio-dark-1440.png` (and remaining tested widths).
- Production backup validated before release: schema23, 36 table snapshots, 706 profile files, no active Inbox requests or task attempts. Activation rechecks idle state and readiness, with rollback through the native release script.

## Production result

Released code `376c02358b877d39ee134ff927d8f293f836ca97` to https://cuan.dojo.cc using the native atomic release script. Public browser smoke passed all eight viewport/theme combinations: Studio is the default route, Inbox New message opens Studio, and all served JS/CSS/font assets match the build byte for byte. Production goal submission was deliberately excluded; no live provider execution or customer messages were triggered.

The isolated test binary also passed Studio acceptance/replay and managed project delegation on the production host with a fake provider and real PTY, without production database access. Readiness is healthy, schema remains23 with automatic migrations disabled. All35 retained application tables and706 account/profile files match the validated backup (login sessions excluded). See `production-browser.json` and `production-studio-*.png`.
