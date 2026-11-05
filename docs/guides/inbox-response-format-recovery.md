# Recovering a completed conversation response

When the provider completes but its coordinator JSON is invalid, Cuan performs
at most one format-repair stage. It supplies the original completed response as
an explicitly untrusted JSON string along with the original conversation scope.
The repair retains findings, uncertainty, task identity and authorization;
it does not restart the user's investigation from scratch.

The runtime denies file reads, searches, MCP and execution tools during this
stage. Only the provider's native finish/format-response tool is allowed. The
workspace is not added as an inspection directory. The existing response and
application action validators still apply before any task or other application
action can be accepted. A forbidden tool, interrupted provider, exhausted bound
or invalid result cannot claim that work was completed. The candidate never
appears in public activity steps. Oversized candidates beyond the existing
40,000-byte transport limit do not trigger another provider turn.

This does not reduce the chosen model, original request deadline or verification
criteria. First-turn implementation claims still need source evidence. A follow-up
that only prioritizes previously discussed proposals should give a concrete
recommendation and tradeoff, without repeating the catalogue or inspecting
unrelated health metrics. Prior proposals remain proposals; fresh implementation
claims require fresh evidence where needed.

Verification:
`go test ./internal/server -run 'TestInboxFormatRepair|TestInboxCLI|TestDecodeInboxAgyResult|TestRecoverCoordinatorReply'`.
Fixtures verify reuse of the candidate, one bounded repair, absent workspace
inspection arguments, rejection of attempted rereads and no candidate leakage in
activity traces. They demonstrate the removed repeated investigation path, not
a measured production latency guarantee.
