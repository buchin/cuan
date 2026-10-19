# Task list clarity and original-request review

Production: https://cuan.dojo.cc  
Release: `b92470e683acd671a216a50100e2494bf9baca17`

## Changes

- Desktop task table has clearer column proportions, subdued headers, consistent
  row spacing, agent initials, aligned update times and status tabs.
- Search is bounded rather than stretching across the full desktop toolbar.
  Sort has an explicit44px height and custom icons to avoid a small native
  select appearance. Filter/sort/search state remains in the URL.
- Removed redundant visible All-count summary, empty project labels and process
  Finished labels beneath workflow status. Session details retain process state.
- When a title differs from the original instructions, a Request preview keeps
  the original first line visible on desktop, tablet and phone.
- Approval dialog shows the original request and directs reviewers to check
  scope, correct output workspace and verification. No extra approval field or
  checkbox added; existing acceptance/Done gates remain.
- Session guidance explicitly prohibits inventing deliverables through title
  refinement/checklists and requires comparing the final result to original
  instructions, scope and workspace. Passing tests alone is not acceptance.

## Verification

- TypeScript/Vite build passed.
- Targeted task-agent scoped reporting/review-gate test passed.
- Deterministic browser check passed eight viewport/theme combinations:
  360/390/820/1440px in light/dark, no horizontal overflow, original request
  visible, no duplicate Finished state, sort44px, search/sort/filter retention,
  mobile sort and original-request approval context.
- Production authenticated checks passed the same eight combinations, exact
  deployed assets, preserved filters and original request in approval.
- Production smoke used one synthetic task and one finished synthetic attempt;
  both cleaned. No result was approved, no run-agent request/provider call.
- Native atomic deployment passed health/readiness. Private backup validated
  before release with no active task/inbox work.
- Post-smoke retention:35 database tables and631 account-profile files unchanged
  (authentication sessions excluded). Schema23, auto-migrate false; no migration.
- Production project checkout fast-forwarded from29770b2 tob92470e.

## Limits and rollback

Intent alignment remains an agent instruction and human review responsibility,
not an automated semantic correctness guarantee. No live provider behavior was
tested for this guidance change. Existing human instructions, generated titles
and historical results were not rewritten.

Use the documented native previous-release rollback, retaining schema23.
