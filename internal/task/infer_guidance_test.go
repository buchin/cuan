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

func TestOperationalLoginOutcomePreservesHistoricalCodeAuthority(t *testing.T) {
	item := &Task{Title: "Investigasi dan perbaiki kendala login akun pelanggan", Instructions: "Periksa status akun dan riwayat autentikasi. Verifikasi hasil akun melalui layanan yang sudah ada.", ExecutionCategory: ExecutionCategoryOps}
	if !IsAccountOutcome(item) || !ScopePublicationPolicy(item).ReadOnlyRepository {
		t.Fatal("operational diagnosis entered source publication")
	}
	item.ExecutionCategory = ExecutionCategoryReadOnly
	if !IsAccountOutcome(item) {
		t.Fatal("read-only account diagnosis lost independent outcome workflow")
	}
	item.ExecutionCategory = ExecutionCategoryCode
	if IsAccountOutcome(item) || ScopePublicationPolicy(item).ReadOnlyRepository {
		t.Fatal("existing code task was retroactively restricted or exempted")
	}
	item.ExecutionCategory = ExecutionCategoryOps
	item.Instructions = "Fix the authentication controller for this customer account login."
	if IsAccountOutcome(item) {
		t.Fatal("explicit source repair was treated as account-only evidence")
	}
}

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

func TestFrozenAccountOperationScopeDoesNotRequireSourcePublication(t *testing.T) {
	scope := "Cuan managed delegation v1.\nApproved scope digest: d5b3579348ee7e6b57ce8a08911eb3d6bc32680a30da1b5fa8e448a4469dedeb\n### Problem Statement & Outcome\nPengguna dengan email customer@example.invalid membutuhkan aktivasi hak akses produk/membership Dracin 3 pada platform Dojo agar dapat mengakses konten dan fitur digital yang bersangkutan.\n\n### Scope of Work\n1. Cari dan verifikasi akun dengan email customer@example.invalid pada sistem membership Dojo.\n2. Berikan hak akses/entitlement produk Dracin 3 ke akun pengguna tersebut.\n3. Verifikasi status akses bahwa entitlement aktif dan tercatat di database Dojo.\n\n### Definition of Done (DoD)\n- [ ] Akun pengguna customer@example.invalid terverifikasi di Dojo.\n- [ ] Akses produk Dracin 3 berhasil diberikan ke akun tersebut.\n- [ ] Status hak akses terverifikasi aktif tanpa error pada sistem Dojo.\n\nApproval message: aafa00af-d8c1-4d88-8c90-dd6296b5d5cd\nUser authorization: @cuan Gas\nConversation: f299b6de-3470-4fd4-8b9f-30eba98402f8\nReport actual progress and verification evidence. Stay within this scope."
	title := "Aktivasi Akses Dracin 3 untuk customer@example.invalid di Dojo"
	if !IsAccountOperation(title, scope) {
		t.Fatal("frozen business scope lost its operational classification")
	}
	policy := ScopePublicationPolicy(&Task{Title: title, Instructions: scope, ExecutionCategory: ExecutionCategoryCode})
	if !policy.ReadOnlyRepository || !policy.DenyPush || !policy.DenyDeploy {
		t.Fatal("legacy lane granted source publication", policy)
	}
}
