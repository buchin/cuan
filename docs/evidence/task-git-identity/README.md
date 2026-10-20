# Fresh-install task Git identity

A real commit initially failed with "Author identity unknown" under a blank HOME
and disabled system/global Git configuration. The task PTY environment filter
classified GIT_AUTHOR_NAME/EMAIL as sensitive because of its broad AUTH match.

Task launch now supplies the human owner identity, preferring a connected GitHub
account's public name and ID-based no-reply address when available. Explicit
GIT_AUTHOR_NAME/GIT_AUTHOR_EMAIL configuration overrides the derived identity.
The four exact author/committer identity variables survive sanitization; secrets
still do not. Developer-specific config defaults were removed. No shared Git
configuration is written and no agent identity is invented.

Targeted tests pass:
- Real Git init/add/commit under empty HOME and no system/global Git config.
- Commit author email equals the authenticated installation owner.
- Connected GitHub no-reply identity and explicit human override.
- Different owner/invalid identity refusal.
- Existing sanitized environment and queued real-process Git continuation.

This change is deployed in production release f40909a. Legacy standalone runners still
require their explicit human Git identity configuration. Real provider inference
and public OAuth application setup remain separate verification requirements.

`TestFreshTaskProcessCommitsWithOwnerIdentity` additionally passes through the actual task start endpoint, PTY/supervisor and deterministic harness: a real Git commit succeeds with system/global Git config disabled, its author email matches the authenticated Cuan owner, and the task reports done. This replaces model inference only and does not claim a live provider test.

Production read-only browser/asset, actual rollback/reactivation and retention checks pass: schema25, disabled automatic migration, 37 unchanged application tables, 930 unchanged profile files, seven unchanged item states and eleven unchanged notification flags. No production AI task or commit was triggered for verification.
