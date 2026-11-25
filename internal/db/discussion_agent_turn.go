package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var ErrDiscussionTurnNotFound = errors.New("discussion agent turn not found")

type DiscussionAgentTurn struct {
	ID               string                    `json:"id"`
	RoomID           string                    `json:"room_id"`
	PromptMessageID  string                    `json:"prompt_message_id"`
	ReplyMessageID   string                    `json:"reply_message_id,omitempty"`
	AgentID          string                    `json:"agent_id,omitempty"`
	Status           string                    `json:"status"`
	ResumeFromTurnID string                    `json:"resume_from_turn_id,omitempty"`
	ErrorMessage     string                    `json:"error_message,omitempty"`
	Context          map[string]any            `json:"context,omitempty"`
	Steps            []DiscussionAgentTurnStep `json:"steps"`
	StartedAt        *time.Time                `json:"started_at,omitempty"`
	FinishedAt       *time.Time                `json:"finished_at,omitempty"`
	CreatedAt        time.Time                 `json:"created_at"`
	UpdatedAt        time.Time                 `json:"updated_at"`
}

type DiscussionAgentTurnStep struct {
	Seq  int       `json:"seq"`
	Kind string    `json:"kind"`
	Text string    `json:"text"`
	At   time.Time `json:"at"`
}

type DiscussionAgentTurnRepository interface {
	CreateDiscussionAgentTurn(context.Context, *DiscussionAgentTurn) error
	GetDiscussionAgentTurn(context.Context, string) (*DiscussionAgentTurn, error)
	ListDiscussionAgentTurns(context.Context, string) ([]DiscussionAgentTurn, error)
	UpdateDiscussionAgentTurn(context.Context, *DiscussionAgentTurn) error
	AppendDiscussionAgentTurnStep(context.Context, string, DiscussionAgentTurnStep) error
}

func normalizeDiscussionTurnStatus(status string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(status))
	switch s {
	case "queued", "running", "paused", "completed", "failed", "cancelled":
		return s, nil
	}
	return "", errors.New("invalid discussion agent turn status")
}

func (r *SQLProjectStudioRepository) CreateDiscussionAgentTurn(ctx context.Context, turn *DiscussionAgentTurn) error {
	if turn == nil || !IsValidUUID(turn.ID) || !IsValidUUID(turn.RoomID) || !IsValidUUID(turn.PromptMessageID) {
		return ErrInvalidUUID
	}
	status, err := normalizeDiscussionTurnStatus(turn.Status)
	if err != nil {
		return err
	}
	turn.Status = status
	if turn.CreatedAt.IsZero() {
		turn.CreatedAt = time.Now().UTC()
	}
	turn.UpdatedAt = turn.CreatedAt
	ctxJSON, _ := json.Marshal(turn.Context)
	stepsJSON, _ := json.Marshal(turn.Steps)
	_, err = r.db.ExecContext(ctx, `INSERT INTO discussion_agent_turns (id,room_id,prompt_message_id,reply_message_id,agent_id,status,resume_from_turn_id,error_message,context,steps,started_at,finished_at,created_at,updated_at) VALUES ($1,$2,$3,NULLIF($4,'')::uuid,$5,$6,NULLIF($7,'')::uuid,$8,$9,$10,$11,$12,$13,$13)`, turn.ID, turn.RoomID, turn.PromptMessageID, turn.ReplyMessageID, turn.AgentID, turn.Status, turn.ResumeFromTurnID, turn.ErrorMessage, ctxJSON, stepsJSON, turn.StartedAt, turn.FinishedAt, turn.CreatedAt)
	return err
}

func scanDiscussionAgentTurn(scanner interface{ Scan(...any) error }) (*DiscussionAgentTurn, error) {
	var t DiscussionAgentTurn
	var ctxJSON, stepsJSON []byte
	err := scanner.Scan(&t.ID, &t.RoomID, &t.PromptMessageID, &t.ReplyMessageID, &t.AgentID, &t.Status, &t.ResumeFromTurnID, &t.ErrorMessage, &ctxJSON, &stepsJSON, &t.StartedAt, &t.FinishedAt, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(ctxJSON, &t.Context)
	_ = json.Unmarshal(stepsJSON, &t.Steps)
	return &t, nil
}

const discussionTurnColumns = `id,room_id,prompt_message_id,COALESCE(reply_message_id::text,''),agent_id,status,COALESCE(resume_from_turn_id::text,''),error_message,context,steps,started_at,finished_at,created_at,updated_at`

func (r *SQLProjectStudioRepository) GetDiscussionAgentTurn(ctx context.Context, id string) (*DiscussionAgentTurn, error) {
	t, err := scanDiscussionAgentTurn(r.db.QueryRowContext(ctx, `SELECT `+discussionTurnColumns+` FROM discussion_agent_turns WHERE id=$1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrDiscussionTurnNotFound
	}
	return t, err
}
func (r *SQLProjectStudioRepository) ListDiscussionAgentTurns(ctx context.Context, roomID string) ([]DiscussionAgentTurn, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+discussionTurnColumns+` FROM discussion_agent_turns WHERE room_id=$1 ORDER BY created_at,id`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DiscussionAgentTurn{}
	for rows.Next() {
		t, e := scanDiscussionAgentTurn(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}
func (r *SQLProjectStudioRepository) UpdateDiscussionAgentTurn(ctx context.Context, turn *DiscussionAgentTurn) error {
	status, err := normalizeDiscussionTurnStatus(turn.Status)
	if err != nil {
		return err
	}
	turn.Status = status
	ctxJSON, _ := json.Marshal(turn.Context)
	stepsJSON, _ := json.Marshal(turn.Steps)
	turn.UpdatedAt = time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `UPDATE discussion_agent_turns SET reply_message_id=NULLIF($2,'')::uuid,status=$3,resume_from_turn_id=NULLIF($4,'')::uuid,error_message=$5,context=$6,steps=$7,started_at=$8,finished_at=$9,updated_at=$10 WHERE id=$1`, turn.ID, turn.ReplyMessageID, turn.Status, turn.ResumeFromTurnID, turn.ErrorMessage, ctxJSON, stepsJSON, turn.StartedAt, turn.FinishedAt, turn.UpdatedAt)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrDiscussionTurnNotFound
	}
	return nil
}
func (r *SQLProjectStudioRepository) AppendDiscussionAgentTurnStep(ctx context.Context, id string, step DiscussionAgentTurnStep) error {
	result, err := r.db.ExecContext(ctx, `UPDATE discussion_agent_turns SET steps=steps || jsonb_build_array(jsonb_build_object('seq',jsonb_array_length(steps)+1,'kind',$2::text,'text',$3::text,'at',$4::timestamptz)), updated_at=$4 WHERE id=$1`, id, step.Kind, step.Text, step.At)
	if err != nil { return err }
	n, _ := result.RowsAffected()
	if n == 0 { return ErrDiscussionTurnNotFound }
	return nil
}
