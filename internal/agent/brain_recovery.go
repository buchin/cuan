package agent

import (
	"context"
	"sort"
	"time"
)

// RecheckUnavailableBrain recovers a transient connection failure only after the
// pool has no ready brain. Probe one existing owner-bound profile per five-minute
// cooldown; never bypass a quota cooldown, reconnect, accept terms or edit policy.
func RecheckUnavailableBrain(ctx context.Context, owner string) {
	recheckUnavailableBrain(ctx, owner, VerifyProviderAccount)
}
func recheckUnavailableBrain(ctx context.Context, owner string, verify func(context.Context, string, string) (*ProviderAccount, error)) {
	if owner == "" || ctx.Err() != nil {
		return
	}
	profiles, err := ListProviderAccounts(owner)
	if err != nil {
		return
	}
	for _, p := range profiles {
		if p.Legacy || !p.CredentialsPresent {
			continue
		}
		if p.State == "verified" || p.State == "unknown" {
			return
		}
	}
	sort.Slice(profiles, func(i, j int) bool {
		if profiles[i].LastCheckedAt == nil {
			return profiles[j].LastCheckedAt != nil
		}
		if profiles[j].LastCheckedAt == nil {
			return false
		}
		return profiles[i].LastCheckedAt.Before(*profiles[j].LastCheckedAt)
	})
	for _, p := range profiles {
		if p.Legacy || !p.CredentialsPresent || p.State != "unavailable" || p.AgentID == "" || p.ConnectionID == "" {
			continue
		}
		if p.LastCheckedAt != nil && time.Since(*p.LastCheckedAt) < 5*time.Minute {
			continue
		}
		release, err := TryLeaseProviderAccountRefresh(owner, p.ID)
		if err != nil {
			continue
		}
		// Re-read under the cross-process lease so concurrent probes/edits win.
		current, err := GetProviderAccount(owner, p.ID)
		if err != nil || current.State != "unavailable" || current.LastCheckedAt != nil && time.Since(*current.LastCheckedAt) < 5*time.Minute {
			release()
			continue
		}
		probeCtx, cancel := context.WithTimeout(ctx, 36*time.Second)
		_, _ = verify(probeCtx, owner, p.ID)
		cancel()
		release()
		return
	}
}
