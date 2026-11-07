package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/buchin/cuan/internal/task"
)

func TestReviewTextIncludesFinalChecksWithinExistingBudget(t *testing.T) {
	for _, padding := range []string{strings.Repeat("introduction ", 1200), strings.Repeat("文", 5000)} {
		path := filepath.Join(t.TempDir(), "report.md")
		data := "SOURCE FINDINGS\n" + padding + "\nFINAL CHECK: exit 7; result remains unverified."
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		excerpt, scope, err := reviewTextExcerpt(path, 6000)
		if err != nil || scope != "head_and_tail" || len(excerpt) > 6000 || !utf8.ValidString(excerpt) || !strings.HasPrefix(excerpt, "SOURCE FINDINGS") || !strings.HasSuffix(excerpt, "result remains unverified.") || !strings.Contains(excerpt, "middle omitted") {
			t.Fatal("review lost final failure, hid truncation or exceeded budget", scope, err)
		}
	}
	path := filepath.Join(t.TempDir(), "short.md")
	if err := os.WriteFile(path, []byte("Complete report 文"), 0600); err != nil {
		t.Fatal(err)
	}
	excerpt, scope, err := reviewTextExcerpt(path, 6000)
	if err != nil || scope != "complete" || excerpt != "Complete report 文" {
		t.Fatal("short report was changed", err)
	}
	if err := os.WriteFile(path, []byte{0xff, 0xfe}, 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := reviewTextExcerpt(path, 6000); err == nil {
		t.Fatal("binary text became review evidence")
	}
}

func TestReviewStructuredTextMediaTypes(t *testing.T) {
	for _, kind := range []string{"text/plain; charset=utf-8", "application/json", "application/ld+json", "application/xml", "application/vnd.example+xml"} {
		if !reviewableTextMediaType(kind) {
			t.Fatalf("text excluded: %s", kind)
		}
	}
	for _, kind := range []string{"application/pdf", "application/zip", "application/octet-stream", "image/png", "invalid", "application/json; broken"} {
		if reviewableTextMediaType(kind) {
			t.Fatalf("nontext included: %s", kind)
		}
	}
}

func TestAcceptedReviewReportsOutcomeWithoutFutureDeliveryPromise(t *testing.T) {
	s, d, q := stalledReviewFixture(t, "completed")
	row, err := s.taskSvc.Store().GetTask(context.Background(), d.ID)
	if err != nil {
		t.Fatal(err)
	}
	row.ExecutionCategory = task.ExecutionCategoryReadOnly
	if err := s.taskSvc.Store().UpdateTask(context.Background(), row); err != nil {
		t.Fatal(err)
	}
	out := s.applyInboxReviewEvaluation(context.Background(), q, `{"decision":"pass","finding":"Laporan investigasi tersedia dan memenuhi kriteria yang diminta.","explanation":"Cuan akan menyampaikan laporan kepada pengguna.","remaining_work":"none","revision":""}`)
	if !strings.Contains(out, "Laporan investigasi tersedia") || strings.Contains(out, "akan menyampaikan") {
		t.Fatal("accepted result promised an unqueued future delivery", out)
	}
	after, err := s.taskSvc.GetTaskDetail(context.Background(), d.ID)
	if err != nil || after.Status != task.TaskStatusDone {
		t.Fatal("outcome copy changed acceptance", err)
	}
}
