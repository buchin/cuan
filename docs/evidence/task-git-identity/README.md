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

This change is committed but not yet deployed. Legacy standalone runners still
require their explicit human Git identity configuration. Real provider inference
and public OAuth application setup remain separate verification requirements.
