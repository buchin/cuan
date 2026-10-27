## Contextual list creation

Production9fecf67 uses the shared ListCreateAction for desktop Projects, Contacts
and Inbox, above each list's search area. The action remains outside the scrolling
list. The global action is hidden on desktop while the list action is available;
Inbox restores its global action when its sidebar is closed. Below1200px the
list-header action remains in the page header, with labels matching its context.
Project/contact detail views omit mobile creation actions.

Actual production browser checks passed for all three pages at390/820/1440:
exactly one visible action, expected desktop/mobile location, working project and
contact dialogs or Studio navigation, no overflow/page errors. Post-login API
writes were blocked; no production inference was invoked. See production.json.
