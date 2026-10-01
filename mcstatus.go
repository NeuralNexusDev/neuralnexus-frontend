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
	problemJSON      = "application/problem+json"
)

var (
	mcStatusClient = &http.Client{Timeout: mcStatusTimeout}
	mcColorCode    = regexp.MustCompile(`(?s)§.`)
	mcHostPattern  = regexp.MustCompile(`^(?P<host>[A-Za-z0-9_](?:[A-Za-z0-9_-]{0,61}[A-Za-z0-9_])?(?:\.[A-Za-z0-9_](?:[A-Za-z0-9_-]{0,61}[A-Za-z0-9_])?)*)(?::(?P<port>[0-9]{1,5}))?$`)
)

type apiMcStatus struct {
	Motd       string `json:"motd"`
	NumPlayers int    `json:"num_players"`
	MaxPlayers int    `json:"max_players"`
	Version    string `json:"version"`
}

// McStatusEmbedHandler serves the link-preview page for a server status.
func McStatusEmbedHandler(w http.ResponseWriter, r *http.Request) {
	m := mcHostPattern.FindStringSubmatch(r.PathValue("host"))
	var hostPort uint64
	var portErr error
	if m != nil && m[2] != "" {
		hostPort, portErr = strconv.ParseUint(m[2], 10, 16)
	}
	if m == nil || len(m[1]) > maxMcHostname || (m[2] != "" && (portErr != nil || hostPort < 1)) {
		w.Header().Set("Cache-Control", noStore)
		http.Error(w, "invalid server address", http.StatusBadRequest)
		return
	}
	host := strings.ToLower(m[1])
	if m[2] != "" {
		host += ":" + strconv.FormatUint(hostPort, 10)
	}

	query := r.URL.Query()
	data := components.McStatusEmbedData{
		Host:    host,
		Bedrock: query.Get("bedrock") == "true",
		Query:   query.Get("query") != "false",
	}
	if port, err := strconv.ParseUint(query.Get("query_port"), 10, 16); err == nil && port >= 1 {
		data.QueryPort = int(port)
	}

	endpoint := config.APIURL + "/api/v1/mcstatus/" + url.PathEscape(host)
	if q := data.Options().Encode(); q != "" {
		endpoint += "?" + q
	}
	var status apiMcStatus
	var cache string
	code, message, retryAfter := http.StatusServiceUnavailable, "status lookup unavailable", unavailableRetry
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, endpoint, nil)
	var res *http.Response
	if err == nil {
		res, err = mcStatusClient.Do(req)
	}
	if err == nil {
		defer res.Body.Close()
		mediaType, _, _ := mime.ParseMediaType(res.Header.Get("Content-Type"))
		switch {
		case res.StatusCode == http.StatusOK:
			if json.NewDecoder(io.LimitReader(res.Body, maxMcStatusBody)).Decode(&status) == nil && status != (apiMcStatus{}) {
				code, cache = http.StatusOK, onlineMaxAge
				data.Online = true
			}
		case res.StatusCode == http.StatusNotFound && mediaType == problemJSON:
			code, cache = http.StatusOK, offlineMaxAge
		case res.StatusCode == http.StatusTooManyRequests:
			message, retryAfter = "status lookups are rate limited", rateLimitedRetry
		case res.StatusCode == http.StatusInternalServerError:
			message, retryAfter = "status lookup unavailable", unavailableRetry
		default:
			code, message, retryAfter = http.StatusBadGateway, "unexpected status response", ""
		}
	}
	if code != http.StatusOK {
		if retryAfter != "" {
			w.Header().Set("Retry-After", retryAfter)
		}
		w.Header().Set("Cache-Control", noStore)
		http.Error(w, message, code)
		return
	}

	if data.Online {
		motd := strings.ReplaceAll(status.Motd, `\n`, "\n")
		motd = mcColorCode.ReplaceAllString(motd, "")
		for _, line := range strings.Split(motd, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				data.Motd = append(data.Motd, line)
			}
		}
		data.Players = status.NumPlayers
		data.Max = status.MaxPlayers
		data.Version = status.Version
	}
	w.Header().Set("Cache-Control", cache)
	templ.Handler(components.McStatusEmbedPage(data)).ServeHTTP(w, r)
}
