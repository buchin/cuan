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

func TestInspectBehaviorLimitRequiresSeparatePublicationRequest(t *testing.T) {
	for _, c := range []struct {
		text string
		deny bool
	}{
		{"Jangan jalankan tool atau script deploy saat inspect. Deploy hasil perubahan ke production.", false},
		{"jangan menjalankan tool atau script deploy saat inspect. Deploy hasil perubahan ke production.", false},
		{"Do not execute tools or script deploy during the inspect command. Deploy the implementation to production.", false},
		{"Jangan jalankan tool atau script deploy saat inspect.", true},
		{"Jangan jalankan tool atau script deploy saat inspect. Jangan deploy production. Deploy hasil perubahan ke production.", true},
		{"Jangan jalankan tool atau script deploy saat inspect dan setelah selesai. Deploy hasil perubahan ke production.", true},
		{"Jangan deploy. Tambahkan inspect command lalu deploy production.", true},
		{"Do not run deploy during inspect or push. Deploy production.", true},
	} {
		if got := FromInstructions(c.text); got.DenyDeploy != c.deny {
			t.Errorf("%q: %+v", c.text, got)
		}
	}
}

func TestApprovedInspectImplementationKeepsPublicationRequest(t *testing.T) {
	instructions := "Cuan managed delegation v1.\nApproved scope digest: 7e19fb5dd2423aab62f236152aee1271d325d1c7ea17327f70bbe8c7e478209e\n## Konteks\nKerjakan dari source bersih branch `dev` terbaru di repository project Cuan.\n\n## Scope\nTambahkan opsi `--runtime-path` pada subcommand `cuan release inspect`. Tidak ada perubahan UI, tidak ada dependency baru.\n\n## Spesifikasi Perilaku\n- **Dengan `--runtime-path <path>`:** jalankan `release.CheckRuntime` menggunakan PATH eksplisit yang diberikan untuk memverifikasi `required_tools` dari manifest. Jangan jalankan tool atau script deploy saat inspect.\n- **PATH kosong (`--runtime-path \"\"`):** harus gagal (exit non-zero) jika ada tool yang diwajibkan \u2014 jangan fallback ke PATH host.\n- **Tanpa `--runtime-path`:** perilaku `inspect` lama tidak berubah sama sekali (backward-compatible).\n\n## Implementasi\n1. Checkout branch `dev` terbaru, buat branch task bersih.\n2. Tambahkan flag `--runtime-path` di CLI `cuan release inspect`.\n3. Hubungkan ke `release.CheckRuntime` (atau buat fungsinya jika belum ada) yang menerima PATH eksplisit sebagai parameter, bukan membaca PATH host.\n4. Pastikan logika: jika `--runtime-path` diberikan \u2192 gunakan PATH itu saja; jika kosong dan ada required_tools \u2192 error; jika flag tidak ada \u2192 skip runtime check seperti sebelumnya.\n\n## Tes CLI Terarah\nTulis tes terarah (bukan full suite) yang membuktikan:\n- [ ] Tool tersedia di PATH eksplisit \u2192 inspect berhasil (exit 0)\n- [ ] Tool hilang di PATH eksplisit \u2192 inspect gagal dengan pesan informatif (exit non-zero)\n- [ ] PATH kosong (`--runtime-path \"\"`) dengan required_tools \u2192 gagal, tidak fallback ke host PATH\n- [ ] `cuan release inspect` tanpa `--runtime-path` \u2192 berjalan normal tanpa runtime check (perilaku lama)\n\n## Dokumentasi\n- Update `--help` output untuk flag `--runtime-path` dengan deskripsi yang jelas.\n- Tambahkan dokumentasi opsi di panduan release (mis. `docs/` atau file relevan yang sudah ada).\n\n## Checkpoint\nSetelah implementasi dan tes lokal lulus (exit 0), buat commit Git bersih dengan pesan deskriptif.\n\n## Deploy & Verifikasi\n- Gunakan alur native project yang sudah disetujui untuk push/merge ke `dev`, lalu production deployment.\n- Setelah deploy, smoke check layanan sehat.\n- Sesi verifier independen membaca commit yang sama, menjalankan tes CLI terarah di atas, dan mengkonfirmasi semua kriteria terpenuhi sebelum task diterima Done.\n\n## Definition of Done\n- [ ] Flag `--runtime-path` tersedia di `cuan release inspect --help`\n- [ ] Semantik opsi (tersedia/hilang/PATH kosong/tanpa opsi) terbukti oleh 4 tes terarah lulus (exit 0)\n- [ ] Inspect tanpa `--runtime-path` tetap identik perilakunya\n- [ ] Dokumentasi opsi ada di help dan panduan release\n- [ ] Commit Git bersih dibuat\n- [ ] Production menjalankan revisi hasil perubahan dengan layanan sehat\n- [ ] Verifier independen memeriksa source yang benar, hasil CLI, dan semua kriteria di atas sebelum Done\n\nApproval message: 8fb54903-2e53-4332-b5ec-44791372eeb7\nUser authorization: Kerjakan sekarang di repository project Cuan dari source bersih branch dev terbaru. Tambahkan opsi --runtime-path pada cuan release inspect: jika opsi diberikan, periksa required_tools manifest melalui PATH eksplisit tersebut menggunakan release.CheckRuntime; jangan menjalankan tool atau script deploy saat inspect. PATH kosong yang sengaja diberikan harus gagal jika tool diwajibkan, jangan fallback ke PATH host. Tanpa opsi ini perilaku inspect lama tetap sama. Tambahkan tes CLI terarah untuk tool tersedia, hilang, PATH kosong, dan inspect tanpa pemeriksaan runtime; dokumentasikan opsi di help dan panduan release. Scope satu task cohesive, tanpa perubahan UI atau dependency baru. Selesaikan implementasi dan local checks dengan checkpoint bersih, lalu gunakan alur native project yang sudah disetujui untuk push/merge/deploy production dan verifier independen. DoD: semantik opsi terbukti oleh tes; production menjalankan revisi hasil perubahan dengan layanan sehat; verifier independen memeriksa source yang benar, hasil CLI dan semua kriteria asli sebelum Done. Jangan minta konfirmasi teknis rutin atau klaim Done hanya karena report dev; laporkan kendala nyata bila ada. Ini otorisasi eksekusi dan publication/deployment scope tersebut.\nConversation: 8c9e48ea-58a0-4f10-88a4-570f21222968\nReport actual progress and verification evidence. Stay within this scope."
	if got := FromInstructions(instructions); got.DenyDeploy || got.DenyPush {
		t.Fatalf("product command behavior became task publication prohibition: %+v", got)
	}
}

func TestPublicationEvidenceCautionIsNotBlanketDeploymentBan(t *testing.T) {
	for _, c := range []struct {
		text string
		deny bool
	}{
		{"Do not repeat deployment solely to obtain evidence. Reconcile saved effects, then deploy the verified correction to production.", false},
		{"Jangan lakukan deploy buta atau menandai task selesai sebelum ada bukti valid. Lakukan verifikasi independen, lalu deploy native dan health check.", false},
		{"Jangan lakukan deploy buta. Deploy native setelah verifikasi.", false},
		{"Jangan mengulang broad suite atau deploy hanya demi bukti bila bukti native yang sah sudah ada. Native deploy dan exact health check pada rilis baru.", false},
		{"Jangan mengulang deploy hanya demi bukti. Deploy native setelah verifikasi.", false},
		{"Jangan mengulang broad suite atau deploy hanya demi bukti bila bukti native yang sah sudah ada.", true},
		{"Jangan deploy. Jangan mengulang deploy hanya demi bukti. Deploy native setelah verifikasi.", true},
		{"Jangan mengulang broad suite atau deploy hanya demi bukti atau mengubah produksi. Deploy native setelah verifikasi.", true},
		{"Do not repeat deployment solely to obtain evidence.", true},
		{"Jangan lakukan deploy buta.", true},
		{"Do not deploy. Do not repeat deployment solely to obtain evidence. Deploy production.", true},
		{"Jangan deploy. Jangan lakukan deploy buta. Deploy production.", true},
		{"Jangan lakukan deploy buta atau deploy production. Deploy production.", true},
		{"Do not repeat deployment solely to obtain evidence or change production. Deploy production.", true},
		{"Do not deploy unless authorized. Deploy production.", true},
	} {
		if got := FromInstructions(c.text); got.DenyDeploy != c.deny {
			t.Errorf("%q: %+v", c.text, got)
		}
	}
}
