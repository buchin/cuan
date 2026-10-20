## Notification quick view

The bell opens an anchored native popover on desktop/tablet and a bottom panel
on phones, without leaving the current page. It previews up to six notifications
using the same rows as the full page. Mark all as read is directly visible in
both panel and full-page headers. View all opens the shareable full notifications
page, whose Back action returns to the originating route. Older browsers without
the native Popover API retain direct navigation to the full page.

Native light-dismiss and Escape close the panel. Heading focus announces the
panel and focus returns to the bell when dismissed from inside. Standard Tab
navigation is preserved; this is a nonmodal quick view. Loading, empty and retry
states are supported. Failed mark-read keeps unread state and an actionable error;
opening a destination still works when saving read state fails. Handled items do
not inflate badges. Links retain existing destination validation.

Clean-build fixture checks pass at390/820/1440 in both themes, including correct
placement, bounds, unchanged source URL, Escape/focus, immediate mark-all failure
and success, navigation failure recovery, View all/Back and Indonesian labels.
The full-page context/group/filter/time/read-state regression checks also pass.
See browser.json, full-page.json and the corresponding scripts/screenshots.
No provider or production data was used in these fixtures.
