package server

import (
	"context"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func TestPhaseSnapshotRecoveryRequiresOriginalApproval(t *testing.T) {
	s, _, parent, child, _ := phaseLineageFixture(t)
	s.inboxStopping.Store(true)
	ctx := context.Background()
	if err := s.taskSvc.FinishTaskLaunches(ctx, child.ID, "stopped"); err != nil {
		t.Fatal(err)
	}
	a := &task.TaskAttempt{ID: "phase-recovery-attempt", TaskID: child.ID, Status: task.AttemptStatusSucceeded, CreatedAt: time.Now()}
	if err := s.taskSvc.Store().CreateAttempt(ctx, a); err != nil {
		t.Fatal(err)
	}
	child.Status = task.TaskStatusReview
	if err := s.taskSvc.Store().UpdateTask(ctx, &child.Task); err != nil {
		t.Fatal(err)
	}
	current, _ := s.taskSvc.GetTaskDetail(ctx, child.ID)
	rows, _ := s.projectStudioRepo.(db.InboxRequestRepository).ListInboxRequests(ctx, child.SourceRoomID)
	var origin db.InboxRequest
	for _, q := range rows {
		if q.MessageID == child.SourceMessageID {
			origin = q
		}
	}
	if origin.RequestID == "" || !s.reviewSnapshotRecoveryAuthorized(ctx, current, origin) {
		t.Fatal("approved automatic phase cannot recover review")
	}
	foreign := origin
	foreign.OwnerID = "foreign"
	if s.reviewSnapshotRecoveryAuthorized(ctx, current, foreign) {
		t.Fatal("phase recovery crossed owner boundary")
	}
	cancelled := origin
	cancelled.State = "cancelled"
	if s.reviewSnapshotRecoveryAuthorized(ctx, current, cancelled) {
		t.Fatal("cancelled origin authorized phase recovery")
	}
	fakeID := db.GenerateUUID()
	if err := s.projectStudioRepo.CreateMessage(ctx, &db.ProjectMessage{ID: fakeID, RoomID: origin.RoomID, AuthorType: "system", AuthorID: "cuan_work_review", Content: "Review phase", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	forged := *current
	forged.SourceMessageID = fakeID
	fake := origin
	fake.MessageID = fakeID
	if s.reviewSnapshotRecoveryAuthorized(ctx, &forged, fake) {
		t.Fatal("arbitrary system input became phase approval")
	}
	parent.Status = "cancelled"
	if err := s.taskSvc.Store().UpdateTask(ctx, &parent.Task); err != nil {
		t.Fatal(err)
	}
	if s.reviewSnapshotRecoveryAuthorized(ctx, current, origin) {
		t.Fatal("cancelled predecessor authorized phase recovery")
	}
	parent.Status = task.TaskStatusDone
	if err := s.taskSvc.Store().UpdateTask(ctx, &parent.Task); err != nil {
		t.Fatal(err)
	}
	approval, _ := s.projectStudioRepo.GetMessageByID(ctx, parent.SourceMessageID)
	approval.Content = "Original plan withdrawn"
	if err := s.projectStudioRepo.UpdateMessage(ctx, approval); err != nil {
		t.Fatal(err)
	}
	if s.reviewSnapshotRecoveryAuthorized(ctx, current, origin) {
		t.Fatal("changed original plan authorized phase recovery")
	}
}
