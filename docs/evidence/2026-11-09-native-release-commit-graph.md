# Native release commit graph

Dependency-reuse task msg_6fb49fb4-6247-4b21-880e-e230a0cd2759 reached
independent source pass on 0db75132688aded56059d491807f888ef3beb32e.
Developer att_af44b4199b7f115b5139112e77a50c61 reconciled baseline 8301726c;
verifier att_471d8a7d85228fc053257cc1c27c8d46 independently passed criteria
1–4 and 6, retaining publication/live checks as pending. Root separately reran
three previously failing cases on exact source copies: independent mutation,
manifest invalidation and ignored-output guard all pass (0.038s). The manifest
case uses a failing npm stub only to observe that invalidation reaches install;
these fixtures establish behavior, not real-world end-to-end speedup.

Native release att_20d4452c7dc94723629b798b46ed2f14 prepared target
e15b962b3aa4449708e14e648b06d089f4c78182 through the registered GitHub channel.
Tests/build/transfer ran, but activation stopped before changing production:
`Regression guard: candidate does not provably contain the active release`.
The running release remained 9eed212c164d1875f3dfc1ea46c1321a94274c1c.

Actual immutable Git checks establish that active release is an ancestor of
e15b962b. Registered read-only Git inspection found the release checkout shallow
with only 5 reachable commits, while project cache is non-shallow and has 1034
reachable commits. PrepareReleaseWorkspace reused the intentionally shallow
verification snapshot. deploy.sh packed the truncated commit graph; activation
correctly refused this incomplete ancestry evidence. No guard was bypassed.

Release preparation now imports only all reachable commit objects from the
existing non-shallow project cache, then removes the known shallow boundary.
Verification snapshots remain shallow. Historical trees/blobs, unrelated refs
and build outputs are not copied; source/developer checkouts stay unchanged.
Existing valid shallow release checkouts can complete that graph without resetting
files/refs. Unknown/dirty receipts still fail; a shallow source cache or oversized
commit pack fails closed. The existing activation ancestry guard is unchanged.

A real Git regression failed before the correction. It now proves distant active
ancestry both in the release checkout and after the same commit-only pack/index
transport used by activation, while a historical content blob remains absent.
The complete internal/git suite passes (13.217s), native project release-workspace
contract passes, and all three deployment ancestry tests pass. Production rollout
of this correction and final native task acceptance remain pending.
