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

## Replaced connection events

All WebSocket message, error and close callbacks are bound to the currently
active socket. A delayed handshake or close from a replaced connection must not
reset the screen, write old output, disable stdin or start another reconnect.
`web/tests/terminal-socket-lifecycle.test.mjs` executes the component callbacks
against replaced and active sockets. This closes a connection race; it does not
establish the root cause of the remaining physical-keyboard report.

Fural also preserves its canvas on soft reconnect by skipping the history seed.
Cuan's runner bridge unconditionally replays scrollback before live output.
Simply removing the browser reset would duplicate that history. Adopting this
behavior requires an ordered replay/catch-up contract across the bridge, rather
than a browser-only change that could silently miss output during disconnection.

## Preserve PTY bytes

Binary WebSocket frames are raw PTY output, not JSON control frames. They must
reach xterm unchanged. The old JSON-looking-text regex removed legitimate
terminal content and could independently decode partial UTF-8 frames. The
socket lifecycle regression now checks byte-for-byte preservation of ANSI,
JSON output and Unicode split across frames. This establishes stream fidelity,
not resolution of the physical keyboard report. Soft reconnect replay and
render backpressure remain separate from this fix.

## Slow viewer recovery

The PTY session previously dropped output chunks silently when a subscriber's
256-chunk queue filled. A partial ANSI/UTF-8 sequence could therefore be followed
by later bytes, corrupting that viewer's terminal state. A slow viewer is now
detached instead; its socket closes so the existing reconnect path can replay
retained history. Healthy viewers and the agent process continue, and input is
not resent merely because output recovery occurs. The runner Unix bridge also
closes its connection when the subscriber detaches, waking its input reader.

`TestSessionSlowSubscriberCannotContinueWithMissingBytes` verifies the exact
overflow case, healthy-viewer isolation and retained-history replay. This is
gap detection and recovery, not Fural's full render-credit protocol or a soft
reconnect implementation. Sustained overload can still cause reconnects, and
history remains bounded to the existing retention window.

## Negotiated runner output credit

The browser requests `output_ack=1`. The runner WebSocket bridge advertises
`output_ack: true` only when it supports this mode. It sends at most one 4096-byte
binary PTY frame until the browser acknowledges its sequence after xterm's write
callback and a task yield. Keyboard input and heartbeat use independent readers
and remain available while output waits. Duplicate, stale and future credits
cannot release another frame, and credit messages never reach the PTY.

Unacknowledged frames expire with the connection timeout; disconnect also
unblocks the sender. Legacy clients and the managed-session path keep their
existing transport unless support is explicitly negotiated. The upstream
subscriber queue is still bounded: sustained slow rendering can cause the
overflow recovery above, rather than silently corrupting terminal output.

`TestRunnerOutputCreditDoesNotBlockKeyboardOrLeakIntoPTY` exercises a real Unix
socket and WebSocket. Browser evidence checks actual xterm acknowledgements at
390px and 1440px. This limits rendering backlog; it does not implement lossless
soft reconnect or prove the physical Android keyboard issue resolved.

## Shared composer and automatic accessory keys

Inbox and terminal use the same `ComposerInput` and textarea sizing function.
The terminal composer keeps its own PTY send, attachment, slash-command and
draft behavior, with one compact row instead of a full-width input above buttons.

On touch mobile screens, terminal accessory keys appear when an editable input
inside the terminal is focused and the visible viewport shrinks substantially.
They disappear when the keyboard closes or focus leaves the terminal. A hardware
keyboard alone does not show the bar. There is no separate keyboard-toggle button.
The browser fixture tests viewport shrink/restore with a preserved draft; actual
OS keyboard behavior still needs device verification, particularly after rotation
or when a terminal is mounted with the keyboard already open.
