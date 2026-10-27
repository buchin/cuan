## Saved Google account setup recovery

Release6f0cdb5 deployed to dedicated staging and production. The native wizard
probe confirmed that an unsized PTY clipped the theme screen before its Next
button; a40x120 PTY displayed theme and terms navigation. Existing dev2 Google
credentials were retained and its provider-owned onboardingComplete became true
through the wizard. The owner-authenticated finish_setup verification returned
HTTP200, state verified, using claude-sonnet-4-6. The account was then bound as
dev2 and permitted as a backup for the original native delivery test. Model
fallback was initially off for dev2. The owner subsequently explicitly enabled
it; permitted fallback is now active. Gemini availability is not claimed.

Targeted tests cover saved credentials, owner isolation, terms required, correct
terminal size and repeated completion. Frontend build and deployment health pass.
No new Google authorization code was requested. Native repository delivery subsequently passed and was independently verified;
see ../native-github-delivery/independent-proof.json.
