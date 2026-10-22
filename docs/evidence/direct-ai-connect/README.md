## Direct AI connection, system appearance and provider diagnostics

Studio now opens the Google connection dialog directly, creates its agent identity
automatically and keeps the goal/attachment draft in place. The Settings roster
retains its explicit naming flow. Browser fixtures at360/390/820/1440px in both
themes prove no agent-name input in Studio, no navigation away from Studio,
preserved goal draft, optional project setup, no execution during connection,
and one explicit goal submission. Failure/runtime-unavailable checks still pass.
These are fixture API checks, not live Google consent or inference.

Appearance defaults to System when no valid stored preference exists. A live
system preference change updates the theme; explicitly saved Light/Dark choices
remain honored. The dedicated /brand/cuan-favicon.svg path uses the exact kepeng
asset. Actual Dracin local app favicon matched the release; the public Fural
proxy's reserved /favicon.svg returned a different SVG. The dedicated link avoids
that path collision. These source changes are not deployed yet.

## Actual account and first-run observations

The owner connected one dedicated Google AI account. The failed chat request
8a728c49-2697-4b63-9301-12037e08f09c produced a runtime session and provider exit3.
Its private CLI log reports FAILED_PRECONDITION(code400): User location is not
supported for the API use. One separate minimal native CLI check using the same
dedicated profile also exited3 with the same message, without replaying the
user's request. This is provider wording, not evidence that an API key was used
or that the user's Google AI Pro subscription is invalid.

A separate actual interactive task was blocked at the first-run theme screen.
Selecting the default theme on that same live session advanced to Terms of
Service & Data Use, with optional interaction sharing preselected. No Done/terms
confirmation or data-sharing acceptance was sent. A consent question is pending.
Do not restart that live task or claim the account is execution-ready. First-run
wizard automation still needs implementation and verified completion; theme
selection alone is not sufficient evidence of account readiness.

New failures now classify this explicit provider restriction safely rather than
suggesting a generic reconnect. Recovery policy does not automatically retry the
persistent restriction even when no-effect evidence exists. Targeted decoder and
recovery tests pass. Never expose raw CLI logs/tokens in user-facing traces.


## Documented pre-login CLI preferences

The official CLI reference documents settings.json `colorScheme` and
`enableTelemetry` (https://antigravity.google/docs/cli/reference.md). Profile login
now prepares `colorScheme: terminal` and `enableTelemetry: false` before starting
the authentication process. These are defaults only; explicit preferences and
unknown settings survive. Malformed/linked settings stop with a safe error rather
than being overwritten. Atomic private writes and directory fsync retain settings.

Targeted tests prove a real child fixture reaches the Google URL only after
those settings exist, no credentials are fabricated, explicit settings stay
byte-identical, and malformed or linked files remain untouched. Six targeted
settings/login contract checks passed in0.117s. This is not proof that the actual
first-run wizard has completed. Metrics telemetry is distinct from provider
interaction-data consent; no terms acceptance or interaction-sharing preference
is inferred or written. Live wizard completion still awaits owner confirmation.


## Consent resolved and actual provider diagnosis

The owner explicitly approved provider terms with interaction sharing OFF. On the
same live task, the checkbox was deselected, focus moved Previous → Done, then
Done was confirmed. CLI persisted onboardingComplete=true and identified Google
AI Pro. The existing initial task prompt then returned the same Gemini location
restriction. Its exact attempt was closed and recorded blocked with evidence;
no duplicate attempt or user-message replay occurred.

The saved account's usage API succeeded: Sonnet/Opus had32.6486% remaining and
Gemini models approximately0.14947%. A separate minimal Sonnet check using that
same profile returned structured SUCCESS with exactly CUAN_ACCOUNT_CHECK_OK,
exit0 and no tool events. This proves this account works on Dracin with Sonnet;
the generic location failure must not be interpreted as rejecting the entire
account or as evidence of API-key billing.

New source automates the observed theme/terms sequence only after the web owner
confirms provider terms. The disclosure links the actual provider terms/privacy
pages. Optional sharing is deselected before Done; unrecognized screens receive
no input. Account-bound OAuth does not finish until both new credentials and the
provider's persisted onboardingComplete marker are present. Credential freshness
uses a prelaunch digest baseline rather than filesystem timestamp precision;
unchanged old credentials cannot establish successful authorization.

With no explicit model, Auto now selects one text model with available usage,
excluding cooldowns. At equal quota, balanced Sonnet is preferred. This initial
selection does not enable model fallback or override a fixed model. Connection
verification uses the same Auto selection, so a bad hidden CLI default cannot
incorrectly mark an otherwise working account unavailable. Existing explicit
model/fallback-policy tests remain passing. Integrated actual-child wizard,
consent/privacy, credential baseline, owner API and quota-routing tests pass;
clean build/browser verification and isolated deployment follow separately.
