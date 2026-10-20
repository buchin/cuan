## Mobile account navigation

Moved the mobile account trigger to the last bottom-navigation item, Account. The menu opens upward with Settings, Profile & password and Sign out. Removed header avatars from main pages and phone conversations. Conversation/session views retain their focused layout; Back returns to the list and bottom navigation. Desktop and tablet account placement remains at the sidebar bottom.

TypeScript/Vite build passed. Chromium fixture verified all five main pages at360,390,767px in light/dark; five navigation items, no header account trigger, viewport-safe popover, Escape/focus behavior, settings navigation and profile action. Sidebar account menus remained available at820/1440px. Existing task/session/inbox fixture checks also passed. No actual password change or sign-out was performed.

Production release verification follows after deployment.
