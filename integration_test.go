package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// noFollowClient is an HTTP client that does not follow redirects,
// so tests can inspect the redirect response directly.
var noFollowClient = &http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// newTestServer starts a real HTTP server backed by the redirect handler
// with the given DNS stub. The caller must call ts.Close().
func newTestServer(t *testing.T, txt []string) *httptest.Server {
	t.Helper()
	orig := lookupTXT
	t.Cleanup(func() { lookupTXT = orig })
	lookupTXT = func(host string) ([]string, error) {
		return txt, nil
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthzHandler)
	mux.HandleFunc("/", redirectHandler)
	return httptest.NewServer(mux)
}

func TestIntegration_302(t *testing.T) {
	ts := newTestServer(t, []string{"Redirects to https://example.com/landing"})
	defer ts.Close()

	resp, err := noFollowClient.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Errorf("status: want 302, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "https://example.com/landing" {
		t.Errorf("Location: want https://example.com/landing, got %q", loc)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "" {
		t.Errorf("Cache-Control: want empty on 302, got %q", cc)
	}
}

func TestIntegration_301(t *testing.T) {
	ts := newTestServer(t, []string{"Redirects permanently to https://example.com/"})
	defer ts.Close()

	resp, err := noFollowClient.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusMovedPermanently {
		t.Errorf("status: want 301, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "https://example.com/" {
		t.Errorf("Location: want https://example.com/, got %q", loc)
	}
	if cc := resp.Header.Get("Cache-Control"); cc == "" {
		t.Error("Cache-Control: want header on 301, got none")
	}
}

func TestIntegration_308(t *testing.T) {
	ts := newTestServer(t, []string{"Redirects to https://example.com/ with 308"})
	defer ts.Close()

	resp, err := noFollowClient.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusPermanentRedirect {
		t.Errorf("status: want 308, got %d", resp.StatusCode)
	}
	if cc := resp.Header.Get("Cache-Control"); cc == "" {
		t.Error("Cache-Control: want header on 308, got none")
	}
}

func TestIntegration_PathMatch(t *testing.T) {
	ts := newTestServer(t, []string{
		"Redirects from /docs/* to https://docs.example.com/*",
		"Redirects to https://example.com/",
	})
	defer ts.Close()

	// Specific path match takes priority.
	resp, err := noFollowClient.Get(ts.URL + "/docs/intro")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Errorf("status: want 302, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "https://docs.example.com/intro" {
		t.Errorf("Location: want https://docs.example.com/intro, got %q", loc)
	}

	// Catch-all applies when no path matches.
	resp, err = noFollowClient.Get(ts.URL + "/other")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Errorf("status: want 302, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "https://example.com/" {
		t.Errorf("Location: want https://example.com/, got %q", loc)
	}
}

func TestIntegration_DNSFailure(t *testing.T) {
	orig := lookupTXT
	t.Cleanup(func() { lookupTXT = orig })
	lookupTXT = func(host string) ([]string, error) {
		return nil, &dnsError{"no such host"}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", redirectHandler)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := noFollowClient.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// DNS failure should fall back with a 302 to the fallback URL.
	if resp.StatusCode != http.StatusFound {
		t.Errorf("status: want 302 fallback, got %d", resp.StatusCode)
	}
}

func TestIntegration_Healthz(t *testing.T) {
	ts := newTestServer(t, nil)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("healthz: want 200, got %d", resp.StatusCode)
	}
}

// dnsError is a minimal error type used to simulate DNS failures.
type dnsError struct{ msg string }

func (e *dnsError) Error() string { return e.msg }
