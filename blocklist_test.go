package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseBlocklist(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  map[string]bool
	}{
		{"empty string", "", map[string]bool{}},
		{"single domain", "evil.com", map[string]bool{"evil.com": true}},
		{"multiple domains", "evil.com,bad.org", map[string]bool{"evil.com": true, "bad.org": true}},
		{"whitespace and casing", " Evil.COM , Bad.ORG ", map[string]bool{"evil.com": true, "bad.org": true}},
		{"trailing comma", "evil.com,", map[string]bool{"evil.com": true}},
		{"empty entries", "evil.com,,bad.org", map[string]bool{"evil.com": true, "bad.org": true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseBlocklist(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("parseBlocklist(%q) has %d entries, want %d", tt.input, len(got), len(tt.want))
			}
			for k := range tt.want {
				if !got[k] {
					t.Errorf("parseBlocklist(%q) missing key %q", tt.input, k)
				}
			}
		})
	}
}

func TestIsBlocked(t *testing.T) {
	orig := blocklist
	defer func() { blocklist = orig }()

	// Empty blocklist blocks nothing
	blocklist = map[string]bool{}
	if isBlocked("anything.com") {
		t.Error("empty blocklist should not block")
	}

	// Nil blocklist blocks nothing
	blocklist = nil
	if isBlocked("anything.com") {
		t.Error("nil blocklist should not block")
	}

	blocklist = map[string]bool{"evil.com": true}

	// Exact apex match
	if !isBlocked("evil.com") {
		t.Error("expected evil.com to be blocked")
	}

	// Subdomain match
	if !isBlocked("phish.evil.com") {
		t.Error("expected phish.evil.com to be blocked")
	}

	// Deep subdomain match
	if !isBlocked("a.b.c.evil.com") {
		t.Error("expected a.b.c.evil.com to be blocked")
	}

	// Non-blocked domain
	if isBlocked("good.com") {
		t.Error("expected good.com to not be blocked")
	}
}

func TestRedirectHandlerBlocked(t *testing.T) {
	origBlocklist := blocklist
	origLookup := lookupTXT
	defer func() {
		blocklist = origBlocklist
		lookupTXT = origLookup
	}()

	blocklist = map[string]bool{"evil.com": true}
	lookupTXT = func(host string) ([]string, error) {
		t.Error("lookupTXT should not be called for blocked domain")
		return nil, nil
	}

	req := httptest.NewRequest("GET", "http://evil.com/", nil)
	rr := httptest.NewRecorder()
	redirectHandler(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestRedirectHandlerBlockedSubdomain(t *testing.T) {
	origBlocklist := blocklist
	origLookup := lookupTXT
	defer func() {
		blocklist = origBlocklist
		lookupTXT = origLookup
	}()

	blocklist = map[string]bool{"evil.com": true}
	lookupTXT = func(host string) ([]string, error) {
		t.Error("lookupTXT should not be called for blocked subdomain")
		return nil, nil
	}

	req := httptest.NewRequest("GET", "http://phish.evil.com/", nil)
	rr := httptest.NewRecorder()
	redirectHandler(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestHostPolicyBlocked(t *testing.T) {
	origBlocklist := blocklist
	origLookup := lookupTXT
	defer func() {
		blocklist = origBlocklist
		lookupTXT = origLookup
	}()

	blocklist = map[string]bool{"evil.com": true}
	lookupTXT = func(host string) ([]string, error) {
		t.Error("lookupTXT should not be called for blocked domain")
		return nil, nil
	}

	if err := hostPolicy(context.Background(), "evil.com"); err == nil {
		t.Error("expected error for blocked apex")
	}

	if err := hostPolicy(context.Background(), "phish.evil.com"); err == nil {
		t.Error("expected error for blocked subdomain")
	}
}

func TestRedirectHandlerNotBlocked(t *testing.T) {
	origBlocklist := blocklist
	origLookup := lookupTXT
	defer func() {
		blocklist = origBlocklist
		lookupTXT = origLookup
	}()

	blocklist = map[string]bool{"evil.com": true}
	lookupTXT = func(host string) ([]string, error) {
		return []string{"Redirects to https://example.com/"}, nil
	}

	req := httptest.NewRequest("GET", "http://good.com/", nil)
	rr := httptest.NewRecorder()
	redirectHandler(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("expected 302, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "https://example.com/" {
		t.Errorf("expected redirect to https://example.com/, got %q", loc)
	}
}
