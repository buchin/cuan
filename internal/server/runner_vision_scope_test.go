package server

import (
	"context"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
	"testing"
)

func TestInspectionVisionUsesOriginalScopeInsteadOfGenericTransport(t *testing.T) {
	s := &Server{}
	generic := "Inspect current behavior; inspect actual UI screenshots at original required viewports for UI work. Do not substitute old screenshots."
	for _, tc := range []struct {
		name, scope      string
		criteria         []task.DoDItem
		inspection, want bool
	}{
		{"database inspection", "Migrate PostgreSQL data to SQLite and reconcile row counts", nil, true, false},
		{"backend no UI", "Add quota regression tests. Tidak ada perubahan UI, konfigurasi, push, PR, atau deployment.", nil, true, false},
		{"backend telemetry production constraint", "Fix database status and telemetry timestamp. Batasan ketat: Jangan mengubah komponen UI, jangan menghapus riwayat data/history, jangan mengirim email, dan jangan menggunakan otomasi browser.", nil, true, false},
		{"no touching UI", "Backend saja tanpa menyentuh UI", nil, true, false},
		{"no UI components English", "Fix backend status. Do not modify UI components.", nil, true, false},
		{"negative criterion list", "Add quota regression tests", []task.DoDItem{{Text: "Tidak ada push, PR, deploy, perubahan konfigurasi, perubahan UI, atau pesan keluar."}}, true, false},
		{"english no UI", "Add quota tests without UI changes", nil, true, false},
		{"artifact backend restriction", "Backend/CLI/MCP saja tanpa modifikasi UI, tanpa mengubah/menghapus data pengguna.", nil, true, false},
		{"modification restriction with visual audit", "Tanpa modifikasi UI; periksa tampilan halaman inbox", nil, true, true},
		{"command help display", "Flag sentinel bocor ke output --help, mengotori tampilan.\nTidak ada perubahan UI, dependency baru, atau modul lain.", nil, true, false},
		{"terminal text criterion", "Fix CLI help output", []task.DoDItem{{Text: "Tampilan bantuan CLI bersih tanpa karakter kontrol"}}, true, false},
		{"command then actual UI", "Clean terminal output.\nPerbaiki tampilan halaman settings", nil, true, true},
		{"command and actual UI same line", "Fix CLI output and task card UI layout", nil, true, true},
		{"web page about CLI", "Perbaiki tampilan halaman dokumentasi CLI", nil, true, true},
		{"read only visual audit", "Inspect UI without changing it", nil, true, true},
		{"negative mutation then visual audit", "No UI changes; inspect the actual UI layout", nil, true, true},
		{"negative mutation then screenshots", "Tidak ada perubahan UI. Capture mobile screenshots", nil, true, true},
		{"negative mutation then positive exception", "No UI changes except fix the task card UI", nil, true, true},
		{"visual task without screenshot keyword", "Fix the task card UI layout", nil, true, true},
		{"original visual scope", "Verify actual mobile screenshots", nil, true, true},
		{"original visual criterion", "Improve task detail", []task.DoDItem{{Text: "Inspect screenshots at 390px and desktop"}}, true, true},
		{"ordinary explicit visual revision", "Improve task detail", nil, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := &task.TaskDetailResponse{Task: task.Task{Title: "Work", Instructions: tc.scope, DoDChecklist: tc.criteria}}
			claim := &runner.ClaimTaskResponse{}
			if tc.inspection {
				claim.LiveInspection = &task.LiveInspectionIntent{}
			}
			if got := s.runnerClaimRequiresVision(context.Background(), claim, d, generic); got != tc.want {
				t.Fatalf("vision=%v want %v", got, tc.want)
			}
		})
	}
}
