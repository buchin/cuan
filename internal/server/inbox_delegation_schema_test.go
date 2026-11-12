package server

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestCoordinatorDelegationSchemaMatchesNativePayload(t *testing.T) {
	var schema struct {
		Properties map[string]struct {
			AnyOf []struct {
				Properties map[string]json.RawMessage `json:"properties"`
			} `json:"anyOf"`
		} `json:"properties"`
	}
	if err := json.Unmarshal([]byte(inboxCoordinatorSchema), &schema); err != nil {
		t.Fatal(err)
	}
	delegationBranches := 0
	for _, branch := range schema.Properties["action"].AnyOf {
		var kind struct {
			Enum []string `json:"enum"`
		}
		_ = json.Unmarshal(branch.Properties["kind"], &kind)
		for _, name := range kind.Enum {
			if name != "delegation" {
				continue
			}
			delegationBranches++
			var payload struct {
				Properties map[string]struct {
					Type string `json:"type"`
				} `json:"properties"`
				AdditionalProperties bool `json:"additionalProperties"`
			}
			if err := json.Unmarshal(branch.Properties["payload"], &payload); err != nil {
				t.Fatal(err)
			}
			native := reflect.TypeOf(inboxDelegation{})
			if payload.AdditionalProperties || len(payload.Properties) != native.NumField() {
				t.Fatal("provider accepts free-form delegation rejected by native parser")
			}
			for i := 0; i < native.NumField(); i++ {
				field := native.Field(i)
				key := strings.Split(field.Tag.Get("json"), ",")[0]
				want := "string"
				switch field.Type.Kind() {
				case reflect.Bool:
					want = "boolean"
				case reflect.Slice:
					want = "array"
				}
				if payload.Properties[key].Type != want {
					t.Fatalf("provider/native field mismatch: %s", key)
				}
			}
		}
	}
	if delegationBranches != 1 {
		t.Fatal("delegation must use exactly one typed action branch")
	}
}

func TestDelegationFormatRepairSuppliesNativeFieldContract(t *testing.T) {
	candidate := coordinatorJSON("Perbaiki pekerjaan yang disetujui.", "delegation", map[string]any{"title": "Original work", "description": "Preserve original criteria", "project_id": "registered", "authorization_message_id": "owner", "acceptance_criteria": []string{"Original criterion"}})
	if _, err := decodeInboxCoordinatorResponse(candidate); err == nil {
		t.Fatal("native parser accepted unsupported delegation fields")
	}
	if prompt := inboxFormatRepairPrompt(candidate, false); !strings.Contains(prompt, inboxDelegationPayloadSchema) {
		t.Fatal("formatter still lacks the native delegation contract")
	}
	valid := coordinatorJSON("Hasil akan ditinjau.", "delegation", inboxDelegation{Title: "Original work", Description: "Preserve original criteria", ProjectID: "registered", AuthorizationID: "owner"})
	output, err := decodeInboxCoordinatorResponse(valid)
	if err != nil {
		t.Fatal(err)
	}
	_, delegation, err := parseInboxDelegation(output)
	if err != nil || delegation == nil || delegation.ProjectID != "registered" || delegation.AuthorizationID != "owner" {
		t.Fatal("valid repaired payload lost original binding", err)
	}
}

func TestDelegationRecoveryDoesNotReportInvalidLaunchAsSuccess(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		payload := map[string]any{"title": "Original work", "description": "Preserve original criteria", "project_id": "registered", "authorization_message_id": "owner"}
		if invalid {
			payload["acceptance_criteria"] = []string{"Original criterion"}
		}
		raw, _ := json.Marshal(map[string]any{"reply": "I will continue the approved work.", "topic_title": "", "extra_transport": true, "action": map[string]any{"kind": "delegation", "payload": payload}})
		out, failure := recoverCoordinatorReply(string(raw), false)
		if invalid {
			if out != "" || failure == "" {
				t.Fatal("invalid delegation was salvaged as a successful reply", out)
			}
		} else {
			_, delegation, err := parseInboxDelegation(out)
			if failure != "" || err != nil || delegation == nil || delegation.AuthorizationID != "owner" {
				t.Fatal("valid transport recovery lost approved work", failure, err)
			}
		}
	}
}
