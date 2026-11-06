package server

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/task"
)

func TestExecutionInspectionIsAuthenticatedAndExcludesTaskContent(t *testing.T) {
	s, token, _, _ := inboxFixture(t)
	created, err := s.taskSvc.CreateTask(context.Background(), "owner", task.CreateTaskRequest{Title: "PRIVATE_TITLE", Instructions: "PRIVATE_INSTRUCTIONS"})
	if err != nil {
		t.Fatal(err)
	}
	for _, authorized := range []bool{false, true} {
		request := httptest.NewRequest("GET", "/api/v1/tasks/"+created.ID+"/execution", nil)
		if authorized {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		s.Router().ServeHTTP(response, request)
		if authorized && (response.Code != 200 || !strings.Contains(response.Body.String(), `"intent_kind":"no_attempt"`)) {
			t.Fatal("inspection unavailable", response.Code)
		}
		if !authorized && response.Code != 401 {
			t.Fatal("unauthenticated inspection allowed", response.Code)
		}
		if strings.Contains(response.Body.String(), "PRIVATE_") {
			t.Fatal("private content escaped diagnostic projection")
		}
	}
}
