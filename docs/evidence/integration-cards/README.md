# Integration cards verification

Clean build f03131e passes fixture browser checks at 360/820/1440 in light/dark
themes. Mobile cards stack and wider screens use two columns. GitHub handles
connected/not-connected/setup-needed/status-error with retry; Live Chat presents
an installation guide without claiming a verified connection. Detail navigation
works and no horizontal overflow or browser errors were observed. No provider
calls, messages, task launches or live GitHub authentication were submitted.

Production 0eb1aea passed read-only browser/asset verification, actual rollback/reactivation and retention: schema25, no automatic migrations, 37 unchanged application tables, 930 unchanged profile files, seven unchanged item states and eleven unchanged notification read flags. OAuth reports Setup needed because no OAuth App is configured. Manual tokens remain under Advanced; no real OAuth authorization is claimed.
