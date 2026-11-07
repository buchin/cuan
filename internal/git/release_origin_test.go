package git

import (
	"context"
	"path/filepath"
	"testing"
)

func TestReleaseOriginIsPublicMetadataAndNeverRewritesChangedRemote(t *testing.T) {
	source, developer, original, target, run := snapshotFixture(t)
	before := run(developer, "config", "--local", "--get", "remote.origin.url")
	client := NewClient()
	root, err := client.PrepareReleaseWorkspace(context.Background(), source, filepath.Join(t.TempDir(), "release"), original, target, "dev")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := client.BindReleaseOrigin(context.Background(), root, "https://github.com/owner/repo.git"); err != nil {
			t.Fatal(err)
		}
	}
	if got := run(root, "config", "--local", "--get", "remote.origin.url"); got != "https://github.com/owner/repo.git" {
		t.Fatal("public identity missing", got)
	}
	for _, changed := range []string{"https://github.com/other/repo.git", "https://token@github.com/owner/repo.git"} {
		run(root, "remote", "set-url", "origin", changed)
		if err := client.BindReleaseOrigin(context.Background(), root, "https://github.com/owner/repo.git"); err == nil {
			t.Fatal("changed remote accepted")
		}
		if run(root, "config", "--local", "--get", "remote.origin.url") != changed {
			t.Fatal("changed remote overwritten")
		}
	}
	if run(developer, "config", "--local", "--get", "remote.origin.url") != before {
		t.Fatal("developer remote changed")
	}
}
