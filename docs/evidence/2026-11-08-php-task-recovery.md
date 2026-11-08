# Native PHP verification and autonomous task recovery

Task: `msg_174984aa-57f5-4264-a7c6-a63aedea00e1`.
Room: https://cuan.dojo.cc/inbox/60fdacfc-5584-4968-9000-61e6213d6192

This is an ongoing production observation, not acceptance of the task or proof of
the entire autonomous workflow. Reads used official Cuan CLI commands.

## Observed gaps and repairs

- Ordinary workers omitted registered PHPUnit targets both in server claims and
  runner completion. Releases `1f590ede` and `593116be` repaired both connections.
- Missing targets/runtime initially produced one ambiguous error and an empty
  native verification summary. Release `9eb66974` retains relative target/runtime
  diagnosis, distinguishes missing paths from escaped paths, and does not credit
  prerequisites as passing checks.
- No-change publication classification was enforced at completion but not retained
  for review. Release `2171d814` saves the native decision, bound to current
  successful completion and instruction hash. This never proves a historical
  branch commit was deployed or waives independent original acceptance criteria.
- Native npm preparation existed but Composer preparation did not. Release
  `5358578f` prepares locked private Composer dependencies without scripts/plugins
  or ambient credentials. A real offline Composer fixture preserves the lock and
  proves a post-install script does not run. Custom repositories remain explicit
  project preparation requirements.
- A model pass was saved before a later execution gate rejected failed native
  prerequisites. Release `2640f567` normalizes this specific authenticated failure
  before saving, retaining a scoped preparation replan. A selective policy digest
  initiated a fresh production review automatically, without manual retry.

## Production progression after repair

The fresh review `541e3078-9a89-49de-a07f-3e06c473eae3` completed normally in
approximately170seconds (provider159seconds). Its stored assessment says revise,
unfinished, with a specific prerequisite replan. It automatically launched attempt
`att_8cf50ce009a08c2a9f7dd29386e1342f` (attempt5).

Native Composer setup found a permission failure at vendor/bin/dep. The worker
reported removing the conflicting symlink and completing Composer/frontend setup.
The worker then reported49 authentication tests,12 unit tests and16 package tests
passing. Those are worker claims, not independent observations made by this session.

The runner independently ran the registered MCP target and failed with exit255:
Pest reported Tests\\TestCase configured twice. Primary-source inspection of the
exact registered release confirmed tests/Pest.php applies the TestCase to Feature,
while tests/Feature/Mcp/DojoMcpServerTest.php repeats that declaration. This is a
source test-configuration failure, not a missing runtime or a successful check.

PM review `9ca740ed-0e06-4499-abe4-09e8245c2b6e` correctly preserved the account
findings, identified the native failure and saved a concrete implementation
revision. It automatically launched attempt `att_b49b2b209eeb39f06369d481d9377b53`
(attempt6) to repair the conflicting declarations and rerun registered checks.
That attempt was running at this checkpoint; no independent acceptance is claimed.

## Limits and intervention disclosure

A maintainer repaired Cuan's orchestration and verification code during this task.
One limited task steer was queued after a worker searched AI account session
locations for context; it directed the worker to official bound-task context and
prohibited host-profile/history searches. No manual retry, status acceptance,
Dojo code edit or Dojo deployment was performed by this session during the
post-policy recovery described above. This is not a claim that the whole original
task ran without intervention.

Remaining proof: registered checks pass on the actual result, the appropriate
independent verifier completes original criteria, required publication is observed,
and PM accepts the task with a usable customer reply. End-to-end duration remains
too high for a simple account investigation; autonomous continuation alone is not
proof of acceptable efficiency.

## Exact-source verification checkpoint

Attempt6 passed the native registered tests on source
`52d92fccd2019e492d4c314ade211742fffba35b`, but failed publication. An initial
current-outcome inspection tested the older running source and failed. Repairs
`f9116ffd` and `7bfc60fd` admitted the existing exact tested local source without
rewriting either failure or replaying implementation.

Source verifier `att_5b5efab8a98e24fc7dc325e0d8a6b335` then failed before model
execution: system setup logs span exactly120seconds and its retained diagnostic
contains package downloads, not failing tests. Composer's cache was a disposable
HOME removed on every attempt. The actual registered source remains preserved.

The next repair retains owner-scoped public archives, uses a clean per-snapshot
vendor/auth environment, limits archive size, integrates pressure/lease cleanup,
and reserves at least half the remaining task budget for verification. The
existing one-per-source verifier recovery can recognize only the legacy native
cold-download boundary with empty native checks and no final model result, after
a fresh authenticated runner advertises the changed executable policy. It retains
source, assessment, owner, cancellation rules and failure history. No manual
retry or acceptance is used. Deployment and production recovery are still pending
at this checkpoint; passing isolated tests is not proof that the task is Done.
