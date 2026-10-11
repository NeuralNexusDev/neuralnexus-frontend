package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/a-h/templ"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/components"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/test/testutil"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/test/testutil/fakeapi"
)

const (
	catalogueFixture = `[{"id":"1","node":"pets.read","description":"Read pets","value_type":"","merge":""},{"id":"2","node":"pets.max","description":"Max pets","value_type":"int","merge":"max"},{"id":"3","node":"pets.tags","description":"Pet tags","value_type":"string_list","merge":"union"},{"id":"4","node":"pets.motto","description":"Pet motto","value_type":"string","merge":"first"}]`

	intRefusal    = "Enter a whole number from -9007199254740992 to 9007199254740992"
	typeRefusal   = "This permission has a value type that cannot be edited here"
	restoreMarker = `<i id="restore-marker"></i>`
)

var roleR5 = roleFixture("5", "admins", "Site admins", grantFixture("1", "null"), grantFixture("2", "7"))

func roleFixture(id, name, description string, granted ...string) string {
	return fmt.Sprintf(`{"id":%q,"name":%q,"description":%q,"permissions":[%s]}`, id, name, description, strings.Join(granted, ","))
}

func grantFixture(permissionID, valueJSON string) string {
	var catalogue []components.Permission
	if err := json.Unmarshal([]byte(catalogueFixture), &catalogue); err != nil {
		panic(err)
	}
	for _, permission := range catalogue {
		if permission.ID == permissionID {
			encoded, err := json.Marshal(components.RolePermission{Permission: permission, Value: json.RawMessage(valueJSON)})
			if err != nil {
				panic(err)
			}
			return string(encoded)
		}
	}
	panic("grantFixture: no permission " + permissionID + " in the catalogue")
}

func permissionFixture(id, node, valueType string) string {
	return fmt.Sprintf(`{"id":%q,"node":%q,"description":"","value_type":%q,"merge":""}`, id, node, valueType)
}

func newRoleRequest(id string) *http.Request {
	r := testutil.NewRequest()
	r.SetPathValue("id", id)
	return r
}

func countingRestore(calls *int) func() []templ.Component {
	return func() []templ.Component {
		*calls++
		return []templ.Component{templ.Raw(restoreMarker)}
	}
}

func assertHTMLContent(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/html; charset=utf-8", got)
	}
}

func TestAdminRolesListHandler(t *testing.T) {
	t.Run("RL-01_the_list_renders_every_role_with_its_link_and_the_create_form", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /roles", 200, "["+roleFixture("5", "admins", "Site admins", grantFixture("1", "null"))+","+roleFixture("6", "guests", "")+"]")
		rec := serveRequest("GET", "/admin/roles/list", nil)
		assertStatusCode(t, rec, 200)
		assertHTMLContent(t, rec)
		assertBodyHas(t, rec, "admins", "Site admins", "guests", "/admin/roles/5", "/admin/roles/6", "pets.read", `id="admin-role-create-form"`)
		if body := rec.Body.String(); strings.Index(body, "admins") > strings.Index(body, "guests") {
			t.Errorf("admins comes after guests:\n%s", body)
		}
		f.AssertLines(t, "GET /roles")
	})

	t.Run("RL-02_an_empty_role_list_renders_the_empty_state_and_the_create_form", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /roles", 200, `[]`)
		rec := serveRequest("GET", "/admin/roles/list", nil)
		assertStatusCode(t, rec, 200)
		assertBodyHas(t, rec, `id="admin-roles-empty"`, `id="admin-role-create-form"`)
		assertBodyLacks(t, rec, "/admin/roles/")
	})

	t.Run("RL-03_an_API_failure_answers_with_its_status_and_message_and_no_list", func(t *testing.T) {
		cases := []struct {
			name    string
			status  int
			detail  string
			message string
		}{
			{"403", 403, "Missing permission", "Missing permission"},
			{"500_with_a_detail", 500, "database down", "database down"},
			{"500_without_a_detail", 500, "", "Failed to load roles"},
		}
		for _, tc := range cases {
			t.Run("RL-03_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.Refuse("GET /roles", tc.status, tc.detail)
				rec := serveRequest("GET", "/admin/roles/list", nil)
				assertFailure(t, rec, tc.status, tc.message)
				if got := rec.Body.String(); got != tc.message {
					t.Errorf("body = %q, want %q", got, tc.message)
				}
			})
		}
	})

	t.Run("RL-04_a_401_from_the_API_sends_the_browser_to_the_login_page", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.Refuse("GET /roles", 401, "")
		assertLoginRedirect(t, serveRequest("GET", "/admin/roles/list", nil))
	})
}

func TestAdminRoleCreateHandler(t *testing.T) {
	created := roleFixture("8", "moderator", "Chat staff")

	t.Run("RL-05_a_create_posts_the_name_and_description_and_redirects_to_the_new_role", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("POST /roles", 201, created)
		rec := serveRequest("POST", "/admin/roles", url.Values{"name": {"moderator"}, "description": {"Chat staff"}})
		f.AssertLines(t, `POST /roles {"description":"Chat staff","name":"moderator"}`)
		fakeapi.AssertContentType(t, f.Calls()[0], "application/json")
		assertStatusCode(t, rec, 200)
		if got := rec.Header().Get("HX-Redirect"); got != "/admin/roles/8" {
			t.Errorf("HX-Redirect = %q, want /admin/roles/8", got)
		}
		if rec.Body.Len() != 0 {
			t.Errorf("body = %q, want empty", rec.Body.String())
		}
	})

	t.Run("RL-06_name_and_description_are_trimmed_before_they_are_sent", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("POST /roles", 201, created)
		serveRequest("POST", "/admin/roles", url.Values{"name": {"  moderator\t"}, "description": {"\n Chat staff  "}})
		f.AssertLines(t, `POST /roles {"description":"Chat staff","name":"moderator"}`)
	})

	t.Run("RL-07_a_blank_name_and_a_missing_description_are_forwarded_and_the_refusal_is_shown", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.Refuse("POST /roles", 400, "Name is required")
		rec := serveRequest("POST", "/admin/roles", url.Values{"name": {"  "}})
		f.AssertLines(t, `POST /roles {"description":"","name":""}`)
		assertFailure(t, rec, 400, "Name is required")
	})

	t.Run("RL-08_the_created_roles_ID_is_path_escaped_in_the_redirect", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("POST /roles", 201, roleFixture("a/b c", "moderator", ""))
		rec := serveRequest("POST", "/admin/roles", url.Values{"name": {"moderator"}})
		if got := rec.Header().Get("HX-Redirect"); got != "/admin/roles/a%2Fb%20c" {
			t.Errorf("HX-Redirect = %q, want /admin/roles/a%%2Fb%%20c", got)
		}
	})

	t.Run("RL-09_a_refusal_shows_the_message_and_restores_the_name_field_with_the_submitted_text", func(t *testing.T) {
		for _, status := range []int{400, 409, 422} {
			t.Run(fmt.Sprintf("RL-09_%d", status), func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.Refuse("POST /roles", status, "Name is invalid")
				rec := serveRequest("POST", "/admin/roles", url.Values{"name": {" Bad Name "}, "description": {"Keep me"}})
				assertFailure(t, rec, status, "Name is invalid")
				assertBodyHas(t, rec, `id="admin-role-create-name"`, `value=" Bad Name "`, `id="admin-role-create-name-error"`)
				assertOutOfBand(t, rec, "admin-role-create-name-field", true)
				if got := rec.Header().Get("HX-Redirect"); got != "" {
					t.Errorf("HX-Redirect = %q, want none", got)
				}
				f.AssertLines(t, `POST /roles {"description":"Keep me","name":"Bad Name"}`)
			})
		}
	})

	t.Run("RL-10_any_other_API_failure_shows_only_the_message_and_restores_nothing", func(t *testing.T) {
		cases := []struct {
			name    string
			status  int
			detail  string
			message string
		}{
			{"403", 403, "Missing permission", "Missing permission"},
			{"500_without_a_detail", 500, "", "Failed to create the role"},
		}
		for _, tc := range cases {
			t.Run("RL-10_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.Refuse("POST /roles", tc.status, tc.detail)
				rec := serveRequest("POST", "/admin/roles", url.Values{"name": {"moderator"}})
				assertFailure(t, rec, tc.status, tc.message)
				if got := rec.Body.String(); got != tc.message {
					t.Errorf("body = %q, want %q", got, tc.message)
				}
				if got := rec.Header().Get("HX-Redirect"); got != "" {
					t.Errorf("HX-Redirect = %q, want none", got)
				}
			})
		}
	})

	t.Run("RL-11_a_401_on_the_create_sends_the_browser_to_the_login_page", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.Refuse("POST /roles", 401, "")
		assertLoginRedirect(t, serveRequest("POST", "/admin/roles", url.Values{"name": {"moderator"}}))
	})
}

func TestLoadRole(t *testing.T) {
	session := func() apiSession { return newSession(testutil.NewRequest()) }

	t.Run("RL-12_the_role_decodes_into_the_editor_data_with_no_catalogue", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /roles/5", 200, roleR5)
		data, err := loadRole(session(), "5")
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if data.Role.ID != "5" || data.Role.Name != "admins" || data.Role.Description != "Site admins" || len(data.Role.Permissions) != 2 {
			t.Fatalf("Role = %+v", data.Role)
		}
		if first, second := data.Role.Permissions[0], data.Role.Permissions[1]; first.Node != "pets.read" || second.Node != "pets.max" || second.ValueType != "int" || string(second.Value) != "7" {
			t.Errorf("Permissions = %+v", data.Role.Permissions)
		}
		if len(data.Catalogue) != 0 || len(data.Drafts) != 0 || data.GrantPermission != "" || data.GrantValue != "" || data.FocusList {
			t.Errorf("data = %+v, want no catalogue, drafts, grant fields or focus", data)
		}
		f.AssertLines(t, "GET /roles/5")
	})

	t.Run("RL-13_the_role_ID_is_path_escaped_in_the_API_path", func(t *testing.T) {
		for _, tc := range idEscapes {
			t.Run("RL-13_"+tc.escaped, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("GET /roles/"+tc.escaped, 200, `{}`)
				if _, err := loadRole(session(), tc.id); err != nil {
					t.Fatalf("err = %v", err)
				}
				f.AssertLines(t, "GET /roles/"+tc.escaped)
			})
		}
	})

	t.Run("RL-14_an_API_failure_returns_the_error_and_empty_data", func(t *testing.T) {
		cases := []struct {
			name    string
			status  int
			detail  string
			message string
		}{
			{"404", 404, "Role not found", "Role not found"},
			{"500_without_a_detail", 500, "", "Failed to load the role"},
			{"401", 401, "", ""},
		}
		for _, tc := range cases {
			t.Run("RL-14_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.Refuse("GET /roles/5", tc.status, tc.detail)
				data, err := loadRole(session(), "5")
				if tc.status == 401 {
					if err != errUnauthorized {
						t.Errorf("err = %v, want errUnauthorized", err)
					}
				} else {
					assertAPIError(t, err, tc.status, tc.message, "GET", "/roles/5")
				}
				if !reflect.DeepEqual(data, components.AdminRoleData{}) {
					t.Errorf("data = %+v, want the zero value", data)
				}
			})
		}
	})

	t.Run("RL-15_a_200_body_that_does_not_decode_gives_a_502_and_empty_data", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /roles/5", 200, `not json`)
		data, err := loadRole(session(), "5")
		assertAPIError(t, err, 502, "Failed to load the role", "GET", "/roles/5")
		if !reflect.DeepEqual(data, components.AdminRoleData{}) {
			t.Errorf("data = %+v, want the zero value", data)
		}
	})
}

func TestLoadRoleEditor(t *testing.T) {
	session := func() apiSession { return newSession(testutil.NewRequest()) }

	t.Run("RL-16_the_role_and_the_permission_catalogue_are_both_loaded_role_first", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /roles/5", 200, roleR5)
		f.On("GET /permissions", 200, catalogueFixture)
		data, err := loadRoleEditor(session(), "5")
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if got := testutil.JSONString(t, data.Role); got != roleR5 {
			t.Errorf("Role = %s, want %s", got, roleR5)
		}
		if len(data.Catalogue) != 4 || data.Catalogue[0].Node != "pets.read" || data.Catalogue[2].ValueType != "string_list" || data.Catalogue[3].Node != "pets.motto" {
			t.Errorf("Catalogue = %+v", data.Catalogue)
		}
		f.AssertLines(t, "GET /roles/5", "GET /permissions")
	})

	t.Run("RL-17_a_failed_role_load_returns_that_error_and_does_not_load_the_catalogue", func(t *testing.T) {
		for _, status := range []int{404, 401} {
			t.Run(fmt.Sprintf("RL-17_%d", status), func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.Refuse("GET /roles/5", status, map[int]string{404: "Role not found", 401: ""}[status])
				data, err := loadRoleEditor(session(), "5")
				if status == 401 {
					if err != errUnauthorized {
						t.Errorf("err = %v, want errUnauthorized", err)
					}
				} else {
					assertAPIError(t, err, 404, "Role not found", "GET", "/roles/5")
				}
				if !reflect.DeepEqual(data, components.AdminRoleData{}) {
					t.Errorf("data = %+v, want the zero value", data)
				}
				f.AssertLines(t, "GET /roles/5")
			})
		}
	})

	t.Run("RL-18_a_failed_catalogue_load_returns_that_error_and_discards_the_loaded_role", func(t *testing.T) {
		cases := []struct {
			name    string
			status  int
			detail  string
			message string
		}{
			{"403", 403, "Missing permission", "Missing permission"},
			{"500_without_a_detail", 500, "", "Failed to load permissions"},
			{"401", 401, "", ""},
		}
		for _, tc := range cases {
			t.Run("RL-18_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("GET /roles/5", 200, roleR5)
				f.Refuse("GET /permissions", tc.status, tc.detail)
				data, err := loadRoleEditor(session(), "5")
				if tc.status == 401 {
					if err != errUnauthorized {
						t.Errorf("err = %v, want errUnauthorized", err)
					}
				} else {
					assertAPIError(t, err, tc.status, tc.message, "GET", "/permissions")
				}
				if !reflect.DeepEqual(data, components.AdminRoleData{}) {
					t.Errorf("data = %+v, want the zero value", data)
				}
			})
		}
	})
}

func TestAdminRoleEditorHandler(t *testing.T) {
	t.Run("RL-19_the_editor_renders_the_role_its_granted_permissions_and_a_grant_form_for_the_rest", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /roles/5", 200, roleR5)
		f.On("GET /permissions", 200, catalogueFixture)
		rec := serveRequest("GET", "/admin/roles/5/editor", nil)
		assertStatusCode(t, rec, 200)
		assertHTMLContent(t, rec)
		assertBodyHas(t, rec, "admins", "Site admins", `id="granted-1"`, `id="granted-2"`, `id="admin-role-grant-form"`, ">pets.tags</option>", ">pets.motto</option>")
		assertBodyLacks(t, rec, ">pets.read</option>", ">pets.max</option>", "autofocus")
		f.AssertLines(t, "GET /roles/5", "GET /permissions")
	})

	t.Run("RL-20_a_role_with_no_grants_offers_every_permission_and_a_role_with_every_grant_shows_no_form", func(t *testing.T) {
		everything := roleFixture("5", "admins", "Site admins", grantFixture("1", "null"), grantFixture("2", "7"), grantFixture("3", `["a"]`), grantFixture("4", `"hi"`))
		cases := []struct {
			name       string
			role       string
			has, lacks []string
		}{
			{"no_grants", roleFixture("5", "admins", "Site admins"),
				[]string{`id="admin-role-permissions-empty"`, ">pets.read</option>", ">pets.max</option>", ">pets.tags</option>", ">pets.motto</option>"},
				[]string{"autofocus"}},
			{"every_grant", everything,
				[]string{`id="admin-role-grant-empty"`},
				[]string{`id="admin-role-grant-form"`, "autofocus"}},
		}
		for _, tc := range cases {
			t.Run("RL-20_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("GET /roles/5", 200, tc.role)
				f.On("GET /permissions", 200, catalogueFixture)
				rec := serveRequest("GET", "/admin/roles/5/editor", nil)
				assertStatusCode(t, rec, 200)
				assertBodyHas(t, rec, tc.has...)
				assertBodyLacks(t, rec, tc.lacks...)
			})
		}
	})

	t.Run("RL-21_the_role_ID_from_the_path_is_decoded_and_escaped_again_for_the_API", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /roles/a%2Fb", 200, roleFixture("a/b", "admins", ""))
		f.On("GET /permissions", 200, catalogueFixture)
		rec := serveRequest("GET", "/admin/roles/a%2Fb/editor", nil)
		assertStatusCode(t, rec, 200)
		f.AssertLines(t, "GET /roles/a%2Fb", "GET /permissions")
	})

	t.Run("RL-22_a_failed_role_load_answers_with_its_status_and_message_and_nothing_else", func(t *testing.T) {
		cases := []struct {
			name    string
			status  int
			detail  string
			message string
		}{
			{"404", 404, "Role not found", "Role not found"},
			{"500_without_a_detail", 500, "", "Failed to load the role"},
		}
		for _, tc := range cases {
			t.Run("RL-22_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.Refuse("GET /roles/5", tc.status, tc.detail)
				rec := serveRequest("GET", "/admin/roles/5/editor", nil)
				assertFailure(t, rec, tc.status, tc.message)
				if got := rec.Body.String(); got != tc.message {
					t.Errorf("body = %q, want %q", got, tc.message)
				}
				f.AssertLines(t, "GET /roles/5")
			})
		}
	})

	t.Run("RL-23_a_failed_catalogue_load_answers_with_its_status_and_message_and_draws_no_editor", func(t *testing.T) {
		cases := []struct {
			name    string
			status  int
			detail  string
			message string
		}{
			{"403", 403, "Missing permission", "Missing permission"},
			{"500_without_a_detail", 500, "", "Failed to load permissions"},
		}
		for _, tc := range cases {
			t.Run("RL-23_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("GET /roles/5", 200, roleR5)
				f.Refuse("GET /permissions", tc.status, tc.detail)
				rec := serveRequest("GET", "/admin/roles/5/editor", nil)
				assertFailure(t, rec, tc.status, tc.message)
				if got := rec.Body.String(); got != tc.message {
					t.Errorf("body = %q, want %q", got, tc.message)
				}
				assertBodyLacks(t, rec, "admins")
			})
		}
	})

	t.Run("RL-24_a_401_on_either_call_sends_the_browser_to_the_login_page", func(t *testing.T) {
		for _, failing := range []string{"role", "catalogue"} {
			t.Run("RL-24_"+failing, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				if failing == "role" {
					f.Refuse("GET /roles/5", 401, "")
				} else {
					f.On("GET /roles/5", 200, roleR5)
					f.Refuse("GET /permissions", 401, "")
				}
				assertLoginRedirect(t, serveRequest("GET", "/admin/roles/5/editor", nil))
			})
		}
	})
}

func TestAdminRoleSaveHandler(t *testing.T) {
	saveForm := func(name, loadedName, description, loadedDescription string) url.Values {
		return url.Values{"name": {name}, "loaded_name": {loadedName}, "description": {description}, "loaded_description": {loadedDescription}}
	}
	save := func(f *fakeapi.FakeAPI, form url.Values) *httptest.ResponseRecorder {
		f.On("PATCH /roles/5", 200, roleFixture("5", "moderators", "Site admins"))
		return serveRequest("POST", "/admin/roles/5", form)
	}

	t.Run("RL-25_a_changed_name_alone_is_sent_as_a_PATCH_with_only_the_name_and_the_editor_parts_are_redrawn", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		rec := save(f, saveForm("moderators", "admins", "Site admins", "Site admins"))
		f.AssertLines(t, `PATCH /roles/5 {"name":"moderators"}`)
		fakeapi.AssertContentType(t, f.Calls()[0], "application/json")
		assertStatusCode(t, rec, 200)
		assertHTMLContent(t, rec)
		assertBodyHas(t, rec, "moderators", `id="admin-role-form"`, `id="admin-role-header"`, `id="admin-role-delete"`, `hx-swap-oob="innerHTML">Saved</p>`)
		assertOutOfBand(t, rec, "admin-role-header", true)
		assertOutOfBand(t, rec, "admin-role-delete", true)
	})

	t.Run("RL-26_a_changed_description_alone_is_sent_with_only_the_description", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		rec := save(f, saveForm("admins", "admins", "Site staff", "Site admins"))
		f.AssertLines(t, `PATCH /roles/5 {"description":"Site staff"}`)
		assertStatusCode(t, rec, 200)
		assertBodyHas(t, rec, "Saved")
	})

	t.Run("RL-27_a_changed_name_and_a_changed_description_are_sent_together", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		save(f, saveForm("moderators", "admins", "Site staff", "Site admins"))
		f.AssertLines(t, `PATCH /roles/5 {"description":"Site staff","name":"moderators"}`)
	})

	t.Run("RL-28_name_and_description_are_trimmed_before_they_are_sent", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		save(f, saveForm("  moderators\t", "admins", "\n Site staff  ", "Site admins"))
		f.AssertLines(t, `PATCH /roles/5 {"description":"Site staff","name":"moderators"}`)
	})

	t.Run("RL-29_a_cleared_description_is_sent_as_an_empty_string", func(t *testing.T) {
		for name, description := range map[string]string{"empty": "", "spaces": "  "} {
			t.Run("RL-29_"+name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				save(f, saveForm("admins", "admins", description, "Site admins"))
				f.AssertLines(t, `PATCH /roles/5 {"description":""}`)
			})
		}
	})

	t.Run("RL-30_a_name_that_differs_only_by_surrounding_spaces_counts_as_a_change_and_is_sent_trimmed", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		save(f, saveForm(" admins ", "admins", "Site admins", "Site admins"))
		f.AssertLines(t, `PATCH /roles/5 {"name":"admins"}`)
	})

	t.Run("RL-31_the_redrawn_editor_shows_what_the_API_answered_not_what_was_typed", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("PATCH /roles/5", 200, roleFixture("5", "admins-two", "Site admins"))
		rec := serveRequest("POST", "/admin/roles/5", saveForm("admins2", "admins", "Site admins", "Site admins"))
		assertBodyHas(t, rec, "admins-two")
		assertBodyLacks(t, rec, "admins2")
	})

	t.Run("RL-32_an_emptied_name_is_refused_before_any_API_call_and_the_field_is_restored_with_the_typed_text", func(t *testing.T) {
		for name, typed := range map[string]string{"empty": "", "spaces": "  "} {
			t.Run("RL-32_"+name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				rec := serveRequest("POST", "/admin/roles/5", saveForm(typed, "admins", "Site admins", "Site admins"))
				assertFailure(t, rec, 400, "Enter a name")
				assertBodyHas(t, rec, `id="admin-role-name"`, `id="admin-role-name-error"`)
				if typed != "" {
					assertBodyHas(t, rec, `value="  "`)
				}
				f.AssertLines(t)
			})
		}
	})

	t.Run("RL-33_an_emptied_name_blocks_the_whole_save_so_a_changed_description_is_not_sent_either", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		rec := serveRequest("POST", "/admin/roles/5", saveForm("", "admins", "Site staff", "Site admins"))
		assertFailure(t, rec, 400, "Enter a name")
		f.AssertLines(t)
	})

	t.Run("RL-34_a_form_with_no_change_answers_Nothing_to_save_and_sends_nothing", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		rec := serveRequest("POST", "/admin/roles/5", saveForm("admins", "admins", "Site admins", "Site admins"))
		assertStatusCode(t, rec, 200)
		if got := rec.Header().Get("HX-Reswap"); got != "none" {
			t.Errorf("HX-Reswap = %q, want none", got)
		}
		assertBodyHas(t, rec, "Nothing to save")
		f.AssertLines(t)
	})

	t.Run("RL-35_a_refusal_of_a_sent_name_shows_the_message_and_restores_the_name_field_with_the_typed_text", func(t *testing.T) {
		for _, status := range []int{400, 409, 422} {
			t.Run(fmt.Sprintf("RL-35_%d", status), func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.Refuse("PATCH /roles/5", status, "Name is taken")
				rec := serveRequest("POST", "/admin/roles/5", saveForm(" moderators", "admins", "Site admins", "Site admins"))
				assertFailure(t, rec, status, "Name is taken")
				assertBodyHas(t, rec, `id="admin-role-name"`, `id="admin-role-name-error"`, `value=" moderators"`)
				assertOutOfBand(t, rec, "admin-role-name-field", true)
				assertBodyLacks(t, rec, "Saved")
				f.AssertLines(t, `PATCH /roles/5 {"name":"moderators"}`)
			})
		}
	})

	t.Run("RL-36_a_refusal_when_only_the_description_was_sent_does_not_touch_the_name_field", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.Refuse("PATCH /roles/5", 400, "Description is invalid")
		rec := serveRequest("POST", "/admin/roles/5", saveForm("admins", "admins", "Site staff", "Site admins"))
		assertFailure(t, rec, 400, "Description is invalid")
		assertBodyLacks(t, rec, "admin-role-name")
	})

	t.Run("RL-37_any_other_API_failure_shows_only_the_message_and_restores_nothing", func(t *testing.T) {
		cases := []struct {
			name    string
			status  int
			detail  string
			message string
		}{
			{"403", 403, "Missing permission", "Missing permission"},
			{"404", 404, "Role not found", "Role not found"},
			{"500_without_a_detail", 500, "", "Failed to save the role"},
		}
		for _, tc := range cases {
			t.Run("RL-37_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.Refuse("PATCH /roles/5", tc.status, tc.detail)
				rec := serveRequest("POST", "/admin/roles/5", saveForm("moderators", "admins", "Site admins", "Site admins"))
				assertFailure(t, rec, tc.status, tc.message)
				assertBodyLacks(t, rec, "admin-role-name", "Saved")
			})
		}
	})

	t.Run("RL-38_a_401_on_the_save_sends_the_browser_to_the_login_page", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.Refuse("PATCH /roles/5", 401, "")
		assertLoginRedirect(t, serveRequest("POST", "/admin/roles/5", saveForm("moderators", "admins", "Site admins", "Site admins")))
	})

	t.Run("RL-39_the_role_ID_is_path_escaped_when_it_is_sent_to_the_API", func(t *testing.T) {
		for _, tc := range idEscapes[:4] {
			t.Run("RL-39_"+tc.escaped, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("PATCH /roles/"+tc.escaped, 200, roleFixture(tc.id, "moderators", ""))
				serveRequest("POST", "/admin/roles/"+tc.escaped, saveForm("moderators", "admins", "", ""))
				f.AssertLines(t, "PATCH /roles/"+tc.escaped+` {"name":"moderators"}`)
			})
		}
	})
}

func TestAdminRoleDeleteHandler(t *testing.T) {
	t.Run("RL-40_a_delete_sends_an_empty_DELETE_and_redirects_the_browser_to_the_role_list", func(t *testing.T) {
		cases := []struct {
			name   string
			status int
			body   string
		}{
			{"204", 204, ""},
			{"200_with_a_body", 200, `{}`},
		}
		for _, tc := range cases {
			t.Run("RL-40_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("DELETE /roles/5", tc.status, tc.body)
				rec := serveRequest("DELETE", "/admin/roles/5", nil)
				f.AssertLines(t, "DELETE /roles/5")
				fakeapi.AssertContentType(t, f.Calls()[0], "")
				assertStatusCode(t, rec, 200)
				if got := rec.Header().Get("HX-Redirect"); got != "/admin/roles" {
					t.Errorf("HX-Redirect = %q, want /admin/roles", got)
				}
				if rec.Body.Len() != 0 {
					t.Errorf("body = %q, want empty", rec.Body.String())
				}
			})
		}
	})

	t.Run("RL-41_the_role_ID_is_path_escaped_when_it_is_sent_to_the_API", func(t *testing.T) {
		for _, tc := range idEscapes[:4] {
			t.Run("RL-41_"+tc.escaped, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("DELETE /roles/"+tc.escaped, 204, ``)
				serveRequest("DELETE", "/admin/roles/"+tc.escaped, nil)
				f.AssertLines(t, "DELETE /roles/"+tc.escaped)
			})
		}
	})

	t.Run("RL-42_an_API_refusal_shows_its_message_and_does_not_redirect", func(t *testing.T) {
		cases := []struct {
			name    string
			status  int
			detail  string
			message string
		}{
			{"404", 404, "Role not found", "Role not found"},
			{"409", 409, "Role is held by users", "Role is held by users"},
			{"500_without_a_detail", 500, "", "Failed to delete the role"},
		}
		for _, tc := range cases {
			t.Run("RL-42_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.Refuse("DELETE /roles/5", tc.status, tc.detail)
				rec := serveRequest("DELETE", "/admin/roles/5", nil)
				assertFailure(t, rec, tc.status, tc.message)
				assertBodyHas(t, rec, `id="admin-role-status"`)
				if got := rec.Header().Get("HX-Redirect"); got != "" {
					t.Errorf("HX-Redirect = %q, want none", got)
				}
				f.AssertLines(t, "DELETE /roles/5")
			})
		}
	})

	t.Run("RL-43_a_401_on_the_delete_sends_the_browser_to_the_login_page", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.Refuse("DELETE /roles/5", 401, "")
		assertLoginRedirect(t, serveRequest("DELETE", "/admin/roles/5", nil))
	})
}

func TestGrantBody(t *testing.T) {
	encode := func(t *testing.T, permission components.Permission, raw string) string {
		t.Helper()
		body, err := grantBody(permission, raw)
		if err != nil {
			t.Fatalf("grantBody(%q, %q) error = %v", permission.ValueType, raw, err)
		}
		return testutil.JSONString(t, body)
	}
	refused := func(t *testing.T, permission components.Permission, raw string, message string) {
		t.Helper()
		body, err := grantBody(permission, raw)
		if body != nil {
			t.Errorf("body = %#v, want nil", body)
		}
		assertAPIError(t, err, 400, message, "", "")
	}
	typed := func(valueType string) components.Permission {
		return components.Permission{ID: "9", ValueType: valueType}
	}

	t.Run("RL-44_a_permission_with_no_value_type_is_granted_as_is_and_sends_no_body", func(t *testing.T) {
		for name, raw := range map[string]string{"empty": "", "text": "ignored", "spaces": "  "} {
			t.Run("RL-44_"+name, func(t *testing.T) {
				body, err := grantBody(typed(""), raw)
				if body != nil || err != nil {
					t.Errorf("body = %#v, err = %v, want nil and nil", body, err)
				}
			})
		}
	})

	t.Run("RL-45_an_int_value_is_trimmed_and_sent_as_a_JSON_number", func(t *testing.T) {
		cases := []struct{ name, raw, want string }{
			{"plain", "42", `{"value":42}`},
			{"padded", " 42 ", `{"value":42}`},
			{"negative", "-7", `{"value":-7}`},
			{"zero", "0", `{"value":0}`},
			{"tab_and_newline", "\t8\n", `{"value":8}`},
		}
		for _, tc := range cases {
			t.Run("RL-45_"+tc.name, func(t *testing.T) {
				if got := encode(t, typed("int"), tc.raw); got != tc.want {
					t.Errorf("encoded = %s, want %s", got, tc.want)
				}
			})
		}
	})

	t.Run("RL-46_the_largest_whole_number_the_API_accepts_is_allowed_in_both_directions", func(t *testing.T) {
		cases := []struct{ name, raw, want string }{
			{"positive", "9007199254740992", `{"value":9007199254740992}`},
			{"negative", "-9007199254740992", `{"value":-9007199254740992}`},
		}
		for _, tc := range cases {
			t.Run("RL-46_"+tc.name, func(t *testing.T) {
				if got := encode(t, typed("int"), tc.raw); got != tc.want {
					t.Errorf("encoded = %s, want %s", got, tc.want)
				}
			})
		}
	})

	t.Run("RL-47_a_whole_number_beyond_the_bound_is_refused", func(t *testing.T) {
		cases := []struct{ name, raw string }{
			{"just_over", "9007199254740993"},
			{"just_under", "-9007199254740993"},
			{"max_int64", "9223372036854775807"},
			{"over_int64", "9223372036854775808"},
			{"under_int64", "-9223372036854775809"},
			{"twenty_digits", "99999999999999999999"},
		}
		for _, tc := range cases {
			t.Run("RL-47_"+tc.name, func(t *testing.T) {
				refused(t, typed("int"), tc.raw, intRefusal)
			})
		}
	})

	t.Run("RL-48_text_that_is_not_a_canonical_whole_number_is_refused", func(t *testing.T) {
		cases := []struct{ name, raw string }{
			{"empty", ""},
			{"spaces", "   "},
			{"letters", "abc"},
			{"decimal", "4.5"},
			{"exponent", "1e3"},
			{"plus_sign", "+5"},
			{"leading_zeros", "007"},
			{"negative_zero", "-0"},
			{"hex", "0x10"},
			{"inner_space", "1 2"},
			{"thousands_separator", "1,000"},
			{"double_minus", "--1"},
		}
		for _, tc := range cases {
			t.Run("RL-48_"+tc.name, func(t *testing.T) {
				refused(t, typed("int"), tc.raw, intRefusal)
			})
		}
	})

	t.Run("RL-49_a_string_list_is_split_on_line_breaks_one_item_per_line", func(t *testing.T) {
		if got := encode(t, typed("string_list"), "a\nb\nc"); got != `{"value":["a","b","c"]}` {
			t.Errorf("encoded = %s", got)
		}
	})

	t.Run("RL-50_list_items_are_trimmed_blank_lines_dropped_order_and_duplicates_kept_and_CRLF_works", func(t *testing.T) {
		if got := encode(t, typed("string_list"), " a \r\n\r\n b\r\n\n a\n"); got != `{"value":["a","b","a"]}` {
			t.Errorf("encoded = %s", got)
		}
	})

	t.Run("RL-51_a_list_with_no_non_blank_line_is_refused", func(t *testing.T) {
		cases := []struct{ name, raw string }{
			{"empty", ""},
			{"newline", "\n"},
			{"blank_lines", "  \r\n \t\n"},
		}
		for _, tc := range cases {
			t.Run("RL-51_"+tc.name, func(t *testing.T) {
				refused(t, typed("string_list"), tc.raw, "Enter at least one item")
			})
		}
	})

	t.Run("RL-52_a_string_value_is_trimmed_and_its_inner_line_break_is_kept", func(t *testing.T) {
		cases := []struct{ name, raw, want string }{
			{"plain", "hello", `{"value":"hello"}`},
			{"padded", "  hello world  ", `{"value":"hello world"}`},
			{"inner_line_break", "a\nb", `{"value":"a\nb"}`},
		}
		for _, tc := range cases {
			t.Run("RL-52_"+tc.name, func(t *testing.T) {
				if got := encode(t, typed("string"), tc.raw); got != tc.want {
					t.Errorf("encoded = %s, want %s", got, tc.want)
				}
			})
		}
	})

	t.Run("RL-53_a_string_with_only_whitespace_is_refused", func(t *testing.T) {
		cases := []struct{ name, raw string }{
			{"empty", ""},
			{"spaces", "   "},
			{"mixed", "\t\n "},
		}
		for _, tc := range cases {
			t.Run("RL-53_"+tc.name, func(t *testing.T) {
				refused(t, typed("string"), tc.raw, "Enter a value")
			})
		}
	})

	t.Run("RL-54_a_value_type_the_form_cannot_edit_is_refused", func(t *testing.T) {
		cases := []struct{ name, valueType string }{
			{"bool", "bool"},
			{"float", "float"},
			{"capitalised_string", "String"},
			{"upper_case_int", "INT"},
			{"padded_int", " int"},
		}
		for _, tc := range cases {
			t.Run("RL-54_"+tc.name, func(t *testing.T) {
				refused(t, typed(tc.valueType), "true", typeRefusal)
			})
		}
	})
}

func TestFindPermission(t *testing.T) {
	session := func() apiSession { return newSession(testutil.NewRequest()) }

	t.Run("RL-55_a_permission_is_found_by_its_ID_and_returned_whole", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /permissions", 200, catalogueFixture)
		got, err := findPermission(session(), "2")
		want := components.Permission{ID: "2", Node: "pets.max", Description: "Max pets", ValueType: "int", Merge: "max"}
		if err != nil || got != want {
			t.Errorf("permission = %+v, err = %v, want %+v", got, err, want)
		}
		f.AssertLines(t, "GET /permissions")
	})

	t.Run("RL-56_an_ID_that_is_not_in_the_catalogue_gives_a_404_and_an_empty_permission", func(t *testing.T) {
		cases := []struct{ name, id string }{
			{"unknown", "99"},
			{"empty", ""},
			{"node_name", "pets.max"},
			{"padded_number", "02"},
		}
		for _, tc := range cases {
			t.Run("RL-56_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("GET /permissions", 200, catalogueFixture)
				got, err := findPermission(session(), tc.id)
				if got != (components.Permission{}) {
					t.Errorf("permission = %+v, want the zero value", got)
				}
				assertAPIError(t, err, 404, "Permission not found", "", "")
			})
		}
	})

	t.Run("RL-57_an_empty_catalogue_gives_the_same_404", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /permissions", 200, `[]`)
		got, err := findPermission(session(), "1")
		if got != (components.Permission{}) {
			t.Errorf("permission = %+v, want the zero value", got)
		}
		assertAPIError(t, err, 404, "Permission not found", "", "")
	})

	t.Run("RL-58_a_failed_catalogue_load_returns_the_error_and_an_empty_permission", func(t *testing.T) {
		cases := []struct {
			name    string
			status  int
			detail  string
			message string
		}{
			{"403", 403, "Missing permission", "Missing permission"},
			{"500_without_a_detail", 500, "", "Failed to load permissions"},
			{"401", 401, "", ""},
		}
		for _, tc := range cases {
			t.Run("RL-58_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.Refuse("GET /permissions", tc.status, tc.detail)
				got, err := findPermission(session(), "1")
				if got != (components.Permission{}) {
					t.Errorf("permission = %+v, want the zero value", got)
				}
				if tc.status == 401 {
					if err != errUnauthorized {
						t.Errorf("err = %v, want errUnauthorized", err)
					}
				} else {
					assertAPIError(t, err, tc.status, tc.message, "GET", "/permissions")
				}
			})
		}
	})
}

func TestRoleDrafts(t *testing.T) {
	full := func() url.Values {
		return url.Values{
			"value_1": {"a"}, "value_2": {"5"}, "name": {"x"}, "loaded_name": {"y"},
			"grant_permission": {"3"}, "grant_value": {"z"}, "granted": {"1", "2"},
		}
	}

	t.Run("RL-59_only_the_value_fields_become_drafts_keyed_by_permission_ID", func(t *testing.T) {
		if got, want := roleDrafts(full(), ""), (map[string]string{"1": "a", "2": "5"}); !reflect.DeepEqual(got, want) {
			t.Errorf("drafts = %v, want %v", got, want)
		}
	})

	t.Run("RL-60_the_excepted_permissions_draft_is_left_out", func(t *testing.T) {
		cases := []struct {
			name, except string
			want         map[string]string
		}{
			{"present", "2", map[string]string{"1": "a"}},
			{"absent", "9", map[string]string{"1": "a", "2": "5"}},
		}
		for _, tc := range cases {
			t.Run("RL-60_"+tc.name, func(t *testing.T) {
				if got := roleDrafts(full(), tc.except); !reflect.DeepEqual(got, tc.want) {
					t.Errorf("drafts = %v, want %v", got, tc.want)
				}
			})
		}
	})

	t.Run("RL-61_a_draft_that_was_emptied_is_kept_as_an_empty_draft", func(t *testing.T) {
		got := roleDrafts(url.Values{"value_1": {""}, "value_2": {"5"}}, "")
		if text, ok := got["1"]; !ok || text != "" || got["2"] != "5" || len(got) != 2 {
			t.Errorf("drafts = %v, want key 1 empty and key 2 with 5", got)
		}
	})

	t.Run("RL-62_only_the_first_value_of_a_repeated_field_is_used", func(t *testing.T) {
		if got, want := roleDrafts(url.Values{"value_1": {"a", "b"}}, ""), (map[string]string{"1": "a"}); !reflect.DeepEqual(got, want) {
			t.Errorf("drafts = %v, want %v", got, want)
		}
	})

	t.Run("RL-63_a_form_with_no_value_field_gives_an_empty_map_that_is_not_nil", func(t *testing.T) {
		got := roleDrafts(url.Values{"name": {"x"}}, "")
		if got == nil || len(got) != 0 {
			t.Errorf("drafts = %#v, want an empty non-nil map", got)
		}
	})

	t.Run("RL-64_only_the_leading_value_prefix_is_removed_and_the_prefix_match_is_exact", func(t *testing.T) {
		form := url.Values{"value_a_b": {"1"}, "value_value_2": {"2"}, "Value_3": {"3"}, "xvalue_4": {"4"}, "value": {"5"}}
		if got, want := roleDrafts(form, ""), (map[string]string{"a_b": "1", "value_2": "2"}); !reflect.DeepEqual(got, want) {
			t.Errorf("drafts = %v, want %v", got, want)
		}
	})
}

func TestRenderGrantsChanged(t *testing.T) {
	render := func(drafts map[string]string, grantPermission, grantValue string, focusList bool, restore func() []templ.Component) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		r := newRoleRequest("5")
		if err := renderGrantsChanged(rec, r, newSession(r), drafts, grantPermission, grantValue, focusList, restore); err != nil {
			writeError(rec, r, err)
		}
		return rec
	}
	stubReload := func(f *fakeapi.FakeAPI, role string) {
		f.On("GET /roles/5", 200, role)
		f.On("GET /permissions", 200, catalogueFixture)
	}
	everyGrant := roleFixture("5", "admins", "Site admins", grantFixture("1", "null"), grantFixture("2", "7"), grantFixture("3", `["a"]`), grantFixture("4", `"hi"`))

	t.Run("RL-65_after_a_successful_reload_the_granted_list_the_grant_form_and_a_cleared_status_are_written", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		stubReload(f, roleR5)
		calls := 0
		rec := render(map[string]string{}, "", "", false, countingRestore(&calls))
		assertStatusCode(t, rec, 200)
		assertHTMLContent(t, rec)
		assertBodyHas(t, rec, `id="granted-1"`, `id="granted-2"`, `id="admin-role-grant-form"`, `id="admin-role-status"`)
		assertOutOfBand(t, rec, "admin-role-grant", true)
		assertOutOfBand(t, rec, "admin-role-granted", false)
		f.AssertLines(t, "GET /roles/5", "GET /permissions")
		if calls != 0 {
			t.Errorf("restore was called %d times, want 0", calls)
		}
	})

	t.Run("RL-66_a_typed_value_in_a_granted_rows_field_is_shown_instead_of_the_saved_value", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		stubReload(f, roleR5)
		rec := render(map[string]string{"2": "99"}, "", "", false, nil)
		assertBodyHas(t, rec, `value="99"`)
		assertBodyLacks(t, rec, `value="7"`)
	})

	t.Run("RL-67_the_grant_form_keeps_the_typed_permission_and_value_unless_that_permission_is_not_on_offer", func(t *testing.T) {
		cases := []struct {
			name, permission string
			has, lacks       []string
		}{
			{"on_offer", "4", []string{`value="4" selected>pets.motto</option>`, `value="keep me"`}, nil},
			{"already_granted", "1", []string{`value="3" selected>pets.tags</option>`}, []string{"keep me"}},
		}
		for _, tc := range cases {
			t.Run("RL-67_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				stubReload(f, roleR5)
				rec := render(map[string]string{}, tc.permission, "keep me", false, nil)
				assertBodyHas(t, rec, tc.has...)
				assertBodyLacks(t, rec, tc.lacks...)
			})
		}
	})

	t.Run("RL-68_focus_moves_to_the_granted_list_when_asked_and_always_when_no_permission_is_left_to_grant", func(t *testing.T) {
		cases := []struct {
			name      string
			role      string
			focusList bool
			focused   bool
		}{
			{"not_asked", roleR5, false, false},
			{"asked", roleR5, true, true},
			{"nothing_left_to_grant", everyGrant, false, true},
		}
		for _, tc := range cases {
			t.Run("RL-68_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				stubReload(f, tc.role)
				rec := render(map[string]string{}, "", "", tc.focusList, nil)
				if tc.focused {
					assertBodyHas(t, rec, "autofocus")
				} else {
					assertBodyLacks(t, rec, "autofocus")
				}
			})
		}
	})

	t.Run("RL-69_a_failed_reload_reports_that_the_change_was_made_and_draws_what_the_restore_function_returns", func(t *testing.T) {
		cases := []struct {
			name    string
			arrange func(f *fakeapi.FakeAPI)
			message string
		}{
			{"role_with_a_detail", func(f *fakeapi.FakeAPI) { f.Problem("GET /roles/5", 500, "database down") }, "database down"},
			{"role_without_a_detail", func(f *fakeapi.FakeAPI) { f.Refuse("GET /roles/5", 500, "") }, "Failed to load the role"},
			{"catalogue", func(f *fakeapi.FakeAPI) {
				f.On("GET /roles/5", 200, roleR5)
				f.Refuse("GET /permissions", 500, "")
			}, "Failed to load permissions"},
		}
		for _, tc := range cases {
			t.Run("RL-69_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				tc.arrange(f)
				calls := 0
				rec := render(map[string]string{}, "", "", false, countingRestore(&calls))
				assertFailure(t, rec, 500, savedPrefix+tc.message)
				assertBodyHas(t, rec, `id="admin-role-status"`, restoreMarker)
				if body := rec.Body.String(); strings.Index(body, `id="admin-role-status"`) > strings.Index(body, restoreMarker) {
					t.Errorf("the marker comes before the status line:\n%s", body)
				}
				if calls != 1 {
					t.Errorf("restore was called %d times, want 1", calls)
				}
			})
		}
	})

	t.Run("RL-70_a_failed_reload_with_no_restore_function_reports_only_the_message_and_the_status_line", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.Problem("GET /roles/5", 500, "database down")
		rec := render(map[string]string{}, "", "", false, nil)
		assertFailure(t, rec, 500, savedPrefix+"database down")
		assertBodyHas(t, rec, `id="admin-role-status"`)
		assertBodyLacks(t, rec, "granted", "admin-role-grant", restoreMarker)
	})

	t.Run("RL-71_a_401_on_the_reload_sends_the_browser_to_the_login_page_and_draws_nothing", func(t *testing.T) {
		for _, failing := range []string{"role", "catalogue"} {
			t.Run("RL-71_"+failing, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				if failing == "role" {
					f.Refuse("GET /roles/5", 401, "")
				} else {
					f.On("GET /roles/5", 200, roleR5)
					f.Refuse("GET /permissions", 401, "")
				}
				calls := 0
				assertLoginRedirect(t, render(map[string]string{}, "", "", false, countingRestore(&calls)))
			})
		}
	})
}

func TestPutGrant(t *testing.T) {
	stubGrant := func(f *fakeapi.FakeAPI, catalogue string) {
		f.On("GET /permissions", 200, catalogue)
		f.On("GET /roles/5", 200, roleR5)
	}
	grant := func(grantPermission, grantValue string) *httptest.ResponseRecorder {
		return serveRequest("POST", "/admin/roles/5/permissions", url.Values{"grant_permission": {grantPermission}, "grant_value": {grantValue}})
	}

	t.Run("RL-72_a_permission_with_no_value_type_is_put_with_no_body_and_the_handler_then_reloads", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		stubGrant(f, catalogueFixture)
		f.On("PUT /roles/5/permissions/1", 204, ``)
		rec := grant("1", "")
		assertStatusCode(t, rec, 200)
		f.AssertLines(t, "GET /permissions", "PUT /roles/5/permissions/1", "GET /roles/5", "GET /permissions")
		fakeapi.AssertContentType(t, f.Calls()[1], "")
	})

	t.Run("RL-73_the_value_typed_in_the_grant_form_is_sent_as_the_typed_body_of_its_value_type", func(t *testing.T) {
		cases := []struct{ name, permission, typed, want string }{
			{"int", "2", " 12 ", `{"value":12}`},
			{"string_list", "3", "x\r\ny\r\n", `{"value":["x","y"]}`},
			{"string", "4", " hi ", `{"value":"hi"}`},
		}
		for _, tc := range cases {
			t.Run("RL-73_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				stubGrant(f, catalogueFixture)
				f.On("PUT /roles/5/permissions/"+tc.permission, 204, ``)
				grant(tc.permission, tc.typed)
				put := f.Calls()[1]
				if put.Method != "PUT" || put.Body != tc.want {
					t.Errorf("call = %s %s, want PUT with body %s", put.Method, put.Body, tc.want)
				}
				fakeapi.AssertContentType(t, put, "application/json")
			})
		}
	})

	t.Run("RL-74_both_the_role_ID_and_the_permission_ID_are_path_escaped_in_the_PUT_path", func(t *testing.T) {
		catalogue := "[" + permissionFixture("c d", "pets.a", "") + "," + permissionFixture("p/q", "pets.b", "") + "," + permissionFixture("t?u", "pets.c", "") + "]"
		cases := []struct{ name, role, roleEscaped, permission, permissionEscaped string }{
			{"slash_role_space_permission", "a/b", "a%2Fb", "c d", "c%20d"},
			{"question_role_slash_permission", "x?y", "x%3Fy", "p/q", "p%2Fq"},
			{"hash_role_question_permission", "r#s", "r%23s", "t?u", "t%3Fu"},
		}
		for _, tc := range cases {
			t.Run("RL-74_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("GET /permissions", 200, catalogue)
				f.On("GET /roles/"+tc.roleEscaped, 200, roleFixture(tc.role, "admins", ""))
				put := "PUT /roles/" + tc.roleEscaped + "/permissions/" + tc.permissionEscaped
				f.On(put, 204, ``)
				serveRequest("POST", "/admin/roles/"+tc.roleEscaped+"/permissions", url.Values{"grant_permission": {tc.permission}})
				if got := f.Lines()[1]; got != put {
					t.Errorf("call = %q, want %q", got, put)
				}
			})
		}
	})

	t.Run("RL-75_a_permission_that_is_not_in_the_catalogue_is_refused_before_the_PUT", func(t *testing.T) {
		for name, id := range map[string]string{"unknown": "99", "empty": ""} {
			t.Run("RL-75_"+name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("GET /permissions", 200, catalogueFixture)
				rec := grant(id, "")
				assertFailure(t, rec, 404, "Permission not found")
				assertBodyHas(t, rec, `id="admin-role-status"`)
				f.AssertLines(t, "GET /permissions")
			})
		}
	})

	t.Run("RL-76_a_value_the_form_cannot_turn_into_a_body_is_refused_before_the_PUT", func(t *testing.T) {
		catalogue := strings.TrimSuffix(catalogueFixture, "]") + "," + permissionFixture("5", "pets.flag", "bool") + "]"
		cases := []struct{ name, permission, typed, message string }{
			{"int", "2", "abc", intRefusal},
			{"string_list", "3", " ", "Enter at least one item"},
			{"string", "4", "", "Enter a value"},
			{"bool", "5", "true", typeRefusal},
		}
		for _, tc := range cases {
			t.Run("RL-76_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("GET /permissions", 200, catalogue)
				assertFailure(t, grant(tc.permission, tc.typed), 400, tc.message)
				f.AssertLines(t, "GET /permissions")
			})
		}
	})

	t.Run("RL-77_a_failed_catalogue_load_stops_the_grant_before_any_write", func(t *testing.T) {
		cases := []struct {
			name    string
			status  int
			detail  string
			message string
		}{
			{"403", 403, "Missing permission", "Missing permission"},
			{"500_without_a_detail", 500, "", "Failed to load permissions"},
		}
		for _, tc := range cases {
			t.Run("RL-77_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.Refuse("GET /permissions", tc.status, tc.detail)
				assertFailure(t, grant("1", ""), tc.status, tc.message)
				f.AssertLines(t, "GET /permissions")
			})
		}
	})

	t.Run("RL-78_a_refused_PUT_shows_its_message_and_does_not_reload_the_role", func(t *testing.T) {
		cases := []struct {
			name    string
			status  int
			detail  string
			message string
		}{
			{"400", 400, "Value out of range", "Value out of range"},
			{"404", 404, "Role not found", "Role not found"},
			{"409", 409, "Already granted", "Already granted"},
			{"500_without_a_detail", 500, "", "Failed to grant the permission"},
		}
		for _, tc := range cases {
			t.Run("RL-78_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("GET /permissions", 200, catalogueFixture)
				f.Refuse("PUT /roles/5/permissions/1", tc.status, tc.detail)
				rec := grant("1", "")
				assertFailure(t, rec, tc.status, tc.message)
				assertBodyLacks(t, rec, "granted-")
				f.AssertLines(t, "GET /permissions", "PUT /roles/5/permissions/1")
			})
		}
	})

	t.Run("RL-79_a_401_on_the_catalogue_or_on_the_PUT_sends_the_browser_to_the_login_page", func(t *testing.T) {
		cases := []struct {
			name  string
			setup func(f *fakeapi.FakeAPI)
			lines []string
		}{
			{"catalogue", func(f *fakeapi.FakeAPI) { f.Refuse("GET /permissions", 401, "") }, []string{"GET /permissions"}},
			{"put", func(f *fakeapi.FakeAPI) {
				f.On("GET /permissions", 200, catalogueFixture)
				f.Refuse("PUT /roles/5/permissions/1", 401, "")
			}, []string{"GET /permissions", "PUT /roles/5/permissions/1"}},
		}
		for _, tc := range cases {
			t.Run("RL-79_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				tc.setup(f)
				assertLoginRedirect(t, grant("1", ""))
				f.AssertLines(t, tc.lines...)
			})
		}
	})
}

func TestAdminRoleGrantHandler(t *testing.T) {
	grantForm := func(permission, typed string, granted []string, drafts url.Values) url.Values {
		form := url.Values{"grant_permission": {permission}, "grant_value": {typed}, "granted": granted}
		for key, values := range drafts {
			form[key] = values
		}
		return form
	}
	grant := func(form url.Values) *httptest.ResponseRecorder {
		return serveRequest("POST", "/admin/roles/5/permissions", form)
	}
	withTags := roleFixture("5", "admins", "Site admins", grantFixture("1", "null"), grantFixture("2", "7"), grantFixture("3", `["a","b"]`))

	t.Run("RL-80_a_grant_puts_the_permission_reloads_the_role_and_the_catalogue_and_redraws_the_list_and_a_fresh_form", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /permissions", 200, catalogueFixture)
		f.On("PUT /roles/5/permissions/3", 204, ``)
		f.On("GET /roles/5", 200, withTags)
		rec := grant(grantForm("3", "a\nb", []string{"1", "2"}, url.Values{"value_2": {"7"}}))
		f.AssertLines(t, "GET /permissions", `PUT /roles/5/permissions/3 {"value":["a","b"]}`, "GET /roles/5", "GET /permissions")
		assertStatusCode(t, rec, 200)
		assertHTMLContent(t, rec)
		assertBodyHas(t, rec, `id="granted-3"`, "pets.tags", `id="admin-role-grant-form"`, ">pets.motto</option>", `id="admin-role-status"`)
		assertOutOfBand(t, rec, "admin-role-grant", true)
		assertOutOfBand(t, rec, "admin-role-granted", false)
		assertBodyLacks(t, rec, ">pets.tags</option>", "autofocus")
	})

	t.Run("RL-81_granting_the_last_available_permission_moves_focus_to_the_granted_list", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /permissions", 200, catalogueFixture)
		f.On("PUT /roles/5/permissions/4", 204, ``)
		f.On("GET /roles/5", 200, roleFixture("5", "admins", "Site admins", grantFixture("1", "null"), grantFixture("2", "7"), grantFixture("3", `["a"]`), grantFixture("4", `"hi"`)))
		rec := grant(grantForm("4", "hi", []string{"1", "2", "3"}, nil))
		assertStatusCode(t, rec, 200)
		assertBodyHas(t, rec, `id="admin-role-grant-empty"`, "autofocus")
		assertBodyLacks(t, rec, `id="admin-role-grant-form"`)
	})

	reloaded := roleFixture("5", "admins", "Site admins", grantFixture("2", "7"), grantFixture("3", `["a"]`))
	for _, tc := range []struct {
		row, name, typed string
		want, lacks      string
	}{
		{"RL-82", "a_value_typed_into_another_granted_row_survives_a_grant", "99", `value="99"`, `value="7"`},
		{"RL-83", "a_value_field_that_was_emptied_stays_empty_after_a_grant", "", `value=""`, `value="7"`},
	} {
		t.Run(tc.row+"_"+tc.name, func(t *testing.T) {
			f := fakeapi.NewFakeAPI(t)
			f.On("GET /permissions", 200, catalogueFixture)
			f.On("PUT /roles/5/permissions/3", 204, ``)
			f.On("GET /roles/5", 200, reloaded)
			rec := grant(grantForm("3", "a", []string{"2"}, url.Values{"value_2": {tc.typed}}))
			assertStatusCode(t, rec, 200)
			assertBodyHas(t, rec, tc.want)
			assertBodyLacks(t, rec, tc.lacks)
		})
	}

	t.Run("RL-84_when_the_reload_fails_after_a_successful_grant_the_message_says_so_and_the_list_and_form_are_redrawn_from_the_form", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /permissions", 200, catalogueFixture)
		f.On("PUT /roles/5/permissions/3", 204, ``)
		f.Problem("GET /roles/5", 500, "database down")
		rec := grant(grantForm("3", "a\nb", []string{"1", "2"}, url.Values{"value_2": {"7"}}))
		assertFailure(t, rec, 500, savedPrefix+"database down")
		assertBodyHas(t, rec, `id="admin-role-status"`, `id="granted-1"`, `id="granted-2"`, `id="granted-3"`, `id="admin-role-grant-form"`, ">pets.motto</option>")
		assertBodyLacks(t, rec, ">pets.tags</option>")
		body := rec.Body.String()
		if !(strings.Index(body, `id="granted-1"`) < strings.Index(body, `id="granted-2"`) && strings.Index(body, `id="granted-2"`) < strings.Index(body, `id="granted-3"`)) {
			t.Errorf("the rows are not in the order 1, 2, 3:\n%s", body)
		}
		f.AssertLines(t, "GET /permissions", `PUT /roles/5/permissions/3 {"value":["a","b"]}`, "GET /roles/5", "GET /permissions")
	})
}

func TestGrantedWith(t *testing.T) {
	failedReload := func(t *testing.T) *fakeapi.FakeAPI {
		t.Helper()
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /permissions", 200, catalogueFixture)
		f.On("PUT /roles/5/permissions/3", 204, ``)
		f.Problem("GET /roles/5", 500, "database down")
		return f
	}
	grant := func(granted []string, drafts url.Values) *httptest.ResponseRecorder {
		form := url.Values{"grant_permission": {"3"}, "grant_value": {"a\nb"}, "granted": granted}
		for key, values := range drafts {
			form[key] = values
		}
		return serveRequest("POST", "/admin/roles/5/permissions", form)
	}

	t.Run("RL-85_the_restored_list_keeps_the_typed_drafts_and_shows_the_typed_grant_value_on_the_new_row", func(t *testing.T) {
		failedReload(t)
		rec := grant([]string{"2"}, url.Values{"value_2": {"99"}})
		assertBodyHas(t, rec, `value="99"`, ">a\nb</textarea>")
		assertBodyLacks(t, rec, `value="7"`)
	})

	t.Run("RL-86_a_granted_ID_that_is_not_in_the_catalogue_is_left_out_of_the_restored_list", func(t *testing.T) {
		failedReload(t)
		rec := grant([]string{"1", "77", "2"}, nil)
		assertBodyHas(t, rec, `id="granted-1"`, `id="granted-2"`, `id="granted-3"`)
		assertBodyLacks(t, rec, "granted-77")
	})

	t.Run("RL-87_when_the_catalogue_cannot_be_loaded_either_the_grant_form_is_replaced_by_the_unavailable_notice", func(t *testing.T) {
		cases := []struct {
			name, role, message string
		}{
			{"role_fails", "", "database down"},
			{"catalogue_fails_on_the_reload", roleR5, "Failed to load permissions"},
		}
		for _, tc := range cases {
			t.Run("RL-87_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				var lookups atomic.Int32
				f.Handle("GET /permissions", func(w http.ResponseWriter, r *http.Request) {
					if lookups.Add(1) == 1 {
						w.Header().Set("Content-Type", "application/json")
						fmt.Fprint(w, catalogueFixture)
						return
					}
					w.Header().Set("Content-Type", "application/problem+json")
					w.WriteHeader(500)
				})
				f.On("PUT /roles/5/permissions/3", 204, ``)
				if tc.role == "" {
					f.Problem("GET /roles/5", 500, "database down")
				} else {
					f.On("GET /roles/5", 200, tc.role)
				}
				rec := grant([]string{"1", "2"}, url.Values{"value_2": {"7"}})
				assertFailure(t, rec, 500, savedPrefix+tc.message)
				assertBodyHas(t, rec, `id="admin-role-grant-unavailable"`)
				assertBodyLacks(t, rec, "granted-")
			})
		}
	})
}

func TestAdminRoleValueHandler(t *testing.T) {
	saveValue := func(permission string, form url.Values) *httptest.ResponseRecorder {
		return serveRequest("POST", "/admin/roles/5/permissions/"+permission, form)
	}

	t.Run("RL-88_a_value_save_puts_the_value_reloads_the_role_and_redraws_the_granted_list_with_focus_on_it", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /permissions", 200, catalogueFixture)
		f.On("PUT /roles/5/permissions/2", 204, ``)
		f.On("GET /roles/5", 200, roleFixture("5", "admins", "Site admins", grantFixture("1", "null"), grantFixture("2", "12")))
		rec := saveValue("2", url.Values{"value_2": {"12"}})
		f.AssertLines(t, "GET /permissions", `PUT /roles/5/permissions/2 {"value":12}`, "GET /roles/5")
		fakeapi.AssertContentType(t, f.Calls()[1], "application/json")
		assertStatusCode(t, rec, 200)
		assertBodyHas(t, rec, `id="granted-2"`, `id="admin-role-status"`, "autofocus")
		assertOutOfBand(t, rec, "admin-role-granted", false)
		assertBodyLacks(t, rec, `id="admin-role-grant-form"`)
	})

	t.Run("RL-89_the_value_comes_from_the_field_of_the_permission_in_the_path_and_the_other_fields_stay_as_drafts", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /permissions", 200, catalogueFixture)
		f.On("PUT /roles/5/permissions/2", 204, ``)
		f.On("GET /roles/5", 200, roleFixture("5", "admins", "Site admins", grantFixture("2", "12"), grantFixture("3", `["old"]`)))
		rec := saveValue("2", url.Values{"value_2": {" 12 "}, "value_3": {"typed"}, "grant_value": {"zzz"}})
		f.AssertLines(t, "GET /permissions", `PUT /roles/5/permissions/2 {"value":12}`, "GET /roles/5")
		assertBodyHas(t, rec, `value="12"`, ">typed</textarea>")
		assertBodyLacks(t, rec, "zzz")
	})

	t.Run("RL-90_a_value_the_form_cannot_turn_into_a_body_is_refused_before_the_PUT", func(t *testing.T) {
		for name, typed := range map[string]string{"letters": "abc", "empty": "", "decimal": "1.5"} {
			t.Run("RL-90_"+name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("GET /permissions", 200, catalogueFixture)
				assertFailure(t, saveValue("2", url.Values{"value_2": {typed}}), 400, intRefusal)
				f.AssertLines(t, "GET /permissions")
			})
		}
	})

	t.Run("RL-91_a_permission_that_is_not_in_the_catalogue_is_refused_before_the_PUT", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /permissions", 200, catalogueFixture)
		assertFailure(t, saveValue("99", url.Values{"value_99": {"1"}}), 404, "Permission not found")
		f.AssertLines(t, "GET /permissions")
	})

	t.Run("RL-92_a_refused_PUT_shows_its_message_and_does_not_reload_the_role", func(t *testing.T) {
		cases := []struct {
			name    string
			status  int
			detail  string
			message string
		}{
			{"400", 400, "Value out of range", "Value out of range"},
			{"409", 409, "Value conflicts", "Value conflicts"},
			{"500_without_a_detail", 500, "", "Failed to grant the permission"},
		}
		for _, tc := range cases {
			t.Run("RL-92_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("GET /permissions", 200, catalogueFixture)
				f.Refuse("PUT /roles/5/permissions/2", tc.status, tc.detail)
				rec := saveValue("2", url.Values{"value_2": {"12"}})
				assertFailure(t, rec, tc.status, tc.message)
				assertBodyLacks(t, rec, "granted-")
				f.AssertLines(t, "GET /permissions", `PUT /roles/5/permissions/2 {"value":12}`)
			})
		}
	})

	t.Run("RL-93_the_IDs_are_path_escaped_and_the_field_name_uses_the_unescaped_permission_ID", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /permissions", 200, "["+permissionFixture("c d", "pets.max", "int")+"]")
		f.On("PUT /roles/a%2Fb/permissions/c%20d", 204, ``)
		f.On("GET /roles/a%2Fb", 200, roleFixture("a/b", "admins", ""))
		serveRequest("POST", "/admin/roles/a%2Fb/permissions/c%20d", url.Values{"value_c d": {"5"}})
		f.AssertLines(t, "GET /permissions", `PUT /roles/a%2Fb/permissions/c%20d {"value":5}`, "GET /roles/a%2Fb")
	})

	t.Run("RL-94_when_the_role_reload_fails_after_a_successful_save_the_message_says_so_and_nothing_else_is_redrawn", func(t *testing.T) {
		cases := []struct {
			name    string
			detail  string
			message string
		}{
			{"with_a_detail", "database down", "database down"},
			{"without_a_detail", "", "Failed to load the role"},
		}
		for _, tc := range cases {
			t.Run("RL-94_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("GET /permissions", 200, catalogueFixture)
				f.On("PUT /roles/5/permissions/2", 204, ``)
				f.Refuse("GET /roles/5", 500, tc.detail)
				rec := saveValue("2", url.Values{"value_2": {"12"}})
				assertFailure(t, rec, 500, savedPrefix+tc.message)
				assertBodyHas(t, rec, `id="admin-role-status"`)
				assertBodyLacks(t, rec, "granted-")
			})
		}
	})

	t.Run("RL-95_a_401_on_any_of_the_three_calls_sends_the_browser_to_the_login_page", func(t *testing.T) {
		for _, failing := range []string{"catalogue", "put", "reload"} {
			t.Run("RL-95_"+failing, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				switch failing {
				case "catalogue":
					f.Refuse("GET /permissions", 401, "")
				case "put":
					f.On("GET /permissions", 200, catalogueFixture)
					f.Refuse("PUT /roles/5/permissions/2", 401, "")
				case "reload":
					f.On("GET /permissions", 200, catalogueFixture)
					f.On("PUT /roles/5/permissions/2", 204, ``)
					f.Refuse("GET /roles/5", 401, "")
				}
				assertLoginRedirect(t, saveValue("2", url.Values{"value_2": {"12"}}))
			})
		}
	})
}

func TestAdminRoleRemoveHandler(t *testing.T) {
	remove := func(permission string, query url.Values) *httptest.ResponseRecorder {
		return serveRequest("DELETE", "/admin/roles/5/permissions/"+permission+"?"+query.Encode(), nil)
	}

	t.Run("RL-96_a_removal_sends_an_empty_DELETE_reloads_the_role_and_the_catalogue_and_redraws_the_list_and_the_form_with_focus_on_the_list", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("DELETE /roles/5/permissions/2", 204, ``)
		f.On("GET /roles/5", 200, roleFixture("5", "admins", "Site admins", grantFixture("1", "null")))
		f.On("GET /permissions", 200, catalogueFixture)
		rec := remove("2", url.Values{"granted": {"1", "2"}})
		f.AssertLines(t, "DELETE /roles/5/permissions/2", "GET /roles/5", "GET /permissions")
		fakeapi.AssertContentType(t, f.Calls()[0], "")
		assertStatusCode(t, rec, 200)
		assertBodyHas(t, rec, `id="granted-1"`, `id="admin-role-grant-form"`, ">pets.max</option>", `id="admin-role-status"`, "autofocus")
		assertBodyLacks(t, rec, "granted-2")
	})

	t.Run("RL-97_values_typed_into_the_other_granted_rows_survive_a_removal_and_the_removed_rows_value_is_dropped", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("DELETE /roles/5/permissions/3", 204, ``)
		f.On("GET /roles/5", 200, roleFixture("5", "admins", "Site admins", grantFixture("2", "7")))
		f.On("GET /permissions", 200, catalogueFixture)
		rec := remove("3", url.Values{"granted": {"2", "3"}, "value_2": {"99"}, "value_3": {"zzz"}})
		assertBodyHas(t, rec, `value="99"`)
		assertBodyLacks(t, rec, `value="7"`, "zzz")
	})

	t.Run("RL-98_the_permission_and_value_typed_in_the_grant_form_survive_a_removal", func(t *testing.T) {
		cases := []struct {
			name, permission, typed string
			has                     []string
		}{
			{"another_permission", "4", "keep me", []string{`value="4" selected>pets.motto</option>`, `value="keep me"`}},
			{"the_removed_permission", "2", "5", []string{`value="2" selected>pets.max</option>`, `value="5"`}},
		}
		for _, tc := range cases {
			t.Run("RL-98_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("DELETE /roles/5/permissions/2", 204, ``)
				f.On("GET /roles/5", 200, roleFixture("5", "admins", "Site admins", grantFixture("1", "null")))
				f.On("GET /permissions", 200, catalogueFixture)
				rec := remove("2", url.Values{"grant_permission": {tc.permission}, "grant_value": {tc.typed}})
				assertBodyHas(t, rec, tc.has...)
			})
		}
	})

	t.Run("RL-99_both_IDs_are_path_escaped_when_sent_to_the_API", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("DELETE /roles/a%2Fb/permissions/c%20d", 204, ``)
		f.On("GET /roles/a%2Fb", 200, roleFixture("a/b", "admins", ""))
		f.On("GET /permissions", 200, catalogueFixture)
		serveRequest("DELETE", "/admin/roles/a%2Fb/permissions/c%20d", nil)
		f.AssertLines(t, "DELETE /roles/a%2Fb/permissions/c%20d", "GET /roles/a%2Fb", "GET /permissions")
	})

	t.Run("RL-100_a_refused_DELETE_shows_its_message_and_does_not_reload", func(t *testing.T) {
		cases := []struct {
			name    string
			status  int
			detail  string
			message string
		}{
			{"404", 404, "Permission not found", "Permission not found"},
			{"409", 409, "Permission is required", "Permission is required"},
			{"500_without_a_detail", 500, "", "Failed to remove the permission"},
		}
		for _, tc := range cases {
			t.Run("RL-100_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.Refuse("DELETE /roles/5/permissions/2", tc.status, tc.detail)
				rec := remove("2", url.Values{"granted": {"1", "2"}})
				assertFailure(t, rec, tc.status, tc.message)
				assertBodyHas(t, rec, `id="admin-role-status"`)
				assertBodyLacks(t, rec, "granted-")
				f.AssertLines(t, "DELETE /roles/5/permissions/2")
			})
		}
	})

	t.Run("RL-101_a_401_on_the_DELETE_or_on_the_reload_sends_the_browser_to_the_login_page", func(t *testing.T) {
		for _, failing := range []string{"delete", "reload"} {
			t.Run("RL-101_"+failing, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				if failing == "delete" {
					f.Refuse("DELETE /roles/5/permissions/2", 401, "")
				} else {
					f.On("DELETE /roles/5/permissions/2", 204, ``)
					f.Refuse("GET /roles/5", 401, "")
				}
				assertLoginRedirect(t, remove("2", url.Values{"granted": {"1", "2"}}))
			})
		}
	})

	t.Run("RL-102_when_the_reload_fails_after_a_successful_removal_the_row_is_deleted_from_the_page_and_the_form_is_rebuilt", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("DELETE /roles/5/permissions/2", 204, ``)
		f.Problem("GET /roles/5", 500, "database down")
		f.On("GET /permissions", 200, catalogueFixture)
		rec := remove("2", url.Values{"granted": {"1", "2"}})
		assertFailure(t, rec, 500, savedPrefix+"database down")
		assertBodyHas(t, rec, `id="admin-role-status"`, "delete:#granted-2", `id="admin-role-grant-form"`, ">pets.max</option>")
		assertBodyLacks(t, rec, "admin-role-granted")
		f.AssertLines(t, "DELETE /roles/5/permissions/2", "GET /roles/5", "GET /permissions")
	})
}

func TestGrantFormWithout(t *testing.T) {
	failedReload := func(t *testing.T) {
		t.Helper()
		f := fakeapi.NewFakeAPI(t)
		f.On("DELETE /roles/5/permissions/2", 204, ``)
		f.Problem("GET /roles/5", 500, "database down")
		f.On("GET /permissions", 200, catalogueFixture)
	}
	remove := func(query url.Values) *httptest.ResponseRecorder {
		return serveRequest("DELETE", "/admin/roles/5/permissions/2?"+query.Encode(), nil)
	}

	t.Run("RL-103_the_rebuilt_form_offers_every_permission_the_pages_other_grants_leave_free_including_the_removed_one", func(t *testing.T) {
		cases := []struct {
			name       string
			granted    []string
			has, lacks []string
		}{
			{"three_granted", []string{"1", "2", "3"}, []string{">pets.max</option>", ">pets.motto</option>"}, []string{">pets.read</option>", ">pets.tags</option>"}},
			{"only_the_removed_one", []string{"2"}, []string{">pets.read</option>", ">pets.max</option>", ">pets.tags</option>", ">pets.motto</option>"}, nil},
		}
		for _, tc := range cases {
			t.Run("RL-103_"+tc.name, func(t *testing.T) {
				failedReload(t)
				rec := remove(url.Values{"granted": tc.granted})
				assertBodyHas(t, rec, tc.has...)
				assertOutOfBand(t, rec, "admin-role-grant", true)
				assertBodyLacks(t, rec, tc.lacks...)
			})
		}
	})

	t.Run("RL-104_the_rebuilt_form_keeps_the_permission_and_value_typed_before_the_removal_unless_that_permission_is_not_on_offer", func(t *testing.T) {
		cases := []struct {
			name, permission, typed string
			has, lacks              []string
		}{
			{"another_permission", "4", "keep me", []string{`value="4" selected>pets.motto</option>`, `value="keep me"`}, nil},
			{"the_removed_permission", "2", "5", []string{`value="2" selected>pets.max</option>`, `value="5"`}, nil},
			{"a_permission_still_granted", "1", "keep me", []string{`value="2" selected>pets.max</option>`}, []string{"keep me"}},
		}
		for _, tc := range cases {
			t.Run("RL-104_"+tc.name, func(t *testing.T) {
				failedReload(t)
				rec := remove(url.Values{"granted": {"1", "2"}, "grant_permission": {tc.permission}, "grant_value": {tc.typed}})
				assertBodyHas(t, rec, tc.has...)
				assertBodyLacks(t, rec, tc.lacks...)
			})
		}
	})

	t.Run("RL-105_when_the_catalogue_cannot_be_loaded_the_form_is_replaced_by_the_unavailable_notice_and_the_row_is_still_deleted", func(t *testing.T) {
		cases := []struct {
			name, message string
			arrange       func(f *fakeapi.FakeAPI)
		}{
			{"role_fails", "database down", func(f *fakeapi.FakeAPI) { f.Problem("GET /roles/5", 500, "database down") }},
			{"catalogue_fails_on_the_reload", "Failed to load permissions", func(f *fakeapi.FakeAPI) {
				f.On("GET /roles/5", 200, roleFixture("5", "admins", "Site admins", grantFixture("1", "null")))
			}},
		}
		for _, tc := range cases {
			t.Run("RL-105_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("DELETE /roles/5/permissions/2", 204, ``)
				f.Refuse("GET /permissions", 500, "")
				tc.arrange(f)
				rec := remove(url.Values{"granted": {"1", "2"}})
				assertFailure(t, rec, 500, savedPrefix+tc.message)
				assertBodyHas(t, rec, `id="admin-role-grant-unavailable"`, "delete:#granted-2")
				assertBodyLacks(t, rec, `id="admin-role-grant-form"`)
			})
		}
	})
}

func TestAdminRoleGrantValueHandler(t *testing.T) {
	pick := func(query string) *httptest.ResponseRecorder {
		return serveRequest("GET", "/admin/roles/5/grant-value"+query, nil)
	}

	t.Run("RL-106_the_value_input_for_the_chosen_permission_is_drawn_empty", func(t *testing.T) {
		cases := []struct{ name, permission, label, empty string }{
			{"int", "2", `aria-label="Value of pets.max"`, `value=""`},
			{"string_list", "3", `aria-label="Value of pets.tags"`, `></textarea>`},
			{"string", "4", `aria-label="Value of pets.motto"`, `value=""`},
		}
		for _, tc := range cases {
			t.Run("RL-106_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("GET /permissions", 200, catalogueFixture)
				rec := pick("?grant_permission=" + tc.permission)
				assertStatusCode(t, rec, 200)
				assertHTMLContent(t, rec)
				assertBodyHas(t, rec, `name="grant_value"`, tc.label, tc.empty)
				f.AssertLines(t, "GET /permissions")
			})
		}
	})

	t.Run("RL-107_a_permission_with_no_value_type_draws_no_input", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /permissions", 200, catalogueFixture)
		rec := pick("?grant_permission=1")
		assertStatusCode(t, rec, 200)
		assertBodyLacks(t, rec, "grant_value")
	})

	t.Run("RL-108_an_unknown_empty_or_missing_permission_gives_a_404", func(t *testing.T) {
		cases := []struct{ name, query string }{
			{"unknown", "?grant_permission=99"},
			{"empty", "?grant_permission="},
			{"missing", ""},
		}
		for _, tc := range cases {
			t.Run("RL-108_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("GET /permissions", 200, catalogueFixture)
				rec := pick(tc.query)
				assertFailure(t, rec, 404, "Permission not found")
				assertBodyHas(t, rec, `id="admin-role-status"`)
			})
		}
	})

	t.Run("RL-109_a_failed_catalogue_load_answers_with_its_status_and_message", func(t *testing.T) {
		cases := []struct {
			name    string
			status  int
			detail  string
			message string
		}{
			{"403", 403, "Missing permission", "Missing permission"},
			{"500_without_a_detail", 500, "", "Failed to load permissions"},
		}
		for _, tc := range cases {
			t.Run("RL-109_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.Refuse("GET /permissions", tc.status, tc.detail)
				assertFailure(t, pick("?grant_permission=2"), tc.status, tc.message)
			})
		}
	})

	t.Run("RL-110_a_401_on_the_catalogue_sends_the_browser_to_the_login_page", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.Refuse("GET /permissions", 401, "")
		assertLoginRedirect(t, pick("?grant_permission=2"))
	})
}

func TestAdminRoleSaveHandlerPaddedText(t *testing.T) {
	t.Run("RL-111_a_name_and_description_with_outer_whitespace_that_equal_the_loaded_ones_are_not_a_change", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		rec := serveRequest("POST", "/admin/roles/5", url.Values{
			"loaded_name": {" admins "}, "name": {" admins "},
			"loaded_description": {" padded "}, "description": {" padded "},
		})
		assertStatusCode(t, rec, 200)
		assertBodyHas(t, rec, ">Nothing to save<")
		f.AssertLines(t)
	})
}

func TestAdminRoleFailuresClearTheStatusLine(t *testing.T) {
	const clear = `<p id="admin-role-status" hx-swap-oob="innerHTML"></p>`
	cases := []struct {
		name, method, target string
		form                 url.Values
		answered             string
		clears               bool
	}{
		{"save", "POST", "/admin/roles/5", url.Values{"loaded_description": {"a"}, "description": {"b"}, "loaded_name": {"admins"}, "name": {"admins"}}, "PATCH /roles/5", true},
		{"delete", "DELETE", "/admin/roles/5", nil, "DELETE /roles/5", true},
		{"grant", "POST", "/admin/roles/5/permissions", url.Values{"grant_permission": {"1"}}, "PUT /roles/5/permissions/1", true},
		{"value_save", "POST", "/admin/roles/5/permissions/2", url.Values{"value_2": {"9"}}, "PUT /roles/5/permissions/2", true},
		{"removal", "DELETE", "/admin/roles/5/permissions/2", nil, "DELETE /roles/5/permissions/2", true},
		{"editor", "GET", "/admin/roles/5/editor", nil, "GET /roles/5", false},
		{"list", "GET", "/admin/roles/list", nil, "GET /roles", false},
	}
	for _, tc := range cases {
		t.Run("RL-112_"+tc.name, func(t *testing.T) {
			f := fakeapi.NewFakeAPI(t)
			f.On("GET /permissions", 200, catalogueFixture)
			f.Problem(tc.answered, 409, "refused")
			rec := serveRequest(tc.method, tc.target, tc.form)
			assertStatusCode(t, rec, 409)
			want := "refused"
			if tc.clears {
				want += clear
			}
			if got := rec.Body.String(); got != want {
				t.Errorf("body = %q, want %q", got, want)
			}
		})
	}
}
