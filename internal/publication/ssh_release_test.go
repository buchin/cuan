package publication

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"golang.org/x/crypto/ssh"
)

func fixtureKey(t *testing.T) ssh.Signer {
	t.Helper()
	_, key, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	signer, e := ssh.NewSignerFromKey(key)
	if e != nil {
		t.Fatal(e)
	}
	return signer
}

func TestVerifyObservedSSHRevisionAndHTTPSHealth(t *testing.T) {
	for _, scenario := range []string{"healthy", "rsa-host-key", "wrong-revision", "wrong-host-key", "unhealthy", "redirect", "oversized", "cancelled", "observe", "observe-malformed"} {
		t.Run(scenario, func(t *testing.T) {
			commit := strings.Repeat("a", 40)
			signer := fixtureKey(t)
			if scenario == "rsa-host-key" {
				key, err := rsa.GenerateKey(rand.Reader, 2048)
				if err != nil {
					t.Fatal(err)
				}
				signer, err = ssh.NewSignerFromKey(key)
				if err != nil {
					t.Fatal(err)
				}
			}
			var authenticated, healthCalled atomic.Bool
			config := &ssh.ServerConfig{PasswordCallback: func(meta ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
				authenticated.Store(true)
				if meta.User() != "project-user" || string(password) != "project-password" {
					t.Error("wrong project identity")
				}
				return nil, nil
			}}
			config.AddHostKey(signer)
			listener, e := net.Listen("tcp", "127.0.0.1:0")
			if e != nil {
				t.Fatal(e)
			}
			defer listener.Close()
			go func() {
				socket, e := listener.Accept()
				if e != nil {
					return
				}
				defer socket.Close()
				connection, channels, requests, e := ssh.NewServerConn(socket, config)
				if e != nil {
					return
				}
				defer connection.Close()
				go ssh.DiscardRequests(requests)
				for incoming := range channels {
					if incoming.ChannelType() != "session" {
						incoming.Reject(ssh.UnknownChannelType, "session only")
						continue
					}
					channel, requests, e := incoming.Accept()
					if e != nil {
						return
					}
					defer channel.Close()
					for request := range requests {
						if request.Type != "exec" {
							request.Reply(false, nil)
							continue
						}
						var cmd struct{ Command string }
						ssh.Unmarshal(request.Payload, &cmd)
						if !strings.Contains(cmd.Command, "readlink -f -- '/var/www/project/current'") || !strings.Contains(cmd.Command, `cat -- "$current/REVISION"`) {
							t.Error("unexpected remote command")
						}
						request.Reply(true, nil)
						revision := commit
						if scenario == "wrong-revision" {
							revision = strings.Repeat("b", 40)
						}
						if scenario == "observe-malformed" {
							revision = "unknown"
						}
						if scenario == "oversized" {
							revision = strings.Repeat("x", 5000)
						}
						channel.Write([]byte(revision + "\n"))
						channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
						channel.Close()
						break
					}
				}
			}()
			health := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				healthCalled.Store(true)
				if scenario == "unhealthy" {
					w.WriteHeader(503)
				} else if scenario == "redirect" {
					http.Redirect(w, r, "https://unrelated.invalid/up", 302)
				}
			}))
			defer health.Close()
			c := Config{Host: listener.Addr().String(), User: "project-user", Password: "project-password", HostKey: string(ssh.MarshalAuthorizedKey(signer.PublicKey())), DeployPath: "/var/www/project", HealthURL: health.URL + "/up"}
			if scenario == "wrong-host-key" {
				c.HostKey = string(ssh.MarshalAuthorizedKey(fixtureKey(t).PublicKey()))
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if scenario == "cancelled" {
				cancel()
			}
			var err error
			if strings.HasPrefix(scenario, "observe") {
				var observed string
				observed, err = observe(ctx, c, "", health.Client().Transport)
				if scenario == "observe" && observed != commit {
					t.Fatal("wrong observed revision", observed)
				}
			} else {
				err = verify(ctx, c, commit, health.Client().Transport)
			}
			if (err == nil) != (scenario == "healthy" || scenario == "rsa-host-key" || scenario == "observe") {
				t.Fatalf("unexpected verification error: %v", err)
			}
			if (scenario == "wrong-host-key" || scenario == "cancelled") && authenticated.Load() {
				t.Fatal("password sent before host trust or after cancellation")
			}
			if (scenario == "observe-malformed" || scenario == "wrong-revision" || scenario == "wrong-host-key" || scenario == "oversized" || scenario == "cancelled") && healthCalled.Load() {
				t.Fatal("health substituted for immutable revision")
			}
		})
	}
}

func TestConfigUsesOnlyExplicitRegisteredEnvironment(t *testing.T) {
	key := fixtureKey(t)
	base := map[string]string{"CUAN_PUBLICATION_VERIFIER": "ssh-release", "CUAN_PUBLICATION_SSH_HOST_ENV": "PROJECT_HOST", "PROJECT_HOST": "127.0.0.1", "CUAN_PUBLICATION_SSH_USER": "project-user", "CUAN_PUBLICATION_SSH_PASSWORD_ENV": "PROJECT_PASSWORD", "PROJECT_PASSWORD": "project-password", "CUAN_PUBLICATION_SSH_HOST_KEY": string(ssh.MarshalAuthorizedKey(key.PublicKey())), "CUAN_PUBLICATION_DEPLOY_PATH": "/var/www/project", "CUAN_PUBLICATION_HEALTH_URL": "https://project.invalid/up"}
	if _, err := FromEnvironment(base); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"PROJECT_PASSWORD", "CUAN_PUBLICATION_SSH_HOST_KEY", "CUAN_PUBLICATION_VERIFIER"} {
		copy := map[string]string{}
		for k, v := range base {
			copy[k] = v
		}
		delete(copy, field)
		t.Setenv(field, base[field])
		if _, err := FromEnvironment(copy); err == nil {
			t.Fatal("ambient configuration accepted")
		}
	}
	for _, path := range []string{"/", "/var/www/../project", "/var/project'; touch /tmp/unsafe; '", "/var/project\ncommand"} {
		copy := map[string]string{}
		for k, v := range base {
			copy[k] = v
		}
		copy["CUAN_PUBLICATION_DEPLOY_PATH"] = path
		if _, err := FromEnvironment(copy); err == nil {
			t.Fatal("unsafe target accepted")
		}
	}
}

// Opt-in only: reads the owner's explicitly exported project configuration.
// No ambient SSH credentials, network host discovery or deployment is used.
func TestNativeRegisteredSSHRelease(t *testing.T) {
	file := os.Getenv("CUAN_PUBLICATION_TEST_ENV_FILE")
	if file == "" {
		t.Skip("native project connection not supplied")
	}
	info, err := os.Stat(file)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		t.Fatal("native fixture must be private")
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal("native project configuration unavailable")
	}
	var env map[string]string
	if json.Unmarshal(raw, &env) != nil {
		t.Fatal("invalid native fixture")
	}
	c, err := FromEnvironment(env)
	if err != nil {
		t.Fatal("native project verifier configuration invalid")
	}
	observed, err := Observe(context.Background(), c)
	if err != nil || observed != os.Getenv("CUAN_PUBLICATION_TEST_COMMIT") {
		t.Fatal("registered live observation mismatch", err)
	}
	if err := Verify(context.Background(), c, os.Getenv("CUAN_PUBLICATION_TEST_COMMIT")); err != nil {
		t.Fatal(err)
	}
}

func TestConfigurationStateSeparatesSetupFromObservationWithoutAmbientCredentials(t *testing.T) {
	key := fixtureKey(t)
	valid := map[string]string{"CUAN_PUBLICATION_VERIFIER": "ssh-release", "CUAN_PUBLICATION_SSH_HOST": "project.invalid", "CUAN_PUBLICATION_SSH_USER": "fixture", "CUAN_PUBLICATION_SSH_PASSWORD": "do-not-return-this", "CUAN_PUBLICATION_SSH_HOST_KEY": string(ssh.MarshalAuthorizedKey(key.PublicKey())), "CUAN_PUBLICATION_DEPLOY_PATH": "/srv/project", "CUAN_PUBLICATION_HEALTH_URL": "https://project.invalid/up"}
	for _, item := range []struct {
		env  map[string]string
		want string
	}{
		{nil, "not_configured"}, {map[string]string{"CUAN_PUBLICATION_VERIFIER": "invented-provider"}, "unsupported"}, {map[string]string{"CUAN_PUBLICATION_VERIFIER": "ssh-release"}, "incomplete"}, {valid, "configured"},
	} {
		if got := ConfigurationState(item.env); got != item.want {
			t.Fatal(got, item.want)
		}
	}
	for k, v := range valid {
		t.Setenv(k, v)
	}
	if ConfigurationState(nil) != "not_configured" {
		t.Fatal("ambient access substituted for registered connection")
	}
}
