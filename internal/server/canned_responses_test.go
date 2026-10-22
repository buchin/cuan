package server

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCannedResponses_CRUD_And_Validation(t *testing.T) {
	srv, token := setupTestChatServer(t)

	// 1. Unauthenticated request should be rejected
	reqNoAuth := httptest.NewRequest(http.MethodGet, "/api/v1/chat/canned-responses", nil)
	recNoAuth := httptest.NewRecorder()
	srv.Router().ServeHTTP(recNoAuth, reqNoAuth)
	if recNoAuth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated request, got %d", recNoAuth.Code)
	}

	// 2. Initial List (contains default canned responses)
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/chat/canned-responses", nil)
	reqList.Header.Set("Authorization", "Bearer "+token)
	recList := httptest.NewRecorder()
	srv.Router().ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 for canned responses list, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		CannedResponses []CannedResponse `json:"canned_responses"`
		Total           int              `json:"total"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal canned responses list: %v", err)
	}
	if listResp.Total < 4 {
		t.Fatalf("expected at least 4 default canned responses, got %d", listResp.Total)
	}

	// 3. Filter by query
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/v1/chat/canned-responses?q=halo", nil)
	reqFilter.Header.Set("Authorization", "Bearer "+token)
	recFilter := httptest.NewRecorder()
	srv.Router().ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 for filtered search, got %d", recFilter.Code)
	}
	var filterResp struct {
		CannedResponses []CannedResponse `json:"canned_responses"`
		Total           int              `json:"total"`
	}
	_ = json.Unmarshal(recFilter.Body.Bytes(), &filterResp)
	if filterResp.Total == 0 || !strings.Contains(strings.ToLower(filterResp.CannedResponses[0].Shortcut), "halo") {
		t.Fatalf("expected search for 'halo' to return results, got %+v", filterResp)
	}

	// 4. Validation: Empty shortcut
	invalidPayload := map[string]string{
		"shortcut": "",
		"title":    "Salam",
		"content":  "Halo",
	}
	b, _ := json.Marshal(invalidPayload)
	reqBad := httptest.NewRequest(http.MethodPost, "/api/v1/chat/canned-responses", bytes.NewReader(b))
	reqBad.Header.Set("Authorization", "Bearer "+token)
	reqBad.Header.Set("Content-Type", "application/json")
	recBad := httptest.NewRecorder()
	srv.Router().ServeHTTP(recBad, reqBad)
	if recBad.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty shortcut, got %d", recBad.Code)
	}

	// 5. Validation: Duplicate shortcut
	dupPayload := map[string]string{
		"shortcut": "halo",
		"title":    "Salam Duplikat",
		"content":  "Halo lagi!",
	}
	b, _ = json.Marshal(dupPayload)
	reqDup := httptest.NewRequest(http.MethodPost, "/api/v1/chat/canned-responses", bytes.NewReader(b))
	reqDup.Header.Set("Authorization", "Bearer "+token)
	reqDup.Header.Set("Content-Type", "application/json")
	recDup := httptest.NewRecorder()
	srv.Router().ServeHTTP(recDup, reqDup)
	if recDup.Code != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate shortcut, got %d: %s", recDup.Code, recDup.Body.String())
	}

	// 6. Create new canned response
	createPayload := map[string]string{
		"shortcut": "promo",
		"title":    "Promo Spesial",
		"content":  "Halo! Gunakan kode CUANHEMAT untuk potongan 25%.",
		"category": "sales",
	}
	b, _ = json.Marshal(createPayload)
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/chat/canned-responses", bytes.NewReader(b))
	reqCreate.Header.Set("Authorization", "Bearer "+token)
	reqCreate.Header.Set("Content-Type", "application/json")
	recCreate := httptest.NewRecorder()
	srv.Router().ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 for create canned response, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var created CannedResponse
	if err := json.Unmarshal(recCreate.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to decode created canned response: %v", err)
	}
	if created.ID == "" || created.Shortcut != "promo" || created.Title != "Promo Spesial" {
		t.Fatalf("unexpected created response: %+v", created)
	}

	// 7. Get by ID
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/chat/canned-responses/"+created.ID, nil)
	reqGet.Header.Set("Authorization", "Bearer "+token)
	recGet := httptest.NewRecorder()
	srv.Router().ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 for get canned response, got %d", recGet.Code)
	}

	// 8. Update canned response
	updatePayload := map[string]string{
		"shortcut": "promo-baru",
		"title":    "Promo Akhir Tahun",
		"content":  "Diskon hingga 50% untuk semua paket!",
		"category": "sales",
	}
	b, _ = json.Marshal(updatePayload)
	reqUpdate := httptest.NewRequest(http.MethodPut, "/api/v1/chat/canned-responses/"+created.ID, bytes.NewReader(b))
	reqUpdate.Header.Set("Authorization", "Bearer "+token)
	reqUpdate.Header.Set("Content-Type", "application/json")
	recUpdate := httptest.NewRecorder()
	srv.Router().ServeHTTP(recUpdate, reqUpdate)
	if recUpdate.Code != http.StatusOK {
		t.Fatalf("expected 200 for update canned response, got %d: %s", recUpdate.Code, recUpdate.Body.String())
	}

	var updated CannedResponse
	_ = json.Unmarshal(recUpdate.Body.Bytes(), &updated)
	if updated.Shortcut != "promo-baru" || updated.Title != "Promo Akhir Tahun" {
		t.Fatalf("unexpected updated fields: %+v", updated)
	}

	// 9. Delete canned response
	reqDelete := httptest.NewRequest(http.MethodDelete, "/api/v1/chat/canned-responses/"+created.ID, nil)
	reqDelete.Header.Set("Authorization", "Bearer "+token)
	recDelete := httptest.NewRecorder()
	srv.Router().ServeHTTP(recDelete, reqDelete)
	if recDelete.Code != http.StatusOK {
		t.Fatalf("expected 200 for delete canned response, got %d", recDelete.Code)
	}

	// 10. Verify deleted
	reqGetAfter := httptest.NewRequest(http.MethodGet, "/api/v1/chat/canned-responses/"+created.ID, nil)
	reqGetAfter.Header.Set("Authorization", "Bearer "+token)
	recGetAfter := httptest.NewRecorder()
	srv.Router().ServeHTTP(recGetAfter, reqGetAfter)
	if recGetAfter.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for deleted canned response, got %d", recGetAfter.Code)
	}
}

func TestOperatorAttachmentRestriction_CustomerVsAgent(t *testing.T) {
	srv, token := setupTestChatServer(t)

	// 1. Initialize visitor chat session
	visitorPayload := map[string]interface{}{
		"guest_uuid": "guest-test-111",
		"product":    "cuan",
		"name":       "Customer Satu",
		"email":      "customer@test.com",
		"message":    "Halo, saya ada kendala.",
	}
	b, _ := json.Marshal(visitorPayload)
	reqInit := httptest.NewRequest(http.MethodPost, "/api/chat/session", bytes.NewReader(b))
	reqInit.Header.Set("Content-Type", "application/json")
	recInit := httptest.NewRecorder()
	srv.Router().ServeHTTP(recInit, reqInit)
	if recInit.Code != http.StatusOK {
		t.Fatalf("failed to init visitor session: %d", recInit.Code)
	}

	var sessResp struct {
		Session struct {
			UUID string `json:"uuid"`
		} `json:"session"`
	}
	_ = json.Unmarshal(recInit.Body.Bytes(), &sessResp)
	sessionUUID := sessResp.Session.UUID

	// 2. Operator attempts to upload attachment specifying customer conversation
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.txt")
	_, _ = part.Write([]byte("Hello world from operator"))
	_ = writer.WriteField("conversation_id", sessionUUID)
	_ = writer.Close()

	reqUploadCust := httptest.NewRequest(http.MethodPost, "/api/v1/chat/attachments", body)
	reqUploadCust.Header.Set("Authorization", "Bearer "+token)
	reqUploadCust.Header.Set("Content-Type", writer.FormDataContentType())
	recUploadCust := httptest.NewRecorder()
	srv.Router().ServeHTTP(recUploadCust, reqUploadCust)

	if recUploadCust.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden when operator uploads to customer conversation, got %d: %s",
			recUploadCust.Code, recUploadCust.Body.String())
	}
	if !strings.Contains(recUploadCust.Body.String(), "customer-to-operator only") {
		t.Fatalf("expected policy explanation in error message, got: %s", recUploadCust.Body.String())
	}

	// 3. Operator uploads file for agent conversation (without customer conversation_id)
	bodyAgent := &bytes.Buffer{}
	writerAgent := multipart.NewWriter(bodyAgent)
	partAgent, _ := writerAgent.CreateFormFile("file", "spec.txt")
	_, _ = partAgent.Write([]byte("Agent specification"))
	_ = writerAgent.Close()

	reqUploadAgent := httptest.NewRequest(http.MethodPost, "/api/v1/chat/attachments", bodyAgent)
	reqUploadAgent.Header.Set("Authorization", "Bearer "+token)
	reqUploadAgent.Header.Set("Content-Type", writerAgent.FormDataContentType())
	recUploadAgent := httptest.NewRecorder()
	srv.Router().ServeHTTP(recUploadAgent, reqUploadAgent)

	if recUploadAgent.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for general/agent upload, got %d: %s",
			recUploadAgent.Code, recUploadAgent.Body.String())
	}

	var agentAtt conversationAttachment
	_ = json.Unmarshal(recUploadAgent.Body.Bytes(), &agentAtt)

	// 4. Operator attempts to send message with attachment to customer conversation
	replyCustWithAtt := map[string]interface{}{
		"message":        "Here is the file you asked for",
		"attachment_ids": []string{agentAtt.ID},
		"author_type":    "agent",
		"author_name":    "Operator",
	}
	bReply, _ := json.Marshal(replyCustWithAtt)
	reqSendAtt := httptest.NewRequest(http.MethodPost, "/api/v1/chat/conversations/"+sessionUUID+"/messages", bytes.NewReader(bReply))
	reqSendAtt.Header.Set("Authorization", "Bearer "+token)
	reqSendAtt.Header.Set("Content-Type", "application/json")
	recSendAtt := httptest.NewRecorder()
	srv.Router().ServeHTTP(recSendAtt, reqSendAtt)

	if recSendAtt.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden when operator sends attachment to customer, got %d: %s",
			recSendAtt.Code, recSendAtt.Body.String())
	}

	// 5. Operator sends text reply without attachment -> Should succeed
	replyClean := map[string]interface{}{
		"message":     "Halo! Kami siap membantu Anda.",
		"author_type": "agent",
		"author_name": "Operator",
	}
	bClean, _ := json.Marshal(replyClean)
	reqSendClean := httptest.NewRequest(http.MethodPost, "/api/v1/chat/conversations/"+sessionUUID+"/messages", bytes.NewReader(bClean))
	reqSendClean.Header.Set("Authorization", "Bearer "+token)
	reqSendClean.Header.Set("Content-Type", "application/json")
	recSendClean := httptest.NewRecorder()
	srv.Router().ServeHTTP(recSendClean, reqSendClean)

	if recSendClean.Code != http.StatusOK {
		t.Fatalf("expected 200 for clean text operator reply, got %d: %s",
			recSendClean.Code, recSendClean.Body.String())
	}
}

func TestVisitorToOperatorAttachmentFlow(t *testing.T) {
	srv, token := setupTestChatServer(t)

	// 1. Initialize visitor chat session
	visitorPayload := map[string]interface{}{
		"guest_uuid": "guest-attachment-user",
		"product":    "cuan",
		"name":       "Visitor Lampiran",
		"email":      "visitor@test.com",
		"message":    "Saya ingin melampirkan bukti transfer.",
	}
	b, _ := json.Marshal(visitorPayload)
	reqInit := httptest.NewRequest(http.MethodPost, "/api/chat/session", bytes.NewReader(b))
	reqInit.Header.Set("Content-Type", "application/json")
	recInit := httptest.NewRecorder()
	srv.Router().ServeHTTP(recInit, reqInit)
	if recInit.Code != http.StatusOK {
		t.Fatalf("failed to init visitor session: %d", recInit.Code)
	}

	var sessResp struct {
		Session struct {
			UUID string `json:"uuid"`
		} `json:"session"`
	}
	_ = json.Unmarshal(recInit.Body.Bytes(), &sessResp)
	sessionUUID := sessResp.Session.UUID

	// 2. Visitor uploads attachment via POST /api/chat/attachments (Open CORS)
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "bukti_transfer.txt")
	_, _ = part.Write([]byte("TRX-ID: 9948271 Invoice Paid"))
	_ = writer.WriteField("session_uuid", sessionUUID)
	_ = writer.Close()

	reqVisUpload := httptest.NewRequest(http.MethodPost, "/api/chat/attachments", body)
	reqVisUpload.Header.Set("Content-Type", writer.FormDataContentType())
	recVisUpload := httptest.NewRecorder()
	srv.Router().ServeHTTP(recVisUpload, reqVisUpload)

	if recVisUpload.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for visitor upload, got %d: %s",
			recVisUpload.Code, recVisUpload.Body.String())
	}

	var visAtt conversationAttachment
	if err := json.Unmarshal(recVisUpload.Body.Bytes(), &visAtt); err != nil {
		t.Fatalf("failed to decode visitor attachment: %v", err)
	}
	if visAtt.ID == "" || visAtt.URL == "" {
		t.Fatalf("expected valid visitor attachment info: %+v", visAtt)
	}

	// 3. Visitor sends message referencing attachment
	visMsgPayload := map[string]interface{}{
		"session_uuid":    sessionUUID,
		"message":         "Ini bukti transfer saya",
		"attachment_url":  visAtt.URL,
		"attachment_name": visAtt.Name,
		"attachment_type": visAtt.Type,
		"attachment_size": visAtt.Size,
	}
	bMsg, _ := json.Marshal(visMsgPayload)
	reqVisMsg := httptest.NewRequest(http.MethodPost, "/api/chat/messages", bytes.NewReader(bMsg))
	reqVisMsg.Header.Set("Content-Type", "application/json")
	recVisMsg := httptest.NewRecorder()
	srv.Router().ServeHTTP(recVisMsg, reqVisMsg)

	if recVisMsg.Code != http.StatusOK {
		t.Fatalf("expected 200 for visitor message with attachment, got %d: %s",
			recVisMsg.Code, recVisMsg.Body.String())
	}

	// 4. Operator downloads visitor attachment via GET /api/v1/chat/attachments/{id}
	reqOpDl := httptest.NewRequest(http.MethodGet, "/api/v1/chat/attachments/"+visAtt.ID, nil)
	reqOpDl.Header.Set("Authorization", "Bearer "+token)
	recOpDl := httptest.NewRecorder()
	srv.Router().ServeHTTP(recOpDl, reqOpDl)
	if recOpDl.Code != http.StatusOK {
		t.Fatalf("expected operator to be able to download visitor attachment, got %d", recOpDl.Code)
	}

	// 5. Visitor downloads attachment via GET /api/chat/attachments/{id}
	reqVisDl := httptest.NewRequest(http.MethodGet, "/api/chat/attachments/"+visAtt.ID, nil)
	recVisDl := httptest.NewRecorder()
	srv.Router().ServeHTTP(recVisDl, reqVisDl)
	if recVisDl.Code != http.StatusOK {
		t.Fatalf("expected visitor to be able to download attachment, got %d", recVisDl.Code)
	}
}

func TestE2E_CustomerChatWithCannedResponseAndAttachmentRestriction(t *testing.T) {
	srv, token := setupTestChatServer(t)

	// Step 1: Customer initiates live chat session with initial message
	visitorPayload := map[string]interface{}{
		"guest_uuid": "guest-e2e-fase3",
		"product":    "cuan",
		"name":       "Dodi Customer",
		"email":      "dodi@example.com",
		"message":    "Halo, bagaimana cara berlangganan?",
	}
	bInit, _ := json.Marshal(visitorPayload)
	reqInit := httptest.NewRequest(http.MethodPost, "/api/chat/session", bytes.NewReader(bInit))
	reqInit.Header.Set("Content-Type", "application/json")
	recInit := httptest.NewRecorder()
	srv.Router().ServeHTTP(recInit, reqInit)
	if recInit.Code != http.StatusOK {
		t.Fatalf("step 1 failed: visitor init: %d %s", recInit.Code, recInit.Body.String())
	}

	var sessResp struct {
		Session struct {
			UUID string `json:"uuid"`
		} `json:"session"`
	}
	_ = json.Unmarshal(recInit.Body.Bytes(), &sessResp)
	sessionUUID := sessResp.Session.UUID

	// Step 2: Customer uploads payment / inquiry screenshot
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "inquiry.txt")
	_, _ = part.Write([]byte("Paket Pro 1 Tahun"))
	_ = writer.WriteField("session_uuid", sessionUUID)
	_ = writer.Close()

	reqUpload := httptest.NewRequest(http.MethodPost, "/api/chat/attachments", body)
	reqUpload.Header.Set("Content-Type", writer.FormDataContentType())
	recUpload := httptest.NewRecorder()
	srv.Router().ServeHTTP(recUpload, reqUpload)
	if recUpload.Code != http.StatusCreated {
		t.Fatalf("step 2 failed: customer upload: %d", recUpload.Code)
	}

	var att conversationAttachment
	_ = json.Unmarshal(recUpload.Body.Bytes(), &att)

	// Customer sends message with attachment
	msgPayload := map[string]interface{}{
		"session_uuid":    sessionUUID,
		"message":         "Saya lampirkan rincian paket yang diinginkan",
		"attachment_url":  att.URL,
		"attachment_name": att.Name,
		"attachment_type": att.Type,
		"attachment_size": att.Size,
	}
	bMsg, _ := json.Marshal(msgPayload)
	reqMsg := httptest.NewRequest(http.MethodPost, "/api/chat/messages", bytes.NewReader(bMsg))
	reqMsg.Header.Set("Content-Type", "application/json")
	recMsg := httptest.NewRecorder()
	srv.Router().ServeHTTP(recMsg, reqMsg)
	if recMsg.Code != http.StatusOK {
		t.Fatalf("step 2 failed: customer send message with attachment: %d", recMsg.Code)
	}

	// Step 3: Operator views conversation in Inbox
	reqConv := httptest.NewRequest(http.MethodGet, "/api/v1/chat/conversations/"+sessionUUID+"/messages", nil)
	reqConv.Header.Set("Authorization", "Bearer "+token)
	recConv := httptest.NewRecorder()
	srv.Router().ServeHTTP(recConv, reqConv)
	if recConv.Code != http.StatusOK {
		t.Fatalf("step 3 failed: operator get conversation: %d", recConv.Code)
	}

	var convResp struct {
		Messages []ChatMessageDTO `json:"messages"`
	}
	_ = json.Unmarshal(recConv.Body.Bytes(), &convResp)
	if len(convResp.Messages) < 2 {
		t.Fatalf("step 3 failed: expected at least 2 messages in conversation, got %d", len(convResp.Messages))
	}

	// Step 4: Operator attempts to upload attachment to this customer conversation -> MUST BE FORBIDDEN
	bodyOp := &bytes.Buffer{}
	writerOp := multipart.NewWriter(bodyOp)
	partOp, _ := writerOp.CreateFormFile("file", "invoice.pdf")
	_, _ = partOp.Write([]byte("%PDF-dummy-data"))
	_ = writerOp.WriteField("conversation_id", sessionUUID)
	_ = writerOp.Close()

	reqOpUpload := httptest.NewRequest(http.MethodPost, "/api/v1/chat/attachments", bodyOp)
	reqOpUpload.Header.Set("Authorization", "Bearer "+token)
	reqOpUpload.Header.Set("Content-Type", writerOp.FormDataContentType())
	recOpUpload := httptest.NewRecorder()
	srv.Router().ServeHTTP(recOpUpload, reqOpUpload)
	if recOpUpload.Code != http.StatusForbidden {
		t.Fatalf("step 4 failed: expected 403 Forbidden for operator upload to customer, got %d", recOpUpload.Code)
	}

	// Step 5: Operator fetches canned response "harga" (template balasan cepat)
	reqCanned := httptest.NewRequest(http.MethodGet, "/api/v1/chat/canned-responses?q=harga", nil)
	reqCanned.Header.Set("Authorization", "Bearer "+token)
	recCanned := httptest.NewRecorder()
	srv.Router().ServeHTTP(recCanned, reqCanned)
	if recCanned.Code != http.StatusOK {
		t.Fatalf("step 5 failed: fetch canned response: %d", recCanned.Code)
	}

	var cannedList struct {
		CannedResponses []CannedResponse `json:"canned_responses"`
	}
	_ = json.Unmarshal(recCanned.Body.Bytes(), &cannedList)
	if len(cannedList.CannedResponses) == 0 {
		t.Fatalf("step 5 failed: no canned response found for 'harga'")
	}
	cannedReplyText := cannedList.CannedResponses[0].Content

	// Step 6: Operator sends canned reply to customer
	opReplyPayload := map[string]interface{}{
		"message":     cannedReplyText,
		"author_type": "agent",
		"author_name": "CS Cuan",
	}
	bOpReply, _ := json.Marshal(opReplyPayload)
	reqOpSend := httptest.NewRequest(http.MethodPost, "/api/v1/chat/conversations/"+sessionUUID+"/messages", bytes.NewReader(bOpReply))
	reqOpSend.Header.Set("Authorization", "Bearer "+token)
	reqOpSend.Header.Set("Content-Type", "application/json")
	recOpSend := httptest.NewRecorder()
	srv.Router().ServeHTTP(recOpSend, reqOpSend)
	if recOpSend.Code != http.StatusOK {
		t.Fatalf("step 6 failed: operator sending canned reply: %d %s", recOpSend.Code, recOpSend.Body.String())
	}

	// Step 7: Visitor polls for messages and receives operator canned reply
	reqPoll := httptest.NewRequest(http.MethodGet, "/api/chat/messages?session_uuid="+sessionUUID+"&after_id=2", nil)
	recPoll := httptest.NewRecorder()
	srv.Router().ServeHTTP(recPoll, reqPoll)
	if recPoll.Code != http.StatusOK {
		t.Fatalf("step 7 failed: visitor poll messages: %d", recPoll.Code)
	}

	var pollResp struct {
		OK       bool             `json:"ok"`
		Messages []ChatMessageDTO `json:"messages"`
	}
	_ = json.Unmarshal(recPoll.Body.Bytes(), &pollResp)
	if !pollResp.OK || len(pollResp.Messages) == 0 {
		t.Fatalf("step 7 failed: expected new messages for visitor: %+v", pollResp)
	}
	if pollResp.Messages[0].Message != cannedReplyText {
		t.Fatalf("step 7 failed: expected operator message to match canned text, got %q vs %q",
			pollResp.Messages[0].Message, cannedReplyText)
	}
}
