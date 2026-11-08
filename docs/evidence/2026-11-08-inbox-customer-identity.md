# Inbox customer identity and read-only CRM

Observed room: `f299b6de-3470-4fd4-8b9f-30eba98402f8`. An owner asked to activate
access for “this user”; the assistant asked for an email already shown in the
header. The prompt only included message text and text-matched contact memory.
The person icon also linked the Inbox `cust_<session>` alias to Contacts, which
selects a canonical CRM contact UUID. These were two disconnected contracts,
not evidence that the visitor had no email or that an external account exists.

## Change and evidence

- The prompt includes the persisted contact bound to the current room. Explicit
  other targets still take precedence; old incidental names do not replace this
  customer's identity. A CRM email remains a lookup hint, never proof of external
  account ownership, payment or entitlement. Public contexts receive no snapshot.
- Contact list/detail responses include `contact_id`, preserving the existing
  conversation identity. The person action uses that canonical identity and is
  available only for a real persisted contact. Transient anonymous sessions still
  work, and reconnects do not merge identities solely by claimed email.
- Isolated AGY chat advertises `list_contacts`/`get_contact` using a fresh private
  per-execution CRM snapshot, in addition to public `web_fetch`. It receives no
  Owner token or DB credentials. `get_contact(current)` resolves the bound contact.
  Other contact details can be searched in the bounded directory; notes explicitly
  state whether fetched and whether older entries remain. Existing application
  memory search covers additional notes. Hidden DB/write tools remain rejected.
  Review, regenerate, reconciliation and other tool-free modes do not gain tools.
- Every execution has its own temporary snapshot and a read-only namespace mount.
  Concurrent rooms cannot replace each other's snapshots. Cleanup affects only
  that execution. Invalid/expired snapshots fail closed without native DB fallback.

Targeted API/prompt and MCP dispatch tests cover the contracts above. A real React
render with isolated local Owner-response fixtures was inspected at390 and1440px.
The person action opened the matching contact, showed email/notes/tags, and never
showed “Contact not found”. There were no page exceptions or horizontal overflow.
Fixtures do not activate access, send a customer message or contact production.

Screenshots: [mobile header](customer-identity/header-mobile.png),
[mobile contact](customer-identity/contact-mobile.png),
[desktop contact](customer-identity/contact-desktop.png).
Reproduce with the running Vite app and an installed Playwright runtime:
`PLAYWRIGHT_MODULE=/path/to/playwright CHROMIUM_EXECUTABLE=/path/to/chromium node web/tests/inbox-contact-navigation.cjs`.
Production deployment and actual agent-tool behavior remain pending at this
checkpoint; this evidence does not prove any Dojo account was changed.

## Operational completion failure in the same conversation

Task `msg_aafa00af-d8c1-4d88-8c90-dd6296b5d5cd`, attempt
`att_f51759429ab5f1d5c6c4d01d373a0465`, reported existing product access and an API
`has_access=true`, but native completion failed a missing source PHPUnit target.
That report is a worker claim, not this session's independent verification.
Its original scope only grants/verifies an existing account entitlement. The
classifier lacked the Indonesian access-operation phrases and defaulted to code.

Account operations now infer the operational lane while explicit source/schema
changes still require code. Native publication policy restricts their repository
work and publication, retaining the original saved project access for the operation.
Registered implementation targets do not enter these restricted claims. Tests
cover both a new operational task and an old code-default task with saved PHP
project targets and auto-deploy enabled; neither inherits those unrelated gates.

The existing bounded saved-evidence path can inspect a legacy account task rejected
solely for that native prerequisite failure after a final report and completed
reported criteria. It preserves the failure and verifies the existing external
outcome, with another activation explicitly forbidden. Ordinary completed account
operations use that same independent read-only observation before PM acceptance.
The native receipt binds the evidence launch and unchanged repository; a worker's
claim alone cannot pass the independent-outcome gate. Code changes, failing actual
assertions, cancellation and incomplete criteria cannot use the legacy recovery.
Production recovery and actual access remain unproven at this checkpoint.

## Production checkpoint

Release `5440ba10245086762315e849cddeadd18fb49204` is healthy on the exact
production SHA, with37 migrations. An initial activation-safety test stopped an
attempt before deployment because the readiness constant still expected36;
`5440ba10` aligned the release gate before the successful deployment.

A private, read-only owner whisper explicitly tested the real runtime in the
same room. Request `7e053722-0996-4588-95a9-f8641f986869` completed with native
`call_mcp_tool` input `cuan-inspect/get_contact {contact_id: current}` and a real
contact result in0.1seconds. Provider execution took27.2seconds. The final reply
identified the correct name/email; both probe and reply remained internal. The
CLI's30-second observation expired just before persisted completion; the same
request was inspected, never resent. The test explicitly forbade access changes,
new tasks/proposals and customer delivery. No such actions occurred in its trace.
This is a maintainer-directed tool test, not proof of autonomous task completion.

The activation task's third attempt, already started by the old release, remained
running at the last read (`att_9b26f069674ded2db9bddfbf6164a7c7`). It had restored
unrelated MCP source/tests in its isolated task context. No task was canceled,
manually retried, accepted or duplicated here. Its actual outcome and any required
scope correction still need observation before claiming the workflow is fixed.
The original login task's exact-source recovery is durably queued behind active
work. Neither task is claimed Done by this checkpoint.
