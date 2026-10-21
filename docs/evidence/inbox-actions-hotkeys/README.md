# Inbox actions and global keyboard shortcuts

Production release: `0b130c1fdaa3f67e908aa4553cab7d7753b0eaac`, tagged
`ui-release-20261021`. Activated atomically with a private pre-release database
backup, health checks and rollback support. No migrations were applied; schema25
remains. Backend source is unchanged from production `09a3fc4`.

## Verified behavior

The original production system notice now renders its Markdown and promotes its
trailing navigation links into a primary **Connect GitHub** action and secondary
**View work** link. These are navigation links styled for touch, not execution
permissions inferred from model text. Unknown destinations remain ordinary
Markdown links. User messages retain their plain-text rendering.

Production checks passed at 390, 820 and 1440 pixels, with no page overflow, a
44-pixel minimum primary action, and exact deployed assets matching the release.
The test used the real existing system message, with no injected message data.
After login, writes were blocked and no production AI inference was invoked.

Global navigation, shortcut help, composer focus and typing exclusion passed on
production. Staging additionally checked contextual New, Studio composer focus,
universal search, dialog exclusion and an isolated terminal DOM fixture. The
staging message/stream payload was mocked to exercise the exact system-message
case without writing test messages. These UI checks do not prove agent execution.

See [shortcut guide](../../guides/keyboard-shortcuts.md), `browser.json` for
staging checks and `production.json` for production checks. Responsive screenshots
are `staging-{width}.png`. Production screenshots remain local verification
files (`production-{width}.png`) to avoid publishing real conversation metadata.

## Separate workflow work remains open

Staging `269e9b0` includes automatic interruption recovery, canonical structured
report evaluation and task-bound private attachment delivery. Native interruption
recovery preserved the original task/artifact, and structured final review
completed without tools. Attachment snapshot/privacy/CLI tests pass.

However, a new native tip-calculator goal exposed a broader coordinator gap:
the proposal was plain prose without its structured approval card; after a normal
user approval the Inbox agent wrote HTML/README into its provider brain directory,
claimed delivery using file:// links, and created zero tasks. Plan mode permits
these provider artifact tools, so it is insufficient to enforce coordinator-only
execution. This failed acceptance must not be presented as successful delivery.
These backend changes were excluded from the production UI release. The main
conversation-to-task boundary, native downloadable delivery and fresh-live AI
onboarding still need end-to-end proof.
