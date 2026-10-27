# Adding a provider CLI to Cuan

This guide explains how to add a new AI provider CLI (e.g. CommandCode, Claude Code, Codex) to Cuan so users can connect it from StudioView and TeamView.

## Principles

- Prefer the provider's maintained official CLI where it provides reliable authentication or execution. Present the integration through Cuan's UI — users should not need a terminal, command flags, manual tokens, or knowledge of the implementation.
- Never reuse ambient developer credentials as Cuan user credentials. User API keys belong to the user and are stored encrypted in ProviderAccount.
- Users choose between connection modes (subscription connect via OAuth, or custom BYOK API key) from the UI.

## Two connection modes

### 1. Subscription connect (OAuth)

Uses the provider's CLI browser sign-in flow. Cuan starts an isolated CLI login, captures the OAuth URL, and the user pastes the authorization code into Cuan. Credentials stay CLI-owned in the user's private profile; Cuan never sees raw tokens.

### 2. Custom provider BYOK (Bring Your Own Key)

The user supplies an API key directly from the provider's dashboard. Cuan encrypts it and stores it in ProviderAccount. The key belongs to the user, not the ambient developer environment.

## Files to change

### Backend (internal/server/)

| File | Purpose |
|------|---------|
| agent_handlers.go | handleGetHarnessAuthStatus, handleInitHarnessAuth, handleConnectCLIAgent — add the new provider harness |
| agent_profile.go | agentProfileHome / ownedAgentProfileHome — allow the new harness for profile binding |
| inbox_cli.go | executeInboxCLIWithSafety — check the provider binary exists before offering it as a chat option |

### Frontend (web/src/)

| File | Purpose |
|------|---------|
| features/team/TeamView.tsx | Add the provider to the add-agent dialog, with BYOK key input if needed |
| features/inbox/StudioView.tsx | Offer the provider in the agent picker when verified |
| api/agents.ts | Extend ConnectCLIAgentRequest if new fields are needed |

### Agent package (internal/agent/)

| File | Purpose |
|------|---------|
| provider.go | Register the provider in DefaultRegistry |
| <provider>.go | Implement Provider (verify connection, list models) |

## Provider registry

Register the provider in DefaultRegistry():

```go
defaultRegistryInstance.Register(commandcode, NewCommandCodeProvider())
```

## Implementation checklist

- [x] Add provider harness to DefaultRegistry() (`internal/agent/provider.go`)
- [x] Implement Provider interface and model catalog with vision capabilities (`internal/agent/commandcode.go`)
- [x] Add commandcode case to handleGetHarnessAuthStatus (`internal/server/agent_handlers.go`)
- [x] Add commandcode case to handleInitHarnessAuth (`internal/server/agent_handlers.go`)
- [x] Add commandcode support to handleConnectCLIAgent (subscription + BYOK) (`internal/server/agent_handlers.go`)
- [x] Add commandcode case to executeInboxCLIWithSafety binary and arguments (`internal/server/inbox_cli.go`)
- [x] Add commandcode to agentProfileHome/ownedAgentProfileHome & handleVerifyAgentProfile (`internal/server/agent_profile.go`)
- [x] Add CommandCode option and model selector to TeamView add-agent dialog (`web/src/features/team/TeamView.tsx`)
- [x] Add CommandCode and vision per-model handling to StudioView agent picker (`web/src/features/inbox/StudioView.tsx`)
- [x] Add vision per-model filtering to AttachmentPicker (`web/src/features/inbox/Attachments.tsx` & `InboxView.tsx`)
- [x] Document limitations, vision per-model handling, and prerequisites

## Vision per-model handling & limitations

Not all AI models support multimodal (vision/image) inputs:
- **Vision-capable models**: Models such as `claude-sonnet-5-5`, `claude-opus-5-5`, `google/gemini-3.8-flash`, `deepseek/deepseek-v4-flash-vision-exp`, `moonshotai/kimi-k2.7-code`, and `qwen/qwen3.8-omni-flash` support image inputs alongside text and documents.
- **Text-only models**: Models such as `commandcode-default`, `claude-fable-5-1`, `deepseek/deepseek-v4-pro`, `moonshotai/kimi-k3.1-pro`, `qwen/qwen3.8-max`, `xai/grok-4.5`, and `reflection/beam:free` only process text instructions and textual workspace context.

### System behavior for Vision:
1. **Model Selection Verification**: When an agent or model is selected in StudioView or InboxView, Cuan checks `isModelVisionCapable(modelId, provider)`.
2. **Automatic Cancellation**: If the selected model does not support vision and image files are currently attached, the image attachments are automatically canceled/removed, and an informative notification is displayed to the user explaining that the selected model is text-only.
3. **Picker Guard**: `AttachmentPicker` disables camera/photo inputs and restricts file uploads to supported documents (`.pdf`, `.txt`, `.md`, `.csv`) when a non-vision model is active.
4. **Backend Enforcement**: `executeInboxCLIWithSafety` checks image attachments before invoking CLI execution; if an image is provided with a text-only model, it returns a clear, actionable error instructing the user to remove image attachments or switch to a vision-capable model.

## Connection Lifecycle & Management in Team/Settings

Users manage both Subscription and BYOK agent connections directly through the Cuan UI without using terminal commands:

### Factual Connection States
1. **Connected (`verified`)**: The provider account is active and verified within the validity window (default 5 minutes). Visualized with a green status badge and an active status indicator.
2. **Sign-in Required (`login_required`)**: OAuth token expired or revoked. Offers a prominent **Reconnect** button that initiates browser OAuth without deleting configured agent state.
3. **Quota Limited (`quota_limited`)**: Google/provider individual account quota reached. Displays quota reset countdown, retry timestamp, and automatically routes failover if secondary accounts are enabled.
4. **Connection Error (`unavailable`)**: Network failure or invalid credentials. Displays a descriptive warning banner with an inline **Reconnect** action.
5. **Not Checked (`unknown`)**: Connection state is pending verification.

### Actions & Touch Controls
- **Last Verified Timestamp**: Every agent card displays the exact formatted time of the last verification (e.g. `Just now`, `5m ago`, `Today, 17:13`).
- **Reconnect**: Dispatches a background verification probe with in-flight state (`Working...`) and double-click idempotency guard.
- **Disconnect**: Safely disables agent routing without deleting credentials or historical transcripts.
- **Replace BYOK Key**: For BYOK connections, allows instant in-place replacement of the API key with inline validation, Save, and Cancel controls.
- **Remove Agent**: Accessible from the action menu with confirmation and optimistic list updates.
- **Accessibility & Mobile Touch Targets**: All interactive elements maintain a minimum 44px (`min-h-11`) touch target and responsive wrap behavior tested down to 360px and 390px viewports.
