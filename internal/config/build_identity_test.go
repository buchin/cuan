package config

import (
	"strings"
	"testing"
)

func TestSourceBuildIdentityCannotBorrowRuntimeRevision(t *testing.T) {
	previousSource, previousCommit := SourceRepository, GitCommit
	t.Cleanup(func() { SourceRepository, GitCommit = previousSource, previousCommit })
	SourceRepository = "example/fork"
	GitCommit = ""
	t.Setenv("CUAN_COMMIT_SHA", strings.Repeat("a", 40))
	if loaded := Load(); loaded.SourceRepositoryURL() != "" {
		t.Fatal("environment revision became compile-time source identity")
	}
	GitCommit = strings.Repeat("b", 40)
	loaded := Load()
	if loaded.CommitSHA != GitCommit || loaded.SourceRepositoryURL() != "https://github.com/example/fork" {
		t.Fatal("paired build metadata was replaced by environment defaults")
	}
}
