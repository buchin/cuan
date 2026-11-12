package server

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/db"
)

const inboxQuotaNoBackup = "AI capacity is temporarily full. Your message is saved and will continue automatically."

type inboxModelKey struct{}

var fetchInboxModelUsage = agent.FetchRunnableAccountModelUsage

type inboxAccountRoute struct {
	accountID, home, model string
	remaining              float64
	primary                bool
	fallback               bool
}

// Only read-only Inbox turns can fail over. Execution tasks are never replayed.
// No native conversation is resumed under a different profile: the same durable
// Cuan conversation context is supplied to a fresh CLI turn, with the same agent.
func (s *Server) executeInboxWithFailover(ctx context.Context, owner string, registered *db.Agent, prompt, home, workspace string, limited bool, step func(string, string), attachments ...string) (string, string) {
	ctx = context.WithValue(ctx, tasteMemberKey{}, owner)
	hasImageAttachment := false
	for _, file := range attachments {
		ext := strings.ToLower(filepath.Ext(file))
		if ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".gif" || ext == ".webp" {
			hasImageAttachment = true
			break
		}
	}

	normHarness := db.NormalizeHarness(registered.Harness, registered.Provider)
	if normHarness != "agy" && normHarness != "antigravity" {
		execCtx := ctx
		if normHarness == "commandcode" {
			var conn *db.AgentConnection
			if registered.ConnectionID != "" {
				conn, _ = s.agentRepo.GetConnectionByID(ctx, registered.ConnectionID)
			}
			if (conn == nil || conn.CredentialsEncrypted == "") && owner != "" {
				if userConn, err := s.agentRepo.GetConnectionByUserAndProvider(ctx, owner, "commandcode"); err == nil && userConn != nil && userConn.CredentialsEncrypted != "" {
					conn = userConn
				}
			}
			if conn != nil && conn.CredentialsEncrypted != "" {
				if key, err := agent.DecryptCredential(conn.CredentialsEncrypted); err == nil {
					execCtx = context.WithValue(execCtx, commandCodeAPIKeyContext{}, key)
				}
			}
		}
		modelID := registered.ModelID
		if idx := strings.Index(modelID, "#"); idx != -1 {
			modelID = modelID[idx+1:]
		}
		if hasImageAttachment && !agent.IsModelVisionCapable(normHarness, modelID) {
			fallbackID := strings.TrimSpace(registered.FallbackModelID)
			if idx := strings.Index(fallbackID, "#"); idx != -1 {
				fallbackID = fallbackID[idx+1:]
			}
			if fallbackID != "" && agent.IsModelVisionCapable(normHarness, fallbackID) {
				modelID = fallbackID
			} else {
				return "", fmt.Sprintf("Model %s does not support image input (vision). Remove image attachments or switch to a vision-capable model.", modelID)
			}
		}
		if modelID != "" && !strings.HasPrefix(modelID, "conn_") && modelID != "commandcode-default" && modelID != "default" {
			execCtx = context.WithValue(execCtx, inboxModelKey{}, modelID)
		}
		out, fail := s.executeInboxCLI(execCtx, registered.Name, normHarness, prompt, home, workspace, step, attachments...)
		if out != "" {
			return out, ""
		}
		fallbackID := strings.TrimSpace(registered.FallbackModelID)
		if idx := strings.Index(fallbackID, "#"); idx != -1 {
			fallbackID = fallbackID[idx+1:]
		}
		if fallbackID != "" && fallbackID != modelID {
			if step != nil {
				step("model_fallback", fmt.Sprintf("Primary model %s encountered limit or error. Routing to fallback model %s.", modelID, fallbackID))
			}
			fbCtx := context.WithValue(execCtx, inboxModelKey{}, fallbackID)
			fbOut, fbFail := s.executeInboxCLI(fbCtx, registered.Name, normHarness, prompt, home, workspace, step, attachments...)
			if fbOut != "" {
				return fbOut, ""
			}
			fail = fbFail
		}
		return out, fail
	}
	routes := s.providerRoutes(ctx, owner, registered, home, limited)
	if hasImageAttachment {
		filtered := make([]inboxAccountRoute, 0, len(routes))
		for _, r := range routes {
			if r.model == "" || agent.IsModelVisionCapable("agy", r.model) {
				filtered = append(filtered, r)
			}
		}
		routes = filtered
	}
	attempts := 0
	busyRoutes := 0
	seen := map[string]bool{}
	triedAccounts := map[string]bool{}
	hadQuotaFailure := limited
	var lastNonQuotaFailure string
	var restriction string
	for len(routes) > 0 {
		if attempts >= 8 || ctx.Err() != nil {
			break
		}
		if lastNonQuotaFailure == inboxReviewProviderWaiting {
			if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) < inboxToolFreeBudget(ctx)+10*time.Second {
				if step != nil {
					step("review_budget_wait", "Waiting to review the saved result rather than starting a provider stage beyond this turn's remaining budget.")
				}
				return "", inboxReviewProviderWaiting
			}
		}
		// Try another permitted account before spending the turn on more models
		// from one failing account. Preserve quota ranking within each group.
		sort.SliceStable(routes, func(i, j int) bool { return !triedAccounts[routes[i].accountID] && triedAccounts[routes[j].accountID] })
		route := routes[0]
		routes = routes[1:]
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
		triedAccounts[route.accountID] = true
		if step != nil {
			if !route.primary {
				step("account_failover", "Continuing the reply with available AI capacity.")
			}
			if route.model == "" {
				step("model_auto", "Using Auto: the AI provider chooses its default model.")
			}
			if route.model != "" {
				if registered.FallbackModelID != "" && route.model == registered.FallbackModelID && route.model != registered.ModelID {
					step("model_fallback", fmt.Sprintf("Using fallback model %s based on agent configuration.", route.model))
				} else if route.remaining == -1 {
					step("model_selected", fmt.Sprintf("Using supported model %s; remaining usage has not been verified.", route.model))
				} else {
					step("model_selected", fmt.Sprintf("Using model %s based on the allowed model policy and available usage evidence.", route.model))
				}
			}
		}
		attemptCtx := context.WithValue(ctx, inboxModelKey{}, route.model)
		attemptCtx = context.WithValue(attemptCtx, inboxProviderAccountTraceKey{}, route.accountID)
		safe := false
		output, failure := func() (string, string) {
			defer release()
			if route.home != "" {
				if workspace != "" {
					_ = agent.PrepareAntigravitySettings(route.home, workspace)
				}
				_ = agent.PrepareAntigravityReadOnlyPermissions(route.home)
			}
			return s.executeInboxCLIWithSafety(attemptCtx, registered.Name, normHarness, prompt, route.home, workspace, step, &safe, attachments...)
		}()
		if output != "" {
			if step != nil && (!route.primary || attempts > 1) {
				step("account_failover_complete", "The reply is ready.")
			}
			return output, ""
		}
		if failure == inboxProviderCapacityWait {
			// The provider ended its own turn on a deadline. Spend the remaining
			// allowed routes before surfacing anything to the user: a single slow
			// account must not discard an otherwise answerable message. Only retry
			// when the turn is safe to repeat; a stage that already performed
			// tool activity must not be replayed on another account.
			if safe {
				lastNonQuotaFailure = inboxProviderCapacityWait
				routes = filterOutAccountRoutes(routes, route.accountID)
				continue
			}
			return "", inboxProviderCapacityWait
		}
		if retryableInboxReviewProvider(ctx, failure, safe, len(attachments)) {
			lastNonQuotaFailure = inboxReviewProviderWaiting
			if step != nil {
				step("review_provider_wait", "Review provider did not return a decision. Trying another allowed account or waiting to review the saved result; no task execution is repeated.")
			}
			routes = filterOutAccountRoutes(routes, route.accountID)
			continue
		}
		if strings.Contains(failure, "location restriction") && safe {
			// A regional rejection can be model-specific (for example Gemini
			// may fail where Sonnet succeeds). Only pre-authorized alternatives
			// already in the route list may be tried.
			restriction = failure
			if route.accountID != "" {
				if err := agent.RecordProviderModelRestriction(owner, route.accountID, route.model); err != nil {
					return "", "Provider location restriction could not be saved. Your message and approved work are preserved."
				}
			}
			if step != nil {
				step("provider_restricted", "This model route is restricted by the provider. Checking allowed alternatives without repeating work.")
			}
			continue
		}
		if !isInboxQuotaFailure(failure) {
			lastNonQuotaFailure = failure
			if !hadQuotaFailure {
				return "", failure
			}
			if !safe {
				return "", failure
			}
			// When failover to a backup account fails with a non-quota error,
			// drop remaining routes for this same account so we don't repeat
			// the same technical issue on different models of the same profile.
			routes = filterOutAccountRoutes(routes, route.accountID)
			if len(routes) > 0 {
				continue
			}
			return "", failure
		}
		hadQuotaFailure = true
		if route.accountID != "" {
			dur, _, _ := agent.ParseProviderQuotaResetDuration(failure)
			if err := agent.RecordProviderModelQuotaWithDuration(owner, route.accountID, route.model, dur); err != nil {
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
		if step != nil {
			step("capacity_wait", "Waiting for available AI capacity.")
		}
		return "", inboxQuotaNoBackup
	}
	if lastNonQuotaFailure == inboxProviderCapacityWait {
		// Every permitted route ended on the provider's own deadline. Report the
		// capacity wait in user terms and keep the message saved for retry.
		if step != nil {
			step("capacity_wait", "Waiting for available AI capacity.")
		}
		return "", inboxQuotaNoBackup
	}
	if lastNonQuotaFailure != "" {
		return "", lastNonQuotaFailure
	}
	if restriction != "" {
		return "", restriction
	}
	if len(routes) > 0 {
		if step != nil {
			step("recovery_safe", "The bounded route check ended after safe reply activity; remaining allowed routes will be checked later.")
			step("capacity_wait", "Waiting to check remaining AI capacity.")
		}
		return "", inboxQuotaNoBackup
	}
	if step != nil {
		step("recovery_safe", "Only safe reply activity occurred; no unfinished external operation will be replayed.")
		step("capacity_wait", "Waiting for available AI capacity.")
	}
	return "", inboxQuotaNoBackup
}
func isInboxQuotaFailure(failure string) bool {
	return strings.HasPrefix(failure, "Provider quota is limited")
}

func filterOutAccountRoutes(routes []inboxAccountRoute, accountID string) []inboxAccountRoute {
	if accountID == "" {
		return routes
	}
	filtered := make([]inboxAccountRoute, 0, len(routes))
	for _, r := range routes {
		if r.accountID != accountID {
			filtered = append(filtered, r)
		}
	}
	return filtered
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

	isCuanFacade := registered.ID == db.AgentIDCuan || registered.Name == db.DefaultAgentHandle || registered.Name == "@cuan"
	if primary == nil && isCuanFacade {
		if _, healthyAcct, err := agent.ResolveHealthyAccountProfile(owner); err == nil && healthyAcct != nil {
			primary = healthyAcct
			// Account metadata and native HOME must be selected together. A previous
			// load-balancer lookup may have returned another healthy brain.
			home, _ = agent.ProviderAccountHome(owner, primary.ID)
		} else {
			for _, p := range profiles {
				if !p.Legacy && p.CredentialsPresent && p.State != "login_required" && p.State != "unavailable" && p.State != "setup_incomplete" {
					primary = p
					home, _ = agent.ProviderAccountHome(owner, primary.ID)
					break
				}
			}
		}
	}

	type candidateRoute struct {
		profile *agent.ProviderAccount
		home    string
		primary bool
	}
	candidates := []candidateRoute{{primary, home, true}}
	for _, candidate := range profiles {
		if len(candidates) >= 8 {
			break
		}
		isEligible := candidate.AllowFailover || isCuanFacade
		if candidate.Legacy || !isEligible || !candidate.CredentialsPresent || candidate.State == "login_required" || candidate.State == "unavailable" || candidate.State == "setup_incomplete" || (!isCuanFacade && (candidate.AgentID == "" || candidate.ConnectionID == "")) || (primary != nil && primary.ID == candidate.ID) || s.agentRepo == nil {
			continue
		}
		if candidate.State == "quota_limited" && (candidate.QuotaRetryAt == nil || time.Now().Before(*candidate.QuotaRetryAt)) {
			continue
		}
		candidateHome, err := s.inboxAccountProfileHome(ctx, owner, registered, candidate)
		if err == nil && candidateHome != "" {
			_ = agent.PrepareAntigravitySettings(candidateHome)
			_ = agent.PrepareAntigravityReadOnlyPermissions(candidateHome)
			_ = agent.EnsureAntigravityOnboarding(candidateHome)
			candidates = append(candidates, candidateRoute{candidate, candidateHome, false})
		}
	}
	// A slow primary must not consume the backup's entire preflight window.
	// At most two quota/catalog probes run at once; results retain stable order.
	usages := make([]agent.AccountModelUsage, len(candidates))
	jobs := make(chan int, len(candidates))
	for i, candidate := range candidates {
		if candidate.profile != nil {
			jobs <- i
		}
	}
	close(jobs)
	var probes sync.WaitGroup
	for worker := 0; worker < 2; worker++ {
		probes.Add(1)
		go func() {
			defer probes.Done()
			for i := range jobs {
				if probeCtx.Err() != nil {
					return
				}
				usages[i] = fetchInboxModelUsage(probeCtx, owner, candidates[i].profile.ID)
			}
		}()
	}
	probes.Wait()
	routes := []inboxAccountRoute{}
	add := func(profile *agent.ProviderAccount, profileHome string, isPrimary bool, usage agent.AccountModelUsage) {
		model := registered.ModelID
		if !agent.IsTextModel(model) || isCuanFacade || model == "gemini-1.5-pro" || model == "gemini-1.5-flash" {
			model = ""
		}
		if isCuanFacade && profile != nil && profile.ModelID != "" {
			model = profile.ModelID
		}
		blocked := (isPrimary && limited) || (profile != nil && profile.State == "quota_limited" && (profile.QuotaRetryAt == nil || time.Now().Before(*profile.QuotaRetryAt)))
		addModel := func(id string, remaining float64, fallback ...bool) {
			if profile != nil && time.Now().Before(profile.ModelCooldowns[id]) {
				return
			}
			accountID := ""
			if profile != nil {
				accountID = profile.ID
			}
			routes = append(routes, inboxAccountRoute{accountID, profileHome, id, remaining, isPrimary, len(fallback) > 0 && fallback[0]})
		}
		if profile != nil && profile.FallbackModelID != "" {
			if blocked {
				return
			}
			addExact := func(id string, fallback bool) {
				remaining := -1.0 // Catalog membership does not establish quota.
				found := false
				for _, item := range usage.Models {
					if item.Model == id {
						found = true
						remaining = item.Remaining
						break
					}
				}
				if usage.Available && !found {
					return
				}
				if remaining == 0 {
					return
				}
				addModel(id, remaining, fallback)
			}
			addExact(profile.ModelID, false)
			if profile.AllowModelFallback && profile.FallbackModelID != profile.ModelID {
				addExact(profile.FallbackModelID, true)
			}
			return
		}
		// Auto delegates initial model choice to the CLI/provider: omit --model.
		// Explicit fallback remains a separate account permission.
		if model == "" {
			if !blocked {
				addModel("", 101)
			}
			if profile == nil || !profile.AllowModelFallback {
				return
			}
		}
		if !blocked && profile != nil && profile.AllowModelFallback && usage.Available {
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
		foundModel := false
		for _, item := range usage.Models {
			if model != "" && item.Model == model {
				foundModel = true
				if item.Remaining > 0 {
					addModel(model, item.Remaining)
				}
				break
			}
		}
		if !foundModel {
			score := -2.0
			if isPrimary {
				score = -1
			}
			addModel(model, score)
		}
		fallbackModel := registered.FallbackModelID
		if fallbackModel != "" && agent.IsTextModel(fallbackModel) && fallbackModel != model {
			fbFound := false
			fbScore := -2.5
			if isPrimary {
				fbScore = -1.5
			}
			for _, item := range usage.Models {
				if item.Model == fallbackModel {
					fbFound = true
					if item.Remaining > 0 {
						addModel(fallbackModel, item.Remaining-0.05)
					}
					break
				}
			}
			if !fbFound {
				addModel(fallbackModel, fbScore)
			}
		}
	}
	for i, candidate := range candidates {
		add(candidate.profile, candidate.home, candidate.primary, usages[i])
	}
	// Rank actual model quota evidence ahead of unknown availability. Ties retain
	// the primary account, then stable model/account order. Unknown is never 100%.
	sort.SliceStable(routes, func(i, j int) bool {
		if routes[i].fallback != routes[j].fallback {
			return !routes[i].fallback
		}
		if routes[i].remaining != routes[j].remaining {
			return routes[i].remaining > routes[j].remaining
		}
		return routes[i].primary && !routes[j].primary
	})

	return routes
}

func (s *Server) leaseProviderRoute(ctx context.Context, owner string, registered *db.Agent, route inboxAccountRoute) (func(), error) {
	lease := agent.TryLeaseProviderAccount
	if review, _ := ctx.Value(inboxTaskReviewKey{}).(bool); review {
		lease = agent.TryLeaseProviderAccountForReview
	}
	release, err := lease(owner, route.accountID)
	if err != nil {
		return nil, err
	}
	if route.accountID == "" {
		return release, nil
	}
	profile, err := agent.GetProviderAccount(owner, route.accountID)
	isCuanFacade := registered.ID == db.AgentIDCuan || registered.Name == db.DefaultAgentHandle || registered.Name == "@cuan"
	allowed := err == nil && profile.CredentialsPresent && ((route.primary && ((profile.AgentID == registered.ID && profile.ConnectionID == registered.ConnectionID) || isCuanFacade)) || (!route.primary && (profile.AllowFailover || isCuanFacade)))
	if allowed && isCuanFacade && route.model != profile.ModelID {
		allowed = profile.AllowModelFallback && (profile.FallbackModelID == "" || route.model == profile.FallbackModelID)
	}
	if allowed && route.fallback && (!profile.AllowModelFallback || profile.FallbackModelID != route.model) {
		allowed = false
	}
	if allowed && !isCuanFacade && route.model != "" && route.model != registered.ModelID && route.model != registered.FallbackModelID && agent.IsTextModel(registered.ModelID) && !profile.AllowModelFallback {
		allowed = false
	}
	if allowed && time.Now().Before(profile.ModelCooldowns[route.model]) {
		allowed = false
	}
	if allowed && profile.State == "quota_limited" && (profile.QuotaRetryAt == nil || time.Now().Before(*profile.QuotaRetryAt)) {
		allowed = false
	}
	if allowed {
		home, e := s.inboxAccountProfileHome(ctx, owner, registered, profile)
		allowed = e == nil && home == route.home
	}

	if !allowed {
		release()
		return nil, fmt.Errorf("Provider route is no longer allowed")
	}
	return release, nil
}

// New account onboarding connects a brain to the assistant, without creating a
// public persona. Legacy bound accounts still obey their agent enablement and
// binding checks; partially bound metadata must never bypass those checks.
func (s *Server) inboxAccountProfileHome(ctx context.Context, owner string, registered *db.Agent, profile *agent.ProviderAccount) (string, error) {
	if profile == nil || profile.Legacy || !profile.CredentialsPresent || (profile.Provider != "agy" && profile.Provider != "antigravity") || profile.State == "login_required" || profile.State == "unavailable" || profile.State == "setup_incomplete" {
		return "", errors.New("Account is not ready for this provider")
	}
	isFacade := registered.ID == db.AgentIDCuan
	if isFacade && profile.AgentID == "" && profile.ConnectionID == "" {
		return agent.ProviderAccountHome(owner, profile.ID)
	}
	if s.agentRepo == nil || profile.AgentID == "" || profile.ConnectionID == "" {
		return "", errors.New("Account binding is incomplete")
	}
	bound, err := s.agentRepo.GetAgentByID(ctx, profile.AgentID)
	if err != nil || bound == nil {
		cleanID := strings.TrimPrefix(profile.AgentID, "@")
		bound, err = s.agentRepo.GetAgentByID(ctx, "agent_"+cleanID)
	}
	if err != nil || bound == nil || !(bound.Enabled || bound.Status == db.AgentStatusActive || bound.Status == "") || (bound.ConnectionID != "" && bound.ConnectionID != profile.ConnectionID) {
		return "", errors.New("Account binding is not runnable")
	}
	harness := db.NormalizeHarness(bound.Harness, bound.Provider)
	if harness != "agy" && harness != "antigravity" && harness != "" {
		return "", errors.New("Account provider does not match its binding")
	}
	return s.ownedAgentProfileHome(ctx, owner, bound)
}
