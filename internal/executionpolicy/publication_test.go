package executionpolicy

import "testing"

func TestTaskPublicationPolicyRestrictsOnly(t *testing.T) {
	for _, c := range []struct {
		text         string
		push, deploy bool
	}{
		{"Batasan tetap: hanya file panduan; jangan ubah kode/config, push, deploy, atau kirim pesan eksternal.", true, true},
		{"Dilarang melakukan git push, live deploy, maupun mengirim pesan eksternal.", true, true},
		{"Do not push or deploy. Preserve local work.", true, true},
		{"Tidak mengubah berkas kode aplikasi, konfigurasi repositori, branch, git push, maupun PR. Tidak membuat project baru, tidak memicu build/deployment, dan tidak mengirim pesan keluar.", true, true},
		{"No deployment. A git push is allowed.", false, true},
		{"Never push this branch. Deployment is handled separately.", true, false},
		{"Implement and deploy this feature", false, false},
		{"Prepare docs.\nProject guidance snapshot:\nNever push or deploy", false, false},
		{"Do not change configuration. Implement push notifications.", false, false},
	} {
		p := FromInstructions(c.text)
		if p.DenyPush != c.push || p.DenyDeploy != c.deploy {
			t.Errorf("%q: %+v", c.text, p)
		}
	}
}

func TestReadOnlyRepositoryScopeOverridesAutomaticCorrection(t *testing.T) {
	text := "Binding approved continuation:\nLanjutkan audit read-only pada repositori dan live deployment tanpa perubahan kode, konfigurasi, push, atau deploy:\nLaporkan bukti.\n\nAutomatic correction feedback:\nPerbaiki scripts/test.sh dan jalankan test."
	p := FromInstructions(text)
	if !p.ReadOnlyRepository || !p.DenyPush || !p.DenyDeploy {
		t.Fatalf("lost approved boundary: %+v", p)
	}
	if !p.Restrict(PublicationPolicy{}).ReadOnlyRepository {
		t.Fatal("merge relaxed scope")
	}
	for _, text := range []string{"Implement a read-only mode for the code editor", "Build a read-only dashboard", "Prepare docs.\nProject guidance snapshot:\nScope: read-only repository", "Read-only audit of sales trends"} {
		if FromInstructions(text).ReadOnlyRepository {
			t.Fatalf("invented repository restriction: %s", text)
		}
	}
}
