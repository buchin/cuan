# Install Cuan, edit your profile and manage notifications

## Install on your home screen

Open **Settings → General → Install Cuan**. Supported browsers show **Install app** when ready. Otherwise use the browser's Install app/Add to Home Screen menu. On iPhone/iPad, use Safari → Share → Add to Home Screen → Add, then open Cuan from that icon. Installation requires HTTPS (localhost is allowed for development). The installed app opens Studio in its own window with the Cuan kepeng icon.

Cuan needs a network connection. Its service worker does not cache account pages, APIs, messages, attachments or tokens. If the server cannot be reached, navigation shows a generic offline page with a retry link. Browser support and the actual system installation dialog depend on the device. New service-worker versions activate without reloading an unsaved form; there is no custom offline data cache.

## Profile and sign-in

Open **Account → Profile & password**, or **Settings → Profile**.

- **Display name** is the name shown in your account menu. It does not change your login. Clear it to use your username.
- **Username** is your login name: 3–32 ASCII letters, numbers, dots, underscores or hyphens, normalized to lowercase. Enter your current password to change it. Use the new username next time you sign in; email sign-in still works.
- **Change password** opens the existing password-change dialog.

Cuan is currently a single-owner application. Username edits update that owner rather than creating another account. The account ID, current sessions, project ownership and agent connections stay the same. Display names and notification state live in private owner preferences; usernames remain in the existing owners table. No schema migration is required.

## Notifications inside Cuan

The bell in the page header opens **Notifications**. It shows up to 100 recent notifications and links to the originating conversation or work page. The secondary navigation below the header provides **All** and **Unread**, including the unread count. The **⋯** header menu contains **Mark all as read** and **Notification settings**. Filter selection survives reload through the URL. Read state is saved on the server and shared across your browsers.

Notifications cover concrete approval requests, work ready for review, blockers/failed agent responses, completed work, and incoming customer messages. Owner replies are excluded from customer-message notifications. Routine progress does not create notifications. The center refreshes every 30 seconds while visible and when the page becomes visible. History reflects states observed by the notification collector, not a full audit log of every transient status.

When an agent presents a concrete plan that needs approval, it can emit a versioned `cuan-approval` marker after the visible plan. Cuan removes the marker from the displayed response, shows **Needs approval**, and adds a notification. The marker never grants approval, creates tasks, or runs work; authenticated user approval and existing delegation rules still apply. Ordinary questions and clarification should not use the marker.

## Optional push notifications

Open **Settings → Notifications** and select **Enable on this device**. Only this click requests browser permission. On iOS/iPadOS, first install Cuan to the home screen and open that installed app (Web Push requires a supported OS/browser). Unsupported browsers continue to use the notification center.

Choose which categories can reach your devices. **Quiet hours** use the saved IANA timezone and whole hours; overnight ranges are supported, and start/end must differ. Quiet hours pause push only; updates remain inside Cuan. **Disable on this device** removes its subscription. Signing out also unsubscribes that browser. Browser/site permission can be changed in browser settings.

Push is encrypted Web Push using persistent application-generated VAPID keys; no operator token setup is required. Only the public key is returned to the browser. The server polls every 30 seconds, sends up to five eligible updates per device per pass, deduplicates successful deliveries, retries temporary failures, and removes endpoints returning 404/410. New devices do not receive historical events; unread updates older than 24 hours are skipped. Up to ten devices are supported. A push service accepting a message does not guarantee OS delivery, which also depends on connectivity and device settings.

Lock-screen notifications contain only a generic Cuan update, never project titles or message contents. Tapping opens the appropriate page and requires sign-in if the session expired. Categories and quiet hours affect push, while the in-app center retains all categories.

## Mobile navigation and attachments

The inactive Account avatar is neutral. Only the current destination uses the active accent; opening the Account popover does not make it a second selected destination. Indonesian navigation keeps the familiar short label **Inbox**.

On mobile, **+** opens **Camera**, **Photos**, and **Files**. Camera requests a rear-camera capture through the native file picker; unsupported devices fall back to their normal picker. Photos accepts supported image formats without requesting camera capture. Files accepts the formats documented in the attachment guide. Desktop opens the file picker directly and retains paste/drag-and-drop. Device camera/gallery behavior is provided by the browser; no camera or notification permission is requested just by loading Cuan.

## Operations, backup and rollback

Back up the database, persistent attachments, and the service user's private `$HOME/.local/share/cuan/preferences/` directory. Preferences include display names, notification history/read state, category/quiet-hour settings, private VAPID keys and push subscription endpoints; keep this folder private. Atomic writes preserve a valid previous file if a replacement fails. Corrupt files produce an error instead of silently discarding settings.

Routes require owner authentication: `GET/PATCH /api/v1/profile`, `PATCH /api/v1/auth/username`, `GET/PATCH /api/v1/notifications`, `GET/PATCH /api/v1/notifications/preferences`, and `GET/POST/DELETE /api/v1/notifications/push`. Sign-in identity changes require the current password. Public assets are `/manifest.webmanifest`, `/sw.js` and `/icons/cuan-*.png`; the service worker is served with no-store and root scope.

Deploy through the native atomic release script with idle checks, private backups, checksums and readiness validation. No database migration is introduced. Rolling back the binary preserves usernames and preference files, although older binaries do not expose the new settings or push worker. To fully retire an installed service worker during an emergency rollback, serve a compatible replacement at `/sw.js` that unregisters itself; keep this path available until clients update. Never delete private preferences or rotate VAPID keys merely to roll back application code.

## Homepage and install suggestion

Anonymous visitors see the public homepage at `/`; Get started opens sign-in. Signed-in visitors to `/` or `/login` go to Studio. `/about` previews the homepage while signed in.

Studio offers a small Install Cuan suggestion when the browser exposes an install prompt, or on iPhone/iPad with manual instructions. The button opens the native install dialog where supported. On iOS, follow Safari → Share → Add to Home Screen → Add. Dismissing the suggestion hides it for seven days in that browser. Standalone installations hide it; browser detection cannot reliably identify an app installed in a different browser. Installation remains available in Settings. Loading Studio never requests permissions automatically.

The permanent **Settings → General → Install Cuan** section remains available after the Studio suggestion is dismissed. The Studio suggestion also links directly to `/settings#install-cuan`. Settings provides ordered instructions for iPhone/iPad, Android, Safari on Mac, and other desktop browsers, or a native Install Cuan button when available. Standalone mode displays Cuan is installed.

Studio uses compact mobile spacing and a 144px initial goal field (176px on desktop). The field grows with text up to 320px, then scrolls internally. The submission button stays immediately below the attachment area; no extra helper footer separates it from the rest of the page. The complete goal block is vertically centered when space permits; the optional installation suggestion follows below the first viewport. Short viewports and the on-screen keyboard can scroll the content.

## Contacts directory

Search contacts by name, email, source or tag. Open **Filters** beside search to select a tag/source; an indicator remains when collapsed filters are active. Clear filters to restore all results. Contact rows show name, email, source and a short tag summary; the update date lives in the detail panel. Select a contact to manage tags or open its connected conversation. English and Indonesian labels follow Settings → Appearance. Contact names, emails, product names and tags keep their original content.
