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

func TestAdminPagesAreNotCached(t *testing.T) {
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
		{"/admin", `id="admin-users-link"`},
		{"/admin/users", `id="admin-users-search"`},
		{"/admin/users/" + idBob, `id="admin-user-form"`},
		{"/admin/roles", `id="admin-role-create-form"`},
		{"/admin/roles/" + idBee, `id="admin-role"`},
		{"/admin/permissions", `id="admin-permission-create-form"`},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			rec := getPage(tc.path)
			assertStatus(t, rec, http.StatusOK)
			if got := rec.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q, want %q", got, "no-store")
			}
			assertBody(t, rec, tc.marker, `<script src="https://cdn.neuralnexus.dev/htmx/htmx.v1.9.5.min.js" defer>`, `&#34;selfRequestsOnly&#34;:true`, `&#34;allowEval&#34;:false`)
		})
	}
}

func TestAdminErrorPagesAreNotCached(t *testing.T) {
	f := newFakeAdmin(t)
	f.problem("GET /users/me/permissions", 500, "boom")
	rec := getPage("/admin")
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want %q", got, "no-store")
	}
}

func TestOtherPagesKeepTheirCaching(t *testing.T) {
	for _, path := range []string{"/", "/account", "/projects"} {
		rec := getPage(path)
		if got := rec.Header().Get("Cache-Control"); got != "" {
			t.Errorf("%s: Cache-Control = %q, want none", path, got)
		}
		if strings.Contains(rec.Body.String(), "htmx") {
			t.Errorf("%s should not load htmx", path)
		}
	}
}

func TestAdminForwardsOnlyTheSessionCookie(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("GET /users/me/permissions", 200, `["users.admin"]`)
	adminReq{method: http.MethodGet, target: "/admin", cookies: []*http.Cookie{
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
	for _, path := range []string{"/admin", "/admin/users", "/admin/users/" + idBob, "/admin/roles", "/admin/roles/" + idBee, "/admin/permissions"} {
		t.Run("page "+path, func(t *testing.T) {
			rec := getPage(path)
			assertStatus(t, rec, http.StatusSeeOther)
			if got := rec.Header().Get("Location"); got != "/login" {
				t.Errorf("Location = %q, want /login", got)
			}
		})
	}
	for _, target := range []string{"/admin/users/rows?offset=200", "/admin/roles/" + idBee + "/grant-value?grant_permission=" + idPRate} {
		t.Run("fragment "+target, func(t *testing.T) {
			rec := adminReq{method: http.MethodGet, target: target, htmx: true}.do()
			if got := rec.Header().Get("HX-Redirect"); got != "/login" {
				t.Errorf("HX-Redirect = %q, want /login", got)
			}
		})
	}
	rec := action(http.MethodPost, "/admin/roles", url.Values{"name": {"x"}})
	if got := rec.Header().Get("HX-Redirect"); got != "/login" {
		t.Errorf("HX-Redirect = %q, want /login", got)
	}
}

func TestAdminRefusalsShowTheAPIMessage(t *testing.T) {
	f := newFakeAdmin(t)
	f.problem("GET /users/me/permissions", 403, "You may not")
	rec := getPage("/admin")
	assertStatus(t, rec, http.StatusForbidden)
	assertBody(t, rec, "You may not")

	f.problem("POST /roles", 409, "A role with that name already exists")
	rec = action(http.MethodPost, "/admin/roles", url.Values{"name": {"system"}})
	assertStatus(t, rec, http.StatusConflict)
	if got := rec.Header().Get("HX-Retarget"); got != "#admin-error" {
		t.Errorf("HX-Retarget = %q", got)
	}
	if got := rec.Header().Get("HX-Reswap"); got != "innerHTML" {
		t.Errorf("HX-Reswap = %q", got)
	}
	if got := rec.Body.String(); got != "A role with that name already exists" {
		t.Errorf("body = %q", got)
	}
}

func TestAdminErrorMessagesAreEscaped(t *testing.T) {
	f := newFakeAdmin(t)
	f.problem("GET /users/me/permissions", 500, hostile)
	rec := getPage("/admin")
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
	rec := getPage("/admin")
	assertStatus(t, rec, http.StatusBadGateway)
	assertBody(t, rec, "Failed to load your permissions")
	rec = action(http.MethodPost, "/admin/roles", url.Values{"name": {"x"}})
	assertStatus(t, rec, http.StatusBadGateway)
	if got := rec.Body.String(); got != "Failed to create the role" {
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
			rec := getPage("/admin")
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

func TestAdminUserListPagesAndNamesRoles(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("GET /users", 200, usersJSON)
	f.on("GET /roles", 200, rolesJSON)
	rec := getPage("/admin/users")
	assertStatus(t, rec, http.StatusOK)
	assertBody(t, rec,
		`id="admin-users-search"`,
		`href="/admin/users/`+idAlice+`"`,
		`data-name="alice"`, `data-id="`+idBob+`"`,
		">bee_admin<", ">system<", "No username",
	)
	assertNoBody(t, rec, `id="admin-users-more"`)
	if got := f.uris(); got[0] != "GET /users?limit=200&offset=0" {
		t.Errorf("first call = %q", got[0])
	}
}

func fullPage(n int) string {
	var users []string
	for i := 0; i < n; i++ {
		users = append(users, fmt.Sprintf(`{"user_id":"%d","username":"user%d","roles":[]}`, 354102516314670000+i, i))
	}
	return "[" + strings.Join(users, ",") + "]"
}

func TestAdminUserListOffersMoreOnlyAfterAFullPage(t *testing.T) {
	cases := []struct {
		rows     int
		wantMore bool
	}{{0, false}, {1, false}, {150, false}, {199, false}, {200, true}}
	for _, tc := range cases {
		t.Run(fmt.Sprint(tc.rows), func(t *testing.T) {
			f := newFakeAdmin(t)
			f.on("GET /users", 200, fullPage(tc.rows))
			f.on("GET /roles", 200, `[]`)
			rec := getPage("/admin/users")
			if got := strings.Contains(rec.Body.String(), `id="admin-users-more"`); got != tc.wantMore {
				t.Errorf("load more present = %v, want %v", got, tc.wantMore)
			}
			if tc.wantMore {
				assertBody(t, rec, `hx-get="/admin/users/rows?offset=200"`)
			}
			if got := strings.Contains(rec.Body.String(), `id="admin-users-empty" hidden`); got != (tc.rows > 0) {
				t.Errorf("the empty notice is hidden = %v with %d rows", got, tc.rows)
			}
		})
	}
}

func TestAdminUserRowsContinueFromTheOffset(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("GET /users", 200, fullPage(200))
	f.on("GET /roles", 200, `[]`)
	rec := adminReq{method: http.MethodGet, target: "/admin/users/rows?offset=400", htmx: true}.do()
	assertStatus(t, rec, http.StatusOK)
	assertBody(t, rec, `hx-get="/admin/users/rows?offset=600"`)
	assertNoBody(t, rec, "<html", "<body")
	if got := f.uris()[0]; got != "GET /users?limit=200&offset=400" {
		t.Errorf("first call = %q", got)
	}
}

func TestAdminUserRowsRejectABadOffset(t *testing.T) {
	f := newFakeAdmin(t)
	for _, offset := range []string{"", "-1", "x", "1.5", "99999999999999999999"} {
		t.Run(offset, func(t *testing.T) {
			f.reset()
			rec := adminReq{method: http.MethodGet, target: "/admin/users/rows?offset=" + offset, htmx: true}.do()
			assertStatus(t, rec, http.StatusBadRequest)
			if len(f.uris()) != 0 {
				t.Errorf("a bad offset reached the API: %v", f.uris())
			}
		})
	}
}

func TestAdminUserListShowsRoleIDsWithoutRoleAccess(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("GET /users", 200, usersJSON)
	f.problem("GET /roles", 403, "forbidden")
	rec := getPage("/admin/users")
	assertStatus(t, rec, http.StatusOK)
	assertBody(t, rec, ">"+idBee+"<")
}

func TestAdminUserListFailedRolesLookupIsAnError(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("GET /users", 200, usersJSON)
	f.problem("GET /roles", 500, "roles are down")
	rec := getPage("/admin/users")
	assertStatus(t, rec, http.StatusInternalServerError)
	assertBody(t, rec, "roles are down")
	assertNoBody(t, rec, `id="admin-users-search"`)
}

func TestAdminUserListRefusedHidesTheSearch(t *testing.T) {
	f := newFakeAdmin(t)
	f.problem("GET /users", 403, "You do not have permission to list users")
	rec := getPage("/admin/users")
	assertStatus(t, rec, http.StatusForbidden)
	assertBody(t, rec, "You do not have permission to list users")
	assertNoBody(t, rec, `id="admin-users-search"`, `id="admin-users"`)
}

func TestAdminUserListEscapesAPIText(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("GET /users", 200, fmt.Sprintf(`[{"user_id":%q,"username":%q,"roles":[%q]}]`, hostile+"id", hostile+"name", idBee))
	f.on("GET /roles", 200, fmt.Sprintf(`[{"id":%q,"name":%q,"description":"d","permissions":[]}]`, idBee, hostile+"role"))
	rec := getPage("/admin/users")
	assertNoBody(t, rec, "<img src=x")
	assertBody(t, rec, "&lt;img src=x", "role", "%3Cimg")
}

const bobJSON = `{"user_id":"` + idBob + `","username":"bob","roles":["` + idBee + `"]}`

func seedUserEditor(f *fakeAdmin) {
	f.on("GET /users/"+idBob, 200, bobJSON)
	f.on("GET /users/"+idBob+"/links", 200, `[{"platform":"discord","platform_username":"bob#1234","platform_id":"9"},{"platform":"steam","platform_id":"76561198000000000"}]`)
	f.on("GET /users/"+idBob+"/permissions", 200, `["beenamegenerator.admin","ratelimit:100"]`)
	f.on("GET /roles", 200, rolesJSON)
}

func TestAdminUserEditorShowsTheAccount(t *testing.T) {
	f := newFakeAdmin(t)
	seedUserEditor(f)
	rec := getPage("/admin/users/" + idBob)
	assertStatus(t, rec, http.StatusOK)
	assertBody(t, rec,
		`id="admin-user-title"`, ">bob<", ">"+idBob+"<",
		`name="loaded_username" value="bob"`,
		`name="loaded_roles" value="`+idBee+`"`,
		`name="roles_editable" value="1"`,
		`name="roles" value="`+idSystem+`"`,
		`name="roles" value="`+idBee+`" checked`,
		"Bee Name Generator Admin",
		"bob#1234", "76561198000000000",
		">beenamegenerator.admin<", ">ratelimit:100<",
		`hx-post="/admin/users/`+idBob+`"`,
	)
	assertNoBody(t, rec, `id="admin-user-roles-note"`, `id="admin-user-links-error"`, `id="admin-user-permissions-error"`)
	if strings.Contains(rec.Body.String(), `name="roles" value="`+idSystem+`" checked`) {
		t.Error("a role the user does not hold is ticked")
	}
}

func TestAdminUserEditorShowsEmptyStates(t *testing.T) {
	f := newFakeAdmin(t)
	seedUserEditor(f)
	f.on("GET /users/"+idBob+"/links", 200, `[]`)
	f.on("GET /users/"+idBob+"/permissions", 200, `[]`)
	rec := getPage("/admin/users/" + idBob)
	assertBody(t, rec, `id="admin-user-links-empty"`, `id="admin-user-permissions-empty"`)
}

func TestAdminUserEditorShowsRefusedSectionsAsErrors(t *testing.T) {
	f := newFakeAdmin(t)
	seedUserEditor(f)
	f.problem("GET /users/"+idBob+"/links", 403, "links are off limits")
	f.problem("GET /users/"+idBob+"/permissions", 500, "permissions are down")
	rec := getPage("/admin/users/" + idBob)
	assertStatus(t, rec, http.StatusOK)
	assertBody(t, rec, `id="admin-user-links-error"`, "links are off limits", `id="admin-user-permissions-error"`, "permissions are down", `id="admin-user-form"`)
	assertNoBody(t, rec, `id="admin-user-links-empty"`, `id="admin-user-permissions-empty"`)
}

func TestAdminUserEditorWithoutRoleAccessIsReadOnly(t *testing.T) {
	f := newFakeAdmin(t)
	seedUserEditor(f)
	f.problem("GET /roles", 403, "forbidden")
	rec := getPage("/admin/users/" + idBob)
	assertStatus(t, rec, http.StatusOK)
	assertBody(t, rec, `id="admin-user-roles-note"`, ">"+idBee+"<")
	assertNoBody(t, rec, `name="roles_editable"`, `name="roles"`)
}

func TestAdminUserEditorFailedRolesLookupIsAnError(t *testing.T) {
	f := newFakeAdmin(t)
	seedUserEditor(f)
	f.problem("GET /roles", 500, "roles are down")
	rec := getPage("/admin/users/" + idBob)
	assertStatus(t, rec, http.StatusInternalServerError)
	assertBody(t, rec, "roles are down")
	assertNoBody(t, rec, `id="admin-user-form"`, `id="admin-user-roles-note"`)
}

func TestAdminUserEditorUnknownUserShowsTheMessageAndNothingFromTheURL(t *testing.T) {
	f := newFakeAdmin(t)
	f.problem("GET /users/999", 404, "User not found")
	rec := getPage("/admin/users/999")
	assertStatus(t, rec, http.StatusNotFound)
	assertBody(t, rec, "User not found")
	assertNoBody(t, rec, `id="admin-user-form"`, `id="admin-user-id"`, ">999<")
}

func TestAdminUserEditorEscapesIDsAndAPIText(t *testing.T) {
	id := hostile + "id"
	f := newFakeAdmin(t)
	escaped := url.PathEscape(id)
	f.on("GET /users/"+escaped, 200, fmt.Sprintf(`{"user_id":%q,"username":%q,"roles":[%q,"gone"]}`, id, hostile+"name", idBee))
	f.on("GET /users/"+escaped+"/links", 200, fmt.Sprintf(`[{"platform":%q,"platform_username":%q,"platform_id":%q}]`, hostile+"platform", hostile+"user", hostile+"pid"))
	f.on("GET /users/"+escaped+"/permissions", 200, fmt.Sprintf(`[%q]`, hostile+"permission"))
	f.on("GET /roles", 200, fmt.Sprintf(`[{"id":%q,"name":%q,"description":%q,"permissions":[]}]`, idBee, hostile+"role", hostile+"description"))
	rec := getPage("/admin/users/" + escaped)
	assertStatus(t, rec, http.StatusOK)
	assertNoBody(t, rec, "<img src=x")
	assertBody(t, rec, "&lt;img src=x", "platform", "permission", "role", "description")

	f.problem("GET /users/"+escaped+"/links", 500, hostile+"links detail")
	rec = getPage("/admin/users/" + escaped)
	assertNoBody(t, rec, "<img src=x")

	f.problem("GET /roles", 403, "forbidden")
	rec = getPage("/admin/users/" + escaped)
	assertNoBody(t, rec, "<img src=x")
}

func saveUser(form url.Values) *httptest.ResponseRecorder {
	return action(http.MethodPost, "/admin/users/"+idBob, form)
}

func userForm(extra url.Values) url.Values {
	form := url.Values{
		"loaded_username": {"bob"},
		"loaded_roles":    {idBee},
		"roles_editable":  {"1"},
		"username":        {"bob"},
		"roles":           {idBee},
	}
	for key, values := range extra {
		form[key] = values
	}
	return form
}

func TestAdminUserSaveSendsOnlyWhatChanged(t *testing.T) {
	cases := []struct {
		name  string
		extra url.Values
		want  string
	}{
		{"username", url.Values{"username": {"  robert  "}}, `PUT /users/` + idBob + ` {"username":"robert"}`},
		{"roles", url.Values{"roles": {idSystem, idBee}}, `PUT /users/` + idBob + ` {"roles":["` + idSystem + `","` + idBee + `"]}`},
		{"both", url.Values{"username": {"robert"}, "roles": {idSystem}}, `PUT /users/` + idBob + ` {"roles":["` + idSystem + `"],"username":"robert"}`},
		{"no roles at all", url.Values{"roles": nil}, `PUT /users/` + idBob + ` {"roles":[]}`},
		{"swapping a role", url.Values{"roles": {idSystem}}, `PUT /users/` + idBob + ` {"roles":["` + idSystem + `"]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeAdmin(t)
			seedUserEditor(f)
			f.on("PUT /users/"+idBob, 200, bobJSON)
			form := userForm(tc.extra)
			if tc.name == "no roles at all" {
				form.Del("roles")
			}
			rec := saveUser(form)
			assertStatus(t, rec, http.StatusOK)
			assertWrites(t, f, tc.want)
			assertBody(t, rec, `id="admin-user"`, `id="admin-user-status" hx-swap-oob="innerHTML">Saved<`)
		})
	}
}

func TestAdminUserSaveWithoutRoleAccessLeavesRolesOut(t *testing.T) {
	f := newFakeAdmin(t)
	seedUserEditor(f)
	f.on("PUT /users/"+idBob, 200, bobJSON)
	form := userForm(url.Values{"username": {"robert"}})
	form.Del("roles_editable")
	form.Del("roles")
	saveUser(form)
	assertWrites(t, f, `PUT /users/`+idBob+` {"username":"robert"}`)
}

func TestAdminUserSaveWithoutAChangeSendsNothing(t *testing.T) {
	f := newFakeAdmin(t)
	seedUserEditor(f)
	rec := saveUser(userForm(nil))
	assertStatus(t, rec, http.StatusOK)
	assertWrites(t, f)
	assertBody(t, rec, `id="admin-user-status" hx-swap-oob="innerHTML">Nothing to save<`)
}

func TestAdminUserSaveRefusesAnEmptiedUsername(t *testing.T) {
	f := newFakeAdmin(t)
	seedUserEditor(f)
	rec := saveUser(userForm(url.Values{"username": {"   "}}))
	assertStatus(t, rec, http.StatusBadRequest)
	if got := rec.Body.String(); got != "Enter a username" {
		t.Errorf("body = %q", got)
	}
	assertWrites(t, f)
}

func TestAdminUserSaveKeepsAnAccountWithoutAUsernameThatWay(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("GET /users/"+idAnon, 200, `{"user_id":"`+idAnon+`","username":"","roles":[]}`)
	f.on("GET /users/"+idAnon+"/links", 200, `[]`)
	f.on("GET /users/"+idAnon+"/permissions", 200, `[]`)
	f.on("GET /roles", 200, rolesJSON)
	f.on("PUT /users/"+idAnon, 200, `{"user_id":"`+idAnon+`","username":"","roles":["`+idSystem+`"]}`)
	rec := action(http.MethodPost, "/admin/users/"+idAnon, url.Values{"loaded_username": {""}, "username": {""}, "roles_editable": {"1"}, "roles": {idSystem}})
	assertStatus(t, rec, http.StatusOK)
	assertWrites(t, f, `PUT /users/`+idAnon+` {"roles":["`+idSystem+`"]}`)
}

func TestAdminUserSaveRefreshesThePermissions(t *testing.T) {
	f := newFakeAdmin(t)
	seedUserEditor(f)
	f.on("PUT /users/"+idBob, 200, bobJSON)
	f.on("GET /users/"+idBob+"/permissions", 200, `["users.admin"]`)
	rec := saveUser(userForm(url.Values{"username": {"robert"}}))
	assertBody(t, rec, ">users.admin<")
	uris := f.uris()
	if uris[0] != "PUT /users/"+idBob {
		t.Errorf("the save should come first: %v", uris)
	}
}

func TestAdminUserSaveSaysWhenThePermissionsCouldNotBeRefreshed(t *testing.T) {
	f := newFakeAdmin(t)
	seedUserEditor(f)
	f.on("PUT /users/"+idBob, 200, bobJSON)
	f.problem("GET /users/"+idBob+"/permissions", 500, "permissions are down")
	rec := saveUser(userForm(url.Values{"username": {"robert"}}))
	assertStatus(t, rec, http.StatusOK)
	assertBody(t, rec, `id="admin-user-permissions-error"`, "permissions are down", ">Saved, but the permissions below are out of date<")
	assertNoBody(t, rec, ">Saved<")
}

func TestAdminUserSaveRefusedShowsTheAPIMessage(t *testing.T) {
	f := newFakeAdmin(t)
	seedUserEditor(f)
	f.problem("PUT /users/"+idBob, 409, "An account with this username already exists")
	rec := saveUser(userForm(url.Values{"username": {"alice"}}))
	assertStatus(t, rec, http.StatusConflict)
	if got := rec.Body.String(); got != "An account with this username already exists" {
		t.Errorf("body = %q", got)
	}
	assertNoBody(t, rec, "Saved")
}

func TestAdminUserSaveReloadFailureIsNotReportedAsAFailedSave(t *testing.T) {
	f := newFakeAdmin(t)
	seedUserEditor(f)
	f.on("PUT /users/"+idBob, 200, bobJSON)
	f.problem("GET /roles", 500, "roles are down")
	rec := saveUser(userForm(url.Values{"username": {"robert"}}))
	assertStatus(t, rec, http.StatusInternalServerError)
	if got := rec.Body.String(); got != "The change was made, but the page could not be refreshed: roles are down" {
		t.Errorf("body = %q", got)
	}
}

func TestAdminUserSaveEscapesAPIText(t *testing.T) {
	f := newFakeAdmin(t)
	seedUserEditor(f)
	f.on("PUT /users/"+idBob, 200, fmt.Sprintf(`{"user_id":%q,"username":%q,"roles":[]}`, idBob, hostile))
	rec := saveUser(userForm(url.Values{"username": {hostile}}))
	assertNoBody(t, rec, "<img src=x")
}

func TestAdminRoleListShowsRolesWithTheirPermissions(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("GET /roles", 200, `[
		{"id":"`+idBee+`","name":"bee_admin","description":"Bee Name Generator Admin","permissions":[
			{"id":"`+idPBee+`","node":"beenamegenerator.admin","description":"d"},
			{"id":"`+idPRate+`","node":"ratelimit","description":"d","value_type":"int","merge":"max","value":100},
			{"id":"`+idPPets+`","node":"petpictures.pets","description":"d","value_type":"string_list","merge":"union","value":["rex","fido"]}
		]}]`)
	rec := getPage("/admin/roles")
	assertStatus(t, rec, http.StatusOK)
	assertBody(t, rec, `href="/admin/roles/`+idBee+`"`, "bee_admin", "Bee Name Generator Admin",
		">beenamegenerator.admin<", ">ratelimit: 100<", ">petpictures.pets: rex, fido<",
		`id="admin-role-create-form"`, `hx-post="/admin/roles"`)
	assertBody(t, rec, `id="admin-roles-empty" hidden`)
}

func TestAdminRoleListRefusedHidesTheCreateForm(t *testing.T) {
	f := newFakeAdmin(t)
	f.problem("GET /roles", 403, "You do not have permission to manage roles and permissions")
	rec := getPage("/admin/roles")
	assertStatus(t, rec, http.StatusForbidden)
	assertBody(t, rec, "You do not have permission to manage roles and permissions")
	assertNoBody(t, rec, `id="admin-role-create-form"`)
}

func TestAdminRoleListEscapesAPIText(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("GET /roles", 200, fmt.Sprintf(`[{"id":%q,"name":%q,"description":%q,"permissions":[
		{"id":"1","node":%q,"description":"d"},
		{"id":"2","node":"ratelimit","description":"d","value_type":"int","value":%q},
		{"id":"3","node":"x","description":"d","value_type":"string_list","value":[%q]}]}]`,
		hostile+"id", hostile+"name", hostile+"description", hostile+"node", hostile+"number-looking", hostile+"item"))
	rec := getPage("/admin/roles")
	assertNoBody(t, rec, "<img src=x")
}

func TestAdminRoleCreatePostsTheTrimmedFieldsAndOpensTheEditor(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("POST /roles", 201, `{"id":"`+idBee+`","name":"moderator","description":"Moderates things","permissions":[]}`)
	rec := action(http.MethodPost, "/admin/roles", url.Values{"name": {" moderator "}, "description": {" Moderates things "}})
	assertStatus(t, rec, http.StatusOK)
	if got := rec.Header().Get("HX-Redirect"); got != "/admin/roles/"+idBee {
		t.Errorf("HX-Redirect = %q", got)
	}
	assertWrites(t, f, `POST /roles {"description":"Moderates things","name":"moderator"}`)
}

func TestAdminRoleCreateRefusedShowsTheAPIMessage(t *testing.T) {
	f := newFakeAdmin(t)
	f.problem("POST /roles", 409, "A role with that name already exists")
	rec := action(http.MethodPost, "/admin/roles", url.Values{"name": {"system"}})
	assertStatus(t, rec, http.StatusConflict)
	if got := rec.Header().Get("HX-Redirect"); got != "" {
		t.Errorf("HX-Redirect = %q, want none", got)
	}
}

func seedRoleEditor(f *fakeAdmin) {
	f.on("GET /roles/"+idBee, 200, `{"id":"`+idBee+`","name":"bee_admin","description":"Bee Name Generator Admin","permissions":[
		{"id":"`+idPBee+`","node":"beenamegenerator.admin","description":"Bee name generator"},
		{"id":"`+idPRate+`","node":"ratelimit","description":"Rate limit","value_type":"int","merge":"max","value":100}]}`)
	f.on("GET /permissions", 200, permissionsJSON)
}

func TestAdminRoleEditorShowsTheRole(t *testing.T) {
	f := newFakeAdmin(t)
	seedRoleEditor(f)
	rec := getPage("/admin/roles/" + idBee)
	assertStatus(t, rec, http.StatusOK)
	assertBody(t, rec,
		`id="admin-role-title"`, ">bee_admin<", ">"+idBee+"<",
		`name="loaded_name" value="bee_admin"`, `name="loaded_description" value="Bee Name Generator Admin"`,
		`hx-post="/admin/roles/`+idBee+`/permissions/`+idPRate+`/remove"`,
		`hx-post="/admin/roles/`+idBee+`/permissions/`+idPRate+`"`,
		`type="number"`, `value="100"`, `aria-label="Value of ratelimit"`,
		`hx-delete="/admin/roles/`+idBee+`"`, `hx-confirm="Delete the role bee_admin?"`,
		`hx-post="/admin/roles/`+idBee+`/permissions"`,
		`hx-get="/admin/roles/`+idBee+`/grant-value"`,
	)
	for _, node := range []string{"petpictures.pets", "motd", "datastore.admin"} {
		assertBody(t, rec, ">"+node+"</option>")
	}
	assertNoBody(t, rec, ">beenamegenerator.admin</option>", ">ratelimit</option>", `id="admin-role-grant-empty"`, "autofocus")
	assertBody(t, rec, `<option value="`+idPPets+`" selected>`)
}

func TestAdminRoleEditorWithoutPermissionsOrGrantsLeft(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("GET /roles/"+idSystem, 200, `{"id":"`+idSystem+`","name":"system","description":"System","permissions":[]}`)
	f.on("GET /permissions", 200, `[]`)
	rec := getPage("/admin/roles/" + idSystem)
	assertBody(t, rec, `id="admin-role-permissions-empty"`, `id="admin-role-grant-empty"`)
	assertNoBody(t, rec, `id="admin-role-grant-form"`)
}

func TestAdminRoleEditorUnknownRoleShowsTheMessageAndNothingFromTheURL(t *testing.T) {
	f := newFakeAdmin(t)
	f.problem("GET /roles/404", 404, "Role not found")
	rec := getPage("/admin/roles/404")
	assertStatus(t, rec, http.StatusNotFound)
	assertBody(t, rec, "Role not found")
	assertNoBody(t, rec, `id="admin-role-form"`, ">404<")
}

func TestAdminRoleEditorFailedPermissionLookupIsAnError(t *testing.T) {
	f := newFakeAdmin(t)
	seedRoleEditor(f)
	f.problem("GET /permissions", 500, "permissions are down")
	rec := getPage("/admin/roles/" + idBee)
	assertStatus(t, rec, http.StatusInternalServerError)
	assertBody(t, rec, "permissions are down")
	assertNoBody(t, rec, `id="admin-role-form"`)
}

func TestAdminRoleEditorEscapesIDsAndAPIText(t *testing.T) {
	id := hostile + "id"
	escaped := url.PathEscape(id)
	f := newFakeAdmin(t)
	f.on("GET /roles/"+escaped, 200, fmt.Sprintf(`{"id":%q,"name":%q,"description":%q,"permissions":[
		{"id":"1","node":%q,"description":%q},
		{"id":"2","node":"motd","description":"d","value_type":"string","value":%q},
		{"id":"3","node":"x","description":"d","value_type":"string_list","value":[%q]}]}`,
		id, hostile+"name", hostile+"description", hostile+"node", hostile+"node description", hostile+"value", hostile+"item"))
	f.on("GET /permissions", 200, fmt.Sprintf(`[{"id":"9","node":%q,"description":"d"}]`, hostile+"ungranted"))
	rec := getPage("/admin/roles/" + escaped)
	assertStatus(t, rec, http.StatusOK)
	assertNoBody(t, rec, "<img src=x")
	assertBody(t, rec, "&lt;img src=x")
}

func putRoleValue(permission string, form url.Values) *httptest.ResponseRecorder {
	return action(http.MethodPost, "/admin/roles/"+idBee+"/permissions/"+permission, form)
}

func TestAdminRoleGrantBuildsTheBodyFromTheTypedValue(t *testing.T) {
	cases := []struct {
		name       string
		permission string
		value      string
		wantWrite  string
	}{
		{"bare", idPStore, "ignored", `PUT /roles/` + idBee + `/permissions/` + idPStore + ` `},
		{"int", idPRate, " 250 ", `PUT /roles/` + idBee + `/permissions/` + idPRate + ` {"value":250}`},
		{"negative int", idPRate, "-5", `PUT /roles/` + idBee + `/permissions/` + idPRate + ` {"value":-5}`},
		{"largest int", idPRate, "9007199254740992", `PUT /roles/` + idBee + `/permissions/` + idPRate + ` {"value":9007199254740992}`},
		{"smallest int", idPRate, "-9007199254740992", `PUT /roles/` + idBee + `/permissions/` + idPRate + ` {"value":-9007199254740992}`},
		{"text", idPMotd, "  hello  ", `PUT /roles/` + idBee + `/permissions/` + idPMotd + ` {"value":"hello"}`},
		{"list", idPPets, "rex\r\n  fido  \n\n\nspot", `PUT /roles/` + idBee + `/permissions/` + idPPets + ` {"value":["rex","fido","spot"]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeAdmin(t)
			seedRoleEditor(f)
			f.on("PUT /roles/"+idBee+"/permissions/"+tc.permission, 204, ``)
			rec := action(http.MethodPost, "/admin/roles/"+idBee+"/permissions", url.Values{"grant_permission": {tc.permission}, "grant_value": {tc.value}})
			assertStatus(t, rec, http.StatusOK)
			assertWrites(t, f, tc.wantWrite)
			assertBody(t, rec, `id="admin-role-grants"`)
			assertNoBody(t, rec, "autofocus", `id="admin-role-form"`, "<html")
		})
	}
}

func TestAdminRoleGrantRefusesUnusableValues(t *testing.T) {
	cases := []struct {
		name, permission, value, message string
	}{
		{"empty int", idPRate, "", "Enter a whole number from -9007199254740992 to 9007199254740992"},
		{"fraction", idPRate, "1.5", "Enter a whole number from -9007199254740992 to 9007199254740992"},
		{"exponent", idPRate, "1e3", "Enter a whole number from -9007199254740992 to 9007199254740992"},
		{"leading zeros", idPRate, "007", "Enter a whole number from -9007199254740992 to 9007199254740992"},
		{"plus sign", idPRate, "+5", "Enter a whole number from -9007199254740992 to 9007199254740992"},
		{"2^53 plus one", idPRate, "9007199254740993", "Enter a whole number from -9007199254740992 to 9007199254740992"},
		{"below -2^53", idPRate, "-9007199254740993", "Enter a whole number from -9007199254740992 to 9007199254740992"},
		{"beyond int64", idPRate, "99999999999999999999", "Enter a whole number from -9007199254740992 to 9007199254740992"},
		{"empty list", idPPets, " \n  \n", "Enter at least one item"},
		{"empty text", idPMotd, "   ", "Enter a value"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeAdmin(t)
			seedRoleEditor(f)
			rec := action(http.MethodPost, "/admin/roles/"+idBee+"/permissions", url.Values{"grant_permission": {tc.permission}, "grant_value": {tc.value}})
			assertStatus(t, rec, http.StatusBadRequest)
			if got := rec.Body.String(); got != tc.message {
				t.Errorf("body = %q, want %q", got, tc.message)
			}
			assertWrites(t, f)
		})
	}
}

func TestAdminRoleGrantUnknownPermission(t *testing.T) {
	f := newFakeAdmin(t)
	seedRoleEditor(f)
	rec := action(http.MethodPost, "/admin/roles/"+idBee+"/permissions", url.Values{"grant_permission": {"123"}})
	assertStatus(t, rec, http.StatusNotFound)
	assertWrites(t, f)
}

func TestAdminRoleGrantRefusedShowsTheAPIMessage(t *testing.T) {
	f := newFakeAdmin(t)
	seedRoleEditor(f)
	f.problem("PUT /roles/"+idBee+"/permissions/"+idPStore, 400, "The value must match")
	rec := action(http.MethodPost, "/admin/roles/"+idBee+"/permissions", url.Values{"grant_permission": {idPStore}})
	assertStatus(t, rec, http.StatusBadRequest)
	if got := rec.Body.String(); got != "The value must match" {
		t.Errorf("body = %q", got)
	}
}

func TestAdminRoleSaveValueKeepsWhatWasTypedInTheGrantForm(t *testing.T) {
	f := newFakeAdmin(t)
	seedRoleEditor(f)
	f.on("PUT /roles/"+idBee+"/permissions/"+idPRate, 204, ``)
	rec := putRoleValue(idPRate, url.Values{"value_" + idPRate: {"250"}, "grant_permission": {idPMotd}, "grant_value": {"half typed"}})
	assertStatus(t, rec, http.StatusOK)
	assertWrites(t, f, `PUT /roles/`+idBee+`/permissions/`+idPRate+` {"value":250}`)
	assertBody(t, rec, `<option value="`+idPMotd+`" selected>`, `value="half typed"`, "autofocus")
}

func TestAdminRoleSaveValueDropsATypedValueForAPermissionNoLongerAvailable(t *testing.T) {
	f := newFakeAdmin(t)
	seedRoleEditor(f)
	f.on("PUT /roles/"+idBee+"/permissions/"+idPRate, 204, ``)
	rec := putRoleValue(idPRate, url.Values{"value_" + idPRate: {"250"}, "grant_permission": {idPBee}, "grant_value": {"stale"}})
	assertNoBody(t, rec, `value="stale"`)
	assertBody(t, rec, `<option value="`+idPPets+`" selected>`)
}

func TestAdminRoleRemoveDeletesTheGrantAndKeepsTheGrantForm(t *testing.T) {
	f := newFakeAdmin(t)
	seedRoleEditor(f)
	f.on("DELETE /roles/"+idBee+"/permissions/"+idPBee, 204, ``)
	rec := action(http.MethodPost, "/admin/roles/"+idBee+"/permissions/"+idPBee+"/remove", url.Values{"grant_permission": {idPMotd}, "grant_value": {"half typed"}})
	assertStatus(t, rec, http.StatusOK)
	assertWrites(t, f, `DELETE /roles/`+idBee+`/permissions/`+idPBee+` `)
	assertBody(t, rec, `<option value="`+idPMotd+`" selected>`, `value="half typed"`, "autofocus")
}

func TestAdminRoleRemoveRefusedShowsTheAPIMessage(t *testing.T) {
	f := newFakeAdmin(t)
	seedRoleEditor(f)
	f.problem("DELETE /roles/"+idBee+"/permissions/"+idPBee, 409, "system and owner keep roles.admin")
	rec := action(http.MethodPost, "/admin/roles/"+idBee+"/permissions/"+idPBee+"/remove", nil)
	assertStatus(t, rec, http.StatusConflict)
	if got := rec.Body.String(); got != "system and owner keep roles.admin" {
		t.Errorf("body = %q", got)
	}
}

func TestAdminRoleGrantValueFragmentMatchesThePermissionType(t *testing.T) {
	cases := []struct {
		permission string
		want       string
		unwanted   string
	}{
		{idPRate, `type="number"`, "<textarea"},
		{idPMotd, `type="text"`, "<textarea"},
		{idPPets, "<textarea", `type="`},
		{idPStore, "", `<input`},
	}
	for _, tc := range cases {
		t.Run(tc.permission, func(t *testing.T) {
			f := newFakeAdmin(t)
			f.on("GET /permissions", 200, permissionsJSON)
			rec := adminReq{method: http.MethodGet, target: "/admin/roles/" + idBee + "/grant-value?grant_permission=" + tc.permission, htmx: true}.do()
			assertStatus(t, rec, http.StatusOK)
			assertBody(t, rec, tc.want)
			assertNoBody(t, rec, tc.unwanted, "<html")
		})
	}
	f := newFakeAdmin(t)
	f.on("GET /permissions", 200, permissionsJSON)
	rec := adminReq{method: http.MethodGet, target: "/admin/roles/" + idBee + "/grant-value?grant_permission=nope", htmx: true}.do()
	assertStatus(t, rec, http.StatusNotFound)
}

func TestAdminRoleSaveSendsOnlyWhatChanged(t *testing.T) {
	cases := []struct {
		name string
		form url.Values
		want string
	}{
		{"name", url.Values{"name": {" bee_manager "}, "description": {"Bee Name Generator Admin"}}, `PATCH /roles/` + idBee + ` {"name":"bee_manager"}`},
		{"description", url.Values{"name": {"bee_admin"}, "description": {" Manages bees "}}, `PATCH /roles/` + idBee + ` {"description":"Manages bees"}`},
		{"both", url.Values{"name": {"bee_manager"}, "description": {"Manages bees"}}, `PATCH /roles/` + idBee + ` {"description":"Manages bees","name":"bee_manager"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeAdmin(t)
			seedRoleEditor(f)
			f.on("PATCH /roles/"+idBee, 200, `{}`)
			tc.form.Set("loaded_name", "bee_admin")
			tc.form.Set("loaded_description", "Bee Name Generator Admin")
			rec := action(http.MethodPost, "/admin/roles/"+idBee, tc.form)
			assertStatus(t, rec, http.StatusOK)
			assertWrites(t, f, tc.want)
			assertBody(t, rec, `id="admin-role-form"`, `id="admin-role-status" hx-swap-oob="innerHTML">Saved<`,
				`id="admin-role-header" hx-swap-oob="true"`, `id="admin-role-delete" hx-swap-oob="true"`)
			assertNoBody(t, rec, `id="admin-role-grants"`)
		})
	}
}

func TestAdminRoleSaveWithoutAChangeSendsNothing(t *testing.T) {
	f := newFakeAdmin(t)
	seedRoleEditor(f)
	rec := action(http.MethodPost, "/admin/roles/"+idBee, url.Values{
		"loaded_name": {"bee_admin"}, "name": {"bee_admin"},
		"loaded_description": {"Bee Name Generator Admin"}, "description": {"Bee Name Generator Admin"},
	})
	assertStatus(t, rec, http.StatusOK)
	assertWrites(t, f)
	assertBody(t, rec, `id="admin-role-status" hx-swap-oob="innerHTML">Nothing to save<`)
}

func TestAdminRoleSaveRefusedShowsTheAPIMessage(t *testing.T) {
	f := newFakeAdmin(t)
	seedRoleEditor(f)
	f.problem("PATCH /roles/"+idBee, 409, "Built-in roles cannot be deleted or renamed")
	rec := action(http.MethodPost, "/admin/roles/"+idBee, url.Values{"loaded_name": {"bee_admin"}, "name": {"renamed"}})
	assertStatus(t, rec, http.StatusConflict)
	assertNoBody(t, rec, "Saved")
}

func TestAdminRoleSaveFailedReloadShowsTheErrorWithoutSaved(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("PATCH /roles/"+idBee, 200, `{}`)
	f.problem("GET /roles/"+idBee, 500, "roles are down")
	rec := action(http.MethodPost, "/admin/roles/"+idBee, url.Values{"loaded_name": {"bee_admin"}, "name": {"renamed"}})
	assertStatus(t, rec, http.StatusInternalServerError)
	assertNoBody(t, rec, "Saved")
}

func TestAdminRoleDeleteRedirectsToTheList(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("DELETE /roles/"+idBee, 204, ``)
	rec := action(http.MethodDelete, "/admin/roles/"+idBee, nil)
	assertStatus(t, rec, http.StatusOK)
	if got := rec.Header().Get("HX-Redirect"); got != "/admin/roles" {
		t.Errorf("HX-Redirect = %q", got)
	}
	assertWrites(t, f, `DELETE /roles/`+idBee+` `)
}

func TestAdminRoleDeleteRefusedShowsTheAPIMessage(t *testing.T) {
	f := newFakeAdmin(t)
	f.problem("DELETE /roles/"+idBee, 409, "The role is assigned to an account")
	rec := action(http.MethodDelete, "/admin/roles/"+idBee, nil)
	assertStatus(t, rec, http.StatusConflict)
	if got := rec.Header().Get("HX-Redirect"); got != "" {
		t.Errorf("HX-Redirect = %q, want none", got)
	}
}

func TestAdminPermissionListShowsTypesAndMerge(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("GET /permissions", 200, permissionsJSON)
	rec := getPage("/admin/permissions")
	assertStatus(t, rec, http.StatusOK)
	assertBody(t, rec,
		">beenamegenerator.admin<", ">int, merge max<", ">string_list, merge union<", ">string, merge first<",
		`hx-delete="/admin/permissions/`+idPRate+`"`, `hx-confirm="Delete the permission ratelimit?"`,
		`id="admin-permission-create-form"`, `group-has-[option[value=int]:checked]:block`,
	)
	assertNoBody(t, rec, "autofocus")
}

func TestAdminPermissionListEmptyAndRefused(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("GET /permissions", 200, `[]`)
	assertBody(t, getPage("/admin/permissions"), `id="admin-permissions-empty"`, `id="admin-permission-create-form"`)

	f.problem("GET /permissions", 403, "no access")
	rec := getPage("/admin/permissions")
	assertStatus(t, rec, http.StatusForbidden)
	assertBody(t, rec, "no access")
	assertNoBody(t, rec, `id="admin-permission-create-form"`)
}

func TestAdminPermissionListEscapesAPIText(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("GET /permissions", 200, fmt.Sprintf(`[{"id":%q,"node":%q,"description":%q,"value_type":%q,"merge":%q}]`,
		hostile+"id", hostile+"node", hostile+"description", hostile+"type", hostile+"merge"))
	rec := getPage("/admin/permissions")
	assertNoBody(t, rec, "<img src=x")
}

func TestAdminPermissionCreateSendsOnlyWhatTheTypeNeeds(t *testing.T) {
	cases := []struct {
		name string
		form url.Values
		want string
	}{
		{"bare", url.Values{"node": {" pets.write "}, "description": {" Write pets "}, "merge": {"min"}}, `{"description":"Write pets","node":"pets.write"}`},
		{"int", url.Values{"node": {"quota"}, "value_type": {"int"}, "merge": {"min"}}, `{"description":"","merge":"min","node":"quota","value_type":"int"}`},
		{"text", url.Values{"node": {"greeting"}, "value_type": {"string"}, "merge": {"max"}}, `{"description":"","node":"greeting","value_type":"string"}`},
		{"list", url.Values{"node": {"names"}, "value_type": {"string_list"}}, `{"description":"","node":"names","value_type":"string_list"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeAdmin(t)
			f.on("POST /permissions", 201, `{}`)
			f.on("GET /permissions", 200, permissionsJSON)
			rec := action(http.MethodPost, "/admin/permissions", tc.form)
			assertStatus(t, rec, http.StatusOK)
			assertWrites(t, f, "POST /permissions "+tc.want)
			assertBody(t, rec, `id="admin-permissions-list"`, `id="admin-permission-create-form" hx-swap-oob="true"`)
			assertNoBody(t, rec, "autofocus")
		})
	}
}

func TestAdminPermissionCreateRefusedShowsTheAPIMessage(t *testing.T) {
	f := newFakeAdmin(t)
	f.problem("POST /permissions", 400, "Nodes are lower-case words")
	rec := action(http.MethodPost, "/admin/permissions", url.Values{"node": {"Bad Node"}})
	assertStatus(t, rec, http.StatusBadRequest)
	if got := rec.Body.String(); got != "Nodes are lower-case words" {
		t.Errorf("body = %q", got)
	}
}

func TestAdminPermissionDeleteReloadsTheListWithFocusOnItsHeading(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("DELETE /permissions/"+idPStore, 204, ``)
	f.on("GET /permissions", 200, permissionsJSON)
	rec := action(http.MethodDelete, "/admin/permissions/"+idPStore, nil)
	assertStatus(t, rec, http.StatusOK)
	assertWrites(t, f, `DELETE /permissions/`+idPStore+` `)
	assertBody(t, rec, "autofocus")
}

func TestAdminPermissionDeleteRefusedShowsTheAPIMessage(t *testing.T) {
	f := newFakeAdmin(t)
	f.problem("DELETE /permissions/"+idPBee, 409, "The permission is granted by a role")
	rec := action(http.MethodDelete, "/admin/permissions/"+idPBee, nil)
	assertStatus(t, rec, http.StatusConflict)
	if got := rec.Body.String(); got != "The permission is granted by a role" {
		t.Errorf("body = %q", got)
	}
}

const idUnlisted = "4242424242424242424"

func TestAdminUserEditorKeepsRolesWithoutACheckbox(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("GET /users/"+idBob, 200, `{"user_id":"`+idBob+`","username":"bob","roles":["`+idBee+`","`+idUnlisted+`"]}`)
	f.on("GET /users/"+idBob+"/links", 200, `[]`)
	f.on("GET /users/"+idBob+"/permissions", 200, `[]`)
	f.on("GET /roles", 200, rolesJSON)
	rec := getPage("/admin/users/" + idBob)
	assertBody(t, rec, `name="kept_roles" value="`+idUnlisted+`"`, `name="loaded_roles" value="`+idUnlisted+`"`)
	assertNoBody(t, rec, `name="kept_roles" value="`+idBee+`"`)
}

func TestAdminUserSaveKeepsRolesWithoutACheckbox(t *testing.T) {
	cases := []struct {
		name  string
		extra url.Values
		want  string
	}{
		{"username only", url.Values{"username": {"robert"}}, `PUT /users/` + idBob + ` {"username":"robert"}`},
		{"roles changed", url.Values{"roles": {idSystem}}, `PUT /users/` + idBob + ` {"roles":["` + idSystem + `","` + idUnlisted + `"]}`},
		{"every checkbox cleared", url.Values{"roles": nil}, `PUT /users/` + idBob + ` {"roles":["` + idUnlisted + `"]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeAdmin(t)
			f.on("PUT /users/"+idBob, 200, bobJSON)
			f.on("GET /users/"+idBob+"/links", 200, `[]`)
			f.on("GET /users/"+idBob+"/permissions", 200, `[]`)
			f.on("GET /roles", 200, rolesJSON)
			form := userForm(tc.extra)
			form["loaded_roles"] = []string{idBee, idUnlisted}
			form["kept_roles"] = []string{idUnlisted}
			if tc.name == "every checkbox cleared" {
				form.Del("roles")
			}
			saveUser(form)
			assertWrites(t, f, tc.want)
		})
	}
}

func TestAdminUserSaveIgnoresTheOrderOfRoles(t *testing.T) {
	f := newFakeAdmin(t)
	seedUserEditor(f)
	form := userForm(nil)
	form["loaded_roles"] = []string{idBee, idSystem}
	form["roles"] = []string{idSystem, idBee}
	rec := saveUser(form)
	assertBody(t, rec, ">Nothing to save<")
	assertWrites(t, f)
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

func TestAdminRoleSaveRefusesAnEmptiedName(t *testing.T) {
	f := newFakeAdmin(t)
	seedRoleEditor(f)
	rec := action(http.MethodPost, "/admin/roles/"+idBee, url.Values{"loaded_name": {"bee_admin"}, "name": {"  "}})
	assertStatus(t, rec, http.StatusBadRequest)
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
			if got := rec.Body.String(); got != message {
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

func TestAdminRoleChangesKeepTheValuesTypedInOtherRows(t *testing.T) {
	typed := func(extra url.Values) url.Values {
		form := url.Values{"value_" + idPRate: {"777"}, "grant_permission": {idPMotd}, "grant_value": {"half typed"}}
		for key, values := range extra {
			form[key] = values
		}
		return form
	}
	t.Run("grant keeps the draft and clears the grant form", func(t *testing.T) {
		f := newFakeAdmin(t)
		seedRoleEditor(f)
		f.on("PUT /roles/"+idBee+"/permissions/"+idPMotd, 204, ``)
		rec := action(http.MethodPost, "/admin/roles/"+idBee+"/permissions", typed(url.Values{"grant_value": {"hello"}}))
		assertBody(t, rec, `value="777"`)
		assertNoBody(t, rec, `value="hello"`, `value="half typed"`)
	})
	t.Run("saving a value keeps the other drafts and shows the saved value", func(t *testing.T) {
		f := newFakeAdmin(t)
		f.on("GET /roles/"+idBee, 200, `{"id":"`+idBee+`","name":"bee_admin","permissions":[
			{"id":"`+idPRate+`","node":"ratelimit","value_type":"int","merge":"max","value":250},
			{"id":"`+idPMotd+`","node":"motd","value_type":"string","merge":"first","value":"hi"}]}`)
		f.on("GET /permissions", 200, permissionsJSON)
		f.on("PUT /roles/"+idBee+"/permissions/"+idPRate, 204, ``)
		rec := putRoleValue(idPRate, url.Values{"value_" + idPRate: {"250"}, "value_" + idPMotd: {"half typed"}})
		assertBody(t, rec, `value="250"`, `value="half typed"`)
		assertNoBody(t, rec, `value="hi"`)
	})
	t.Run("removing keeps the other drafts", func(t *testing.T) {
		f := newFakeAdmin(t)
		seedRoleEditor(f)
		f.on("DELETE /roles/"+idBee+"/permissions/"+idPBee, 204, ``)
		rec := action(http.MethodPost, "/admin/roles/"+idBee+"/permissions/"+idPBee+"/remove", typed(nil))
		assertBody(t, rec, `value="777"`, `value="half typed"`)
	})
}

func TestAdminRoleGrantsFocusTheListOnlyWhenTheirButtonIsGone(t *testing.T) {
	t.Run("a grant that leaves others to grant", func(t *testing.T) {
		f := newFakeAdmin(t)
		seedRoleEditor(f)
		f.on("PUT /roles/"+idBee+"/permissions/"+idPStore, 204, ``)
		rec := action(http.MethodPost, "/admin/roles/"+idBee+"/permissions", url.Values{"grant_permission": {idPStore}})
		assertNoBody(t, rec, "autofocus")
	})
	t.Run("the last grant", func(t *testing.T) {
		f := newFakeAdmin(t)
		f.on("GET /roles/"+idBee, 200, `{"id":"`+idBee+`","name":"bee_admin","permissions":[{"id":"`+idPBee+`","node":"beenamegenerator.admin"}]}`)
		f.on("GET /permissions", 200, `[{"id":"`+idPBee+`","node":"beenamegenerator.admin"}]`)
		f.on("PUT /roles/"+idBee+"/permissions/"+idPBee, 204, ``)
		rec := action(http.MethodPost, "/admin/roles/"+idBee+"/permissions", url.Values{"grant_permission": {idPBee}})
		assertBody(t, rec, "autofocus", `id="admin-role-grant-empty"`)
	})
}

func TestAdminUserRowsFocusTheFirstNewRowAfterTheFirstPage(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("GET /roles", 200, rolesJSON)
	f.on("GET /users", 200, usersJSON)
	rec := adminReq{method: http.MethodGet, target: "/admin/users/rows?offset=3", htmx: true}.do()
	if got := strings.Count(rec.Body.String(), "autofocus"); got != 1 {
		t.Errorf("autofocus appears %d times, want once", got)
	}
	assertNoBody(t, getPage("/admin/users"), "autofocus")
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

func TestAdminUserRowsFromTheStartDoNotTakeFocus(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("GET /roles", 200, rolesJSON)
	f.on("GET /users", 200, usersJSON)
	rec := adminReq{method: http.MethodGet, target: "/admin/users/rows?offset=0", htmx: true}.do()
	assertStatus(t, rec, http.StatusOK)
	assertNoBody(t, rec, "autofocus")
}

func TestAdminUserEditorEscapesTheIDInEveryCall(t *testing.T) {
	const escaped = "a%2Fb%3Fc%23d%25e"
	f := newFakeAdmin(t)
	f.on("GET /users/"+escaped, 200, `{"user_id":"a/b?c#d%e","username":"x","roles":[]}`)
	f.on("GET /users/"+escaped+"/links", 200, `[]`)
	f.on("GET /users/"+escaped+"/permissions", 200, `[]`)
	f.on("GET /roles", 200, rolesJSON)
	rec := getPage("/admin/users/" + escaped)
	assertStatus(t, rec, http.StatusOK)
	assertBody(t, rec, `hx-post="/admin/users/`+escaped+`"`)
}
