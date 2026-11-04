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
