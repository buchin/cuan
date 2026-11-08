package webfetch

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"
)

type publicResolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}
type publicConnector func(context.Context, string, string) (net.Conn, error)

var nonPublicRanges = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("fec0::/10"), netip.MustParsePrefix("3fff::/20"),
	netip.MustParsePrefix("100.64.0.0/10"), netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"), netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"), netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"), netip.MustParsePrefix("::/96"),
	netip.MustParsePrefix("64:ff9b::/96"), netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("100::/64"), netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"), netip.MustParsePrefix("2002::/16"),
}

func publicAddress(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() {
		return false
	}
	for _, prefix := range nonPublicRanges {
		if prefix.Contains(addr) {
			return false
		}
	}
	return true
}

// Validate the DNS answers at connection time, then dial the validated numeric
// address. Redirects and fresh connections use the same gate; a second DNS
// lookup cannot replace an approved answer with an internal address.
func publicDialer(resolver publicResolver, connect publicConnector) publicConnector {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("invalid web destination")
		}
		addresses, err := resolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("public web destination could not be resolved: %w", err)
		}
		if len(addresses) == 0 {
			return nil, fmt.Errorf("public web destination has no addresses")
		}
		for _, address := range addresses {
			if address.Zone != "" || !publicAddress(address.IP) {
				return nil, fmt.Errorf("web_fetch only permits public network destinations")
			}
		}
		var last error
		for _, address := range addresses {
			connection, err := connect(ctx, network, net.JoinHostPort(address.IP.String(), port))
			if err == nil {
				return connection, nil
			}
			last = err
			if ctx.Err() != nil {
				break
			}
		}
		return nil, last
	}
}

func newPublicClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}
	return &http.Client{Timeout: timeout, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("web_fetch redirect limit reached")
		}
		if req.URL.User != nil || req.URL.Hostname() == "" || (strings.ToLower(req.URL.Scheme) != "http" && strings.ToLower(req.URL.Scheme) != "https") {
			return fmt.Errorf("web_fetch redirects must remain public HTTP(S) URLs without credentials")
		}
		return nil
	}, Transport: &http.Transport{
		// Ambient HTTP proxies must not bypass destination validation or receive
		// public URL data through an unrelated developer configuration.
		Proxy: nil, DialContext: publicDialer(net.DefaultResolver, dialer.DialContext),
		ForceAttemptHTTP2: true, TLSHandshakeTimeout: timeout, ResponseHeaderTimeout: timeout,
		DisableKeepAlives: true,
	}}
}
