package webfetch

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type resolverFixture func(context.Context, string) ([]net.IPAddr, error)

func (f resolverFixture) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	return f(ctx, host)
}

func TestPublicAddressBoundary(t *testing.T) {
	for _, s := range []string{"127.0.0.1", "10.1.2.3", "172.16.0.1", "192.168.0.1", "169.254.169.254", "100.64.0.1", "0.1.2.3", "192.0.2.1", "198.19.1.1", "224.0.0.1", "240.1.1.1", "::1", "::ffff:127.0.0.1", "fc00::1", "fe80::1", "fec0::1", "2001:db8::1", "64:ff9b::a00:1"} {
		if publicAddress(net.ParseIP(s)) {
			t.Errorf("nonpublic address accepted: %s", s)
		}
	}
	for _, s := range []string{"1.1.1.1", "8.8.8.8", "2606:4700:4700::1111", "::ffff:8.8.8.8"} {
		if !publicAddress(net.ParseIP(s)) {
			t.Errorf("public address rejected: %s", s)
		}
	}
}

func TestPublicDialPinsValidatedAddressAndRejectsMixedDNS(t *testing.T) {
	for _, mixed := range []bool{false, true} {
		called := ""
		resolve := resolverFixture(func(context.Context, string) ([]net.IPAddr, error) {
			rows := []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}}
			if mixed {
				rows = append(rows, net.IPAddr{IP: net.ParseIP("127.0.0.1")})
			}
			return rows, nil
		})
		connect := func(_ context.Context, _, address string) (net.Conn, error) {
			called = address
			return nil, errors.New("fixture connection not opened")
		}
		_, err := publicDialer(resolve, connect)(context.Background(), "tcp", "example.test:443")
		if err == nil {
			t.Fatal("fixture should not establish a connection")
		}
		if mixed && called != "" {
			t.Fatal("mixed private answer reached connector")
		}
		if !mixed && called != "8.8.8.8:443" {
			t.Fatal("connector performed second hostname resolution", called)
		}
	}
}

func TestPublicFetchRejectsInternalRedirectBeforeConnecting(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.Redirect(w, r, "http://127.0.0.1/private", http.StatusFound)
	}))
	defer server.Close()
	client := newPublicClient(DefaultTimeout)
	transport := client.Transport.(*http.Transport)
	transport.DialContext = publicDialer(resolverFixture(func(_ context.Context, host string) ([]net.IPAddr, error) {
		if host == "example.test" {
			return []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}}, nil
		}
		return []net.IPAddr{{IP: net.ParseIP(host)}}, nil
	}), func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != "8.8.8.8:80" {
			t.Errorf("private destination reached connector: %s", address)
		}
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	})
	_, err := FetchWithOptions(context.Background(), "http://example.test", Options{Client: client})
	if err == nil || !strings.Contains(err.Error(), "public network") || requests != 1 {
		t.Fatalf("redirect boundary failed: requests=%d err=%v", requests, err)
	}
}

func TestDefaultPublicFetchCannotReachLoopback(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++; w.Write([]byte("private fixture")) }))
	defer server.Close()
	if _, err := Fetch(context.Background(), server.URL); err == nil {
		t.Fatal("default public tool reached loopback")
	}
	if requests != 0 {
		t.Fatal("private handler was invoked")
	}
	if _, err := Fetch(context.Background(), "https://user:password@example.com"); err == nil {
		t.Fatal("embedded credentials accepted")
	}
}
