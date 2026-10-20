# Studio branding

Reused the existing vector Cuan kepeng mark for Studio navigation (desktop, collapsed sidebar, and mobile) without repeating the logo above the greeting. Kept Studio labels. The navigation mark inherits active/inactive colors and is decorative to avoid duplicate screen-reader labels; standalone brand marks retain their accessible name.

TypeScript/Vite build passed. Production-build browser fixtures passed at360/390/820/1440px in light/dark mode, including no duplicate greeting logo, decorative navigation SVG, draft persistence, safe retry and Inbox compose routing. The CTA is inside the input frame at bottom right, in a separate row below the textarea; minimum44px target and no overlap. The resize handle is removed. No provider calls. Screenshots and `browser.json` are included. Existing build CSS/bundle warnings remain.
