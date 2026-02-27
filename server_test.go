package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetRedirectSimple(t *testing.T) {
	var redirect *Redirect
	var err error

	dnsTXT := []string{
		"Redirects from /test/* to https://github.com/holic/*",
	}

	redirect, err = getRedirect(dnsTXT, "/test/")
	assertEqual(t, err, nil)
	assertEqual(t, redirect.Location, "https://github.com/holic/")

	redirect, err = getRedirect(dnsTXT, "/test/success")
	assertEqual(t, err, nil)
	assertEqual(t, redirect.Location, "https://github.com/holic/success")

	redirect, err = getRedirect(dnsTXT, "/should/fail")
	assertEqual(t, err.Error(), "No paths matched")
}

func TestGetRedirectComplex(t *testing.T) {
	// Tests that catchalls (even interspersed in the TXT records) apply
	// only after more specific matches
	var redirect *Redirect
	var err error

	dnsTXT := []string{
		"Redirects from /test/* to https://github.com/holic/*",
		"Redirects to https://github.com/holic",
		"Redirects from /noglob/ to https://github.com/holic/noglob",
	}

	redirect, err = getRedirect(dnsTXT, "/")
	assertEqual(t, err, nil)
	assertEqual(t, redirect.Location, "https://github.com/holic")

	redirect, err = getRedirect(dnsTXT, "/test/somepath")
	assertEqual(t, err, nil)
	assertEqual(t, redirect.Location, "https://github.com/holic/somepath")

	redirect, err = getRedirect(dnsTXT, "/noglob/")
	assertEqual(t, err, nil)
	assertEqual(t, redirect.Location, "https://github.com/holic/noglob")

	redirect, err = getRedirect(dnsTXT, "/catch/all")
	assertEqual(t, err, nil)
	assertEqual(t, redirect.Location, "https://github.com/holic")
}

func TestRedirectHandler301CacheControl(t *testing.T) {
	orig := lookupTXT
	defer func() { lookupTXT = orig }()
	lookupTXT = func(host string) ([]string, error) {
		return []string{"Redirects permanently to https://example.com/"}, nil
	}

	req := httptest.NewRequest("GET", "http://go.example.com/", nil)
	rr := httptest.NewRecorder()
	redirectHandler(rr, req)

	if rr.Code != http.StatusMovedPermanently {
		t.Errorf("expected 301, got %d", rr.Code)
	}
	cc := rr.Header().Get("Cache-Control")
	if cc == "" {
		t.Error("expected Cache-Control header on 301, got none")
	}
}

func TestRedirectHandler302NoCacheControl(t *testing.T) {
	orig := lookupTXT
	defer func() { lookupTXT = orig }()
	lookupTXT = func(host string) ([]string, error) {
		return []string{"Redirects to https://example.com/"}, nil
	}

	req := httptest.NewRequest("GET", "http://go.example.com/", nil)
	rr := httptest.NewRecorder()
	redirectHandler(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("expected 302, got %d", rr.Code)
	}
	if cc := rr.Header().Get("Cache-Control"); cc != "" {
		t.Errorf("expected no Cache-Control on 302, got %q", cc)
	}
}

func TestHostPolicy(t *testing.T) {
	orig := lookupTXT
	defer func() { lookupTXT = orig }()

	// Valid: TXT record contains a parseable redirect config
	lookupTXT = func(host string) ([]string, error) {
		return []string{"Redirects to https://example.com"}, nil
	}
	if err := hostPolicy(context.Background(), "foo.example.com"); err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// DNS error
	lookupTXT = func(host string) ([]string, error) {
		return nil, errors.New("no such host")
	}
	if err := hostPolicy(context.Background(), "foo.example.com"); err == nil {
		t.Error("expected error for DNS failure")
	}

	// TXT records exist but none parse as redirect configs
	lookupTXT = func(host string) ([]string, error) {
		return []string{"v=spf1 include:example.com ~all"}, nil
	}
	if err := hostPolicy(context.Background(), "foo.example.com"); err == nil {
		t.Error("expected error when no valid redirect config found")
	}
}

func TestRateLimitedCache(t *testing.T) {
	dir := t.TempDir()
	cache := newRateLimitedCache(dir)
	ctx := context.Background()
	data := []byte("test-cert-data")

	// First two certs for the same apex succeed
	if err := cache.Put(ctx, "sub1.example.com", data); err != nil {
		t.Fatalf("first put failed: %v", err)
	}
	if err := cache.Put(ctx, "sub2.example.com", data); err != nil {
		t.Fatalf("second put failed: %v", err)
	}

	// Third cert for the same apex is rate limited
	if err := cache.Put(ctx, "sub3.example.com", data); err == nil {
		t.Error("expected rate limit error for third cert this week")
	}

	// Different apex is not affected
	if err := cache.Put(ctx, "sub1.other.org", data); err != nil {
		t.Fatalf("different apex put failed: %v", err)
	}

	// Non-domain keys (e.g., acme account key) bypass rate limiting
	if err := cache.Put(ctx, "acme_account+key", data); err != nil {
		t.Fatalf("acme account key put failed: %v", err)
	}
}
