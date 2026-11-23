package task

import "strings"

// EffectiveStatus presents durable launch intent without rewriting task history.
// Accepted completion wins; a newly queued recovery supersedes its old attempt.
func EffectiveStatus(status, attempt, launch string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	switch {
	case status == "done":
		return "done"
	case launch == "queued" || launch == "claimed":
		return "queued"
	case attempt == "running":
		return "doing"
	case attempt == "queued":
		return "queued"
	case attempt == "canceled" || attempt == "cancelled" || launch == "cancelled" || status == "canceled" || status == "cancelled":
		return "canceled"
	case attempt == "failed" || attempt == "interrupted" || status == "failed":
		return "blocked"
	case status == "queued" || status == "pending":
		return "queued"
	case launch == "stopped" && attempt != "succeeded":
		return "blocked"
	default:
		return status
	}
}

// EffectiveStatusSQL uses tasks t, latest attempt a and latest launch l. Keep the
// grouped counters and paged selection on the same read model as EffectiveStatus.
const EffectiveStatusSQL = `CASE
 WHEN lower(btrim(t.status)) = 'done' THEN 'done'
 WHEN l.state IN ('queued','claimed') THEN 'queued'
 WHEN a.status = 'running' THEN 'doing'
 WHEN a.status = 'queued' THEN 'queued'
 WHEN a.status IN ('canceled','cancelled') OR l.state = 'cancelled' OR lower(btrim(t.status)) IN ('canceled','cancelled') THEN 'canceled'
 WHEN a.status IN ('failed','interrupted') OR lower(btrim(t.status)) = 'failed' THEN 'blocked'
 WHEN lower(btrim(t.status)) IN ('queued','pending') THEN 'queued'
 WHEN l.state = 'stopped' AND COALESCE(a.status, '') != 'succeeded' THEN 'blocked'
 ELSE lower(btrim(t.status)) END`
