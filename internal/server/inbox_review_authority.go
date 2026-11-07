package server

import (
	"context"
	"github.com/buchin/cuan/internal/executionpolicy"
	"github.com/buchin/cuan/internal/task"
)

// Review context distinguishes owner limits from a stricter automatic verifier
// stage. It never authorizes execution or changes the current attempt's policy.
func (s *Server) inboxApprovedTaskScope(ctx context.Context, d *task.TaskDetailResponse) (*executionpolicy.PublicationPolicy, string) {
	if d == nil {
		return nil, ""
	}
	owner, err := s.ownerContinuationLaunch(ctx, d)
	if err != nil {
		return nil, ""
	}
	policy := executionpolicy.FromInstructions(d.Instructions)
	continuation := ""
	if owner != nil {
		policy = policy.Restrict(executionpolicy.FromInstructions(owner.Revision))
		continuation = owner.Revision
	}
	return &policy, continuation
}
