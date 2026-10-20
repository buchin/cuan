## Safe automatic reply recovery

Application release: 3b230b7c2225e69b4036983b657920328210e1d6.

A newly started reply records an explicit recovery policy. After a verified
no-effect failure, the dispatcher can create one stable child attempt after a
short backoff. A reconstructed host reuses the original message/context. A
second failure does not create a third automatic attempt. Older failures, later
messages, active work, cancellation, quota/auth failures and unknown outcomes
are excluded.

Memory/API tests cover eligibility and exhaustion, manual/automatic races,
server reconstruction, one original user message, bounded provider invocations,
and original context. Real isolated PostgreSQL testing mixes twelve manual and
automatic calls and verifies one child. Browser fixtures at 390/820/1440 verify
the recovering state needs no click and suppresses manual retry/status buttons
while recovery is being prepared. No real provider is used.

Clean build and native atomic production activation passed. Read-only production
UI/API smoke passed without creating messages or retries. Before deployment,
the old database had zero rows carrying the new policy, proving old requests
could not become eligible merely through deployment. All 36 application tables
and 863 profile files matched the snapshot afterward; schema24 and automatic
migration settings were unchanged. See production.json.

This closes known safe reply recovery, not the entire product goal. See the
requirements audit for remaining task process ownership, checkpoint/context and
approval-scope gaps.
