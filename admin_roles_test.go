package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestAdminRoleListShowsRolesWithTheirPermissions(t *testing.T) {
	f := newFakeBackend(t)
	f.on("GET /roles", 200, `[
		{"id":"`+idBee+`","name":"bee_admin","description":"Bee Name Generator Admin","permissions":[
			{"id":"`+idPBee+`","node":"beenamegenerator.admin","description":"d"},
			{"id":"`+idPRate+`","node":"ratelimit","description":"d","value_type":"int","merge":"max","value":100},
			{"id":"`+idPPets+`","node":"petpictures.pets","description":"d","value_type":"string_list","merge":"union","value":["rex","fido"]}
		]}]`)
	rec := getPage("/admin/roles/list")
	assertStatus(t, rec, http.StatusOK)
	assertBody(t, rec, `href="/admin/roles/`+idBee+`"`, "bee_admin", "Bee Name Generator Admin",
		">beenamegenerator.admin<", ">ratelimit: 100<", ">petpictures.pets: rex, fido<",
		`id="admin-role-create-form"`, `hx-post="/admin/roles"`)
	assertNoBody(t, rec, `id="admin-roles-empty"`)
}

func TestAdminRoleListWithoutRolesSaysSo(t *testing.T) {
	f := newFakeBackend(t)
	f.on("GET /roles", 200, `[]`)
	assertBody(t, getPage("/admin/roles/list"), `id="admin-roles-empty"`, ">No roles<", `id="admin-role-create-form"`)
}

func TestAdminRoleListRefusedHidesTheCreateForm(t *testing.T) {
	f := newFakeBackend(t)
	f.problem("GET /roles", 403, "You do not have permission to manage roles and permissions")
	rec := getPage("/admin/roles/list")
	assertStatus(t, rec, http.StatusForbidden)
	assertBody(t, rec, "You do not have permission to manage roles and permissions")
	assertNoBody(t, rec, `id="admin-role-create-form"`)
}

func TestAdminRoleListEscapesAPIText(t *testing.T) {
	f := newFakeBackend(t)
	f.on("GET /roles", 200, fmt.Sprintf(`[{"id":%q,"name":%q,"description":%q,"permissions":[
		{"id":"1","node":%q,"description":"d"},
		{"id":"2","node":"ratelimit","description":"d","value_type":"int","value":%q},
		{"id":"3","node":"x","description":"d","value_type":"string_list","value":[%q]}]}]`,
		hostile+"id", hostile+"name", hostile+"description", hostile+"node", hostile+"number-looking", hostile+"item"))
	rec := getPage("/admin/roles/list")
	assertNoBody(t, rec, "<img src=x")
}

func TestAdminRoleCreatePostsTheTrimmedFieldsAndOpensTheEditor(t *testing.T) {
	f := newFakeBackend(t)
	f.on("POST /roles", 201, `{"id":"`+idBee+`","name":"moderator","description":"Moderates things","permissions":[]}`)
	rec := action(http.MethodPost, "/admin/roles", url.Values{"name": {" moderator "}, "description": {" Moderates things "}})
	assertStatus(t, rec, http.StatusOK)
	if got := rec.Header().Get("HX-Redirect"); got != "/admin/roles/"+idBee {
		t.Errorf("HX-Redirect = %q", got)
	}
	assertWrites(t, f, `POST /roles {"description":"Moderates things","name":"moderator"}`)
}

func TestAdminRoleCreateRefusedShowsTheAPIMessage(t *testing.T) {
	f := newFakeBackend(t)
	f.problem("POST /roles", 409, "A role with that name already exists")
	rec := action(http.MethodPost, "/admin/roles", url.Values{"name": {"system"}})
	assertStatus(t, rec, http.StatusConflict)
	if got := rec.Header().Get("HX-Redirect"); got != "" {
		t.Errorf("HX-Redirect = %q, want none", got)
	}
}

func seedRoleEditor(f *fakeBackend) {
	f.on("GET /roles/"+idBee, 200, `{"id":"`+idBee+`","name":"bee_admin","description":"Bee Name Generator Admin","permissions":[
		{"id":"`+idPBee+`","node":"beenamegenerator.admin","description":"Bee name generator"},
		{"id":"`+idPRate+`","node":"ratelimit","description":"Rate limit","value_type":"int","merge":"max","value":100}]}`)
	f.on("GET /permissions", 200, permissionsJSON)
}

func TestAdminRoleEditorShowsTheRole(t *testing.T) {
	f := newFakeBackend(t)
	seedRoleEditor(f)
	rec := getPage("/admin/roles/" + idBee + "/editor")
	assertStatus(t, rec, http.StatusOK)
	assertBody(t, rec,
		`id="admin-role-title"`, ">bee_admin<", ">"+idBee+"<",
		`name="loaded_name" value="bee_admin"`, `name="loaded_description" value="Bee Name Generator Admin"`,
		`hx-delete="/admin/roles/`+idBee+`/permissions/`+idPRate+`"`,
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
	f := newFakeBackend(t)
	f.on("GET /roles/"+idSystem, 200, `{"id":"`+idSystem+`","name":"system","description":"System","permissions":[]}`)
	f.on("GET /permissions", 200, `[]`)
	rec := getPage("/admin/roles/" + idSystem + "/editor")
	assertBody(t, rec, `id="admin-role-permissions-empty"`, `id="admin-role-grant-empty"`)
	assertNoBody(t, rec, `id="admin-role-grant-form"`)
}

func TestAdminRoleEditorUnknownRoleShowsTheMessageAndNothingFromTheURL(t *testing.T) {
	f := newFakeBackend(t)
	f.problem("GET /roles/404", 404, "Role not found")
	rec := getPage("/admin/roles/404/editor")
	assertStatus(t, rec, http.StatusNotFound)
	assertBody(t, rec, "Role not found")
	assertNoBody(t, rec, `id="admin-role-form"`, ">404<")
}

func TestAdminRoleEditorFailedPermissionLookupIsAnError(t *testing.T) {
	f := newFakeBackend(t)
	seedRoleEditor(f)
	f.problem("GET /permissions", 500, "permissions are down")
	rec := getPage("/admin/roles/" + idBee + "/editor")
	assertStatus(t, rec, http.StatusInternalServerError)
	assertBody(t, rec, "permissions are down")
	assertNoBody(t, rec, `id="admin-role-form"`)
}

func TestAdminRoleEditorEscapesIDsAndAPIText(t *testing.T) {
	id := hostile + "id"
	escaped := url.PathEscape(id)
	f := newFakeBackend(t)
	f.on("GET /roles/"+escaped, 200, fmt.Sprintf(`{"id":%q,"name":%q,"description":%q,"permissions":[
		{"id":"1","node":%q,"description":%q},
		{"id":"2","node":"motd","description":"d","value_type":"string","value":%q},
		{"id":"3","node":"x","description":"d","value_type":"string_list","value":[%q]}]}`,
		id, hostile+"name", hostile+"description", hostile+"node", hostile+"node description", hostile+"value", hostile+"item"))
	f.on("GET /permissions", 200, fmt.Sprintf(`[{"id":"9","node":%q,"description":"d"}]`, hostile+"ungranted"))
	rec := getPage("/admin/roles/" + escaped + "/editor")
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
			f := newFakeBackend(t)
			seedRoleEditor(f)
			f.on("PUT /roles/"+idBee+"/permissions/"+tc.permission, 204, ``)
			rec := action(http.MethodPost, "/admin/roles/"+idBee+"/permissions", url.Values{"grant_permission": {tc.permission}, "grant_value": {tc.value}})
			assertStatus(t, rec, http.StatusOK)
			assertWrites(t, f, tc.wantWrite)
			assertBody(t, rec, `id="admin-role-granted"`, `id="admin-role-grant" hx-swap-oob="true"`)
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
		{"negative zero", idPRate, "-0", "Enter a whole number from -9007199254740992 to 9007199254740992"},
		{"trailing zero fraction", idPRate, "1.0", "Enter a whole number from -9007199254740992 to 9007199254740992"},
		{"plus sign", idPRate, "+5", "Enter a whole number from -9007199254740992 to 9007199254740992"},
		{"2^53 plus one", idPRate, "9007199254740993", "Enter a whole number from -9007199254740992 to 9007199254740992"},
		{"below -2^53", idPRate, "-9007199254740993", "Enter a whole number from -9007199254740992 to 9007199254740992"},
		{"beyond int64", idPRate, "99999999999999999999", "Enter a whole number from -9007199254740992 to 9007199254740992"},
		{"empty list", idPPets, " \n  \n", "Enter at least one item"},
		{"empty text", idPMotd, "   ", "Enter a value"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeBackend(t)
			seedRoleEditor(f)
			rec := action(http.MethodPost, "/admin/roles/"+idBee+"/permissions", url.Values{"grant_permission": {tc.permission}, "grant_value": {tc.value}})
			assertStatus(t, rec, http.StatusBadRequest)
			if got := bannerText(rec); got != tc.message {
				t.Errorf("body = %q, want %q", got, tc.message)
			}
			assertWrites(t, f)
		})
	}
}

func TestAdminRoleGrantUnknownPermission(t *testing.T) {
	f := newFakeBackend(t)
	seedRoleEditor(f)
	rec := action(http.MethodPost, "/admin/roles/"+idBee+"/permissions", url.Values{"grant_permission": {"123"}})
	assertStatus(t, rec, http.StatusNotFound)
	assertWrites(t, f)
}

func TestAdminRoleGrantRefusedShowsTheAPIMessage(t *testing.T) {
	f := newFakeBackend(t)
	seedRoleEditor(f)
	f.problem("PUT /roles/"+idBee+"/permissions/"+idPStore, 400, "The value must match")
	rec := action(http.MethodPost, "/admin/roles/"+idBee+"/permissions", url.Values{"grant_permission": {idPStore}})
	assertStatus(t, rec, http.StatusBadRequest)
	if got := bannerText(rec); got != "The value must match" {
		t.Errorf("body = %q", got)
	}
}

func TestAdminRoleSaveValueAnswersWithTheGrantedListAlone(t *testing.T) {
	f := newFakeBackend(t)
	seedRoleEditor(f)
	f.on("PUT /roles/"+idBee+"/permissions/"+idPRate, 204, ``)
	rec := putRoleValue(idPRate, url.Values{"value_" + idPRate: {"250"}, "grant_permission": {idPMotd}, "grant_value": {"half typed"}})
	assertStatus(t, rec, http.StatusOK)
	assertWrites(t, f, `PUT /roles/`+idBee+`/permissions/`+idPRate+` {"value":250}`)
	assertBody(t, rec, `id="admin-role-granted"`, "autofocus", `id="admin-role-status" hx-swap-oob="innerHTML"></p>`)
	assertNoBody(t, rec, `id="admin-role-grant"`, `half typed`)
}

func TestAdminRoleSaveValueDoesNotReloadThePermissionList(t *testing.T) {
	f := newFakeBackend(t)
	seedRoleEditor(f)
	f.on("PUT /roles/"+idBee+"/permissions/"+idPRate, 204, ``)
	putRoleValue(idPRate, url.Values{"value_" + idPRate: {"250"}})
	var lookups int
	for _, uri := range f.uris() {
		if uri == "GET /permissions" {
			lookups++
		}
	}
	if lookups != 1 {
		t.Errorf("GET /permissions was called %d times, want 1: %v", lookups, f.uris())
	}
}

func TestAdminRoleRemoveDropsATypedValueForAPermissionNoLongerAvailable(t *testing.T) {
	f := newFakeBackend(t)
	seedRoleEditor(f)
	f.on("DELETE /roles/"+idBee+"/permissions/"+idPBee, 204, ``)
	rec := actionDelete("/admin/roles/"+idBee+"/permissions/"+idPBee, url.Values{"grant_permission": {idPRate}, "grant_value": {"stale"}})
	assertNoBody(t, rec, `value="stale"`)
	assertBody(t, rec, `<option value="`+idPPets+`" selected>`)
}

func TestAdminRoleRemoveDeletesTheGrantAndKeepsTheGrantForm(t *testing.T) {
	f := newFakeBackend(t)
	seedRoleEditor(f)
	f.on("DELETE /roles/"+idBee+"/permissions/"+idPBee, 204, ``)
	rec := actionDelete("/admin/roles/"+idBee+"/permissions/"+idPBee, url.Values{"grant_permission": {idPMotd}, "grant_value": {"half typed"}})
	assertStatus(t, rec, http.StatusOK)
	assertWrites(t, f, `DELETE /roles/`+idBee+`/permissions/`+idPBee+` `)
	assertBody(t, rec, `id="admin-role-granted"`, `id="admin-role-grant" hx-swap-oob="true"`,
		`<option value="`+idPMotd+`" selected>`, `value="half typed"`, "autofocus")
}

func TestAdminRoleRemoveRefusedShowsTheAPIMessage(t *testing.T) {
	f := newFakeBackend(t)
	seedRoleEditor(f)
	f.problem("DELETE /roles/"+idBee+"/permissions/"+idPBee, 409, "system and owner keep roles.admin")
	rec := actionDelete("/admin/roles/"+idBee+"/permissions/"+idPBee, nil)
	assertStatus(t, rec, http.StatusConflict)
	if got := bannerText(rec); got != "system and owner keep roles.admin" {
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
			f := newFakeBackend(t)
			f.on("GET /permissions", 200, permissionsJSON)
			rec := pageReq{method: http.MethodGet, target: "/admin/roles/" + idBee + "/grant-value?grant_permission=" + tc.permission, htmx: true}.do()
			assertStatus(t, rec, http.StatusOK)
			assertBody(t, rec, tc.want)
			assertNoBody(t, rec, tc.unwanted, "<html")
		})
	}
	f := newFakeBackend(t)
	f.on("GET /permissions", 200, permissionsJSON)
	rec := pageReq{method: http.MethodGet, target: "/admin/roles/" + idBee + "/grant-value?grant_permission=nope", htmx: true}.do()
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
			f := newFakeBackend(t)
			seedRoleEditor(f)
			f.on("PATCH /roles/"+idBee, 200, `{"id":"`+idBee+`","name":"stored_name","description":"Stored description","permissions":[]}`)
			tc.form.Set("loaded_name", "bee_admin")
			tc.form.Set("loaded_description", "Bee Name Generator Admin")
			rec := action(http.MethodPost, "/admin/roles/"+idBee, tc.form)
			assertStatus(t, rec, http.StatusOK)
			assertWrites(t, f, tc.want)
			assertBody(t, rec, `id="admin-role-form"`, `id="admin-role-status" hx-swap-oob="innerHTML">Saved<`,
				`id="admin-role-header" hx-swap-oob="true"`, `id="admin-role-delete" hx-swap-oob="true"`)
			assertBody(t, rec, `name="loaded_name" value="stored_name"`, `name="loaded_description" value="Stored description"`,
				`hx-confirm="Delete the role stored_name?"`, ">stored_name<")
			assertNoBody(t, rec, `id="admin-role-granted"`, `id="admin-role-grant"`)
			for _, uri := range f.uris() {
				if uri == "GET /roles/"+idBee {
					t.Errorf("the save read the role again: %v", f.uris())
				}
			}
		})
	}
}

func TestAdminRoleRemoveReloadFailureRemovesTheRow(t *testing.T) {
	f := newFakeBackend(t)
	f.on("DELETE /roles/"+idBee+"/permissions/"+idPBee, 204, ``)
	f.problem("GET /roles/"+idBee, 500, "down")
	rec := actionDelete("/admin/roles/"+idBee+"/permissions/"+idPBee, nil)
	assertStatus(t, rec, http.StatusInternalServerError)
	assertBody(t, rec, `hx-swap-oob="delete:#granted-`+idPBee+`"`)

	f.on("DELETE /roles/"+idBee+"/permissions/not-a-number", 204, ``)
	rec = actionDelete("/admin/roles/"+idBee+"/permissions/not-a-number", nil)
	assertNoBody(t, rec, "delete:#")
}

func TestAdminRoleRemoveReloadFailureOffersTheRemovedPermissionAgain(t *testing.T) {
	f := newFakeBackend(t)
	f.on("DELETE /roles/"+idBee+"/permissions/"+idPBee, 204, ``)
	f.problem("GET /roles/"+idBee, 500, "down")
	f.on("GET /permissions", 200, permissionsJSON)
	rec := actionDelete("/admin/roles/"+idBee+"/permissions/"+idPBee, url.Values{"granted": {idPBee, idPRate}})
	assertStatus(t, rec, http.StatusInternalServerError)
	assertBody(t, rec, `id="admin-role-grant" hx-swap-oob="true"`, `<option value="`+idPBee+`"`, `<option value="`+idPStore+`"`)
	assertNoBody(t, rec, `<option value="`+idPRate+`"`)
}

func TestAdminRoleRemoveReloadFailureWithoutTheCatalogueReplacesTheGrantForm(t *testing.T) {
	f := newFakeBackend(t)
	f.on("DELETE /roles/"+idBee+"/permissions/"+idPBee, 204, ``)
	f.problem("GET /roles/"+idBee, 500, "down")
	f.problem("GET /permissions", 500, "down")
	rec := actionDelete("/admin/roles/"+idBee+"/permissions/"+idPBee, url.Values{"granted": {idPBee, idPRate}})
	assertBody(t, rec, `id="admin-role-grant" hx-swap-oob="true"`, `id="admin-role-grant-unavailable"`, `hx-swap-oob="delete:#granted-`+idPBee+`"`)
	assertNoBody(t, rec, "<option")
}

func TestAdminRoleSaveWithoutAChangeSendsNothing(t *testing.T) {
	f := newFakeBackend(t)
	seedRoleEditor(f)
	rec := action(http.MethodPost, "/admin/roles/"+idBee, url.Values{
		"loaded_name": {"bee_admin"}, "name": {"bee_admin"},
		"loaded_description": {"Bee Name Generator Admin"}, "description": {"Bee Name Generator Admin"},
	})
	assertStatus(t, rec, http.StatusOK)
	if uris := f.uris(); len(uris) != 0 {
		t.Errorf("the save called the API: %v", uris)
	}
	if got := rec.Header().Get("HX-Reswap"); got != "none" {
		t.Errorf("HX-Reswap = %q, want none", got)
	}
	assertBody(t, rec, `id="admin-role-status" hx-swap-oob="innerHTML">Nothing to save<`)
}

func TestAdminRoleSaveRefusedShowsTheAPIMessage(t *testing.T) {
	f := newFakeBackend(t)
	seedRoleEditor(f)
	f.problem("PATCH /roles/"+idBee, 409, "Built-in roles cannot be deleted or renamed")
	rec := action(http.MethodPost, "/admin/roles/"+idBee, url.Values{"loaded_name": {"bee_admin"}, "name": {"renamed"}})
	assertStatus(t, rec, http.StatusConflict)
	assertNoBody(t, rec, "Saved")
}

func TestAdminRoleDeleteRedirectsToTheList(t *testing.T) {
	f := newFakeBackend(t)
	f.on("DELETE /roles/"+idBee, 204, ``)
	rec := action(http.MethodDelete, "/admin/roles/"+idBee, nil)
	assertStatus(t, rec, http.StatusOK)
	if got := rec.Header().Get("HX-Redirect"); got != "/admin/roles" {
		t.Errorf("HX-Redirect = %q", got)
	}
	assertWrites(t, f, `DELETE /roles/`+idBee+` `)
}

func TestAdminRoleDeleteRefusedShowsTheAPIMessage(t *testing.T) {
	f := newFakeBackend(t)
	f.problem("DELETE /roles/"+idBee, 409, "The role is assigned to an account")
	rec := action(http.MethodDelete, "/admin/roles/"+idBee, nil)
	assertStatus(t, rec, http.StatusConflict)
	if got := rec.Header().Get("HX-Redirect"); got != "" {
		t.Errorf("HX-Redirect = %q, want none", got)
	}
}

func TestAdminRoleSaveRefusesAnEmptiedName(t *testing.T) {
	f := newFakeBackend(t)
	seedRoleEditor(f)
	rec := action(http.MethodPost, "/admin/roles/"+idBee, url.Values{"loaded_name": {"bee_admin"}, "name": {"  "}})
	assertStatus(t, rec, http.StatusBadRequest)
	assertWrites(t, f)
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
		f := newFakeBackend(t)
		seedRoleEditor(f)
		f.on("PUT /roles/"+idBee+"/permissions/"+idPMotd, 204, ``)
		rec := action(http.MethodPost, "/admin/roles/"+idBee+"/permissions", typed(url.Values{"grant_value": {"hello"}}))
		assertBody(t, rec, `value="777"`)
		assertNoBody(t, rec, `value="hello"`, `value="half typed"`)
	})
	t.Run("saving a value keeps the other drafts and shows the saved value", func(t *testing.T) {
		f := newFakeBackend(t)
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
		f := newFakeBackend(t)
		seedRoleEditor(f)
		f.on("DELETE /roles/"+idBee+"/permissions/"+idPBee, 204, ``)
		rec := actionDelete("/admin/roles/"+idBee+"/permissions/"+idPBee, typed(nil))
		assertBody(t, rec, `value="777"`, `value="half typed"`)
	})
}

func TestAdminRoleGrantsFocusTheListOnlyWhenTheirButtonIsGone(t *testing.T) {
	t.Run("a grant that leaves others to grant", func(t *testing.T) {
		f := newFakeBackend(t)
		seedRoleEditor(f)
		f.on("PUT /roles/"+idBee+"/permissions/"+idPStore, 204, ``)
		rec := action(http.MethodPost, "/admin/roles/"+idBee+"/permissions", url.Values{"grant_permission": {idPStore}})
		assertNoBody(t, rec, "autofocus")
	})
	t.Run("the last grant", func(t *testing.T) {
		f := newFakeBackend(t)
		f.on("GET /roles/"+idBee, 200, `{"id":"`+idBee+`","name":"bee_admin","permissions":[{"id":"`+idPBee+`","node":"beenamegenerator.admin"}]}`)
		f.on("GET /permissions", 200, `[{"id":"`+idPBee+`","node":"beenamegenerator.admin"}]`)
		f.on("PUT /roles/"+idBee+"/permissions/"+idPBee, 204, ``)
		rec := action(http.MethodPost, "/admin/roles/"+idBee+"/permissions", url.Values{"grant_permission": {idPBee}})
		assertBody(t, rec, "autofocus", `id="admin-role-grant-empty"`)
	})
}

func TestAdminRoleChangesEmptyTheStatusLine(t *testing.T) {
	f := newFakeBackend(t)
	seedRoleEditor(f)
	f.on("DELETE /roles/"+idBee+"/permissions/"+idPBee, 204, ``)
	rec := actionDelete("/admin/roles/"+idBee+"/permissions/"+idPBee, nil)
	assertBody(t, rec, `<p id="admin-role-status" hx-swap-oob="innerHTML"></p>`)
}

func TestAdminRoleEditorQueuesActionsOnTheEditor(t *testing.T) {
	f := newFakeBackend(t)
	seedRoleEditor(f)
	rec := getPage("/admin/roles/" + idBee + "/editor")
	assertBody(t, rec,
		`id="admin-role" class="space-y-6" data-status="admin-role-status" data-busy-region hx-sync:inherited="this:drop"`,
		`id="admin-role-grants" class="space-y-6" hx-target:inherited="#admin-role-granted" hx-swap:inherited="outerHTML"`,
		`hx-sync="this:replace"`,
	)
	if got := strings.Count(rec.Body.String(), "hx-sync"); got != 2 {
		t.Errorf("hx-sync appears %d times, want the editor and the grant select", got)
	}
	assertNoBody(t, rec, "closest")
}
