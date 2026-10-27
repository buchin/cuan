## Mobile account navigation

Moved the mobile account trigger to the last bottom-navigation item, Account. The menu opens upward with Settings, Profile & password and Sign out. Removed header avatars from main pages and phone conversations. Conversation/session views retain their focused layout; Back returns to the list and bottom navigation. Desktop and tablet account placement remains at the sidebar bottom.

TypeScript/Vite build passed. Chromium fixture verified all five main pages at360,390,767px in light/dark; five navigation items, no header account trigger, viewport-safe popover, Escape/focus behavior, settings navigation and profile action. Sidebar account menus remained available at820/1440px. Existing task/session/inbox fixture checks also passed. No actual password change or sign-out was performed.

Deployed code `e899d4c70fd7450192e647a29edbc60937f797ec` to https://cuan.dojo.cc through the native atomic release. Production UI checks passed at360/390/820/1440px: mobile account is in bottom navigation, no header avatar, menu stays within viewport and existing desktop/sidebar actions remain available. Existing eight light/dark task-result/navigation smoke cases and exact asset comparison passed. Account identity is masked in production popover screenshots.

Validated private backup retained, readiness healthy, schema23 unchanged/no migration.35 existing tables and706 account-profile files unchanged (login sessions excluded). Synthetic smoke tasks deleted; no provider executions, password changes or real sign-out. Persistent project checkout fast-forwarded cleanly to the deployed code. See `production-browser.json` and production screenshots.
