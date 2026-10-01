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
	maxMcHostLength = 260
	mcStatusTimeout = 15 * time.Second
	maxMcStatusBody = 1 << 20
)

var (
	mcStatusClient = &http.Client{Timeout: mcStatusTimeout}
	mcHexColor     = regexp.MustCompile(`§x(?:§[0-9a-fA-F]){6}`)
	mcColorCode    = regexp.MustCompile(`(?s)§.`)
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

func fetchMcStatus(r *http.Request, data components.McStatusEmbedData) (apiMcStatus, bool) {
	var status apiMcStatus
	endpoint := config.APIURL + "/api/v1/mcstatus/" + url.PathEscape(data.Host)
	if q := data.Options().Encode(); q != "" {
		endpoint += "?" + q
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, endpoint, nil)
	if err != nil {
		return status, false
	}
	res, err := mcStatusClient.Do(req)
	if err != nil {
		return status, false
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return status, false
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, maxMcStatusBody)).Decode(&status); err != nil {
		return status, false
	}
	return status, true
}

// McStatusEmbedHandler serves the link-preview page for /mcstatus/{host}.
func McStatusEmbedHandler(w http.ResponseWriter, r *http.Request) {
	host := strings.TrimSpace(r.PathValue("host"))
	if host == "" || host == "." || host == ".." || len(host) > maxMcHostLength {
		http.Error(w, "invalid server address", http.StatusBadRequest)
		return
	}
	query := r.URL.Query()
	data := components.McStatusEmbedData{
		Host:    host,
		Bedrock: query.Get("bedrock") == "true",
		Query:   query.Get("query") == "true",
	}
	if status, ok := fetchMcStatus(r, data); ok {
		data.Online = true
		data.Motd = motdLines(status.Motd)
		data.Players = status.NumPlayers
		data.Max = status.MaxPlayers
		data.Version = status.Version
	}
	templ.Handler(components.McStatusEmbedPage(data)).ServeHTTP(w, r)
}
