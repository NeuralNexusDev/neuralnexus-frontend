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
	mcStatusTimeout    = 15 * time.Second
	maxMcStatusBody    = 1 << 20
	noStore            = "no-store"
	onlineMaxAge       = "public, max-age=60"
	offlineMaxAge      = "public, max-age=30"
	unavailableRetry   = "30"
	unavailableMessage = "status lookup unavailable"
	rateLimitedRetry   = "60"
	maxMcHostname      = 253
	problemJSON        = "application/problem+json"
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
	fail := func(code int, message, retryAfter string) {
		if retryAfter != "" {
			w.Header().Set("Retry-After", retryAfter)
		}
		w.Header().Set("Cache-Control", noStore)
		http.Error(w, message, code)
	}

	m := mcHostPattern.FindStringSubmatch(r.PathValue("host"))
	if m == nil || len(m[1]) > maxMcHostname {
		fail(http.StatusBadRequest, "invalid server address", "")
		return
	}
	host := strings.ToLower(m[1])
	if m[2] != "" {
		port, err := strconv.ParseUint(m[2], 10, 16)
		if err != nil || port < 1 {
			fail(http.StatusBadRequest, "invalid server address", "")
			return
		}
		host += ":" + strconv.FormatUint(port, 10)
	}

	query := r.URL.Query()
	data := components.McStatusEmbedData{
		Host:    host,
		Bedrock: query.Get("bedrock") == "true",
		Query:   query.Get("query") != "false",
	}
	if port, err := strconv.ParseUint(query.Get("query_port"), 10, 16); err == nil {
		data.QueryPort = int(port)
	}

	endpoint := config.APIURL + "/api/v1/mcstatus/" + url.PathEscape(host)
	if q := data.Options().Encode(); q != "" {
		endpoint += "?" + q
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, endpoint, nil)
	if err != nil {
		fail(http.StatusServiceUnavailable, unavailableMessage, unavailableRetry)
		return
	}
	res, err := mcStatusClient.Do(req)
	if err != nil {
		fail(http.StatusServiceUnavailable, unavailableMessage, unavailableRetry)
		return
	}
	defer res.Body.Close()

	switch {
	case res.StatusCode == http.StatusOK:
		var status apiMcStatus
		if json.NewDecoder(io.LimitReader(res.Body, maxMcStatusBody)).Decode(&status) != nil || status == (apiMcStatus{}) {
			fail(http.StatusServiceUnavailable, unavailableMessage, unavailableRetry)
			return
		}
		motd := strings.ReplaceAll(status.Motd, `\n`, "\n")
		motd = mcColorCode.ReplaceAllString(motd, "")
		for _, line := range strings.Split(motd, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				data.Motd = append(data.Motd, line)
			}
		}
		data.Online = true
		data.Players = status.NumPlayers
		data.Max = status.MaxPlayers
		data.Version = status.Version
		w.Header().Set("Cache-Control", onlineMaxAge)
	case res.StatusCode == http.StatusNotFound:
		if mediaType, _, _ := mime.ParseMediaType(res.Header.Get("Content-Type")); mediaType != problemJSON {
			fail(http.StatusBadGateway, "unexpected status response", "")
			return
		}
		w.Header().Set("Cache-Control", offlineMaxAge)
	case res.StatusCode == http.StatusTooManyRequests:
		fail(http.StatusServiceUnavailable, "status lookups are rate limited", rateLimitedRetry)
		return
	case res.StatusCode == http.StatusInternalServerError:
		fail(http.StatusServiceUnavailable, unavailableMessage, unavailableRetry)
		return
	default:
		fail(http.StatusBadGateway, "unexpected status response", "")
		return
	}
	templ.Handler(components.McStatusEmbedPage(data)).ServeHTTP(w, r)
}
