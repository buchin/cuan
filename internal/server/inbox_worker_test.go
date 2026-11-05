package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/db"
)

func TestExternalChatWorkerSurvivesAPIShutdownAndRetainsFIFO(t *testing.T) {
	externalChatWorkerContract(t, nil)
}

func TestExternalChatWorkerPostgres(t *testing.T) {
	withPhasePostgres(t, func(database *sql.DB) { externalChatWorkerContract(t, database) })
}

func externalChatWorkerContract(t *testing.T, database *sql.DB) {
	api, _, call, home := inboxFixture(t)
	if database != nil {
		if _, err := database.ExecContext(context.Background(), `INSERT INTO projects(id,name,repo_url) VALUES('default','Fixture','') ON CONFLICT DO NOTHING`); err != nil {
			t.Fatal(err)
		}
		api.projectStudioRepo = db.NewSQLProjectStudioRepository(database)
	}
	api.cfg.InboxExecutionRole = "api"
	apiCtx, stopAPI := context.WithCancel(context.Background())
	defer stopAPI()
	api.StartInboxDispatcher(apiCtx)
	post := func(text string) db.InboxRequest {
		w := call("POST", "/api/v1/chat/conversations/agent_lifecycle/messages", `{"request_id":"`+db.GenerateUUID()+`","message":"`+text+`"}`)
		if w.Code != 202 {
			t.Fatal(w.Code, w.Body.String())
		}
		var accepted struct{ Request db.InboxRequest }
		if err := json.Unmarshal(w.Body.Bytes(), &accepted); err != nil {
			t.Fatal(err)
		}
		return accepted.Request
	}
	first := post("First isolated chat turn")
	second := post("Second isolated chat turn")
	time.Sleep(30 * time.Millisecond)
	waitInboxState(t, api, first.RoomID, first.RequestID, "queued")
	if _, err := os.Stat(filepath.Join(home, "prompts")); !os.IsNotExist(err) {
		t.Fatal("API executed a provider")
	}
	cfg := *api.cfg
	cfg.InboxExecutionRole = "worker"
	worker := NewServer(&cfg, nil, api.authSvc)
	worker.projectStudioRepo = api.projectStudioRepo
	worker.SetAgentRepository(api.agentRepo)
	workerCtx, stopWorker := context.WithCancel(context.Background())
	defer stopWorker()
	worker.StartInboxDispatcher(workerCtx)
	waitInboxState(t, worker, first.RoomID, first.RequestID, "running")
	stopAPI()
	waitInboxState(t, worker, first.RoomID, second.RequestID, "completed")
	rows, _ := api.projectStudioRepo.(db.InboxRequestRepository).ListInboxRequests(context.Background(), first.RoomID)
	if len(rows) != 2 || rows[0].State != "completed" || rows[1].State != "completed" {
		t.Fatal("lost accepted conversation", rows)
	}
	if _, err := os.Stat(filepath.Join(home, "overlap")); !os.IsNotExist(err) {
		t.Fatal("same-room providers overlapped")
	}
}

func TestInboxWorkerPostgresEventsSurviveAPIReconnect(t *testing.T) {
	dsn := os.Getenv("CUAN_LIFECYCLE_SQL_DSN")
	if dsn == "" {
		t.Skip("requires disposable PostgreSQL DSN")
	}
	database, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	room := db.GenerateUUID()
	worker := &Server{cfg: &config.Config{InboxExecutionRole: "worker"}, db: database}
	for generation := 0; generation < 2; generation++ {
		api := &Server{cfg: &config.Config{InboxExecutionRole: "api", DatabaseURL: dsn}, db: database, roomEventBroker: NewRoomEventBroker()}
		subscription := api.roomEventBroker.Subscribe(room)
		ctx, cancel := context.WithCancel(context.Background())
		api.StartInboxEvents(ctx)
		// Initial subscription refresh proves LISTEN was established.
		select {
		case <-subscription:
		case <-time.After(3 * time.Second):
			cancel()
			t.Fatal("API did not establish event subscription")
		}
		worker.publishInbox(room)
		select {
		case event := <-subscription:
			if event.RoomID != room || event.Type != "inbox.updated" || event.Data != nil {
				t.Fatal("unsafe or unrelated event payload", event)
			}
		case <-time.After(3 * time.Second):
			cancel()
			t.Fatal("worker progress did not reach API")
		}
		cancel()
		api.roomEventBroker.Unsubscribe(room, subscription)
	}
}

func TestChatWorkerDrainDoesNotCancelActiveConversation(t *testing.T) {
	s := &Server{cfg: &config.Config{InboxExecutionRole: "worker"}}
	s.inboxWorkers.Store("active-room", true)
	canceled := false
	s.inboxCancels.Store("active-request", context.CancelFunc(func() { canceled = true }))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	finished := make(chan error, 1)
	go func() { finished <- s.DrainInboxWorker(ctx) }()
	for !s.inboxStopping.Load() {
		time.Sleep(time.Millisecond)
	}
	select {
	case <-finished:
		t.Fatal("drain ended while work remained")
	default:
	}
	s.inboxWorkers.Delete("active-room")
	if err := <-finished; err != nil || canceled {
		t.Fatal("active provider canceled by drain", err)
	}
	s.kickInboxRoom("new-room")
	if _, exists := s.inboxWorkers.Load("new-room"); exists {
		t.Fatal("draining worker admitted more work")
	}
}
