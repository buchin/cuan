# Compact notification navigation

Notifications now has a dedicated secondary navigation directly below the shared page header: All and Unread (with count). Filter selection is encoded in the URL and survives reload. The header overflow menu contains Mark all as read and Notification settings. The introduction and multi-row action toolbar were removed; the redundant notification bell is hidden on this page.

Production TypeScript/Vite build passed. Targeted browser fixture passed at 360/390/820/1440px, light/dark, plus Indonesian mobile: first list row within160px of the viewport top,44px secondary navigation and menu targets, unread count/filter persistence, menu keyboard arrows/Home/End/Escape, focus return, outside close, failed mark-read preserving unread state, successful mark-all clearing counts, disabled action at zero unread, settings navigation and no horizontal overflow. No provider or production writes in fixtures.
