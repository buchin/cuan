# Contacts and live chat

Open Contacts from the sidebar or phone bottom navigation. Contacts is a CRM
directory, separate from Settings → Agents and widget installation.

## Browse and organize contacts

Visitors are saved when they open the live chat widget. Passive page views do
not create contacts. Search by name, email, product, or tag; filter by tag or
source. Clear filters restores the complete list.

Select a contact to view its identity, source, last update, tags, and connected
conversation. Updated is a saved timestamp, not an online indicator. On phones
and tablets, details replace the list; Back to contacts returns to it. Wide
desktops (1200px and wider) show a320px contact-list sidebar beside a spacious detail panel. The list and detail scroll independently. Search and filters remain in the list; returning from detail preserves them. `/contacts?contact=ID` supports direct links and reload.

Choose Open conversation to reply through Inbox. Manual contacts do not offer
this action because they have no connected messaging channel.

## Create a manual contact

Select **New contact** at the top right. Enter a name and optional email, then
Save contact. The saved contact opens immediately and can be tagged.

Manual contacts have the source Manual. Saving one does not send a message or
email, verify identity, merge contacts with matching emails, or establish a chat
channel. They do not appear in the New message recipient picker. Editing an
existing contact's name/email is not yet supported.

Authenticated API: `POST /api/v1/contacts` with JSON:

```json
{
  "request_id": "7e1e62ee-98cc-4676-894a-0b24815b598c",
  "name": "Customer name",
  "email": "customer@example.com"
}
```

Generate a new UUID for each new contact; retain it when retrying the same save.
Names are trimmed and limited to 120 characters. Email is optional and validated
when present. Identical retries return the same contact (201 on creation, 200 on
retry). Reusing a key with different identity data returns 409. The form keeps
the key after a connection failure. Responses contain `contact`, including
`is_manual: true`. Creation does not create a room, message, or provider call.
The `manual_` visitor namespace is reserved and rejected by public widget init.

## Manage tags

Select a tag chip to remove it, or type a tag and press Enter/Add. Existing tags
are suggested while typing. Limits: 20 tags per contact and 40 characters per tag;
case-insensitive duplicates are ignored.

Changes are sent only when Save tags is selected, including any tag still in the
input. All tags may be removed. Save errors retain the draft. Discard changes
restores saved tags. Drafts survive switching contacts within the page, but not
reload or navigation away. Tags support segmentation; they do not represent
broadcast consent. Broadcast is not available yet.

## Install the widget

Open **Settings → Integrations → Live Chat** for installation code, identity
options, and checks. Place the script before your closing `body` tag:

```html
<script src="https://your-cuan.example/chat/widget.js"
        data-product="your-website" defer></script>
```

Use your Cuan installation's origin. `data-product` separates visitor identities
by product. The browser stores a random guest ID and a session for each Cuan
origin/product. Reload resumes a registered session. Clearing browser storage
creates a new anonymous identity.

### Optional email identifier

Set `data-email` and `data-name` from the signed-in user, using your framework's
HTML escaping. Alternatively, after the widget is available:

```js
await window.$dojoChat.identify({
  email: 'customer@example.com',
  name: 'Customer name'
})
```

The existing Dojo shim supports open, close, toggle, setMessage, and startTopic.
The Crisp compatibility shim accepts queued and later identity updates:

```js
window.$crisp = window.$crisp || []
window.$crisp.push(['set', 'user:email', ['customer@example.com']])
window.$crisp.push(['set', 'user:nickname', ['Customer name']])
```

Identify before opening chat prepares metadata; afterward it updates the same
browser/product contact. Empty values preserve saved metadata. Email is normalized
and validated on the server but remains unverified, visitor-provided data.
Matching emails do not authenticate visitors, merge devices, or grant history
access. Safe logout/account separation on shared browsers is not implemented;
do not use widget identity as an authentication boundary.

## Persistence and limits

PostgreSQL stores contacts, visitor/session identities, email, and tags. A visitor
session UUID is a secret capability for its conversation; do not publish it.
Visitors cannot select internal or agent rooms through the session payload.
Internal contact list/create/tag APIs require owner authentication. The deployment
is single-owner; product labels do not provide tenant isolation. The directory
currently loads all contacts; server pagination is needed for large datasets.

Older rooms remain accessible through Inbox. They are not guessed or backfilled
into CRM identities from room titles. Broadcast, subscription consent, cross-device
deduplication, verified email, and contact merging are not implemented.

## Deploy, verify, and roll back

Schema22 (`000022_contacts_crm`) adds the CRM table/index. Manual creation and the
English UI require no additional migration. Back up the database and account
profiles, check that Inbox/task execution is idle, and validate the backup before
restarting. Keep production auto-migration disabled. Test migrations against an
isolated staging schema and apply them explicitly using the standard migrator.

The native deployment procedure uses `scripts/fural-deploy.sh`, a release lock,
private backup, checksums, atomic activation, readiness checks, and rollback.
For an application-only rollback, preserve schema22 and contact data; do not run
a down migration or restore the database. Installation-specific commands and
health/rollback details are in [the operations guide](message_task_lifecycle.md).

Smoke checks: responsive list/detail, tags and retry, conversation links, widget
identity/reload, manual save/reload, and same-key retry without duplicates.
Fixture checks are in `docs/evidence/english-sidebar/browser-check.cjs` and
`widget-check.cjs`; earlier CRM and production evidence remains in
`docs/evidence/contacts-crm/`, `crm-patterns/`, and `context-new/`. Use isolated
fixtures or exact synthetic records for write checks; preserve real contacts.
