package main

import (
	"strings"

	"golang.org/x/net/publicsuffix"
)

var blocklist map[string]bool

func parseBlocklist(s string) map[string]bool {
	m := make(map[string]bool)
	for _, entry := range strings.Split(s, ",") {
		entry = strings.TrimSpace(strings.ToLower(entry))
		if entry != "" {
			m[entry] = true
		}
	}
	return m
}

func isBlocked(host string) bool {
	if len(blocklist) == 0 {
		return false
	}
	apex, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err != nil {
		return false
	}
	return blocklist[apex]
}
