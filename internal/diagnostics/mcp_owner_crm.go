package diagnostics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Normal Owner MCP and the private per-turn CRM snapshot are distinct transports.
// Bound task sessions cannot fall through to an ambient Owner client or database.
func ownerCRMClient() (*CuanClient, error) {
	if err := taskSessionOwnerWriteError(); err != nil {
		return nil, err
	}
	client := GetCuanClient()
	if client == nil || !client.IsConfigured() {
		return nil, errors.New("Connect Cuan to inspect its CRM")
	}
	return client, nil
}

func readOwnerCRM(ctx context.Context, client *CuanClient, path string, result any) error {
	response, err := client.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return errors.New("Cuan CRM could not be reached")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("Cuan CRM unavailable (HTTP %d)", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, (4<<20)+1))
	if err != nil || len(data) > 4<<20 || json.Unmarshal(data, result) != nil {
		return errors.New("Cuan CRM response unavailable or exceeds the inspection budget")
	}
	return nil
}

func ownerContacts(ctx context.Context, client *CuanClient) ([]ContactSummary, error) {
	var result struct {
		Contacts []struct {
			ContactSummary
			ConversationID string `json:"conversation_id"`
		} `json:"contacts"`
	}
	if err := readOwnerCRM(ctx, client, "/api/v1/contacts", &result); err != nil {
		return nil, err
	}
	if result.Contacts == nil {
		return nil, errors.New("Cuan CRM contact directory is missing")
	}
	contacts := make([]ContactSummary, 0, len(result.Contacts))
	for _, row := range result.Contacts {
		if row.ConversationID != "" {
			if row.SessionUUID != "" && row.SessionUUID != row.ConversationID {
				return nil, errors.New("Cuan CRM returned conflicting conversation identities")
			}
			row.SessionUUID = row.ConversationID
		}
		contacts = append(contacts, row.ContactSummary)
	}
	return contacts, nil
}

func ownerContactList(ctx context.Context, query, product string, limit int) (any, error) {
	client, err := ownerCRMClient()
	if err != nil {
		return nil, err
	}
	contacts, err := ownerContacts(ctx, client)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	query = strings.ToLower(strings.TrimSpace(query))
	matched := []ContactSummary{}
	for _, c := range contacts {
		if product != "" && c.Product != product {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(c.ID+"\n"+c.SessionUUID+"\n"+c.GuestUUID+"\n"+c.Name+"\n"+c.Email), query) {
			continue
		}
		matched = append(matched, c)
	}
	more := len(matched) > limit
	if more {
		matched = matched[:limit]
	}
	return map[string]any{"contacts": matched, "total": len(matched), "has_more": more, "observed_at": time.Now().UTC(), "source": "configured_owner_api", "untrusted_data": true}, nil
}

func ownerContactDetail(ctx context.Context, identity string) (any, error) {
	identity = strings.TrimSpace(identity)
	if identity == "" {
		return nil, errors.New("contact_id is required")
	}
	client, err := ownerCRMClient()
	if err != nil {
		return nil, err
	}
	contacts, err := ownerContacts(ctx, client)
	if err != nil {
		return nil, err
	}
	var found *ContactSummary
	for i := range contacts {
		c := &contacts[i]
		if c.ID != identity && c.SessionUUID != identity && c.GuestUUID != identity {
			continue
		}
		if found != nil {
			return nil, errors.New("Contact identity is ambiguous; use its exact contact ID")
		}
		found = c
	}
	if found == nil {
		return nil, errors.New("Contact was not found in the current connected CRM directory")
	}
	var notes struct {
		Items   []map[string]any `json:"items"`
		HasMore *bool            `json:"has_more"`
	}
	if err := readOwnerCRM(ctx, client, "/api/v1/contacts/"+url.PathEscape(found.ID)+"/memory", &notes); err != nil {
		return nil, err
	}
	if notes.HasMore == nil {
		return nil, errors.New("Contact relationship notes could not be verified")
	}
	if notes.Items == nil {
		notes.Items = []map[string]any{}
	}
	return map[string]any{"contact": found, "memories": notes.Items, "memories_loaded": true, "memory_has_more": *notes.HasMore, "observed_at": time.Now().UTC(), "source": "configured_owner_api", "untrusted_data": true}, nil
}
