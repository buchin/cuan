## Compact project overview

Projects use a40px search field, underlined Active/Archived filters with counts,
compact folder rows, task-state counts, latest project/task activity and a route
chevron. Blocked counts use neutral text: a task failure does not itself prove
that the owner must intervene. Working and review counts remain visible.
The contextual create action is40px on mobile and retains its desktop label.

An isolated build passes TypeScript/Vite. Fixture browser checks pass at360,
390,820 and1440px in light/dark themes: search/no matches, archive switching,
project task navigation, accurate state counts, create button dimensions and no
horizontal overflow. No tasks or provider requests are launched by these tests.
Screenshots and structured browser results accompany the exact check script.
