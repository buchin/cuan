# Backend scope lists must not request visual preparation

Fresh task msg_b82e3c7d-138b-4914-bb84-4084c8b8b9cc explicitly excludes
`tidak ada UI, browser/screenshot, email`. Its first worker nevertheless logged
`Preparing locked frontend dependencies in the private development workspace`.
Read-only observation found a 580,485,120-byte workspace, including a
368,943,104-byte web directory. No visual recipe/capture event had been recorded
at this observation; do not claim screenshots were executed in this attempt.

The server removed only a narrower set of screenshot prohibitions before
classifying vision. Bare UI items and Indonesian `tidak ada` lists were not
recognized. The same exclusions could also be mistaken for visual/frontend scope
by inspection and source-verifier preparation. This made a backend job prepare
unneeded tooling and left later native capture admission inconsistent with the
original prohibition.

A bounded shared execution-policy matcher now recognizes explicit negative
visual lists in Indonesian and English. It stops at unrecognized items or later
affirmative instructions. The existing capability classifiers remove those
negative items; native recipe binding checks their capture prohibition. Actual
supplied images and positive frontend requirements remain independent. A
no-UI-changes restriction alone does not prohibit capturing an existing UI.
No task instructions, approval hashes or acceptance criteria are rewritten.

Server and runner regressions failed before integration. After correction,
shared policy, backend capability and real-recipe binding regressions pass;
source-verifier positive frontend cases, genuine image reading and original-scope
classification regressions pass. A real harmless recipe fixture is present in
the binding test, so absence of a recipe cannot make the prohibition test pass.
Tests never launch a browser or execute that recipe.

The supervisor was deliberately drained with SIGUSR1 to prevent new claims
before rollout. The existing developer was not canceled. Official deployment
must restore the supervisor through its auto-undrain/restart hook. The initial
frontend preparation remains a recorded deviation: this fresh task is no longer
an untouched autonomous benchmark. Existing copied dependencies are not deleted
from an active workspace to manufacture savings.
