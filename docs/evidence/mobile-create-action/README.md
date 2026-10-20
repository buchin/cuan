## Compact mobile creation actions

A shared creation button now renders a 44×44px contextual icon below 768px: compose for New message, person-plus for New contact, folder-plus for New project, and plus for New task. Accessible labels/tooltips retain full action names. Desktop/tablet keep plus and text. The phone conversation header reuses the same component.

TypeScript/Vite build passed. Chromium fixture verified four page actions at 360/390/767/820/1440px in light/dark: target dimensions, icon/text visibility, accessible names, every creation dialog and Escape dismissal, no horizontal overflow, and conversation-header action. No provider calls or actual creation submissions.

Production verification follows release.
