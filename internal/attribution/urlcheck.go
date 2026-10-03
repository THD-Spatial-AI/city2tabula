package attribution

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"syscall"
	"time"
)

// URLCheckTimeout bounds one URL check, redirects included.
const URLCheckTimeout = 15 * time.Second

// NewURLCheckClient returns the HTTP client CheckURL is meant to be called with.
// It refuses to connect to a non-public address, so a provider page redirected
// to an internal host cannot make the check probe the server's own network.
// The address is checked after DNS resolution, at every connection, redirects
// included. Proxies are not used: through one, only the proxy's address would
// be checked.
func NewURLCheckClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = (&net.Dialer{Timeout: URLCheckTimeout, Control: refuseInternal}).DialContext
	return &http.Client{Timeout: URLCheckTimeout, Transport: transport}
}

// sharedAddressSpace is 100.64.0.0/10, carrier-grade NAT, which netip does not
// count as private.
var sharedAddressSpace = netip.MustParsePrefix("100.64.0.0/10")

// refuseInternal is a net.Dialer Control function that fails a connection to a
// loopback, private, link-local (cloud metadata included), shared, multicast or
// unspecified address.
func refuseInternal(_, address string, _ syscall.RawConn) error {
	addrPort, err := netip.ParseAddrPort(address)
	if err != nil {
		return fmt.Errorf("parse dial address %s: %w", address, err)
	}
	ip := addrPort.Addr().Unmap()
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() || sharedAddressSpace.Contains(ip) {
		return fmt.Errorf("refused internal address %s", ip)
	}
	return nil
}

// CheckURL reports whether url answers with a success status after redirects.
// It asks with HEAD first and falls back to GET when HEAD fails or is refused,
// since some providers' servers reject HEAD outright.
func CheckURL(ctx context.Context, client *http.Client, url string) error {
	if request(ctx, client, http.MethodHead, url) == nil {
		return nil
	}
	return request(ctx, client, http.MethodGet, url)
}

// request sends one request and fails on a transport error or a status of 400
// or above. The body is never read.
func request(ctx context.Context, client *http.Client, method, url string) error {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return fmt.Errorf("build %s request for %s: %w", method, url, err)
	}
	// Some servers refuse Go's default User-Agent.
	req.Header.Set("User-Agent", "City2TABULA attribution check")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%s %s: HTTP %d", method, url, resp.StatusCode)
	}
	return nil
}
