## Account models, quota and response details

The account editor loads the installed provider catalog after sign-in, preserves
saved model IDs absent from a refreshed catalog, and offers retry on discovery
failure. No manual model entry is required. Provider usage displays remaining
quota, reset and last-check times for installed models only; missing data stays
unknown. Positive quota is not connection verification. CommandCode account
connection and reconnect remain separate unfinished work.

Response activity details open in the shared right-hand SlideDrawer on desktop
and a full-width closable panel at390px. This is an overlay, not a docked fourth
column. The conversation does not grow when details open. Queue cancellation
and stopping a running reply have distinct labels; backend confirmation behavior
is unchanged. A second redundant capacity explanation was removed.

Verification: production frontend build; targeted quota/recovery tests; browser
fixtures at390/1440px with100 timeline events, panel open/close and stable chat
position, catalog success/error/retry, saved selection retention, unsigned account
catalog prevention, usage meter and obsolete-model exclusion. Screenshots were
visually inspected. These are fixture UI checks, not real provider authorization
or completed native work.

Reproduce from repository root:

```sh
(cd web && npm run build)
node docs/evidence/accounts/model-picker-check.cjs
```

Artifacts: `/home/dev/reports/cuan-account-models/`.

## Settings master–detail and message boundaries

Desktop Settings now keeps a compact category navigation beside its bounded
reading canvas and opens General at the index. Mobile keeps category-list then
single-detail navigation, with the existing header back action. Personal sections
opened from Settings remain in Settings; Account deep links remain supported.
Category subtitles and large icon blocks were removed from the navigation.

Agent messages containing task/status cards retain the normal soft incoming
bubble. The former broad :has() rule incorrectly stripped padding/background
from the entire mixed message. Canceled attempts now take precedence over the
board's retained blocked state, displaying Cancelled instead of Needs attention.

Browser fixtures390/1440 verify desktop category persistence, default General,
mobile master/detail visibility, stable Settings personal routes, and nontransparent
mixed-message background/padding with a real rendered task card. Updated screenshots
were directly inspected. These checks do not establish native task completion.

## Canceled work and evaluation claims

Canceled attempts produce a plain cancellation notice without a passing checklist,
PO decision request or automatic revision. A canceled attempt is not evidence of
acceptance. DoD cards no longer compute a completion ratio from narrative evidence
items: that list is not the authoritative acceptance checklist and excludes gaps.
The original evaluation and underlying task criteria remain accessible in details.

A fixture containing one checked evidence bullet and one unmet criterion verifies
that the rendered report does not claim1/1 criteria passed. Screenshots are at
`review-evidence-390.png` and `review-evidence-1440.png` in the report directory.
These changes are verified locally and held while the native E2E attempt is live.
