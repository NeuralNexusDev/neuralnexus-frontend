package main

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/p0t4t0sandwich/neuralnexus-frontend/config"
	mw "github.com/p0t4t0sandwich/neuralnexus-frontend/middleware"
)

const secretSession = "secret-session-value"

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

func TestResponsesCarryTheSecurityHeaders(t *testing.T) {
	f := newFakeBackend(t)
	f.on("GET /users/me/permissions", 200, `["users.admin"]`)
	for _, target := range []string{"/teapot", "/admin/cards", "/public/site.webmanifest"} {
		t.Run(target, func(t *testing.T) {
			rec := getPage(target)
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
}

func TestStateChangesNeedHTMXAndASameOriginSite(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /unlisted", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("GET /unlisted", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("DELETE /removable", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusAccepted) })
	handler := requireHTMXForWrites(mux)
	cases := []struct {
		name, method, target string
		headers              map[string]string
		want                 int
	}{
		{"a new route without HX-Request", http.MethodPost, "/unlisted", nil, http.StatusForbidden},
		{"a new route with HX-Request", http.MethodPost, "/unlisted", map[string]string{"HX-Request": "true"}, http.StatusNoContent},
		{"HX-Request set to something else", http.MethodPost, "/unlisted", map[string]string{"HX-Request": "false"}, http.StatusForbidden},
		{"a cross-site request", http.MethodPost, "/unlisted", map[string]string{"HX-Request": "true", "Sec-Fetch-Site": "cross-site"}, http.StatusForbidden},
		{"a same-site request", http.MethodPost, "/unlisted", map[string]string{"HX-Request": "true", "Sec-Fetch-Site": "same-site"}, http.StatusForbidden},
		{"a same-origin request", http.MethodPost, "/unlisted", map[string]string{"HX-Request": "true", "Sec-Fetch-Site": "same-origin"}, http.StatusNoContent},
		{"a request that began in the address bar", http.MethodPost, "/unlisted", map[string]string{"HX-Request": "true", "Sec-Fetch-Site": "none"}, http.StatusNoContent},
		{"a method the route does not list reaches the catch-all", http.MethodDelete, "/unlisted", nil, http.StatusAccepted},
		{"a delete without HX-Request", http.MethodDelete, "/removable", nil, http.StatusForbidden},
		{"a read", http.MethodGet, "/unlisted", nil, http.StatusNoContent},
		{"a cross-site read", http.MethodGet, "/unlisted", map[string]string{"Sec-Fetch-Site": "cross-site"}, http.StatusNoContent},
		{"the catch-all route", http.MethodPost, "/anywhere", nil, http.StatusAccepted},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.target, nil)
			for header, value := range tc.headers {
				req.Header.Set(header, value)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			assertStatus(t, rec, tc.want)
		})
	}
}

func TestStateChangesKeepTheMuxAnswerForAMethodItDoesNotServe(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /only", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	rec := httptest.NewRecorder()
	requireHTMXForWrites(mux).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/only", nil))
	assertStatus(t, rec, http.StatusMethodNotAllowed)
}

func TestFailedAPICallsLogTheirCauseWithoutTheSession(t *testing.T) {
	cookie := &http.Cookie{Name: "session", Value: secretSession}
	t.Run("an API that cannot be reached", func(t *testing.T) {
		down := httptest.NewServer(http.NotFoundHandler())
		apiURL := config.APIURL
		config.APIURL = down.URL
		t.Cleanup(func() { config.APIURL = apiURL })
		down.Close()
		buf := captureLog(t)
		rec := pageReq{method: http.MethodGet, target: "/admin/roles/list", cookies: []*http.Cookie{cookie}}.do()
		assertStatus(t, rec, http.StatusBadGateway)
		for _, want := range []string{"request failed", "status=502", `api="GET /roles"`, "request_id=", "connection refused"} {
			if !strings.Contains(buf.String(), want) {
				t.Errorf("the log is missing %q:\n%s", want, buf.String())
			}
		}
		if strings.Contains(buf.String(), secretSession) {
			t.Errorf("the log holds the session cookie:\n%s", buf.String())
		}
	})
	t.Run("an API that answers 500", func(t *testing.T) {
		f := newFakeBackend(t)
		f.problem("GET /roles", 500, "the database is down")
		buf := captureLog(t)
		rec := pageReq{method: http.MethodGet, target: "/admin/roles/list", cookies: []*http.Cookie{cookie}}.do()
		assertStatus(t, rec, http.StatusInternalServerError)
		for _, want := range []string{"request failed", "status=500", `api="GET /roles"`, "request_id="} {
			if !strings.Contains(buf.String(), want) {
				t.Errorf("the log is missing %q:\n%s", want, buf.String())
			}
		}
		if strings.Contains(buf.String(), secretSession) {
			t.Errorf("the log holds the session cookie:\n%s", buf.String())
		}
	})
	t.Run("an API that refuses with a 4xx", func(t *testing.T) {
		f := newFakeBackend(t)
		f.problem("GET /roles", 403, "You do not have permission")
		buf := captureLog(t)
		pageReq{method: http.MethodGet, target: "/admin/roles/list", cookies: []*http.Cookie{cookie}}.do()
		if strings.Contains(buf.String(), "request failed") {
			t.Errorf("a refusal was logged as a failure:\n%s", buf.String())
		}
	})
}

func TestAbandonedRequestsAreNotLoggedAsFailures(t *testing.T) {
	newFakeBackend(t)
	buf := captureLog(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rec := httptest.NewRecorder()
	NewWebServer("", false).Setup().ServeHTTP(rec, httptest.NewRequestWithContext(ctx, http.MethodGet, "/admin/roles/list", nil))
	if strings.Contains(buf.String(), "request failed") {
		t.Errorf("a request the client abandoned was logged as a failure:\n%s", buf.String())
	}
}

func TestAdminRequestsStopWaitingForASlowAPI(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}))
	t.Cleanup(slow.Close)
	apiURL, timeout := config.APIURL, pageRequestTimeout
	config.APIURL, pageRequestTimeout = slow.URL, 50*time.Millisecond
	t.Cleanup(func() { config.APIURL, pageRequestTimeout = apiURL, timeout })
	captureLog(t)
	start := time.Now()
	rec := getPage("/admin/roles/list")
	assertStatus(t, rec, http.StatusBadGateway)
	if time.Since(start) > 2*time.Second {
		t.Errorf("the request waited %s for the API", time.Since(start))
	}
}

func TestAPanicInAHandlerAnswers500AndLogsWhereItHappened(t *testing.T) {
	buf := captureLog(t)
	handler := mw.RecoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("broken handler")
	}))
	req := httptest.NewRequest(http.MethodPost, "/boom", strings.NewReader("password=hunter2"))
	req.AddCookie(&http.Cookie{Name: "session", Value: secretSession})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assertStatus(t, rec, http.StatusInternalServerError)
	if got := rec.Body.String(); got != "Internal Server Error\n" {
		t.Errorf("body = %q", got)
	}
	for _, want := range []string{"panic serving POST /boom", "broken handler"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("the log is missing %q:\n%s", want, buf.String())
		}
	}
	for _, unwanted := range []string{secretSession, "hunter2"} {
		if strings.Contains(buf.String(), unwanted) {
			t.Errorf("the log holds %q:\n%s", unwanted, buf.String())
		}
	}
}

func TestAPanicLogsTheRequestIDThatTheRequestCarries(t *testing.T) {
	buf := captureLog(t)
	handler := mw.CreateStack(mw.RecoveryMiddleware, mw.RequestIDMiddleware)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("broken handler")
	}))
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	req.Header.Set(mw.XRequestIDHeader, "424242")
	handler.ServeHTTP(httptest.NewRecorder(), req)
	if !strings.Contains(buf.String(), "panic serving GET /boom: request_id=424242 broken handler") {
		t.Errorf("the log line does not carry the request ID:\n%s", buf.String())
	}

	buf.Reset()
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/boom", nil))
	if !strings.Contains(buf.String(), "request_id=") || strings.Contains(buf.String(), "request_id=N/A") {
		t.Errorf("the log line does not carry a generated request ID:\n%s", buf.String())
	}
}

func TestAPanicAfterTheHeaderLeavesTheResponseAlone(t *testing.T) {
	cases := []struct {
		name       string
		handler    func(w http.ResponseWriter)
		wantStatus int
		wantBody   string
	}{
		{"before anything is written", func(http.ResponseWriter) {}, http.StatusInternalServerError, "Internal Server Error\n"},
		{"after the body started", func(w http.ResponseWriter) { _, _ = w.Write([]byte("partial")) }, http.StatusOK, "partial"},
		{"after the header", func(w http.ResponseWriter) { w.WriteHeader(http.StatusCreated) }, http.StatusCreated, ""},
		{"after a redirect", func(w http.ResponseWriter) { w.Header().Set("HX-Redirect", "/x"); w.WriteHeader(http.StatusOK) }, http.StatusOK, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := captureLog(t)
			handler := mw.RecoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tc.handler(w)
				panic("broken handler")
			}))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))
			assertStatus(t, rec, tc.wantStatus)
			if got := rec.Body.String(); got != tc.wantBody {
				t.Errorf("body = %q, want %q", got, tc.wantBody)
			}
			if !strings.Contains(buf.String(), "panic serving GET /boom") {
				t.Errorf("the panic was not logged:\n%s", buf.String())
			}
		})
	}
}

func TestLinksRenderTheirPaths(t *testing.T) {
	f := newFakeBackend(t)
	f.on("GET /users", 200, usersJSON)
	f.on("GET /roles", 200, rolesJSON)
	f.on("GET /users/me/permissions", 200, `["users.admin","roles.admin"]`)
	cases := []struct{ target, want string }{
		{"/admin/users", `href="/admin"`},
		{"/admin/users/" + idBob, `href="/admin/users"`},
		{"/admin/roles", `href="/admin/permissions"`},
		{"/admin/roles/" + idBee, `href="/admin/roles"`},
		{"/admin/permissions", `href="/admin/roles"`},
		{"/projects", `href="/project/bee-name-generator"`},
		{"/admin/cards", `href="/admin/users"`},
		{"/admin/users/list", `href="/admin/users/` + idBob + `"`},
		{"/admin/roles/list", `href="/admin/roles/` + idBee + `"`},
	}
	for _, tc := range cases {
		t.Run(tc.target, func(t *testing.T) {
			assertBody(t, getPage(tc.target), tc.want)
		})
	}
}

func TestRefusedCreateFormsFlagTheFieldAndKeepTheBanner(t *testing.T) {
	cases := []struct {
		name, target, route, field, value string
		form                              url.Values
	}{
		{"role", "/admin/roles", "POST /roles", "admin-role-create-name", "Bad Name", url.Values{"name": {"Bad Name"}}},
		{"permission", "/admin/permissions", "POST /permissions", "admin-permission-create-node", "Bad Node", url.Values{"node": {"Bad Node"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name+" refused", func(t *testing.T) {
			f := newFakeBackend(t)
			f.problem(tc.route, 409, "Already exists")
			rec := action(http.MethodPost, tc.target, tc.form)
			assertStatus(t, rec, http.StatusConflict)
			if got := bannerText(rec); got != "Already exists" {
				t.Errorf("banner = %q", got)
			}
			assertBody(t, rec,
				`id="`+tc.field+`-field" hx-swap-oob="true"`,
				`aria-describedby="`+tc.field+`-error `+tc.field+`-help"`,
				`aria-invalid="true"`, ` autofocus`,
				`value="Bad `, `<p id="`+tc.field+`-error" class="`, `>Already exists</p>`)
		})
		t.Run(tc.name+" failed server side", func(t *testing.T) {
			f := newFakeBackend(t)
			f.problem(tc.route, 500, "database down")
			captureLog(t)
			rec := action(http.MethodPost, tc.target, tc.form)
			assertStatus(t, rec, http.StatusInternalServerError)
			assertNoBody(t, rec, `aria-invalid`, `-error"`)
		})
	}
}

func TestRefusedUsernamesFlagTheUsernameField(t *testing.T) {
	t.Run("emptied", func(t *testing.T) {
		f := newFakeBackend(t)
		seedUserEditor(f)
		rec := saveUser(userForm(url.Values{"username": {"  "}}))
		assertBody(t, rec, `id="admin-user-username-field" hx-swap-oob="true"`, `aria-invalid="true"`, `aria-describedby="admin-user-username-error"`, `<p id="admin-user-username-error"`, ">Enter a username</p>")
	})
	t.Run("taken", func(t *testing.T) {
		f := newFakeBackend(t)
		seedUserEditor(f)
		f.problem("PUT /users/"+idBob, 409, "An account with this username already exists")
		rec := saveUser(userForm(url.Values{"username": {"alice"}}))
		assertStatus(t, rec, http.StatusConflict)
		assertBody(t, rec, `id="admin-user-username-field" hx-swap-oob="true"`, `value="alice"`, ">An account with this username already exists</p>")
	})
	t.Run("a refused role change leaves the username field alone", func(t *testing.T) {
		f := newFakeBackend(t)
		seedUserEditor(f)
		f.problem("PUT /users/"+idBob, 409, "That role cannot be held with another")
		rec := saveUser(userForm(url.Values{"roles": {idBee, idSystem}}))
		assertStatus(t, rec, http.StatusConflict)
		assertNoBody(t, rec, `admin-user-username-field`, `aria-invalid`)
	})
}

func TestRefusedRoleNamesFlagTheNameField(t *testing.T) {
	roleForm := func(name string) url.Values {
		return url.Values{"loaded_name": {"bee_admin"}, "name": {name}, "loaded_description": {"Bee Name Generator Admin"}, "description": {"Bee Name Generator Admin"}}
	}
	t.Run("emptied", func(t *testing.T) {
		newFakeBackend(t)
		rec := action(http.MethodPost, "/admin/roles/"+idBee, roleForm("  "))
		assertStatus(t, rec, http.StatusBadRequest)
		assertBody(t, rec, `id="admin-role-name-field" hx-swap-oob="true"`, `aria-invalid="true"`, ` autofocus`,
			`aria-describedby="admin-role-name-error"`, `<p id="admin-role-name-error"`, ">Enter a name</p>")
	})
	t.Run("taken", func(t *testing.T) {
		f := newFakeBackend(t)
		f.problem("PATCH /roles/"+idBee, 409, "A role with this name already exists")
		rec := action(http.MethodPost, "/admin/roles/"+idBee, roleForm("owner"))
		assertStatus(t, rec, http.StatusConflict)
		assertBody(t, rec, `id="admin-role-name-field" hx-swap-oob="true"`, `aria-invalid="true"`, ` autofocus`,
			`value="owner"`, ">A role with this name already exists</p>")
	})
	t.Run("a refused description change leaves the name field alone", func(t *testing.T) {
		f := newFakeBackend(t)
		f.problem("PATCH /roles/"+idBee, 409, "That description is reserved")
		form := roleForm("bee_admin")
		form.Set("description", "changed")
		rec := action(http.MethodPost, "/admin/roles/"+idBee, form)
		assertStatus(t, rec, http.StatusConflict)
		assertNoBody(t, rec, `admin-role-name-field`, `aria-invalid`)
	})
	t.Run("a failure on the server leaves the name field alone", func(t *testing.T) {
		f := newFakeBackend(t)
		f.problem("PATCH /roles/"+idBee, 500, "database down")
		captureLog(t)
		rec := action(http.MethodPost, "/admin/roles/"+idBee, roleForm("owner"))
		assertStatus(t, rec, http.StatusInternalServerError)
		assertNoBody(t, rec, `admin-role-name-field`, `aria-invalid`)
	})
}

func TestFailedAPICallsLogTheMethodAndPathOfTheCall(t *testing.T) {
	cases := []struct {
		name   string
		call   func() *httptest.ResponseRecorder
		route  string
		status int
	}{
		{"a bee review", func() *httptest.ResponseRecorder {
			return reviewBee(url.Values{"name": {"royal jelly/queen?"}, "action": {"accept"}})
		}, "PUT /bee-name-generator/suggestion/royal%20jelly%2Fqueen%3F", http.StatusBadGateway},
		{"a user save", func() *httptest.ResponseRecorder {
			return saveUser(userForm(url.Values{"username": {"robert"}}))
		}, "PUT /users/" + idBob, http.StatusBadGateway},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeBackend(t)
			f.problem(tc.route, tc.status, "the API is down")
			buf := captureLog(t)
			assertStatus(t, tc.call(), tc.status)
			failure, _, _ := strings.Cut(buf.String(), "\n")
			if !strings.Contains(failure, `api="`+tc.route+`"`) {
				t.Errorf("the failure line does not hold the call %q:\n%s", tc.route, failure)
			}
		})
	}
}

func TestFieldHelpIsTiedToItsInput(t *testing.T) {
	f := newFakeBackend(t)
	f.on("GET /roles", 200, `[]`)
	f.on("GET /permissions", 200, `[]`)
	assertBody(t, getPage("/admin/roles/list"), `id="admin-role-create-name-help"`, `aria-describedby="admin-role-create-name-help"`)
	assertNoBody(t, getPage("/admin/roles/list"), `aria-invalid`)
	assertBody(t, getPage("/admin/permissions/list"), `id="admin-permission-create-node-help"`, `aria-describedby="admin-permission-create-node-help"`)
}

func TestUserListTellsHowManyUsersEachResponseHolds(t *testing.T) {
	f := newFakeBackend(t)
	f.on("GET /users", 200, usersJSON)
	f.on("GET /roles", 200, `[]`)
	shell := getPage("/admin/users")
	assertBody(t, shell, `id="admin-users-count" role="status" class="`)
	assertBody(t, shell, `></p>`)
	assertNoBody(t, shell, `users</p>`)
	assertBody(t, getPage("/admin/users/list"), `id="admin-users-count" role="status" hx-swap-oob="innerHTML"`, ">3 users</p>")
	rec := pageReq{method: http.MethodGet, target: "/admin/users/rows?search=bob", htmx: true}.do()
	assertBody(t, rec, `id="admin-users-count" role="status" hx-swap-oob="innerHTML"`, ">1 user</p>")
	rec = pageReq{method: http.MethodGet, target: "/admin/users/rows?search=zzz", htmx: true}.do()
	assertBody(t, rec, ">No matches</p>")
	f.on("GET /users", 200, fullPage(200))
	rec = pageReq{method: http.MethodGet, target: "/admin/users/rows?offset=200&search=u", htmx: true}.do()
	assertBody(t, rec, "more users. Searched the first ")
}

func TestTheGlueScriptLoadsBeforeHTMX(t *testing.T) {
	newFakeBackend(t)
	for _, target := range []string{"/admin", "/account", "/project/bee-name-generator"} {
		t.Run(target, func(t *testing.T) {
			body := getPage(target).Body.String()
			glue, htmx := strings.Index(body, `src="/public/js/htmx-glue.js"`), strings.Index(body, "htmx.min.js")
			if glue < 0 || htmx < 0 || glue > htmx {
				t.Errorf("htmx-glue.js at %d, htmx.min.js at %d, want the glue first so it hears the first request", glue, htmx)
			}
		})
	}
}
