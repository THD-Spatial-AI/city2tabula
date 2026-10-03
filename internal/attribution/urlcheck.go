package attribution

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// URLCheckTimeout bounds one URL check, redirects included.
const URLCheckTimeout = 15 * time.Second

// NewURLCheckClient returns the HTTP client CheckURL is meant to be called with.
func NewURLCheckClient() *http.Client {
	return &http.Client{Timeout: URLCheckTimeout}
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
