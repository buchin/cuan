## Observed transport gap

While checking a real customer reply, ordinary configured Owner MCP `get_contact`
returned a successful-looking `database_offline` object. Both contact tools used
ambient local PostgreSQL discovery rather than the configured Cuan API. This was
not evidence that production CRM was down or that the customer had no contact.
The isolated per-turn chat CRM snapshot is a separate existing transport.

## Correction and proof

Owner contact tools now read authenticated `/api/v1/contacts` and the existing
contact-memory endpoint through the configured client. They never fall back to
an ambient database. Current contact ID, conversation binding and unambiguous
guest identity are supported; email/name text cannot silently choose a record.
Filtering retains API order, limits declare whether more matches exist, notes
declare loaded/more state, and bounded/API/schema failures are errors rather
than empty directories or invented empty notes. Bound task sessions cannot
inherit the Owner client; their private snapshot behavior remains unchanged.

The first mocked test encoded its directory using the consumer's ContactSummary
struct. Real production then exposed a second mismatch: CRM's wire field is
`conversation_id`, whereas the normalized MCP snapshot uses `session_uuid`.
The adapter now normalizes this explicitly and rejects conflicting identities.
The regression fixture uses literal actual API JSON rather than the consumer
struct, so it detects that mismatch.

Targeted diagnostics tests pass for authenticated read-only transport, filtering,
limits, conversation resolution, ambiguous guest IDs, missing contacts, API and
note failures, pagination and task-bound isolation; existing private snapshot
tests also pass. The official local CLI was rebuilt from maintained source.

Authenticated production MCP then resolved room60fdacfc-5584-4968-9000-61e6213d6192
to contact49a12715-f8a6-4b3a-9019-b73838ed3889 at2026-11-09T03:39:19Z with
source=configured_owner_api, memories_loaded=true and memory_has_more=false.
No CRM record, customer message or external account was changed.

The official releaseb0d4f0d94ce361a493cb2924b7e86163f2cf91c2 completed at
2026-11-09T03:43:41Z with exact-SHA production health and38 migrations. Repeating
the same official MCP read after activation resolves the same contact and notes
state. The user's new public message67094169-3e6e-4590-86a0-9fdcbbf970a6 still has
its persisted visitor/delivered receipt after this deployment; no replay was used.
