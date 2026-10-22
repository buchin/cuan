## Project repository tab and icons

Projects expose Settings, Repository and Activity as separate navigation links.
Repository lives at /projects/PROJECT_ID/repository and survives direct navigation
and reload. Default keeps its repository-free design and omits that tab.

Settings contains name, goal and guidance. Repository shows the selected/connected
repository and GitHub picker; manual URL and branch overrides stay in collapsed
Advanced. Leaving branch empty follows the repository default. Existing saved
connections and task workspace bindings are retained. General/repository drafts
persist across these two tabs and Save changes saves the pending project draft.

Project-list icons keep the folder base; GitHub repositories add a GitHub badge,
other repositories a Git badge, and repository-free projects have no badge. Labels
are accessible without depending on color. The global Projects icon is unchanged.

Clean production build and browser API fixtures passed at390/820/1440 in both
themes: route/reload, one active tab, draft retention, GitHub selection, default
branch, failed-save retention and confirmed save. Read-only actual production
browser verification passed at390/820/1440 on9ff1d46: saved URL/branch retained,
Advanced collapsed, Settings fields excluded and no overflow. No application
mutation or provider call was made. Initial browser navigation observation
attempts timed out; waiting for navigation commit and then the actual form
allowed the final checks to complete. A deploy terminal also timed out after
activation; independent release/readiness checks confirmed its successful state.

See browser.json, production.json and screenshots. To reproduce the fixture from
repository root: CUAN_RELEASE_ROOT=/path/to/clean/release node
 docs/evidence/project-repository-tab/browser-check.cjs.
