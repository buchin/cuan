package server

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/jackc/pgx/v5"
)

func (s *Server) executesInbox() bool {
	return s.cfg == nil || s.cfg.InboxExecutionRole != "api"
}

// Stop admission locally, then let owned work finish within its original durable
// deadlines. API restarts never call this on the separately supervised worker.
func (s *Server) DrainInboxWorker(ctx context.Context) error {
	s.inboxAdmission.Lock()
	s.inboxStopping.Store(true)
	s.inboxAdmission.Unlock()
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		busy := false
		s.inboxWorkers.Range(func(_, _ any) bool { busy = true; return false })
		if !busy {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
}

func (s *Server) notifyInboxProcess(room string) {
	if s.db == nil || s.cfg == nil || s.cfg.InboxExecutionRole == "" || !db.IsValidUUID(room) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := s.db.ExecContext(ctx, `SELECT pg_notify('cuan_inbox_updated', $1)`, room); err != nil {
		log.Print("Inbox change notification unavailable; recorded work remains durable")
	}
}

func (s *Server) notifyChatDelta(roomID, requestID, delta string) {
	if s.roomEventBroker != nil {
		s.roomEventBroker.Broadcast(roomID, "chat.delta", map[string]any{
			"request_id": requestID,
			"delta":      delta,
		})
	}
	if s.db == nil || s.cfg == nil || s.cfg.InboxExecutionRole == "" || !db.IsValidUUID(roomID) || delta == "" {
		return
	}
	payload, err := json.Marshal(map[string]string{
		"room_id":     roomID,
		"request_id":  requestID,
		"delta":       delta,
		"instance_id": s.instanceID,
	})
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, _ = s.db.ExecContext(ctx, `SELECT pg_notify('cuan_chat_delta', $1)`, string(payload))
}

// PostgreSQL is already the durable queue; NOTIFY only wakes dispatch/SSE.
// Missed notifications never authorize replay and reconnect reloads subscriptions.
func (s *Server) StartInboxEvents(ctx context.Context) {
	if s.db == nil || s.cfg == nil || s.cfg.InboxExecutionRole == "" {
		return
	}
	go func() {
		for ctx.Err() == nil {
			setupCtx, setupCancel := context.WithTimeout(ctx, 5*time.Second)
			conn, err := pgx.Connect(setupCtx, s.cfg.DatabaseURL)
			if err == nil {
				_, err = conn.Exec(setupCtx, "LISTEN cuan_inbox_updated; LISTEN cuan_chat_delta")
				setupCancel()
				if err == nil {
					s.refreshInboxSubscribers()
				}
				for err == nil {
					n, waitErr := conn.WaitForNotification(ctx)
					err = waitErr
					if err == nil {
						if n.Channel == "cuan_inbox_updated" && db.IsValidUUID(n.Payload) {
							if s.roomEventBroker != nil {
								s.roomEventBroker.Broadcast(n.Payload, "inbox.updated", nil)
							}
							if s.executesInbox() {
								s.kickInboxRoom(n.Payload)
							}
						} else if n.Channel == "cuan_chat_delta" {
							var p struct {
								RoomID     string `json:"room_id"`
								RequestID  string `json:"request_id"`
								Delta      string `json:"delta"`
								InstanceID string `json:"instance_id"`
							}
							if json.Unmarshal([]byte(n.Payload), &p) == nil && p.RoomID != "" && p.Delta != "" {
								if (p.InstanceID == "" || p.InstanceID != s.instanceID) && s.roomEventBroker != nil {
									s.roomEventBroker.Broadcast(p.RoomID, "chat.delta", map[string]any{
										"request_id": p.RequestID,
										"delta":      p.Delta,
									})
								}
							}
						}
					}
				}
				closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
				_ = conn.Close(closeCtx)
				cancel()
			}
			setupCancel()
			if ctx.Err() != nil {
				return
			}
			log.Print("Inbox event connection unavailable; reconnecting")
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
		}
	}()
}

func (s *Server) refreshInboxSubscribers() {
	if s.roomEventBroker == nil {
		return
	}
	s.roomEventBroker.mu.RLock()
	rooms := make([]string, 0, len(s.roomEventBroker.subscribers))
	for room := range s.roomEventBroker.subscribers {
		rooms = append(rooms, room)
	}
	s.roomEventBroker.mu.RUnlock()
	for _, room := range rooms {
		s.roomEventBroker.Broadcast(room, "inbox.updated", nil)
	}
}
