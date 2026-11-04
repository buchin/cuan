# Mobile terminal input

Cuan normalizes Android/iOS IME input on touch devices before sending it to the
existing PTY socket. The approach comes from Fural's terminal input pipeline:
growing full-word resends become deltas, duplicate composition updates are
dropped, and autocorrect rewrites the affected suffix. Ordinary characters keep
the helper textarea context; Enter and line-killing controls clear stale context.
Desktop input is unchanged.

On reconnect to the same PTY, the screen replay retains helper textarea context
and re-seeds the mobile input tracker. A replacement session or attempt clears
that context. Already sent bytes are never replayed; this is not an acknowledged
delivery protocol. Screen output is never interpreted as user input.

Validation:

```sh
cd web
npm run typecheck
node --test tests/terminal-mobile-input.test.mjs tests/terminal-input-buffer.test.mjs
npm run build
```

Rendered browser checks at 390px and 1440px exercise the real xterm component
with a fixture WebSocket: native keyboard events, synthetic growing IME input
while output streams, and same-attempt reconnect with only unsent input flushed.
Screenshots were inspected. These checks do not emulate the complete native
SwiftKey composition lifecycle and do not prove every mobile keyboard works.
Terminal application redraws and server-side input handling remain separate
diagnostic paths if disappearance persists.

## Stable geometry follow-up

The first IME adaptation did not establish resolution of the reported native
keyboard problem. Comparing Fural revealed a separate issue: Cuan mixed
FitAddon.fit with a manual cell calculation that rounded up. Geometry now uses
one measured-cell calculation, reserves only complete cells, coalesces keyboard
layout animation and deduplicates PTY sizes per socket. A pending remote resize
is canceled whenever a newer fit is scheduled. Repeated connected frames on the
same socket no longer reset the screen or input context; a genuine reconnect
still resets for its scrollback replay because Cuan has no seed-skip protocol.

The rendered fixture initially exposed two initial PTY row sizes (47×11 then
47×14). The stable pipeline now sends one settled initial grid, sends no new
size for20 unchanged observer notifications, and sends at most one final resize
for a simulated keyboard-height animation. Pending helper context survives
streaming output, layout changes and duplicate handshakes. Reconnect retains
the existing unsent-input contract. Run from repository root:

```sh
node docs/evidence/terminal-geometry-check.cjs
```

This tests the real xterm component with fixture transport at390/1440px, not the
complete Android/SwiftKey OS composition lifecycle. It does not prove that every
redraw inside a provider TUI preserves that provider's own edit buffer.
