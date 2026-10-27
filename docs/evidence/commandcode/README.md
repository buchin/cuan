# CommandCode auth responsive evidence

Captured against the built Cuan web artifact served at `https://cuan-commandcode-evidence-dojo.fural.space/` from commit `ced635bfa1da65b339e2929692266cc63faca89e`.

## Reproduce

```sh
cd web
npm install --ignore-scripts
npm run build
# publish web/dist with any static HTTP server, then open /login
```

Browser capture commands (managed browser):

- `browser_navigate https://cuan-commandcode-evidence-dojo.fural.space/login`
- `browser_screenshot width=360 height=800` → [360px capture](commandcode-login-360.svg)
- `browser_screenshot width=390 height=844` → [390px capture](commandcode-login-390.svg)
- `browser_evaluate` accessibility probe → [machine result](accessibility.json)

The route is a reproducible local/static build, but the authenticated Team/Settings view could not be reached: the app returned `Could not check workspace setup. Check your connection and try again.` because the provider/API endpoint is unavailable from the sandbox. Therefore this evidence records the blocker and does not claim Team/Settings DoD passed.

## Setup failure diagnostics

When `/api/v1/auth/status` cannot be reached, the login screen now keeps the
workspace in retry mode and reports a safe diagnostic (`HTTP <status>` or the
network error). It never displays tokens, API keys, terminal output, or provider
credentials. After the endpoint recovers, **Try again** re-runs setup discovery;
existing auth state is then loaded before `/settings/agents` is opened.
