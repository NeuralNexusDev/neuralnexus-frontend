package main

import (
	"fmt"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/p0t4t0sandwich/neuralnexus-frontend/components"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/test/testutil"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/test/testutil/fakeapi"
)

const (
	permissionListJSON    = `[{"id":"1","node":"pets.read","description":"Read pets","value_type":"","merge":""},{"id":"2","node":"pets.max","description":"Max pets","value_type":"int","merge":"max"}]`
	createdPermissionJSON = `{"id":"9","node":"pets.write","description":"Write pets","value_type":"","merge":""}`
)

func TestPermissionsListHandler(t *testing.T) {
	t.Run("PM-01_renders_every_permission_the_API_returns", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /permissions", 200, permissionListJSON)
		rec := serveRequest("GET", "/admin/permissions/list", nil)
		assertStatusCode(t, rec, 200)
		if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
			t.Errorf("Content-Type = %q", got)
		}
		assertBodyHas(t, rec, "pets.read", "pets.max", "Read pets", "Max pets", `id="permission-1"`, `id="permission-2"`, `id="admin-permission-create-form"`)
		assertBodyLacks(t, rec, "autofocus")
		if got := f.Lines(); !slices.Equal(got, []string{"GET /permissions"}) {
			t.Errorf("API calls = %q, want one GET /permissions without a body", got)
		}
	})

	t.Run("PM-02_an_empty_list_renders_the_empty_state_and_the_create_form", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /permissions", 200, `[]`)
		rec := serveRequest("GET", "/admin/permissions/list", nil)
		assertStatusCode(t, rec, 200)
		assertBodyHas(t, rec, `id="admin-permissions-empty"`, `id="admin-permission-create-form"`)
		assertBodyLacks(t, rec, `id="permission-`)
	})

	t.Run("PM-03_an_API_failure_answers_with_its_status_and_message", func(t *testing.T) {
		cases := []struct {
			name    string
			status  int
			detail  string
			message string
		}{
			{"403_with_a_detail", 403, "Missing permission", "Missing permission"},
			{"500_with_a_detail", 500, "database down", "database down"},
			{"500_without_a_detail", 500, "", "Failed to load permissions"},
		}
		for _, tc := range cases {
			t.Run("PM-03_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				if tc.detail == "" {
					f.On("GET /permissions", tc.status, ``)
				} else {
					f.Problem("GET /permissions", tc.status, tc.detail)
				}
				rec := serveRequest("GET", "/admin/permissions/list", nil)
				assertFailure(t, rec, tc.status, tc.message)
				if got := rec.Body.String(); got != tc.message {
					t.Errorf("body = %q, want exactly %q", got, tc.message)
				}
			})
		}
	})

	t.Run("PM-04_a_401_sends_the_browser_to_the_login_page", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /permissions", 401, ``)
		assertLoginRedirect(t, serveRequest("GET", "/admin/permissions/list", nil))
	})
}

func TestLoadPermissions(t *testing.T) {
	want := []components.Permission{
		{ID: "1", Node: "pets.read", Description: "Read pets"},
		{ID: "2", Node: "pets.max", Description: "Max pets", ValueType: "int", Merge: "max"},
	}

	t.Run("PM-05_decodes_the_permissions_and_passes_focusList_through", func(t *testing.T) {
		for _, focusList := range []bool{false, true} {
			t.Run(fmt.Sprintf("PM-05_focusList_%t", focusList), func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("GET /permissions", 200, permissionListJSON)
				data, err := loadPermissions(newSession(testutil.NewRequest()), focusList)
				if err != nil {
					t.Fatalf("err = %v", err)
				}
				if !slices.Equal(data.Permissions, want) {
					t.Errorf("Permissions = %+v, want %+v", data.Permissions, want)
				}
				if data.FocusList != focusList {
					t.Errorf("FocusList = %t, want %t", data.FocusList, focusList)
				}
				if got := f.Lines(); !slices.Equal(got, []string{"GET /permissions"}) {
					t.Errorf("API calls = %q, want one GET /permissions", got)
				}
			})
		}
	})

	t.Run("PM-06_an_empty_array_gives_an_empty_list", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /permissions", 200, `[]`)
		data, err := loadPermissions(newSession(testutil.NewRequest()), false)
		if err != nil || len(data.Permissions) != 0 {
			t.Errorf("Permissions = %+v, err = %v, want an empty list and no error", data.Permissions, err)
		}
	})

	t.Run("PM-07_an_API_failure_returns_an_error_and_an_empty_value", func(t *testing.T) {
		t.Run("PM-07_500", func(t *testing.T) {
			f := fakeapi.NewFakeAPI(t)
			f.Problem("GET /permissions", 500, "database down")
			data, err := loadPermissions(newSession(testutil.NewRequest()), true)
			assertAPIError(t, err, 500, "database down", "GET", "/permissions")
			if !reflect.DeepEqual(data, components.AdminPermissionsData{}) {
				t.Errorf("data = %+v, want the zero value", data)
			}
		})
		t.Run("PM-07_401", func(t *testing.T) {
			f := fakeapi.NewFakeAPI(t)
			f.On("GET /permissions", 401, ``)
			data, err := loadPermissions(newSession(testutil.NewRequest()), true)
			if err != errUnauthorized {
				t.Errorf("err = %v, want errUnauthorized", err)
			}
			if !reflect.DeepEqual(data, components.AdminPermissionsData{}) {
				t.Errorf("data = %+v, want the zero value", data)
			}
		})
	})
}

func TestPermissionsCreateHandler(t *testing.T) {
	const post = "POST /permissions "

	t.Run("PM-08_a_create_without_a_value_type_posts_node_and_description_and_reloads", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("POST /permissions", 201, createdPermissionJSON)
		f.On("GET /permissions", 200, `[`+createdPermissionJSON+`]`)
		rec := serveRequest("POST", "/admin/permissions", url.Values{"node": {"pets.write"}, "description": {"Write pets"}, "value_type": {""}, "merge": {"max"}})
		assertStatusCode(t, rec, 200)
		want := []string{post + `{"description":"Write pets","node":"pets.write"}`, "GET /permissions"}
		if got := f.Lines(); !slices.Equal(got, want) {
			t.Errorf("API calls = %q, want %q", got, want)
		}
		if got := f.Calls()[0].Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q", got)
		}
		assertBodyHas(t, rec, `id="permission-9"`, "pets.write", `id="admin-permissions-list"`, `id="admin-permission-create-form"`)
	})

	t.Run("PM-09_node_and_description_are_trimmed", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("POST /permissions", 201, createdPermissionJSON)
		f.On("GET /permissions", 200, `[]`)
		serveRequest("POST", "/admin/permissions", url.Values{"node": {"  pets.write\t"}, "description": {"\n Write pets  "}})
		if got := f.Lines(); len(got) == 0 || got[0] != post+`{"description":"Write pets","node":"pets.write"}` {
			t.Errorf("API calls = %q, want the trimmed body", got)
		}
	})

	t.Run("PM-10_an_int_permission_sends_its_value_type_and_merge_rule", func(t *testing.T) {
		for _, merge := range []string{"max", "min"} {
			t.Run("PM-10_"+merge, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("POST /permissions", 201, createdPermissionJSON)
				f.On("GET /permissions", 200, `[]`)
				serveRequest("POST", "/admin/permissions", url.Values{"node": {"pets.max"}, "description": {"Max pets"}, "value_type": {"int"}, "merge": {merge}})
				want := post + `{"description":"Max pets","merge":"` + merge + `","node":"pets.max","value_type":"int"}`
				if got := f.Lines(); len(got) == 0 || got[0] != want {
					t.Errorf("API calls = %q, want %q first", got, want)
				}
			})
		}
	})

	t.Run("PM-11_an_int_permission_without_a_merge_field_sends_an_empty_merge", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("POST /permissions", 201, createdPermissionJSON)
		f.On("GET /permissions", 200, `[]`)
		serveRequest("POST", "/admin/permissions", url.Values{"node": {"pets.max"}, "description": {"Max pets"}, "value_type": {"int"}})
		want := post + `{"description":"Max pets","merge":"","node":"pets.max","value_type":"int"}`
		if got := f.Lines(); len(got) == 0 || got[0] != want {
			t.Errorf("API calls = %q, want %q first", got, want)
		}
	})

	t.Run("PM-12_a_non-int_value_type_sends_no_merge", func(t *testing.T) {
		cases := []struct{ valueType, merge string }{
			{"string", "max"},
			{"string_list", "min"},
		}
		for _, tc := range cases {
			t.Run("PM-12_"+tc.valueType, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("POST /permissions", 201, createdPermissionJSON)
				f.On("GET /permissions", 200, `[]`)
				serveRequest("POST", "/admin/permissions", url.Values{"node": {"pets.max"}, "description": {"Max pets"}, "value_type": {tc.valueType}, "merge": {tc.merge}})
				want := post + `{"description":"Max pets","node":"pets.max","value_type":"` + tc.valueType + `"}`
				if got := f.Lines(); len(got) == 0 || got[0] != want {
					t.Errorf("API calls = %q, want %q first", got, want)
				}
			})
		}
	})

	t.Run("PM-13_a_blank_node_is_forwarded_and_the_API_refusal_is_shown", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.Problem("POST /permissions", 400, "Node is required")
		rec := serveRequest("POST", "/admin/permissions", url.Values{"node": {"   "}})
		assertStatusCode(t, rec, 400)
		assertBodyHas(t, rec, "Node is required")
		if got := f.Lines(); !slices.Equal(got, []string{post + `{"description":"","node":""}`}) {
			t.Errorf("API calls = %q, want only the POST", got)
		}
	})

	t.Run("PM-14_a_refusal_shows_the_message_and_restores_the_submitted_node", func(t *testing.T) {
		for _, status := range []int{400, 409, 422} {
			t.Run(fmt.Sprintf("PM-14_%d", status), func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.Problem("POST /permissions", status, "Node is invalid")
				rec := serveRequest("POST", "/admin/permissions", url.Values{"node": {" Bad Node "}})
				assertFailure(t, rec, status, "Node is invalid")
				assertBodyHas(t, rec, `id="admin-permission-create-node"`, `value=" Bad Node "`)
				if got := f.Lines(); len(got) != 1 || !strings.HasPrefix(got[0], post) {
					t.Errorf("API calls = %q, want only the POST", got)
				}
			})
		}
	})

	t.Run("PM-15_any_other_failure_shows_only_the_message", func(t *testing.T) {
		cases := []struct {
			name    string
			status  int
			detail  string
			message string
		}{
			{"403_with_a_detail", 403, "Missing permission", "Missing permission"},
			{"500_without_a_detail", 500, "", "Failed to create the permission"},
		}
		for _, tc := range cases {
			t.Run("PM-15_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				if tc.detail == "" {
					f.On("POST /permissions", tc.status, ``)
				} else {
					f.Problem("POST /permissions", tc.status, tc.detail)
				}
				rec := serveRequest("POST", "/admin/permissions", url.Values{"node": {"pets.write"}})
				assertFailure(t, rec, tc.status, tc.message)
				if got := rec.Body.String(); got != tc.message {
					t.Errorf("body = %q, want exactly %q", got, tc.message)
				}
				if got := f.Lines(); len(got) != 1 || !strings.HasPrefix(got[0], post) {
					t.Errorf("API calls = %q, want only the POST", got)
				}
			})
		}
	})

	t.Run("PM-16_a_401_on_the_create_sends_the_browser_to_the_login_page", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("POST /permissions", 401, ``)
		assertLoginRedirect(t, serveRequest("POST", "/admin/permissions", url.Values{"node": {"pets.write"}}))
		if got := f.Lines(); len(got) != 1 || !strings.HasPrefix(got[0], post) {
			t.Errorf("API calls = %q, want only the POST", got)
		}
	})

	t.Run("PM-17_a_failed_reload_after_a_create_reports_it_and_appends_the_row", func(t *testing.T) {
		cases := []struct {
			name    string
			detail  string
			message string
		}{
			{"with_a_detail", "database down", "The change was made, but the page could not be refreshed: database down"},
			{"without_a_detail", "", "The change was made, but the page could not be refreshed: Failed to load permissions"},
		}
		for _, tc := range cases {
			t.Run("PM-17_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("POST /permissions", 201, createdPermissionJSON)
				if tc.detail == "" {
					f.On("GET /permissions", 500, ``)
				} else {
					f.Problem("GET /permissions", 500, tc.detail)
				}
				rec := serveRequest("POST", "/admin/permissions", url.Values{"node": {"pets.write"}, "description": {"Write pets"}})
				assertFailure(t, rec, 500, tc.message)
				assertBodyHas(t, rec, `id="permission-9"`, "pets.write", `id="admin-permission-create-form"`)
				got := f.Lines()
				if len(got) != 2 || !strings.HasPrefix(got[0], post) || got[1] != "GET /permissions" {
					t.Errorf("API calls = %q, want the POST then the GET", got)
				}
			})
		}
	})

	t.Run("PM-18_a_401_on_the_reload_after_a_create_sends_the_browser_to_the_login_page", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("POST /permissions", 201, createdPermissionJSON)
		f.On("GET /permissions", 401, ``)
		assertLoginRedirect(t, serveRequest("POST", "/admin/permissions", url.Values{"node": {"pets.write"}}))
	})
}

func TestPermissionsDeleteHandler(t *testing.T) {
	t.Run("PM-19_a_delete_sends_an_empty_DELETE_then_reloads_with_focus_on_the_list", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("DELETE /permissions/7", 204, ``)
		f.On("GET /permissions", 200, permissionListJSON)
		rec := serveRequest("DELETE", "/admin/permissions/7", nil)
		assertStatusCode(t, rec, 200)
		if got := f.Lines(); !slices.Equal(got, []string{"DELETE /permissions/7", "GET /permissions"}) {
			t.Errorf("API calls = %q, want the DELETE without a body, then the GET", got)
		}
		if got := f.Calls()[0].Header.Values("Content-Type"); len(got) != 0 {
			t.Errorf("Content-Type = %q, want none", got)
		}
		assertBodyHas(t, rec, `id="admin-permissions-list"`, `id="admin-permissions-title" tabindex="-1" autofocus`)
		assertBodyLacks(t, rec, `id="permission-7"`)
	})

	t.Run("PM-20_the_permission_ID_is_path-escaped_when_it_is_sent", func(t *testing.T) {
		for _, escaped := range []string{"a%20b", "a%2Fb", "a%3Fb", "a%23b"} {
			t.Run("PM-20_"+escaped, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("DELETE /permissions/"+escaped, 204, ``)
				f.On("GET /permissions", 200, `[]`)
				rec := serveRequest("DELETE", "/admin/permissions/"+escaped, nil)
				assertStatusCode(t, rec, 200)
				if got := f.Lines(); !slices.Equal(got, []string{"DELETE /permissions/" + escaped, "GET /permissions"}) {
					t.Errorf("API calls = %q, want the escaped DELETE then the GET", got)
				}
			})
		}
	})

	t.Run("PM-21_an_API_refusal_shows_its_message_and_does_not_reload", func(t *testing.T) {
		cases := []struct {
			name    string
			status  int
			detail  string
			message string
		}{
			{"404", 404, "Permission not found", "Permission not found"},
			{"409", 409, "Permission is in use", "Permission is in use"},
			{"500_without_a_detail", 500, "", "Failed to delete the permission"},
		}
		for _, tc := range cases {
			t.Run("PM-21_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				if tc.detail == "" {
					f.On("DELETE /permissions/7", tc.status, ``)
				} else {
					f.Problem("DELETE /permissions/7", tc.status, tc.detail)
				}
				rec := serveRequest("DELETE", "/admin/permissions/7", nil)
				assertFailure(t, rec, tc.status, tc.message)
				if got := rec.Body.String(); got != tc.message {
					t.Errorf("body = %q, want exactly %q", got, tc.message)
				}
				if got := f.Lines(); !slices.Equal(got, []string{"DELETE /permissions/7"}) {
					t.Errorf("API calls = %q, want only the DELETE", got)
				}
			})
		}
	})

	t.Run("PM-22_a_401_on_the_delete_sends_the_browser_to_the_login_page", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("DELETE /permissions/7", 401, ``)
		assertLoginRedirect(t, serveRequest("DELETE", "/admin/permissions/7", nil))
		if got := f.Lines(); !slices.Equal(got, []string{"DELETE /permissions/7"}) {
			t.Errorf("API calls = %q, want only the DELETE", got)
		}
	})

	t.Run("PM-23_a_failed_reload_after_a_delete_reports_it_and_removes_the_row", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("DELETE /permissions/7", 204, ``)
		f.Problem("GET /permissions", 500, "database down")
		rec := serveRequest("DELETE", "/admin/permissions/7", nil)
		assertFailure(t, rec, 500, "The change was made, but the page could not be refreshed: database down")
		assertBodyHas(t, rec, `delete:#permission-7`)
		if got := f.Lines(); !slices.Equal(got, []string{"DELETE /permissions/7", "GET /permissions"}) {
			t.Errorf("API calls = %q, want the DELETE then the GET", got)
		}
	})

	t.Run("PM-24_a_failed_reload_after_a_delete_of_a_non-numeric_ID_has_no_row_directive", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("DELETE /permissions/abc", 204, ``)
		f.Problem("GET /permissions", 500, "database down")
		rec := serveRequest("DELETE", "/admin/permissions/abc", nil)
		message := "The change was made, but the page could not be refreshed: database down"
		assertFailure(t, rec, 500, message)
		if got := rec.Body.String(); got != message {
			t.Errorf("body = %q, want exactly %q", got, message)
		}
	})

	t.Run("PM-25_a_401_on_the_reload_after_a_delete_sends_the_browser_to_the_login_page", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("DELETE /permissions/7", 204, ``)
		f.On("GET /permissions", 401, ``)
		assertLoginRedirect(t, serveRequest("DELETE", "/admin/permissions/7", nil))
	})
}
