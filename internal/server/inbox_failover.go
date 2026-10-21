package server

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/db"
)

const inboxQuotaNoBackup = "No allowed account/model route is currently available. Your message is saved. Check usage and fallback settings in Settings → Agents or try again after the limit resets."

type inboxModelKey struct{}

var fetchInboxModelUsage = agent.FetchRunnableAccountModelUsage

type inboxAccountRoute struct {
	accountID, home, model string
	remaining              float64
	primary                bool
}

// Only read-only Inbox turns can fail over. Execution tasks are never replayed.
// No native conversation is resumed under a different profile: the same durable
// Cuan conversation context is supplied to a fresh CLI turn, with the same agent.
func (s *Server) executeInboxWithFailover(ctx context.Context, owner string, registered *db.Agent, prompt, home, workspace string, limited bool, step func(string, string), attachments ...string) (string, string) {
	if registered.Harness != "agy" && registered.Harness != "antigravity" {
		return s.executeInboxCLI(ctx, registered.Name, registered.Harness, prompt, home, workspace, step, attachments...)
	}
	routes := s.providerRoutes(ctx, owner, registered, home, limited)
	attempts := 0
	busyRoutes := 0
	seen := map[string]bool{}
	for _, route := range routes {
		if attempts >= 3 || ctx.Err() != nil {
			break
		}
		key := route.accountID + ":" + route.model
		if seen[key] {
			continue
		}
		seen[key] = true
		release, leaseErr := s.leaseProviderRoute(ctx, owner, registered, route)
		if leaseErr != nil {
			if errors.Is(leaseErr, agent.ErrAccountBusy) {
				busyRoutes++
			}
			continue
		}
		attempts++
		if step != nil {
			if !route.primary {
				step("account_failover", "Continuing this agent's reply with an allowed backup account.")
			}
			if route.model != "" {
				if route.remaining == -1 {
					step("model_selected", fmt.Sprintf("Using supported model %s; remaining usage has not been verified.", route.model))
				} else {
					step("model_selected", fmt.Sprintf("Using model %s based on the allowed model policy and available usage evidence.", route.model))
				}
			}
		}
		attemptCtx := context.WithValue(ctx, inboxModelKey{}, route.model)
		safe := false
		output, failure := func() (string, string) {
			defer release()
			return s.executeInboxCLIWithSafety(attemptCtx, registered.Name, registered.Harness, prompt, route.home, workspace, step, &safe, attachments...)
		}()
		if output != "" {
			if step != nil && (!route.primary || attempts > 1) {
				step("account_failover_complete", "This agent's reply completed using the selected account and model.")
			}
			return output, ""
		}
		if !isInboxQuotaFailure(failure) {
			return "", failure
		}
		if route.accountID != "" {
			if err := agent.RecordProviderModelQuota(owner, route.accountID, route.model); err != nil {
				return "", "The provider reached a usage limit, but its cooldown could not be saved. Your message is saved. Check the account status."
			}
		}
		if !safe {
			return "", "The provider reached a usage limit after tool activity that cannot be safely repeated. Your message is saved. No automatic account retry was started. Open Details to check the previous activity."
		}
		if step != nil {
			step("provider_cooldown", "The selected account/model reached a usage limit. Checking the next allowed route.")
		}
	}
	if ctx.Err() != nil {
		return "", safeInboxProviderFailure(ctx.Err(), "")
	}
	if attempts == 0 && busyRoutes > 0 {
		if step != nil {
			step("recovery_safe", "No provider execution was started; account session slots are full.")
		}
		return "", "All available AI session slots are in use. Your message is saved. Try again when a session finishes."
	}
	return "", inboxQuotaNoBackup
}
func isInboxQuotaFailure(failure string) bool {
	return strings.HasPrefix(failure, "Provider quota is limited.")
}

// Shared preflight for new Inbox turns and task launches; no process is started here.
func (s *Server) providerRoutes(ctx context.Context, owner string, registered *db.Agent, home string, limited bool) []inboxAccountRoute {
	probeCtx, cancelProbes := context.WithTimeout(ctx, 8*time.Second)
	defer cancelProbes()
	primary, _ := agent.ProviderAccountForConnection(owner, registered.ID, registered.ConnectionID)
	profiles, err := agent.ListProviderAccounts(owner)
	if err != nil {
		profiles = nil
	}
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].ID < profiles[j].ID })
	routes := []inboxAccountRoute{}
	add := func(profile *agent.ProviderAccount, profileHome string, isPrimary bool) {
		model := registered.ModelID
		if !agent.IsTextModel(model) {
			model = ""
		}
		usage := agent.AccountModelUsage{}
		if profile != nil {
			usage = fetchInboxModelUsage(probeCtx, owner, profile.ID)
		}
		blocked := (isPrimary && limited) || (profile != nil && profile.State == "quota_limited")
		addModel := func(id string, remaining float64) {
			if profile != nil && time.Now().Before(profile.ModelCooldowns[id]) {
				return
			}
			accountID := ""
			if profile != nil {
				accountID = profile.ID
			}
			routes = append(routes, inboxAccountRoute{accountID, profileHome, id, remaining, isPrimary})
		}
		// No fixed model means Auto, not an instruction to use the CLI's hidden
		// default. Choose one evidenced initial model without enabling fallback.
		if profile != nil && model == "" && !profile.AllowModelFallback && usage.Available {
			if best, ok := agent.AutomaticTextModel(usage, profile.ModelCooldowns, time.Now()); ok {
				addModel(best.Model, best.Remaining)
			}
			return
		}
		if profile != nil && profile.AllowModelFallback && usage.Available {
			for _, item := range usage.Models {
				if (item.Remaining > 0 || item.Remaining == -1) && agent.IsTextModel(item.Model) {
					addModel(item.Model, item.Remaining)
				}
			}
			return
		}
		// An account-wide legacy failure does not establish which alternative model
		// is healthy. Without fresh model evidence, do not guess around that limit.
		if blocked {
			return
		}
		if usage.Available && model == "" {
			positive := false
			for _, item := range usage.Models {
				if item.Remaining > 0 {
					positive = true
				}
			}
			if !positive {
				return
			}
		}
		for _, item := range usage.Models {
			if model != "" && item.Model == model {
				if item.Remaining > 0 {
					addModel(model, item.Remaining)
				}
				return
			}
		}
		score := -2.0
		if isPrimary {
			score = -1
		}
		addModel(model, score)
	}
	add(primary, home, true)
	checked := 0
	for _, candidate := range profiles {
		if ctx.Err() != nil || checked >= 7 {
			break
		}
		if candidate.Legacy || !candidate.AllowFailover || !candidate.CredentialsPresent || candidate.State == "login_required" || candidate.State == "unavailable" || candidate.AgentID == "" || candidate.ConnectionID == "" || (primary != nil && primary.ID == candidate.ID) {
			continue
		}
		if s.agentRepo == nil {
			continue
		}
		bound, err := s.agentRepo.GetAgentByID(ctx, candidate.AgentID)
		if err != nil || bound == nil || !bound.Enabled || bound.ConnectionID != candidate.ConnectionID || (bound.Harness != "agy" && bound.Harness != "antigravity") {
			continue
		}
		candidateHome, err := s.ownedAgentProfileHome(ctx, owner, bound)
		if err != nil || candidateHome == "" {
			continue
		}
		checked++
		add(candidate, candidateHome, false)
	}
	// Rank actual model quota evidence ahead of unknown availability. Ties retain
	// the primary account, then stable model/account order. Unknown is never 100%.
	sort.SliceStable(routes, func(i, j int) bool {
		if routes[i].remaining != routes[j].remaining {
			return routes[i].remaining > routes[j].remaining
		}
		return routes[i].primary && !routes[j].primary
	})

	return routes
}

func (s *Server) leaseProviderRoute(ctx context.Context, owner string, registered *db.Agent, route inboxAccountRoute) (func(), error) {
	release, err := agent.TryLeaseProviderAccount(owner, route.accountID)
	if err != nil {
		return nil, err
	}
	if route.accountID == "" {
		return release, nil
	}
	profile, err := agent.GetProviderAccount(owner, route.accountID)
	allowed := err == nil && profile.CredentialsPresent && ((route.primary && profile.AgentID == registered.ID && profile.ConnectionID == registered.ConnectionID) || (!route.primary && profile.AllowFailover))
	if allowed && route.model != "" && route.model != registered.ModelID && agent.IsTextModel(registered.ModelID) && !profile.AllowModelFallback {
		allowed = false
	}
	if allowed && time.Now().Before(profile.ModelCooldowns[route.model]) {
		allowed = false
	}
	if allowed && route.model == "" && profile.State == "quota_limited" {
		allowed = false
	}
	if allowed {
		bound, e := s.agentRepo.GetAgentByID(ctx, profile.AgentID)
		if e != nil || bound == nil || !bound.Enabled || bound.ConnectionID != profile.ConnectionID {
			allowed = false
		} else {
			home, e := s.ownedAgentProfileHome(ctx, owner, bound)
			if e != nil || home != route.home {
				allowed = false
			}
		}
	}
	if !allowed {
		release()
		return nil, fmt.Errorf("Provider route is no longer allowed")
	}
	return release, nil
}
