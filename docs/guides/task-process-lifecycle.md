# Task PTY process lifecycle

A terminal leader exiting is not proof that its commands have stopped. Commands
in the same process group can ignore TERM and terminal hangup while continuing
to hold workspace locks or modify saved files. Marking an attempt stopped before
cleaning that group can let a retry overlap the old work.

`SpawnPTY` owns the fresh process group created by `creack/pty`. After waiting for
the leader, its reaper closes the descriptor, marks it unavailable for input, then
cleans that owned group before releasing `waitDone`. This applies to cancellation,
timeout and ordinary leader exit, including a successful leader exit. The original
leader's wait error and exit status are retained; cleanup does not turn failure
into success or accept a task's deliverables.

Remaining group members receive TERM with a 200 ms grace period, followed by KILL
and a bounded 200 ms exit wait. Context cancellation keeps the existing three-second
leader wait delay. These bounds avoid waiting indefinitely for PID1 to reap orphan
zombies; zombies cannot continue task work. A repeated/concurrent `Close` waits for
the same cleanup rather than returning merely because another caller already
marked the descriptor closed. No extra per-keystroke or per-output work is added.

This is process-group cleanup, not a host-wide process sweep. Only the group ID
created for this PTY is signalled. An unrelated PTY is not affected. Commands that
explicitly create another session/group or daemonize require their own managed
lifecycle; this mechanism does not claim to discover every descendant. It also
cannot prove that old production timeout failures were caused by surviving children.
Do not kill unverified old production processes or restart tasks to test this.

## Verification

```sh
go test ./internal/terminal -count=1
go test ./internal/runner -run 'TestPTYDiagnostic|TestRunner_PTY|Test.*Quota|Test.*Brain' -count=1
```

The Linux regression fixture starts a real same-group child that ignores TERM/HUP
and writes a heartbeat file. Before the fix, both context cancellation and normal
leader exit returned from `Wait` while that child was still live. The repaired
fixture verifies cancellation, natural exit and explicit close, bounded completion,
a stopped/dead child, and no subsequent heartbeat writes. Additional tests cover
concurrent closes and an unrelated PTY remaining alive. The two original regression
cases also passed ten repetitions after adding bounded asynchronous-signal delivery
handling. Tests create and clean only their own local fixture process groups.
