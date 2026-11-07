package server

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/publication"
	"golang.org/x/crypto/ssh"
)

func TestNativeSSHRuntimePinsSavedTrustAndIsolatesClientConfiguration(t *testing.T) {
	for _, scenario := range []string{"valid", "changed trust", "symlink trust", "cancelled", "concurrent"} {
		t.Run(scenario, func(t *testing.T) {
			workspace := t.TempDir()
			base := filepath.Join(workspace, ".git", "cuan-native-runtime")
			if err := os.MkdirAll(base, 0700); err != nil {
				t.Fatal(err)
			}
			root, err := os.OpenRoot(workspace)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			tools := t.TempDir()
			output := filepath.Join(t.TempDir(), "arguments")
			for _, tool := range []string{"ssh", "scp"} {
				if err := os.WriteFile(filepath.Join(tools, tool), []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$OUTPUT\"\n"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", tools+":"+os.Getenv("PATH"))
			_, key, _ := ed25519.GenerateKey(rand.Reader)
			signer, _ := ssh.NewSignerFromKey(key)
			config := publication.Config{Host: "project.invalid:2222", User: "owner", Password: "private-password", HostKey: strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))}
			ctx := context.Background()
			if scenario == "cancelled" {
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cancelled
			}
			bin, err := prepareNativeReleaseSSH(ctx, root, workspace, config)
			if scenario == "cancelled" {
				if err == nil {
					t.Fatal("cancelled runtime prepared")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			trustPath := filepath.Join(base, "ssh", "known_hosts")
			trust, err := os.ReadFile(trustPath)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(trust), "[project.invalid]:2222 "+config.HostKey) {
				t.Fatal("saved host key/port not pinned")
			}
			configuration, err := os.ReadFile(filepath.Join(base, "ssh", "config"))
			if err != nil {
				t.Fatal(err)
			}
			for _, required := range []string{"StrictHostKeyChecking yes", "GlobalKnownHostsFile /dev/null", "IdentityAgent none", "PubkeyAuthentication no"} {
				if !strings.Contains(string(configuration), required) {
					t.Fatal("native SSH isolation missing", required)
				}
			}
			if strings.Contains(string(configuration)+string(trust), config.Password) {
				t.Fatal("runtime files retained credential")
			}
			switch scenario {
			case "changed trust":
				_, changed, _ := ed25519.GenerateKey(rand.Reader)
				changedSigner, _ := ssh.NewSignerFromKey(changed)
				config.HostKey = strings.TrimSpace(string(ssh.MarshalAuthorizedKey(changedSigner.PublicKey())))
			case "symlink trust":
				if err := os.Remove(trustPath); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(t.TempDir(), "foreign"), trustPath); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "concurrent" {
				results := make(chan error, 2)
				for i := 0; i < 2; i++ {
					go func() { _, err := prepareNativeReleaseSSH(ctx, root, workspace, config); results <- err }()
				}
				for i := 0; i < 2; i++ {
					if err := <-results; err != nil {
						t.Fatal("concurrent preparation conflicted", err)
					}
				}
			} else {
				_, err := prepareNativeReleaseSSH(ctx, root, workspace, config)
				if (err == nil) != (scenario == "valid") {
					t.Fatal("changed native trust accepted or identical trust rejected", err)
				}
			}
			if scenario == "changed trust" {
				saved, _ := os.ReadFile(trustPath)
				if string(saved) != string(trust) {
					t.Fatal("host key replaced silently")
				}
				return
			}
			if scenario == "symlink trust" {
				return
			}
			for _, tool := range []string{"ssh", "scp"} {
				command := exec.Command(filepath.Join(bin, tool), "owner@project.invalid", "literal;not-a-command")
				command.Env = []string{"OUTPUT=" + output, "PATH=/usr/bin:/bin"}
				if err := command.Run(); err != nil {
					t.Fatal(err)
				}
				arguments, err := os.ReadFile(output)
				if err != nil || string(arguments) != "-F\n"+filepath.Join(base, "ssh", "config")+"\nowner@project.invalid\nliteral;not-a-command\n" {
					t.Fatal("private SSH config/argv not preserved", err)
				}
			}
		})
	}
}
