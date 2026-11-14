package agent

import "time"

// ReconcileSelectedModelQuota applies fresh provider capacity evidence to an
// older, unscoped account cooldown. It does not certify login or runnable models.
func ReconcileSelectedModelQuota(owner, id string, usage AccountModelUsage) (*ProviderAccount, error) {
	accountProfileMu.Lock()
	defer accountProfileMu.Unlock()
	a, err := GetProviderAccount(owner, id)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if a.Legacy || !a.CredentialsPresent || a.State != "quota_limited" || a.ModelID == "" ||
		a.QuotaRetryAt == nil || a.LastCheckedAt == nil || !now.Before(*a.QuotaRetryAt) ||
		!usage.Available || usage.CheckedAt.IsZero() || !usage.CheckedAt.After(*a.LastCheckedAt) ||
		now.Sub(usage.CheckedAt) > time.Minute || usage.CheckedAt.After(now.Add(5*time.Second)) ||
		now.Before(a.ModelCooldowns[a.ModelID]) {
		return a, nil
	}
	available := false
	for _, model := range usage.Models {
		if (model.Model == a.ModelID || NormalizeModelAlias(model.Model) == NormalizeModelAlias(a.ModelID)) && model.Remaining > 0 {
			available = true
			break
		}
	}
	if !available {
		return a, nil
	}
	// The original failed route is unknown. Keep its deadline on Auto rather
	// than guessing Gemini/Claude quota groups or discarding the observation.
	if a.ModelCooldowns == nil {
		a.ModelCooldowns = map[string]time.Time{}
	}
	if a.ModelCooldowns[""].Before(*a.QuotaRetryAt) {
		a.ModelCooldowns[""] = *a.QuotaRetryAt
	}
	a.State = "unknown"
	a.QuotaRetryAt = nil
	a.VerifiedAt = nil
	a.LastCheckedAt = &usage.CheckedAt
	a.Message = "Selected model has available quota. Connection readiness has not been rechecked."
	if err := saveProviderAccount(a); err != nil {
		return nil, err
	}
	return a, nil
}
