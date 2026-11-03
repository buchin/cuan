# Settings, contacts, and connected AI accounts

Settings pages share `DetailLayout` and `BackAction`: the app navbar owns the
page title. The body supplies a short description once. `GroupedSection` and
`GroupedRow` provide consistent form spacing and stack controls on narrow screens.
Search and tabs use `ListSearch`, `ListTabs`, and `ListTab`. Technical application
status is available in a `SlideDrawer`; memory capacity and learning rules expand
on demand. Profile roles remain on the profile page rather than every navbar.

## Where to configure services

- Settings → Integrations: GitHub, Live Chat, and Resend email delivery.
- Settings → Apps & Skills: connected app capabilities and the skills catalog.
- Account → Notifications: personal push preferences and quiet hours. Email
  delivery configuration links to its canonical integration page.
- General: application health, version/updates, installation, and shortcuts.
  Project configuration stays inside the project; routines have their own page.

Inbox has no permanent Live Chat settings shortcut. Its disconnected empty state
still offers Connect Live Chat. That onboarding link opens
`/settings/integrations/live-chat?returnTo=...` and returns to the originating
Inbox. Direct Settings navigation returns to Integrations. Only allowlisted
internal destinations can become return links.

## AI account management

The public assistant defaults to `@cuan`; its name remains editable. It is one
assistant supported by multiple explicit user-connected profiles. Installation
logins are compatibility state and are excluded from the connected-account list;
new OSS installs must connect their own accounts. No installation credential is
converted into a user credential.

Open an account row or its action menu to edit its display name and preferred
model. An empty model selects Auto. The owner can disconnect using a confirmation
dialog. Existing conversations and task history stay intact. An account currently
used by execution cannot be disconnected until that work finishes. Disconnect
checks profile ownership, takes an exclusive execution gate, cancels its scoped
sign-in sessions, and removes its private profile. BYOK disconnect explicitly
clears the encrypted saved key and disables that provider; an empty key in an
ordinary metadata edit continues to preserve it. Provider-side revocation remains
under the user's control.

Subscription email is shown when available. Google profile identity is read from
that account's OAuth envelope, including nested token files. Check connection can
refresh missing identity from Google's OIDC UserInfo endpoint with that private
profile's access token. Lookup is bounded to five seconds and never follows
redirects. Failure or missing email scope displays Email identity unavailable;
it does not invent an address or mark the connection verified. Other provider
identity formats are not inferred. Tokens and profile paths never enter the UI.

## Contacts

Contact counts appear compactly in the search control rather than a separate
row. Manual reload lives in the list action menu; load failures offer Try again.
A background refresh runs every 30 seconds while visible and on window focus;
it pauses during edits to preserve drafts. Contact details share the compact back
action and reading typography. Notes use flat rows with hairline dividers.
