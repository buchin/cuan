# Attachment and language verification

Targeted server tests: authenticated upload/download, disk persistence, private headers, foreign-owner exclusion, supported file types, rejected unsupported/fake/binary/empty/oversized files, five-file limit, immutable attachment references, reserved-envelope rejection, attachment-only messages, replay/conflict identity, readable text and image paths in coordinator context. Delegation uses a fake provider and real PTY; the task retains file references, has directory access and reads the original fixture file. Existing delegation/provenance and terminal lifetime tests pass.

TypeScript/Vite build passes. Existing CSS/bundle-size build warnings remain. The production-build browser fixture checks eight viewport/theme cases and real multipart XHR against isolated mock APIs: upload progress, preview, remove, clipboard paste, drop, reload persistence, accepted Studio/Inbox attachments, authenticated download, language switching, browser Auto detection, unsupported-locale fallback and retained drafts. Long filenames and Indonesian rendering are checked at360/390/820/1440px without horizontal overflow. No real provider invocations.

The first locale dictionary intentionally covers the main entry/conversation/settings flows; other labels use English fallback. Native provider image/PDF comprehension is not benchmarked by fake-provider tests. Original files and native input/access are verified without incurring paid provider calls.

## Production result

Released `2f8c91d1d5de0f21c8e4f6a339b4caebe20ab02c` through the native atomic workflow. Authenticated live browser smoke verified all eight viewport/theme cases, real multipart image upload, private preview/download,401 without authentication, language persistence, retained draft/attachment on locale change, and exact JS/CSS/font assets. No goal or chat messages were submitted and no provider was invoked. The single synthetic upload was removed by its exact ID/name; other uploads were preserved.

An isolated server-test binary also passed attachment acceptance/security and managed-task file reading with fake provider and real PTY on the production host, without production database access. Readiness is healthy, schema23 unchanged, automatic migration disabled;35 retained application tables and706 profile files match the validated private backup (login sessions excluded).

Future ideas from discussion: PWA home-screen installation with private API data excluded from offline caches; editable display name/unique login username; actionable notifications for approval, blockers, completion and new customer messages. These are recommendations, not shipped features in this release.
