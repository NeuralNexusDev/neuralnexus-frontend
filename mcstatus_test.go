package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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
	target := "/project/mc-status/" + url.PathEscape(host)
	if rawQuery != "" {
		target += "?" + rawQuery
	}
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.SetPathValue("host", host)
	rec := httptest.NewRecorder()
	McStatusPageHandler(rec, req)
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

const bedrockBody = `{"host":"a.com","port":19132,"motd":"Hi","num_players":1,"max_players":2,"version":"v"}`

const onlineBody = `{"host":"a.com","port":25565,"motd":"§aHello\\n§c§lWorld","num_players":3,"max_players":20,"version":"Paper 1.21"}`

func TestEmbedOnline(t *testing.T) {
	f := newFakeAPI(t, http.StatusOK, `{"host":"play.example.net","port":80,"motd":"§aHello\\n§c§lWorld","num_players":3,"max_players":20,"version":"Paper 1.21"}`)
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
	if f.request.URL.Path != "/api/v1/mcstatus/Play.Example.NET:00080" {
		t.Errorf("API path = %q", f.request.URL.Path)
	}
	if got := f.request.URL.Query().Get("query"); got != "true" {
		t.Errorf("query = %q, want true by default", got)
	}

	h := head(t, rec.Body.String())
	for _, want := range []string{
		"<title>play.example.net:80</title>",
		`property="og:title" content="play.example.net:80"`,
		"Hello\nWorld\nPlayers: 3/20\nVersion: Paper 1.21",
		`property="og:url" content="` + testSiteURL + `/project/mc-status/play.example.net:80"`,
		`property="og:image" content="` + f.server.URL + `/api/v1/mcstatus/icon/play.example.net:80"`,
		`property="og:image:alt" content="play.example.net:80 server icon"`,
		`property="og:image:width" content="64"`,
		`property="og:image:height" content="64"`,
		`property="og:type" content="website"`,
		`name="twitter:card" content="summary"`,
		`name="robots" content="noindex"`,
		`rel="canonical" href="` + testSiteURL + `/project/mc-status/play.example.net:80"`,
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

func TestEmbedIPv6Host(t *testing.T) {
	cases := []struct {
		name, query, body, wantURI, wantHost, wantSuffix string
	}{
		{"java", "", `{"host":"2001:db8::1","port":25565,"num_players":1}`, "/api/v1/mcstatus/%5B2001:DB8::1%5D:25565?query=true", "[2001:db8::1]", ""},
		{"java with a port", "", `{"host":"2001:db8::1","port":25566,"num_players":1}`, "/api/v1/mcstatus/%5B2001:DB8::1%5D:25565?query=true", "[2001:db8::1]:25566", ""},
		{"bedrock", "bedrock=true", `{"host":"2001:db8::1","port":19132,"num_players":1}`, "/api/v1/mcstatus/%5B2001:DB8::1%5D:25565?bedrock=true", "[2001:db8::1]", "?bedrock=true"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeAPI(t, http.StatusOK, tc.body)
			rec := getEmbed("[2001:DB8::1]:25565", tc.query)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d", rec.Code)
			}
			if f.request == nil || f.request.RequestURI != tc.wantURI {
				t.Errorf("API request = %v, want %s", f.request, tc.wantURI)
			}
			path := strings.NewReplacer("[", "%5B", "]", "%5D").Replace(tc.wantHost)
			pageURL := testSiteURL + "/project/mc-status/" + path + tc.wantSuffix
			h := head(t, rec.Body.String())
			for _, want := range []string{
				"<title>" + tc.wantHost + "</title>",
				`property="og:title" content="` + tc.wantHost + `"`,
				`property="og:url" content="` + pageURL + `"`,
				`rel="canonical" href="` + pageURL + `"`,
				`property="og:image" content="` + f.server.URL + `/api/v1/mcstatus/icon/` + path + tc.wantSuffix + `"`,
				`property="og:image:alt" content="` + tc.wantHost + ` server icon"`,
			} {
				if !strings.Contains(h, want) {
					t.Errorf("head is missing %q:\n%s", want, h)
				}
			}
		})
	}
}

func TestEmbedShowsThePortOnlyWhenItIsNotTheDefault(t *testing.T) {
	cases := []struct {
		name, query, host string
		port              int
		wantHost          string
	}{
		{"java default", "", "a.com", 25565, "a.com"},
		{"java other", "", "a.com", 25566, "a.com:25566"},
		{"java on the Bedrock default", "", "a.com", 19132, "a.com:19132"},
		{"bedrock default", "bedrock=true", "a.com", 19132, "a.com"},
		{"bedrock other", "bedrock=true", "a.com", 19133, "a.com:19133"},
		{"bedrock on the Java default", "bedrock=true", "a.com", 25565, "a.com:25565"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			newFakeAPI(t, http.StatusOK, fmt.Sprintf(`{"host":%q,"port":%d,"num_players":1}`, tc.host, tc.port))
			suffix := ""
			if tc.query != "" {
				suffix = "?" + tc.query
			}
			h := head(t, getEmbed(tc.host, tc.query).Body.String())
			for _, want := range []string{
				"<title>" + tc.wantHost + "</title>",
				`property="og:url" content="` + testSiteURL + "/project/mc-status/" + tc.wantHost + suffix + `"`,
				`rel="canonical" href="` + testSiteURL + "/project/mc-status/" + tc.wantHost + suffix + `"`,
				`/api/v1/mcstatus/icon/` + tc.wantHost + suffix + `"`,
			} {
				if !strings.Contains(h, want) {
					t.Errorf("head is missing %q:\n%s", want, h)
				}
			}
		})
	}
}

func TestEmbedForwardsOptions(t *testing.T) {
	cases := []struct {
		name, query string
		wantAPI     url.Values
		wantURL     string
	}{
		{"query off", "query=false", url.Values{}, "/project/mc-status/a.com?query=false"},
		{"query port", "query_port=25575", url.Values{"query": {"true"}, "query_port": {"25575"}}, "/project/mc-status/a.com?query_port=25575"},
		{"highest query port", "query_port=65535", url.Values{"query": {"true"}, "query_port": {"65535"}}, "/project/mc-status/a.com?query_port=65535"},
		{"query port above the highest", "query_port=65536", url.Values{"query": {"true"}}, "/project/mc-status/a.com"},
		{"query port with leading zeros", "query_port=0025575", url.Values{"query": {"true"}, "query_port": {"25575"}}, "/project/mc-status/a.com?query_port=25575"},
		{"signed query port is dropped", "query_port=%2B80", url.Values{"query": {"true"}}, "/project/mc-status/a.com"},
		{"invalid query port is dropped", "query_port=99999", url.Values{"query": {"true"}}, "/project/mc-status/a.com"},
		{"query port is ignored when the query is off", "query=false&query_port=25575", url.Values{}, "/project/mc-status/a.com?query=false"},
		{"bedrock", "bedrock=true", url.Values{"bedrock": {"true"}}, "/project/mc-status/a.com?bedrock=true"},
		{"bedrock ignores the query and its port", "bedrock=true&query_port=25575", url.Values{"bedrock": {"true"}}, "/project/mc-status/a.com?bedrock=true"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := onlineBody
			if strings.HasPrefix(tc.query, "bedrock=true") {
				body = bedrockBody
			}
			f := newFakeAPI(t, http.StatusOK, body)
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
			if want := `rel="canonical" href="` + testSiteURL + tc.wantURL + `"`; !strings.Contains(head(t, rec.Body.String()), want) {
				t.Errorf("canonical should be %q:\n%s", tc.wantURL, head(t, rec.Body.String()))
			}
		})
	}
}

func TestEmbedBedrockHasAnImage(t *testing.T) {
	f := newFakeAPI(t, http.StatusOK, bedrockBody)
	rec := getEmbed("a.com", "bedrock=true")
	want := `property="og:image" content="` + f.server.URL + `/api/v1/mcstatus/icon/a.com?bedrock=true"`
	if !strings.Contains(head(t, rec.Body.String()), want) {
		t.Errorf("missing %s", want)
	}
}

func TestEmbedOffline(t *testing.T) {
	f := newFakeAPI(t, http.StatusNotFound, `{"host":"a.com","port":25565,"status":404}`)
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
	if !strings.Contains(h, "<title>a.com</title>") {
		t.Errorf("missing the host as the title:\n%s", h)
	}
	want := `property="og:image" content="` + f.server.URL + `/api/v1/mcstatus/icon/a.com"`
	if !strings.Contains(h, want) {
		t.Errorf("missing %s", want)
	}
}

func assertBarePage(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want %q", got, "no-store")
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="mc-status-host"`) {
		t.Error("the checker form should still be served")
	}
	if h := head(t, body); strings.Contains(h, "og:") || !strings.Contains(h, "<title>NeuralNexus</title>") {
		t.Errorf("the generic head should carry no preview tags:\n%s", h)
	}
}

func TestEmbedLookupFailures(t *testing.T) {
	cases := []struct {
		name      string
		apiStatus int
		body      string
	}{
		{"rate limited", http.StatusTooManyRequests, `{"detail":"boom"}`},
		{"api error", http.StatusInternalServerError, `{"detail":"boom"}`},
		{"empty object", http.StatusOK, `{}`},
		{"null", http.StatusOK, `null`},
		{"not json", http.StatusOK, `<html>boom`},
		{"mistyped field", http.StatusOK, `{"motd":"Hi","num_players":"x"}`},
		{"oversized body", http.StatusOK, bodyOfSize(1<<20 + 1)},
		{"undocumented status", http.StatusTeapot, `{"detail":"boom"}`},
		{"unauthorized", http.StatusUnauthorized, `{"detail":"boom"}`},
		{"bad gateway", http.StatusBadGateway, `{"detail":"boom"}`},
		{"service unavailable", http.StatusServiceUnavailable, `{"detail":"boom"}`},
		{"gateway timeout", http.StatusGatewayTimeout, `{"detail":"boom"}`},
		{"rejected host", http.StatusBadRequest, `{"detail":"The host must be a domain name, an IPv4 address or an IPv6 address, optionally followed by a port."}`},
		{"online without a host", http.StatusOK, `{"port":25565,"motd":"Hi","num_players":1,"max_players":2}`},
		{"online without a port", http.StatusOK, `{"host":"a.com","motd":"Hi","num_players":1,"max_players":2}`},
		{"offline without a host", http.StatusNotFound, `{"port":25565,"status":404}`},
		{"offline without a port", http.StatusNotFound, `{"host":"a.com","status":404}`},
		{"offline without a body", http.StatusNotFound, ``},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			newFakeAPI(t, tc.apiStatus, tc.body)
			assertBarePage(t, getEmbed("a.com", ""))
		})
	}
}

func TestEmbedAPIUnreachable(t *testing.T) {
	f := newFakeAPI(t, http.StatusOK, onlineBody)
	f.server.Close()
	assertBarePage(t, getEmbed("a.com", ""))
}

func TestEmbedCancelsTheUpstreamLookupWhenTheClientGoesAway(t *testing.T) {
	arrived := make(chan struct{})
	released := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(arrived)
		<-r.Context().Done()
		close(released)
	}))
	t.Cleanup(server.Close)
	apiURL := config.APIURL
	config.APIURL = server.URL
	t.Cleanup(func() { config.APIURL = apiURL })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/project/mc-status/a.com", nil).WithContext(ctx)
	req.SetPathValue("host", "a.com")
	done := make(chan struct{})
	go func() {
		McStatusPageHandler(httptest.NewRecorder(), req)
		close(done)
	}()

	select {
	case <-arrived:
	case <-time.After(5 * time.Second):
		t.Fatal("the lookup never reached the API")
	}
	cancel()
	for _, ch := range []chan struct{}{released, done} {
		select {
		case <-ch:
		case <-time.After(5 * time.Second):
			t.Fatal("cancelling the client request did not cancel the upstream lookup")
		}
	}
}

func TestEmbedClosesTheUpstreamBody(t *testing.T) {
	cases := []struct {
		name        string
		status      int
		contentType string
		body        string
	}{
		{"online", http.StatusOK, "application/json", onlineBody + strings.Repeat(" ", 1<<16)},
		{"offline", http.StatusNotFound, "application/problem+json", `{"host":"a.com","port":25565,"status":404}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var opened, open atomic.Int32
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
				switch state {
				case http.StateNew:
					opened.Add(1)
					open.Add(1)
				case http.StateClosed:
					open.Add(-1)
				}
			}
			server.Start()
			t.Cleanup(server.Close)
			apiURL := config.APIURL
			config.APIURL = server.URL
			t.Cleanup(func() { config.APIURL = apiURL })

			rec := getEmbed("a.com", "")
			http.DefaultTransport.(*http.Transport).CloseIdleConnections()
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			if n := opened.Load(); n != 1 {
				t.Fatalf("upstream connections opened = %d, want 1", n)
			}
			deadline := time.Now().Add(2 * time.Second)
			for open.Load() != 0 && time.Now().Before(deadline) {
				time.Sleep(10 * time.Millisecond)
			}
			if n := open.Load(); n != 0 {
				t.Errorf("upstream connection still open after handler returned: %d", n)
			}
		})
	}
}

func TestEmbedRejectsAnInvalidAPIURL(t *testing.T) {
	apiURL := config.APIURL
	config.APIURL = "http://a b"
	t.Cleanup(func() { config.APIURL = apiURL })
	assertBarePage(t, getEmbed("a.com", ""))
}

func TestEmbedAcceptsAHostAtTheLimit(t *testing.T) {
	f := newFakeAPI(t, http.StatusOK, onlineBody)
	if rec := getEmbed(strings.Repeat("a", 260), ""); rec.Code != http.StatusOK || f.request == nil {
		t.Errorf("status = %d, API called = %v", rec.Code, f.request != nil)
	}
}

func TestEmbedGuardsTheRequestPath(t *testing.T) {
	f := newFakeAPI(t, http.StatusOK, onlineBody)
	cases := []struct{ name, host string }{
		{"dot", "."},
		{"dot dot", ".."},
		{"over the limit", strings.Repeat("a", 261)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f.request = nil
			assertBarePage(t, getEmbed(tc.host, ""))
			if f.request != nil {
				t.Error("the host must not reach the API")
			}
		})
	}
}

func TestEmbedPassesTheHostThroughToTheAPI(t *testing.T) {
	cases := map[string]string{
		"Example.COM":         "Example.COM",
		"example.com:00080":   "example.com:00080",
		"[2001:DB8::1]:25565": "%5B2001:DB8::1%5D:25565",
		"2001:db8::1":         "2001:db8::1",
		"bad host":            "bad%20host",
		"a.com/b":             "a.com%2Fb",
		"a.com?x=1":           "a.com%3Fx=1",
		"a.com#f":             "a.com%23f",
		"%41":                 "%2541",
	}
	for raw, want := range cases {
		t.Run(raw, func(t *testing.T) {
			f := newFakeAPI(t, http.StatusOK, onlineBody)
			getEmbed(raw, "")
			if f.request == nil || f.request.RequestURI != "/api/v1/mcstatus/"+want+"?query=true" {
				t.Errorf("API request = %v, want /api/v1/mcstatus/%s?query=true", f.request, want)
			}
		})
	}
}

func TestEmbedServesTheBareCheckerWhenTheAPIRejectsTheHost(t *testing.T) {
	f := newFakeAPI(t, http.StatusBadRequest, `{"detail":"The host must be a domain name, an IPv4 address or an IPv6 address, optionally followed by a port."}`)
	assertBarePage(t, getEmbed("localhost", ""))
	if f.request == nil || f.request.URL.Path != "/api/v1/mcstatus/localhost" {
		t.Errorf("API request = %v", f.request)
	}
}

func TestEmbedDescriptionIsTruncatedAndEscaped(t *testing.T) {
	motd := strings.Repeat("é", 300)
	version := strings.Repeat("v", 100)
	newFakeAPI(t, http.StatusOK, fmt.Sprintf(`{"host":"a.com","port":25565,"motd":%q,"num_players":1,"max_players":2,"version":%q}`, motd, version))
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
	newFakeAPI(t, http.StatusOK, fmt.Sprintf(`{"host":"a.com","port":25565,"motd":%q,"num_players":1,"max_players":2,"version":%q}`, motd, version))
	h := head(t, getEmbed("a.com", "").Body.String())
	if strings.Contains(h, "…") {
		t.Errorf("text exactly at the limit must not be truncated:\n%s", h)
	}
	if !strings.Contains(h, motd+"\nPlayers: 1/2\nVersion: "+version) {
		t.Errorf("full MOTD and version expected:\n%s", h)
	}
}

func TestEmbedEscapesServerText(t *testing.T) {
	newFakeAPI(t, http.StatusOK, `{"host":"a.com","port":25565,"motd":"<script>alert(1)</script>\"><img src=x>","num_players":1,"max_players":2,"version":"<b>1</b>"}`)
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
	newFakeAPI(t, http.StatusOK, `{"host":"a.com","port":25565,"motd":"Hi","num_players":0,"max_players":5,"version":""}`)
	h := head(t, getEmbed("a.com", "").Body.String())
	if strings.Contains(h, "Version:") {
		t.Errorf("an empty version should not render a Version line:\n%s", h)
	}
}

func TestEmbedMotdLines(t *testing.T) {
	cases := []struct{ name, motd, want string }{
		{"colour codes and a line break", `§aHello\n§c§lWorld`, "Hello\nWorld\n"},
		{"hex colour", `§x§f§f§0§0§0§0Red`, "Red\n"},
		{"padding and blank lines", `  padded  \n\n  lines  `, "padded\nlines\n"},
		{"no codes", `no codes`, "no codes\n"},
		{"a real newline", "already\nsplit", "already\nsplit\n"},
		{"nothing but a code", `§a`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			newFakeAPI(t, http.StatusOK, fmt.Sprintf(`{"host":"a.com","port":25565,"motd":%q,"num_players":1,"max_players":2,"version":"v"}`, tc.motd))
			h := head(t, getEmbed("a.com", "").Body.String())
			if want := tc.want + "Players: 1/2\nVersion: v"; !strings.Contains(h, `content="`+want+`"`) {
				t.Errorf("description should be %q:\n%s", want, h)
			}
		})
	}
}

func bodyOfSize(size int) string {
	const prefix, suffix = `{"host":"a.com","port":25565,"motd":"`, `","num_players":1,"max_players":2,"version":"v"}`
	return prefix + strings.Repeat("a", size-len(prefix)-len(suffix)) + suffix
}

func TestEmbedAcceptsABodyAtTheLimit(t *testing.T) {
	newFakeAPI(t, http.StatusOK, bodyOfSize(1<<20))
	if rec := getEmbed("a.com", ""); rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "public, max-age=60" {
		t.Errorf("status = %d, Cache-Control = %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
}

func TestEmbedTreatsOnlyProblemJSON404AsOffline(t *testing.T) {
	cases := []struct {
		name, contentType string
		wantOffline       bool
	}{
		{"problem+json", "application/problem+json", true},
		{"problem+json with a charset", "application/problem+json; charset=utf-8", true},
		{"whitespace before the parameters", "application/problem+json ; charset=utf-8", true},
		{"mixed case", "Application/Problem+JSON", true},
		{"problem+xml", "application/problem+xml", false},
		{"problem+json as a parameter", "text/x; a=application/problem+json", false},
		{"other +json type", "application/vnd.api+json", false},
		{"problem+json with a suffix", "application/problem+json2", false},
		{"plain json", "application/json", false},
		{"html from a proxy", "text/html", false},
		{"no content type", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			newFakeAPIWithType(t, http.StatusNotFound, tc.contentType, `{"host":"a.com","port":25565,"status":404}`)
			rec := getEmbed("a.com", "")
			if !tc.wantOffline {
				assertBarePage(t, rec)
				return
			}
			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want 200", rec.Code)
			}
			if got := rec.Header().Get("Cache-Control"); got != "public, max-age=30" {
				t.Errorf("Cache-Control = %q, want %q", got, "public, max-age=30")
			}
			if !strings.Contains(head(t, rec.Body.String()), "Server offline or unreachable") {
				t.Error("an offline server should get the offline description")
			}
		})
	}
}

func TestEmbedRouteWiring(t *testing.T) {
	f := newFakeAPI(t, http.StatusOK, `{"host":"play.example.com","port":25570,"num_players":1}`)
	router := NewWebServer("", false).Setup()

	get := httptest.NewRecorder()
	router.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/project/mc-status/Play.Example.com:25570?query=false", nil))
	if get.Code != http.StatusOK || !strings.Contains(get.Body.String(), `property="og:title" content="play.example.com:25570"`) {
		t.Fatalf("GET /project/mc-status/{host} did not reach the handler: %d %s", get.Code, get.Body.String())
	}
	if f.request == nil || f.request.URL.Path != "/api/v1/mcstatus/Play.Example.com:25570" {
		t.Errorf("the host path value did not reach the API: %v", f.request)
	}

	catchAll := httptest.NewRecorder()
	router.ServeHTTP(catchAll, httptest.NewRequest(http.MethodGet, "/", nil))
	f.request = nil
	bare := httptest.NewRecorder()
	router.ServeHTTP(bare, httptest.NewRequest(http.MethodGet, "/project/mc-status", nil))
	if bare.Code != http.StatusOK || !strings.Contains(bare.Body.String(), `id="mc-status-host"`) || strings.Contains(bare.Body.String(), "og:title") {
		t.Errorf("GET /project/mc-status should serve the bare checker: %d", bare.Code)
	}
	if bare.Header().Get("Cache-Control") != "" {
		t.Errorf("the bare checker should not set Cache-Control, got %q", bare.Header().Get("Cache-Control"))
	}
	if f.request != nil {
		t.Error("the bare checker must not call the API")
	}

	for _, path := range []string{"/project/mc-status", "/project/mc-status/a.com"} {
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
			if rec.Code != catchAll.Code || rec.Body.String() != catchAll.Body.String() {
				t.Errorf("%s %s should fall through to the catch-all route, got %d", method, path, rec.Code)
			}
		}
	}

	f.request = nil
	gone := httptest.NewRecorder()
	router.ServeHTTP(gone, httptest.NewRequest(http.MethodGet, "/mcstatus/a.com", nil))
	if gone.Code != catchAll.Code || gone.Body.String() != catchAll.Body.String() {
		t.Errorf("GET /mcstatus/{host} should no longer be a route, got %d", gone.Code)
	}
	if f.request != nil {
		t.Error("the removed route must not reach the API")
	}
}
