package attribution

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCheckURL(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {})
	mux.HandleFunc("/missing", http.NotFound)
	mux.HandleFunc("/no-head", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/moved", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ok", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/moved-to-missing", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/missing", http.StatusFound)
	})
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	closed := httptest.NewServer(mux)
	closedURL := closed.URL
	closed.Close()

	tests := []struct {
		name, url, wantErr string
	}{
		{"success", srv.URL + "/ok", ""},
		{"not found", srv.URL + "/missing", "HTTP 404"},
		{"HEAD refused, GET answers", srv.URL + "/no-head", ""},
		{"redirect to success", srv.URL + "/moved", ""},
		{"redirect to not found", srv.URL + "/moved-to-missing", "HTTP 404"},
		{"timeout", srv.URL + "/slow", "Client.Timeout"},
		{"connection refused", closedURL + "/ok", "connection refused"},
	}
	client := &http.Client{Timeout: 200 * time.Millisecond}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckURL(context.Background(), client, tc.url)
			if tc.wantErr == "" {
				if err != nil {
					t.Errorf("CheckURL(%s) = %v, want nil", tc.url, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) || !strings.Contains(err.Error(), tc.url) {
				t.Errorf("CheckURL(%s) = %v, want an error naming the URL and containing %q", tc.url, err, tc.wantErr)
			}
		})
	}
}

func TestCheckURL_SendsUserAgent(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.UserAgent()
	}))
	defer srv.Close()

	if err := CheckURL(context.Background(), srv.Client(), srv.URL); err != nil {
		t.Fatalf("CheckURL: %v", err)
	}
	if got != "City2TABULA attribution check" {
		t.Errorf("User-Agent = %q, want %q", got, "City2TABULA attribution check")
	}
}

func TestRefuseInternal(t *testing.T) {
	refused := []string{
		"127.0.0.1:80", "[::1]:443", "10.1.2.3:80", "172.16.0.1:80", "192.168.1.1:8080",
		"169.254.169.254:80", "100.64.0.1:80", "0.0.0.0:80", "[fd00::1]:443", "[fe80::1]:443",
		"[::ffff:127.0.0.1]:80", "224.0.0.1:80",
	}
	for _, addr := range refused {
		if err := refuseInternal("tcp", addr, nil); err == nil || !strings.Contains(err.Error(), "refused internal address") {
			t.Errorf("refuseInternal(%s) = %v, want a refusal", addr, err)
		}
	}
	for _, addr := range []string{"93.184.215.14:443", "[2606:2800:21f:cb07:6820:80da:af6b:8b2c]:443", "100.128.0.1:80"} {
		if err := refuseInternal("tcp", addr, nil); err != nil {
			t.Errorf("refuseInternal(%s) = %v, want nil", addr, err)
		}
	}
}

// The check client must refuse a server on an internal address, which is
// where a hostile redirect would send it.
func TestNewURLCheckClient_RefusesInternalHost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	err := CheckURL(context.Background(), NewURLCheckClient(), srv.URL)
	if err == nil || !strings.Contains(err.Error(), "refused internal address 127.0.0.1") {
		t.Errorf("CheckURL(%s) with the check client = %v, want a refusal of 127.0.0.1", srv.URL, err)
	}
}
