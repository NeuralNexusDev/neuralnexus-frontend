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

	"github.com/a-h/templ"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/components"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/config"
)

const (
	maxMcStatusBody = 1 << 20
	noStore         = "no-store"
	onlineMaxAge    = "public, max-age=60"
	offlineMaxAge   = "public, max-age=30"
	maxMcHostInput  = 260
	problemJSON     = "application/problem+json"
)

var mcColorCode = regexp.MustCompile(`(?s)§.`)

type apiMcStatus struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Motd       string `json:"motd"`
	NumPlayers int    `json:"num_players"`
	MaxPlayers int    `json:"max_players"`
	Version    string `json:"version"`
}

func decodeMcStatus(body io.Reader) (apiMcStatus, bool) {
	var status apiMcStatus
	if json.NewDecoder(io.LimitReader(body, maxMcStatusBody)).Decode(&status) != nil {
		return status, false
	}
	if status.Host == "" {
		return status, false
	}
	if status.Port == 0 {
		return status, false
	}
	return status, true
}

// McStatusPageHandler serves the status checker, with link-preview tags when the server lookup succeeds or reports offline.
func McStatusPageHandler(w http.ResponseWriter, r *http.Request) {
	rawHost := r.PathValue("host")
	if rawHost == "" {
		templ.Handler(components.McStatusPage()).ServeHTTP(w, r)
		return
	}
	bare := func() {
		w.Header().Set("Cache-Control", noStore)
		templ.Handler(components.McStatusPage()).ServeHTTP(w, r)
	}
	if len(rawHost) > maxMcHostInput {
		bare()
		return
	}
	if rawHost == "." || rawHost == ".." {
		bare()
		return
	}

	query := r.URL.Query()
	data := components.McStatusEmbedData{
		Bedrock: query.Get("bedrock") == "true",
		Query:   query.Get("query") != "false",
	}
	if port, err := strconv.ParseUint(query.Get("query_port"), 10, 16); err == nil {
		data.QueryPort = int(port)
	}

	endpoint := config.APIURL + "/api/v1/mcstatus/" + url.PathEscape(rawHost)
	if q := data.Options().Encode(); q != "" {
		endpoint += "?" + q
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, endpoint, nil)
	if err != nil {
		bare()
		return
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		bare()
		return
	}
	defer res.Body.Close()

	switch {
	case res.StatusCode == http.StatusOK:
		status, ok := decodeMcStatus(res.Body)
		if !ok {
			bare()
			return
		}
		motd := strings.ReplaceAll(status.Motd, `\n`, "\n")
		motd = mcColorCode.ReplaceAllString(motd, "")
		for _, line := range strings.Split(motd, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				data.Motd = append(data.Motd, line)
			}
		}
		data.Host = status.Host
		data.Port = status.Port
		data.Online = true
		data.Players = status.NumPlayers
		data.Max = status.MaxPlayers
		data.Version = status.Version
		w.Header().Set("Cache-Control", onlineMaxAge)
	case res.StatusCode == http.StatusNotFound:
		if mediaType, _, _ := mime.ParseMediaType(res.Header.Get("Content-Type")); mediaType != problemJSON {
			bare()
			return
		}
		status, ok := decodeMcStatus(res.Body)
		if !ok {
			bare()
			return
		}
		data.Host = status.Host
		data.Port = status.Port
		w.Header().Set("Cache-Control", offlineMaxAge)
	default:
		bare()
		return
	}
	templ.Handler(components.McStatusEmbedPage(data)).ServeHTTP(w, r)
}
