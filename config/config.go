// Package config reads and validates the environment once at startup.
package config

import (
	"log"
	"net/url"
	"os"
	"strings"
)

var (
	APIURL  = requireURL("NN_API_URL")
	SiteURL = requireURL("NN_SITE_URL")
)

// requireURL reads an http(s) URL from the environment, trimming trailing
// slashes, and exits with a clear message if it is unset or malformed.
func requireURL(name string) string {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		log.Fatalf("%s environment variable must be set", name)
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		log.Fatalf("%s must be an http(s) URL, got %q", name, raw)
	}
	return strings.TrimRight(raw, "/")
}
