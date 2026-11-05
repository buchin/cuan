# Runner PTY diagnostics

The interactive terminal and persisted diagnostic logs have different consumers.
The terminal receives the original PTY bytes through its existing session/socket
bridge. `ptyDiagnosticCollector` only prepares records sent to the log API.

PTY frames can split a line, ANSI escape, UTF-8 character or provider error at any
byte. The collector waits for newline or carriage return before emitting a record,
flushes a final unterminated message, and caps a pending fragment at 16 KiB without
splitting valid UTF-8. Only repeated, recognized animated spinner states are
collapsed. Repeated commands, error messages and factual output are retained.
Logs from one output batch use one HTTP request instead of one request per line.

Provider quota detection uses a rolling bounded window, independently of completed
lines. The existing quota phrases can therefore stop a run even when split across
frames with no final newline. Existing successful-report and cancellation gates
still determine the final attempt result; a spinner is never completion evidence.
A detached slow diagnostic subscriber emits a system warning rather than silently
pretending the retained log is complete. Subscriber recovery remains separate from
lossless terminal reconnect; this change does not provide an unbounded transcript.

## Failure evidence and limits

On 2026-11-04 the latest native React attempt ended after its adaptive timeout
(about 45 minutes), while retained logs mostly contained animated `Running command`
fragments. The Inbox preview attempt ended with exit 2. Its exit code alone did not
identify a provider, filesystem, command or task-criteria cause. The old log reader
consumed partial `ReadString` output on every frame, and spinner records displaced
useful history under the existing log retention limit. This establishes a diagnostic
and quota-detection defect, not the original cause of either failed process.

This fix cannot restore already discarded logs, prove those tasks finished, or
justify blindly replaying them. No historical task is started or marked done by
this change. Subsequent diagnoses must use actual task/attempt evidence through
the official Cuan CLI/MCP, and distinguish a saved agent report from a terminal
process merely exiting.

## Verification

```sh
go test ./internal/runner -run 'TestPTYDiagnostic|TestRunner_PTY|Test.*Quota' -count=1
```

Tests cover every frame split of complete ANSI/UTF-8 diagnostic lines, 10,000
spinner redraws without loss of repeated errors, bounded unterminated output,
UTF-8 fragment boundaries, split quota phrases, and real PTY processes that emit
split failures and final messages without newline. A real split-quota process
must stop as quota-limited before its execution deadline. Existing interactive
PTY/socket tests continue to check terminal input and output independently.

## Partial redraws and terminal outcomes

Recognized spinner statuses also arrive as progressively painted prefixes such
as `Running comm`, `Reading file` and `Managing tasks`, interleaved with
backspaces. Prefixes of at least four characters map to the same known status for
consecutive redraw suppression. Specific commands, error suffixes, ordinary
non-spinner text and repeated factual failures remain separate records. The live
terminal stream is unchanged. Other unrecognized TUI repaint patterns can still
produce noise; this is not a complete terminal screen emulator.

The final attempt status log uses a separate three-second delivery context after
execution ends, including cancellation and deadline expiry. A slow log endpoint
cannot consume the completion API's independent fifteen-second delivery budget.
This improves subsequent diagnosis; it cannot restore discarded historical output
or establish the causes of the existing exit-2 and timeout failures.

Provider-owned CLI logs can contain failure details absent from the TUI stream.
See [storage pressure](storage-pressure.md) for the confirmed preview storage
failure and bounded attempt-specific provider log classification. Generic exit
codes alone still do not establish a cause.

## Final reports and intentional shutdown

A saved final report does not turn an unexpected non-zero process exit into
success. Native PTY execution records when the runner itself initiates closure
after a still-current final report, then normalizes only the expected signal
exits from that closure. The native signal remains in attempt diagnostics.
Readiness is rechecked after output drain so a newer progress report can keep the
session alive. Report polling is canceled and joined before execution returns.

An explicit final blocker also closes the idle CLI instead of occupying an AI
slot until the hard deadline. Its result stays blocked and cannot pass acceptance
or trigger project publication. Timeout and user cancellation remain authoritative.
The local terminal completion callback uses its own bounded context, so the HTTP
request that originally opened the terminal does not own final reconciliation.
