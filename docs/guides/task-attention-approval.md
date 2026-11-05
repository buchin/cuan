# Approving a focused next step

After automatic corrections reach their limit, Cuan keeps the task blocked and
proposes the specific remaining work from its saved review. The proposal does
not start anything. The task card shows the recommendation above its footer:

- **Detail** on the left opens the saved work and evidence.
- **Others** on the right opens the internal composer for different guidance.
  Existing draft text is preserved; nothing is sent automatically.
- **Approve** selects the exact saved proposal and continues the same task
  through normal durable launch admission. It does not accept an unverified
  result, create a new task, or waive the original scope and checks.

Users can also type their own instructions normally. In customer conversations,
alternative guidance stays in the internal whisper composer.

The server binds a continuation to its original room/project, current attempt,
and persisted review assessment. Changed instructions/evidence, canceled or
completed work, a newer attempt, and active launches invalidate old proposals.
Provider-authored metadata alone grants no authority. Missing or external
prerequisites do not receive a fabricated executable approval action.

Existing historical messages remain unchanged; new exhausted-budget reviews
produce the new proposals. No historical task is restarted by deployment.

Verification: `go test ./internal/server -run 'TestInboxContinuation|TestInboxReviewExhaustedBudgetCannotPromiseContinuation|TestNativeApprovalBookkeeping'`
and `node docs/evidence/task-attention-check.cjs` after building the web app.
The browser fixture inspects 390px/1440px light/dark layouts, preserves drafts,
checks internal mode, and confirms one persisted-plan decision per approval.
