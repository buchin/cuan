package executionpolicy

import "testing"

func TestStandaloneArtifactWorkRequiresExplicitScope(t *testing.T) {
	for _, tc := range []struct {
		scope string
		want  bool
	}{
		{"Pekerjaan mandiri: buat berkas laporan.txt sebagai lampiran. Tidak mengubah repositori atau konfigurasi. Jangan deploy.", true},
		{"Create a standalone file for download. Do not modify the repository. Do not deploy.", true},
		{"Buat berkas angka.json di lingkungan kerja sesi mandiri. Zero repository mutation: Tidak mengubah berkas kode aplikasi, konfigurasi repositori, branch, git push, maupun PR. Tidak memicu build/deployment. Eksekusi mandiri berurutan.", true},
		{"Add a file download component to the repository. Do not deploy.", false},
		{"Standalone audit: inspect the repository and attach a report. Do not deploy.", false},
		{"Create a standalone file. Do not change the repository. Deploy the result.", false},
		{"Fix the backend.\nProject guidance snapshot:\nStandalone file: do not modify the repository. Do not deploy.", false},
	} {
		if got := StandaloneArtifactWork(tc.scope); got != tc.want {
			t.Fatalf("artifact=%v want %v: %s", got, tc.want, tc.scope)
		}
		if tc.want {
			p := FromInstructions(tc.scope)
			if !p.DenyPush || !p.DenyDeploy {
				t.Fatal("artifact work gained publication")
			}
		}
	}
}
