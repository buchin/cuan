package server

import (
	"context"
	"errors"
	"fmt"
	"github.com/buchin/cuan/internal/git"
	"net/mail"
	"strings"
)

func (s *Server) taskCommitEnvironment(ctx context.Context, ownerID string) ([]string, error) {
	identity, err := s.authSvc.OwnerIdentity(ctx)
	if err != nil || identity.ID != ownerID {
		return nil, errors.New("Task owner identity is unavailable")
	}
	name := identity.Username
	email := identity.Email
	if name == "" {
		name = strings.SplitN(email, "@", 2)[0]
	}
	// Prefer GitHub's verified account identity and no-reply address when known.
	if path, e := githubConnectionPath(ownerID); e == nil {
		githubConnectionMu.Lock()
		account, _, e := git.LoadConnection(path, s.cfg.AuthSecret)
		githubConnectionMu.Unlock()
		if e == nil && account.ID > 0 && account.Login != "" {
			name = account.Name
			if name == "" {
				name = account.Login
			}
			email = fmt.Sprintf("%d+%s@users.noreply.github.com", account.ID, account.Login)
		}
	}
	if strings.TrimSpace(s.cfg.GitAuthorName) != "" {
		name = strings.TrimSpace(s.cfg.GitAuthorName)
	}
	if strings.TrimSpace(s.cfg.GitAuthorEmail) != "" {
		email = strings.TrimSpace(s.cfg.GitAuthorEmail)
	}
	address, e := mail.ParseAddress(email)
	if name == "" || len(name) > 200 || strings.ContainsAny(name, "\r\n\x00<>") || e != nil || address.Address != email || strings.ContainsAny(email, "\r\n\x00") {
		return nil, errors.New("Configure a valid human Git author name and email before committing task work")
	}
	return []string{"GIT_AUTHOR_NAME=" + name, "GIT_AUTHOR_EMAIL=" + email, "GIT_COMMITTER_NAME=" + name, "GIT_COMMITTER_EMAIL=" + email}, nil
}
