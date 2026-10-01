package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/components"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/config"
)

const (
	mcStatusTimeout  = 15 * time.Second
	maxMcStatusBody  = 1 << 20
	onlineMaxAge     = "public, max-age=60"
	offlineMaxAge    = "public, max-age=30"
	unavailableRetry = "30"
)

var (
	mcStatusClient = &http.Client{Timeout: mcStatusTimeout}
	mcHexColor     = regexp.MustCompile(`§x(?:§[0-9a-fA-F]){6}`)
	mcColorCode    = regexp.MustCompile(`(?s)§.`)
	mcHostPattern  = regexp.MustCompile(`^(?:[A-Za-z0-9](?:[A-Za-z0-9._-]*[A-Za-z0-9])?|\[[0-9A-Fa-f:.]+\])(?::[0-9]{1,5})?$`)
)

type lookupResult int

const (
	lookupOnline lookupResult = iota
	lookupOffline
	lookupUnavailable
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
	motd = mcColorCode.ReplaceAllString(mcHexColor.ReplaceAllString(motd, ""), "")
	var lines []string
	for _, line := range strings.Split(motd, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// 404 (or the legacy 502) means the server gave no status; anything else is
// the API or the network failing, which must not be cached as "offline".
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
	case http.StatusNotFound, http.StatusBadGateway:
		return status, lookupOffline
	default:
		return status, lookupUnavailable
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, maxMcStatusBody)).Decode(&status); err != nil {
		return status, lookupUnavailable
	}
	return status, lookupOnline
}

func McStatusEmbedHandler(w http.ResponseWriter, r *http.Request) {
	host := r.PathValue("host")
	if !mcHostPattern.MatchString(host) || len(host) > 259 {
		http.Error(w, "invalid server address", http.StatusBadRequest)
		return
	}
	query := r.URL.Query()
	data := components.McStatusEmbedData{
		Host:    host,
		Bedrock: query.Get("bedrock") == "true",
		Query:   query.Get("query") == "true",
	}
	status, result := fetchMcStatus(r, data)
	switch result {
	case lookupUnavailable:
		w.Header().Set("Retry-After", unavailableRetry)
		w.Header().Set("Cache-Control", "no-store")
		http.Error(w, "status lookup unavailable", http.StatusServiceUnavailable)
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
