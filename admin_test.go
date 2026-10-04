package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/p0t4t0sandwich/neuralnexus-frontend/config"
)

// Snowflake-sized IDs, above 2^53, so any number conversion changes them.
const (
	idSystem = "3541025163146757610"
	idBee    = "3541025163146757630"
	idPBee   = "3541025163146757710"
	idPRate  = "3541025163146757730"
	idPPets  = "3541025163146757780"
	idPMotd  = "3541025163146757790"
	idPStore = "3541025163146757800"
	idAlice  = "3541025163146759010"
	idBob    = "3541025163146759020"
	idAnon   = "3541025163146759030"

	hostile = `<img src=x onerror="window.__xss=1">`
)

type fakeResponse struct {
	status int
	body   string
}

type fakeCall struct {
	method, uri, body, cookie string
}

type fakeAdmin struct {
	mu     sync.Mutex
	calls  []fakeCall
	routes map[string]fakeResponse
}

func newFakeAdmin(t *testing.T) *fakeAdmin {
	t.Helper()
	f := &fakeAdmin{routes: map[string]fakeResponse{}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		cookie := ""
		if c, err := r.Cookie("session"); err == nil {
			cookie = c.Value
		}
		f.mu.Lock()
		uri := strings.TrimPrefix(r.URL.RequestURI(), "/api/v1")
		f.calls = append(f.calls, fakeCall{method: r.Method, uri: uri, body: string(body), cookie: cookie})
		response, ok := f.routes[r.Method+" "+strings.TrimPrefix(r.URL.EscapedPath(), "/api/v1")]
		f.mu.Unlock()
		if !ok {
			response = fakeResponse{status: http.StatusInternalServerError, body: `{"detail":"unexpected call"}`}
		}
		if response.status >= http.StatusBadRequest {
			w.Header().Set("Content-Type", "application/problem+json")
		} else {
			w.Header().Set("Content-Type", "application/json")
		}
		w.WriteHeader(response.status)
		fmt.Fprint(w, response.body)
	}))
	t.Cleanup(server.Close)
	apiURL := config.APIURL
	config.APIURL = server.URL
	t.Cleanup(func() { config.APIURL = apiURL })
	return f
}

func (f *fakeAdmin) on(route string, status int, body string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.routes[route] = fakeResponse{status: status, body: body}
}

func (f *fakeAdmin) problem(route string, status int, detail string) {
	f.on(route, status, fmt.Sprintf(`{"detail":%q}`, detail))
}

func (f *fakeAdmin) writes() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, call := range f.calls {
		if call.method != http.MethodGet {
			out = append(out, call.method+" "+call.uri+" "+call.body)
		}
	}
	return out
}

func (f *fakeAdmin) uris() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, call := range f.calls {
		out = append(out, call.method+" "+call.uri)
	}
	return out
}

func (f *fakeAdmin) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = nil
}

type adminReq struct {
	method, target string
	form           url.Values
	htmx           bool
	cookies        []*http.Cookie
}

func (a adminReq) do() *httptest.ResponseRecorder {
	var body io.Reader
	if a.form != nil {
		body = strings.NewReader(a.form.Encode())
	}
	req := httptest.NewRequest(a.method, a.target, body)
	if a.form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if a.htmx {
		req.Header.Set("HX-Request", "true")
	}
	for _, c := range a.cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	NewWebServer("", false).Setup().ServeHTTP(rec, req)
	return rec
}

func getPage(target string) *httptest.ResponseRecorder {
	return adminReq{method: http.MethodGet, target: target}.do()
}

func action(method, target string, form url.Values) *httptest.ResponseRecorder {
	return adminReq{method: method, target: target, form: form, htmx: true}.do()
}

// bannerText is the text of an error response without the status line it empties out of band.
func bannerText(rec *httptest.ResponseRecorder) string {
	body, _, _ := strings.Cut(rec.Body.String(), "<p id=")
	return body
}

func short(body string) string {
	if start := strings.Index(body, `class="relative isolate`); start >= 0 {
		body = body[start:]
	}
	if len(body) > 700 {
		return body[:700] + "..."
	}
	return body
}

func assertBody(t *testing.T, rec *httptest.ResponseRecorder, want ...string) {
	t.Helper()
	body := rec.Body.String()
	for _, w := range want {
		if !strings.Contains(body, w) {
			t.Errorf("the response is missing %q:\n%s", w, short(body))
		}
	}
}

func assertNoBody(t *testing.T, rec *httptest.ResponseRecorder, unwanted ...string) {
	t.Helper()
	body := rec.Body.String()
	for _, u := range unwanted {
		if strings.Contains(body, u) {
			t.Errorf("the response should not contain %q:\n%s", u, short(body))
		}
	}
}

func assertStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Errorf("status = %d, want %d: %s", rec.Code, want, short(rec.Body.String()))
	}
}

func assertWrites(t *testing.T, f *fakeAdmin, want ...string) {
	t.Helper()
	got := f.writes()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("API writes = %q, want %q", got, want)
	}
}

const (
	rolesJSON = `[
		{"id":"` + idSystem + `","name":"system","description":"System","permissions":[]},
		{"id":"` + idBee + `","name":"bee_admin","description":"Bee Name Generator Admin","permissions":[
			{"id":"` + idPBee + `","node":"beenamegenerator.admin","description":"Bee name generator"},
			{"id":"` + idPRate + `","node":"ratelimit","description":"Rate limit","value_type":"int","merge":"max","value":100}
		]}
	]`
	permissionsJSON = `[
		{"id":"` + idPBee + `","node":"beenamegenerator.admin","description":"Bee name generator"},
		{"id":"` + idPRate + `","node":"ratelimit","description":"Rate limit","value_type":"int","merge":"max"},
		{"id":"` + idPPets + `","node":"petpictures.pets","description":"Pet pictures","value_type":"string_list","merge":"union"},
		{"id":"` + idPMotd + `","node":"motd","description":"Message of the day","value_type":"string","merge":"first"},
		{"id":"` + idPStore + `","node":"datastore.admin","description":"Data store"}
	]`
	usersJSON = `[
		{"user_id":"` + idAlice + `","username":"alice","roles":["` + idSystem + `"]},
		{"user_id":"` + idBob + `","username":"bob","roles":["` + idBee + `","` + idSystem + `"]},
		{"user_id":"` + idAnon + `","username":"","roles":[]}
	]`
)

func TestAdminShellsHoldNoDataAndLoadTheirContent(t *testing.T) {
	f := newFakeAdmin(t)
	cases := []struct{ path, loads string }{
		{"/admin", "/admin/cards"},
		{"/admin/users", "/admin/users/list"},
		{"/admin/users/" + idBob, "/admin/users/" + idBob + "/editor"},
		{"/admin/roles", "/admin/roles/list"},
		{"/admin/roles/" + idBee, "/admin/roles/" + idBee + "/editor"},
		{"/admin/permissions", "/admin/permissions/list"},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			f.reset()
			rec := getPage(tc.path)
			assertStatus(t, rec, http.StatusOK)
			assertBody(t, rec, `hx-get="`+tc.loads+`"`, `hx-trigger="load"`, `hx-swap="outerHTML"`, `id="admin-error"`,
				`<script src="https://cdn.neuralnexus.dev/htmx/htmx.v1.9.5.min.js" defer>`, `&#34;selfRequestsOnly&#34;:true`, `&#34;allowEval&#34;:false`)
			if len(f.uris()) != 0 {
				t.Errorf("a shell called the API: %v", f.uris())
			}
		})
	}
}

func TestAdminUserAndRoleShellsHaveAStatusLine(t *testing.T) {
	assertBody(t, getPage("/admin/users/"+idBob), `id="admin-user-status" role="status"`)
	assertBody(t, getPage("/admin/roles/"+idBee), `id="admin-role-status" role="status"`)
	assertNoBody(t, getPage("/admin/users"), `role="status"`)
}

func TestAdminShellsEscapeTheIDInTheirLoadPath(t *testing.T) {
	rec := getPage("/admin/users/" + url.PathEscape(hostile+"id"))
	assertNoBody(t, rec, "<img src=x")
	assertBody(t, rec, `hx-get="/admin/users/%3Cimg%20src=x%20onerror=%22window.__xss=1%22%3Eid/editor"`)
}

func TestAdminFragmentsHaveNoPageChrome(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("GET /users/me/permissions", 200, `["users.admin","roles.admin"]`)
	f.on("GET /users", 200, usersJSON)
	f.on("GET /roles", 200, rolesJSON)
	f.on("GET /permissions", 200, permissionsJSON)
	f.on("GET /users/"+idBob, 200, `{"user_id":"`+idBob+`","username":"bob","roles":["`+idBee+`"]}`)
	f.on("GET /users/"+idBob+"/links", 200, `[]`)
	f.on("GET /users/"+idBob+"/permissions", 200, `[]`)
	f.on("GET /roles/"+idBee, 200, `{"id":"`+idBee+`","name":"bee_admin","description":"d","permissions":[]}`)
	cases := []struct{ path, marker string }{
		{"/admin/cards", `id="admin-users-link"`},
		{"/admin/users/list", `id="admin-users-search"`},
		{"/admin/users/" + idBob + "/editor", `id="admin-user-form"`},
		{"/admin/roles/list", `id="admin-role-create-form"`},
		{"/admin/roles/" + idBee + "/editor", `id="admin-role-form"`},
		{"/admin/permissions/list", `id="admin-permission-create-form"`},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			rec := getPage(tc.path)
			assertStatus(t, rec, http.StatusOK)
			assertBody(t, rec, tc.marker)
			assertNoBody(t, rec, "<html", "<body", `id="admin-error" role="alert"`)
		})
	}
}

func TestAdminForwardsOnlyTheSessionCookie(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("GET /users/me/permissions", 200, `["users.admin"]`)
	adminReq{method: http.MethodGet, target: "/admin/cards", cookies: []*http.Cookie{
		{Name: "session", Value: "jwt-value"},
		{Name: "nonce", Value: "other"},
	}}.do()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) != 1 || f.calls[0].cookie != "jwt-value" {
		t.Errorf("API calls = %+v, want one call carrying the session cookie", f.calls)
	}
}

func TestAdminActionsNeedHTMX(t *testing.T) {
	f := newFakeAdmin(t)
	cases := []struct{ method, target string }{
		{http.MethodPost, "/admin/users/" + idBob},
		{http.MethodPost, "/admin/roles"},
		{http.MethodPost, "/admin/roles/" + idBee},
		{http.MethodDelete, "/admin/roles/" + idBee},
		{http.MethodPost, "/admin/roles/" + idBee + "/permissions"},
		{http.MethodPost, "/admin/roles/" + idBee + "/permissions/" + idPRate},
		{http.MethodPost, "/admin/roles/" + idBee + "/permissions/" + idPRate + "/remove"},
		{http.MethodPost, "/admin/permissions"},
		{http.MethodDelete, "/admin/permissions/" + idPRate},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.target, func(t *testing.T) {
			f.reset()
			rec := adminReq{method: tc.method, target: tc.target, form: url.Values{"name": {"x"}}}.do()
			assertStatus(t, rec, http.StatusForbidden)
			if len(f.uris()) != 0 {
				t.Errorf("a request without HX-Request reached the API: %v", f.uris())
			}
		})
	}
}

func TestAdminSignedOutVisitors(t *testing.T) {
	f := newFakeAdmin(t)
	for _, route := range []string{"GET /users/me/permissions", "GET /users", "GET /roles", "GET /permissions", "GET /users/" + idBob, "GET /roles/" + idBee, "POST /roles"} {
		f.problem(route, 401, "sign in")
	}
	for _, target := range []string{
		"/admin/cards", "/admin/users/list", "/admin/users/" + idBob + "/editor", "/admin/roles/list",
		"/admin/roles/" + idBee + "/editor", "/admin/permissions/list", "/admin/users/rows?offset=200",
	} {
		t.Run("fragment "+target, func(t *testing.T) {
			rec := getPage(target)
			assertStatus(t, rec, http.StatusUnauthorized)
			if got := rec.Header().Get("HX-Redirect"); got != "/login" {
				t.Errorf("HX-Redirect = %q, want /login", got)
			}
		})
	}
	rec := adminReq{method: http.MethodGet, target: "/admin/roles/" + idBee + "/grant-value?grant_permission=" + idPRate, htmx: true}.do()
	if got := rec.Header().Get("HX-Redirect"); got != "/login" {
		t.Errorf("HX-Redirect = %q, want /login", got)
	}
	rec = action(http.MethodPost, "/admin/roles", url.Values{"name": {"x"}})
	if got := rec.Header().Get("HX-Redirect"); got != "/login" {
		t.Errorf("HX-Redirect = %q, want /login", got)
	}
}

func TestAdminRefusalsShowTheAPIMessage(t *testing.T) {
	f := newFakeAdmin(t)
	f.problem("GET /users/me/permissions", 403, "You may not")
	rec := getPage("/admin/cards")
	assertStatus(t, rec, http.StatusForbidden)
	if got := bannerText(rec); got != "You may not" {
		t.Errorf("body = %q", got)
	}
	if got := rec.Header().Get("HX-Retarget"); got != "#admin-error" {
		t.Errorf("HX-Retarget = %q", got)
	}

	f.problem("POST /roles", 409, "A role with that name already exists")
	rec = action(http.MethodPost, "/admin/roles", url.Values{"name": {"system"}})
	assertStatus(t, rec, http.StatusConflict)
	if got := rec.Header().Get("HX-Retarget"); got != "#admin-error" {
		t.Errorf("HX-Retarget = %q", got)
	}
	if got := rec.Header().Get("HX-Reswap"); got != "innerHTML" {
		t.Errorf("HX-Reswap = %q", got)
	}
	if got := bannerText(rec); got != "A role with that name already exists" {
		t.Errorf("body = %q", got)
	}
}

func TestAdminErrorMessagesAreEscaped(t *testing.T) {
	f := newFakeAdmin(t)
	f.problem("GET /users/me/permissions", 500, hostile)
	rec := getPage("/admin/cards")
	assertNoBody(t, rec, "<img src=x")
	assertBody(t, rec, "&lt;img src=x")

	f.problem("POST /roles", 400, hostile)
	rec = action(http.MethodPost, "/admin/roles", url.Values{"name": {"x"}})
	assertNoBody(t, rec, "<img")
	assertBody(t, rec, "&lt;img src=x")
}

func TestAdminAPIDownGivesAGenericMessage(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("GET /users/me/permissions", 200, `["users.admin"]`)
	config.APIURL = "http://127.0.0.1:1"
	rec := getPage("/admin/cards")
	assertStatus(t, rec, http.StatusBadGateway)
	assertBody(t, rec, "Failed to load your permissions")
	rec = action(http.MethodPost, "/admin/roles", url.Values{"name": {"x"}})
	assertStatus(t, rec, http.StatusBadGateway)
	if got := bannerText(rec); got != "Failed to create the role" {
		t.Errorf("body = %q", got)
	}
}

func TestAdminDashboardShowsTheCardsForThePermissionsHeld(t *testing.T) {
	cases := []struct {
		name        string
		permissions string
		users       bool
		roles       bool
	}{
		{"users", `["users.admin"]`, true, false},
		{"roles", `["roles.admin"]`, false, true},
		{"both", `["ratelimit:1000","users.admin","roles.admin"]`, true, true},
		{"valued node", `["users.admin:1"]`, true, false},
		{"neither", `["ratelimit:1000"]`, false, false},
		{"lookalikes", `["users.administrator","xroles.admin","roles.adminx:1"]`, false, false},
		{"none", `[]`, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeAdmin(t)
			f.on("GET /users/me/permissions", 200, tc.permissions)
			rec := getPage("/admin/cards")
			assertStatus(t, rec, http.StatusOK)
			cards := map[string]bool{
				`id="admin-users-link"`:       tc.users,
				`id="admin-roles-link"`:       tc.roles,
				`id="admin-permissions-link"`: tc.roles,
				`id="admin-denied"`:           !tc.users && !tc.roles,
			}
			for marker, want := range cards {
				if got := strings.Contains(rec.Body.String(), marker); got != want {
					t.Errorf("%s present = %v, want %v", marker, got, want)
				}
			}
		})
	}
}

func TestAdminSavesLeaveUnchangedPaddedTextAlone(t *testing.T) {
	f := newFakeAdmin(t)
	seedUserEditor(f)
	form := userForm(url.Values{"loaded_username": {" bob "}, "username": {" bob "}})
	assertBody(t, saveUser(form), ">Nothing to save<")
	seedRoleEditor(f)
	rec := action(http.MethodPost, "/admin/roles/"+idBee, url.Values{
		"loaded_name": {" bee_admin "}, "name": {" bee_admin "},
		"loaded_description": {" padded "}, "description": {" padded "},
	})
	assertBody(t, rec, ">Nothing to save<")
	assertWrites(t, f)
}

func TestAdminReloadFailureAfterAWriteIsNotReportedAsAFailedWrite(t *testing.T) {
	const message = "The change was made, but the page could not be refreshed: the API is down"
	cases := []struct {
		name   string
		reload string
		req    func() *httptest.ResponseRecorder
		seed   func(f *fakeAdmin)
	}{
		{"role save", "GET /roles/" + idBee, func() *httptest.ResponseRecorder {
			return action(http.MethodPost, "/admin/roles/"+idBee, url.Values{"loaded_name": {"bee_admin"}, "name": {"renamed"}})
		}, func(f *fakeAdmin) { f.on("PATCH /roles/"+idBee, 200, `{}`) }},
		{"grant", "GET /roles/" + idBee, func() *httptest.ResponseRecorder {
			return action(http.MethodPost, "/admin/roles/"+idBee+"/permissions", url.Values{"grant_permission": {idPStore}})
		}, func(f *fakeAdmin) {
			f.on("GET /permissions", 200, permissionsJSON)
			f.on("PUT /roles/"+idBee+"/permissions/"+idPStore, 204, ``)
		}},
		{"value", "GET /roles/" + idBee, func() *httptest.ResponseRecorder {
			return putRoleValue(idPRate, url.Values{"value_" + idPRate: {"5"}})
		}, func(f *fakeAdmin) {
			f.on("GET /permissions", 200, permissionsJSON)
			f.on("PUT /roles/"+idBee+"/permissions/"+idPRate, 204, ``)
		}},
		{"remove", "GET /roles/" + idBee, func() *httptest.ResponseRecorder {
			return action(http.MethodPost, "/admin/roles/"+idBee+"/permissions/"+idPBee+"/remove", nil)
		}, func(f *fakeAdmin) { f.on("DELETE /roles/"+idBee+"/permissions/"+idPBee, 204, ``) }},
		{"permission create", "GET /permissions", func() *httptest.ResponseRecorder {
			return action(http.MethodPost, "/admin/permissions", url.Values{"node": {"pets.write"}})
		}, func(f *fakeAdmin) { f.on("POST /permissions", 201, `{}`) }},
		{"permission delete", "GET /permissions", func() *httptest.ResponseRecorder {
			return action(http.MethodDelete, "/admin/permissions/"+idPStore, nil)
		}, func(f *fakeAdmin) { f.on("DELETE /permissions/"+idPStore, 204, ``) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeAdmin(t)
			tc.seed(f)
			f.problem(tc.reload, 500, "the API is down")
			rec := tc.req()
			assertStatus(t, rec, http.StatusInternalServerError)
			if got := bannerText(rec); got != message {
				t.Errorf("body = %q, want %q", got, message)
			}
			if len(f.writes()) != 1 {
				t.Errorf("API writes = %q, want one", f.writes())
			}
		})
	}
}

func TestAdminReloadFailureAfterAWriteStillSignsOutAnExpiredSession(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("DELETE /permissions/"+idPStore, 204, ``)
	f.problem("GET /permissions", 401, "expired")
	rec := action(http.MethodDelete, "/admin/permissions/"+idPStore, nil)
	if got := rec.Header().Get("HX-Redirect"); got != "/login" {
		t.Errorf("HX-Redirect = %q", got)
	}
}

func TestAdminPathsEscapeIDsThatLookLikeURLs(t *testing.T) {
	const id = "a/b?c#d%e"
	const escaped = "a%2Fb%3Fc%23d%25e"
	f := newFakeAdmin(t)
	f.on("PUT /users/"+escaped, 200, `{"user_id":"x","roles":[]}`)
	f.on("GET /users/"+escaped+"/links", 200, `[]`)
	f.on("GET /users/"+escaped+"/permissions", 200, `[]`)
	f.on("GET /roles", 200, rolesJSON)
	f.on("POST /roles", 201, `{"id":"`+id+`"}`)
	f.on("DELETE /roles/"+escaped+"/permissions/"+escaped, 204, ``)
	f.on("GET /roles/"+escaped, 200, `{"id":"`+id+`","name":"n","permissions":[]}`)
	f.on("GET /permissions", 200, `[]`)

	action(http.MethodPost, "/admin/users/"+escaped, url.Values{"username": {"robert"}, "loaded_username": {"bob"}})
	action(http.MethodPost, "/admin/roles/"+escaped+"/permissions/"+escaped+"/remove", nil)
	rec := action(http.MethodPost, "/admin/roles", url.Values{"name": {"n"}})
	if got := rec.Header().Get("HX-Redirect"); got != "/admin/roles/"+escaped {
		t.Errorf("HX-Redirect = %q", got)
	}
	assertWrites(t, f,
		`PUT /users/`+escaped+` {"username":"robert"}`,
		`DELETE /roles/`+escaped+`/permissions/`+escaped+` `,
		`POST /roles {"description":"","name":"n"}`,
	)
}

func TestAdminHTMXIsLoadedFromTheCDNWithHardenedConfig(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("GET /users/me/permissions", 200, `["users.admin"]`)
	rec := getPage("/admin")
	assertBody(t, rec,
		`<script src="https://cdn.neuralnexus.dev/htmx/htmx.v1.9.5.min.js" defer>`,
		`&#34;allowEval&#34;:false`, `&#34;allowScriptTags&#34;:false`, `&#34;selfRequestsOnly&#34;:true`,
	)
}

func TestAccountAdminLinkShowsForEitherAdminPermission(t *testing.T) {
	for _, permissions := range []string{`["users.admin"]`, `["roles.admin:1"]`, `["ratelimit:5","users.admin"]`} {
		t.Run(permissions, func(t *testing.T) {
			f := newFakeAdmin(t)
			f.on("GET /users/me/permissions", 200, permissions)
			rec := getPage("/account/admin-link")
			assertStatus(t, rec, http.StatusOK)
			assertBody(t, rec, `id="admin-dashboard-link"`, `href="/admin"`)
		})
	}
}

func TestAccountAdminLinkIsEmptyWithoutAnAdminPermission(t *testing.T) {
	for _, permissions := range []string{`[]`, `["ratelimit:1000"]`, `["beenamegenerator.admin"]`, `["users.administrator"]`, `["xusers.admin"]`} {
		t.Run(permissions, func(t *testing.T) {
			f := newFakeAdmin(t)
			f.on("GET /users/me/permissions", 200, permissions)
			rec := getPage("/account/admin-link")
			assertStatus(t, rec, http.StatusOK)
			if rec.Body.Len() != 0 {
				t.Errorf("body = %q, want empty", rec.Body.String())
			}
		})
	}
}

func TestAccountAdminLinkIsEmptyWhenThePermissionsCannotBeRead(t *testing.T) {
	f := newFakeAdmin(t)
	f.problem("GET /users/me/permissions", 401, "expired")
	rec := getPage("/account/admin-link")
	assertStatus(t, rec, http.StatusOK)
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want empty", rec.Body.String())
	}
}

func TestAccountContentLoadsTheAdminLinkThroughHTMX(t *testing.T) {
	f := newFakeAdmin(t)
	seedAccount(f)
	rec := getPage("/account/content")
	assertBody(t, rec, `hx-get="/account/admin-link"`, `hx-trigger="load"`, `hx-swap="outerHTML"`)
	assertNoBody(t, rec, "showAdminDashboardLink", `id="admin-dashboard-link"`)
}

func TestAdminFragmentsEmptyTheBannerOutOfBand(t *testing.T) {
	const clear = `<div id="admin-error" hx-swap-oob="innerHTML"></div>`
	f := newFakeAdmin(t)
	seedUserEditor(f)
	seedRoleEditor(f)
	f.on("PUT /users/"+idBob, 200, bobJSON)
	f.on("PUT /roles/"+idBee+"/permissions/"+idPStore, 204, ``)
	f.on("DELETE /permissions/"+idPStore, 204, ``)
	f.on("GET /users", 200, usersJSON)
	f.on("GET /roles", 200, rolesJSON)
	cases := map[string]*httptest.ResponseRecorder{
		"user save":   saveUser(userForm(url.Values{"username": {"robert"}})),
		"grant":       action(http.MethodPost, "/admin/roles/"+idBee+"/permissions", url.Values{"grant_permission": {idPStore}}),
		"grant value": adminReq{method: http.MethodGet, target: "/admin/roles/" + idBee + "/grant-value?grant_permission=" + idPMotd, htmx: true}.do(),
		"user rows":   adminReq{method: http.MethodGet, target: "/admin/users/rows", htmx: true}.do(),
	}
	f.on("GET /permissions", 200, permissionsJSON)
	cases["permission delete"] = action(http.MethodDelete, "/admin/permissions/"+idPStore, nil)
	for name, rec := range cases {
		t.Run(name, func(t *testing.T) {
			assertStatus(t, rec, http.StatusOK)
			assertBody(t, rec, clear)
		})
	}
}

func TestAdminErrorsEmptyTheStatusLineOfTheEditorTheyCameFrom(t *testing.T) {
	cases := []struct {
		name, method, target, route, want string
	}{
		{"user", http.MethodPost, "/admin/users/" + idBob, "PUT /users/" + idBob, `<p id="admin-user-status" hx-swap-oob="innerHTML"></p>`},
		{"role", http.MethodDelete, "/admin/roles/" + idBee, "DELETE /roles/" + idBee, `<p id="admin-role-status" hx-swap-oob="innerHTML"></p>`},
		{"user editor", http.MethodGet, "/admin/users/" + idBob + "/editor", "GET /users/" + idBob, `<p id="admin-user-status" hx-swap-oob="innerHTML"></p>`},
		{"role editor", http.MethodGet, "/admin/roles/" + idBee + "/editor", "GET /roles/" + idBee, `<p id="admin-role-status" hx-swap-oob="innerHTML"></p>`},
		{"user list", http.MethodGet, "/admin/users/list", "GET /users", ""},
		{"role list", http.MethodGet, "/admin/roles/list", "GET /roles", ""},
		{"user rows", http.MethodGet, "/admin/users/rows", "GET /users", ""},
		{"permissions", http.MethodDelete, "/admin/permissions/" + idPBee, "DELETE /permissions/" + idPBee, ""},
		{"role create", http.MethodPost, "/admin/roles", "POST /roles", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeAdmin(t)
			f.problem(tc.route, 409, "refused")
			form := url.Values{"username": {"robert"}, "loaded_username": {"bob"}, "name": {"x"}}
			rec := action(tc.method, tc.target, form)
			assertStatus(t, rec, http.StatusConflict)
			if tc.want == "" {
				if got := rec.Body.String(); got != "refused" {
					t.Errorf("body = %q, want just the message", got)
				}
				return
			}
			assertBody(t, rec, "refused"+tc.want)
		})
	}
}

func TestAdminActionsRefuseAFormThatCannotBeRead(t *testing.T) {
	f := newFakeAdmin(t)
	targets := []string{
		"/admin/users/" + idBob,
		"/admin/roles",
		"/admin/roles/" + idBee,
		"/admin/roles/" + idBee + "/permissions",
		"/admin/roles/" + idBee + "/permissions/" + idPRate,
		"/admin/roles/" + idBee + "/permissions/" + idPRate + "/remove",
		"/admin/permissions",
	}
	for _, target := range targets {
		t.Run(target, func(t *testing.T) {
			f.reset()
			req := httptest.NewRequest(http.MethodPost, target, strings.NewReader("name=%zz"))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("HX-Request", "true")
			rec := httptest.NewRecorder()
			NewWebServer("", false).Setup().ServeHTTP(rec, req)
			assertStatus(t, rec, http.StatusBadRequest)
			if got := bannerText(rec); got != "The form could not be read" {
				t.Errorf("body = %q", got)
			}
			if len(f.uris()) != 0 {
				t.Errorf("an unreadable form reached the API: %v", f.uris())
			}
		})
	}
}

func TestBeeAdminLinkShowsForTheBeeAdminPermission(t *testing.T) {
	for _, permissions := range []string{`["beenamegenerator.admin"]`, `["ratelimit:5","beenamegenerator.admin:1"]`} {
		t.Run(permissions, func(t *testing.T) {
			f := newFakeAdmin(t)
			f.on("GET /users/me/permissions", 200, permissions)
			rec := getPage("/project/bee-name-generator/admin-link")
			assertStatus(t, rec, http.StatusOK)
			assertBody(t, rec, `id="bee-admin-link"`, `href="/project/bee-name-generator/admin"`)
		})
	}
}

func TestBeeAdminLinkIsEmptyWithoutTheBeeAdminPermission(t *testing.T) {
	for _, permissions := range []string{`[]`, `["users.admin","roles.admin"]`, `["beenamegenerator|*"]`, `["beenamegenerator.administrator"]`, `["xbeenamegenerator.admin"]`} {
		t.Run(permissions, func(t *testing.T) {
			f := newFakeAdmin(t)
			f.on("GET /users/me/permissions", 200, permissions)
			rec := getPage("/project/bee-name-generator/admin-link")
			assertStatus(t, rec, http.StatusOK)
			if rec.Body.Len() != 0 {
				t.Errorf("body = %q, want empty", rec.Body.String())
			}
		})
	}
}

func TestBeeAdminLinkIsEmptyWhenThePermissionsCannotBeRead(t *testing.T) {
	f := newFakeAdmin(t)
	f.problem("GET /users/me/permissions", 500, "down")
	rec := getPage("/project/bee-name-generator/admin-link")
	assertStatus(t, rec, http.StatusOK)
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want empty", rec.Body.String())
	}
}

func TestBeeNameGeneratorPageLoadsTheAdminLinkThroughHTMX(t *testing.T) {
	rec := getPage("/project/bee-name-generator")
	assertBody(t, rec, `hx-get="/project/bee-name-generator/admin-link"`, `hx-trigger="load"`, "htmx.v1.9.5.min.js")
	assertNoBody(t, rec, "showBeeAdminLink", `id="bee-admin-link"`)
}
