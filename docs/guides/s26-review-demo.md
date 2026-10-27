# S26 Sprint Review demo

Sprint is ready for PO Review; only PO Review may close it. S26.5 completion does not itself complete the sprint.

## Demonstrate the outcome

1. Open https://cuan-s25-dojo.fural.space/team in the existing owner's authenticated browser. One roster shows pin1 and pin2, one connection each. pin1 retains Kuota habis; pin2's earlier verification expired to Belum diperiksa. Do not click Periksa or repeat consent merely for this demo.
2. Open pin1 Inbox: its saved real reply and later contextual quota failures remain. Refresh: request/message UUIDs and failure bubbles persist, without replay or account switching. Open pin2 Inbox: retained actual nonce reply is independent evidence of the original second identity, not a new live provider check.
3. Inspect mobile screenshots and evidence JSON for 360/390 px. The observed result is actual served staging, with no response fixture/asset overlay. CLI isolation/progress fixture evidence is explicitly separate from actual provider replies.
4. Open https://cuan.dojo.cc/team and /inbox. S26 assets are deployed and healthy on schema20. Production had zero agents before release and still has zero: no staging credential, identity or history was silently copied. The empty-state Add agent action is available for ordinary use. No new production-provider reply is claimed.
5. Review actual native activation/rollback/automatic-failure recovery records in docs/sprints/evidence/S26/. Old S25 works on additive schema20. Private backups exist outside web/release payloads; no restore/down migration was used. Existing production table contents and private file hashes remain identical.

## What to accept and limits

S26 delivers one agent/one connection, private runtime isolation, factual expiring status, and durable contextual Inbox acceptance/failure/recovery. Keyed retries preserve at-most-once accepted execution; legacy requests without request_id cannot safely replay a lost response. Browser storage must be enabled and trusted. Process-loss recovery conservatively reports unknown provider outcome, never replays automatically.

Real second-account identities and replies were accepted in isolated staging. No quota bypass, rotation, repeated consent or new provider success was used during release. Headless Chromium is not physical Android/iOS keyboard certification. Existing CSS/chunk build warnings remain. Run-history is still in memory and resets at restart; persisted messages/task data remain. Staging SQL fixtures are labelled and retained; the native drill's separate service was stopped, with its private backup/payload retained. The historical alternate-slug sandbox is retained because ownership/retention is unclear.

## Recommended next backlog for PO Planning

Derived from PO's screenshot corrections, single-agent/connection direction, mobile concern, and the planning-to-review limitations:

- Improve contextual Inbox replies: the earlier greeting-only reply failed nonce acceptance until the existing developer context was used. Prioritize useful context and reply quality without expanding to rooms/delegation.
- Decide whether existing same-owner staging agents/history should be deliberately promoted to production. Current release preserves environment separation; any transfer needs an explicit migration plan and privacy/rollback gates, rather than repeating consent or copying credentials silently.
- Verify physical Android/iOS keyboard, viewport and safe-area behavior; current evidence covers browser emulation only.
- Migrate remaining API consumers to persisted request_id and plan eventual removal of unsafe legacy replay expectations.
- Evaluate future provider adapters only as a separately approved scope; current compatibility labels are not functioning new integrations.

**Rekomendasi:** contextual reply quality first — highest immediate usefulness, moderate effort, high urgency. **Alternatif:** deliberate same-owner production onboarding/continuity first — high adoption impact, bounded migration effort, urgency depends on the PO's intended production use. **Alasan:** both address observed usage gaps while keeping rooms, delegation and quota workarounds out of scope. No scope change is implemented during this sprint.
