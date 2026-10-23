# Adding a provider CLI to Cuan

This guide explains how to add a new AI provider CLI (e.g. CommandCode, Claude Code, Codex) to Cuan so users can connect it from StudioView and TeamView.

## Principles

- Prefer the provider's maintained official CLI where it provides reliable authentication or execution. Present the integration through Cuan's UI — users should not need a terminal, command flags, manual tokens, or knowledge of the implementation.
- Never reuse Fural or ambient developer credentials as Cuan user credentials. User API keys belong to the user and are stored encrypted in ProviderAccount.
- Users choose between connection modes (subscription connect via OAuth, or custom BYOK API key) from the UI.

## Two connection modes

### 1. Subscription connect (OAuth)

Uses the provider's CLI browser sign-in flow. Cuan starts an isolated CLI login, captures the OAuth URL, and the user pastes the authorization code into Cuan. Credentials stay CLI-owned in the user's private profile; Cuan never sees raw tokens.

### 2. Custom provider BYOK (Bring Your Own Key)

The user supplies an API key directly from the provider's dashboard. Cuan encrypts it and stores it in ProviderAccount. The key belongs to the user, not Fural or the developer.

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

- [ ] Add provider harness to DefaultRegistry()
- [ ] Add commandcode case to handleGetHarnessAuthStatus
- [ ] Add commandcode case to handleInitHarnessAuth
- [ ] Add commandcode support to handleConnectCLIAgent (subscription + BYOK)
- [ ] Add commandcode case to executeInboxCLIWithSafety binary check
- [ ] Add commandcode to agentProfileHome/ownedAgentProfileHome
- [ ] Add CommandCode option to TeamView add-agent dialog
- [ ] Add CommandCode to StudioView agent picker
- [ ] Document limitations and prerequisites
