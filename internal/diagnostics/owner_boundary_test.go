package diagnostics

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type forbiddenOwnerConnector struct{ connects *int }

func (c forbiddenOwnerConnector) Connect(context.Context) (driver.Conn, error) {
	*c.connects++
	return nil, errors.New("unexpected owner database access")
}
func (c forbiddenOwnerConnector) Driver() driver.Driver { return forbiddenOwnerDriver{} }

type forbiddenOwnerDriver struct{}

func (forbiddenOwnerDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("unexpected database open")
}

func TestBoundTaskCannotWriteAsOwner(t *testing.T) {
	for _, capability := range []string{"CUAN_TASK_CAPABILITY_FILE", "CUAN_TASK_CAPABILITY_JSON"} {
		t.Run(capability, func(t *testing.T) {
			t.Setenv("CUAN_TASK_CAPABILITY_FILE", "")
			t.Setenv("CUAN_TASK_CAPABILITY_JSON", "")
			// Even an unreadable/malformed capability must not fall back to Owner.
			t.Setenv(capability, "bound-task")
			requests, connects := 0, 0
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{}`))
			}))
			defer api.Close()
			client := &CuanClient{BaseURL: api.URL, AuthToken: "ambient-owner", HTTPClient: api.Client()}
			oldClient, oldDB := globalCuanClient, platformDB
			database := sql.OpenDB(forbiddenOwnerConnector{&connects})
			defer database.Close()
			platformDB = database
			t.Cleanup(func() { globalCuanClient = oldClient; platformDB = oldDB })
			ctx := context.Background()
			operations := []func() (any, error){
				func() (any, error) { return SendInboxMessage(ctx, "room", "Worker result", false, 0) },
				func() (any, error) { return CreateInboxConversation(ctx, "agent_cuan", "project", "Work") },
				func() (any, error) {
					return CreateTaskDraft(ctx, "project", "Unauthorized work", "Do work", "normal", nil)
				},
				func() (any, error) { return UpdateTaskStatus(ctx, "task", "done", "Result", "Verified") },
				func() (any, error) { return ToggleAgentSkill(ctx, "agent_cuan", "skill", true) },
			}
			// Exercise both an already-cached owner client and the database fallback.
			for _, configured := range []bool{true, false} {
				globalCuanClient = client
				if !configured {
					globalCuanClient = &CuanClient{}
				}
				for _, call := range operations {
					if _, err := call(); err == nil || !strings.Contains(err.Error(), "task report") {
						t.Fatalf("missing task-report redirect: %v", err)
					}
				}
			}
			for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
				if _, err := client.doRequest(ctx, method, "/mutation", nil); err == nil {
					t.Fatalf("%s reached Owner API", method)
				}
			}
			if requests != 0 || connects != 0 {
				t.Fatalf("owner side effects: HTTP=%d DB=%d", requests, connects)
			}
			// Read-only diagnostics remain available for an approved audit.
			response, err := client.doRequest(ctx, http.MethodGet, "/inspection", nil)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if requests != 1 {
				t.Fatal("read-only inspection was blocked")
			}
			home := t.TempDir()
			t.Setenv("HOME", home)
			var out, errOut bytes.Buffer
			if code := RunConfigCLI([]string{"set", "auth_token", "replacement"}, &out, &errOut); code == 0 {
				t.Fatal("task overwrote host owner credentials")
			}
			if _, err := os.Stat(filepath.Join(home, ".cuan", "config.json")); !os.IsNotExist(err) {
				t.Fatal("config was written")
			}
			var mcpOut, mcpErr bytes.Buffer
			request := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"send_inbox_message","arguments":{"room_id":"room","message":"Worker output"}}}` + "\n"
			if code := RunMCP(strings.NewReader(request), &mcpOut, &mcpErr); code != 0 {
				t.Fatalf("MCP transport failed: %s", mcpErr.String())
			}
			if !strings.Contains(mcpOut.String(), "task report") {
				t.Fatalf("MCP lost actionable denial: %s", mcpOut.String())
			}
		})
	}
}

func TestOwnerCanStillSendInboxMessage(t *testing.T) {
	t.Setenv("CUAN_TASK_CAPABILITY_FILE", "")
	t.Setenv("CUAN_TASK_CAPABILITY_JSON", "")
	requests := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("Authorization") != "Bearer owner-token" || r.Method != http.MethodPost {
			t.Error("owner identity/method changed")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"message_id":"owner-message"}`))
	}))
	defer api.Close()
	old := globalCuanClient
	t.Cleanup(func() { globalCuanClient = old })
	globalCuanClient = &CuanClient{BaseURL: api.URL, AuthToken: "owner-token", HTTPClient: api.Client()}
	if _, err := SendInboxMessage(context.Background(), "room", "My actual instruction", false, 0); err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatalf("expected one owner message, got %d", requests)
	}
}

func TestInboxSendNeverFabricatesOwnerThroughDatabase(t *testing.T) {
	t.Setenv("CUAN_TASK_CAPABILITY_FILE", "")
	t.Setenv("CUAN_TASK_CAPABILITY_JSON", "")
	connects := 0
	database := sql.OpenDB(forbiddenOwnerConnector{&connects})
	defer database.Close()
	oldClient, oldDB := globalCuanClient, platformDB
	t.Cleanup(func() { globalCuanClient = oldClient; platformDB = oldDB })
	globalCuanClient = &CuanClient{}
	platformDB = database
	if _, err := SendInboxMessage(context.Background(), "room", "Unauthenticated message", false, 0); err == nil || !strings.Contains(err.Error(), "authenticated Cuan API") {
		t.Fatalf("missing authenticated owner boundary: %v", err)
	}
	if connects != 0 {
		t.Fatal("owner was fabricated through host database")
	}
}

func TestBoundTaskMCPAdvertisesInspectionWithoutOwnerWrites(t *testing.T) {
	t.Setenv("CUAN_TASK_CAPABILITY_FILE", "bound-task")
	var out, errOut bytes.Buffer
	if code := RunMCP(strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`+"\n"), &out, &errOut); code != 0 {
		t.Fatal(errOut.String())
	}
	for _, name := range []string{"send_inbox_message", "create_inbox_conversation", "create_task_draft", "update_task_status", "toggle_agent_skill", "update_project_environment"} {
		if strings.Contains(out.String(), `"name":"`+name+`"`) {
			t.Fatalf("advertised owner mutation %s", name)
		}
	}
	for _, name := range []string{"inspect_database_query", "get_task", "inspect_git_status"} {
		if !strings.Contains(out.String(), `"name":"`+name+`"`) {
			t.Fatalf("removed audit tool %s", name)
		}
	}
}
