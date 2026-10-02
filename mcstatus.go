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
	maxMcHostname   = 253
	problemJSON     = "application/problem+json"
)

var (
	mcColorCode   = regexp.MustCompile(`(?s)§.`)
	mcHostPattern = regexp.MustCompile(`^(?P<host>[A-Za-z0-9_](?:[A-Za-z0-9_-]{0,61}[A-Za-z0-9_])?(?:\.[A-Za-z0-9_](?:[A-Za-z0-9_-]{0,61}[A-Za-z0-9_])?)*)(?::(?P<port>[0-9]{1,5}))?$`)
)

type apiMcStatus struct {
	Motd       string `json:"motd"`
	NumPlayers int    `json:"num_players"`
	MaxPlayers int    `json:"max_players"`
	Version    string `json:"version"`
}

// McStatusPageHandler serves the status checker, with link-preview tags when the server lookup succeeds or reports offline.
func McStatusPageHandler(w http.ResponseWriter, r *http.Request) {
	rawHost := r.PathValue("host")
	if rawHost == "" {
		templ.Handler(components.McStatusPage()).ServeHTTP(w, r)
		return
	}
	bare := func(status int) {
		w.Header().Set("Cache-Control", noStore)
		templ.Handler(components.McStatusPage(), templ.WithStatus(status)).ServeHTTP(w, r)
	}

	m := mcHostPattern.FindStringSubmatch(rawHost)
	if m == nil || len(m[1]) > maxMcHostname {
		bare(http.StatusBadRequest)
		return
	}
	host := strings.ToLower(m[1])
	if m[2] != "" {
		port, err := strconv.ParseUint(m[2], 10, 16)
		if err != nil || port < 1 {
			bare(http.StatusBadRequest)
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
		bare(http.StatusOK)
		return
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		bare(http.StatusOK)
		return
	}
	defer res.Body.Close()

	switch {
	case res.StatusCode == http.StatusOK:
		var status apiMcStatus
		if json.NewDecoder(io.LimitReader(res.Body, maxMcStatusBody)).Decode(&status) != nil || status == (apiMcStatus{}) {
			bare(http.StatusOK)
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
			bare(http.StatusOK)
			return
		}
		w.Header().Set("Cache-Control", offlineMaxAge)
	default:
		bare(http.StatusOK)
		return
	}
	templ.Handler(components.McStatusEmbedPage(data)).ServeHTTP(w, r)
}
