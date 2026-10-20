## Notification filter-row action

Full-page Mark all as read now sits to the right of All/Unread. Phones use the
check-check icon with an accessible label and tooltip; at768px and above the text
is visible too. The action has a44px minimum target, busy state and disables when
no unresolved unread items remain. It no longer competes with the page title in
the application header. The header actions menu retains notification settings.
The desktop quick-view panel keeps its existing action in the panel header.

Clean TypeScript/Vite build passes, excluding unrelated extension WIP. Targeted
fixture browser checks cover360/390/820/1440 in both themes: full untruncated title,
right-aligned filter-row action, icon/text breakpoint and touch target, context,
relative times, date groups, unread filtering and resolved-state exclusion.
Mark-read failure preserves unread state and does not block destination navigation;
success updates state, all-read disables the action, and Indonesian labels pass.
See browser.json, browser-check.cjs and dark-360-read.png.

Existing CSS/bundle warnings remain unchanged. Production delivery evidence is
recorded after activation. No provider calls or production mark-read actions are
part of verification.
