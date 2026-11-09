package server

import (
	"context"
	"path/filepath"

	"github.com/buchin/cuan/internal/db"
)

type inboxTurnContext struct {
	Prompt, Workspace string
	Review            bool
}

func (s *Server) prepareInboxTurnContext(ctx context.Context, q db.InboxRequest) (inboxTurnContext, error) {
	reconcile := db.InboxRetryNeedsReconciliation(q)
	if !reconcile {
		if snapshot, review := s.inboxTaskReviewPrompt(ctx, q); review {
			workspace := ""
			// The native task binds the repository. Do not rediscover it from
			// conversational keywords or inspect source/backlog that review discards.
			if d := s.inboxReviewedTask(ctx, q); d != nil && s.projectSvc != nil && d.ProjectID != "" {
				if p, err := s.projectSvc.Get(ctx, d.ProjectID); err == nil && p != nil && p.LocalPath != "" {
					workspace = p.LocalPath
					if absolute, err := filepath.Abs(workspace); err == nil {
						workspace = absolute
					}
				}
			}
			return inboxTurnContext{Prompt: snapshot, Workspace: workspace, Review: true}, nil
		}
	}
	prompt, err := s.inboxContext(ctx, q, reconcile)
	if err != nil {
		return inboxTurnContext{}, err
	}
	workspace := ""
	if !reconcile {
		var projectContext string
		workspace, projectContext = s.inboxProjectContext(ctx, q)
		prompt = truncateProjectText(projectContext, 24000) + "\n\n" + s.inboxCoordinatorTopicContext(ctx, q) + prompt
	}
	return inboxTurnContext{Prompt: prompt, Workspace: workspace}, nil
}
