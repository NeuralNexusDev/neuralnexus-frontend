package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/p0t4t0sandwich/neuralnexus-frontend/config"
)

const testSiteURL = "http://site.test"

type fakeAPI struct {
	server  *httptest.Server
	request *http.Request
}

func newFakeAPI(t *testing.T, status int, body string) *fakeAPI {
	t.Helper()
	contentType := "application/json"
	if status >= http.StatusBadRequest {
		contentType = "application/problem+json"
	}
	return newFakeAPIWithType(t, status, contentType, body)
}

func newFakeAPIWithType(t *testing.T, status int, contentType, body string) *fakeAPI {
	t.Helper()
	f := &fakeAPI{}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.request = r
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}))
	t.Cleanup(f.server.Close)

	apiURL, siteURL := config.APIURL, config.SiteURL
	config.APIURL, config.SiteURL = f.server.URL, testSiteURL
	t.Cleanup(func() { config.APIURL, config.SiteURL = apiURL, siteURL })
	return f
}

func getEmbed(host, rawQuery string) *httptest.ResponseRecorder {
	target := "/mcstatus/" + url.PathEscape(host)
	if rawQuery != "" {
		target += "?" + rawQuery
	}
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.SetPathValue("host", host)
	rec := httptest.NewRecorder()
	McStatusEmbedHandler(rec, req)
	return rec
}

func head(t *testing.T, body string) string {
	t.Helper()
	start, end := strings.Index(body, "<head"), strings.Index(body, "</head>")
	if start < 0 || end < start {
		t.Fatalf("no <head> in response: %s", body)
	}
	return body[start:end]
}

const onlineBody = `{"motd":"§aHello\\n§c§lWorld","num_players":3,"max_players":20,"version":"Paper 1.21"}`

func TestEmbedOnline(t *testing.T) {
	f := newFakeAPI(t, http.StatusOK, onlineBody)
	rec := getEmbed("Play.Example.NET:00080", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=60" {
		t.Errorf("Cache-Control = %q", got)
	}
	if f.request == nil {
		t.Fatal("the API was not called")
	}
	if f.request.URL.Path != "/api/v1/mcstatus/play.example.net:80" {
		t.Errorf("API path = %q", f.request.URL.Path)
	}
	if got := f.request.URL.Query().Get("query"); got != "true" {
		t.Errorf("query = %q, want true by default", got)
	}

	h := head(t, rec.Body.String())
	for _, want := range []string{
		`property="og:title" content="IP: play.example.net:80"`,
		"Hello\nWorld\nPlayers: 3/20\nVersion: Paper 1.21",
		`property="og:url" content="` + testSiteURL + `/mcstatus/play.example.net:80"`,
		`property="og:image" content="` + f.server.URL + `/api/v1/mcstatus/icon/play.example.net:80"`,
		`property="og:image:alt" content="play.example.net:80 server icon"`,
		`property="og:image:width" content="64"`,
		`property="og:image:height" content="64"`,
		`property="og:type" content="website"`,
		`name="twitter:card" content="summary"`,
		`name="robots" content="noindex"`,
		`rel="canonical" href="` + testSiteURL + `/mcstatus/play.example.net:80"`,
		`property="og:site_name"`,
		`name="theme-color"`,
	} {
		if !strings.Contains(h, want) {
			t.Errorf("head is missing %q:\n%s", want, h)
		}
	}
	if strings.Contains(rec.Body.String(), "§") {
		t.Error("colour codes leaked into the page")
	}
}

func TestEmbedForwardsOptions(t *testing.T) {
	cases := []struct {
		name, query string
		wantAPI     url.Values
		wantURL     string
	}{
		{"query off", "query=false", url.Values{}, "/mcstatus/a.com?query=false"},
		{"query port", "query_port=25575", url.Values{"query": {"true"}, "query_port": {"25575"}}, "/mcstatus/a.com?query_port=25575"},
		{"query port with leading zeros", "query_port=0025575", url.Values{"query": {"true"}, "query_port": {"25575"}}, "/mcstatus/a.com?query_port=25575"},
		{"invalid query port is dropped", "query_port=99999", url.Values{"query": {"true"}}, "/mcstatus/a.com"},
		{"query port is ignored when the query is off", "query=false&query_port=25575", url.Values{}, "/mcstatus/a.com?query=false"},
		{"bedrock", "bedrock=true", url.Values{"bedrock": {"true"}}, "/mcstatus/a.com?bedrock=true"},
		{"bedrock ignores the query and its port", "bedrock=true&query_port=25575", url.Values{"bedrock": {"true"}}, "/mcstatus/a.com?bedrock=true"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeAPI(t, http.StatusOK, onlineBody)
			rec := getEmbed("a.com", tc.query)
			if f.request == nil {
				t.Fatal("the API was not called")
			}
			if got := f.request.URL.Query(); got.Encode() != tc.wantAPI.Encode() {
				t.Errorf("API query = %q, want %q", got.Encode(), tc.wantAPI.Encode())
			}
			if want := `property="og:url" content="` + testSiteURL + tc.wantURL + `"`; !strings.Contains(head(t, rec.Body.String()), want) {
				t.Errorf("og:url should be %q:\n%s", tc.wantURL, head(t, rec.Body.String()))
			}
		})
	}
}

func TestEmbedBedrockHasNoImage(t *testing.T) {
	newFakeAPI(t, http.StatusOK, onlineBody)
	rec := getEmbed("a.com", "bedrock=true")
	if strings.Contains(head(t, rec.Body.String()), "og:image") {
		t.Error("Bedrock servers have no icon, og:image should be absent")
	}
}

func TestEmbedOffline(t *testing.T) {
	newFakeAPI(t, http.StatusNotFound, `{"status":404}`)
	rec := getEmbed("a.com", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=30" {
		t.Errorf("Cache-Control = %q", got)
	}
	h := head(t, rec.Body.String())
	if !strings.Contains(h, "Server offline or unreachable") {
		t.Errorf("missing offline description:\n%s", h)
	}
	if strings.Contains(h, "og:image") {
		t.Error("offline servers should not advertise an icon")
	}
}

func TestEmbedLookupFailures(t *testing.T) {
	const (
		unavailable = "status lookup unavailable"
		rateLimited = "status lookups are rate limited"
		unexpected  = "unexpected status response"
	)
	cases := []struct {
		name       string
		apiStatus  int
		body       string
		wantStatus int
		wantRetry  string
		wantBody   string
	}{
		{"rate limited", http.StatusTooManyRequests, `{"detail":"boom"}`, http.StatusServiceUnavailable, "60", rateLimited},
		{"api error", http.StatusInternalServerError, `{"detail":"boom"}`, http.StatusServiceUnavailable, "30", unavailable},
		{"empty object", http.StatusOK, `{}`, http.StatusServiceUnavailable, "30", unavailable},
		{"null", http.StatusOK, `null`, http.StatusServiceUnavailable, "30", unavailable},
		{"not json", http.StatusOK, `<html>boom`, http.StatusServiceUnavailable, "30", unavailable},
		{"mistyped field", http.StatusOK, `{"motd":"Hi","num_players":"x"}`, http.StatusServiceUnavailable, "30", unavailable},
		{"oversized body", http.StatusOK, `{"motd":"` + strings.Repeat("a", 1<<20+1) + `"}`, http.StatusServiceUnavailable, "30", unavailable},
		{"undocumented status", http.StatusTeapot, `{"detail":"boom"}`, http.StatusBadGateway, "", unexpected},
		{"unauthorized", http.StatusUnauthorized, `{"detail":"boom"}`, http.StatusBadGateway, "", unexpected},
		{"bad gateway", http.StatusBadGateway, `{"detail":"boom"}`, http.StatusBadGateway, "", unexpected},
		{"service unavailable", http.StatusServiceUnavailable, `{"detail":"boom"}`, http.StatusBadGateway, "", unexpected},
		{"gateway timeout", http.StatusGatewayTimeout, `{"detail":"boom"}`, http.StatusBadGateway, "", unexpected},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			newFakeAPI(t, tc.apiStatus, tc.body)
			rec := getEmbed("a.com", "")
			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if got := rec.Header().Get("Retry-After"); got != tc.wantRetry {
				t.Errorf("Retry-After = %q, want %q", got, tc.wantRetry)
			}
			if got := rec.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q, want %q", got, "no-store")
			}
			if got := rec.Body.String(); got != tc.wantBody+"\n" {
				t.Errorf("body = %q, want only %q", got, tc.wantBody)
			}
		})
	}
}

func TestEmbedAPIUnreachable(t *testing.T) {
	f := newFakeAPI(t, http.StatusOK, onlineBody)
	f.server.Close()
	rec := getEmbed("a.com", "")
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") != "30" {
		t.Errorf("status = %d, Retry-After = %q", rec.Code, rec.Header().Get("Retry-After"))
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want %q", got, "no-store")
	}
}

func TestEmbedRejectsInvalidHosts(t *testing.T) {
	f := newFakeAPI(t, http.StatusOK, onlineBody)
	cases := []struct{ name, host string }{
		{"empty label", "a..b"},
		{"leading dot", ".a.com"},
		{"illegal character", "a_b$c"},
		{"space", "bad host"},
		{"path", "a.com/b"},
		{"port zero", "a.com:0"},
		{"port too large", "a.com:65536"},
		{"port too long", "a.com:123456"},
		{"trailing colon", "a.com:"},
		{"label over 63", strings.Repeat("a", 64) + ".com"},
		{"name over 253", strings.Repeat(strings.Repeat("a", 60)+".", 5) + "com"},
		{"ipv6", "::1"},
		{"bracketed ipv6", "[::1]"},
		{"bracketed ipv6 with a port", "[2001:db8::1]:25565"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f.request = nil
			rec := getEmbed(tc.host, "")
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", rec.Code)
			}
			if got := rec.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q, want %q", got, "no-store")
			}
			if f.request != nil {
				t.Error("an invalid host must not reach the API")
			}
		})
	}
}

func TestNormalizeMcHost(t *testing.T) {
	cases := map[string]string{
		"Example.COM":       "example.com",
		"example.com:25565": "example.com:25565",
		"example.com:00080": "example.com:80",
		"a_b.example.com":   "a_b.example.com",
	}
	for raw, want := range cases {
		if got, ok := normalizeMcHost(raw); !ok || got != want {
			t.Errorf("normalizeMcHost(%q) = %q, %v; want %q", raw, got, ok, want)
		}
	}
}

func TestEmbedDescriptionIsTruncatedAndEscaped(t *testing.T) {
	motd := strings.Repeat("é", 300)
	version := strings.Repeat("v", 100)
	newFakeAPI(t, http.StatusOK, fmt.Sprintf(`{"motd":%q,"num_players":1,"max_players":2,"version":%q}`, motd, version))
	h := head(t, getEmbed("a.com", "").Body.String())

	if !strings.Contains(h, strings.Repeat("é", 200)+"…\nPlayers: 1/2\nVersion: "+strings.Repeat("v", 64)+"…") {
		t.Errorf("MOTD and version should be truncated:\n%s", h)
	}
	if strings.Contains(h, strings.Repeat("é", 201)) {
		t.Error("MOTD was not truncated at the limit")
	}
}

func TestEmbedKeepsTextAtTheLimit(t *testing.T) {
	motd := strings.Repeat("é", 200)
	version := strings.Repeat("v", 64)
	newFakeAPI(t, http.StatusOK, fmt.Sprintf(`{"motd":%q,"num_players":1,"max_players":2,"version":%q}`, motd, version))
	h := head(t, getEmbed("a.com", "").Body.String())
	if strings.Contains(h, "…") {
		t.Errorf("text exactly at the limit must not be truncated:\n%s", h)
	}
	if !strings.Contains(h, motd+"\nPlayers: 1/2\nVersion: "+version) {
		t.Errorf("full MOTD and version expected:\n%s", h)
	}
}

func TestEmbedEscapesServerText(t *testing.T) {
	newFakeAPI(t, http.StatusOK, `{"motd":"<script>alert(1)</script>\"><img src=x>","num_players":1,"max_players":2,"version":"<b>1</b>"}`)
	body := getEmbed("a.com", "").Body.String()
	if !strings.Contains(body, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Error("the MOTD should still appear, escaped")
	}
	for _, bad := range []string{"<script>alert", "<img src=x>", "<b>1</b>"} {
		if strings.Contains(body, bad) {
			t.Errorf("unescaped server text %q in response", bad)
		}
	}
}

func TestEmbedOmitsEmptyVersion(t *testing.T) {
	newFakeAPI(t, http.StatusOK, `{"motd":"Hi","num_players":0,"max_players":5,"version":""}`)
	h := head(t, getEmbed("a.com", "").Body.String())
	if strings.Contains(h, "Version:") {
		t.Errorf("an empty version should not render a Version line:\n%s", h)
	}
}

func TestMotdLines(t *testing.T) {
	cases := map[string][]string{
		`§aHello\n§c§lWorld`:      {"Hello", "World"},
		`§x§f§f§0§0§0§0Red`:       {"Red"},
		`  padded  \n\n  lines  `: {"padded", "lines"},
		`no codes`:                {"no codes"},
		"already\nsplit":          {"already", "split"},
	}
	for in, want := range cases {
		got := motdLines(in)
		if strings.Join(got, "|") != strings.Join(want, "|") {
			t.Errorf("motdLines(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEmbedAcceptsABodyAtTheLimit(t *testing.T) {
	const prefix, suffix = `{"motd":"`, `","num_players":1,"max_players":2,"version":"v"}`
	body := prefix + strings.Repeat("a", 1<<20-len(prefix)-len(suffix)) + suffix
	if len(body) != 1<<20 {
		t.Fatalf("test body is %d bytes, want exactly 1<<20", len(body))
	}
	newFakeAPI(t, http.StatusOK, body)
	if rec := getEmbed("a.com", ""); rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "public, max-age=60" {
		t.Errorf("status = %d, Cache-Control = %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
}

func TestEmbedTreatsOnlyProblemJSON404AsOffline(t *testing.T) {
	cases := []struct {
		name, contentType string
		wantStatus        int
		wantCache         string
	}{
		{"problem+json", "application/problem+json", http.StatusOK, "public, max-age=30"},
		{"problem+json with a charset", "application/problem+json" + "; charset=utf-8", http.StatusOK, "public, max-age=30"},
		{"plain json", "application/json", http.StatusBadGateway, "no-store"},
		{"html from a proxy", "text/html", http.StatusBadGateway, "no-store"},
		{"no content type", "", http.StatusBadGateway, "no-store"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			newFakeAPIWithType(t, http.StatusNotFound, tc.contentType, `{"status":404}`)
			rec := getEmbed("a.com", "")
			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if got := rec.Header().Get("Cache-Control"); got != tc.wantCache {
				t.Errorf("Cache-Control = %q, want %q", got, tc.wantCache)
			}
		})
	}
}

func TestEmbedRouteWiring(t *testing.T) {
	f := newFakeAPI(t, http.StatusOK, onlineBody)
	router := NewWebServer("", false).Setup()

	get := httptest.NewRecorder()
	router.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/mcstatus/Play.Example.com:25570?query=false", nil))
	if get.Code != http.StatusOK || !strings.Contains(get.Body.String(), `property="og:title" content="IP: play.example.com:25570"`) {
		t.Fatalf("GET /mcstatus/{host} did not reach the handler: %d %s", get.Code, get.Body.String())
	}
	if f.request == nil || f.request.URL.Path != "/api/v1/mcstatus/play.example.com:25570" {
		t.Errorf("the host path value did not reach the API: %v", f.request)
	}

	f.request = nil
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(method, "/mcstatus/a.com", nil))
		if strings.Contains(rec.Body.String(), "IP: a.com") {
			t.Errorf("%s was served by the preview handler", method)
		}
	}
	if f.request != nil {
		t.Error("non-GET requests must not reach the API")
	}
}
