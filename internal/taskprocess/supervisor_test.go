package taskprocess

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func init() {
	if len(os.Args) > 1 && os.Args[1] == Command {
		os.Exit(Run(os.Args[2:], os.Stdin, os.Stdout, os.Stderr))
	}
	if len(os.Args) > 1 && os.Args[1] == "--test-task-parent" {
		executable, _ := os.Executable()
		child := exec.Command(executable, Command, os.Args[2], strconv.Itoa(os.Getpid()), "--", "/bin/sh", "-c", `trap '' TERM HUP; echo $$ > "$1"; echo ORIGINAL_PROGRESS > "$2"; while :; do sleep 0.05; done`, "fixture", os.Args[3], os.Args[4])
		child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		child.Stderr = os.Stderr
		if err := child.Start(); err != nil {
			os.Exit(3)
		}
		os.WriteFile(os.Args[5], []byte(strconv.Itoa(child.Process.Pid)), 0600)
		for {
			time.Sleep(time.Hour)
		}
	}
}

func TestSupervisorFencesLiveAgentAndPreservesProgressAfterParentLoss(t *testing.T) {
	root := t.TempDir()
	lock := filepath.Join(root, "task.lock")
	agentPIDFile := filepath.Join(root, "agent.pid")
	checkpoint := filepath.Join(root, "checkpoint")
	supervisorPIDFile := filepath.Join(root, "supervisor.pid")
	executable, _ := os.Executable()
	parent := exec.Command(executable, "--test-task-parent", lock, agentPIDFile, checkpoint, supervisorPIDFile)
	parent.Stderr = os.Stderr
	if err := parent.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { parent.Process.Kill(); parent.Wait() }()
	var supervisorPID int
	defer func() {
		if supervisorPID > 1 {
			syscall.Kill(-supervisorPID, syscall.SIGKILL)
		}
	}()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if body, err := os.ReadFile(checkpoint); err == nil && strings.TrimSpace(string(body)) == "ORIGINAL_PROGRESS" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("actual task agent did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	body, _ := os.ReadFile(supervisorPIDFile)
	supervisorPID, _ = strconv.Atoi(string(body))
	if supervisorPID <= 1 {
		t.Fatal("missing supervisor identity")
	}
	if active, err := Active(lock); err != nil || !active {
		t.Fatal("live task lost ownership", err)
	}
	duplicate := exec.Command(executable, Command, lock, strconv.Itoa(os.Getpid()), "--", "/bin/sh", "-c", "echo duplicate")
	if err := duplicate.Run(); err == nil || duplicate.ProcessState.ExitCode() != ConflictExit {
		t.Fatal("replacement ran while old agent was active")
	}
	if err := parent.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	parent.Wait()
	deadline = time.Now().Add(4 * time.Second)
	for {
		active, err := Active(lock)
		if err != nil {
			t.Fatal(err)
		}
		if !active {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("ownership not released after parent/process death")
		}
		time.Sleep(10 * time.Millisecond)
	}
	body, _ = os.ReadFile(agentPIDFile)
	agentPID, _ := strconv.Atoi(strings.TrimSpace(string(body)))
	if stat, err := os.ReadFile("/proc/" + strconv.Itoa(agentPID) + "/stat"); err == nil {
		tail := string(stat)[strings.LastIndex(string(stat), ")")+1:]
		if !strings.HasPrefix(strings.TrimSpace(tail), "Z") {
			t.Fatal("old agent still running after ownership release")
		}
	}
	continued := exec.Command(executable, Command, lock, strconv.Itoa(os.Getpid()), "--", "/bin/sh", "-c", `test "$(cat "$1")" = ORIGINAL_PROGRESS; echo CONTINUED`, "fixture", checkpoint)
	output, err := continued.CombinedOutput()
	if err != nil || strings.TrimSpace(string(output)) != "CONTINUED" {
		t.Fatal("continuation lost saved progress", err, string(output))
	}
}

func TestSupervisorRejectsSymlinkAndReturnsAgentExit(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	os.WriteFile(target, []byte("preserve"), 0600)
	link := filepath.Join(root, "link")
	os.Symlink(target, link)
	if _, err := Acquire(link); err == nil {
		t.Fatal("symlink lock accepted")
	}
	body, _ := os.ReadFile(target)
	if string(body) != "preserve" {
		t.Fatal("lock modified another file")
	}
	executable, _ := os.Executable()
	command := exec.Command(executable, Command, filepath.Join(root, "task.lock"), strconv.Itoa(os.Getpid()), "--", "/bin/sh", "-c", "exit 7")
	if err := command.Run(); err == nil || command.ProcessState.ExitCode() != 7 {
		t.Fatal("agent exit code not preserved")
	}
}
