package config

import (
	"fmt"
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
	value, err := parseURL(name, os.Getenv(name))
	if err != nil {
		log.Fatal(err)
	}
	return value
}

func parseURL(name, value string) (string, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return "", fmt.Errorf("%s environment variable must be set", name)
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || !validPort(u.Port()) {
		return "", fmt.Errorf("%s must be an http(s) URL, got %q", name, raw)
	}
	return strings.TrimRight(raw, "/"), nil
}

func validPort(port string) bool {
	if port == "" {
		return true
	}
	n, err := strconv.Atoi(port)
	return err == nil && n >= 1 && n <= 65535
}
