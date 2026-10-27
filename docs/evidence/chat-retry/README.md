## Reply recovery release

Application release: `12b044629281a4b1326a0a7ee96795d0fd5be805`,
https://cuan.dojo.cc, schema24 with automatic migrations disabled.

Targeted memory/API/runtime tests and the clean TypeScript/Vite build passed.
A real isolated PostgreSQL schema test sent twelve concurrent retry requests and
verified one child attempt, one original user message, unchanged correlation,
owner/room restrictions, and rejection of uncertain/active outcomes. A fake-provider
integration test retried the same source three times and verified one provider
execution, the original context, and one final reply.

Browser fixtures passed at 390/820/1440px, including a lost acceptance response,
same-source retry, one visible user message, status-only uncertain outcomes,
quota navigation, and no horizontal overflow. See browser.json.

Read-only production smoke verified backend recovery metadata and mobile/desktop
controls using existing failures. It started no retry and made no AI calls.
The native release health/readiness checks passed. All 36 application tables
(excluding ephemeral auth sessions) and 809 account-profile files matched the
pre-release snapshot. See production.json.

This release is a safe fresh attempt, not native provider-session Resume.
