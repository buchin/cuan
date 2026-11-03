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
