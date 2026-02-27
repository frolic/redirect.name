package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/net/publicsuffix"
)

var lookupTXT = net.LookupTXT

func fallback(w http.ResponseWriter, r *http.Request, reason string) {
	location := os.Getenv("FALLBACK_URL")
	if location == "" {
		location = "http://redirect.name/"
	}
	if reason != "" {
		location = fmt.Sprintf("%s#reason=%s", location, url.QueryEscape(reason))
	}
	http.Redirect(w, r, location, 302)
}

func getRedirect(txt []string, url string) (*Redirect, error) {
	var catchAlls []*Config
	for _, record := range txt {
		config := Parse(record)
		if config == nil {
			continue
		}
		if config.From == "" {
			catchAlls = append(catchAlls, config)
			continue
		}
		redirect := Translate(url, config)
		if redirect != nil {
			return redirect, nil
		}
	}

	for _, config := range catchAlls {
		redirect := Translate(url, config)
		if redirect != nil {
			return redirect, nil
		}
	}

	return nil, errors.New("No paths matched")
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ok")
}

func redirectHandler(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.Host, ":")
	host := parts[0]

	hostname := fmt.Sprintf("_redirect.%s", host)
	txt, err := net.LookupTXT(hostname)
	if err != nil {
		fallback(w, r, fmt.Sprintf("Could not resolve hostname (%v)", err))
		return
	}

	redirect, err := getRedirect(txt, r.URL.String())
	if err != nil {
		fallback(w, r, err.Error())
	} else {
		http.Redirect(w, r, redirect.Location, redirect.Status)
	}
}

// hostPolicy validates that a host has a _redirect TXT record before
// autocert will issue a certificate for it.
func hostPolicy(ctx context.Context, host string) error {
	hostname := fmt.Sprintf("_redirect.%s", host)
	txt, err := lookupTXT(hostname)
	if err != nil {
		return fmt.Errorf("DNS lookup failed for %s: %w", hostname, err)
	}
	for _, record := range txt {
		if Parse(record) != nil {
			return nil
		}
	}
	return fmt.Errorf("no valid redirect config in TXT records for %s", hostname)
}

// rateLimitedCache wraps autocert.DirCache and enforces a limit of 2 new
// certificates per apex domain per week to stay well within Let's Encrypt
// rate limits. The counter resets on restart and on weekly rollover.
type rateLimitedCache struct {
	autocert.Cache
	mu     sync.Mutex
	counts map[string]int
	weekOf time.Time
}

func newRateLimitedCache(dir string) *rateLimitedCache {
	return &rateLimitedCache{
		Cache:  autocert.DirCache(dir),
		counts: make(map[string]int),
		weekOf: time.Now().Truncate(7 * 24 * time.Hour),
	}
}

func (c *rateLimitedCache) Put(ctx context.Context, key string, data []byte) error {
	apex, err := publicsuffix.EffectiveTLDPlusOne(key)
	if err != nil {
		// Not a domain key (e.g., acme_account+key) — skip rate limiting.
		return c.Cache.Put(ctx, key, data)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	week := time.Now().Truncate(7 * 24 * time.Hour)
	if !week.Equal(c.weekOf) {
		c.counts = make(map[string]int)
		c.weekOf = week
	}

	if c.counts[apex] >= 2 {
		return fmt.Errorf("rate limit exceeded: 2 certs already issued for %s this week", apex)
	}

	if err := c.Cache.Put(ctx, key, data); err != nil {
		return err
	}
	c.counts[apex]++
	return nil
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthzHandler)
	mux.HandleFunc("/", redirectHandler)

	certDir := os.Getenv("CERT_DIR")
	if certDir == "" {
		port := os.Getenv("PORT")
		if port == "" {
			port = "8081"
		}
		srv := &http.Server{
			Addr:         ":" + port,
			Handler:      mux,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
		}
		log.Printf("Listening on http://0.0.0.0:%s", port)
		log.Fatal(srv.ListenAndServe())
		return
	}

	manager := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		Cache:      newRateLimitedCache(certDir),
		HostPolicy: hostPolicy,
	}

	httpSrv := &http.Server{
		Addr:         ":80",
		Handler:      manager.HTTPHandler(mux),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}
	go func() {
		log.Fatal(httpSrv.ListenAndServe())
	}()

	httpsSrv := &http.Server{
		Addr:         ":443",
		Handler:      mux,
		TLSConfig:    manager.TLSConfig(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}
	log.Printf("Listening on :80 and :443")
	log.Fatal(httpsSrv.ListenAndServeTLS("", ""))
}
