## Subscription and BYOK connection lifecycle

Team settings shows each agent's credential source (Subscription, BYOK, or Google account), provider, selected model, connection state, and the latest check time. Secret values are never rendered; subscription connections do not expose terminal or API-key instructions.

Use **Reconnect** to resume an interrupted sign-in or run a fresh provider check. Repeated clicks are ignored while an action is active, and reload keeps the server's authoritative state. **Disconnect** disables routing for the agent while retaining its identity and history. The actions menu provides permanent deletion after an explicit confirmation.

The lifecycle uses these states: connected/verified, login required, invalid or unavailable, quota limited, and not checked. Quota and availability messages include recovery guidance without asking users to re-enter credentials unnecessarily. Controls keep a 44px touch target and remain usable at 360px and 390px widths.
