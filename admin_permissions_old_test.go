package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestAdminPermissionListShowsTypesAndMerge(t *testing.T) {
	f := newFakeBackend(t)
	f.on("GET /permissions", 200, permissionsJSON)
	rec := getPage("/admin/permissions/list")
	assertStatus(t, rec, http.StatusOK)
	assertBody(t, rec,
		">beenamegenerator.admin<", ">int, merge max<", ">string_list, merge union<", ">string, merge first<",
		`hx-delete="/admin/permissions/`+idPRate+`"`, `hx-confirm="Delete the permission ratelimit?"`,
		`id="admin-permission-create-form"`, `group-has-[option[value=int]:checked]:block`,
	)
	assertNoBody(t, rec, "autofocus")
}

func TestAdminPermissionListEmptyAndRefused(t *testing.T) {
	f := newFakeBackend(t)
	f.on("GET /permissions", 200, `[]`)
	assertBody(t, getPage("/admin/permissions/list"), `id="admin-permissions-empty"`, `id="admin-permission-create-form"`)

	f.problem("GET /permissions", 403, "no access")
	rec := getPage("/admin/permissions/list")
	assertStatus(t, rec, http.StatusForbidden)
	assertBody(t, rec, "no access")
	assertNoBody(t, rec, `id="admin-permission-create-form"`)
}

func TestAdminPermissionListEscapesAPIText(t *testing.T) {
	f := newFakeBackend(t)
	f.on("GET /permissions", 200, fmt.Sprintf(`[{"id":%q,"node":%q,"description":%q,"value_type":%q,"merge":%q}]`,
		hostile+"id", hostile+"node", hostile+"description", hostile+"type", hostile+"merge"))
	rec := getPage("/admin/permissions/list")
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
			f := newFakeBackend(t)
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
	f := newFakeBackend(t)
	f.problem("POST /permissions", 400, "Nodes are lower-case words")
	rec := action(http.MethodPost, "/admin/permissions", url.Values{"node": {"Bad Node"}})
	assertStatus(t, rec, http.StatusBadRequest)
	if got := bannerText(rec); got != "Nodes are lower-case words" {
		t.Errorf("body = %q", got)
	}
}

func TestAdminPermissionDeleteReloadsTheListWithFocusOnItsHeading(t *testing.T) {
	f := newFakeBackend(t)
	f.on("DELETE /permissions/"+idPStore, 204, ``)
	f.on("GET /permissions", 200, permissionsJSON)
	rec := action(http.MethodDelete, "/admin/permissions/"+idPStore, nil)
	assertStatus(t, rec, http.StatusOK)
	assertWrites(t, f, `DELETE /permissions/`+idPStore+` `)
	assertBody(t, rec, "autofocus")
}

func TestAdminPermissionDeleteRefusedShowsTheAPIMessage(t *testing.T) {
	f := newFakeBackend(t)
	f.problem("DELETE /permissions/"+idPBee, 409, "The permission is granted by a role")
	rec := action(http.MethodDelete, "/admin/permissions/"+idPBee, nil)
	assertStatus(t, rec, http.StatusConflict)
	if got := bannerText(rec); got != "The permission is granted by a role" {
		t.Errorf("body = %q", got)
	}
}

func TestAdminPermissionsRootQueuesActionsAndNamesTheTarget(t *testing.T) {
	f := newFakeBackend(t)
	f.on("GET /permissions", 200, permissionsJSON)
	rec := getPage("/admin/permissions/list")
	assertBody(t, rec, `id="admin-permissions-root" class="space-y-6" data-busy-region hx-sync:inherited="this:drop" hx-target:inherited="#admin-permissions-list" hx-swap:inherited="outerHTML"`)
	if got := strings.Count(rec.Body.String(), "hx-sync"); got != 1 {
		t.Errorf("hx-sync appears %d times, want only the root", got)
	}
	assertNoBody(t, rec, "closest")
}

func TestAdminPermissionDeleteReloadFailureRemovesTheRow(t *testing.T) {
	f := newFakeBackend(t)
	f.on("DELETE /permissions/"+idPStore, 204, ``)
	f.problem("GET /permissions", 500, "down")
	rec := action(http.MethodDelete, "/admin/permissions/"+idPStore, nil)
	assertBody(t, rec, `hx-swap-oob="delete:#permission-`+idPStore+`"`)
}

func TestAdminPermissionCreateReloadFailureListsTheNewPermission(t *testing.T) {
	f := newFakeBackend(t)
	f.on("POST /permissions", 201, `{"id":"`+idPStore+`","node":"pets.write","description":"Write pets"}`)
	f.problem("GET /permissions", 500, "down")
	rec := action(http.MethodPost, "/admin/permissions", url.Values{"node": {"pets.write"}})
	assertBody(t, rec, `hx-swap-oob="beforeend:#admin-permissions"`, `id="permission-`+idPStore+`"`, `hx-swap-oob="delete:#admin-permissions-empty"`, ">pets.write<")
}

func TestAdminPermissionCreateReloadFailureClearsTheForm(t *testing.T) {
	f := newFakeBackend(t)
	f.on("POST /permissions", 201, `{}`)
	f.problem("GET /permissions", 500, "down")
	rec := action(http.MethodPost, "/admin/permissions", url.Values{"node": {"pets.write"}})
	assertBody(t, rec, `id="admin-permission-create-form" hx-swap-oob="true"`)
}
