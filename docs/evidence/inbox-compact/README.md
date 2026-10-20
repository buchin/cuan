## Inbox compact controls

Archive is part of the conversation filter row, with a 44px touch target and accessible label. Read titles use normal weight; unread titles retain semibold. Row menus remain visible but quieter.

Archive/Restore uses a fixed Undo snackbar above mobile navigation, or bottom right on desktop. It dismisses after eight seconds, pauses for mouse hover, keyboard focus, pending requests or errors, and does not steal focus. Archived items remain available after dismissal.

Validation: isolated production frontend build; existing cross-page gesture/persistence regression; additional browser checks at 360, 390, 820 and 1440px for aligned filters, touch target, no overflow, fixed snackbar, focus pause and automatic dismissal. No provider calls or task launches.

Run: node docs/evidence/inbox-compact/browser-check.cjs
