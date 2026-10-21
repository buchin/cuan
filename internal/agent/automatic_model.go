package agent

import (
	"strings"
	"time"
)

// AutomaticTextModel chooses an evidenced initial model, not a fallback from an
// explicit model. Prefer balanced Sonnet over heavier models at equal quota.
func AutomaticTextModel(usage AccountModelUsage, cooldowns map[string]time.Time, now time.Time) (ModelUsage, bool) {
	var best ModelUsage
	found := false
	if !usage.Available {
		return best, false
	}
	for _, item := range usage.Models {
		if (item.Remaining == 0 || item.Remaining < -1) || !IsTextModel(item.Model) || now.Before(cooldowns[item.Model]) {
			continue
		}
		tie := found && item.Remaining == best.Remaining && strings.Contains(item.Model, "sonnet") && !strings.Contains(best.Model, "sonnet")
		if !found || item.Remaining > best.Remaining || tie {
			best = item
			found = true
		}
	}
	return best, found
}
