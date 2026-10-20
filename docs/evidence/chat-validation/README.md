## Denied-tool final-result correction and UI feedback

Read-only production diagnosis found the two owner questions shown in the reported screenshots: both were incorrectly marked completed, with short 78/84-character preambles and a recorded runtime_permission_denied step. Their provider processes exited 0, but denied_actions was nonempty. The validator previously rejected denied actions only when response text was empty. Raw conversation content, credentials and runtime session IDs were omitted from the diagnostic report.

The validator now rejects any denied_actions result, including a nonempty preamble, without account replay or task execution. The next failed request reports the denied tool honestly. Existing historical messages/statuses are preserved; no original user request was rerun. The prompt requires an actual answer or concrete limitation rather than an unfinished promise. Safe prior failure outcomes are included in chat context so the agent can answer questions about recorded errors without reading filesystem transcripts.

Active bubbles show the latest safe recorded activity, with session IDs/raw tool arguments excluded. Compact search now keeps its input visible while the results scroll, uses denser icon/title/excerpt rows, highlights query text, avoids duplicate title/excerpt text, and cleans Markdown and internal Unix paths from the visual preview. This is preview cleanup, not arbitrary secret detection or backend secret redaction. Application logos link to Studio; landing branding already links to `/`. Draft preservation was verified.

Targeted validator/runtime/context Go tests pass. Chromium fixture tests pass for latest activity, exclusion of session IDs, logo navigation/draft retention at 390/1440px, and universal-search layout/navigation at 360/820/1440px in both themes. See browser.json and the universal-search evidence. No live provider calls.

## AGY permission routing

Fural source comparison confirmed its Antigravity ACP adapter passes
`--dangerously-skip-permissions` (backend/internal/antigravityacp/adapter.go),
and its AgentSession manager supplies the same flag with an external bwrap
boundary. Cuan's task adapter already passed this flag; Inbox did not.

Inbox now passes `--dangerously-skip-permissions --sandbox` instead of
`--mode plan --sandbox`. Its instructions allow read-only terminal inspection
inside the selected project, retain the prohibition on mutations/deployment,
and keep approved execution in application-managed tasks. No interactive tool
approval is expected. The CLI sandbox flag is retained; it must not be described
as an OS-enforced read-only filesystem or as equivalent to Fural's external
sandbox. Prompt instructions are not an access-control boundary. This change
does not add such an external sandbox.

Fake-provider tests verify the exact flags, selected project CWD, account HOME,
read-only instructions, and that an unexpected denied result still fails safely
without replay or leaking raw diagnostics. These tests do not prove every
command is accepted by the real provider. No paid inference or original user
message was replayed for verification.
