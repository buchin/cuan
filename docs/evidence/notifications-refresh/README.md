## Contextual notification feed

Notifications now show conversation/project/contact context, a specific event
or work title, a short status, relative time and an explicit destination label.
The list uses compact rows with category icons, restrained unread indicators,
Today/Yesterday/Earlier groups, and a full-date tooltip. Long titles are bounded
rather than expanding every row. English and Indonesian copy are supported.

Opening a row navigates immediately, even when saving its read flag fails. The
flag remains unchanged on failure and feedback remains available on returning to
Notifications. Navigation itself does not approve, retry or execute work.

Existing IDs and read flags are retained when metadata refreshes. Approved or
deferred proposals, superseded task alerts, active retries and recovered replies
no longer count as unresolved unread alerts. Historical rows stay available in
All. Recovery in progress is distinct from success: a later failed attempt remains
an active alert rather than being labelled recovered. Already-resolved events do
not create new notification noise or push delivery. Lock-screen push text remains
generic; the in-app feed does not include raw message or plan-body previews.
Context/title masking covers the existing known credential patterns.

## Verification

- Targeted memory-store tests: contextual proposal state; approved/deferred
  reconciliation; in-progress, failed and completed retries; preserved read flags;
  no new resolved-event noise; task revision/completion reconciliation; existing
  push dedupe, quiet-hours and device-expiry checks. Passed in0.273s.
- Real disposable PostgreSQL: approval and deferral, safe context, no raw message
  previews, pending/failed/completed retry states, and an old retained alert behind
  more than100 newer recovered attempts. Passed in0.81s; database dropped. No
  production data writes or paid model calls.
- Built frontend with fixture APIs at390/820/1440 in light/dark themes: context,
  status and destination labels; compact day groups; relative/full time; bounded
  long titles; resolved events excluded from Unread; successful read updates;
  failed read writes still navigate; Indonesian actions; mark-all-read; no
  horizontal overflow or browser exceptions. See browser.json and screenshots.
- TypeScript/Vite build and diff whitespace checks pass. Existing bundle/CSS
  build warnings are unchanged. No schema migration is introduced.

The source for the SQL test is notifications_context_test.go. It requires an empty
`cuan_notification_feed_*` database via CUAN_NOTIFICATION_TEST_DATABASE_URL.

## Production delivery

Release5961b4a94f7e7deb07ddd589bb70f993bd8ad8ef is active at
https://cuan.dojo.cc, schema25, auto-migration disabled. Private backup:
`/opt/cuan/backups/notifications-context-release-20261020`.

Read-only production API and browser checks pass at390/820/1440 in both themes:
contextual metadata, no old generic reply-failure title, relative times/full-date
tooltip, unread filtering and no horizontal overflow. The feed contains11 items,
9 with refreshed contextual metadata; older unknown records retain fallback UI.
No notification was clicked, marked read, retried or approved in production.
No model call or non-login API write was made. Reading the feed refreshes derived
notification metadata in its existing preference file.

Actual rollback to49f89ec and reactivation of5961b4a both passed without schema
changes. All37 application tables (auth sessions excluded) and878 provider profile
files remain unchanged. All11 existing notification read flags and non-feed
preferences are preserved; IDs are unique and retention remains100. Production
screenshots containing actual work titles remain private; published screenshots
use fixtures only. See production.json.
