## Goal-first onboarding with installation prerequisites

Studio uses one primary action: connect AI when needed, then Get it done after
returning to the saved goal. Signing in never submits the goal. Project connection
stays optional and secondary. Missing runtime, unsupported deployment and request
failures retain the draft, explain next steps and provide a retry button.
Administrator guidance is collapsed. English default and Indonesian copy included.

The existing auth-status endpoint includes installation prerequisite booleans
only. It does not execute providers, read account contents, refresh credentials
or claim verified execution. Known Claude/Codex agent paths are not gated by
AGY-specific component checks. The stock Docker AI path remains unsupported.
Installer plan/completion output now states that limitation explicitly.

Targeted checks: provider-marker test proves prerequisite inspection does not
execute binaries; seven isolated installer contracts pass; TypeScript/Vite build
passes. Fixture browser tests at360/390/820/1440 in light/dark verify Google
connection return, retained goal, optional project cancellation, no automatic
submission, missing/unsupported/error retry, and one explicit final submission.
No live OAuth or inference is used by these fixtures.

This improves onboarding and installation guidance. A unified native application
installer and the full container AI runtime remain separate outstanding work.

## Production

Release7c54fa0 passes authenticated read-only Studio checks at all four widths in
both themes. Actual prerequisite endpoint reports installed components and does
not claim verified execution. Tested/released frontend hashes match. Actual
rollback/reactivation and retained data/profile/notification checks pass. No
production OAuth, goal submission or provider inference was performed.
