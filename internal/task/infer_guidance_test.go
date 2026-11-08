package task

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/project"
)

func TestExecutionClassificationUsesApprovedScopeNotProjectDefaults(t *testing.T) {
	p := &project.ProjectResponse{ID: "classifier-project", Metadata: json.RawMessage(`{"cuan_settings":{"goal":"Implement features and deploy production","instructions":"Use backend migrations when necessary","auto_deploy":true,"environment":"DATABASE_MIGRATION_MODE=enabled"}}`)}
	for _, c := range []struct {
		title, scope, want string
		migration          bool
	}{
		{"Audit logs", "Inspect the existing logs and report findings.", ExecutionCategoryReadOnly, false},
		{"Audit Commit & Status Live Deployment", "Inspect repository history and production status. Do not deploy.", ExecutionCategoryReadOnly, false},
		{"Audit live deployment", "Compare current commit with production deployment status.", ExecutionCategoryReadOnly, false},
		{"Deploy the build", "Deploy the latest build, then inspect logs.", ExecutionCategoryCode, false},
		{"Sync contacts", "Sync contacts from the existing provider.", ExecutionCategoryOps, false},
		{"Implement feature", "Add a database migration and verify it.", ExecutionCategoryCode, true},
	} {
		t.Run(c.title, func(t *testing.T) {
			svc := NewService(NewMemoryStore(), &classificationProjectFinder{p: p})
			saved, err := svc.CreateTask(context.Background(), "owner", CreateTaskRequest{ProjectID: p.ID, Title: c.title, Instructions: c.scope})
			if err != nil {
				t.Fatal(err)
			}
			if saved.ExecutionCategory != c.want || IsTaskSchemaMigration(saved) != c.migration {
				t.Fatalf("classification %s migration %v", saved.ExecutionCategory, IsTaskSchemaMigration(saved))
			}
			if !strings.Contains(saved.Instructions, "Implement features and deploy production") || !strings.Contains(saved.Instructions, "DATABASE_MIGRATION_MODE") {
				t.Fatal("classification discarded project guidance")
			}
		})
	}
}

type classificationProjectFinder struct{ p *project.ProjectResponse }

func (f *classificationProjectFinder) Get(context.Context, string) (*project.ProjectResponse, error) {
	return f.p, nil
}

func TestAccountOperationsDoNotInheritRepositoryImplementationLane(t *testing.T) {
	for _, c := range []struct{ title, scope, want string }{
		{"Aktivasi Akses Dracin 3", "Cari dan verifikasi akun dengan email customer@example.invalid. Berikan hak akses produk Dracin 3. Verifikasi status akses aktif di database Dojo.", ExecutionCategoryOps},
		{"Berikan hak akses", "Aktifkan akses user ini. Jangan ubah kode. Jangan deploy.", ExecutionCategoryOps},
		{"Grant access", "Activate access for the customer so they can use existing content and features. Verify the entitlement.", ExecutionCategoryOps},
		{"Grant access", "Grant access and implement a new middleware to enforce it.", ExecutionCategoryCode},
		{"Activate subscription", "Add a database migration and verify it.", ExecutionCategoryCode},
		{"Inspect access", "Read-only: inspect existing entitlement and report findings. Do not activate access.", ExecutionCategoryReadOnly},
		{"Grant access", "Grant permission and deploy a new build.", ExecutionCategoryCode},
	} {
		if got := InferExecutionCategory(c.title, c.scope); got != c.want {
			t.Fatalf("%s: got %s want %s", c.title, got, c.want)
		}
	}
}
