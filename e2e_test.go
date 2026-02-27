//go:build e2e

package main

import (
	"net/http"
	"testing"
	"time"
)

var e2eClient = &http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
	Timeout: 10 * time.Second,
}

// TestE2E_GithubRedirect verifies that github.redirect.name redirects to the
// GitHub repo. Requires real DNS and a live server.
//
// Run with: go test -tags e2e -v ./...
func TestE2E_GithubRedirect(t *testing.T) {
	resp, err := e2eClient.Get("http://github.redirect.name/")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Errorf("status: want 302, got %d", resp.StatusCode)
	}

	want := "https://github.com/frolic/redirect.name"
	if loc := resp.Header.Get("Location"); loc != want {
		t.Errorf("Location: want %q, got %q", want, loc)
	}
}

// TestE2E_HTTPS verifies that the same redirect is served over HTTPS and that
// the TLS handshake completes (i.e. a certificate has been provisioned).
func TestE2E_HTTPS(t *testing.T) {
	resp, err := e2eClient.Get("https://github.redirect.name/")
	if err != nil {
		t.Fatalf("HTTPS request failed (cert may not be provisioned): %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Errorf("status: want 302, got %d", resp.StatusCode)
	}

	want := "https://github.com/frolic/redirect.name"
	if loc := resp.Header.Get("Location"); loc != want {
		t.Errorf("Location: want %q, got %q", want, loc)
	}
}

// TestE2E_Healthz verifies the healthz endpoint on the live server.
func TestE2E_Healthz(t *testing.T) {
	resp, err := e2eClient.Get("https://github.redirect.name/healthz")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("healthz: want 200, got %d", resp.StatusCode)
	}
}
