## Compact mobile creation actions

A shared creation button now renders a 44×44px contextual icon below 768px: compose for New message, person-plus for New contact, folder-plus for New project, and plus for New task. Accessible labels/tooltips retain full action names. Desktop/tablet keep plus and text. The phone conversation header reuses the same component.

TypeScript/Vite build passed. Chromium fixture verified four page actions at 360/390/767/820/1440px in light/dark: target dimensions, icon/text visibility, accessible names, every creation dialog and Escape dismissal, no horizontal overflow, and conversation-header action. No provider calls or actual creation submissions.

Deployed `345051604ed7f6810b647ae466cbe8917bf8c23f` to https://cuan.dojo.cc through the native atomic release. Authenticated production checks passed all four creation actions at360/390/820/1440px, including icon/text visibility,44px mobile width and opening/dismissing dialogs without submitting them. Exact assets matched the build; existing eight task result/navigation viewport/theme checks also passed. Header-only screenshots exclude conversation content.

Readiness healthy, private backup retained.35 existing tables and706 profile files unchanged (login sessions excluded), schema23/no migration. Synthetic task fixtures cleaned; no real provider calls, new messages or submitted creation forms. Persistent project checkout fast-forwarded to deployed code. See `production-browser.json`.
