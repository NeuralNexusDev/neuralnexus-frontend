package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestAdminUserListPagesAndNamesRoles(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("GET /users", 200, usersJSON)
	f.on("GET /roles", 200, rolesJSON)
	rec := getPage("/admin/users/list")
	assertStatus(t, rec, http.StatusOK)
	assertBody(t, rec,
		`id="admin-users-search"`,
		`href="/admin/users/`+idAlice+`"`,
		">alice<", ">"+idBob+"<",
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
			rec := getPage("/admin/users/list")
			if got := strings.Contains(rec.Body.String(), `id="admin-users-more"`); got != tc.wantMore {
				t.Errorf("load more present = %v, want %v", got, tc.wantMore)
			}
			if tc.wantMore {
				assertBody(t, rec, `hx-get="/admin/users/rows?offset=200"`)
			}
			if got := strings.Contains(rec.Body.String(), `id="admin-users-empty"`); got != (tc.rows == 0) {
				t.Errorf("the empty notice is shown = %v with %d rows", got, tc.rows)
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
	for _, offset := range []string{"-1", "x", "1.5", "99999999999999999999"} {
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
	rec := getPage("/admin/users/list")
	assertStatus(t, rec, http.StatusOK)
	assertBody(t, rec, ">"+idBee+"<")
}

func TestAdminUserListFailedRolesLookupIsAnError(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("GET /users", 200, usersJSON)
	f.problem("GET /roles", 500, "roles are down")
	rec := getPage("/admin/users/list")
	assertStatus(t, rec, http.StatusInternalServerError)
	assertBody(t, rec, "roles are down")
	assertNoBody(t, rec, `id="admin-users-search"`)
}

func TestAdminUserListRefusedHidesTheSearch(t *testing.T) {
	f := newFakeAdmin(t)
	f.problem("GET /users", 403, "You do not have permission to list users")
	rec := getPage("/admin/users/list")
	assertStatus(t, rec, http.StatusForbidden)
	assertBody(t, rec, "You do not have permission to list users")
	assertNoBody(t, rec, `id="admin-users-search"`, `id="admin-users"`)
}

func TestAdminUserListEscapesAPIText(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("GET /users", 200, fmt.Sprintf(`[{"user_id":%q,"username":%q,"roles":[%q]}]`, hostile+"id", hostile+"name", idBee))
	f.on("GET /roles", 200, fmt.Sprintf(`[{"id":%q,"name":%q,"description":"d","permissions":[]}]`, idBee, hostile+"role"))
	rec := getPage("/admin/users/list")
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
	rec := getPage("/admin/users/" + idBob + "/editor")
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
	rec := getPage("/admin/users/" + idBob + "/editor")
	assertBody(t, rec, `id="admin-user-links-empty"`, `id="admin-user-permissions-empty"`)
}

func TestAdminUserEditorShowsRefusedSectionsAsErrors(t *testing.T) {
	f := newFakeAdmin(t)
	seedUserEditor(f)
	f.problem("GET /users/"+idBob+"/links", 403, "links are off limits")
	f.problem("GET /users/"+idBob+"/permissions", 500, "permissions are down")
	rec := getPage("/admin/users/" + idBob + "/editor")
	assertStatus(t, rec, http.StatusOK)
	assertBody(t, rec, `id="admin-user-links-error"`, "links are off limits", `id="admin-user-permissions-error"`, "permissions are down", `id="admin-user-form"`)
	assertNoBody(t, rec, `id="admin-user-links-empty"`, `id="admin-user-permissions-empty"`)
}

func TestAdminUserEditorWithoutRoleAccessIsReadOnly(t *testing.T) {
	f := newFakeAdmin(t)
	seedUserEditor(f)
	f.problem("GET /roles", 403, "forbidden")
	rec := getPage("/admin/users/" + idBob + "/editor")
	assertStatus(t, rec, http.StatusOK)
	assertBody(t, rec, `id="admin-user-roles-note"`, ">"+idBee+"<")
	assertNoBody(t, rec, `name="roles_editable"`, `name="roles"`)
}

func TestAdminUserEditorFailedRolesLookupIsAnError(t *testing.T) {
	f := newFakeAdmin(t)
	seedUserEditor(f)
	f.problem("GET /roles", 500, "roles are down")
	rec := getPage("/admin/users/" + idBob + "/editor")
	assertStatus(t, rec, http.StatusInternalServerError)
	assertBody(t, rec, "roles are down")
	assertNoBody(t, rec, `id="admin-user-form"`, `id="admin-user-roles-note"`)
}

func TestAdminUserEditorUnknownUserShowsTheMessageAndNothingFromTheURL(t *testing.T) {
	f := newFakeAdmin(t)
	f.problem("GET /users/999", 404, "User not found")
	rec := getPage("/admin/users/999/editor")
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
	rec := getPage("/admin/users/" + escaped + "/editor")
	assertStatus(t, rec, http.StatusOK)
	assertNoBody(t, rec, "<img src=x")
	assertBody(t, rec, "&lt;img src=x", "platform", "permission", "role", "description")

	f.problem("GET /users/"+escaped+"/links", 500, hostile+"links detail")
	rec = getPage("/admin/users/" + escaped + "/editor")
	assertNoBody(t, rec, "<img src=x")

	f.problem("GET /roles", 403, "forbidden")
	rec = getPage("/admin/users/" + escaped + "/editor")
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
			assertBody(t, rec, `id="admin-user-form"`, `id="admin-user-status" hx-swap-oob="innerHTML">Saved<`,
				`id="admin-user-header" hx-swap-oob="true"`, `id="admin-user-permissions-section" hx-swap-oob="true"`)
			assertNoBody(t, rec, `id="admin-user-links`)
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
	if got := bannerText(rec); got != "Enter a username" {
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
	if got := bannerText(rec); got != "An account with this username already exists" {
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
	if got := bannerText(rec); got != "The change was made, but the page could not be refreshed: roles are down" {
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

const idUnlisted = "4242424242424242424"

func TestAdminUserEditorKeepsRolesWithoutACheckbox(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("GET /users/"+idBob, 200, `{"user_id":"`+idBob+`","username":"bob","roles":["`+idBee+`","`+idUnlisted+`"]}`)
	f.on("GET /users/"+idBob+"/links", 200, `[]`)
	f.on("GET /users/"+idBob+"/permissions", 200, `[]`)
	f.on("GET /roles", 200, rolesJSON)
	rec := getPage("/admin/users/" + idBob + "/editor")
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

func TestAdminUserRowsFocusTheFirstNewRowAfterTheFirstPage(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("GET /roles", 200, rolesJSON)
	f.on("GET /users", 200, usersJSON)
	rec := adminReq{method: http.MethodGet, target: "/admin/users/rows?offset=3", htmx: true}.do()
	if got := strings.Count(rec.Body.String(), "autofocus"); got != 1 {
		t.Errorf("autofocus appears %d times, want once", got)
	}
	assertNoBody(t, getPage("/admin/users/list"), "autofocus")
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
	rec := getPage("/admin/users/" + escaped + "/editor")
	assertStatus(t, rec, http.StatusOK)
	assertBody(t, rec, `hx-post="/admin/users/`+escaped+`"`)
}

func userPage(from, n int) string {
	var users []string
	for i := from; i < from+n; i++ {
		users = append(users, fmt.Sprintf(`{"user_id":"%d","username":"User%d","roles":[]}`, 3541025163146700000+int64(i), i))
	}
	return "[" + strings.Join(users, ",") + "]"
}

func TestAdminUserSearchMatchesNamesAndIDsIgnoringCase(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("GET /users", 200, usersJSON)
	f.on("GET /roles", 200, `[]`)
	cases := []struct {
		search string
		want   []string
		not    []string
	}{
		{"ALI", []string{">alice<"}, []string{">bob<"}},
		{"  bob  ", []string{">bob<"}, []string{">alice<"}},
		{idAnon[len(idAnon)-4:], []string{">" + idAnon + "<"}, []string{">alice<"}},
		{"nobody", nil, []string{">alice<", ">bob<"}},
	}
	for _, tc := range cases {
		t.Run(tc.search, func(t *testing.T) {
			rec := adminReq{method: http.MethodGet, target: "/admin/users/rows?search=" + url.QueryEscape(tc.search), htmx: true}.do()
			assertStatus(t, rec, http.StatusOK)
			assertBody(t, rec, tc.want...)
			assertNoBody(t, rec, tc.not...)
			if len(tc.want) == 0 {
				assertBody(t, rec, `id="admin-users-empty"`, "No users found")
			}
		})
	}
}

func TestAdminUserSearchReadsPagesUntilTheListEnds(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("GET /roles", 200, `[]`)
	f.on("GET /users", 200, fullPage(200))
	rec := adminReq{method: http.MethodGet, target: "/admin/users/rows?search=user19", htmx: true}.do()
	assertStatus(t, rec, http.StatusOK)
	uris := f.uris()
	wantCalls := []string{
		"GET /users?limit=200&offset=0", "GET /users?limit=200&offset=200", "GET /users?limit=200&offset=400",
		"GET /users?limit=200&offset=600", "GET /users?limit=200&offset=800", "GET /roles",
	}
	if strings.Join(uris, "|") != strings.Join(wantCalls, "|") {
		t.Errorf("calls = %v, want %v", uris, wantCalls)
	}
	assertBody(t, rec, `hx-get="/admin/users/rows?offset=1000&amp;search=user19"`, "Searched the first 1000 users")
}

func TestAdminUserSearchStopsAtAShortPage(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("GET /roles", 200, `[]`)
	f.on("GET /users", 200, fullPage(150))
	rec := adminReq{method: http.MethodGet, target: "/admin/users/rows?search=user1", htmx: true}.do()
	if got := len(f.uris()); got != 2 {
		t.Errorf("calls = %v, want one page and the roles", f.uris())
	}
	assertNoBody(t, rec, `id="admin-users-more"`, "Searched the first")
}

func TestAdminUserSearchWithoutMatchesInTheScannedUsersOffersMore(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("GET /roles", 200, `[]`)
	f.on("GET /users", 200, fullPage(200))
	rec := adminReq{method: http.MethodGet, target: "/admin/users/rows?search=zzz", htmx: true}.do()
	assertBody(t, rec, "No matches in the first 1000 users", `id="admin-users-more"`)
}

func TestAdminUserRowsAfterTheFirstPageHaveNoEmptyNotice(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("GET /roles", 200, `[]`)
	f.on("GET /users", 200, `[]`)
	rec := adminReq{method: http.MethodGet, target: "/admin/users/rows?offset=200&search=zzz", htmx: true}.do()
	assertNoBody(t, rec, `id="admin-users-empty"`)
}

func TestAdminUserListWithoutASearchReadsOnePage(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("GET /roles", 200, `[]`)
	f.on("GET /users", 200, fullPage(200))
	rec := getPage("/admin/users/list")
	if got := len(f.uris()); got != 2 {
		t.Errorf("calls = %v, want one page and the roles", f.uris())
	}
	assertBody(t, rec, `hx-get="/admin/users/rows?offset=200"`)
	assertNoBody(t, rec, "Searched the first")
}

func TestAdminUserSearchBoxQueriesTheServer(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("GET /users", 200, usersJSON)
	f.on("GET /roles", 200, `[]`)
	assertBody(t, getPage("/admin/users/list"),
		`name="search"`, `hx-get="/admin/users/rows"`, `hx-trigger="input changed delay:250ms, search"`,
		`hx-target="#admin-users"`, `hx-sync="this:replace"`)
	assertNoBody(t, getPage("/admin/users/list"), "oninput", "filterAdminUsers")
}

func TestAdminUserSaveDoesNotReloadTheLinkedAccounts(t *testing.T) {
	f := newFakeAdmin(t)
	seedUserEditor(f)
	f.on("PUT /users/"+idBob, 200, bobJSON)
	rec := saveUser(userForm(url.Values{"username": {"robert"}}))
	assertStatus(t, rec, http.StatusOK)
	for _, uri := range f.uris() {
		if strings.HasSuffix(uri, "/links") {
			t.Errorf("a save reloaded the linked accounts: %v", f.uris())
		}
	}
}

func TestAdminUserPageShowsEveryRegion(t *testing.T) {
	f := newFakeAdmin(t)
	seedUserEditor(f)
	rec := getPage("/admin/users/" + idBob + "/editor")
	assertBody(t, rec, `id="admin-user"`, `id="admin-user-header"`, `id="admin-user-form"`, `id="admin-user-permissions-section"`,
		`hx-target="#admin-user-form"`, `id="admin-user" class="space-y-6" hx-sync="this:drop"`)
	assertNoBody(t, rec, `hx-swap-oob="true"`)
}
