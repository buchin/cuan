## Native fresh-profile OAuth start

The actual Cuan agent login driver was run against native AGY on the dedicated
Dracin test host with a genuinely empty temporary HOME. No saved credentials,
provider profile or Fural credentials were copied. The real provider OAuth URL
was validated, without manual theme selection or a user inference prompt.
No authorization code was submitted and no provider terms were accepted. The
login was cancelled, its process exited, and no credentials/onboarding-complete
state appeared. The temporary profile was removed by test cleanup.

This establishes the initial sign-in link and cancellation path only. It does
not establish full Google authorization or post-authorization setup. The owner
must still complete a fresh sign-in to prove those steps.

An initial test incorrectly asserted that native AGY would preserve literal
colorScheme/enableTelemetry fields in settings.json. The provider rewrites this
file during initialization. The final opt-in test checks the real authorization
flow and does not claim persistence of those preferences or acceptance of terms.
The fresh run did not show a theme wizard before the OAuth link. Existing unit
coverage for the observed post-login setup screens remains separate evidence.

See native-start.json. The test is opt-in and never invokes inference or signs
in automatically: TestLiveFreshProfileOAuthStart in internal/agent.
