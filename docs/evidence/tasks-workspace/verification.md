# Tasks workspace verification

- Vite/TypeScript production build passed.
- Targeted task service, HTTP lifecycle, real PTY, terminal manager and chat-to-task regression checks passed.
- Fixture browser checks passed across360/390/820/1440px and light/dark themes, with complete edit, launch recovery, terminal, revision/approval, activity, delete and create workflows. See `browser.json`.
- Dedicated PostgreSQL `cuan_tasks_workspace` migration22→23 and DBStore round trip passed. The database was removed after verification. Legacy rows and old-style SQL remained compatible; completed task approval survived delayed reconciliation.
- The production pre-migration private dump was validated;36 table snapshots and507 account-profile files backed up. No active inbox requests or task attempts.
- Schema23 explicitly applied while auto-migrate remained false. The previous deployed application remained healthy on schema23.
- No real provider calls were used for these checks. PTY checks use deterministic local executables.

The legacy `TestServer_RunTaskAgentEndpoint` and `TestServer_TaskTerminal_RunTaskAgent_BYOAgentHarness` expect an anonymous POST to start execution; they return401 under the current authenticated API. They are superseded for launch behavior by authenticated Tasks workspace lifecycle checks, which verify registered agents, actual PTYs, and durable attempts.

Production activation, smoke, and data-retention receipts are recorded separately after deployment.
