# Studio branding

Reused the existing vector Cuan kepeng mark for Studio navigation (desktop, collapsed sidebar, and mobile) without repeating the logo above the greeting. Kept Studio labels. The navigation mark inherits active/inactive colors and is decorative to avoid duplicate screen-reader labels; standalone brand marks retain their accessible name.

TypeScript/Vite build passed. Production-build browser fixtures passed at360/390/820/1440px in light/dark mode, including no duplicate greeting logo, decorative navigation SVG, draft persistence, safe retry and Inbox compose routing. The CTA is outside the input frame, right aligned below it with16px spacing. Help text sits on the left on desktop and on its own row on mobile; minimum44px target and no overlap. The resize handle is removed. No provider calls. Screenshots and `browser.json` are included. Existing build CSS/bundle warnings remain.

## Production

Released `97c5f5ebf018b15a808dab9835cf56ab55bcd7df` using the native atomic release workflow. Public browser checks passed at360/390/820/1440px in light/dark: Studio navigation uses the decorative kepeng, greeting has no repeated logo, textarea has no resize handle, CTA is at least44px tall and aligned bottom right below the editable area, and Inbox compose still routes to Studio. Served JS/CSS/fonts match the built assets. No goal submissions or provider executions. Readiness healthy, schema23 unchanged, all35 retained application tables and706 profile files unchanged (login sessions excluded).


Follow-up: external CTA layout replaces the earlier internal composer action. Build and browser checks are rerun for this change.

External CTA follow-up released as `4e040004c466e8dd6e2acf3dc9ba63d2fc937ae5`. Live browser checks passed all eight viewport/theme combinations, explicitly checking the button is outside the textarea frame, aligned right and at least12px below the editable area. Exact build assets and readiness passed;35 retained application tables and706 profile files remain unchanged. No live provider requests.

Mobile layout follow-up: full-width CTA exactly12px below input, centered help text below CTA, greeting near the top instead of vertically centered. Desktop keeps right-aligned CTA and left help text. Build and eight fixture viewport/theme cases passed, including width, gap, compact heading position and no overflow.
