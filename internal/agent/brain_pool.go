package agent

import (
	"errors"
	"sort"
	"sync/atomic"
	"time"
)

var brainCursor atomic.Uint64
var ErrBrainCapacity = errors.New("AI capacity is temporarily full. Your work is saved and waiting for capacity.")

// LeaseHealthyBrain selects only explicitly connected profiles owned by this user.
// Cross-process leases prevent oversubscribing a brain; cooldowns prevent repeatedly
// routing to an exhausted account. Identity and conversation stay in Cuan.
func LeaseHealthyBrain(owner string, excluded map[string]bool, requiresVision ...bool) (string, *ProviderAccount, func(), error) {
	return leaseHealthyBrain(owner, excluded, TryLeaseProviderAccount, requiresVision...)
}

func LeaseHealthyBrainForTask(owner string, excluded map[string]bool, requiresVision ...bool) (string, *ProviderAccount, func(), error) {
	return leaseHealthyBrain(owner, excluded, TryLeaseProviderAccountForTask, requiresVision...)
}

func leaseHealthyBrain(owner string, excluded map[string]bool, lease func(string, string) (func(), error), requiresVision ...bool) (string, *ProviderAccount, func(), error) {
	if owner == "" {
		return "", nil, nil, ErrBrainCapacity
	}
	profiles, err := ListProviderAccounts(owner)
	if err != nil {
		return "", nil, nil, ErrBrainCapacity
	}
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].ID < profiles[j].ID })
	if len(profiles) == 0 {
		return "", nil, nil, ErrBrainCapacity
	}
	start := int(brainCursor.Add(1)-1) % len(profiles)
	for i := 0; i < len(profiles); i++ {
		p := profiles[(start+i)%len(profiles)]
		if p.Provider != "agy" && p.Provider != "antigravity" && p.Provider != "google" {
			continue
		}
		if len(requiresVision) > 0 && requiresVision[0] && p.ModelID != "" && !IsModelVisionCapable("agy", p.ModelID) {
			continue
		}
		if p.Legacy || excluded[p.ID] || !p.CredentialsPresent || p.State == "login_required" || p.State == "unavailable" || p.State == "setup_incomplete" {
			continue
		}
		if p.State == "quota_limited" && (p.QuotaRetryAt == nil || time.Now().Before(*p.QuotaRetryAt)) {
			continue
		}
		if time.Now().Before(p.ModelCooldowns[""]) {
			continue
		}
		release, err := lease(owner, p.ID)
		if err != nil {
			continue
		}
		current, err := GetProviderAccount(owner, p.ID)
		if err != nil || !current.CredentialsPresent || current.State == "quota_limited" || current.State == "unavailable" || current.State == "login_required" {
			release()
			continue
		}
		home, err := ProviderAccountHome(owner, p.ID)
		if err != nil || !IsAntigravityLoggedInInDir(home) {
			release()
			continue
		}
		return home, current, release, nil
	}
	return "", nil, nil, ErrBrainCapacity
}
