# Execution checkpoints and investigation budgets

All runner tasks receive the reporting contract, including tasks without a stored
acceptance checklist. An empty checklist does not waive verification. The task
must report factual progress through its bound capability before broad/risky/slow
work and at least every five minutes during substantial work. A progress report
is not a final result and does not stop execution.

At dispatch, the runner supplies its actual UTC execution deadline and remaining
time. The investigation guidance requires a reproducible failure or falsifiable
hypothesis early, a factual checkpoint by halfway, and time for verification and
reporting. Repeated broad searches without new evidence should narrow/change the
hypothesis rather than consume the entire execution window. These instructions
do not extend deadlines, waive scope or acceptance, or prove model compliance.

The historical native React timeout's provider transcript showed continuing
successful file reads and searches, rather than a single hung command: its final
retained segment contained 63 run_command calls, 21 read_file calls and no final
Cuan task report. This is evidence of extended investigation and missing saved
reporting, not proof that a particular command or provider503 caused the timeout.
Its saved task checklist was empty. The unified builder previously emitted its
reporting section only when checklist entries existed; the separate task
capability guidance did already request reports. The new unconditional reporting
section and explicit budget close that inconsistent prompt contract. They do
not retroactively repair the failed task or justify blindly reopening it.

Verification covers no-checklist reporting, idempotent prompt assembly, exact and
expired deadlines, the actual dispatched adapter prompt, cancellation outcomes,
and existing PTY/read-only execution behavior. Native model compliance and task
completion must still be observed separately through official Cuan task evidence.
