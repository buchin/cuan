package task

import (
	"testing"
)

func TestInferIndonesianBugfixTitleAndDescriptionIsNotReadOnly(t *testing.T) {
	title := "Perbaikan Scroll Textarea New Work & Relokasi Task Bar ke Bawah Navbar"
	instructions := "Cuan managed delegation v1.\nProblem Statement:\n1. Mengetik pesan panjang di textarea halaman New work memicu scroll melompat.\n2. Task card saat ini menempel di atas composer.\n\nScope:\n1. Perbaikan Bug Scroll Textarea (New work).\n2. Relokasi & Redesign Task Banner.\n\nDefinition of Done (DoD):\n- Mengetik teks panjang di textarea New work berjalan mulus.\n- Inspect the layout."
	if got := InferExecutionCategory(title, instructions); got != ExecutionCategoryCode {
		t.Fatalf("expected ExecutionCategoryCode, got %q", got)
	}
}

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

func TestInferStandaloneFileWithDilarangRepositoryScope(t *testing.T) {
	scope := "Cuan managed delegation v1.\n### Problem Statement & Scope\nBuat file standalone `cleanup-workflow.md` secara mandiri di ruang artifact privat sesuai instruksi eksplisit pengguna.\n\n### Batasan & Larangan Ketat\n- Dilarang membuka atau mengubah repositori aplikasi.\n- Dilarang melakukan clone, build, tes aplikasi, browser automation, SSH, akses database, menghubungi customer, git push, atau deploy.\n- Wajib bekerja secara eksklusif di ruang artifact privat."
	if got := InferExecutionCategory("Buat dan verifikasi file cleanup-workflow.md", scope); got != ExecutionCategoryOps {
		t.Fatal("explicit standalone scope routed to application repository", got)
	}
}

// An inspection title must win over implementation words that merely appear in
// the body. This is the production case that sent a research task to the code
// lane and then to a publication gate it could never satisfy.
func TestInferInspectionTitleBeatsIncidentalImplementationWord(t *testing.T) {
	body := "Evaluasi kelayakan paket VPS menggunakan tool native web_fetch yang baru saja diimplementasikan pada Cuan."
	if got := InferExecutionCategory("Sesi Analisis Mandiri: Audit Paket cloudra.id via web_fetch", body); got != ExecutionCategoryReadOnly {
		t.Fatalf("audit title routed to %q, want readonly", got)
	}
	for _, title := range []string{
		"Investigasi dan perbaiki kendala login",
		"Audit keamanan repositori",
		"Analisis penyebab kegagalan build",
	} {
		if got := InferExecutionCategory(title, body); got != ExecutionCategoryReadOnly {
			t.Fatalf("inspection title %q routed to %q", title, got)
		}
	}
	// Real implementation work must stay in the code lane.
	for _, title := range []string{
		"Implementasi fitur resolve conversation",
		"Tambahkan halaman pengaturan baru",
		"Perbaiki masalah pada repositori",
	} {
		if got := InferExecutionCategory(title, "Ubah komponen frontend dan deploy."); got != ExecutionCategoryCode {
			t.Fatalf("implementation title %q routed to %q", title, got)
		}
	}
}

// Whole-word matching keeps unrelated words from hijacking the lane.
func TestInferTitleTermsRespectWordBoundaries(t *testing.T) {
	for _, title := range []string{
		"Implementasi auditorium virtual", // contains "audit" as a prefix of auditorium
		"Tambah fitur monitorkan transaksi",
	} {
		if got := InferExecutionCategory(title, "Buat komponen frontend."); got != ExecutionCategoryCode {
			t.Fatalf("title %q routed to %q, want code", title, got)
		}
	}
}
