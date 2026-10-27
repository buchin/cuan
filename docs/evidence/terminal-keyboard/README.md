## Mobile terminal input

Candidated030a0b connects to an existing session in interactive mode by default.
It applies the server's connected/status mode to stdin handling and uses the
keyboard button solely to focus the terminal. Focus occurs synchronously in the
click handler rather than after a WebSocket acknowledgement. The terminal is not
auto-focused on connection, so opening the page does not demand the keyboard.

Candidate build and browser checks passed at390/820/1440 using an intercepted
WebSocket. Default interactive mode, focus on the xterm textarea and typed stdin
were verified against the mock only. No stdin or mode command was sent to the
actual production terminal. Browser metadata GETs were read-only. See browser.json.
This does not prove visibility of an actual Android/iOS system keyboard; that
needs a physical-device check after activation.

Production remains79c04c0 while the quota-stalled task session remains open.
The canonical deployment guard refuses restart during active work. User approval
was requested to end that specific quota-stalled session before deployment.
No cancellation or restart is performed before that answer.
