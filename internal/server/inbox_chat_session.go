package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/diagnostics"
)

// prepareInboxChatSession creates an isolated, per-turn authenticated MCP session
// file bound to the requesting owner and workspace, with a short-lived token
// that is revoked when the chat turn completes.
func (s *Server) prepareInboxChatSession(ctx context.Context, workspace, crmSnapshot string) (string, func(), error) {
	noop := func() {}
	trace, ok := ctx.Value(inboxRequestTraceKey{}).(inboxRequestTrace)
	owner, _ := ctx.Value(tasteMemberKey{}).(string)
	if !ok || owner == "" || trace.RoomID == "" || trace.RequestID == "" || s.authSvc == nil {
		return "", noop, nil
	}

	token, err := s.authSvc.IssueEphemeralSession(ctx, owner, 20*time.Minute)
	if err != nil {
		return "", noop, err
	}

	apiURL := "http://127.0.0.1:8080"
	if s.cfg != nil && strings.TrimSpace(s.cfg.RunnerServerURL) != "" {
		apiURL = strings.TrimRight(strings.TrimSpace(s.cfg.RunnerServerURL), "/")
	}

	currentContactID := ""
	if crmSnapshot != "" {
		if rawCRM, readErr := os.ReadFile(crmSnapshot); readErr == nil {
			var snap diagnostics.InboxCRMSnapshot
			if json.Unmarshal(rawCRM, &snap) == nil {
				currentContactID = snap.CurrentContactID
			}
		}
	}

	session := diagnostics.InboxChatSession{
		OwnerID:          owner,
		RoomID:           trace.RoomID,
		RequestID:        trace.RequestID,
		Workspace:        workspace,
		CurrentContactID: currentContactID,
		APIURL:           apiURL,
		AuthToken:        token,
		ObservedAt:       time.Now().UTC(),
	}
	raw, err := json.Marshal(session)
	if err != nil {
		_ = s.authSvc.Logout(context.Background(), token)
		return "", noop, err
	}
	if len(raw) > 1<<20 {
		_ = s.authSvc.Logout(context.Background(), token)
		return "", noop, fmt.Errorf("chat session exceeds the bounded context limit")
	}

	directory, err := os.MkdirTemp("", "cuan-inbox-chat-")
	if err != nil {
		_ = s.authSvc.Logout(context.Background(), token)
		return "", noop, err
	}
	cleanup := func() {
		_ = s.authSvc.Logout(context.Background(), token)
		_ = os.RemoveAll(directory)
	}
	path := filepath.Join(directory, "session.json")
	if err = os.WriteFile(path, raw, 0600); err != nil {
		cleanup()
		return "", noop, err
	}
	return path, cleanup, nil
}
