# Studio simplification and mobile navigation

Studio preserves the existing heading and composer while removing the return link, agent label, project picker, preference banners, memory link and grounding chips. Text, attachment controls, setup/error recovery and draft persistence remain. Project deep links still pass their context without an extra choice.

Studio is a distinct main navigation destination. Mobile uses Studio, Inbox, Tasks, Projects and Account; the account popover provides Contacts and Settings. Shared mobile headers show the section icon rather than a brand shortcut. Detail back buttons and conversation identity remain intact; desktop headers do not repeat section icons.

Validation: production frontend build, existing affected source checks updated to the approved behavior, and `node docs/evidence/studio/browser-check.cjs`. The browser harness serves the built assets with fixture APIs, never production writes/provider execution. It checks 390px and 1440px in light/dark, reload draft retention, empty send disabled, response-loss retry with the same request ID, accepted conversation navigation, Inbox new-message return to Studio, mobile active navigation, Contacts/Settings access and Indonesian placeholder. Screenshots are written under `/home/dev/reports/cuan-studio-simple` (override `CUAN_VISUAL_OUTPUT`). Real screenshots inspected for whitespace, typography, low visual noise and overflow. This is not a live AI conversation E2E test.

## Compact account menu follow-up

The account popover now contains a native identity button and only Settings, Contacts and Sign out on both desktop and mobile. Removed role badge, category headings, direct password/language controls, keyboard shortcut entry and routine reload action. Circular avatars, theme colors, 44px touch targets, Escape focus return and outside dismissal are retained. Logout is a quiet secondary action; its behavior is unchanged. Profile, appearance/language and notification links reuse the existing personal settings definitions in the Settings index. Keyboard help remains accessible via `?` or Settings → General; the existing application error screen retains its reload recovery.

`CUAN_VISUAL_OUTPUT=/home/dev/reports/cuan-account-simple node docs/evidence/studio/browser-check.cjs` verifies the compact menu on 390px/1440px, both themes, exactly three actions plus identity, height under260px, touch targets, Escape/focus, Contacts/Settings navigation and reachable appearance/profile pages. Screenshots inspected directly. APIs are fixtures; no real logout or profile mutation occurs.

## Settings header back navigation

Settings and Account detail pages now use the shared header back link instead of a second back row inside the content. The link targets the immediate route parent, including integration subpages; it does not depend on browser history. Desktop uses the same header behavior without repeating breadcrumbs. The index normally shows the section icon. Valid task/conversation `returnTo` / `return_to` context travels through parent and integration links and is exposed as the index header back action. External return URLs are rejected. GitHub authorization still receives its original return context; sign-in behavior and credentials are unchanged.

Schedules empty state has one Create Schedule action and no surrounding dashed box. Populated lists retain New Schedule. Shared DetailLayout supports optional back content for pages whose header owns navigation. Embedded Agent Memory and integration pages no longer repeat return links.

Browser evidence: `CUAN_VISUAL_OUTPUT=/home/dev/reports/cuan-settings-header-back node docs/evidence/studio/browser-check.cjs`. Built frontend assets with fixture APIs, 390px and1440px, light/dark; inspected schedule/GitHub screenshots directly. Checked create dialog opens, empty/populated CTA exclusivity, direct integration deep links, GitHub→Integrations→Settings→original task, Live Chat/Resend parents, Account→index, and rejected external return URL. No production schedule creation, provider login or other external writes were performed.
