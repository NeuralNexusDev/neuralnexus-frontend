package config

import (
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
)

var (
	APIURL  = requireURL("NN_API_URL")
	SiteURL = requireURL("NN_SITE_URL")
)

func requireURL(name string) string {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		log.Fatalf("%s environment variable must be set", name)
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || !validPort(u.Port()) {
		log.Fatalf("%s must be an http(s) URL, got %q", name, raw)
	}
	return strings.TrimRight(raw, "/")
}

func validPort(port string) bool {
	if port == "" {
		return true
	}
	n, err := strconv.Atoi(port)
	return err == nil && n >= 1 && n <= 65535
}
