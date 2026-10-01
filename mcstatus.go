package main

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/components"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/config"
)

const (
	mcStatusTimeout  = 15 * time.Second
	maxMcStatusBody  = 1 << 20
	noStore          = "no-store"
	onlineMaxAge     = "public, max-age=60"
	offlineMaxAge    = "public, max-age=30"
	unavailableRetry = "30"
	rateLimitedRetry = "60"
	maxMcHostname    = 253
	maxMcPort        = 65535
	problemJSON      = "application/problem+json"
)

var (
	mcStatusClient = &http.Client{Timeout: mcStatusTimeout}
	mcColorCode    = regexp.MustCompile(`(?s)§.`)
	mcHostPattern  = regexp.MustCompile(`^(?P<host>(?:[A-Za-z0-9_](?:[A-Za-z0-9_-]{0,61}[A-Za-z0-9_])?(?:\.[A-Za-z0-9_](?:[A-Za-z0-9_-]{0,61}[A-Za-z0-9_])?)*))(?::(?P<port>[0-9]{1,5}))?$`)
)

type lookupResult int

const (
	lookupOnline lookupResult = iota
	lookupOffline
	lookupUnavailable
	lookupRateLimited
	lookupUnexpected
)

type apiMcStatus struct {
	Motd       string `json:"motd"`
	NumPlayers int    `json:"num_players"`
	MaxPlayers int    `json:"max_players"`
	Version    string `json:"version"`
}

// The API sends line breaks as a literal backslash-n.
func motdLines(motd string) []string {
	motd = strings.ReplaceAll(motd, `\n`, "\n")
	motd = mcColorCode.ReplaceAllString(motd, "")
	var lines []string
	for _, line := range strings.Split(motd, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// 404 means the server gave no status, but only as a problem+json response: a
// bare 404 comes from a proxy or a wrong API URL and must not be cached as
// "offline". 429 and 500 must not be cached as "offline" either. This client
// sends no Authorization header, so 401 is unexpected.
func fetchMcStatus(r *http.Request, data components.McStatusEmbedData) (apiMcStatus, lookupResult) {
	var status apiMcStatus
	endpoint := config.APIURL + "/api/v1/mcstatus/" + url.PathEscape(data.Host)
	if q := data.Options().Encode(); q != "" {
		endpoint += "?" + q
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, endpoint, nil)
	if err != nil {
		return status, lookupUnavailable
	}
	res, err := mcStatusClient.Do(req)
	if err != nil {
		return status, lookupUnavailable
	}
	defer res.Body.Close()
	switch res.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		if mediaType, _, _ := mime.ParseMediaType(res.Header.Get("Content-Type")); mediaType != problemJSON {
			return status, lookupUnexpected
		}
		return status, lookupOffline
	case http.StatusInternalServerError:
		return status, lookupUnavailable
	case http.StatusTooManyRequests:
		return status, lookupRateLimited
	default:
		return status, lookupUnexpected
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, maxMcStatusBody)).Decode(&status); err != nil {
		return status, lookupUnavailable
	}
	if status == (apiMcStatus{}) {
		return status, lookupUnavailable
	}
	return status, lookupOnline
}

func parseMcPort(raw string) (int, bool) {
	n, err := strconv.Atoi(raw)
	return n, err == nil && n >= 1 && n <= maxMcPort
}

func normalizeMcHost(raw string) (string, bool) {
	m := mcHostPattern.FindStringSubmatch(raw)
	if m == nil || len(m[1]) > maxMcHostname {
		return "", false
	}
	if port := m[2]; port != "" {
		n, ok := parseMcPort(port)
		if !ok {
			return "", false
		}
		return strings.ToLower(m[1]) + ":" + strconv.Itoa(n), true
	}
	return strings.ToLower(m[1]), true
}

func writeLookupFailure(w http.ResponseWriter, code int, message, retryAfter string) {
	if retryAfter != "" {
		w.Header().Set("Retry-After", retryAfter)
	}
	w.Header().Set("Cache-Control", noStore)
	http.Error(w, message, code)
}

func McStatusEmbedHandler(w http.ResponseWriter, r *http.Request) {
	host, ok := normalizeMcHost(r.PathValue("host"))
	if !ok {
		w.Header().Set("Cache-Control", noStore)
		http.Error(w, "invalid server address", http.StatusBadRequest)
		return
	}
	query := r.URL.Query()
	data := components.McStatusEmbedData{
		Host:    host,
		Bedrock: query.Get("bedrock") == "true",
		Query:   query.Get("query") != "false",
	}
	if n, ok := parseMcPort(query.Get("query_port")); ok {
		data.QueryPort = n
	}
	status, result := fetchMcStatus(r, data)
	switch result {
	case lookupUnavailable:
		writeLookupFailure(w, http.StatusServiceUnavailable, "status lookup unavailable", unavailableRetry)
		return
	case lookupRateLimited:
		writeLookupFailure(w, http.StatusServiceUnavailable, "status lookups are rate limited", rateLimitedRetry)
		return
	case lookupUnexpected:
		writeLookupFailure(w, http.StatusBadGateway, "unexpected status response", "")
		return
	case lookupOnline:
		data.Online = true
		data.Motd = motdLines(status.Motd)
		data.Players = status.NumPlayers
		data.Max = status.MaxPlayers
		data.Version = status.Version
		w.Header().Set("Cache-Control", onlineMaxAge)
	default:
		w.Header().Set("Cache-Control", offlineMaxAge)
	}
	templ.Handler(components.McStatusEmbedPage(data)).ServeHTTP(w, r)
}
