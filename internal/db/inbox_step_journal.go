package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

const InboxStepSnapshotLimit = 100

// Keep the exact evidence used by recovery reducers, plus recent activity.
// The journal, not this projection, is the complete chronological record.
func compactInboxSteps(steps []InboxStep) []InboxStep {
	if len(steps) <= InboxStepSnapshotLimit {
		return steps
	}
	keep := map[int]bool{0: true}
	latest := map[string]int{}
	finished := 0
	for i := len(steps) - 1; i >= 0; i-- {
		s := steps[i]
		switch s.Kind {
		case "finished":
			if finished < 5 {
				keep[i] = true
				finished++
			}
		case "execution_timeout_returned", "execution_returned", "provider", "recovery_safe", "recovery_policy", "runtime_permission_denied", "trace_history_incomplete":
			if _, ok := latest[s.Kind]; !ok {
				latest[s.Kind] = i
				keep[i] = true
			}
		}
		if strings.HasPrefix(s.Kind, "runtime_tool_") {
			if _, ok := latest["tool"]; !ok {
				latest["tool"] = i
				keep[i] = true
			}
		}
		text := strings.ToLower(s.Text)
		for key, match := range map[string]bool{
			"quota":  s.Kind == "provider_cooldown" || strings.Contains(text, "usage limit") || strings.Contains(text, "quota"),
			"budget": s.Kind == "runtime_tool_budget_exceeded" || strings.Contains(text, "tool budget") || strings.Contains(text, "tool limit"),
		} {
			if match {
				if _, ok := latest[key]; !ok {
					latest[key] = i
					keep[i] = true
				}
			}
		}
	}
	for i := len(steps) - 1; i >= 0 && len(keep) < InboxStepSnapshotLimit; i-- {
		keep[i] = true
	}
	out := make([]InboxStep, 0, len(keep))
	for i, s := range steps {
		if keep[i] {
			out = append(out, s)
		}
	}
	return out
}

func nextInboxStep(steps []InboxStep, kind, text string) InboxStep {
	seq := 1
	if len(steps) > 0 {
		seq = steps[len(steps)-1].Seq + 1
	}
	return InboxStep{Seq: seq, Kind: kind, Text: text, At: time.Now().UTC()}
}

// The request row must be locked by the caller. Journal and live projection are
// committed together, including finalization. Initial retry markers are seeded
// lazily so retries created by older binaries remain readable during rollout.
func recordInboxStepTx(ctx context.Context, tx *sql.Tx, room, id string, steps []InboxStep, archived int, kind, text string) error {
	step := nextInboxStep(steps, kind, text)
	if step.Seq <= archived {
		step.Seq = archived + 1
	}
	steps = append(steps, step)
	for _, s := range steps {
		if s.Seq <= archived {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO inbox_request_steps(room_id,request_id,seq,kind,text,at) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`, room, id, s.Seq, s.Kind, s.Text, s.At); err != nil {
			return err
		}
	}
	raw, err := json.Marshal(compactInboxSteps(steps))
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE inbox_requests SET steps=$3 WHERE room_id=$1 AND request_id=$2`, room, id, raw)
	return err
}

func (r *MemoryProjectStudioRepository) recordInboxStep(key string, q *InboxRequest, kind, text string) {
	if r.stepJournal == nil {
		r.stepJournal = map[string][]InboxStep{}
	}
	if _, ok := r.stepJournal[key]; !ok {
		r.stepJournal[key] = append([]InboxStep(nil), q.Steps...)
	}
	step := nextInboxStep(q.Steps, kind, text)
	r.stepJournal[key] = append(r.stepJournal[key], step)
	q.Steps = compactInboxSteps(append(append([]InboxStep(nil), q.Steps...), step))
}

type InboxStepPage struct {
	Steps   []InboxStep `json:"steps"`
	NextSeq int         `json:"next_seq"`
	HasMore bool        `json:"has_more"`
}

type InboxStepJournalRepository interface {
	ListInboxSteps(context.Context, string, string, string, int, int) (InboxStepPage, error)
}

func inboxStepPage(steps []InboxStep, after, limit int) InboxStepPage {
	out := InboxStepPage{Steps: []InboxStep{}, NextSeq: after}
	for _, s := range steps {
		if s.Seq <= after {
			continue
		}
		if len(out.Steps) == limit {
			out.HasMore = true
			break
		}
		out.Steps = append(out.Steps, s)
		out.NextSeq = s.Seq
	}
	return out
}

func (r *MemoryProjectStudioRepository) ListInboxSteps(_ context.Context, room, id, owner string, after, limit int) (InboxStepPage, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	q, ok := r.requests[room+id]
	if !ok || (q.OwnerID != "" && q.OwnerID != owner) {
		return InboxStepPage{}, sql.ErrNoRows
	}
	steps, ok := r.stepJournal[room+id]
	if !ok {
		steps = q.Steps
	}
	return inboxStepPage(steps, after, limit), nil
}

func (r *SQLProjectStudioRepository) ListInboxSteps(ctx context.Context, room, id, owner string, after, limit int) (InboxStepPage, error) {
	// Restrict both existence and rows to the same owner. Include initial retry
	// markers not yet journaled, without duplicating stages already archived.
	var allowed bool
	if err := r.db.QueryRowContext(ctx, `SELECT true FROM inbox_requests WHERE room_id=$1 AND request_id=$2 AND (owner_id='' OR owner_id=$3)`, room, id, owner).Scan(&allowed); err != nil {
		return InboxStepPage{}, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT seq,kind,text,at FROM (
 SELECT s.seq,s.kind,s.text,s.at FROM inbox_request_steps s JOIN inbox_requests q USING(room_id,request_id) WHERE s.room_id=$1 AND s.request_id=$2 AND (q.owner_id='' OR q.owner_id=$3)
 UNION ALL
 SELECT (s->>'seq')::integer,s->>'kind',s->>'text',(s->>'at')::timestamptz FROM inbox_requests q CROSS JOIN LATERAL jsonb_array_elements(q.steps) s WHERE q.room_id=$1 AND q.request_id=$2 AND (q.owner_id='' OR q.owner_id=$3) AND NOT EXISTS (SELECT 1 FROM inbox_request_steps a WHERE a.room_id=q.room_id AND a.request_id=q.request_id AND a.seq=(s->>'seq')::integer)
 ) stages WHERE seq>$4 ORDER BY seq LIMIT $5`, room, id, owner, after, limit+1)
	if err != nil {
		return InboxStepPage{}, err
	}
	defer rows.Close()
	steps := []InboxStep{}
	for rows.Next() {
		var s InboxStep
		if err := rows.Scan(&s.Seq, &s.Kind, &s.Text, &s.At); err != nil {
			return InboxStepPage{}, err
		}
		steps = append(steps, s)
	}
	if err := rows.Err(); err != nil {
		return InboxStepPage{}, err
	}
	return inboxStepPage(steps, after, limit), nil
}
