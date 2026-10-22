## Release verification — 2026-10-21

Deployed source: `94ff5f3ff53b522a21fb0cde2ba67e233e8f8e14`.
Production: https://cuan.dojo.cc
Dedicated test: https://cuan-goal-test-dojo-dojo.fural.space

## Implemented behavior

- Studio connects Google directly, retains the draft, and does not require an agent name.
- Supported Antigravity first-run theme and terms screens are handled during connection setup. Terms require explicit consent; optional interaction sharing is left off. Unknown screens receive no automatic input.
- A saved token alone does not establish readiness: persisted onboarding completion is required.
- Auto model selection uses evidenced remaining quota and respects explicit model and fallback choices.
- Appearance defaults to System. The dedicated Cuan favicon path avoids gateway favicon replacement.

## Evidence

- Targeted agent and server tests, production web build, direct-connect browser fixtures and theme fixtures passed before release.
- Actual dedicated account verification succeeded using `claude-sonnet-4-6`.
- A normal Studio request completed with the exact requested reply `CUAN_STUDIO_SETUP_VERIFIED`; zero tasks were created. The request ID was retained across response-status handling, preventing duplicate submission effects.
- Production activation passed health/readiness checks; schema remains 25 with automatic migration disabled.
- Production browser checks passed at 390, 820 and 1440 px: System appearance follows dark media preference, no horizontal overflow, dedicated favicon, and served assets exactly match the build.
- Production smoke made no AI calls and no application writes beyond login.
- Retention check: 37 application tables unchanged, 930 profile files byte-identical, 7 item states and 11 notification read flags preserved. Private backups and previous release retained.

## Remaining proof

A completely fresh live OAuth account has not yet exercised the automatic wizard end to end. The actual existing account completed native onboarding with consent, and automation is covered by PTY fixtures. Full goal-to-task execution plus controlled interruption/recovery remains separate unfinished acceptance work. These results do not establish universal model/location availability.
