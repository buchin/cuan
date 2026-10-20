User cukup menyampaikan tujuan dan memberi persetujuan; Cuan mengurus pekerjaan
sampai tuntas, termasuk memulihkan gangguan tanpa kehilangan konteks atau
menggandakan pekerjaan.

## Product implications

The Inbox agent clarifies intent, proposes consequential work for approval,
delegates approved execution, reports progress and outcomes, and suggests useful
next steps. The user should not need internal task/session terminology or
individual tool approvals to get ordinary work done.

Recovery must preserve the original goal and approval context. A retry or
continuation is one durable operation even across repeated clicks or reconnects.
Known safe failures can be retried; unknown execution outcomes need reconciliation
before another execution. Quota/auth problems need an available authorized
account/model or a clear recovery action.

## Current delivery and remaining gaps

AGY chat analysis now has external read-only filesystem isolation with automatic
tool approval. Safe failed replies support one idempotent new attempt using the
saved message/context. Existing task delegation, progress and results remain in
their current workflow.

Native provider-session checkpoint continuation, network egress policy, and a
verified turnkey Docker AGY runtime remain gaps. A fresh retry must not be
presented as resuming saved execution progress. Extension foundations are being
developed separately; this goal does not claim all recovery/product work complete.
