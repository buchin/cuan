## Project and task workspace refresh

Projects now use one contextual header with New task scoped to that project.
Settings/archive move into Project actions. Task sessions have one back action,
a two-line title, consistent status wording, and session numbers in Detail.
Project rows name their task count and show attention counts from actual data.
Task rows show bounded titles, status and latest update. No backend status is changed.

Finished sessions open with the result. Original request and verification are
expandable on the same page, with no nested vertical scrolling. Composer stays
outside the result scroll area. Result reading size is14px; input remains16px on
mobile to avoid browser zoom and touch targets remain at least44px.
Task/session routes remain independently addressable. `?panel=detail` opens the
technical drawer after reload; Copy link copies that URL and explains that
workspace access is still required. Links do not grant public access.

Clean build excludes unrelated extension WIP. Fixture browser checks pass at
390/820/1440 in light/dark: contextual creation/preselected project, one back action,
result-first layout, collapsed untruncated request, truthful review status, single
page scroll, composer bounds, drawer link reload/copy/close, return navigation and
no horizontal overflow. Active terminal remains visible; simulated completion
switches automatically to the saved result without another task launch. No real
provider was called. See browser.json and browser-check.cjs.

Existing CSS syntax and bundle-size build warnings remain separate from this change.
Production release evidence is recorded separately after activation.
