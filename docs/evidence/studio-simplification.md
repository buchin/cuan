# Studio simplification and mobile navigation

Studio preserves the existing heading and composer while removing the return link, agent label, project picker, preference banners, memory link and grounding chips. Text, attachment controls, setup/error recovery and draft persistence remain. Project deep links still pass their context without an extra choice.

Studio is a distinct main navigation destination. Mobile uses Studio, Inbox, Tasks, Projects and Account; the account popover provides Contacts and Settings. Shared mobile headers show the section icon rather than a brand shortcut. Detail back buttons and conversation identity remain intact; desktop headers do not repeat section icons.

Validation: production frontend build, existing affected source checks updated to the approved behavior, and `node docs/evidence/studio/browser-check.cjs`. The browser harness serves the built assets with fixture APIs, never production writes/provider execution. It checks 390px and 1440px in light/dark, reload draft retention, empty send disabled, response-loss retry with the same request ID, accepted conversation navigation, Inbox new-message return to Studio, mobile active navigation, Contacts/Settings access and Indonesian placeholder. Screenshots are written under `/home/dev/reports/cuan-studio-simple` (override `CUAN_VISUAL_OUTPUT`). Real screenshots inspected for whitespace, typography, low visual noise and overflow. This is not a live AI conversation E2E test.

## Compact account menu follow-up

The account popover now contains a native identity button and only Settings, Contacts and Sign out on both desktop and mobile. Removed role badge, category headings, direct password/language controls, keyboard shortcut entry and routine reload action. Circular avatars, theme colors, 44px touch targets, Escape focus return and outside dismissal are retained. Logout is a quiet secondary action; its behavior is unchanged. Profile, appearance/language and notification links reuse the existing personal settings definitions in the Settings index. Keyboard help remains accessible via `?` or Settings → General; the existing application error screen retains its reload recovery.

`CUAN_VISUAL_OUTPUT=/home/dev/reports/cuan-account-simple node docs/evidence/studio/browser-check.cjs` verifies the compact menu on 390px/1440px, both themes, exactly three actions plus identity, height under260px, touch targets, Escape/focus, Contacts/Settings navigation and reachable appearance/profile pages. Screenshots inspected directly. APIs are fixtures; no real logout or profile mutation occurs.
