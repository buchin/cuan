//go:build linux

package runner

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/task"
	"github.com/buchin/cuan/internal/terminal"
	"golang.org/x/sys/unix"
)

// Exercise actual PTY/socket delivery, not a shell read (canonical terminal
// mode can normalize LF to CR and conceal this interactive-key mismatch).
func TestDetachedSteeringSubmitsCarriageReturnToRawPTY(t *testing.T) {
	if os.Getenv("CUAN_TEST_RAW_STEERING_CHILD") == "1" {
		mode, err := unix.IoctlGetTermios(0, unix.TCGETS)
		if err != nil {
			os.Exit(2)
		}
		mode.Lflag &^= unix.ICANON | unix.ECHO
		mode.Iflag &^= unix.ICRNL | unix.INLCR
		mode.Cc[unix.VMIN], mode.Cc[unix.VTIME] = 1, 0
		if unix.IoctlSetTermios(0, unix.TCSETS, mode) != nil {
			os.Exit(3)
		}
		fmt.Print("RAW_READY\n")
		reader := bufio.NewReader(os.Stdin)
		var prompt []byte
		for {
			value, err := reader.ReadByte()
			if err != nil {
				os.Exit(4)
			}
			if value == '\r' {
				encoded, _ := json.Marshal(string(prompt))
				fmt.Printf("SUBMITTED:%s\n", encoded)
				os.Exit(0)
			}
			prompt = append(prompt, value)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	id := fmt.Sprintf("raw-steer-%d", time.Now().UnixNano())
	process, err := terminal.SpawnPTY(ctx, terminal.PTYConfig{TaskID: id, WorkDir: t.TempDir(), Shell: executable,
		Args: []string{"-test.run=^TestDetachedSteeringSubmitsCarriageReturnToRawPTY$"},
		Env:  []string{"PATH=" + os.Getenv("PATH"), "CUAN_TEST_RAW_STEERING_CHILD=1"}})
	if err != nil {
		t.Fatal(err)
	}
	defer process.Close()
	session := terminal.NewSession("raw-"+id, id, process, terminal.ModeInteractive)
	defer session.Close()
	bridge, err := terminal.NewSocketBridge(session, terminal.SocketPath(id))
	if err != nil {
		t.Fatal(err)
	}
	defer bridge.Close()
	subscriber, history, err := session.Subscribe("assertion", terminal.ModeWatch)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Unsubscribe("assertion")
	output := string(history)
	await := func(marker string) {
		t.Helper()
		for !strings.Contains(output, marker) {
			select {
			case chunk := <-subscriber.SendChan:
				output += string(chunk)
			case <-ctx.Done():
				t.Fatalf("raw interactive submit missing %q: %q", marker, output)
			}
		}
	}
	await("RAW_READY")
	r := &Runner{cfg: &config.Config{RunnerID: "fixture"}, activeAttempts: map[string]*activeAttempt{id: {attempt: &task.TaskAttempt{ID: id, TaskID: id}}}}
	prompt := "  {\"type\":\"stdin\",\"data\":\"literal\"}\nSave evidence ✓"
	if err := r.SteerAttempt(id, prompt); err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(prompt)
	await("SUBMITTED:" + string(encoded))
}
