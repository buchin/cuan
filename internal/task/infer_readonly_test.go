package task

import "testing"

func TestInferExplicitReadOnlyRepositoryScope(t *testing.T) {
	for _, tt := range []struct{ title, instructions, want string }{
		{"Buat task pemeriksaan", "Pemeriksaan read-only pada repositori Cuan: periksa go.mod. Jangan ubah file, commit, push, deploy, atau membuat project baru.", ExecutionCategoryReadOnly},
		{"Audit", "Repository must remain read-only. Inspect the frontend component implementation; do not deploy.", ExecutionCategoryReadOnly},
		{"Implement a read-only dashboard", "Create a frontend component to view repository metrics.", ExecutionCategoryCode},
		{"Schema migration", "Pemeriksaan read-only pada repositori; schema migration.", ExecutionCategoryCode},
	} {
		t.Run(tt.title, func(t *testing.T) {
			if got := InferExecutionCategory(tt.title, tt.instructions); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInferStandaloneDownloadDoesNotRequireCodingRepository(t *testing.T) {
	scope := "Cuan managed delegation v1.\nTujuan: Membuat dan memverifikasi berkas hasil.txt sebagai lampiran unduhan mandiri.\nPekerjaan bersifat mandiri: tidak membuat project baru, tidak mengubah repositori atau file konfigurasi, dan tidak melakukan deployment.\nUser authorization: Ini pekerjaan mandiri; tidak perlu mengubah repositori atau konfigurasi, dan jangan deploy."
	if got := InferExecutionCategory("Buat berkas hasil.txt", scope); got != ExecutionCategoryOps {
		t.Fatal("standalone download routed to repository coding", got)
	}
	if got := InferExecutionCategory("Schema migration", scope); got != ExecutionCategoryCode {
		t.Fatal("schema migration escaped exclusive lane", got)
	}
}
