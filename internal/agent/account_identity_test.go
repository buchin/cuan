package agent

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type identityTransport func(*http.Request) (*http.Response, error)

func (f identityTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestAccountIdentityUsesOnlyPrivateProfile(t *testing.T) {
	home := t.TempDir()
	path := AntigravityCredentialsFilePath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"token":{"access_token":"fixture-private","id_token":"e30.eyJlbWFpbCI6Im93bmVyQGV4YW1wbGUuY29tIn0.sig"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if got := extractAccountEmail(home); got != "owner@example.com" {
		t.Fatalf("nested identity missing: %q", got)
	}
	client := &http.Client{Transport: identityTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://openidconnect.googleapis.com/v1/userinfo" || r.Header.Get("Authorization") != "Bearer fixture-private" {
			t.Fatal("identity request escaped account or provider")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"email":"owner@example.com","email_verified":true}`)), Header: make(http.Header)}, nil
	})}
	if got := lookupGoogleAccountEmail(context.Background(), home, client); got != "owner@example.com" {
		t.Fatalf("missing verified email: %q", got)
	}
	if got := lookupGoogleAccountEmail(context.Background(), t.TempDir(), client); got != "" {
		t.Fatal("missing profile inherited identity")
	}
}
func TestAccountIdentityDoesNotFollowRedirectOrTrustUnverifiedEmail(t *testing.T) {
	home := t.TempDir()
	path := AntigravityCredentialsFilePath(home)
	os.MkdirAll(filepath.Dir(path), 0700)
	os.WriteFile(path, []byte(`{"access_token":"fixture"}`), 0600)
	for _, status := range []int{302, 200} {
		calls := 0
		client := &http.Client{Transport: identityTransport(func(r *http.Request) (*http.Response, error) {
			calls++
			return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(`{"email":"unverified@example.com","email_verified":false}`)), Header: http.Header{"Location": []string{"https://example.com/"}}, Request: r}, nil
		})}
		if got := lookupGoogleAccountEmail(context.Background(), home, client); got != "" || calls != 1 {
			t.Fatal("redirect or unverified identity accepted")
		}
	}
}
func TestAccountDisconnectChecksOwnerAndActiveExecution(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	a, err := CreateProviderAccount("owner-a", "Work")
	if err != nil {
		t.Fatal(err)
	}
	if err := DeleteProviderAccount("owner-b", a.ID); err == nil {
		t.Fatal("another owner deleted account")
	}
	release, err := TryLeaseProviderAccount("owner-a", a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := DeleteProviderAccount("owner-a", a.ID); err == nil {
		t.Fatal("active account deleted")
	}
	release()
	if err := DeleteProviderAccount("owner-a", a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := GetProviderAccount("owner-a", a.ID); err == nil {
		t.Fatal("account retained after disconnect")
	}
}

func TestUserAccountListDoesNotExposeInstallationLogin(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path := AntigravityCredentialsFilePath(ConfigHome())
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"token":{"access_token":"fixture-installation-token"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	accounts, err := ListProviderAccounts("owner-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 0 {
		t.Fatal("installation login appeared as user-connected AI account")
	}
}
