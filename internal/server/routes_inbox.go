package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// registerVisitorChatRoutes mounts public Dojo-compatible live chat widget and visitor APIs.
func (s *Server) registerVisitorChatRoutes(r chi.Router) {
	r.Post("/api/webhooks/resend", s.handleResendReceiptWebhook)
	r.With(chatCORSMiddleware).Get("/chat/widget.js", s.handleChatWidgetJS)
	r.With(chatCORSMiddleware).Options("/chat/widget.js", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r.Route("/api/chat", func(cr chi.Router) {
		cr.Use(chatCORSMiddleware)
		cr.Post("/session", s.handleVisitorChatSession)
		cr.Get("/messages", s.handleVisitorGetMessages)
		cr.Post("/messages", s.handleVisitorSendMessage)
		cr.Post("/attachments", s.handleVisitorUploadAttachment)
		cr.Get("/attachments/{id}", s.handleVisitorDownloadAttachment)
		cr.Post("/sessions/{uuid}/page", s.handleVisitorPageTelemetry)
		cr.Post("/receipts", s.handleVisitorReceipts)
		cr.Post("/idle/check", s.handleTriggerChatIdleCheck)
		cr.Get("/contacts", s.handleInternalListContacts)
		cr.Get("/conversations", s.handleInternalListContacts)
		cr.Get("/canned-responses", s.handleListCannedResponses)
		cr.Get("/conversations/{id}/messages", s.handleInternalGetConversationMessages)
		cr.Get("/conversations/{id}/grounding", s.handleGetConversationTasteGrounding)
		cr.Post("/conversations/{id}/messages", s.handleInternalSendConversationMessage)
		cr.Get("/conversations/{id}/events", s.handleInboxStream)
		cr.Post("/conversations/{id}/messages/{messageID}/task", s.handleInboxMessageTask)
		cr.Post("/conversations/{id}/requests/{requestID}/cancel", s.handleCancelInboxRequest)
		cr.Post("/conversations/{id}/requests/{requestID}/retry", s.handleRetryInboxRequest)
	})
}

// registerInboxRoutes mounts conversational inbox, chat messages, canned responses, attachments, and approvals.
func (s *Server) registerInboxRoutes(pr chi.Router) {
	// Conversational Inbox Engine & Thread Management (Sprint 17 S17.2 & S17.3)
	pr.Get("/inbox", s.handleListInboxThreads)
	pr.Post("/inbox", s.handleCreateInboxThread)
	pr.Get("/inbox/{id}", s.handleGetInboxThread)
	pr.Post("/inbox/{id}/messages", s.handleCreateInboxMessage)
	pr.Post("/inbox/{id}/approve", s.handleApproveInboxThread)

	// Direct Messenger Hub (Sprint 18)
	pr.Get("/extensions", s.handleExtensions)
	pr.Post("/extensions/{extensionID}/actions/{action}", s.handleExtensionAction)
	pr.Get("/search", s.handleUniversalSearch)
	pr.Get("/contacts/search", s.handleCRMSearch)
	pr.Get("/contacts/{id}/memory", s.handleContactMemory)
	pr.Post("/contacts/{id}/memory", s.handleContactMemory)
	pr.Patch("/contacts/{id}/memory/{memoryID}", s.handleContactMemoryChange)
	pr.Delete("/contacts/{id}/memory/{memoryID}", s.handleContactMemoryChange)
	pr.Get("/contacts", s.handleCRMList)
	pr.Post("/contacts", s.handleCRMCreate)
	pr.Patch("/contacts/{id}", s.handleCRMUpdate)
	pr.Get("/chat/contacts", s.handleInternalListContacts)
	pr.Get("/chat/conversations", s.handleInternalListContacts)
	pr.Get("/inbox/conversations", s.handleInternalListContacts)
	pr.Get("/chat/canned-responses", s.handleListCannedResponses)
	pr.Post("/chat/canned-responses", s.handleCreateCannedResponse)
	pr.Get("/chat/canned-responses/{id}", s.handleGetCannedResponse)
	pr.Put("/chat/canned-responses/{id}", s.handleUpdateCannedResponse)
	pr.Patch("/chat/canned-responses/{id}", s.handleUpdateCannedResponse)
	pr.Delete("/chat/canned-responses/{id}", s.handleDeleteCannedResponse)
	pr.Post("/chat/conversations", s.handleCreateAgentTopic)
	pr.Post("/studio/goals", s.handleStudioGoal)
	pr.Post("/chat/attachments", s.handleUploadConversationAttachment)
	pr.Get("/chat/attachments/{id}", s.handleDownloadConversationAttachment)
	pr.Get("/chat/conversations/{id}/messages", s.handleInternalGetConversationMessages)
	pr.Post("/chat/conversations/{id}/receipts", s.handleOwnerChatReceipts)
	pr.Get("/chat/conversations/{id}/grounding", s.handleGetConversationTasteGrounding)
	pr.Post("/chat/conversations/{id}/messages", s.handleInternalSendConversationMessage)
	pr.Get("/chat/conversations/{id}/events", s.handleInboxStream)
	pr.Post("/chat/conversations/{id}/messages/{messageID}/task", s.handleInboxMessageTask)
	pr.Post("/chat/conversations/{id}/requests/{requestID}/cancel", s.handleCancelInboxRequest)
	pr.Post("/chat/conversations/{id}/requests/{requestID}/retry", s.handleRetryInboxRequest)
}
