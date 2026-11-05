package git

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestRepositoryValidationDoesNotBorrowHostCredentials(t *testing.T) {
	for _, key := range []string{"GH_TOKEN", "GITHUB_TOKEN", "SSH_AUTH_SOCK", "GIT_SSH_COMMAND", "GIT_ASKPASS"} {
		t.Setenv(key, "ambient-fixture")
	}
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "credential.helper")
	t.Setenv("GIT_CONFIG_VALUE_0", "ambient-fixture")
	root := t.TempDir()
	wrapper := filepath.Join(root, "git-fixture")
	script := `#!/usr/bin/python3
import os,sys,base64
assert sys.argv[1:]==['ls-remote','--heads','--symref','https://github.com/fixture/repo.git']
assert not any(os.environ.get(k) for k in ['HOME','GH_TOKEN','GITHUB_TOKEN','SSH_AUTH_SOCK','GIT_SSH_COMMAND','GIT_ASKPASS'])
assert os.environ['GIT_CONFIG_GLOBAL']=='/dev/null'
settings={os.environ['GIT_CONFIG_KEY_'+str(i)]:os.environ['GIT_CONFIG_VALUE_'+str(i)] for i in range(int(os.environ['GIT_CONFIG_COUNT']))}
assert settings['credential.helper']=='' and settings['core.hooksPath']=='/dev/null'
assert base64.b64decode(settings['http.https://github.com/.extraheader'].split('Basic ')[1]).decode()=='x-access-token:project-fixture'
print('aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\trefs/heads/main')
`
	if err := os.WriteFile(wrapper, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) }))
	defer api.Close()
	client := NewClient()
	client.gitBinary = wrapper
	client.SetAPIBaseURL(api.URL)
	info, err := client.ValidateRepository(context.Background(), "fixture/repo", "project-fixture")
	if err != nil || info == nil || len(info.Branches) != 1 || info.Branches[0] != "main" {
		t.Fatal("isolated validation failed", err)
	}
}
