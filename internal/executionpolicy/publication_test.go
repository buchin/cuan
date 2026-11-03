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
