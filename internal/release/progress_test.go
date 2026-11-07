package release

import (
	"context"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestProgressWriterOnlyForwardsBoundedMilestones(t *testing.T) {
	var got []string
	w := &progressWriter{observe: func(s string) { got = append(got, s) }}
	for _, chunk := range []string{"private-token\n", "::cuan-release-st", "age::build\n", "::cuan-release-stage::build\n", "::cuan-release-stage::health private-token\n", strings.Repeat("x", 300) + "::cuan-release-stage::health\n", "health\n", "::cuan-release-stage::health\n"} {
		if n, err := w.Write([]byte(chunk)); err != nil || n != len(chunk) {
			t.Fatal(n, err)
		}
	}
	if !reflect.DeepEqual(got, []string{"Building release artifacts", "Checking production health"}) {
		t.Fatalf("unexpected progress: %q", got)
	}
	if len(w.line) > 128 {
		t.Fatal("unbounded line retention")
	}
}

func TestReleaseProgressArrivesBeforeProcessCompletesAndDoesNotAcceptFailure(t *testing.T) {
	root, commit := fixture(t, "printf '::cuan-release-stage::build\\n'\nwhile [ ! -f \"$ACK\" ]; do sleep 0.01; done\nexit 7\n", goodManifest)
	plan, err := Load(context.Background(), root, commit, "dev")
	if err != nil {
		t.Fatal(err)
	}
	ack := t.TempDir() + "/ack"
	var got []string
	result := RunWithProgress(context.Background(), plan, []string{"PATH=" + os.Getenv("PATH"), "ACK=" + ack}, func(s string) {
		got = append(got, s)
		if err := os.WriteFile(ack, []byte("observed"), 0600); err != nil {
			t.Error(err)
		}
	})
	if !reflect.DeepEqual(got, []string{"Building release artifacts"}) || result.ExitCode != 7 || result.Error == "" || !result.SourceUnchanged {
		t.Fatalf("progress lost or failure accepted: %q %#v", got, result)
	}
}
