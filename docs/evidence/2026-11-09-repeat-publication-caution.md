# Repeated-effect caution and approved publication

The fresh native task msg_d0843350-b9f0-4682-b326-c0adaa2498e8 explicitly
requests native deployment and exact health. Its original authorization also
says: "Jangan mengulang broad suite atau deploy hanya demi bukti bila bukti
native yang sah sudah ada." Attempt att_3da7d26b0b37cc739045416a691baa2c was
claimed with deny_deploy=true. This prohibits an otherwise approved first
publication, rather than merely avoiding repeated effects for missing evidence.

The existing publication-caution parser recognized an English equivalent and
Indonesian blind-deploy wording, but not the actual repeated-evidence wording.
A regression test reproduced the false ban before correction. The bounded
recognition now includes the repeated deploy-for-evidence caution, including
its conjunction with broad-suite repetition. It still requires a separate
positive publication request. Unknown qualifiers, an unconditional owner ban,
a read-only execution category and independent verifier restrictions retain
their existing precedence. No saved instruction, approval hash, attempt status
or checklist is modified.

All internal/executionpolicy tests pass. Targeted task tests verify approved
execution retains publication, a native source verifier remains read-only,
and a narrower attempt ban still wins. This is a local correction; production
rollout and recovery of this task have not yet been observed. The fresh task
therefore cannot be represented as a fully untouched successful pipeline.

## Full original-scope regression

Official rollout of 02a6b86adc6a63f5053b41b2b31ecd5e980ae192 completed
16:10:23Z–16:13:36Z (192.43 seconds), exit 0 and exact healthy SHA. The active
detached developer survived the supervisor restart. However, the official
execution diagnostic still returned deny_deploy=true. The original authorization
contained another false prohibition: "backend tidak melakukan copy ...;
immutable source ...; verifier independen ...; native deploy ...". The parser
let a negative predicate consume independent clauses after a semicolon.

The prohibition boundary now stops at semicolons, as the existing positive
publication parser already does. A regression includes the complete original
saved task scope, not just the caution sentence. Additional tests prove an
explicit ban after a semicolon is still enforced and a push prohibition before
a semicolon does not implicitly prohibit a separately requested deployment.
All executionpolicy tests and targeted attempt-boundary tests pass. This second
correction still requires production rollout and readback before claiming the
original task's publication path is restored.
