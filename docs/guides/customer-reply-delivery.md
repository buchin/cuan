# Customer reply delivery

Accepted work in a customer conversation should leave the owner with a usable
reply, alongside its investigation or implementation report. Cuan prepares that
reply from the accepted findings in a separate, tool-free Inbox turn. This does
not reopen the task, create an execution attempt, deploy changes or contact the
customer. Existing accepted results without a draft use the same reconciliation.

The owner sees an internal **Customer reply draft · Not sent yet** attachment.
**Review & Use** fills the public reply composer for editing; sending remains a
separate owner action. Internal task links and investigation notes stay outside
the draft. A completed investigation must not become a claim that a repair was
published. The frontend never fabricates a reply from a Done/Review label.

Delivery is bound to the accepted attempt, current evidence digest, original
owner authorization and customer room. Changed evidence, cancellation, a different
owner or a nonaccepted result prevents generation. The server does not expose
project environment snapshots to this turn. Capacity waits retain the same saved
request; a failed/interrupted or unusable delivery gets at most one further
delivery turn. Neither turn can launch implementation work. Prepared text is
retained so uncertain message persistence does not regenerate a different reply.
Only a persisted agent reply counts as delivery; preparation alone is insufficient.

Candidate queries are bounded and use raw task/attempt/activity identifiers.
The final application still validates the exact accepted review and original
authorization. No historical task status, report or assistant message is rewritten.

Verification covers memory and disposable PostgreSQL lifecycle, original task
and attempt preservation, owner isolation, cancellation, duplicate scans and
uncertain persistence. `node docs/evidence/customer-reply-delivery-check.cjs`
renders the actual frontend at 390px and 1440px and verifies that Review & Use
fills the composer without sending a customer message.
