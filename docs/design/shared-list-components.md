# Shared list and header components

Project, Tasks, Contacts, and Inbox share the same control proportions. Use these
components instead of copying local markup:

| Component | Location | Contract |
| --- | --- | --- |
| PageHeader / PageTitle / PageHeaderBrand | components/layout/PageHeader.tsx | 56px mobile, 52px desktop; 16px title; 28px brand |
| IconButton | components/ui/IconButton.tsx | 44px touch target, 18px glyph, label and keyboard focus |
| ListSearch | components/layout/ListControls.tsx | 44px height; 16px mobile input, 14px desktop; muted surface |
| ListTabs / ListTab | components/layout/ListControls.tsx | Quiet underline, plain counts; arrows/Home/End and roving focus |
| ListRowContent | components/layout/ListControls.tsx | 14px title, 12px metadata, consistent spacing |
| ActionMenuItem | components/ui/ActionMenuItem.tsx | 44px menu row and shared typography |

Contextual create icons can differ by meaning; their glyph size and touch target
must match. Secondary details belong behind disclosure or in a drawer. Shared
headers must not be overridden by substring selectors such as `[class*="h-13"]`;
that also overrides responsive classes on mobile. Theme channels are OKLCH: use
`oklch(var(--background))`, never HSL with those channels.

Task details use a flexible-height heading so long titles wrap. The overview
explains what is happening and what to do next before showing technical history.
Do not make a globally pinned terminal the default task entry.

For every change, render real 390px and desktop screenshots and inspect loaded
content, typography, spacing, overflow, keyboard interaction and disabled states.
Automated source checks alone are insufficient. See design_guidelines.md §7.
