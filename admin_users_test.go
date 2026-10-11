package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/p0t4t0sandwich/neuralnexus-frontend/components"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/test/testutil"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/test/testutil/fakeapi"
)

const (
	roleListJSON = `[{"id":"r1","name":"Moderator","description":"Moderates","permissions":[]},{"id":"r2","name":"Admin","description":"Administers","permissions":[]}]`
	linkListJSON = `[{"platform":"discord","platform_username":"bob#1","platform_id":"1","verified":true,"login_enabled":false}]`
	usersCall    = "GET /users?limit=200&offset="
	savedPrefix  = "The change was made, but the page could not be refreshed: "
)

var idEscapes = []struct{ id, escaped string }{
	{"a b", "a%20b"},
	{"a/b", "a%2Fb"},
	{"a?b", "a%3Fb"},
	{"a#b", "a%23b"},
	{"100%", "100%25"},
}

func userJSON(id, username string, roles ...string) string {
	if roles == nil {
		roles = []string{}
	}
	encoded, err := json.Marshal(components.UserAccount{UserID: id, Username: username, Roles: roles})
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func userList(from, n int, username func(i int) string) []string {
	users := make([]string, n)
	for k := range users {
		i := from + k
		users[k] = userJSON("u"+strconv.Itoa(i), username(i))
	}
	return users
}

func serveUserPages(f *fakeapi.FakeAPI, users []string) {
	f.Handle("GET /users", func(w http.ResponseWriter, r *http.Request) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		start := min(offset, len(users))
		end := min(offset+limit, len(users))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, "["+strings.Join(users[start:end], ",")+"]")
	})
}

func stubUserEditor(f *fakeapi.FakeAPI) {
	f.On("GET /users/u1", 200, userJSON("u1", "Bob", "r1"))
	f.On("GET /roles", 200, roleListJSON)
	f.On("GET /users/u1/links", 200, linkListJSON)
	f.On("GET /users/u1/permissions", 200, `["pets.read","pets.max:5"]`)
}

func userSaveForm(overrides url.Values) url.Values {
	form := url.Values{
		"username":        {"Bob"},
		"loaded_username": {"Bob"},
		"roles_editable":  {"1"},
		"roles":           {"r1"},
		"loaded_roles":    {"r1"},
	}
	for key, values := range overrides {
		form[key] = values
	}
	return form
}

func twoUsersJSON() string {
	return "[" + userJSON("u1", "Bob", "r1") + "," + userJSON("u2", "alice") + "]"
}

func usernames() func(i int) string {
	return func(i int) string { return "user" + strconv.Itoa(i) }
}

func userIDs(users []components.UserAccount) []string {
	ids := make([]string, len(users))
	for i, user := range users {
		ids[i] = user.UserID
	}
	return ids
}

func TestListRoles(t *testing.T) {
	t.Run("US-05_a_200_answer_returns_the_roles_in_order_and_marks_them_readable", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /roles", 200, roleListJSON)
		got, readable, err := listRoles(newSession(testutil.NewRequest()))
		if err != nil || !readable || len(got) != 2 {
			t.Fatalf("roles = %+v, readable = %t, err = %v", got, readable, err)
		}
		if got[0].ID != "r1" || got[0].Name != "Moderator" || got[0].Description != "Moderates" || got[1].ID != "r2" || got[1].Name != "Admin" || got[1].Description != "Administers" {
			t.Errorf("roles = %+v", got)
		}
		f.AssertLines(t, "GET /roles")
	})

	t.Run("US-06_an_empty_or_null_list_is_readable_with_no_roles", func(t *testing.T) {
		for _, body := range []string{`[]`, `null`} {
			t.Run("US-06_"+body, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("GET /roles", 200, body)
				got, readable, err := listRoles(newSession(testutil.NewRequest()))
				if err != nil || !readable || len(got) != 0 {
					t.Errorf("roles = %+v, readable = %t, err = %v", got, readable, err)
				}
			})
		}
	})

	t.Run("US-07_a_403_means_the_roles_cannot_be_read_and_is_not_an_error", func(t *testing.T) {
		cases := []struct{ name, detail string }{
			{"with_a_detail", "Missing permission"},
			{"empty_body", ""},
		}
		for _, tc := range cases {
			t.Run("US-07_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.Refuse("GET /roles", 403, tc.detail)
				got, readable, err := listRoles(newSession(testutil.NewRequest()))
				if got != nil || readable || err != nil {
					t.Errorf("roles = %+v, readable = %t, err = %v, want nil, false and no error", got, readable, err)
				}
			})
		}
	})

	t.Run("US-08_any_other_failure_returns_an_error_no_roles_and_readable_false", func(t *testing.T) {
		cases := []struct {
			name         string
			status       int
			detail, body string
			message      string
		}{
			{"500_with_a_detail", 500, "database down", "", "database down"},
			{"500_without_a_detail", 500, "", "", "Failed to load roles"},
			{"404_with_a_detail", 404, "Not found", "", "Not found"},
			{"401", 401, "", "", ""},
			{"200_not_json", 200, "", "not json", "Failed to load roles"},
		}
		for _, tc := range cases {
			t.Run("US-08_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				if tc.body != "" {
					f.On("GET /roles", tc.status, tc.body)
				} else {
					f.Refuse("GET /roles", tc.status, tc.detail)
				}
				got, readable, err := listRoles(newSession(testutil.NewRequest()))
				if got != nil || readable {
					t.Errorf("roles = %+v, readable = %t, want nil and false", got, readable)
				}
				switch tc.status {
				case 401:
					if err != errUnauthorized {
						t.Errorf("err = %v, want errUnauthorized", err)
					}
				case 200:
					assertAPIError(t, err, 502, tc.message, "GET", "/roles")
				default:
					assertAPIError(t, err, tc.status, tc.message, "GET", "/roles")
				}
			})
		}
	})
}

func TestLoadUsersPage(t *testing.T) {
	session := func() apiSession { return newSession(testutil.NewRequest()) }

	t.Run("US-09_without_a_search_it_reads_one_page_at_the_offset_and_then_the_roles", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /users", 200, twoUsersJSON())
		f.On("GET /roles", 200, roleListJSON)
		data, err := loadUsersPage(session(), 0, "")
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		want := []components.UserAccount{{UserID: "u1", Username: "Bob", Roles: []string{"r1"}}, {UserID: "u2", Username: "alice", Roles: []string{}}}
		if !reflect.DeepEqual(data.Users, want) {
			t.Errorf("Users = %+v, want %+v", data.Users, want)
		}
		if data.Search != "" || data.Start != 0 || data.NextOffset != 2 || data.More {
			t.Errorf("data = %+v, want no search, Start 0, NextOffset 2 and no More", data)
		}
		f.AssertLines(t, usersCall+"0", "GET /roles")
	})

	t.Run("US-10_the_offset_goes_to_the_API_and_into_Start_and_NextOffset", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /users", 200, "["+strings.Join(userList(0, 3, usernames()), ",")+"]")
		f.On("GET /roles", 200, roleListJSON)
		data, err := loadUsersPage(session(), 400, "")
		if err != nil || data.Start != 400 || data.NextOffset != 403 || data.More {
			t.Errorf("data = %+v, err = %v, want Start 400, NextOffset 403 and no More", data, err)
		}
		f.AssertLines(t, usersCall+"400", "GET /roles")
	})

	t.Run("US-11_a_full_page_without_a_search_reports_more_and_reads_no_further_page", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		serveUserPages(f, userList(0, 1000, usernames()))
		f.On("GET /roles", 200, roleListJSON)
		data, err := loadUsersPage(session(), 0, "")
		if err != nil || len(data.Users) != 200 || data.NextOffset != 200 || !data.More {
			t.Errorf("%d users, NextOffset %d, More %t, err = %v", len(data.Users), data.NextOffset, data.More, err)
		}
		f.AssertLines(t, usersCall+"0", "GET /roles")
	})

	t.Run("US-12_an_empty_page_gives_no_users_and_leaves_NextOffset_at_the_offset", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /users", 200, `[]`)
		f.On("GET /roles", 200, roleListJSON)
		data, err := loadUsersPage(session(), 600, "")
		if err != nil || len(data.Users) != 0 || data.Start != 600 || data.NextOffset != 600 || data.More {
			t.Errorf("data = %+v, err = %v", data, err)
		}
	})

	t.Run("US-13_a_search_matches_the_username_ignoring_case_and_echoes_the_search_as_typed", func(t *testing.T) {
		cases := []struct {
			search string
			want   []string
		}{
			{"BOB", []string{"u1", "u2"}},
			{"ali", []string{"u3"}},
		}
		for _, tc := range cases {
			t.Run("US-13_"+tc.search, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("GET /users", 200, "["+userJSON("u1", "Bob")+","+userJSON("u2", "bobby")+","+userJSON("u3", "Alice")+"]")
				f.On("GET /roles", 200, roleListJSON)
				data, err := loadUsersPage(session(), 0, tc.search)
				if err != nil || !reflect.DeepEqual(userIDs(data.Users), tc.want) || data.Search != tc.search {
					t.Errorf("users %q, Search %q, err = %v, want %q as typed", userIDs(data.Users), data.Search, err, tc.want)
				}
			})
		}
	})

	t.Run("US-14_a_search_also_matches_the_user_ID", func(t *testing.T) {
		cases := []struct{ search, want string }{
			{"c12", "abc123"},
			{"xyz", "xyz"},
			{"ABC", "abc123"},
		}
		for _, tc := range cases {
			t.Run("US-14_"+tc.search, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("GET /users", 200, `[{"user_id":"abc123","username":"Bob"},{"user_id":"zzz999","username":"carol"},{"user_id":"xyz","username":""}]`)
				f.On("GET /roles", 200, roleListJSON)
				data, err := loadUsersPage(session(), 0, tc.search)
				if err != nil || !reflect.DeepEqual(userIDs(data.Users), []string{tc.want}) {
					t.Errorf("users %q, err = %v, want %q", userIDs(data.Users), err, tc.want)
				}
			})
		}
	})

	t.Run("US-15_outer_whitespace_is_trimmed_from_the_search", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /users", 200, twoUsersJSON())
		f.On("GET /roles", 200, roleListJSON)
		data, err := loadUsersPage(session(), 0, "  Bob\t")
		if err != nil || !reflect.DeepEqual(userIDs(data.Users), []string{"u1"}) || data.Search != "Bob" {
			t.Errorf("users %q, Search %q, err = %v", userIDs(data.Users), data.Search, err)
		}
	})

	t.Run("US-16_a_whitespace-only_search_behaves_as_no_search", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		serveUserPages(f, userList(0, 1000, usernames()))
		f.On("GET /roles", 200, roleListJSON)
		data, err := loadUsersPage(session(), 0, "   ")
		if err != nil || len(data.Users) != 200 || data.Search != "" || !data.More {
			t.Errorf("%d users, Search %q, More %t, err = %v", len(data.Users), data.Search, data.More, err)
		}
		f.AssertLines(t, usersCall+"0", "GET /roles")
	})

	t.Run("US-17_a_search_reads_following_pages_until_a_short_page", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		serveUserPages(f, userList(0, 457, func(i int) string {
			if i%100 == 0 {
				return "bob"
			}
			return "x" + strconv.Itoa(i)
		}))
		f.On("GET /roles", 200, roleListJSON)
		data, err := loadUsersPage(session(), 0, "bob")
		if err != nil || !reflect.DeepEqual(userIDs(data.Users), []string{"u0", "u100", "u200", "u300", "u400"}) || data.More || data.NextOffset != 457 {
			t.Errorf("users %q, More %t, NextOffset %d, err = %v", userIDs(data.Users), data.More, data.NextOffset, err)
		}
		f.AssertLines(t, usersCall+"0", usersCall+"200", usersCall+"400", "GET /roles")
	})

	t.Run("US-18_a_search_reads_at_most_five_pages_and_reports_that_more_remain", func(t *testing.T) {
		cases := []struct {
			offset    int
			wantCalls []string
			wantIDs   []string
			wantNext  int
		}{
			{0, []string{usersCall + "0", usersCall + "200", usersCall + "400", usersCall + "600", usersCall + "800", "GET /roles"}, []string{"u0", "u250", "u500", "u750"}, 1000},
			{400, []string{usersCall + "400", usersCall + "600", usersCall + "800", usersCall + "1000", usersCall + "1200", "GET /roles"}, []string{"u500", "u750", "u1000", "u1250"}, 1400},
		}
		for _, tc := range cases {
			t.Run("US-18_offset_"+strconv.Itoa(tc.offset), func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				serveUserPages(f, userList(0, 2000, func(i int) string {
					if i%250 == 0 {
						return "bob"
					}
					return "x" + strconv.Itoa(i)
				}))
				f.On("GET /roles", 200, roleListJSON)
				data, err := loadUsersPage(session(), tc.offset, "bob")
				if err != nil || !reflect.DeepEqual(userIDs(data.Users), tc.wantIDs) || data.Start != tc.offset || data.NextOffset != tc.wantNext || !data.More {
					t.Errorf("users %q, Start %d, NextOffset %d, More %t, err = %v", userIDs(data.Users), data.Start, data.NextOffset, data.More, err)
				}
				f.AssertLines(t, tc.wantCalls...)
			})
		}
	})

	t.Run("US-19_a_full_page_followed_by_an_empty_page_ends_a_search_without_a_Load_more", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		serveUserPages(f, userList(0, 200, usernames()))
		f.On("GET /roles", 200, roleListJSON)
		data, err := loadUsersPage(session(), 0, "x")
		if err != nil || data.More || data.NextOffset != 200 {
			t.Errorf("More %t, NextOffset %d, err = %v", data.More, data.NextOffset, err)
		}
		f.AssertLines(t, usersCall+"0", usersCall+"200", "GET /roles")
	})

	t.Run("US-20_a_search_with_no_match_still_reports_how_far_it_read", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		serveUserPages(f, userList(0, 1400, usernames()))
		f.On("GET /roles", 200, roleListJSON)
		data, err := loadUsersPage(session(), 0, "zzz")
		if err != nil || len(data.Users) != 0 || !data.More || data.NextOffset != 1000 {
			t.Errorf("%d users, More %t, NextOffset %d, err = %v", len(data.Users), data.More, data.NextOffset, err)
		}
	})

	t.Run("US-21_role_names_are_keyed_by_role_ID", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /users", 200, twoUsersJSON())
		f.On("GET /roles", 200, roleListJSON)
		data, err := loadUsersPage(session(), 0, "")
		if err != nil || !reflect.DeepEqual(data.RoleNames, map[string]string{"r1": "Moderator", "r2": "Admin"}) {
			t.Errorf("RoleNames = %+v, err = %v", data.RoleNames, err)
		}
	})

	t.Run("US-22_a_403_on_the_roles_leaves_RoleNames_nil_and_still_returns_the_users", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /users", 200, twoUsersJSON())
		f.On("GET /roles", 403, ``)
		data, err := loadUsersPage(session(), 0, "")
		if err != nil || data.RoleNames != nil || len(data.Users) != 2 {
			t.Errorf("RoleNames = %+v, %d users, err = %v", data.RoleNames, len(data.Users), err)
		}
	})

	t.Run("US-23_a_failed_first_page_returns_the_error_and_does_not_read_the_roles", func(t *testing.T) {
		cases := []struct {
			name    string
			status  int
			detail  string
			message string
		}{
			{"500_with_a_detail", 500, "database down", "database down"},
			{"500_without_a_detail", 500, "", "Failed to load users"},
			{"403_with_a_detail", 403, "Missing permission", "Missing permission"},
			{"401", 401, "", ""},
		}
		for _, tc := range cases {
			t.Run("US-23_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.Refuse("GET /users", tc.status, tc.detail)
				f.On("GET /roles", 200, roleListJSON)
				data, err := loadUsersPage(session(), 0, "")
				if tc.status == 401 {
					if err != errUnauthorized {
						t.Errorf("err = %v, want errUnauthorized", err)
					}
				} else {
					assertAPIError(t, err, tc.status, tc.message, "GET", "/users?limit=200&offset=0")
				}
				if !reflect.DeepEqual(data, components.AdminUsersData{}) {
					t.Errorf("data = %+v, want the zero value", data)
				}
				f.AssertLines(t, usersCall+"0")
			})
		}
	})

	t.Run("US-24_a_failure_on_a_later_search_page_drops_the_matches_already_found", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		page := "[" + strings.Join(userList(0, 200, func(i int) string { return "bob" + strconv.Itoa(i) }), ",") + "]"
		f.Handle("GET /users", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Query().Get("offset") == "0" {
				fmt.Fprint(w, page)
				return
			}
			w.WriteHeader(500)
			fmt.Fprint(w, `{"detail":"database down"}`)
		})
		f.On("GET /roles", 200, roleListJSON)
		data, err := loadUsersPage(session(), 0, "bob")
		assertAPIError(t, err, 500, "database down", "GET", "/users?limit=200&offset=200")
		if !reflect.DeepEqual(data, components.AdminUsersData{}) {
			t.Errorf("data = %+v, want the zero value", data)
		}
		f.AssertLines(t, usersCall+"0", usersCall+"200")
	})

	t.Run("US-25_a_roles_failure_other_than_403_returns_the_error_and_empty_data", func(t *testing.T) {
		for _, status := range []int{500, 401} {
			t.Run("US-25_"+strconv.Itoa(status), func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("GET /users", 200, twoUsersJSON())
				f.Refuse("GET /roles", status, map[int]string{500: "database down", 401: ""}[status])
				data, err := loadUsersPage(session(), 0, "")
				if status == 401 {
					if err != errUnauthorized {
						t.Errorf("err = %v, want errUnauthorized", err)
					}
				} else {
					assertAPIError(t, err, 500, "database down", "GET", "/roles")
				}
				if !reflect.DeepEqual(data, components.AdminUsersData{}) {
					t.Errorf("data = %+v, want the zero value", data)
				}
			})
		}
	})
}

func TestAdminUsersListHandler(t *testing.T) {
	const list = "/admin/users/list"

	t.Run("US-26_renders_the_first_page_with_the_search_box_the_users_and_their_role_names", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /users", 200, twoUsersJSON())
		f.On("GET /roles", 200, roleListJSON)
		rec := serveRequest("GET", list, nil)
		assertStatusCode(t, rec, 200)
		if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
			t.Errorf("Content-Type = %q", got)
		}
		assertBodyHas(t, rec, `id="admin-users-search"`, "Bob", "alice", "u1", "u2", `href="/admin/users/u1"`, `href="/admin/users/u2"`, ">Moderator<", ">2 users<")
		assertBodyLacks(t, rec, ">r1<", "autofocus")
		f.AssertLines(t, usersCall+"0", "GET /roles")
	})

	t.Run("US-27_ignores_offset_and_search_in_its_query_string", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /users", 200, twoUsersJSON())
		f.On("GET /roles", 200, roleListJSON)
		rec := serveRequest("GET", list+"?offset=200&search=bob", nil)
		assertBodyHas(t, rec, `href="/admin/users/u1"`, `href="/admin/users/u2"`)
		f.AssertLines(t, usersCall+"0", "GET /roles")
	})

	t.Run("US-28_an_unreadable_role_list_still_renders_the_users_with_their_role_IDs", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /users", 200, twoUsersJSON())
		f.On("GET /roles", 403, ``)
		rec := serveRequest("GET", list, nil)
		assertStatusCode(t, rec, 200)
		assertBodyHas(t, rec, "u1", "Bob", ">r1<")
		assertBodyLacks(t, rec, "Moderator")
	})

	t.Run("US-29_no_users_renders_the_empty_state", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /users", 200, `[]`)
		f.On("GET /roles", 200, roleListJSON)
		rec := serveRequest("GET", list, nil)
		assertStatusCode(t, rec, 200)
		assertBodyHas(t, rec, "admin-users-empty", "No users found")
		assertBodyLacks(t, rec, "/admin/users/u")
	})

	t.Run("US-30_a_full_first_page_renders_a_Load_more_button_for_the_next_offset", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /users", 200, "["+strings.Join(userList(0, 200, usernames()), ",")+"]")
		f.On("GET /roles", 200, roleListJSON)
		rec := serveRequest("GET", list, nil)
		assertBodyHas(t, rec, "admin-users-more-button", `hx-get="/admin/users/rows?offset=200"`)
	})

	t.Run("US-31_an_API_failure_answers_with_its_status_and_message_and_no_list", func(t *testing.T) {
		cases := []struct {
			name    string
			prepare func(f *fakeapi.FakeAPI)
			status  int
			message string
		}{
			{"users_403", func(f *fakeapi.FakeAPI) { f.Problem("GET /users", 403, "Missing permission") }, 403, "Missing permission"},
			{"users_500_without_a_detail", func(f *fakeapi.FakeAPI) { f.On("GET /users", 500, ``) }, 500, "Failed to load users"},
			{"roles_500", func(f *fakeapi.FakeAPI) {
				f.On("GET /users", 200, twoUsersJSON())
				f.Problem("GET /roles", 500, "database down")
			}, 500, "database down"},
		}
		for _, tc := range cases {
			t.Run("US-31_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				tc.prepare(f)
				rec := serveRequest("GET", list, nil)
				assertFailure(t, rec, tc.status, tc.message)
				assertBodyLacks(t, rec, `href="/admin/users/`)
			})
		}
	})

	t.Run("US-32_a_401_from_the_users_call_or_the_roles_call_sends_the_browser_to_the_login_page", func(t *testing.T) {
		cases := []struct {
			name    string
			prepare func(f *fakeapi.FakeAPI)
		}{
			{"users", func(f *fakeapi.FakeAPI) { f.On("GET /users", 401, ``) }},
			{"roles", func(f *fakeapi.FakeAPI) {
				f.On("GET /users", 200, twoUsersJSON())
				f.On("GET /roles", 401, ``)
			}},
		}
		for _, tc := range cases {
			t.Run("US-32_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				tc.prepare(f)
				assertLoginRedirect(t, serveRequest("GET", list, nil))
			})
		}
	})
}

func TestAdminUserRowsHandler(t *testing.T) {
	const rows = "/admin/users/rows"

	t.Run("US-33_returns_the_rows_and_the_count_line_without_the_list_shell", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /users", 200, twoUsersJSON())
		f.On("GET /roles", 200, roleListJSON)
		rec := serveRequest("GET", rows, nil)
		assertStatusCode(t, rec, 200)
		assertBodyHas(t, rec, `href="/admin/users/u1"`, `href="/admin/users/u2"`, ">2 users<")
		if !strings.Contains(testutil.TagByID(rec.Body.String(), "admin-users-count"), "hx-swap-oob") {
			t.Error("the count line has no hx-swap-oob")
		}
		assertBodyLacks(t, rec, "autofocus", "admin-users-search")
		f.AssertLines(t, usersCall+"0", "GET /roles")
	})

	t.Run("US-34_a_positive_offset_goes_to_the_API_and_the_first_row_takes_focus", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /users", 200, "["+strings.Join(userList(0, 3, usernames()), ",")+"]")
		f.On("GET /roles", 200, roleListJSON)
		rec := serveRequest("GET", rows+"?offset=200", nil)
		assertStatusCode(t, rec, 200)
		assertBodyHas(t, rec, `href="/admin/users/u0" autofocus`, ">3 more users<")
		if got := strings.Count(rec.Body.String(), "autofocus"); got != 1 {
			t.Errorf("autofocus appears %d times, want once", got)
		}
		f.AssertLines(t, usersCall+"200", "GET /roles")
	})

	t.Run("US-35_an_absent_or_empty_offset_reads_from_the_start_with_no_focus_change", func(t *testing.T) {
		for _, query := range []string{"", "?offset=", "?offset=0"} {
			t.Run("US-35_"+query, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("GET /users", 200, twoUsersJSON())
				f.On("GET /roles", 200, roleListJSON)
				rec := serveRequest("GET", rows+query, nil)
				assertBodyLacks(t, rec, "autofocus")
				f.AssertLines(t, usersCall+"0", "GET /roles")
			})
		}
	})

	t.Run("US-36_an_offset_that_is_not_a_whole_number_from_0_is_refused_before_any_API_call", func(t *testing.T) {
		for _, offset := range []string{"abc", "1.5", "-1", "99999999999999999999"} {
			t.Run("US-36_"+offset, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				rec := serveRequest("GET", rows+"?offset="+offset, nil)
				assertFailure(t, rec, 400, "The offset must be a whole number from 0")
				if got := rec.Body.String(); got != "The offset must be a whole number from 0" {
					t.Errorf("body = %q", got)
				}
				f.AssertLines(t)
			})
		}
	})

	t.Run("US-37_the_search_query_filters_the_rows", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /users", 200, "["+userJSON("u1", "Bob")+","+userJSON("u2", "bobby")+","+userJSON("u3", "Alice")+"]")
		f.On("GET /roles", 200, roleListJSON)
		rec := serveRequest("GET", rows+"?search=bob", nil)
		assertStatusCode(t, rec, 200)
		assertBodyHas(t, rec, `href="/admin/users/u1"`, `href="/admin/users/u2"`, ">2 users<")
		assertBodyLacks(t, rec, `href="/admin/users/u3"`)
	})

	t.Run("US-38_a_search_from_an_offset_reads_five_pages_from_that_offset_and_the_Load_more_button_keeps_both", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		serveUserPages(f, userList(0, 2000, usernames()))
		f.On("GET /roles", 200, roleListJSON)
		rec := serveRequest("GET", rows+"?offset=400&search=zzz", nil)
		assertStatusCode(t, rec, 200)
		f.AssertLines(t, usersCall+"400", usersCall+"600", usersCall+"800", usersCall+"1000", usersCall+"1200", "GET /roles")
		button := testutil.TagByID(rec.Body.String(), "admin-users-more-button")
		if button == "" || !strings.Contains(button, "offset=1400") || !strings.Contains(button, "search=zzz") {
			t.Errorf("Load more button = %q, want offset=1400 and search=zzz", button)
		}
		assertBodyHas(t, rec, ">No matches in the first 1400 users<")
		assertBodyLacks(t, rec, "No more users")
	})

	t.Run("US-39_a_page_past_the_end_reports_no_more_users_and_takes_focus", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /users", 200, `[]`)
		f.On("GET /roles", 200, roleListJSON)
		rec := serveRequest("GET", rows+"?offset=400", nil)
		assertStatusCode(t, rec, 200)
		assertBodyHas(t, rec, "admin-users-end", "autofocus", ">No more users<")
		if got := rec.Body.String(); strings.Count(got, "No more users") < 2 {
			t.Errorf("the notice and the count line should both say No more users:\n%s", got)
		}
	})

	t.Run("US-40_a_search_with_no_match_on_a_short_list_says_so", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /users", 200, twoUsersJSON())
		f.On("GET /roles", 200, roleListJSON)
		rec := serveRequest("GET", rows+"?search=zzz", nil)
		assertStatusCode(t, rec, 200)
		assertBodyHas(t, rec, ">No matches<")
		assertBodyLacks(t, rec, `href="/admin/users/`)
	})

	t.Run("US-41_an_API_failure_answers_with_its_status_and_message_and_no_rows", func(t *testing.T) {
		cases := []struct {
			name    string
			prepare func(f *fakeapi.FakeAPI)
			status  int
			message string
		}{
			{"users_403", func(f *fakeapi.FakeAPI) { f.Problem("GET /users", 403, "Missing permission") }, 403, "Missing permission"},
			{"users_500_without_a_detail", func(f *fakeapi.FakeAPI) { f.On("GET /users", 500, ``) }, 500, "Failed to load users"},
			{"roles_500", func(f *fakeapi.FakeAPI) {
				f.On("GET /users", 200, twoUsersJSON())
				f.Problem("GET /roles", 500, "database down")
			}, 500, "database down"},
		}
		for _, tc := range cases {
			t.Run("US-41_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				tc.prepare(f)
				rec := serveRequest("GET", rows, nil)
				assertFailure(t, rec, tc.status, tc.message)
				assertBodyLacks(t, rec, `href="/admin/users/`)
			})
		}
	})

	t.Run("US-42_a_401_from_the_users_call_or_the_roles_call_sends_the_browser_to_the_login_page", func(t *testing.T) {
		cases := []struct {
			name    string
			prepare func(f *fakeapi.FakeAPI)
		}{
			{"users", func(f *fakeapi.FakeAPI) { f.On("GET /users", 401, ``) }},
			{"roles", func(f *fakeapi.FakeAPI) {
				f.On("GET /users", 200, twoUsersJSON())
				f.On("GET /roles", 401, ``)
			}},
		}
		for _, tc := range cases {
			t.Run("US-42_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				tc.prepare(f)
				assertLoginRedirect(t, serveRequest("GET", rows, nil))
			})
		}
	})
}

func TestLoadUserEditor(t *testing.T) {
	session := func() apiSession { return newSession(testutil.NewRequest()) }
	editorCalls := []string{"GET /users/u1", "GET /roles", "GET /users/u1/links", "GET /users/u1/permissions"}

	t.Run("US-43_loading_without_a_user_reads_the_user_roles_links_and_permissions_in_order", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		stubUserEditor(f)
		data, err := loadUserEditor(session(), "u1", nil, true)
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if !reflect.DeepEqual(data.User, components.UserAccount{UserID: "u1", Username: "Bob", Roles: []string{"r1"}}) {
			t.Errorf("User = %+v", data.User)
		}
		if len(data.Roles) != 2 || data.Roles[0].ID != "r1" || data.Roles[1].ID != "r2" || !data.RolesReadable {
			t.Errorf("Roles = %+v, RolesReadable = %t", data.Roles, data.RolesReadable)
		}
		wantLink := components.LinkedAccount{Platform: "discord", PlatformUsername: "bob#1", PlatformID: "1", Verified: true}
		if len(data.Links) != 1 || data.Links[0] != wantLink || data.LinksError != "" || data.PermissionsErr != "" {
			t.Errorf("Links = %+v, LinksError = %q, PermissionsErr = %q", data.Links, data.LinksError, data.PermissionsErr)
		}
		if !reflect.DeepEqual(data.Permissions, []string{"pets.read", "pets.max:5"}) {
			t.Errorf("Permissions = %q", data.Permissions)
		}
		f.AssertLines(t, editorCalls...)
	})

	t.Run("US-44_the_user_ID_is_path-escaped_in_the_user_links_and_permissions_paths", func(t *testing.T) {
		for _, tc := range idEscapes {
			t.Run("US-44_"+tc.escaped, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				base := "/users/" + tc.escaped
				f.On("GET "+base, 200, userJSON(tc.id, "Bob"))
				f.On("GET /roles", 200, `[]`)
				f.On("GET "+base+"/links", 200, `[]`)
				f.On("GET "+base+"/permissions", 200, `[]`)
				if _, err := loadUserEditor(session(), tc.id, nil, true); err != nil {
					t.Fatalf("err = %v", err)
				}
				f.AssertLines(t, "GET "+base, "GET /roles", "GET "+base+"/links", "GET "+base+"/permissions")
			})
		}
	})

	t.Run("US-45_a_user_passed_in_is_used_as_given_and_not_fetched_again", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		stubUserEditor(f)
		f.On("GET /users/u1", 200, userJSON("u1", "someone-else"))
		given := components.UserAccount{UserID: "u1", Username: "carol", Roles: []string{"r2"}}
		data, err := loadUserEditor(session(), "u1", &given, true)
		if err != nil || !reflect.DeepEqual(data.User, given) {
			t.Errorf("User = %+v, err = %v, want the given user", data.User, err)
		}
		f.AssertLines(t, editorCalls[1:]...)
	})

	t.Run("US-46_without_links_it_does_not_call_the_links_route", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		stubUserEditor(f)
		given := components.UserAccount{UserID: "u1", Username: "Bob"}
		data, err := loadUserEditor(session(), "u1", &given, false)
		if err != nil || data.Links != nil || data.LinksError != "" {
			t.Errorf("Links = %+v, LinksError = %q, err = %v", data.Links, data.LinksError, err)
		}
		f.AssertLines(t, "GET /roles", "GET /users/u1/permissions")
	})

	t.Run("US-47_an_unreadable_role_list_is_reported_as_not_readable_and_the_rest_still_loads", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		stubUserEditor(f)
		f.On("GET /roles", 403, ``)
		data, err := loadUserEditor(session(), "u1", nil, true)
		if err != nil || data.Roles != nil || data.RolesReadable {
			t.Errorf("Roles = %+v, RolesReadable = %t, err = %v", data.Roles, data.RolesReadable, err)
		}
		if data.User.UserID != "u1" || len(data.Links) != 1 || len(data.Permissions) != 2 {
			t.Errorf("data = %+v, want the user, the link and the permissions", data)
		}
	})

	t.Run("US-48_a_links_failure_becomes_a_message_on_the_data_and_does_not_fail_the_load", func(t *testing.T) {
		cases := []struct {
			name    string
			status  int
			detail  string
			body    string
			message string
		}{
			{"500_with_a_detail", 500, "database down", "", "database down"},
			{"500_without_a_detail", 500, "", "", "Failed to load the linked accounts"},
			{"404_with_a_detail", 404, "Not found", "", "Not found"},
			{"200_not_json", 200, "", "not json", "Failed to load the linked accounts"},
		}
		for _, tc := range cases {
			t.Run("US-48_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				stubUserEditor(f)
				if tc.body != "" {
					f.On("GET /users/u1/links", tc.status, tc.body)
				} else {
					f.Refuse("GET /users/u1/links", tc.status, tc.detail)
				}
				data, err := loadUserEditor(session(), "u1", nil, true)
				if err != nil || data.LinksError != tc.message || data.Links != nil || len(data.Permissions) != 2 {
					t.Errorf("LinksError = %q, Links = %+v, Permissions = %q, err = %v", data.LinksError, data.Links, data.Permissions, err)
				}
			})
		}
	})

	t.Run("US-49_a_permissions_failure_becomes_a_message_on_the_data_and_does_not_fail_the_load", func(t *testing.T) {
		cases := []struct {
			name    string
			status  int
			detail  string
			body    string
			message string
		}{
			{"500_with_a_detail", 500, "database down", "", "database down"},
			{"500_without_a_detail", 500, "", "", "Failed to load the permissions"},
			{"200_not_json", 200, "", "not json", "Failed to load the permissions"},
		}
		for _, tc := range cases {
			t.Run("US-49_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				stubUserEditor(f)
				if tc.body != "" {
					f.On("GET /users/u1/permissions", tc.status, tc.body)
				} else {
					f.Refuse("GET /users/u1/permissions", tc.status, tc.detail)
				}
				data, err := loadUserEditor(session(), "u1", nil, true)
				if err != nil || data.PermissionsErr != tc.message || data.Permissions != nil || len(data.Links) != 1 {
					t.Errorf("PermissionsErr = %q, Permissions = %q, Links = %+v, err = %v", data.PermissionsErr, data.Permissions, data.Links, err)
				}
			})
		}
	})

	t.Run("US-50_a_failed_user_load_returns_the_error_and_calls_nothing_further", func(t *testing.T) {
		cases := []struct {
			name    string
			status  int
			detail  string
			body    string
			wantAPI int
			message string
		}{
			{"404_with_a_detail", 404, "User not found", "", 404, "User not found"},
			{"500_without_a_detail", 500, "", "", 500, "Failed to load the user"},
			{"401", 401, "", "", 401, ""},
			{"200_not_json", 200, "", "not json", 502, "Failed to load the user"},
		}
		for _, tc := range cases {
			t.Run("US-50_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				stubUserEditor(f)
				if tc.body != "" {
					f.On("GET /users/u1", tc.status, tc.body)
				} else {
					f.Refuse("GET /users/u1", tc.status, tc.detail)
				}
				data, err := loadUserEditor(session(), "u1", nil, true)
				if tc.status == 401 {
					if err != errUnauthorized {
						t.Errorf("err = %v, want errUnauthorized", err)
					}
				} else {
					assertAPIError(t, err, tc.wantAPI, tc.message, "GET", "/users/u1")
				}
				if !reflect.DeepEqual(data, components.AdminUserData{}) {
					t.Errorf("data = %+v, want the zero value", data)
				}
				f.AssertLines(t, "GET /users/u1")
			})
		}
	})

	t.Run("US-51_a_roles_failure_other_than_403_returns_the_error_and_calls_nothing_further", func(t *testing.T) {
		for _, status := range []int{500, 401} {
			t.Run("US-51_"+strconv.Itoa(status), func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				stubUserEditor(f)
				if status == 401 {
					f.On("GET /roles", 401, ``)
				} else {
					f.Problem("GET /roles", 500, "database down")
				}
				data, err := loadUserEditor(session(), "u1", nil, true)
				if status == 401 {
					if err != errUnauthorized {
						t.Errorf("err = %v, want errUnauthorized", err)
					}
				} else {
					assertAPIError(t, err, 500, "database down", "GET", "/roles")
				}
				if !reflect.DeepEqual(data, components.AdminUserData{}) {
					t.Errorf("data = %+v, want the zero value", data)
				}
				f.AssertLines(t, "GET /users/u1", "GET /roles")
			})
		}
	})

	t.Run("US-52_a_401_on_the_links_or_the_permissions_call_returns_the_sign-in_error", func(t *testing.T) {
		for _, route := range []string{"GET /users/u1/links", "GET /users/u1/permissions"} {
			t.Run("US-52_"+route, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				stubUserEditor(f)
				f.On(route, 401, ``)
				data, err := loadUserEditor(session(), "u1", nil, true)
				if err != errUnauthorized || !reflect.DeepEqual(data, components.AdminUserData{}) {
					t.Errorf("data = %+v, err = %v, want the zero value and errUnauthorized", data, err)
				}
			})
		}
	})
}

func TestAdminUserEditorHandler(t *testing.T) {
	const editor = "/admin/users/u1/editor"
	editorCalls := []string{"GET /users/u1", "GET /roles", "GET /users/u1/links", "GET /users/u1/permissions"}

	t.Run("US-53_renders_the_header_form_roles_links_and_permissions", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		stubUserEditor(f)
		rec := serveRequest("GET", editor, nil)
		assertStatusCode(t, rec, 200)
		if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
			t.Errorf("Content-Type = %q", got)
		}
		assertBodyHas(t, rec, "Bob", "u1", `id="admin-user-form"`,
			`name="loaded_username" value="Bob"`, `name="loaded_roles" value="r1"`, `name="roles_editable" value="1"`,
			`name="roles" value="r1" checked`, `name="roles" value="r2" class=`, "Moderator", "Administers", "discord", "bob#1", "pets.read", "pets.max:5")
		assertBodyLacks(t, rec, `name="roles" value="r2" checked`)
		f.AssertLines(t, editorCalls...)
	})

	t.Run("US-54_the_ID_from_the_path_is_escaped_again_for_the_API", func(t *testing.T) {
		for _, tc := range idEscapes {
			t.Run("US-54_"+tc.escaped, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				base := "/users/" + tc.escaped
				f.On("GET "+base, 200, userJSON(tc.id, "Bob"))
				f.On("GET /roles", 200, `[]`)
				f.On("GET "+base+"/links", 200, `[]`)
				f.On("GET "+base+"/permissions", 200, `[]`)
				rec := serveRequest("GET", "/admin/users/"+tc.escaped+"/editor", nil)
				assertStatusCode(t, rec, 200)
				f.AssertLines(t, "GET "+base, "GET /roles", "GET "+base+"/links", "GET "+base+"/permissions")
			})
		}
	})

	t.Run("US-55_an_unreadable_role_list_hides_the_role_controls", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		stubUserEditor(f)
		f.On("GET /roles", 403, ``)
		rec := serveRequest("GET", editor, nil)
		assertStatusCode(t, rec, 200)
		assertBodyHas(t, rec, ">r1<", "admin-user-roles-note", `name="loaded_roles" value="r1"`)
		assertBodyLacks(t, rec, "roles_editable", `name="roles"`, "kept_roles")
	})

	t.Run("US-56_a_held_role_that_is_not_in_the_role_list_is_carried_as_a_kept_role", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		stubUserEditor(f)
		f.On("GET /users/u1", 200, userJSON("u1", "Bob", "r1", "r9"))
		rec := serveRequest("GET", editor, nil)
		assertBodyHas(t, rec, `name="kept_roles" value="r9"`, `name="loaded_roles" value="r9"`)
		assertBodyLacks(t, rec, `name="roles" value="r9"`)
	})

	t.Run("US-57_a_links_or_permissions_failure_shows_its_message_and_the_rest_of_the_editor", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		stubUserEditor(f)
		f.Problem("GET /users/u1/links", 500, "database down")
		f.On("GET /users/u1/permissions", 500, ``)
		rec := serveRequest("GET", editor, nil)
		assertStatusCode(t, rec, 200)
		assertBodyHas(t, rec, "admin-user-links-error", "database down", "admin-user-permissions-error", "Failed to load the permissions", `id="admin-user-form"`)
	})

	t.Run("US-58_a_user_with_no_links_and_no_permissions_shows_the_empty_states", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		stubUserEditor(f)
		f.On("GET /users/u1/links", 200, `[]`)
		f.On("GET /users/u1/permissions", 200, `[]`)
		rec := serveRequest("GET", editor, nil)
		assertStatusCode(t, rec, 200)
		assertBodyHas(t, rec, "admin-user-links-empty", "admin-user-permissions-empty")
	})

	t.Run("US-59_a_failed_load_answers_with_its_status_and_message_and_no_editor", func(t *testing.T) {
		cases := []struct {
			name    string
			prepare func(f *fakeapi.FakeAPI)
			status  int
			message string
		}{
			{"user_404", func(f *fakeapi.FakeAPI) { f.Problem("GET /users/u1", 404, "User not found") }, 404, "User not found"},
			{"user_500_without_a_detail", func(f *fakeapi.FakeAPI) { f.On("GET /users/u1", 500, ``) }, 500, "Failed to load the user"},
			{"roles_500", func(f *fakeapi.FakeAPI) { f.Problem("GET /roles", 500, "database down") }, 500, "database down"},
		}
		for _, tc := range cases {
			t.Run("US-59_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				stubUserEditor(f)
				tc.prepare(f)
				rec := serveRequest("GET", editor, nil)
				assertFailure(t, rec, tc.status, tc.message)
				assertBodyLacks(t, rec, "admin-user-form")
			})
		}
	})

	t.Run("US-60_a_401_from_any_of_the_four_calls_sends_the_browser_to_the_login_page", func(t *testing.T) {
		for _, route := range editorCalls {
			t.Run("US-60_"+route, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				stubUserEditor(f)
				f.On(route, 401, ``)
				assertLoginRedirect(t, serveRequest("GET", editor, nil))
			})
		}
	})
}

func TestRenderUserSave(t *testing.T) {
	t.Run("US-61_writes_the_form_the_header_the_permissions_and_the_status_line_then_clears_the_banner", func(t *testing.T) {
		data := components.AdminUserData{
			User:          components.UserAccount{UserID: "u1", Username: "carol", Roles: []string{"r1"}},
			Roles:         []components.Role{{ID: "r1", Name: "Moderator", Description: "Moderates"}, {ID: "r2", Name: "Admin", Description: "Administers"}},
			RolesReadable: true,
			Permissions:   []string{"pets.read"},
		}
		rec := httptest.NewRecorder()
		renderUserSave(rec, testutil.NewRequest(), data, "Saved")
		if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
			t.Errorf("Content-Type = %q", got)
		}
		body := rec.Body.String()
		for _, want := range []string{`id="admin-user-form"`, `name="loaded_username" value="carol"`, "carol", "u1", "pets.read", `id="admin-user-status" hx-swap-oob="innerHTML">Saved<`, `<div id="page-error" hx-swap-oob="innerHTML"></div>`} {
			if !strings.Contains(body, want) {
				t.Errorf("the body lacks %q:\n%s", want, body)
			}
		}
		for _, id := range []string{"admin-user-header", "admin-user-permissions-section"} {
			if !strings.Contains(testutil.TagByID(body, id), `hx-swap-oob="true"`) {
				t.Errorf("%s has no hx-swap-oob", id)
			}
		}
	})
}

func TestAdminUserSaveHandler(t *testing.T) {
	const save = "/admin/users/u1"
	const put = "PUT /users/u1 "
	reload := []string{"GET /roles", "GET /users/u1/permissions"}

	saveWith := func(t *testing.T, form url.Values, saved string) (*fakeapi.FakeAPI, *httptest.ResponseRecorder) {
		t.Helper()
		f := fakeapi.NewFakeAPI(t)
		stubUserEditor(f)
		f.On("PUT /users/u1", 200, saved)
		return f, serveRequest("POST", save, form)
	}
	carol := userJSON("u1", "carol", "r1")

	t.Run("US-01_posted_roles_that_match_the_loaded_roles_as_a_set_are_not_sent", func(t *testing.T) {
		cases := []struct {
			name          string
			roles, loaded []string
		}{
			{"same_members_same_order", []string{"r1", "r2"}, []string{"r1", "r2"}},
			{"duplicate_in_the_posted_roles", []string{"r1", "r1"}, []string{"r1"}},
			{"duplicate_in_the_loaded_roles", []string{"r1"}, []string{"r1", "r1"}},
		}
		for _, tc := range cases {
			t.Run("US-01_"+tc.name, func(t *testing.T) {
				f, rec := saveWith(t, userSaveForm(url.Values{"roles": tc.roles, "loaded_roles": tc.loaded}), carol)
				assertStatusCode(t, rec, 200)
				assertBodyHas(t, rec, "Nothing to save")
				f.AssertLines(t)
			})
		}
	})

	t.Run("US-02_posted_and_loaded_roles_that_are_both_empty_match_and_posted_roles_over_no_loaded_roles_are_sent", func(t *testing.T) {
		t.Run("US-02_both_empty", func(t *testing.T) {
			f, rec := saveWith(t, userSaveForm(url.Values{"roles": {}, "loaded_roles": {}}), carol)
			assertStatusCode(t, rec, 200)
			assertBodyHas(t, rec, "Nothing to save")
			f.AssertLines(t)
		})

		t.Run("US-02_posted_role_and_no_loaded_roles", func(t *testing.T) {
			f, _ := saveWith(t, userSaveForm(url.Values{"roles": {"r1"}, "loaded_roles": {}}), carol)
			f.AssertLines(t, append([]string{put + `{"roles":["r1"]}`}, reload...)...)
		})
	})

	t.Run("US-03_posted_roles_that_differ_from_the_loaded_roles_as_a_set_are_sent", func(t *testing.T) {
		cases := []struct {
			name          string
			roles, loaded []string
			body          string
		}{
			{"other_member", []string{"r1", "r2"}, []string{"r1", "r3"}, `{"roles":["r1","r2"]}`},
			{"subset", []string{"r1"}, []string{"r1", "r2"}, `{"roles":["r1"]}`},
			{"duplicate_hides_a_missing_member", []string{"r1", "r1"}, []string{"r1", "r2"}, `{"roles":["r1","r1"]}`},
		}
		for _, tc := range cases {
			t.Run("US-03_"+tc.name, func(t *testing.T) {
				f, _ := saveWith(t, userSaveForm(url.Values{"roles": tc.roles, "loaded_roles": tc.loaded}), carol)
				f.AssertLines(t, append([]string{put + tc.body}, reload...)...)
			})
		}
	})

	t.Run("US-62_a_changed_username_sends_only_the_username_then_reloads_roles_and_permissions", func(t *testing.T) {
		f, rec := saveWith(t, userSaveForm(url.Values{"username": {"carol"}}), carol)
		assertStatusCode(t, rec, 200)
		f.AssertLines(t, append([]string{put + `{"username":"carol"}`}, reload...)...)
		fakeapi.AssertContentType(t, f.Calls()[0], "application/json")
	})

	t.Run("US-63_the_username_is_trimmed_before_it_is_sent", func(t *testing.T) {
		f, _ := saveWith(t, userSaveForm(url.Values{"username": {"  carol \t"}}), carol)
		f.AssertLines(t, append([]string{put + `{"username":"carol"}`}, reload...)...)
	})

	t.Run("US-64_a_changed_role_set_sends_only_the_roles", func(t *testing.T) {
		f, _ := saveWith(t, userSaveForm(url.Values{"roles": {"r1", "r2"}}), userJSON("u1", "Bob", "r1", "r2"))
		f.AssertLines(t, append([]string{put + `{"roles":["r1","r2"]}`}, reload...)...)
	})

	t.Run("US-65_a_changed_username_and_a_changed_role_set_are_sent_together", func(t *testing.T) {
		f, _ := saveWith(t, userSaveForm(url.Values{"username": {"carol"}, "roles": {"r2"}}), userJSON("u1", "carol", "r2"))
		f.AssertLines(t, append([]string{put + `{"roles":["r2"],"username":"carol"}`}, reload...)...)
	})

	t.Run("US-66_kept_roles_are_sent_after_the_checked_roles", func(t *testing.T) {
		f, _ := saveWith(t, userSaveForm(url.Values{"roles": {"r1"}, "kept_roles": {"r9"}, "loaded_roles": {"r9"}}), userJSON("u1", "Bob", "r1", "r9"))
		f.AssertLines(t, append([]string{put + `{"roles":["r1","r9"]}`}, reload...)...)
	})

	t.Run("US-67_unchecking_every_role_sends_an_empty_list_and_not_null", func(t *testing.T) {
		f, _ := saveWith(t, userSaveForm(url.Values{"roles": {}}), userJSON("u1", "Bob"))
		f.AssertLines(t, append([]string{put + `{"roles":[]}`}, reload...)...)
	})

	t.Run("US-68_the_same_roles_in_another_order_do_not_count_as_a_change", func(t *testing.T) {
		f, _ := saveWith(t, userSaveForm(url.Values{"username": {"carol"}, "roles": {"r2", "r1"}, "loaded_roles": {"r1", "r2"}}), carol)
		f.AssertLines(t, append([]string{put + `{"username":"carol"}`}, reload...)...)
	})

	t.Run("US-69_roles_are_ignored_unless_the_form_marks_them_editable", func(t *testing.T) {
		variants := []struct {
			name     string
			editable url.Values
		}{
			{"no_roles_editable", url.Values{"roles_editable": {}}},
			{"roles_editable_0", url.Values{"roles_editable": {"0"}}},
		}
		for _, v := range variants {
			t.Run("US-69_"+v.name+"_username_unchanged", func(t *testing.T) {
				overrides := url.Values{"roles": {"r2"}, "loaded_roles": {"r1"}, "roles_editable": v.editable["roles_editable"]}
				f, rec := saveWith(t, userSaveForm(overrides), carol)
				assertStatusCode(t, rec, 200)
				assertBodyHas(t, rec, "Nothing to save")
				f.AssertLines(t)
			})
			t.Run("US-69_"+v.name+"_username_changed", func(t *testing.T) {
				overrides := url.Values{"username": {"carol"}, "roles": {"r2"}, "loaded_roles": {"r1"}, "roles_editable": v.editable["roles_editable"]}
				f, _ := saveWith(t, userSaveForm(overrides), carol)
				f.AssertLines(t, append([]string{put + `{"username":"carol"}`}, reload...)...)
			})
		}
	})

	t.Run("US-70_a_save_with_no_change_sends_nothing_and_says_so", func(t *testing.T) {
		f, rec := saveWith(t, userSaveForm(nil), carol)
		assertStatusCode(t, rec, 200)
		if got := rec.Header().Get("HX-Reswap"); got != "none" {
			t.Errorf("HX-Reswap = %q, want none", got)
		}
		assertBodyHas(t, rec, `id="admin-user-status" hx-swap-oob="innerHTML">Nothing to save<`)
		f.AssertLines(t)
	})

	t.Run("US-71_an_untouched_empty_username_does_not_trigger_the_username_refusal", func(t *testing.T) {
		f, rec := saveWith(t, userSaveForm(url.Values{"username": {""}, "loaded_username": {""}, "loaded_roles": {}}), userJSON("u1", "", "r1"))
		assertStatusCode(t, rec, 200)
		f.AssertLines(t, append([]string{put + `{"roles":["r1"]}`}, reload...)...)
	})

	t.Run("US-72_a_username_with_outer_whitespace_that_equals_the_loaded_one_is_not_a_change", func(t *testing.T) {
		f, rec := saveWith(t, userSaveForm(url.Values{"username": {" bob "}, "loaded_username": {" bob "}}), carol)
		assertStatusCode(t, rec, 200)
		assertBodyHas(t, rec, "Nothing to save")
		f.AssertLines(t)
	})

	t.Run("US-73_an_emptied_or_blank_username_is_refused_and_the_field_restored_with_the_typed_text", func(t *testing.T) {
		cases := []struct {
			name           string
			username, held string
		}{
			{"emptied", "", "Bob"},
			{"blank_over_a_name", "  ", "Bob"},
			{"blank_over_no_name", "  ", ""},
		}
		for _, tc := range cases {
			t.Run("US-73_"+tc.name, func(t *testing.T) {
				f, rec := saveWith(t, userSaveForm(url.Values{"username": {tc.username}, "loaded_username": {tc.held}}), carol)
				assertFailure(t, rec, 400, "Enter a username")
				assertBodyHas(t, rec, "admin-user-username-error", `id="admin-user-status" hx-swap-oob="innerHTML"></p>`)
				assertOutOfBand(t, rec, "admin-user-username-field", true)
				if tc.username != "" {
					assertBodyHas(t, rec, `value="`+tc.username+`"`)
				}
				f.AssertLines(t)
			})
		}
	})

	t.Run("US-74_a_refused_username_stops_a_role_change_from_being_sent", func(t *testing.T) {
		f, rec := saveWith(t, userSaveForm(url.Values{"username": {""}, "roles": {"r2"}}), carol)
		assertFailure(t, rec, 400, "Enter a username")
		f.AssertLines(t)
	})

	t.Run("US-75_the_user_ID_is_path-escaped_in_the_PUT_path", func(t *testing.T) {
		for _, tc := range idEscapes {
			t.Run("US-75_"+tc.escaped, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				base := "/users/" + tc.escaped
				f.On("PUT "+base, 200, userJSON(tc.id, "carol"))
				f.On("GET /roles", 200, roleListJSON)
				f.On("GET "+base+"/permissions", 200, `[]`)
				rec := serveRequest("POST", "/admin/users/"+tc.escaped, userSaveForm(url.Values{"username": {"carol"}}))
				assertStatusCode(t, rec, 200)
				f.AssertLines(t, "PUT "+base+` {"username":"carol"}`, "GET /roles", "GET "+base+"/permissions")
			})
		}
	})

	t.Run("US-76_after_a_save_the_response_carries_the_APIs_saved_user_not_the_submitted_values", func(t *testing.T) {
		_, rec := saveWith(t, userSaveForm(url.Values{"username": {"Carol"}}), userJSON("u1", "carol", "r1", "r2"))
		assertStatusCode(t, rec, 200)
		assertBodyHas(t, rec, "carol", "u1", `name="loaded_username" value="carol"`, `name="loaded_roles" value="r1"`, `name="loaded_roles" value="r2"`,
			"pets.read", `id="admin-user-status" hx-swap-oob="innerHTML">Saved<`, `id="admin-user-form"`)
		assertBodyLacks(t, rec, "Carol")
	})

	t.Run("US-77_a_permissions_failure_after_the_write_still_saves_and_says_the_permissions_are_out_of_date", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		stubUserEditor(f)
		f.On("PUT /users/u1", 200, carol)
		f.Problem("GET /users/u1/permissions", 500, "database down")
		rec := serveRequest("POST", save, userSaveForm(url.Values{"username": {"carol"}}))
		assertStatusCode(t, rec, 200)
		assertBodyHas(t, rec, "Saved, but the permissions below are out of date", "admin-user-permissions-error", "database down", `name="loaded_username" value="carol"`)
	})

	t.Run("US-78_an_unreadable_role_list_after_the_write_still_saves", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		stubUserEditor(f)
		f.On("PUT /users/u1", 200, carol)
		f.On("GET /roles", 403, ``)
		rec := serveRequest("POST", save, userSaveForm(url.Values{"username": {"carol"}}))
		assertStatusCode(t, rec, 200)
		assertBodyHas(t, rec, `id="admin-user-status" hx-swap-oob="innerHTML">Saved<`)
		assertBodyLacks(t, rec, "roles_editable")
	})

	t.Run("US-79_a_refusal_of_a_changed_username_shows_the_message_and_restores_the_field", func(t *testing.T) {
		for _, status := range []int{400, 409, 422} {
			t.Run("US-79_"+strconv.Itoa(status), func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.Problem("PUT /users/u1", status, "Username taken")
				rec := serveRequest("POST", save, userSaveForm(url.Values{"username": {" Carol "}}))
				assertFailure(t, rec, status, "Username taken")
				assertBodyHas(t, rec, "admin-user-username-error", `value=" Carol "`, `id="admin-user-status" hx-swap-oob="innerHTML"></p>`)
				f.AssertLines(t, put+`{"username":"Carol"}`)
			})
		}
	})

	t.Run("US-80_a_refusal_of_a_roles-only_change_does_not_touch_the_username_field", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.Problem("PUT /users/u1", 409, "Role not allowed")
		rec := serveRequest("POST", save, userSaveForm(url.Values{"roles": {"r2"}}))
		assertFailure(t, rec, 409, "Role not allowed")
		assertBodyLacks(t, rec, "admin-user-username")
		f.AssertLines(t, put+`{"roles":["r2"]}`)
	})

	t.Run("US-81_any_other_API_failure_shows_only_the_message_and_restores_nothing", func(t *testing.T) {
		cases := []struct {
			name    string
			status  int
			detail  string
			message string
		}{
			{"403", 403, "Missing permission", "Missing permission"},
			{"404", 404, "User not found", "User not found"},
			{"500_with_a_detail", 500, "database down", "database down"},
			{"500_without_a_detail", 500, "", "Failed to save the user"},
		}
		for _, tc := range cases {
			t.Run("US-81_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.Refuse("PUT /users/u1", tc.status, tc.detail)
				rec := serveRequest("POST", save, userSaveForm(url.Values{"username": {"carol"}}))
				assertFailure(t, rec, tc.status, tc.message)
				assertBodyLacks(t, rec, "admin-user-username")
				f.AssertLines(t, put+`{"username":"carol"}`)
			})
		}
	})

	t.Run("US-82_a_401_on_the_write_sends_the_browser_to_the_login_page", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("PUT /users/u1", 401, ``)
		assertLoginRedirect(t, serveRequest("POST", save, userSaveForm(url.Values{"username": {"carol"}})))
		f.AssertLines(t, put+`{"username":"carol"}`)
	})

	t.Run("US-83_a_failed_reload_after_a_write_reports_it_and_restores_the_header_and_loaded_values", func(t *testing.T) {
		cases := []struct {
			name    string
			status  int
			detail  string
			body    string
			want    int
			message string
		}{
			{"500_with_a_detail", 500, "database down", "", 500, "database down"},
			{"500_without_a_detail", 500, "", "", 500, "Failed to load roles"},
			{"200_not_json", 200, "", "not json", 502, "Failed to load roles"},
		}
		for _, tc := range cases {
			t.Run("US-83_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				stubUserEditor(f)
				f.On("PUT /users/u1", 200, userJSON("u1", "carol", "r1", "r2"))
				if tc.body != "" {
					f.On("GET /roles", tc.status, tc.body)
				} else {
					f.Refuse("GET /roles", tc.status, tc.detail)
				}
				rec := serveRequest("POST", save, userSaveForm(url.Values{"username": {"carol"}}))
				assertFailure(t, rec, tc.want, savedPrefix+tc.message)
				assertBodyHas(t, rec, "carol", "u1", `name="loaded_username" value="carol"`, `name="loaded_roles" value="r1"`, `name="loaded_roles" value="r2"`)
				assertOutOfBand(t, rec, "admin-user-header", true)
				assertOutOfBand(t, rec, "admin-user-loaded", true)
				f.AssertLines(t, put+`{"username":"carol"}`, "GET /roles")
			})
		}
	})

	t.Run("US-84_a_401_on_the_reload_after_a_write_sends_the_browser_to_the_login_page", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		stubUserEditor(f)
		f.On("PUT /users/u1", 200, carol)
		f.On("GET /roles", 401, ``)
		rec := serveRequest("POST", save, userSaveForm(url.Values{"username": {"carol"}}))
		assertLoginRedirect(t, rec)
		assertBodyLacks(t, rec, "The change was made")
	})
}

func TestAdminUserFailuresClearTheStatusLine(t *testing.T) {
	cases := []struct {
		name, method, target string
		form                 url.Values
		answered             string
		want                 string
	}{
		{"save", "POST", "/admin/users/u1", url.Values{"loaded_username": {"bob"}, "username": {"bob"}, "roles_editable": {"1"}, "loaded_roles": {"r1"}, "roles": {"r1", "r2"}}, "PUT /users/u1", `refused<p id="admin-user-status" hx-swap-oob="innerHTML"></p>`},
		{"editor", "GET", "/admin/users/u1/editor", nil, "GET /users/u1", "refused"},
		{"list", "GET", "/admin/users/list", nil, "GET /users", "refused"},
		{"rows", "GET", "/admin/users/rows", nil, "GET /users", "refused"},
	}
	for _, tc := range cases {
		t.Run("US-85_"+tc.name, func(t *testing.T) {
			f := fakeapi.NewFakeAPI(t)
			f.Problem(tc.answered, 409, "refused")
			rec := serveRequest(tc.method, tc.target, tc.form)
			assertStatusCode(t, rec, 409)
			if got := rec.Body.String(); got != tc.want {
				t.Errorf("body = %q, want %q", got, tc.want)
			}
		})
	}
}
