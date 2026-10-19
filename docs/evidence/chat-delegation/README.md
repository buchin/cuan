## Chat delegation verification

Removed the draft-only requirement for execution requests. Added internal coordinator/development contracts, structured delegation through the existing task runner, scoped authorization provenance and project checks, source/request idempotency, progress delivery, durable coordinator result review and bounded concrete corrections.

Targeted deterministic tests exercise actual chat acceptance → provider action → tracked task → real PTY → final report → coordinator result returned to chat. They cover replay without new attempts, internal context visibility, unauthorized system/foreign approval sources, malformed/unknown protocol fields, concrete corrections of the same task and a three-attempt ceiling. No real provider is called.

Existing targeted inbox/draft/project/context, task runner/workspace and explicit Done tests passed. The product guide documents role boundaries, usage and current limitations: the model interprets semantic consent; provenance validation alone does not prove it, there is no new sprint scheduler, and live provider adherence is not fixture-tested.

## Production release

Code `0232d2360ddf8834c1ab2ffd84b2d0d3647a001e` deployed to https://cuan.dojo.cc using the native atomic release. Idle gate passed with zero active work; validated database and profile backup retained privately. Readiness healthy, schema23 unchanged, automatic migrations disabled.

The same compiled tests ran on the production host in an isolated temporary environment with memory stores and fake provider: full delegation, task reporting, result review, save-only behavior, approval provenance, repeated delivery, concrete revisions and bounded attempts passed, along with interactive Done/Review/Blocked reporting. No real provider or production database was used by the fixture.

Authenticated production smoke passed eight viewport/theme cases, result/history navigation, original-request preservation, optional metadata/terminal inspection, approval gates, ready/draft state and exact deployed asset comparison. Synthetic tasks were deleted; no run-agent/provider calls or real acceptance. See `production-browser.json` and screenshots.

Retention verified35 existing tables unchanged (login sessions excluded) and706 account-profile files unchanged. Project checkout fast-forwarded cleanly from84e65e0 to0232d23. Live provider adherence to the new delegation protocol and semantic approval interpretation was not tested. Existing historical tasks are excluded from automatic review; the path applies to newly delegated work.
