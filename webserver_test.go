package main

import (
	"bytes"
	"context"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/config"
	mw "github.com/p0t4t0sandwich/neuralnexus-frontend/middleware"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/test/testutil"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/test/testutil/fakeapi"
)

func serveSetup(req *http.Request) *httptest.ResponseRecorder {
	return serveHandler(NewWebServer("", false).Setup(), req)
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func captureServerLog(t *testing.T) *syncBuffer {
	t.Helper()
	b := &syncBuffer{}
	log.SetOutput(b)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return b
}

func shortSocket(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "nn")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return filepath.Join(dir, "s")
}

func waitForServer(t *testing.T, network, address string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		conn, err := net.Dial(network, address)
		if err == nil {
			conn.Close()
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("nothing listens on %s %s: %v", network, address, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestNewWebServer(t *testing.T) {
	t.Run("SV-01 the server keeps the address and the socket flag it is given", func(t *testing.T) {
		for _, uds := range []bool{false, true} {
			s := NewWebServer("0.0.0.0:8090", uds)
			if s.Address != "0.0.0.0:8090" || s.UsingUDS != uds {
				t.Errorf("server = %+v, want address 0.0.0.0:8090 and UsingUDS %t", s, uds)
			}
		}
	})
}

func TestSetupRoutes(t *testing.T) {
	t.Run("SV-02 each page route serves its own page", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		cases := []struct{ target, marker string }{
			{"/", `showAuthErrorFromQuery();`},
			{"/login", `>Login</h1>`},
			{"/register", `>Sign Up</h1>`},
			{"/projects", `>Projects</h1>`},
			{"/teapot", `418`},
			{"/account", `<title>Account - NeuralNexus</title>`},
			{"/admin", `<title>Admin - NeuralNexus</title>`},
			{"/admin/users", `<title>Users - NeuralNexus</title>`},
			{"/admin/users/u1", `<title>Edit user - NeuralNexus</title>`},
			{"/admin/roles", `<title>Roles - NeuralNexus</title>`},
			{"/admin/roles/r1", `<title>Edit role - NeuralNexus</title>`},
			{"/admin/permissions", `<title>Permissions - NeuralNexus</title>`},
			{"/project/bee-name-generator", `<title>Bee name generator - NeuralNexus</title>`},
			{"/project/bee-name-generator/admin", `<title>Bee name review - NeuralNexus</title>`},
		}
		for _, tc := range cases {
			t.Run(tc.target, func(t *testing.T) {
				rec := serveSetup(httptest.NewRequest(http.MethodGet, tc.target, nil))
				assertStatusCode(t, rec, http.StatusOK)
				assertBodyHas(t, rec, tc.marker)
			})
		}
		f.AssertLines(t)
	})

	t.Run("SV-03 each fragment route reaches its handler and answers a fragment", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /users/me/permissions", 200, `["users.admin","roles.admin","beenamegenerator.admin"]`)
		f.On("GET /users", 200, `[{"user_id":"u1","username":"bob","roles":["r1"]}]`)
		f.On("GET /roles", 200, `[{"id":"r1","name":"bee_admin","description":"d","permissions":[]}]`)
		f.On("GET /permissions", 200, `[{"id":"p1","node":"pets.write","description":"d","value_type":"int","merge":"max"}]`)
		f.On("GET /users/u1", 200, `{"user_id":"u1","username":"bob","roles":["r1"]}`)
		f.On("GET /users/u1/links", 200, `[]`)
		f.On("GET /users/u1/permissions", 200, `[]`)
		f.On("GET /roles/r1", 200, `{"id":"r1","name":"bee_admin","description":"d","permissions":[]}`)
		f.On("GET /users/me", 200, `{"username":"bob"}`)
		f.On("GET /users/me/settings", 200, `{"password_auth":true}`)
		f.On("GET /users/me/links", 200, `[]`)
		f.On("GET /bee-name-generator/suggestion/100", 200, `{"suggestions":["buzz"]}`)
		cases := []struct{ target, marker string }{
			{"/account/content", `id="account-username"`},
			{"/admin/cards", `id="admin-users-link"`},
			{"/admin/users/list", `id="admin-users-search"`},
			{"/admin/users/rows", `id="admin-users-count"`},
			{"/admin/users/u1/editor", `id="admin-user-form"`},
			{"/admin/roles/list", `id="admin-role-create-form"`},
			{"/admin/roles/r1/editor", `id="admin-role-form"`},
			{"/admin/roles/r1/grant-value?grant_permission=p1", `name="grant_value"`},
			{"/admin/permissions/list", `id="admin-permission-create-form"`},
			{"/project/bee-name-generator/admin-link", `id="bee-admin-link"`},
			{"/project/bee-name-generator/admin/suggestions", `buzz`},
		}
		for _, tc := range cases {
			t.Run(tc.target, func(t *testing.T) {
				rec := serveRequest(http.MethodGet, tc.target, nil)
				assertStatusCode(t, rec, http.StatusOK)
				assertBodyHas(t, rec, tc.marker)
				assertBodyLacks(t, rec, "<html", "<body")
			})
		}
	})

	t.Run("SV-04 each route that changes something refuses a request without HX-Request and never calls the API", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		cases := []struct{ method, target string }{
			{http.MethodPost, "/admin/users/u1"},
			{http.MethodPost, "/admin/roles"},
			{http.MethodPost, "/admin/roles/r1"},
			{http.MethodDelete, "/admin/roles/r1"},
			{http.MethodPost, "/admin/roles/r1/permissions"},
			{http.MethodPost, "/admin/roles/r1/permissions/p1"},
			{http.MethodDelete, "/admin/roles/r1/permissions/p1"},
			{http.MethodPost, "/admin/permissions"},
			{http.MethodDelete, "/admin/permissions/p1"},
			{http.MethodPost, "/account/settings"},
			{http.MethodPost, "/account/links/discord"},
			{http.MethodDelete, "/account/links/discord"},
			{http.MethodPost, "/project/bee-name-generator/admin/suggestions"},
		}
		for _, tc := range cases {
			t.Run(tc.method+" "+tc.target, func(t *testing.T) {
				req := httptest.NewRequest(tc.method, tc.target, strings.NewReader(url.Values{"name": {"x"}}.Encode()))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				rec := serveSetup(req)
				assertStatusCode(t, rec, http.StatusForbidden)
				if got := rec.Body.String(); got != "Forbidden\n" {
					t.Errorf("body = %q, want %q", got, "Forbidden\n")
				}
			})
		}
		f.AssertLines(t)
	})

	t.Run("SV-05 the files in public are served and a missing one is a 404", func(t *testing.T) {
		rec := serveSetup(httptest.NewRequest(http.MethodGet, "/public/site.webmanifest", nil))
		assertStatusCode(t, rec, http.StatusOK)
		if rec.Body.Len() == 0 {
			t.Error("the manifest is empty")
		}
		assertStatusCode(t, serveSetup(httptest.NewRequest(http.MethodGet, "/public/nothing-here.txt", nil)), http.StatusNotFound)
	})
}

func TestSetupMiddleware(t *testing.T) {
	t.Run("SV-06 every kind of response carries the security headers", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /users/me/permissions", 200, `["users.admin"]`)
		cases := []struct{ name, method, target string }{
			{"page", http.MethodGet, "/teapot"},
			{"fragment", http.MethodGet, "/admin/cards"},
			{"static file", http.MethodGet, "/public/site.webmanifest"},
			{"missing file", http.MethodGet, "/public/nothing-here.txt"},
			{"refused write", http.MethodPost, "/admin/roles"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				req := httptest.NewRequest(tc.method, tc.target, nil)
				if tc.name == "fragment" {
					req.Header.Set("HX-Request", "true")
				}
				rec := serveSetup(req)
				for header, want := range map[string]string{
					"X-Content-Type-Options":  "nosniff",
					"Referrer-Policy":         "same-origin",
					"Content-Security-Policy": "frame-ancestors 'none'",
					"X-Frame-Options":         "DENY",
				} {
					if got := rec.Header().Get(header); got != want {
						t.Errorf("%s = %q, want %q", header, got, want)
					}
				}
			})
		}
	})

	t.Run("SV-07 the request log line carries the request ID, the forwarded address and the user of the session", func(t *testing.T) {
		secret := mw.JWT_SECRET
		mw.JWT_SECRET = []byte("test-secret")
		t.Cleanup(func() { mw.JWT_SECRET = secret })
		token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, mw.SessionClaims{
			RegisteredClaims: jwt.RegisteredClaims{Subject: "user-123", Audience: jwt.ClaimStrings{config.SiteURL}},
		}).SignedString(mw.JWT_SECRET)
		if err != nil {
			t.Fatal(err)
		}
		logged := captureLog(t)
		req := httptest.NewRequest(http.MethodGet, "/teapot?x=1", nil)
		req.Header.Set(mw.XRequestIDHeader, "424242")
		req.Header.Set(mw.XForwardedForHeader, "203.0.113.9")
		req.AddCookie(testutil.SessionCookie(token))
		serveSetup(req)
		if got := logged.String(); !strings.Contains(got, "424242 user-123 203.0.113.9 200 GET /teapot ") {
			t.Errorf("log = %q, want the request line with the ID, the user and the forwarded address", got)
		}
	})
}

func TestRun(t *testing.T) {
	get := func(t *testing.T, client *http.Client, base string) {
		t.Helper()
		resp, err := client.Get(base + "/teapot")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK || resp.Header.Get("X-Frame-Options") != "DENY" {
			t.Errorf("status %d and X-Frame-Options %q, want 200 and DENY", resp.StatusCode, resp.Header.Get("X-Frame-Options"))
		}
	}
	socketClient := func(path string) *http.Client {
		return &http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", path)
		}}}
	}

	t.Run("SV-08 a server on a socket path answers requests over the socket", func(t *testing.T) {
		path := shortSocket(t)
		captureServerLog(t)
		go NewWebServer(path, true).Run()
		waitForServer(t, "unix", path)
		get(t, socketClient(path), "http://unix")
	})

	t.Run("SV-09 a stale file at the socket path is removed and logged before the server listens", func(t *testing.T) {
		path := shortSocket(t)
		if err := os.WriteFile(path, []byte("stale"), 0o600); err != nil {
			t.Fatal(err)
		}
		logged := captureServerLog(t)
		go NewWebServer(path, true).Run()
		waitForServer(t, "unix", path)
		if want := "Removing existing socket file " + path; !strings.Contains(logged.String(), want) {
			t.Errorf("log = %q, want %q", logged.String(), want)
		}
		get(t, socketClient(path), "http://unix")
	})

	t.Run("SV-10 a socket path that cannot be removed ends Run with that error", func(t *testing.T) {
		path := shortSocket(t)
		if err := os.MkdirAll(filepath.Join(path, "inner"), 0o700); err != nil {
			t.Fatal(err)
		}
		captureServerLog(t)
		if err := NewWebServer(path, true).Run(); err == nil {
			t.Error("Run returned nil, want the removal error")
		}
	})

	t.Run("SV-11 a socket path that cannot be listened on ends Run with the listen error", func(t *testing.T) {
		path := filepath.Join(shortSocket(t), "missing", "s")
		if err := NewWebServer(path, true).Run(); err == nil {
			t.Error("Run returned nil, want the listen error")
		}
	})

	t.Run("SV-12 a server on a TCP address answers requests", func(t *testing.T) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		address := listener.Addr().String()
		listener.Close()
		captureServerLog(t)
		go NewWebServer(address, false).Run()
		waitForServer(t, "tcp", address)
		get(t, http.DefaultClient, "http://"+address)
	})

	t.Run("SV-13 a TCP address that cannot be listened on ends Run with the listen error", func(t *testing.T) {
		taken, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { taken.Close() })
		for name, address := range map[string]string{"not a port": "127.0.0.1:notaport", "already in use": taken.Addr().String()} {
			t.Run(name, func(t *testing.T) {
				captureServerLog(t)
				if err := NewWebServer(address, false).Run(); err == nil {
					t.Errorf("Run(%q) returned nil, want an error", address)
				}
			})
		}
	})
}
