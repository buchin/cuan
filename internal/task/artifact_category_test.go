package task

import (
	"context"
	"testing"

	"github.com/buchin/cuan/internal/executionpolicy"
)

func TestStandaloneArtifactCategoryCannotOverrideExplicitWorkspaceScope(t *testing.T) {
	scope := "Problem Statement:\nMembuat file standalone cleanup-workflow.md di ruang artifact privat tanpa memodifikasi repositori aplikasi.\n\nScope:\nBuat file cleanup-workflow.md di ruang artifact privat.\n\nBatasan Ketat:\n- Jangan membuka atau mengubah repositori aplikasi.\n- Jangan clone, build, tes aplikasi, browser, SSH, database, menghubungi customer, push atau deploy.\n- Pakai ruang artifact privat.\n\nDefinition of Done (DoD):\n1. File cleanup-workflow.md tersedia di ruang artifact privat.\n2. Tidak ada perubahan pada repositori aplikasi, git status bersih, dan tidak ada aksi deploy/build/database/SSH.\n3. Verifikasi isi aktual dilakukan oleh sesi verifier native terpisah setelah worker.\n4. Lampirkan hasil artifact."
	if !executionpolicy.StandaloneArtifactWork(scope) {
		t.Fatal("explicit artifact scope was not recognized")
	}
	for _, hint := range []string{"", ExecutionCategoryCode, ExecutionCategoryReadOnly, ExecutionCategoryOps} {
		t.Run(hint, func(t *testing.T) {
			svc := NewService(NewMemoryStore(), nil)
			item, err := svc.CreateTask(context.Background(), "owner", CreateTaskRequest{Title: "Buat cleanup-workflow.md", Instructions: scope, ExecutionCategory: hint})
			if err != nil || item.ExecutionCategory != ExecutionCategoryOps {
				t.Fatal("category hint overrode standalone workspace", item, err)
			}
			code := ExecutionCategoryCode
			updated, err := svc.UpdateTask(context.Background(), item.ID, "owner", UpdateTaskRequest{ExecutionCategory: &code})
			if err != nil || updated.ExecutionCategory != ExecutionCategoryOps {
				t.Fatal("metadata update overrode standalone workspace", updated, err)
			}
		})
	}
}

func TestArtifactCategoryConstraintPreservesRepositoryAndMigrationWork(t *testing.T) {
	for _, scope := range []string{
		"Implement a file download component in the application repository. Do not deploy.",
		"Create a standalone file describing a schema migration. Do not modify the repository. Do not deploy.",
	} {
		if got := scopeConstrainedCategory("Requested work", scope, ExecutionCategoryCode); got != ExecutionCategoryCode {
			t.Fatal("repository/schema work escaped its execution lane", got)
		}
	}
}
