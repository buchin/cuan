## Native coordinator and downloadable delivery

Dedicated Dracin staging `1dd7b42`, schema25. No production inference or
production conversation replay was used.

The first two proposal turns exposed over-strict event classification: native
`user_input`, `finish`, and the schema `finish` tool were marked unsafe. Transcript
inspection identified response formatting only, not implementation. The first two
failed request records are preserved. Commits `e914bbe` and `1dd7b42` classify
these known lifecycle/formatting events while rejecting write/terminal tools.

A subsequent owner message in the same conversation successfully produced a
persisted structured proposal. No task existed before approval. Clicking the
real **Approve and start** UI created exactly one task in Default with the exact
persisted scope and original approval. Its first native execution succeeded,
reported verification, and attached HTML and README through `task report`.

The files were downloaded with the real Inbox buttons. Downloads in a separate
unauthenticated browser context returned401. The downloaded HTML was opened in
Chromium: five conversion values, empty state, mobile overflow and absence of
network requests/page errors passed. The automatic report evaluator completed
and explicitly attributed checks to the work session's report. It created no
new task or execution.

See native-delivery.json and browser-verification.json for identifiers, hashes
and exact results. The downloads directory contains the actual delivered files.
This proves the controlled native proposal/approval/task/report/download path.
It does not prove fresh AI OAuth onboarding or live private GitHub access, and
does not erase the earlier interrupted-task evidence or failed scenarios.

## Read-only activity audit — 2026-10-21

Authenticated GET of the existing task detail confirmed one attempt and a final
`agent_report` with state `done`, followed by succeeded execution. The detail
contained no `agent_report` with state `progress`. Thus this short native task
proves start/final delivery, not intermediate progress reporting. No conversation
was replayed, task changed, or provider invoked for this audit. Progress-report
forwarding remains covered by targeted application fixtures.
