package server

import (
	"github.com/go-chi/chi/v5"
)

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
	pr.Post("/chat/conversations/{id}/messages", s.handleInternalSendConversationMessage)
	pr.Get("/chat/conversations/{id}/events", s.handleInboxStream)
	pr.Post("/chat/conversations/{id}/messages/{messageID}/task", s.handleInboxMessageTask)
	pr.Post("/chat/conversations/{id}/requests/{requestID}/cancel", s.handleCancelInboxRequest)
	pr.Post("/chat/conversations/{id}/requests/{requestID}/retry", s.handleRetryInboxRequest)
}
