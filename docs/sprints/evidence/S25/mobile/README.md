# Mobile Inbox correction evidence

Deployed application: `fb83e133a6ed7babc1ff603568b1e03897139035`. Production, staging and candidate each pass 12 measured states. Before is a retained build rendered against the same production API, not a historical device screenshot. Drafts were unsent.

Keyboard means Chromium focused textarea plus reduced CSS viewport, not a physical OS keyboard. Synthetic safe-area/visualViewport tests are separately labeled. Original PO photo was inspected; its exact closed-keyboard blank gap did not reproduce in Chromium.

| CSS viewport | State | Before | Candidate | Staging | Production |
| --- | --- | --- | --- | --- | --- |
| 360×800 | closed | [PNG](before-360x800-closed.png) | [PNG](after-360x800-closed.png) | [PNG](staging-360x800-closed.png) | [PNG](production-360x800-closed.png) |
| 360×800 | keyboard-emulated | [PNG](before-360x800-keyboard-emulated.png) | [PNG](after-360x800-keyboard-emulated.png) | [PNG](staging-360x800-keyboard-emulated.png) | [PNG](production-360x800-keyboard-emulated.png) |
| 390×844 | closed | [PNG](before-390x844-closed.png) | [PNG](after-390x844-closed.png) | [PNG](staging-390x844-closed.png) | [PNG](production-390x844-closed.png) |
| 390×844 | keyboard-emulated | [PNG](before-390x844-keyboard-emulated.png) | [PNG](after-390x844-keyboard-emulated.png) | [PNG](staging-390x844-keyboard-emulated.png) | [PNG](production-390x844-keyboard-emulated.png) |
| 412×915 | closed | [PNG](before-412x915-closed.png) | [PNG](after-412x915-closed.png) | [PNG](staging-412x915-closed.png) | [PNG](production-412x915-closed.png) |
| 412×915 | keyboard-emulated | [PNG](before-412x915-keyboard-emulated.png) | [PNG](after-412x915-keyboard-emulated.png) | [PNG](staging-412x915-keyboard-emulated.png) | [PNG](production-412x915-keyboard-emulated.png) |
| 800×360 | closed | [PNG](before-800x360-closed.png) | [PNG](after-800x360-closed.png) | [PNG](staging-800x360-closed.png) | [PNG](production-800x360-closed.png) |
| 800×360 | keyboard-emulated | [PNG](before-800x360-keyboard-emulated.png) | [PNG](after-800x360-keyboard-emulated.png) | [PNG](staging-800x360-keyboard-emulated.png) | [PNG](production-800x360-keyboard-emulated.png) |
| 844×390 | closed | [PNG](before-844x390-closed.png) | [PNG](after-844x390-closed.png) | [PNG](staging-844x390-closed.png) | [PNG](production-844x390-closed.png) |
| 844×390 | keyboard-emulated | [PNG](before-844x390-keyboard-emulated.png) | [PNG](after-844x390-keyboard-emulated.png) | [PNG](staging-844x390-keyboard-emulated.png) | [PNG](production-844x390-keyboard-emulated.png) |
| 915×412 | closed | [PNG](before-915x412-closed.png) | [PNG](after-915x412-closed.png) | [PNG](staging-915x412-closed.png) | [PNG](production-915x412-closed.png) |
| 915×412 | keyboard-emulated | [PNG](before-915x412-keyboard-emulated.png) | [PNG](after-915x412-keyboard-emulated.png) | [PNG](staging-915x412-keyboard-emulated.png) | [PNG](production-915x412-keyboard-emulated.png) |

Measured geometry/scroll: [before](before.json), [candidate](after.json), [staging](staging.json), [production](production.json). Additional [synthetic checks](production-additional.json) and [screenshot](production-safe-area-long-draft.png). Release/data safeguards: [release](release.json), [rollback](rollback.txt), [before hashes](data-before.json), [activation hashes](data-after-activation.json), [final hashes](data-final.json).

Build: [log](build.txt). Reproducible scripts: [matrix](verification.cjs), [additional](additional-verification.cjs). Supply private token file via CUAN_E2E_TOKEN_FILE and an installed Playwright/Chromium as described in the release guide. Never commit token files.

All table hashes/counts are unchanged, including migrations. Readiness says 19; previous db4382f remains retained. No new provider call was used for this layout correction. Existing real reply was independently retrieved after service restarts. Physical-device keyboard/chrome/inset acceptance remains unverified; review must retain this limit.
