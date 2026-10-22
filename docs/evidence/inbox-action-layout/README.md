# Inbox action layout verification

Clean frontend build 6435951 passes fixture browser checks at 360, 820 and 1440
pixels in light/dark themes. Changing the running status from a short to a long
message does not change Cancel's horizontal position. Detail opens its timeline
without page overflow. Desktop selected row and its menu share the same background;
the menu remains keyboard accessible. Conversation avatars and previews are smaller.
No messages, task launches or provider calls were submitted.

Production release 19d3e79 is healthy on schema25, auto-migration disabled.
Read-only browser checks verify exact deployed assets and continuous selected row.
Actual rollback drill was refused because live Inbox work appeared after release;
it was not forced. All original application table rows remain unchanged; one
Inbox request and two messages were added during traffic. All 913 backed-up profile
files remain present, but four differ in bytes; one of seven existing item read
states changed, while eleven notification read flags were retained. These profile
and read-state changes are observed during traffic, not attributed by this test.
The private pre-release backup remains available. No production inference or
GitHub mutation was submitted by the verifier. This release also includes tested
task worktree isolation, retention and rollback compatibility gates; deterministic
PTY/worker tests do not establish behavior of every real provider sandbox.
