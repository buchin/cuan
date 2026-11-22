package server

import (
	"context"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/db"
)

// ResolvedConversation holds the resolved room and target agent metadata.
type ResolvedConversation struct {
	RoomID          string
	TargetAgentID   string
	TargetAgentName string
	TargetHarness   string
	IsAgent         bool
}

// resolveConversationContext resolves any conversation identifier (room UUID, agent ID, contact alias, handle)
// into a valid room UUID in project_rooms and the target agent persona.
func (s *Server) resolveConversationContext(ctx context.Context, convID string, messageText string) (result ResolvedConversation) {
	inboxRoomResolutionMu.Lock()
	defer inboxRoomResolutionMu.Unlock()

	// Legacy rooms keep their IDs/history, while new turns use the public identity.
	defer func() {
		if result.IsAgent && s.agentRepo != nil {
			if isLegacyInternalAgent(result.TargetAgentID) {
				if a, err := s.agentRepo.GetAgentByID(ctx, db.AgentIDCuan); err == nil && a != nil && a.Enabled {
					result.TargetAgentID, result.TargetAgentName = a.ID, a.Name
					result.TargetHarness = db.NormalizeHarness(a.Harness, a.Provider)
				}
			}
		}
	}()
	s.ensureDefaultAgentRooms(ctx)

	result = ResolvedConversation{
		RoomID:        strings.TrimSpace(convID),
		TargetHarness: "agy",
	}

	trimmedID := strings.TrimSpace(convID)

	// 1. Fast path: If trimmedID is a valid room UUID and already exists, resolve directly
	if db.IsValidUUID(trimmedID) && s.projectStudioRepo != nil {
		if rm, err := s.projectStudioRepo.GetRoomByID(ctx, trimmedID); err == nil && rm != nil {
			result.RoomID = rm.ID
			if roomAgentID(rm.Topic) != "" {
				result.IsAgent = true
				result.TargetAgentID = roomAgentID(rm.Topic)
				result.TargetAgentName = rm.Title
				if s.agentRepo != nil {
					ag, err := s.agentRepo.GetAgentByID(ctx, result.TargetAgentID)
					if err != nil || ag == nil {
						ag, _ = s.agentRepo.GetAgentByID(ctx, db.AgentIDCuan)
					}
					if ag == nil {
						if ags, err := s.agentRepo.ListAgents(ctx, db.AgentFilter{}); err == nil {
							for _, candidate := range ags {
								if candidate.Enabled {
									ag = &candidate
									break
								}
							}
						}
					}
					if ag != nil {
						result.TargetAgentID = ag.ID
						result.TargetHarness = ag.Harness
						if ag.Name != "" && result.TargetAgentName == "" {
							result.TargetAgentName = ag.Name
						}
					}
				}
				if result.TargetAgentID == "" || strings.HasPrefix(result.TargetAgentID, "system:") {
					result.TargetAgentID = db.AgentIDCuan
				}
				result.TargetHarness = db.NormalizeHarness(result.TargetHarness, "")
				return result
			}
			return result
		}
	}

	// 3. Clean candidate agent ID from contact_ or agent_ prefix
	cleanAgentID := strings.TrimPrefix(trimmedID, "contact_")

	// 4. Check if convID matches an agent in agentRepo
	if s.agentRepo != nil {
		if ag, err := s.agentRepo.GetAgentByID(ctx, cleanAgentID); err == nil && ag != nil {
			result.TargetAgentID = ag.ID
			result.TargetAgentName = ag.Name
			result.TargetHarness = ag.Harness
			result.IsAgent = true
		} else if ags, err := s.agentRepo.ListAgents(ctx, db.AgentFilter{}); err == nil {
			for _, a := range ags {
				handleClean := strings.ToLower(strings.TrimPrefix(a.Name, "@"))
				reqClean := strings.ToLower(strings.TrimPrefix(cleanAgentID, "@"))
				deterministicUUID := db.GenerateDeterministicUUID("agent", a.ID)
				if a.ID == cleanAgentID || a.ID == trimmedID || handleClean == reqClean || trimmedID == deterministicUUID {
					result.TargetAgentID = a.ID
					result.TargetAgentName = a.Name
					result.TargetHarness = a.Harness
					result.IsAgent = true
					break
				}
			}
		}
	}

	// If resolved to an agent directly, find or create room for this agent
	if result.IsAgent {
		if s.projectStudioRepo != nil {
			var foundRoomID string
			if allRooms, err := s.projectStudioRepo.ListAllRooms(ctx); err == nil {
				for _, rm := range allRooms {
					if rm.Topic == "agent:"+result.TargetAgentID || rm.Title == result.TargetAgentName {
						foundRoomID = rm.ID
						break
					}
				}
			}
			if foundRoomID == "" {
				foundRoomID = db.GenerateDeterministicUUID("agent", result.TargetAgentID)
				now := time.Now().UTC()
				_ = s.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{
					ID:        foundRoomID,
					ProjectID: "default",
					Title:     result.TargetAgentName,
					Topic:     "agent:" + result.TargetAgentID,
					CreatedAt: now,
					UpdatedAt: now,
				})
			}
			result.RoomID = foundRoomID
		}
		result.TargetHarness = db.NormalizeHarness(result.TargetHarness, "")
		return result
	}

	// 5. If convID is a room UUID in projectStudioRepo
	if s.projectStudioRepo != nil {
		if rm, err := s.projectStudioRepo.GetRoomByID(ctx, trimmedID); err == nil && rm != nil {
			result.RoomID = rm.ID
			if roomAgentID(rm.Topic) != "" && s.agentRepo != nil {
				result.IsAgent = true
				result.TargetAgentID = roomAgentID(rm.Topic)
				result.TargetAgentName = rm.Title
				ag, err := s.agentRepo.GetAgentByID(ctx, result.TargetAgentID)
				if (err != nil || ag == nil) && s.agentRepo != nil {
					ag, _ = s.agentRepo.GetAgentByID(ctx, db.AgentIDCuan)
				}
				if ag != nil {
					result.TargetAgentID = ag.ID
					result.TargetHarness = ag.Harness
					if ag.Name != "" && result.TargetAgentName == "" {
						result.TargetAgentName = ag.Name
					}
				}
				result.TargetHarness = db.NormalizeHarness(result.TargetHarness, "")
				return result
			}
		}
	}

	// 6. Check mention in message if any
	mention := extractMentionHandle(messageText)
	if mention != "" {
		if s.agentRepo != nil {
			if ags, err := s.agentRepo.ListAgents(ctx, db.AgentFilter{}); err == nil {
				var cuanAgent *db.Agent
				for _, a := range ags {
					if a.Enabled && (a.ID == db.AgentIDCuan || strings.EqualFold(a.Name, db.DefaultAgentHandle)) {
						cuanCopy := a
						cuanAgent = &cuanCopy
						break
					}
				}

				mLower := strings.ToLower(mention)
				// If cuanAgent is active, legacy team/bot handles map to cuanAgent
				if cuanAgent != nil && (mLower == "@cuan" || mLower == "cuan" || mLower == "@team" || mLower == "@sm" || mLower == "@dev" || mLower == "@support") {
					result.TargetAgentID = cuanAgent.ID
					result.TargetAgentName = cuanAgent.Name
					result.TargetHarness = cuanAgent.Harness
					result.IsAgent = true
				}

				if !result.IsAgent {
					for _, a := range ags {
						if strings.EqualFold(a.Name, mention) || strings.EqualFold("@"+strings.TrimPrefix(a.Name, "@"), mention) {
							result.TargetAgentID = a.ID
							result.TargetAgentName = a.Name
							result.TargetHarness = a.Harness
							result.IsAgent = true
							break
						}
					}
				}

				// Special reserved handles: @team or @sm (Scrum Master) or @dev (Developer) when cuanAgent is not present
				if !result.IsAgent {
					if mLower == "@team" || mLower == "@sm" {
						for _, a := range ags {
							roleLower := strings.ToLower(a.Role)
							nameLower := strings.ToLower(a.Name)
							if strings.Contains(roleLower, "scrum") || strings.Contains(nameLower, "scrum") || strings.Contains(nameLower, "sm") {
								result.TargetAgentID = a.ID
								result.TargetAgentName = a.Name
								result.TargetHarness = a.Harness
								result.IsAgent = true
								break
							}
						}
						// If still not matched, pick first active enabled agent for @team
						if !result.IsAgent {
							for _, a := range ags {
								if a.Enabled {
									result.TargetAgentID = a.ID
									result.TargetAgentName = a.Name
									result.TargetHarness = a.Harness
									result.IsAgent = true
									break
								}
							}
						}
					} else if mLower == "@dev" {
						for _, a := range ags {
							roleLower := strings.ToLower(a.Role)
							nameLower := strings.ToLower(a.Name)
							if strings.Contains(roleLower, "dev") || strings.Contains(nameLower, "dev") || strings.Contains(nameLower, "worker") {
								result.TargetAgentID = a.ID
								result.TargetAgentName = a.Name
								result.TargetHarness = a.Harness
								result.IsAgent = true
								break
							}
						}
					} else if mLower == "@support" {
						for _, a := range ags {
							roleLower := strings.ToLower(a.Role)
							nameLower := strings.ToLower(a.Name)
							if strings.Contains(roleLower, "support") || strings.Contains(nameLower, "support") {
								result.TargetAgentID = a.ID
								result.TargetAgentName = a.Name
								result.TargetHarness = a.Harness
								result.IsAgent = true
								break
							}
						}
					}
				}
			}
		}
		// Fallback for default @cuan or Scrum Master if not yet registered in agentRepo
		if !result.IsAgent {
			mLower := strings.ToLower(mention)
			if mLower == "@cuan" || mLower == "cuan" || mLower == "@team" || mLower == "@sm" {
				result.TargetAgentID = db.AgentIDCuan
				result.TargetAgentName = db.DefaultAgentHandle
				result.TargetHarness = "agy"
				result.IsAgent = true
			} else if mLower == "@dev" {
				result.TargetAgentID = db.AgentIDCuan
				result.TargetAgentName = db.DefaultAgentHandle
				result.TargetHarness = "agy"
				result.IsAgent = true
			} else if mLower == "@support" {
				result.TargetAgentID = db.AgentIDCuan
				result.TargetAgentName = db.DefaultAgentHandle
				result.TargetHarness = "agy"
				result.IsAgent = true
			}
		}
	}

	// 7. Ensure RoomID is a valid UUID so SQL insert into project_messages never fails
	if !db.IsValidUUID(result.RoomID) {
		if s.projectStudioRepo != nil {
			var matchedRoomID string
			if allRooms, err := s.projectStudioRepo.ListAllRooms(ctx); err == nil {
				for _, rm := range allRooms {
					if rm.Title == trimmedID || rm.Topic == trimmedID {
						matchedRoomID = rm.ID
						break
					}
				}
			}
			if matchedRoomID == "" {
				matchedRoomID = db.GenerateUUID()
				now := time.Now().UTC()
				_ = s.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{
					ID:        matchedRoomID,
					ProjectID: "default",
					Title:     trimmedID,
					Topic:     "general",
					CreatedAt: now,
					UpdatedAt: now,
				})
			}
			result.RoomID = matchedRoomID
		} else {
			result.RoomID = db.GenerateUUID()
		}
	}

	result.TargetHarness = db.NormalizeHarness(result.TargetHarness, "")
	return result
}
