package server

import (
	"context"
	"errors"
	"log"
	"os"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/db"
)

func (s *Server) publishInbox(room string) {
	if s.roomEventBroker != nil {
		s.roomEventBroker.Broadcast(room, "inbox.updated", nil)
	}
	s.notifyInboxProcess(room)
}
func (s *Server) kickInboxRoom(room string) {
	if !s.executesInbox() || s.inboxStopping.Load() {
		return
	}
	if _, loaded := s.inboxWorkers.LoadOrStore(room, true); loaded {
		return
	}
	go func() { defer s.inboxWorkers.Delete(room); s.drainInboxRoom(room) }()
}

// StartInboxDispatcher resumes accepted queued work after a service restart.
// Claimed/running work is never replayed; its durable deadline yields interrupted.
func (s *Server) StartInboxDispatcher(ctx context.Context) {
	go func() {
		tick := time.NewTicker(5 * time.Second)
		defer tick.Stop()
		scan := func() {
			if s.cfg == nil || s.cfg.InboxExecutionRole != "worker" {
				if s.taskSvc != nil {
					if s.terminalManager != nil {
						for _, session := range s.terminalManager.ActiveSessions() {
							history := s.terminalManager.GetTaskHistory(session.TaskID())
							if len(history) == 0 {
								continue
							}
							latest := history[len(history)-1]
							if latest.SessionID == session.ID() && latest.AttemptID != "" && latest.Status == "running" {
								if err := s.taskSvc.RenewLocalAttemptLease(ctx, latest.AttemptID); err != nil {
									log.Print("local attempt heartbeat unavailable")
								}
							}
						}
					}
					if err := s.taskSvc.RecoverLocalAttempts(ctx); err != nil {
						log.Print("local attempt recovery unavailable")
					}
					_ = s.taskSvc.ReconcileOrphanLaunches(ctx, time.Now().UTC())
					_ = s.ReconcileZombies(ctx)
				}
				if s.runnerSvc != nil {
					_, _ = s.runnerSvc.ReconcileExpiredLeases(ctx)
				}
				s.kickTaskLaunches()
			}
			if !s.executesInbox() {
				return
			}
			if s.projectStudioRepo == nil {
				return
			}
			var roomIDs []string
			if pending, ok := s.projectStudioRepo.(db.InboxDispatchRepository); ok {
				var err error
				roomIDs, err = pending.ListInboxDispatchRooms(ctx, time.Now().UTC())
				if err != nil {
					log.Print("Inbox dispatch discovery unavailable")
					return
				}
			} else {
				rooms, err := s.projectStudioRepo.ListAllRooms(ctx)
				if err != nil {
					return
				}
				for _, room := range rooms {
					roomIDs = append(roomIDs, room.ID)
				}
			}
			for _, roomID := range roomIDs {
				if ctx.Err() != nil {
					return
				}
				s.kickInboxRoom(roomID)
			}
			s.kickInboxTaskUpdates(ctx)
		}
		scan()
		for {
			select {
			case <-ctx.Done():
				s.inboxStopping.Store(true)
				s.cancelGitHubDeviceOwner("")
				s.inboxCancels.Range(func(_, v any) bool { v.(context.CancelFunc)(); return true })
				return
			case <-tick.C:
				scan()
			}
		}
	}()
}

// Historical result inspection must not hold up accepted Inbox turns. Only one
// delivery scan runs at a time, independently of the bounded chat worker slots.
func (s *Server) kickInboxTaskUpdates(parent context.Context) {
	const key = "__inbox_task_updates"
	if !s.executesInbox() || s.inboxStopping.Load() {
		return
	}
	if _, loaded := s.inboxWorkers.LoadOrStore(key, true); loaded {
		return
	}
	go func() {
		defer s.inboxWorkers.Delete(key)
		ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
		defer cancel()
		s.deliverInboxTaskUpdates(ctx)
	}()
}
func (s *Server) drainInboxRoom(room string) {
	store, ok := s.projectStudioRepo.(db.InboxLifecycleRepository)
	if !ok {
		return
	}
	// A bounded room queue prevents an unbounded number of provider jobs.
	for i := 0; i < 20; i++ {
		if s.inboxStopping.Load() {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		requests, err := store.ListInboxRequests(ctx, room)
		cancel()
		if err != nil {
			return
		}
		if candidate := db.InboxAutomaticRecoveryCandidate(requests, time.Now()); candidate != nil {
			if recovery, ok := s.projectStudioRepo.(db.InboxAutoRetryRepository); ok {
				recoveryCtx, recoveryCancel := context.WithTimeout(context.Background(), 5*time.Second)
				child, fresh, recoveryErr := recovery.AutoRetryInboxRequest(recoveryCtx, room, candidate.RequestID, candidate.OwnerID)
				recoveryCancel()
				if recoveryErr == nil && fresh {
					requests = append(requests, child)
					s.publishInbox(room)
				}
			}
		}
		var next *db.InboxRequest
		for j := range requests {
			if requests[j].State == "queued" {
				if inboxCapacityWaiting(requests[j], time.Now()) {
					return
				}
				next = &requests[j]
				break
			}
		}
		if next == nil {
			return
		}
		// Empty rooms and capacity backoff must not occupy provider slots while
		// reading history; otherwise scans of many idle rooms starve real work.
		select {
		case s.inboxSlots <- struct{}{}:
		default:
			return
		}
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		q, claimed, err := s.claimInboxForDispatch(ctx, store, room, next.RequestID)
		cancel()
		if err != nil || !claimed {
			<-s.inboxSlots
			return
		}
		s.runInboxRequest(store, q)
		<-s.inboxSlots
	}
}
func (s *Server) runInboxRequest(store db.InboxLifecycleRepository, q db.InboxRequest) {
	lifecycleTimeout := 16 * time.Minute
	if v := os.Getenv("INBOX_LIFECYCLE_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			lifecycleTimeout = d
		}
	}
	ctx, cancel := context.WithDeadline(context.Background(), inboxRequestExecutionDeadline(time.Now(), q.Deadline, lifecycleTimeout))
	ctx = context.WithValue(ctx, inboxRequestTraceKey{}, inboxRequestTrace{RequestID: q.RequestID, RoomID: q.RoomID, AgentID: q.AgentID})
	ctx = context.WithValue(ctx, tasteMemberKey{}, q.OwnerID)
	if source, err := s.projectStudioRepo.GetMessageByID(ctx, q.MessageID); err == nil && source != nil && source.AuthorType == "user" && source.AuthorID == "operator_user" && decodeRegenerateReplyID(source.Content) != "" {
		ctx = context.WithValue(ctx, inboxRegenerateReplyKey{}, true)
	}
	s.learnInboxPreferences(ctx, q)
	handoffPersistenceRetry := false
	ctx = context.WithValue(ctx, inboxHandoffPersistenceKey{}, &handoffPersistenceRetry)
	reviewRefresh := false
	ctx = context.WithValue(ctx, inboxReviewRefreshKey{}, &reviewRefresh)
	if db.InboxRetryNeedsReconciliation(q) {
		ctx = context.WithValue(ctx, inboxRetryReconcileKey{}, true)
	}
	if s.inboxStopping.Load() && (s.cfg == nil || s.cfg.InboxExecutionRole != "worker") {
		cancel()
	}
	key := q.RoomID + ":" + q.RequestID
	s.inboxCancels.Store(key, context.CancelFunc(cancel))
	defer s.inboxCancels.Delete(key)
	defer cancel()
	// Detect cancellation by another server as well as local cancellation.
	done := make(chan struct{})
	defer close(done)
	go func() {
		tick := time.NewTicker(500 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-tick.C:
				check, c := context.WithTimeout(ctx, 2*time.Second)
				all, err := store.ListInboxRequests(check, q.RoomID)
				c()
				if err != nil {
					continue
				}
				for _, item := range all {
					if item.RequestID == q.RequestID && item.State != "running" {
						cancel()
						return
					}
				}
			}
		}
	}()
	step := func(kind, text string) {
		eventCtx, eventCancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer eventCancel()
		if err := store.AppendInboxStep(eventCtx, q.RoomID, q.RequestID, kind, text); err == nil {
			s.publishInbox(q.RoomID)
		} else {
			// Recovery must never infer safety from a silently incomplete trace.
			// Stop this execution; the existing unknown-outcome path preserves
			// its message and requires reconciliation rather than replay.
			log.Printf("inbox stage persistence failed room=%s request=%s kind=%s", q.RoomID, q.RequestID, kind)
			cancel()
		}
	}
	step("accepted", "Message received.")
	recoveryPolicy := "safe-reply-v1"
	// Opt in only new authenticated operator turns, never visitor or retry turns.
	if len(q.Steps) == 0 || (q.Steps[0].Kind != "retry" && q.Steps[0].Kind != "retry_auto" && q.Steps[0].Kind != "retry_reconcile") {
		if source, err := s.projectStudioRepo.GetMessageByID(ctx, q.MessageID); err == nil && source != nil && source.RoomID == q.RoomID && source.AuthorType == "user" && source.AuthorID == "operator_user" && q.OwnerID != "" {
			recoveryPolicy = "safe-reply-timeout-reconcile-v2"
		}
	}
	step("recovery_policy", recoveryPolicy)
	q.State = "completed"
	q.SafeError = ""
	var topicTitle string
	var reply *db.ProjectMessage
	if saved, ok := s.acceptedCustomerReplyResult(ctx, q, s.inboxReviewedTask(ctx, q)); ok {
		step("accepted_draft", "Using the customer reply from the accepted review.")
		name := db.DefaultAgentHandle
		if s.agentRepo != nil {
			if registered, err := s.runtimeAgent(ctx, q.AgentID); err == nil && registered != nil {
				name = registered.Name
			}
		}
		now := time.Now().UTC()
		reply = &db.ProjectMessage{ID: q.ReplyID, RoomID: q.RoomID, AuthorType: "agent", AuthorID: q.AgentID, AuthorName: name, Content: deduplicateWorkLinks(s.applyInboxReviewEvaluation(ctx, q, saved)), ReplyToID: &q.MessageID, CreatedAt: now, UpdatedAt: now}
		s.RecordMessageGrounding(q.ReplyID, s.ActiveBrandGroundingTastesForOwner(q.OwnerID))
	} else if q.AgentID != "" {
		if q.OwnerID == "" {
			q.SafeError = "Owner identity is unavailable. The message was not rerun."
		} else if s.agentRepo == nil {
			q.SafeError = "Agent list is unavailable."
		} else {
			registered, err := s.runtimeAgent(ctx, q.AgentID)
			if err != nil || registered == nil || !registered.Enabled {
				if allAgents, lErr := s.agentRepo.ListAgents(ctx, db.AgentFilter{}); lErr == nil {
					for _, a := range allAgents {
						if a.Enabled {
							copyAgent := a
							registered = &copyAgent
							break
						}
					}
				}
				if registered == nil {
					registered = &db.Agent{
						ID:      q.AgentID,
						Name:    db.DefaultAgentHandle,
						Harness: "agy",
						Enabled: true,
						Status:  db.AgentStatusActive,
					}
				}
			}
			if registered == nil || !registered.Enabled {
				q.SafeError = "The agent is unavailable or disabled. Your message is still saved."
			} else {
				status := agent.LegacyConnectionVerification(q.OwnerID, registered.ID, registered.ConnectionID).State
				if profile, e := agent.ProviderAccountForConnection(q.OwnerID, registered.ID, registered.ConnectionID); e == nil && profile != nil {
					status = profile.State
				} else if strings.HasPrefix(registered.ConnectionID, "conn_profile_") {
					if conn, e := s.agentRepo.GetConnectionByID(ctx, registered.ConnectionID); e == nil && conn != nil {
						if profile, e := agent.ProviderAccountForConnection(q.OwnerID, registered.ID, conn.ID); e == nil && profile != nil {
							status = profile.State
						}
					}
				} else if registered.ID == db.AgentIDCuan || registered.Name == db.DefaultAgentHandle || registered.Name == "@cuan" {
					if _, healthy, err := agent.ResolveHealthyAccountProfile(q.OwnerID); err == nil && healthy != nil {
						status = healthy.State
					}
				}
				home, err := s.ownedAgentProfileHome(ctx, q.OwnerID, registered)
				if err != nil {
					q.SafeError = "The agent connection is not ready. Check its sign-in status."
				} else {
					turn, err := s.prepareInboxTurnContext(ctx, q)
					if err != nil {
						q.SafeError = "Could not read conversation context. Your message is saved."
					} else {
						prompt, workspace := turn.Prompt, turn.Workspace
						step("context", "Project context checked for analysis.")
						var output, failure string
						snapshotReview := turn.Review
						plan, planErr := s.inboxApprovedPlan(ctx, q)
						if db.InboxRetryNeedsReconciliation(q) {
							step("reconciliation", "Checking the saved message against current recorded state without replaying earlier actions.")
							reconcileCtx := context.WithValue(ctx, inboxRetryReconcileKey{}, true)
							output, failure = s.executeInboxWithFailover(reconcileCtx, q.OwnerID, registered, prompt, home, "", status == "quota_limited", step)
						} else if planErr != nil {
							failure = "Could not read the approved plan. No new execution was started."
						} else if plan != nil {
							if plan.Action != "defer" {
								step("approved_plan", "Starting the exact plan approved in this conversation.")
							}
							if plan.Action == "defer" {
								output = "The plan is saved for later. No work was started by this action. You can approve it here when you are ready."
							} else {
								output = approvedPlanDelegation(plan, q.MessageID)
							}
						} else if autoDelegation, ok := s.inboxAutoDelegationPrompt(ctx, q); ok {
							step("auto_delegation", "Advancing to the next approved phase automatically.")
							output = autoDelegation
						} else {
							inferenceCtx := ctx
							if s.inboxProposalDecision(ctx, q) {
								inferenceCtx = context.WithValue(inferenceCtx, inboxProposalDecisionKey{}, true)
							}
							if snapshot := prompt; snapshotReview {
								inferenceCtx = context.WithValue(ctx, inboxTaskReviewKey{}, true)
								if isCustomerReplyDelivery(q, s.inboxReviewedTask(ctx, q)) {
									inferenceCtx = context.WithValue(inferenceCtx, inboxCustomerReplyKey{}, true)
								}
								snapshotReview = true
								prompt = snapshot
								if snapshot == "" {
									output = s.inboxStaleReviewOutcome(ctx, q)
									snapshotReview = false
								} else if handoff, ready := s.inboxCodeVerificationHandoff(ctx, q); ready {
									step("source_verification_handoff", "Routing the tested developer commit to independent verification.")
									output = handoff
								} else if handoff, ready := s.inboxNativeCheckHandoff(ctx, q); ready {
									step("native_check_correction", "Routing the recorded test failure to developer correction without another model decision.")
									output = handoff
								} else if handoff, ready := s.inboxArtifactVerificationHandoff(ctx, q); ready {
									step("verification_handoff", "Routing delivered files to independent verification.")
									output = handoff
								}
							}
							if output == "" {
								attachments := s.inboxAttachmentPaths(ctx, q)
								if snapshotReview {
									attachments = s.inboxReviewImagePaths(ctx, q)
									if inferenceCtx.Value(inboxCustomerReplyKey{}) == true {
										attachments = nil
									}
								}
								output, failure = s.executeInboxWithFailover(inferenceCtx, q.OwnerID, registered, prompt, home, workspace, status == "quota_limited", step, attachments...)
							}
						}
						output, topicTitle = extractTopicTitle(output)
						if results, searched, searchErr := s.inboxMemorySearchResults(ctx, q, output); searched {
							if searchErr != nil {
								output = searchErr.Error()
							} else {
								step("contact_search", "Searching saved contact notes.")
								if emptyMemorySearch(results) {
									output = s.inboxMemorySearchFallback(ctx, q, true)
									failure = ""
								} else {
									answerCtx, answerCancel := context.WithTimeout(ctx, 45*time.Second)
									answerCtx = context.WithValue(answerCtx, inboxAnswerOnlyKey{}, true)
									answerStep := func(kind, text string) {
										step(kind, text)
										if strings.HasPrefix(kind, "runtime_tool_") {
											answerCancel()
										}
									}
									answerPrompt, contextErr := s.inboxContext(ctx, q)
									if contextErr == nil {
										output, failure = s.executeInboxWithFailover(answerCtx, q.OwnerID, registered, answerPrompt+results, home, "", false, answerStep)
									}
									answerCancel()
									if contextErr != nil || output == "" || strings.Contains(output, "```cuan-") {
										output = s.inboxMemorySearchFallback(ctx, q, false)
										failure = ""
									}
								}
							}
						}
						if clean, suggestion := extractTopicTitle(output); suggestion != "" {
							output, topicTitle = clean, suggestion
						} else {
							output = clean
						}
						if !db.InboxRetryNeedsReconciliation(q) && s.inboxReviewedTask(ctx, q) == nil && output == "" && ctx.Err() == nil && (strings.Contains(strings.ToLower(failure), "tool permission") || strings.Contains(strings.ToLower(failure), "action outside the chat workflow") || strings.Contains(strings.ToLower(failure), "could not be validated") || strings.Contains(strings.ToLower(failure), "tool budget")) {
							step("response_recovery", "A tool or validation issue occurred. Answering directly from conversation context with structured actions.")
							directRecoveryTimeout := 120 * time.Second
							if v := os.Getenv("INBOX_DIRECT_RECOVERY_TIMEOUT"); v != "" {
								if d, err := time.ParseDuration(v); err == nil && d > 0 {
									directRecoveryTimeout = d
								}
							}
							directCtx, directCancel := context.WithTimeout(ctx, directRecoveryTimeout)
							directCtx = context.WithValue(directCtx, inboxCoordinatorDirectKey{}, true)
							directCtx = context.WithValue(directCtx, inboxSchemaRetryKey{}, true)
							directPrompt, contextErr := s.inboxContext(directCtx, q)
							if contextErr == nil {
								recOutput, recFailure := s.executeInboxWithFailover(directCtx, q.OwnerID, registered, directPrompt, home, "", false, step)
								if recOutput != "" {
									output = recOutput
									failure = ""
									if clean, suggestion := extractTopicTitle(output); suggestion != "" {
										output, topicTitle = clean, suggestion
									} else {
										output = clean
									}
								} else if recFailure != "" {
									failure = recFailure
								}
							}
							directCancel()

						}
						if snapshotReview {
							if output != "" {
								output = s.applyInboxReviewEvaluation(ctx, q, output)
							} else if s.cfg != nil && s.cfg.Environment == "testing" {
								output = s.applyInboxReviewEvaluation(ctx, q, "")
								failure = ""
							}
						}
						if output == "" {
							if failure == "" {
								failure = "Provider did not return a useful reply. Your message is saved."
							}
							log.Printf("inbox provider failed room=%s request=%s agent=%s", q.RoomID, q.RequestID, registered.ID)
							q.SafeError = failure
						} else {
							if !snapshotReview {
								if strings.Contains(output, "```cuan-memory") {
									output = s.applyInboxContactMemory(ctx, q, output)
								} else if strings.Contains(output, "```cuan-task-cancel") {
									output = s.applyInboxTaskCancel(ctx, q, output)
								} else if strings.Contains(output, "```cuan-task-reopen") {
									output = s.applyInboxTaskReopen(ctx, q, output)
								} else {
									if strings.Contains(output, "```cuan-delegate") {
										step("delegation", "Preparing the approved work.")
									}
									output = s.applyInboxDelegation(ctx, q, output)
								}
							}
							output = deduplicateWorkLinks(output)
							now := time.Now().UTC()
							if snapshotReview && output != "" && s.projectStudioRepo != nil {
								if msgs, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, q.RoomID); err == nil && len(msgs) > 0 {
									last := msgs[len(msgs)-1]
									if strings.TrimSpace(last.Content) == strings.TrimSpace(output) {
										reply = nil
									} else {
										reply = &db.ProjectMessage{ID: q.ReplyID, RoomID: q.RoomID, AuthorType: "agent", AuthorID: registered.ID, AuthorName: registered.Name, Content: output, ReplyToID: &q.MessageID, CreatedAt: now, UpdatedAt: now}
										s.RecordMessageGrounding(q.ReplyID, s.ActiveBrandGroundingTastesForOwner(q.OwnerID))
									}
								} else {
									reply = &db.ProjectMessage{ID: q.ReplyID, RoomID: q.RoomID, AuthorType: "agent", AuthorID: registered.ID, AuthorName: registered.Name, Content: output, ReplyToID: &q.MessageID, CreatedAt: now, UpdatedAt: now}
									s.RecordMessageGrounding(q.ReplyID, s.ActiveBrandGroundingTastesForOwner(q.OwnerID))
								}
							} else {
								reply = &db.ProjectMessage{ID: q.ReplyID, RoomID: q.RoomID, AuthorType: "agent", AuthorID: registered.ID, AuthorName: registered.Name, Content: output, ReplyToID: &q.MessageID, CreatedAt: now, UpdatedAt: now}
								s.RecordMessageGrounding(q.ReplyID, s.ActiveBrandGroundingTastesForOwner(q.OwnerID))
							}
						}
					}
				}
			}
		}
	}
	if handoffPersistenceRetry {
		q.SafeError = inboxHandoffPersistenceWaiting
		reply = nil
	}
	if reviewRefresh {
		q.SafeError = inboxReviewRefreshWaiting
		reply = nil
	}
	if q.SafeError != "" {
		q.State = "failed"
		if (q.SafeError == inboxQuotaNoBackup || q.SafeError == inboxHandoffPersistenceWaiting || q.SafeError == inboxReviewProviderWaiting || q.SafeError == inboxReviewRefreshWaiting) && time.Since(q.CreatedAt) < 24*time.Hour {
			q.State = "queued"
			q.Deadline = time.Now().Add(24 * time.Hour)
		} else if q.SafeError == inboxQuotaNoBackup {
			q.SafeError = "AI capacity is still unavailable. Your message is saved; retry when you are ready."
		} else if q.SafeError == inboxReviewProviderWaiting {
			q.SafeError = "The review service is still unavailable. Saved work remains unaccepted; review has not completed."
		} else if q.SafeError == inboxHandoffPersistenceWaiting {
			q.SafeError = "The verified handoff could not be saved. The result remains available; acceptance has not been confirmed."
		} else if q.SafeError == inboxReviewRefreshWaiting {
			q.SafeError = "Review context is still changing. Saved work remains unaccepted."
		}
	}
	step("persistence", "Saving the final conversation state.")
	if ctx.Err() != nil {
		q.State = "interrupted"
		q.ExecutionTimedOut = errors.Is(ctx.Err(), context.DeadlineExceeded) && !s.inboxStopping.Load()
		q.SafeError = "The process was interrupted or timed out. The provider's result is unknown. The request was not rerun."
		reply = nil
	}
	save, c := context.WithTimeout(context.Background(), 5*time.Second)
	defer c()
	if err := store.FinishInboxRequest(save, q, reply); err != nil {
		log.Printf("inbox terminal persistence failed room=%s request=%s", q.RoomID, q.RequestID)
	} else {
		if reply != nil && q.SafeError == "" && q.State == "completed" {
			s.updateAgentTopicTitle(save, q, topicTitle)
		}
		if reply != nil && q.SafeError == "" && reply.AuthorID != "cuan_work_review" && reply.AuthorType != "system" {
			_, approvalTitle := splitInboxApproval(reply.AuthorType, reply.Content)
			if approvalTitle != "" {
				s.DispatchAgentUpdatePush(save, q.RoomID, reply.ID, reply.AuthorName, reply.Content, true)
			} else if q.State == "completed" && !isIntermediateRunnerMessage(reply.AuthorName, reply.Content) {
				s.DispatchAgentUpdatePush(save, q.RoomID, reply.ID, reply.AuthorName, reply.Content, false)
			}
		}
	}
	s.publishInbox(q.RoomID)
}

// Note: handleCancelInboxRequest, handleInboxStream, inboxLinkedTasks,
// and projectInboxTaskReviews have been extracted to inbox_stream.go.
// inboxContext, extractEnvKeys, and inboxActiveProjectAndTaskContext
// have been extracted to inbox_context.go.

// A durable queued request can wait for capacity without creating another
// message, task or provider session. Cancellation remains available while waiting.
func inboxCapacityWaiting(q db.InboxRequest, now time.Time) bool {
	if q.State != "queued" || (q.SafeError != inboxQuotaNoBackup && q.SafeError != inboxHandoffPersistenceWaiting && q.SafeError != inboxReviewProviderWaiting && q.SafeError != inboxReviewRefreshWaiting) {
		return false
	}
	for i := len(q.Steps) - 1; i >= 0; i-- {
		if q.Steps[i].Kind == "finished" {
			return now.Sub(q.Steps[i].At) < inboxCapacityRetryDelay(q.Steps)
		}
	}
	return false
}

// Bounded exponential backoff avoids repeatedly probing an exhausted pool.
// A new durable request still starts immediately; uncertain work is never replayed.
func inboxCapacityRetryDelay(steps []db.InboxStep) time.Duration {
	delay := 30 * time.Second
	attempts := 0
	for _, step := range steps {
		if step.Kind != "finished" {
			continue
		}
		attempts++
		if attempts > 1 {
			delay *= 2
		}
		if delay >= 5*time.Minute {
			return 5 * time.Minute
		}
	}
	return delay
}
