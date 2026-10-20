# Studio verification

- Targeted Go tests passed: Studio authenticated acceptance, replay and conflict, validation, missing agent; managed project workspace creation/reuse; Default preservation; delegation protocol; existing/topic delegation; approved save/provenance; managed project execution.
- Delegation fixtures use a deterministic fake provider and real PTY. Managed project execution confirms its working directory matches the registered workspace. No paid provider invocations.
- Frontend TypeScript/Vite production build passed. Existing bundle-size and CSS minification warnings remain.
- `browser-check.cjs` serves the production build with isolated API fixtures. `browser.json` records eight viewport/theme combinations (360/390/820/1440, light/dark), default Studio route, accessible goal input, 16px text, draft persistence, no selectors, Inbox New message routing, retry identity and accepted Inbox topic. No browser exceptions or horizontal overflow.
- Screenshots: `studio-light-390.png`, `studio-dark-390.png`, `studio-light-1440.png`, `studio-dark-1440.png` (and remaining tested widths).
- Production backup validated before release: schema23, 36 table snapshots, 706 profile files, no active Inbox requests or task attempts. Activation rechecks idle state and readiness, with rollback through the native release script.
