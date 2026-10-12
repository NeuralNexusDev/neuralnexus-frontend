package components

import (
	"encoding/json"
	"html"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/p0t4t0sandwich/neuralnexus-frontend/test/testutil"
)

var markupPattern = regexp.MustCompile(`<[^>]*>`)

var (
	oobName   = map[bool]string{true: "oob", false: "in_place"}
	focusName = map[bool]string{true: "focused", false: "not_focused"}
)

var catalogueC = []Permission{
	{ID: "1", Node: "pets.read", Description: "Read pets"},
	{ID: "2", Node: "pets.max", Description: "Max pets", ValueType: "int", Merge: "max"},
	{ID: "3", Node: "pets.tags", Description: "Pet tags", ValueType: "string_list", Merge: "union"},
	{ID: "4", Node: "pets.motto", Description: "Pet motto", ValueType: "string", Merge: "first"},
}

func grantOf(id, value string) RolePermission {
	for _, permission := range catalogueC {
		if permission.ID == id {
			grant := RolePermission{Permission: permission}
			if value != "" {
				grant.Value = json.RawMessage(value)
			}
			return grant
		}
	}
	panic("grantOf: no permission " + id)
}

func grantedIDs(ids ...string) []RolePermission {
	var grants []RolePermission
	for _, id := range ids {
		grants = append(grants, grantOf(id, ""))
	}
	return grants
}

func permissionIDs(permissions []Permission) []string {
	ids := []string{}
	for _, permission := range permissions {
		ids = append(ids, permission.ID)
	}
	return ids
}

func visibleText(page string) string {
	return html.UnescapeString(markupPattern.ReplaceAllString(page, ""))
}

func assertTagsEndCleanly(t testing.TB, tags []string) {
	t.Helper()
	for _, tag := range tags {
		if !strings.HasSuffix(tag, ">") || strings.Count(tag, "<") != 1 {
			t.Errorf("tag %q does not end at its own > or holds a raw <", tag)
		}
	}
}

func inputByName(t testing.TB, page, name string) string {
	t.Helper()
	for _, element := range []string{"input", "textarea", "select"} {
		for _, tag := range openTags(page, element) {
			if got, _ := tagAttr(tag, "name"); got == name {
				return tag
			}
		}
	}
	t.Errorf("no field named %q in:\n%s", name, page)
	return ""
}

func attrOfTag(t testing.TB, tag, name, want string) {
	t.Helper()
	if got, ok := tagAttr(tag, name); !ok || got != want {
		t.Errorf("%s = %q (present %t), want %q; tag %s", name, got, ok, want, tag)
	}
}

func TestAdminDashboardLink(t *testing.T) {
	t.Run("CA-039_the_link_to_the_dashboard_has_the_id_and_the_target_the_account_page_and_the_browser_tests_use", func(t *testing.T) {
		out := renderString(t, AdminDashboardLink())
		assertOnce(t, out, "admin-dashboard-link")
		assertElement(t, out, "admin-dashboard-link", "a")
		assertAttr(t, out, "admin-dashboard-link", "href", "/admin")
		assertText(t, out, "admin-dashboard-link", "Admin dashboard")
		if n := strings.Count(out, "<a "); n != 1 {
			t.Errorf("found %d links, want 1", n)
		}
	})
}

func TestAdminTextBlock(t *testing.T) {
	t.Run("CA-040_the_name_and_the_description_are_rendered_in_that_order", func(t *testing.T) {
		out := renderString(t, adminTextBlock("admins", "Site admins"))
		assertInOrder(t, out, ">admins</span>", ">Site admins</span>")
	})

	t.Run("CA-041_typed_text_is_escaped_in_both_spans", func(t *testing.T) {
		name, description := "<i>a</i> & \"b\"", "<b>x</b> 'y'"
		out := renderString(t, adminTextBlock(name, description))
		assertContains(t, out, "&lt;i&gt;a&lt;/i&gt;", "&lt;b&gt;x&lt;/b&gt;")
		assertLacks(t, out, "<i>", "<b>")
		spans := regexp.MustCompile(`<span class="[^"]*block[^"]*">([^<]*)</span>`).FindAllStringSubmatch(out, -1)
		if len(spans) != 2 || html.UnescapeString(spans[0][1]) != name || html.UnescapeString(spans[1][1]) != description {
			t.Errorf("spans = %q, want the two inputs", spans)
		}
	})
}

func TestAdminCard(t *testing.T) {
	t.Run("CA-042_the_card_renders_an_anchor_with_the_id_href_title_and_description_it_is_given", func(t *testing.T) {
		out := renderString(t, adminCard("c1", "/x", "Title", "Text"))
		assertElement(t, out, "c1", "a")
		assertAttr(t, out, "c1", "href", "/x")
		assertContains(t, out, ">Title</span>", ">Text</span>")
	})
}

func TestAdminCards(t *testing.T) {
	t.Run("CA-044_an_account_with_only_the_users_permission_sees_only_the_users_card", func(t *testing.T) {
		out := renderString(t, AdminCards(AdminDashboardData{Users: true}))
		assertAttr(t, out, "admin-users-link", "href", "/admin/users")
		assertLacks(t, out, `id="admin-roles-link"`, `id="admin-permissions-link"`, `id="admin-denied"`)
	})

	t.Run("CA-045_an_account_with_only_the_roles_permission_sees_the_roles_and_permissions_cards", func(t *testing.T) {
		out := renderString(t, AdminCards(AdminDashboardData{Roles: true}))
		assertAttr(t, out, "admin-roles-link", "href", "/admin/roles")
		assertAttr(t, out, "admin-permissions-link", "href", "/admin/permissions")
		assertLacks(t, out, `id="admin-users-link"`, `id="admin-denied"`)
	})

	t.Run("CA-046_an_account_with_both_permissions_sees_all_three_cards_in_order", func(t *testing.T) {
		out := renderString(t, AdminCards(AdminDashboardData{Users: true, Roles: true}))
		assertInOrder(t, out, `id="admin-users-link"`, `id="admin-roles-link"`, `id="admin-permissions-link"`)
		assertLacks(t, out, `id="admin-denied"`)
	})

	t.Run("CA-047_an_account_with_neither_permission_sees_only_the_denied_note", func(t *testing.T) {
		out := renderString(t, AdminCards(AdminDashboardData{}))
		assertText(t, out, "admin-denied", "Your account has no admin permissions")
		assertLacks(t, out, `id="admin-users-link"`, `id="admin-roles-link"`, `id="admin-permissions-link"`)
	})
}

func TestAdminDashboardPage(t *testing.T) {
	t.Run("CA-048_the_page_is_a_full_document_titled_for_the_admin_pages_and_loads_the_cards_from_the_cards_route", func(t *testing.T) {
		out := renderString(t, AdminDashboardPage())
		assertContains(t, out, "<title>Admin - NeuralNexus</title>", `hx-get="/admin/cards"`, ">Admin Dashboard</h1>")
		assertLacks(t, out, "data-status")
	})
}

func TestAdminUsersList(t *testing.T) {
	t.Run("CA-050_the_search_input_sends_its_text_to_the_rows_route_as_the_admin_types", func(t *testing.T) {
		out := renderString(t, AdminUsersList(AdminUsersData{}))
		for name, want := range map[string]string{
			"type": "search", "name": "search", "hx-get": "/admin/users/rows", "hx-trigger": "input changed delay:250ms, search",
			"hx-target": "#admin-users", "hx-swap": "innerHTML", "hx-sync": "this:replace", "autocomplete": "off",
		} {
			assertAttr(t, out, "admin-users-search", name, want)
		}
	})

	t.Run("CA-052_the_first_page_of_users_is_rendered_inside_the_list_the_search_swaps", func(t *testing.T) {
		d := AdminUsersData{Users: []UserAccount{{UserID: "u1", Username: "Bob"}, {UserID: "u2", Username: "Carol"}}, NextOffset: 2}
		out := renderString(t, AdminUsersList(d))
		assertElement(t, out, "admin-users", "ul")
		list, _ := innerHTML(out, "admin-users")
		assertContains(t, list, `href="/admin/users/u1"`, `href="/admin/users/u2"`)
		if all, inList := strings.Count(out, "<li"), strings.Count(list, "<li"); all != inList {
			t.Errorf("%d rows sit outside the list", all-inList)
		}
	})
}

func TestAdminUsersCount(t *testing.T) {
	t.Run("CA-054_the_count_line_shows_the_summary_of_the_data_and_passes_the_out_of_band_flag_on", func(t *testing.T) {
		d := AdminUsersData{Users: []UserAccount{{UserID: "u1"}, {UserID: "u2"}, {UserID: "u3"}}, NextOffset: 3}
		for _, oob := range []bool{true, false} {
			t.Run("CA-054_"+oobName[oob], func(t *testing.T) {
				out := renderString(t, AdminUsersCount(d, oob))
				assertText(t, out, "admin-users-count", "3 users")
				if oob {
					assertAttr(t, out, "admin-users-count", "hx-swap-oob", "innerHTML")
				} else {
					assertFlag(t, out, "admin-users-count", "hx-swap-oob", false)
				}
			})
		}
	})
}

func TestUsersCountLine(t *testing.T) {
	t.Run("CA-055_without_the_out_of_band_flag_the_line_is_an_empty_live_region_with_no_swap_attribute", func(t *testing.T) {
		out := renderString(t, usersCountLine("", false))
		assertAttr(t, out, "admin-users-count", "role", "status")
		assertFlag(t, out, "admin-users-count", "hx-swap-oob", false)
		assertText(t, out, "admin-users-count", "")
	})

	t.Run("CA-056_with_the_flag_the_line_carries_the_swap_attribute_and_the_text", func(t *testing.T) {
		out := renderString(t, usersCountLine("2 users", true))
		assertAttr(t, out, "admin-users-count", "hx-swap-oob", "innerHTML")
		assertAttr(t, out, "admin-users-count", "role", "status")
		assertText(t, out, "admin-users-count", "2 users")
	})
}

func TestAdminUserRows(t *testing.T) {
	three := []UserAccount{{UserID: "u1", Username: "A"}, {UserID: "u2", Username: "B"}, {UserID: "u3", Username: "C"}}

	t.Run("CA-066_a_user_without_a_username_is_shown_as_No_username_with_the_ID", func(t *testing.T) {
		out := renderString(t, AdminUserRows(AdminUsersData{Users: []UserAccount{{UserID: "u3"}}}))
		assertContains(t, out, "No username", ">u3</span>")
	})

	t.Run("CA-067_the_user_ID_is_path_escaped_in_the_link", func(t *testing.T) {
		out := renderString(t, AdminUserRows(AdminUsersData{Users: []UserAccount{{UserID: "a/b", Username: "x"}}}))
		attrOfTag(t, openTags(out, "a")[0], "href", "/admin/users/a%2Fb")
	})

	t.Run("CA-069_only_the_first_rows_link_takes_focus_and_only_when_FocusFirst_is_set", func(t *testing.T) {
		for _, focus := range []bool{true, false} {
			t.Run("CA-069_"+focusName[focus], func(t *testing.T) {
				links := openTags(renderString(t, AdminUserRows(AdminUsersData{Users: three, FocusFirst: focus})), "a")
				if len(links) != 3 {
					t.Fatalf("found %d links, want 3", len(links))
				}
				for i, link := range links {
					if _, ok := tagAttr(link, "autofocus"); ok != (focus && i == 0) {
						t.Errorf("link %d autofocus = %t, want %t", i, ok, focus && i == 0)
					}
				}
			})
		}
	})

	t.Run("CA-070_a_page_past_the_end_shows_the_end_note_which_takes_focus_when_asked", func(t *testing.T) {
		for _, focus := range []bool{true, false} {
			t.Run("CA-070_"+focusName[focus], func(t *testing.T) {
				out := renderString(t, AdminUserRows(AdminUsersData{Start: 400, FocusFirst: focus}))
				assertAttr(t, out, "admin-users-end", "tabindex", "-1")
				assertText(t, out, "admin-users-end", "No more users")
				assertFlag(t, out, "admin-users-end", "autofocus", focus)
			})
		}
	})

	t.Run("CA-071_the_end_note_is_shown_only_for_an_empty_later_page_with_nothing_more_to_load", func(t *testing.T) {
		cases := []struct {
			name string
			data AdminUsersData
		}{
			{"users_present", AdminUsersData{Users: three, Start: 400}},
			{"first_page", AdminUsersData{}},
			{"more_to_load", AdminUsersData{Start: 400, More: true, NextOffset: 600}},
		}
		for _, tc := range cases {
			t.Run("CA-071_"+tc.name, func(t *testing.T) {
				assertLacks(t, renderString(t, AdminUserRows(tc.data)), `id="admin-users-end"`)
			})
		}
	})

	t.Run("CA-072_an_empty_first_page_shows_the_empty_row_worded_by_whether_the_search_stopped_early", func(t *testing.T) {
		cases := []struct {
			name string
			data AdminUsersData
			want string
		}{
			{"nothing_found", AdminUsersData{}, "No users found"},
			{"search_stopped_early", AdminUsersData{More: true, NextOffset: 1000}, "No matches in the first 1000 users"},
		}
		for _, tc := range cases {
			t.Run("CA-072_"+tc.name, func(t *testing.T) {
				assertText(t, renderString(t, AdminUserRows(tc.data)), "admin-users-empty", tc.want)
			})
		}
	})

	t.Run("CA-073_more_users_render_a_Load_more_button_that_requests_the_next_page_into_its_own_place", func(t *testing.T) {
		out := renderString(t, AdminUserRows(AdminUsersData{Users: three[:2], More: true, NextOffset: 400, Search: "bob"}))
		row, _ := innerHTML(out, "admin-users-more")
		assertContains(t, row, `id="admin-users-more-button"`)
		for name, want := range map[string]string{
			"type": "button", "hx-get": "/admin/users/rows?offset=400&search=bob", "hx-target": "#admin-users-more", "hx-swap": "outerHTML", "hx-sync": "this:drop",
		} {
			assertAttr(t, out, "admin-users-more-button", name, want)
		}
		assertText(t, out, "admin-users-more-button", "Load more")
	})

	t.Run("CA-200_the_Load_more_path_carries_the_offset_and_adds_a_search_after_it", func(t *testing.T) {
		cases := []struct {
			name   string
			offset int
			search string
			want   string
		}{
			{"offset_0", 0, "", "/admin/users/rows?offset=0"},
			{"offset_200", 200, "", "/admin/users/rows?offset=200"},
			{"search", 400, "bob", "/admin/users/rows?offset=400&search=bob"},
		}
		for _, tc := range cases {
			t.Run("CA-200_"+tc.name, func(t *testing.T) {
				out := renderString(t, AdminUserRows(AdminUsersData{More: true, NextOffset: tc.offset, Search: tc.search}))
				assertAttr(t, out, "admin-users-more-button", "hx-get", tc.want)
			})
		}
	})

	t.Run("CA-201_a_search_with_reserved_characters_is_query_escaped_in_the_Load_more_path", func(t *testing.T) {
		out := renderString(t, AdminUserRows(AdminUsersData{More: true, NextOffset: 200, Search: "a b&c=d#é"}))
		assertAttr(t, out, "admin-users-more-button", "hx-get", "/admin/users/rows?offset=200&search=a+b%26c%3Dd%23%C3%A9")
	})

	t.Run("CA-202_a_role_in_the_name_map_is_shown_by_its_name", func(t *testing.T) {
		out := renderString(t, AdminUserRows(AdminUsersData{Users: []UserAccount{{UserID: "u1", Username: "A", Roles: []string{"r1"}}}, RoleNames: map[string]string{"r1": "Moderator"}}))
		assertContains(t, out, ">Moderator</span>")
		assertLacks(t, out, ">r1</span>")
	})

	t.Run("CA-203_a_role_missing_from_the_map_or_a_nil_map_is_shown_by_its_ID", func(t *testing.T) {
		cases := []struct {
			name  string
			names map[string]string
			id    string
		}{
			{"missing_role", map[string]string{"r1": "Moderator"}, "r2"},
			{"nil_map", nil, "r1"},
		}
		for _, tc := range cases {
			t.Run("CA-203_"+tc.name, func(t *testing.T) {
				out := renderString(t, AdminUserRows(AdminUsersData{Users: []UserAccount{{UserID: "u1", Username: "A", Roles: []string{tc.id}}}, RoleNames: tc.names}))
				assertContains(t, out, ">"+tc.id+"</span>")
				assertLacks(t, out, "Moderator")
			})
		}
	})

	t.Run("CA-074_without_more_users_there_is_no_Load_more_row", func(t *testing.T) {
		assertLacks(t, renderString(t, AdminUserRows(AdminUsersData{Users: three[:2]})), `id="admin-users-more"`)
	})

	t.Run("CA-075_the_Load_more_row_says_how_far_a_search_looked_and_only_for_a_search", func(t *testing.T) {
		for name, search := range map[string]string{"with_a_search": "bob", "without_a_search": ""} {
			t.Run("CA-075_"+name, func(t *testing.T) {
				out := renderString(t, AdminUserRows(AdminUsersData{More: true, NextOffset: 1000, Search: search}))
				row, _ := innerHTML(out, "admin-users-more")
				if got := strings.Contains(row, "Searched the first 1000 users"); got != (search != "") {
					t.Errorf("row mentions the search depth = %t, want %t", got, search != "")
				}
			})
		}
	})

	t.Run("CA-076_the_Load_more_button_takes_focus_only_when_it_is_the_sole_new_content", func(t *testing.T) {
		cases := []struct {
			name  string
			users []UserAccount
			focus bool
			want  bool
		}{
			{"no_users_focus_first", nil, true, true},
			{"users_focus_first", three[:2], true, false},
			{"no_users_no_focus", nil, false, false},
		}
		for _, tc := range cases {
			t.Run("CA-076_"+tc.name, func(t *testing.T) {
				out := renderString(t, AdminUserRows(AdminUsersData{Users: tc.users, More: true, NextOffset: 200, FocusFirst: tc.focus}))
				assertFlag(t, out, "admin-users-more-button", "autofocus", tc.want)
			})
		}
	})

	t.Run("CA-077_typed_names_are_escaped_in_text_and_in_the_search_path", func(t *testing.T) {
		username := "<i>a</i> \"b\" & 'c'"
		d := AdminUsersData{
			Users:     []UserAccount{{UserID: "<u>", Username: username, Roles: []string{"r1"}}},
			RoleNames: map[string]string{"r1": "<b>x</b>"}, More: true, Search: `"><i>`, NextOffset: 200,
		}
		out := renderString(t, AdminUserRows(d))
		assertLacks(t, out, "<i>", "<b>", "<u>")
		assertContains(t, visibleText(out), username, "<b>x</b>", "<u>")
		attrOfTag(t, testutil.TagByID(out, "admin-users-more-button"), "hx-get", "/admin/users/rows?offset=200&search=%22%3E%3Ci%3E")
		link := openTags(out, "a")[0]
		if href, _ := tagAttr(link, "href"); strings.ContainsAny(href, `<"`) {
			t.Errorf("href %q holds a raw < or \"", href)
		}
		assertTagsEndCleanly(t, []string{link})
	})
}

func TestAdminUsersPage(t *testing.T) {
	t.Run("CA-078_the_page_is_a_full_document_with_its_title_that_loads_the_list_links_back_to_the_dashboard_and_holds_an_empty_count_line", func(t *testing.T) {
		out := renderString(t, AdminUsersPage())
		assertContains(t, out, "<title>Users - NeuralNexus</title>", `hx-get="/admin/users/list"`, ">Users</h1>")
		if !regexp.MustCompile(`<a href="/admin"[^>]*>Back to the admin dashboard</a>`).MatchString(out) {
			t.Errorf("no link to /admin reading Back to the admin dashboard in:\n%s", out)
		}
		assertAttr(t, out, "admin-users-count", "role", "status")
		assertText(t, out, "admin-users-count", "")
		assertFlag(t, out, "admin-users-count", "hx-swap-oob", false)
	})
}

var adminUserFixture = AdminUserData{
	User:          UserAccount{UserID: "u1", Username: "Bob", Roles: []string{"r1"}},
	Roles:         []Role{{ID: "r1", Name: "Moderator", Description: "Moderates"}, {ID: "r2", Name: "Admin", Description: "Administers"}},
	RolesReadable: true,
	Links:         []LinkedAccount{{Platform: "discord", PlatformUsername: "bob#1"}},
	Permissions:   []string{"pets.read"},
}

func TestAdminUserPage(t *testing.T) {
	t.Run("CA-081_the_page_is_a_full_document_with_its_title_that_loads_the_editor_for_the_ID_and_holds_the_status_line_the_editor_names", func(t *testing.T) {
		out := renderString(t, AdminUserPage("u1"))
		assertContains(t, out, "<title>Edit user - NeuralNexus</title>")
		tag := regexp.MustCompile(`<div[^>]*hx-get="/admin/users/u1/editor"[^>]*>`).FindString(out)
		attrOfTag(t, tag, "hx-trigger", "load")
		assertAttr(t, out, "admin-user-status", "role", "status")
		if !regexp.MustCompile(`<a href="/admin/users"[^>]*>Back to users</a>`).MatchString(out) {
			t.Errorf("no link to /admin/users reading Back to users in:\n%s", out)
		}
	})
}

func TestAdminUserContent(t *testing.T) {
	t.Run("CA-082_the_editor_container_names_its_status_line_and_marks_itself_as_a_one_request_region", func(t *testing.T) {
		out := renderString(t, AdminUserContent(adminUserFixture))
		assertAttr(t, out, "admin-user", "data-status", "admin-user-status")
		assertFlag(t, out, "admin-user", "data-busy-region", true)
		assertAttr(t, out, "admin-user", "hx-sync:inherited", "this:drop")
	})

	t.Run("CA-083_the_editor_holds_the_header_the_form_the_linked_accounts_and_the_permissions_none_of_them_out_of_band", func(t *testing.T) {
		out := renderString(t, AdminUserContent(adminUserFixture))
		for _, id := range []string{"admin-user-header", "admin-user-form", "admin-user-links", "admin-user-permissions-section"} {
			assertOnce(t, out, id)
		}
		assertLacks(t, out, "hx-swap-oob")
	})
}

func TestAdminUserUsername(t *testing.T) {
	t.Run("CA-084_the_username_field_is_a_labelled_text_input_with_the_current_value_and_no_error", func(t *testing.T) {
		out := renderString(t, AdminUserUsername("Bob", "", false))
		assertOnce(t, out, "admin-user-username-field")
		assertContains(t, out, `for="admin-user-username"`, ">Username</label>")
		tag := testutil.TagByID(out, "admin-user-username")
		for name, want := range map[string]string{"name": "username", "type": "text", "value": "Bob", "autocomplete": "off"} {
			attrOfTag(t, tag, name, want)
		}
		for _, name := range []string{"required", "aria-invalid", "aria-describedby"} {
			if _, ok := tagAttr(tag, name); ok {
				t.Errorf("tag %s holds %s", tag, name)
			}
		}
		assertLacks(t, out, `id="admin-user-username-error"`)
	})

	t.Run("CA-085_a_refusal_shows_the_message_and_marks_the_input_invalid_and_described_by_it", func(t *testing.T) {
		out := renderString(t, AdminUserUsername("", "Enter a username", false))
		assertText(t, out, "admin-user-username-error", "Enter a username")
		assertAttr(t, out, "admin-user-username", "aria-invalid", "true")
		assertAttr(t, out, "admin-user-username", "aria-describedby", "admin-user-username-error")
		assertFlag(t, out, "admin-user-username", "autofocus", true)
		assertFlag(t, out, "admin-user-username", "value", false)
	})
}

func TestAdminUserHeader(t *testing.T) {
	t.Run("CA-088_the_header_shows_the_username_and_the_ID_in_their_own_elements", func(t *testing.T) {
		out := renderString(t, AdminUserHeader(AdminUserData{User: UserAccount{UserID: "u1", Username: "Bob"}}, false))
		assertText(t, out, "admin-user-title", "Bob")
		assertText(t, out, "admin-user-id", "u1")
		assertFlag(t, out, "admin-user-header", "hx-swap-oob", false)
	})
}

func TestAdminUserLoaded(t *testing.T) {
	t.Run("CA-092_the_loaded_values_are_hidden_inputs_the_save_handler_compares_against", func(t *testing.T) {
		out := renderString(t, AdminUserLoaded(AdminUserData{User: UserAccount{Username: "Bob", Roles: []string{"r1", "r2"}}}, false))
		assertFlag(t, out, "admin-user-loaded", "hidden", true)
		assertFlag(t, out, "admin-user-loaded", "hx-swap-oob", false)
		inputs := openTags(out, "input")
		if len(inputs) != 3 {
			t.Fatalf("found %d inputs, want 3", len(inputs))
		}
		for i, want := range []struct{ name, value string }{{"loaded_username", "Bob"}, {"loaded_roles", "r1"}, {"loaded_roles", "r2"}} {
			attrOfTag(t, inputs[i], "type", "hidden")
			attrOfTag(t, inputs[i], "name", want.name)
			attrOfTag(t, inputs[i], "value", want.value)
		}
	})

	t.Run("CA-093_a_user_without_roles_has_no_loaded_role_input_and_an_empty_username_is_sent_as_an_empty_value", func(t *testing.T) {
		out := renderString(t, AdminUserLoaded(AdminUserData{User: UserAccount{Username: ""}}, false))
		inputs := openTags(out, "input")
		if len(inputs) != 1 {
			t.Fatalf("found %d inputs, want 1", len(inputs))
		}
		attrOfTag(t, inputs[0], "name", "loaded_username")
		if value, _ := tagAttr(inputs[0], "value"); value != "" {
			t.Errorf("value = %q, want empty", value)
		}
		assertLacks(t, out, "loaded_roles")
	})

	t.Run("CA-094_the_out_of_band_copy_marks_the_element_for_swapping", func(t *testing.T) {
		for _, oob := range []bool{true, false} {
			t.Run("CA-094_"+oobName[oob], func(t *testing.T) {
				out := renderString(t, AdminUserLoaded(adminUserFixture, oob))
				if oob {
					assertAttr(t, out, "admin-user-loaded", "hx-swap-oob", "true")
				} else {
					assertFlag(t, out, "admin-user-loaded", "hx-swap-oob", false)
				}
			})
		}
	})

	t.Run("CA-095_the_loaded_values_are_escaped_in_the_attributes", func(t *testing.T) {
		username, role := "\"><script>x</script> & 'y'", `"><b>`
		out := renderString(t, AdminUserLoaded(AdminUserData{User: UserAccount{Username: username, Roles: []string{role}}}, false))
		assertLacks(t, out, "<script>", "<b>")
		inputs := openTags(out, "input")
		if len(inputs) != 2 {
			t.Fatalf("found %d inputs, want 2", len(inputs))
		}
		attrOfTag(t, inputs[0], "value", username)
		attrOfTag(t, inputs[1], "value", role)
		assertTagsEndCleanly(t, inputs)
	})
}

func TestAdminUserForm(t *testing.T) {
	t.Run("CA-096_the_form_posts_to_the_users_route_and_swaps_itself_with_the_answer", func(t *testing.T) {
		out := renderString(t, AdminUserForm(adminUserFixture))
		assertAttr(t, out, "admin-user-form", "hx-post", "/admin/users/u1")
		assertAttr(t, out, "admin-user-form", "hx-target", "#admin-user-form")
		assertAttr(t, out, "admin-user-form", "hx-swap", "outerHTML")
		assertAttr(t, out, "admin-user-save", "type", "submit")
		assertText(t, out, "admin-user-save", "Save")
	})

	t.Run("CA-097_the_user_ID_is_path_escaped_in_hx_post", func(t *testing.T) {
		for id, want := range map[string]string{"a/b": "/admin/users/a%2Fb", "a b": "/admin/users/a%20b"} {
			t.Run("CA-097_"+strings.ReplaceAll(id, "/", "slash_"), func(t *testing.T) {
				d := adminUserFixture
				d.User.UserID = id
				assertAttr(t, renderString(t, AdminUserForm(d)), "admin-user-form", "hx-post", want)
			})
		}
	})

	t.Run("CA-098_the_form_carries_the_loaded_values_and_the_current_username_in_its_field", func(t *testing.T) {
		out := renderString(t, AdminUserForm(adminUserFixture))
		form, _ := innerHTML(out, "admin-user-form")
		loaded, _ := innerHTML(form, "admin-user-loaded")
		inputs := openTags(loaded, "input")
		if len(inputs) != 2 {
			t.Fatalf("the loaded values hold %d inputs, want 2", len(inputs))
		}
		attrOfTag(t, inputs[0], "name", "loaded_username")
		attrOfTag(t, inputs[0], "value", "Bob")
		attrOfTag(t, inputs[1], "name", "loaded_roles")
		attrOfTag(t, inputs[1], "value", "r1")
		assertFlag(t, out, "admin-user-loaded", "hx-swap-oob", false)
		assertAttr(t, out, "admin-user-username", "value", "Bob")
		assertLacks(t, out, `id="admin-user-username-error"`)
	})

	t.Run("CA-102_the_roles_sit_in_a_fieldset_with_a_legend", func(t *testing.T) {
		out := renderString(t, AdminUserForm(adminUserFixture))
		assertInOrder(t, out, "<fieldset", "<legend", ">Roles</legend>", `id="admin-user-roles"`, "</fieldset>")
	})

	t.Run("CA-103_role_names_descriptions_and_IDs_are_escaped", func(t *testing.T) {
		d := AdminUserData{
			User:          UserAccount{UserID: "u1", Roles: []string{`r9"><b>`}},
			Roles:         []Role{{ID: `"><b>1`, Name: "<i>n</i>", Description: "<u>d</u> & 'x'"}},
			RolesReadable: true,
		}
		out := renderString(t, AdminUserForm(d))
		assertLacks(t, out, "<i>", "<u>", "<b>")
		attrOfTag(t, inputByName(t, out, "roles"), "value", `"><b>1`)
		attrOfTag(t, inputByName(t, out, "kept_roles"), "value", `r9"><b>`)
		assertContains(t, visibleText(out), "<i>n</i>", "<u>d</u> & 'x'")
		assertTagsEndCleanly(t, openTags(out, "input"))
	})
}

func TestAdminUserLinks(t *testing.T) {
	t.Run("CA-104_each_linked_account_shows_its_platform_and_its_name", func(t *testing.T) {
		d := AdminUserData{Links: []LinkedAccount{{Platform: "discord", PlatformUsername: "bob#1"}, {Platform: "twitch", PlatformID: "42"}}}
		out := renderString(t, adminUserLinks(d))
		assertContains(t, out, ">Linked accounts</h2>")
		list, _ := innerHTML(out, "admin-user-links")
		if n := strings.Count(list, "<li"); n != 2 {
			t.Errorf("the list holds %d rows, want 2", n)
		}
		assertInOrder(t, list, ">discord</span>", ">bob#1</span>", ">twitch</span>", ">42</span>")
		assertLacks(t, out, `id="admin-user-links-error"`, `id="admin-user-links-empty"`)
	})

	t.Run("CA-204_a_linked_account_without_a_platform_username_is_named_by_its_platform_ID", func(t *testing.T) {
		cases := []struct {
			name string
			link LinkedAccount
			want string
		}{
			{"username", LinkedAccount{Platform: "discord", PlatformUsername: "bob#1", PlatformID: "123"}, ">bob#1</span>"},
			{"platform_id", LinkedAccount{Platform: "discord", PlatformID: "123"}, ">123</span>"},
		}
		for _, tc := range cases {
			t.Run("CA-204_"+tc.name, func(t *testing.T) {
				out := renderString(t, adminUserLinks(AdminUserData{Links: []LinkedAccount{tc.link}}))
				assertContains(t, out, tc.want)
			})
		}
		out := renderString(t, adminUserLinks(AdminUserData{Links: []LinkedAccount{{Platform: "discord"}}}))
		assertContains(t, out, `truncate"></span>`)
	})

	t.Run("CA-105_a_user_with_no_links_shows_the_empty_note", func(t *testing.T) {
		out := renderString(t, adminUserLinks(AdminUserData{}))
		assertText(t, out, "admin-user-links-empty", "No linked accounts")
		assertContains(t, out, ">Linked accounts</h2>")
	})

	t.Run("CA-106_a_load_failure_shows_its_message_as_an_alert_and_no_list_even_when_links_are_set", func(t *testing.T) {
		out := renderString(t, adminUserLinks(AdminUserData{LinksError: "Failed to load the linked accounts", Links: adminUserFixture.Links}))
		assertAttr(t, out, "admin-user-links-error", "role", "alert")
		assertText(t, out, "admin-user-links-error", "Failed to load the linked accounts")
		assertLacks(t, out, ` id="admin-user-links"`, `id="admin-user-links-empty"`)
	})

	t.Run("CA-107_platform_names_usernames_and_the_error_message_are_escaped", func(t *testing.T) {
		t.Run("CA-107_list", func(t *testing.T) {
			out := renderString(t, adminUserLinks(AdminUserData{Links: []LinkedAccount{{Platform: "<i>p</i>", PlatformUsername: "<b>u</b> & 'x'"}}}))
			assertLacks(t, out, "<i>", "<b>")
			assertContains(t, visibleText(out), "<i>p</i>", "<b>u</b> & 'x'")
		})
		t.Run("CA-107_error", func(t *testing.T) {
			out := renderString(t, adminUserLinks(AdminUserData{LinksError: "<u>e</u>"}))
			assertLacks(t, out, "<u>")
			assertContains(t, visibleText(out), "<u>e</u>")
		})
	})
}

func TestAdminUserPermissions(t *testing.T) {
	t.Run("CA-108_the_effective_permissions_show_as_a_list_of_chips_under_a_heading", func(t *testing.T) {
		out := renderString(t, AdminUserPermissions(AdminUserData{Permissions: []string{"pets.read", "pets.max:5"}}, false))
		assertFlag(t, out, "admin-user-permissions-section", "hx-swap-oob", false)
		assertContains(t, out, ">Effective permissions</h2>")
		list, _ := innerHTML(out, "admin-user-permissions")
		assertInOrder(t, list, ">pets.read</li>", ">pets.max:5</li>")
	})

	t.Run("CA-109_a_user_with_no_permissions_shows_the_empty_note", func(t *testing.T) {
		out := renderString(t, AdminUserPermissions(AdminUserData{}, false))
		assertText(t, out, "admin-user-permissions-empty", "No permissions")
		assertLacks(t, out, ` id="admin-user-permissions"`)
	})

	t.Run("CA-110_a_load_failure_shows_its_message_as_an_alert_and_no_list_even_when_permissions_are_set", func(t *testing.T) {
		out := renderString(t, AdminUserPermissions(AdminUserData{PermissionsErr: "Failed to load the permissions", Permissions: []string{"pets.read"}}, false))
		assertAttr(t, out, "admin-user-permissions-error", "role", "alert")
		assertText(t, out, "admin-user-permissions-error", "Failed to load the permissions")
		assertLacks(t, out, ` id="admin-user-permissions"`, `id="admin-user-permissions-empty"`)
	})

	t.Run("CA-111_the_out_of_band_copy_marks_the_section_for_swapping", func(t *testing.T) {
		for _, oob := range []bool{true, false} {
			t.Run("CA-111_"+oobName[oob], func(t *testing.T) {
				out := renderString(t, AdminUserPermissions(adminUserFixture, oob))
				if oob {
					assertAttr(t, out, "admin-user-permissions-section", "hx-swap-oob", "true")
				} else {
					assertFlag(t, out, "admin-user-permissions-section", "hx-swap-oob", false)
				}
			})
		}
	})

	t.Run("CA-112_permission_nodes_and_the_error_message_are_escaped", func(t *testing.T) {
		t.Run("CA-112_list", func(t *testing.T) {
			out := renderString(t, AdminUserPermissions(AdminUserData{Permissions: []string{"<b>x</b> & 'y'"}}, false))
			assertLacks(t, out, "<b>")
			assertContains(t, visibleText(out), "<b>x</b> & 'y'")
		})
		t.Run("CA-112_error", func(t *testing.T) {
			out := renderString(t, AdminUserPermissions(AdminUserData{PermissionsErr: "<u>e</u>"}, false))
			assertLacks(t, out, "<u>")
			assertContains(t, visibleText(out), "<u>e</u>")
		})
	})
}

func TestAdminRolesContent(t *testing.T) {
	t.Run("CA-115_the_role_ID_is_path_escaped_in_the_link", func(t *testing.T) {
		out := renderString(t, AdminRolesContent(AdminRolesData{Roles: []Role{{ID: "a/b", Name: "x"}}}))
		attrOfTag(t, openTags(out, "a")[0], "href", "/admin/roles/a%2Fb")
	})

	t.Run("CA-116_a_roles_permissions_show_as_chips_labelled_with_node_and_value_and_a_role_without_any_shows_none", func(t *testing.T) {
		roles := []Role{
			{ID: "5", Name: "admins", Permissions: []RolePermission{grantOf("1", ""), grantOf("2", "5")}},
			{ID: "6", Name: "guests"},
		}
		out := renderString(t, AdminRolesContent(AdminRolesData{Roles: roles}))
		list, _ := innerHTML(out, "admin-roles")
		rows := regexp.MustCompile(`(?s)<li>.*?</li>`).FindAllString(list, -1)
		if len(rows) != 2 {
			t.Fatalf("the list holds %d rows, want 2", len(rows))
		}
		chip := `<span class="` + chipClass + `">`
		assertContains(t, rows[0], chip+"pets.read</span>", chip+"pets.max: 5</span>")
		assertLacks(t, rows[1], chip)
	})

	t.Run("CA-117_with_no_roles_the_empty_note_is_shown_and_the_create_form_stays", func(t *testing.T) {
		out := renderString(t, AdminRolesContent(AdminRolesData{}))
		assertText(t, out, "admin-roles-empty", "No roles")
		assertText(t, out, "admin-roles", "")
		assertOnce(t, out, "admin-role-create-form")
	})

	t.Run("CA-118_the_create_form_posts_the_name_and_the_description_to_the_roles_route_and_drops_a_second_submit", func(t *testing.T) {
		out := renderString(t, AdminRolesContent(AdminRolesData{}))
		assertAttr(t, out, "admin-role-create-form", "hx-post", "/admin/roles")
		assertAttr(t, out, "admin-role-create-form", "hx-sync", "this:drop")
		assertAttr(t, out, "admin-role-create-name", "name", "name")
		assertAttr(t, out, "admin-role-create-description", "name", "description")
		assertAttr(t, out, "admin-role-create-submit", "type", "submit")
		assertText(t, out, "admin-role-create-submit", "Create role")
	})

	t.Run("CA-119_role_names_descriptions_and_permission_labels_are_escaped", func(t *testing.T) {
		role := Role{ID: "5", Name: "<i>n</i>", Description: "<b>d</b> & 'x'", Permissions: []RolePermission{{Permission: Permission{Node: "<u>p</u>"}}}}
		out := renderString(t, AdminRolesContent(AdminRolesData{Roles: []Role{role}}))
		assertLacks(t, out, "<i>", "<b>", "<u>")
		assertContains(t, visibleText(out), "<i>n</i>", "<b>d</b> & 'x'", "<u>p</u>")
	})
}

func TestAdminRolesPage(t *testing.T) {
	t.Run("CA-120_the_page_is_a_full_document_with_its_title_that_loads_the_list_and_links_to_the_dashboard_and_the_permissions_page", func(t *testing.T) {
		out := renderString(t, AdminRolesPage())
		assertContains(t, out, "<title>Roles - NeuralNexus</title>", ">Roles</h1>")
		tag := regexp.MustCompile(`<div[^>]*hx-get="/admin/roles/list"[^>]*>`).FindString(out)
		attrOfTag(t, tag, "hx-trigger", "load")
		for href, text := range map[string]string{"/admin": "Back to the admin dashboard", "/admin/permissions": "Permissions"} {
			if !regexp.MustCompile(`<a href="` + href + `"[^>]*>` + text + `</a>`).MatchString(out) {
				t.Errorf("no link to %s reading %s in:\n%s", href, text, out)
			}
		}
	})
}

func TestAdminValueField(t *testing.T) {
	t.Run("CA-123_an_int_permission_renders_a_whole_number_input", func(t *testing.T) {
		out := renderString(t, adminValueField(Permission{Node: "pets.max", ValueType: "int"}, "7", "grant_value"))
		inputs := openTags(out, "input")
		if len(inputs) != 1 {
			t.Fatalf("found %d inputs, want 1", len(inputs))
		}
		for name, want := range map[string]string{"type": "number", "step": "1", "name": "grant_value", "value": "7", "autocomplete": "off", "aria-label": "Value of pets.max"} {
			attrOfTag(t, inputs[0], name, want)
		}
	})

	t.Run("CA-124_a_string_permission_renders_a_text_input", func(t *testing.T) {
		out := renderString(t, adminValueField(Permission{Node: "pets.motto", ValueType: "string"}, "hi", "value_4"))
		inputs := openTags(out, "input")
		if len(inputs) != 1 {
			t.Fatalf("found %d inputs, want 1", len(inputs))
		}
		for name, want := range map[string]string{"type": "text", "name": "value_4", "value": "hi", "aria-label": "Value of pets.motto"} {
			attrOfTag(t, inputs[0], name, want)
		}
		if _, ok := tagAttr(inputs[0], "step"); ok {
			t.Errorf("tag %s holds step", inputs[0])
		}
	})

	t.Run("CA-125_a_string_list_permission_renders_a_textarea_with_one_item_per_line", func(t *testing.T) {
		out := renderString(t, adminValueField(Permission{Node: "pets.tags", ValueType: "string_list"}, "a\nb", "value_3"))
		areas := openTags(out, "textarea")
		if len(areas) != 1 {
			t.Fatalf("found %d textareas, want 1", len(areas))
		}
		for name, want := range map[string]string{"name": "value_3", "rows": "3", "placeholder": "One item per line", "aria-label": "Value of pets.tags"} {
			attrOfTag(t, areas[0], name, want)
		}
		assertContains(t, out, areas[0]+"a\nb</textarea>")
	})

	t.Run("CA-126_a_permission_without_a_value_type_or_an_unknown_type_renders_nothing", func(t *testing.T) {
		for name, valueType := range map[string]string{"no_type": "", "unknown_type": "bool"} {
			t.Run("CA-126_"+name, func(t *testing.T) {
				if out := renderString(t, adminValueField(Permission{Node: "pets.read", ValueType: valueType}, "x", "grant_value")); strings.TrimSpace(out) != "" {
					t.Errorf("output = %q, want empty", out)
				}
			})
		}
	})

	t.Run("CA-127_the_value_and_the_node_are_escaped_in_all_three_controls", func(t *testing.T) {
		value, node := `</textarea><i>x</i> "q" & 'y'`, "<b>n</b>"
		for _, valueType := range []string{"int", "string", "string_list"} {
			t.Run("CA-127_"+valueType, func(t *testing.T) {
				out := renderString(t, adminValueField(Permission{Node: node, ValueType: valueType}, value, "grant_value"))
				assertLacks(t, out, "<i>", "<b>")
				field := inputByName(t, out, "grant_value")
				attrOfTag(t, field, "aria-label", "Value of "+node)
				if valueType == "string_list" {
					if n := strings.Count(out, "</textarea>"); n != 1 {
						t.Errorf("found %d </textarea>, want 1", n)
					}
					content := out[strings.Index(out, field)+len(field) : strings.Index(out, "</textarea>")]
					if got := html.UnescapeString(content); got != value {
						t.Errorf("textarea content = %q, want %q", got, value)
					}
					return
				}
				attrOfTag(t, field, "value", value)
			})
		}
	})
}

var adminRoleFixture = AdminRoleData{
	Role:      Role{ID: "5", Name: "admins", Description: "Site admins", Permissions: []RolePermission{grantOf("1", "")}},
	Catalogue: catalogueC,
}

func TestAdminRolePage(t *testing.T) {
	t.Run("CA-132_the_page_is_a_full_document_with_its_title_that_loads_the_editor_for_the_ID_and_holds_the_status_line_the_editor_names", func(t *testing.T) {
		out := renderString(t, AdminRolePage("5"))
		assertContains(t, out, "<title>Edit role - NeuralNexus</title>")
		tag := regexp.MustCompile(`<div[^>]*hx-get="/admin/roles/5/editor"[^>]*>`).FindString(out)
		attrOfTag(t, tag, "hx-trigger", "load")
		assertAttr(t, out, "admin-role-status", "role", "status")
		if !regexp.MustCompile(`<a href="/admin/roles"[^>]*>Back to roles</a>`).MatchString(out) {
			t.Errorf("no link to /admin/roles reading Back to roles in:\n%s", out)
		}
	})
}

func TestAdminRoleContent(t *testing.T) {
	t.Run("CA-133_the_editor_container_names_its_status_line_and_marks_itself_as_a_one_request_region", func(t *testing.T) {
		out := renderString(t, AdminRoleContent(adminRoleFixture))
		assertAttr(t, out, "admin-role", "data-status", "admin-role-status")
		assertFlag(t, out, "admin-role", "data-busy-region", true)
		assertAttr(t, out, "admin-role", "hx-sync:inherited", "this:drop")
	})

	t.Run("CA-134_the_editor_holds_the_header_the_rename_form_the_grants_and_the_delete_button_none_of_them_out_of_band", func(t *testing.T) {
		out := renderString(t, AdminRoleContent(adminRoleFixture))
		for _, id := range []string{"admin-role-header", "admin-role-form", "admin-role-grants", "admin-role-delete"} {
			assertOnce(t, out, id)
		}
		assertLacks(t, out, "hx-swap-oob")
	})
}

func TestAdminRoleHeader(t *testing.T) {
	t.Run("CA-135_the_header_shows_the_roles_name_and_ID_in_their_own_elements", func(t *testing.T) {
		out := renderString(t, AdminRoleHeader(AdminRoleData{Role: Role{ID: "5", Name: "admins"}}, false))
		assertText(t, out, "admin-role-title", "admins")
		assertText(t, out, "admin-role-id", "5")
		assertFlag(t, out, "admin-role-header", "hx-swap-oob", false)
	})
}

func TestAdminRoleForm(t *testing.T) {
	t.Run("CA-138_the_form_posts_to_the_roles_route_and_swaps_itself_with_the_answer", func(t *testing.T) {
		out := renderString(t, AdminRoleForm(adminRoleFixture))
		assertAttr(t, out, "admin-role-form", "hx-post", "/admin/roles/5")
		assertAttr(t, out, "admin-role-form", "hx-target", "#admin-role-form")
		assertAttr(t, out, "admin-role-form", "hx-swap", "outerHTML")
		assertAttr(t, out, "admin-role-save", "type", "submit")
		assertText(t, out, "admin-role-save", "Save")
	})

	t.Run("CA-139_the_role_ID_is_path_escaped_in_hx_post", func(t *testing.T) {
		for id, want := range map[string]string{"a/b": "/admin/roles/a%2Fb", "a b": "/admin/roles/a%20b"} {
			t.Run("CA-139_"+strings.ReplaceAll(id, "/", "slash_"), func(t *testing.T) {
				d := adminRoleFixture
				d.Role.ID = id
				assertAttr(t, renderString(t, AdminRoleForm(d)), "admin-role-form", "hx-post", want)
			})
		}
	})

	t.Run("CA-140_the_form_carries_the_loaded_name_and_description_as_hidden_inputs", func(t *testing.T) {
		out := renderString(t, AdminRoleForm(adminRoleFixture))
		for name, want := range map[string]string{"loaded_name": "admins", "loaded_description": "Site admins"} {
			field := inputByName(t, out, name)
			attrOfTag(t, field, "type", "hidden")
			attrOfTag(t, field, "value", want)
		}
	})

	t.Run("CA-141_the_name_and_description_fields_hold_the_current_values_under_the_names_the_handler_reads", func(t *testing.T) {
		out := renderString(t, AdminRoleForm(adminRoleFixture))
		assertAttr(t, out, "admin-role-name", "name", "name")
		assertAttr(t, out, "admin-role-name", "value", "admins")
		assertFlag(t, out, "admin-role-name", "required", true)
		assertAttr(t, out, "admin-role-description", "name", "description")
		assertAttr(t, out, "admin-role-description", "value", "Site admins")
		assertFlag(t, out, "admin-role-description", "required", false)
		assertContains(t, out, `for="admin-role-name"`, `for="admin-role-description"`)
	})

	t.Run("CA-142_the_name_and_description_are_escaped_in_the_hidden_and_visible_inputs", func(t *testing.T) {
		name, description := `"><i>x</i> & 'y'`, `"><b>z</b>`
		out := renderString(t, AdminRoleForm(AdminRoleData{Role: Role{ID: "5", Name: name, Description: description}}))
		assertLacks(t, out, "<i>", "<b>")
		attrOfTag(t, inputByName(t, out, "loaded_name"), "value", name)
		attrOfTag(t, inputByName(t, out, "loaded_description"), "value", description)
		attrOfTag(t, inputByName(t, out, "name"), "value", name)
		attrOfTag(t, inputByName(t, out, "description"), "value", description)
		assertTagsEndCleanly(t, openTags(out, "input"))
	})
}

func TestAdminRoleNameField(t *testing.T) {
	t.Run("CA-143_the_name_field_is_a_required_labelled_text_input_with_the_current_value_and_no_error", func(t *testing.T) {
		out := renderString(t, AdminRoleName("admins", "", false))
		field, _ := innerHTML(out, "admin-role-name-field")
		assertContains(t, field, `for="admin-role-name"`, ">Name</label>")
		tag := testutil.TagByID(out, "admin-role-name")
		attrOfTag(t, tag, "name", "name")
		attrOfTag(t, tag, "value", "admins")
		for name, want := range map[string]bool{"required": true, "aria-invalid": false, "aria-describedby": false, "placeholder": false} {
			if _, ok := tagAttr(tag, name); ok != want {
				t.Errorf("%s present = %t, want %t; tag %s", name, ok, want, tag)
			}
		}
		assertLacks(t, out, `id="admin-role-name-error"`)
	})

	t.Run("CA-144_a_refusal_shows_the_message_and_marks_the_input_invalid_and_described_by_it", func(t *testing.T) {
		out := renderString(t, AdminRoleName("x", "Name is taken", false))
		assertText(t, out, "admin-role-name-error", "Name is taken")
		assertAttr(t, out, "admin-role-name", "aria-invalid", "true")
		assertAttr(t, out, "admin-role-name", "aria-describedby", "admin-role-name-error")
		assertFlag(t, out, "admin-role-name", "autofocus", true)
		assertAttr(t, out, "admin-role-name", "value", "x")
	})
}

func TestAdminRoleCreateName(t *testing.T) {
	const help = "Lower-case letters, digits and underscores, starting with a letter"

	t.Run("CA-147_the_create_name_field_is_required_has_a_placeholder_and_a_help_text_the_input_points_to", func(t *testing.T) {
		out := renderString(t, AdminRoleCreateName("", "", false))
		assertAttr(t, out, "admin-role-create-name", "name", "name")
		assertFlag(t, out, "admin-role-create-name", "required", true)
		assertAttr(t, out, "admin-role-create-name", "placeholder", "moderator")
		assertAttr(t, out, "admin-role-create-name", "aria-describedby", "admin-role-create-name-help")
		assertFlag(t, out, "admin-role-create-name", "aria-invalid", false)
		assertText(t, out, "admin-role-create-name-help", help)
		assertLacks(t, out, `id="admin-role-create-name-error"`)
	})

	t.Run("CA-148_a_refusal_shows_the_message_first_in_the_description_and_marks_the_input_invalid", func(t *testing.T) {
		out := renderString(t, AdminRoleCreateName("Bad Name", "Name is invalid", false))
		assertText(t, out, "admin-role-create-name-error", "Name is invalid")
		assertAttr(t, out, "admin-role-create-name", "value", "Bad Name")
		assertAttr(t, out, "admin-role-create-name", "aria-invalid", "true")
		assertFlag(t, out, "admin-role-create-name", "autofocus", true)
		assertAttr(t, out, "admin-role-create-name", "aria-describedby", "admin-role-create-name-error admin-role-create-name-help")
		assertText(t, out, "admin-role-create-name-help", help)
	})
}

func TestAdminRoleDelete(t *testing.T) {
	t.Run("CA-151_the_delete_button_sends_a_DELETE_for_the_role_after_a_confirmation_naming_it", func(t *testing.T) {
		out := renderString(t, AdminRoleDelete(AdminRoleData{Role: Role{ID: "5", Name: "admins"}}, false))
		assertAttr(t, out, "admin-role-delete", "type", "button")
		assertAttr(t, out, "admin-role-delete", "hx-delete", "/admin/roles/5")
		assertAttr(t, out, "admin-role-delete", "hx-confirm", "Delete the role admins?")
		assertFlag(t, out, "admin-role-delete", "hx-swap-oob", false)
		assertText(t, out, "admin-role-delete", "Delete role")
	})

	t.Run("CA-152_the_out_of_band_copy_marks_the_button_for_swapping", func(t *testing.T) {
		for _, oob := range []bool{true, false} {
			t.Run("CA-152_"+oobName[oob], func(t *testing.T) {
				out := renderString(t, AdminRoleDelete(adminRoleFixture, oob))
				if oob {
					assertAttr(t, out, "admin-role-delete", "hx-swap-oob", "true")
				} else {
					assertFlag(t, out, "admin-role-delete", "hx-swap-oob", false)
				}
			})
		}
	})

	t.Run("CA-153_the_name_in_the_confirmation_and_the_ID_in_the_path_are_escaped", func(t *testing.T) {
		out := renderString(t, AdminRoleDelete(AdminRoleData{Role: Role{ID: "a/b", Name: `"><i>x</i> & 'y'`}}, false))
		assertAttr(t, out, "admin-role-delete", "hx-delete", "/admin/roles/a%2Fb")
		assertAttr(t, out, "admin-role-delete", "hx-confirm", `Delete the role "><i>x</i> & 'y'?`)
		assertLacks(t, out, "<i>")
		assertTagsEndCleanly(t, openTags(out, "button"))
	})
}

func TestAdminRoleGrants(t *testing.T) {
	t.Run("CA-154_the_grants_region_sends_its_changes_to_the_granted_list_and_swaps_it_whole", func(t *testing.T) {
		out := renderString(t, AdminRoleGrants(adminRoleFixture))
		assertAttr(t, out, "admin-role-grants", "hx-target:inherited", "#admin-role-granted")
		assertAttr(t, out, "admin-role-grants", "hx-swap:inherited", "outerHTML")
	})

	t.Run("CA-155_the_granted_list_and_the_grant_form_both_sit_inside_the_region_and_neither_is_out_of_band", func(t *testing.T) {
		out := renderString(t, AdminRoleGrants(adminRoleFixture))
		region, _ := innerHTML(out, "admin-role-grants")
		assertContains(t, region, `id="admin-role-granted"`, `id="admin-role-grant"`)
		assertLacks(t, out, "hx-swap-oob")
	})
}

func TestAdminRoleGranted(t *testing.T) {
	granted := func(grants ...RolePermission) AdminRoleData {
		return AdminRoleData{Role: Role{ID: "5", Permissions: grants}}
	}

	t.Run("CA-156_every_granted_permission_has_a_hidden_granted_input_inside_the_list_container", func(t *testing.T) {
		out := renderString(t, AdminRoleGranted(granted(grantOf("1", ""), grantOf("2", "7")), false))
		assertFlag(t, out, "admin-role-granted", "hx-swap-oob", false)
		inner, _ := innerHTML(out, "admin-role-granted")
		var values []string
		for _, tag := range openTags(inner, "input") {
			if name, _ := tagAttr(tag, "name"); name == "granted" {
				attrOfTag(t, tag, "type", "hidden")
				value, _ := tagAttr(tag, "value")
				values = append(values, value)
			}
		}
		if want := []string{"1", "2"}; !reflect.DeepEqual(values, want) {
			t.Errorf("granted values = %q, want %q", values, want)
		}
	})

	t.Run("CA-157_the_list_has_a_heading_that_can_take_focus_and_takes_it_only_when_FocusList_is_set", func(t *testing.T) {
		for _, focus := range []bool{true, false} {
			t.Run("CA-157_"+focusName[focus], func(t *testing.T) {
				d := granted(grantOf("1", ""))
				d.FocusList = focus
				out := renderString(t, AdminRoleGranted(d, false))
				assertElement(t, out, "admin-role-permissions-title", "h2")
				assertAttr(t, out, "admin-role-permissions-title", "tabindex", "-1")
				assertText(t, out, "admin-role-permissions-title", "Granted permissions")
				assertFlag(t, out, "admin-role-permissions-title", "autofocus", focus)
			})
		}
	})

	t.Run("CA-158_a_role_without_grants_shows_the_empty_note_and_no_list_or_hidden_input", func(t *testing.T) {
		out := renderString(t, AdminRoleGranted(granted(), false))
		assertText(t, out, "admin-role-permissions-empty", "This role grants no permissions")
		assertLacks(t, out, ` id="admin-role-permissions"`, `name="granted"`)
		assertOnce(t, out, "admin-role-permissions-title")
	})

	t.Run("CA-160_each_row_has_a_remove_button_that_sends_a_DELETE_and_includes_the_whole_grants_region", func(t *testing.T) {
		out := renderString(t, AdminRoleGranted(granted(grantOf("1", ""), grantOf("2", "5")), false))
		for _, want := range []struct{ id, node string }{{"1", "pets.read"}, {"2", "pets.max"}} {
			row, _ := innerHTML(out, "granted-"+want.id)
			remove := openTags(row, "button")[0]
			attrOfTag(t, remove, "type", "button")
			attrOfTag(t, remove, "aria-label", "Remove "+want.node)
			attrOfTag(t, remove, "hx-delete", "/admin/roles/5/permissions/"+want.id)
			attrOfTag(t, remove, "hx-include", "#admin-role-grants")
		}
	})

	t.Run("CA-161_a_valued_grant_has_a_small_form_that_posts_its_value_under_value_id_and_includes_the_region", func(t *testing.T) {
		cases := []struct {
			name, id, node, field, valueType, value string
			content                                 string
			grant                                   RolePermission
		}{
			{"int", "2", "pets.max", "input", "number", "7", "", grantOf("2", "7")},
			{"string", "4", "pets.motto", "input", "text", "hi", "", grantOf("4", `"hi"`)},
			{"string_list", "3", "pets.tags", "textarea", "", "", "a\nb", grantOf("3", `["a","b"]`)},
		}
		for _, tc := range cases {
			t.Run("CA-161_"+tc.name, func(t *testing.T) {
				out := renderString(t, AdminRoleGranted(granted(tc.grant), false))
				row, _ := innerHTML(out, "granted-"+tc.id)
				form := openTags(row, "form")[0]
				attrOfTag(t, form, "hx-post", "/admin/roles/5/permissions/"+tc.id)
				attrOfTag(t, form, "hx-include", "#admin-role-grants")
				field := inputByName(t, row, "value_"+tc.id)
				if !strings.HasPrefix(field, "<"+tc.field) {
					t.Errorf("field = %s, want a %s", field, tc.field)
				}
				if tc.field == "input" {
					attrOfTag(t, field, "type", tc.valueType)
					attrOfTag(t, field, "value", tc.value)
				} else {
					assertContains(t, row, field+tc.content+"</textarea>")
				}
				submit := openTags(row, "button")[1]
				attrOfTag(t, submit, "type", "submit")
				attrOfTag(t, submit, "aria-label", "Save value of "+tc.node)
				assertContains(t, row, ">Save value</button>")
			})
		}
	})

	t.Run("CA-162_a_grant_without_a_value_type_has_no_value_form", func(t *testing.T) {
		out := renderString(t, AdminRoleGranted(granted(grantOf("1", "")), false))
		row, _ := innerHTML(out, "granted-1")
		assertContains(t, row, `aria-label="Remove pets.read"`)
		assertLacks(t, row, "<form", "value_1")
	})

	t.Run("CA-163_a_typed_draft_is_shown_in_a_rows_field_instead_of_the_saved_value_and_an_emptied_draft_stays_empty", func(t *testing.T) {
		cases := []struct {
			name   string
			drafts map[string]string
			want   string
		}{
			{"typed", map[string]string{"2": "99"}, "99"},
			{"emptied", map[string]string{"2": ""}, ""},
			{"no_draft", nil, "7"},
		}
		for _, tc := range cases {
			t.Run("CA-163_"+tc.name, func(t *testing.T) {
				d := granted(grantOf("2", "7"))
				d.Drafts = tc.drafts
				field := inputByName(t, renderString(t, AdminRoleGranted(d, false)), "value_2")
				if got, _ := tagAttr(field, "value"); got != tc.want {
					t.Errorf("value = %q, want %q", got, tc.want)
				}
			})
		}
	})

	t.Run("CA-164_the_out_of_band_copy_marks_the_container_for_swapping", func(t *testing.T) {
		for _, oob := range []bool{true, false} {
			t.Run("CA-164_"+oobName[oob], func(t *testing.T) {
				out := renderString(t, AdminRoleGranted(adminRoleFixture, oob))
				if oob {
					assertAttr(t, out, "admin-role-granted", "hx-swap-oob", "true")
				} else {
					assertFlag(t, out, "admin-role-granted", "hx-swap-oob", false)
				}
			})
		}
	})

	t.Run("CA-165_nodes_descriptions_IDs_and_values_are_escaped_in_text_and_attributes", func(t *testing.T) {
		grant := RolePermission{
			Permission: Permission{ID: "<u>1", Node: "<i>n</i>", Description: "<b>d</b> & 'x'", ValueType: "string"},
			Value:      json.RawMessage(`"\"><s>v</s>"`),
		}
		for name, draft := range map[string]string{"saved_value": "", "draft": `"><em>e</em>`} {
			t.Run("CA-165_"+name, func(t *testing.T) {
				d := granted(grant)
				want := `"><s>v</s>`
				if draft != "" {
					d.Drafts, want = map[string]string{"<u>1": draft}, draft
				}
				out := renderString(t, AdminRoleGranted(d, false))
				assertLacks(t, out, "<i>", "<b>", "<u>", "<s>", "<em>")
				attrOfTag(t, inputByName(t, out, "value_<u>1"), "value", want)
				attrOfTag(t, inputByName(t, out, "granted"), "value", "<u>1")
				attrOfTag(t, openTags(out, "button")[0], "aria-label", "Remove <i>n</i>")
				assertContains(t, visibleText(out), "<i>n</i>", "<b>d</b> & 'x'")
				assertTagsEndCleanly(t, append(openTags(out, "input"), openTags(out, "button")...))
			})
		}
	})

	t.Run("CA-166_both_IDs_are_path_escaped_in_the_remove_and_save_requests", func(t *testing.T) {
		grant := RolePermission{Permission: Permission{ID: "c d", Node: "pets.motto", ValueType: "string"}, Value: json.RawMessage(`"hi"`)}
		out := renderString(t, AdminRoleGranted(AdminRoleData{Role: Role{ID: "a/b", Permissions: []RolePermission{grant}}}, false))
		want := "/admin/roles/a%2Fb/permissions/c%20d"
		attrOfTag(t, openTags(out, "button")[0], "hx-delete", want)
		attrOfTag(t, openTags(out, "form")[0], "hx-post", want)
	})
}

func TestAdminRoleGrantForm(t *testing.T) {
	t.Run("CA-167_the_out_of_band_copy_marks_the_grant_region_for_swapping", func(t *testing.T) {
		for _, oob := range []bool{true, false} {
			t.Run("CA-167_"+oobName[oob], func(t *testing.T) {
				out := renderString(t, AdminRoleGrantForm(adminRoleFixture, oob))
				if oob {
					assertAttr(t, out, "admin-role-grant", "hx-swap-oob", "true")
				} else {
					assertFlag(t, out, "admin-role-grant", "hx-swap-oob", false)
				}
			})
		}
	})

	t.Run("CA-168_the_grant_form_posts_to_the_roles_permissions_route_and_includes_the_whole_grants_region", func(t *testing.T) {
		out := renderString(t, AdminRoleGrantForm(adminRoleFixture, false))
		assertAttr(t, out, "admin-role-grant-form", "hx-post", "/admin/roles/5/permissions")
		assertAttr(t, out, "admin-role-grant-form", "hx-include", "#admin-role-grants")
		assertContains(t, out, ">Grant a permission</h2>")
		assertAttr(t, out, "admin-role-grant-submit", "type", "submit")
		assertText(t, out, "admin-role-grant-submit", "Grant")
	})

	t.Run("CA-169_the_permission_select_asks_for_the_matching_value_input_when_it_changes", func(t *testing.T) {
		out := renderString(t, AdminRoleGrantForm(adminRoleFixture, false))
		assertElement(t, out, "admin-role-grant-permission", "select")
		for name, want := range map[string]string{
			"name": "grant_permission", "aria-label": "Permission", "hx-get": "/admin/roles/5/grant-value", "hx-trigger": "change",
			"hx-target": "#admin-role-grant-value", "hx-swap": "innerHTML", "hx-sync": "this:replace",
		} {
			assertAttr(t, out, "admin-role-grant-permission", name, want)
		}
		assertOnce(t, out, "admin-role-grant-value")
	})

	t.Run("CA-174_a_selected_permission_without_a_value_type_leaves_the_value_region_empty", func(t *testing.T) {
		d := AdminRoleData{Role: Role{ID: "5", Permissions: grantedIDs("2", "3", "4")}, Catalogue: catalogueC}
		out := renderString(t, AdminRoleGrantForm(d, false))
		options := openTags(out, "option")
		if len(options) != 1 {
			t.Fatalf("found %d options, want 1", len(options))
		}
		attrOfTag(t, options[0], "value", "1")
		if _, ok := tagAttr(options[0], "selected"); !ok {
			t.Errorf("option %s is not selected", options[0])
		}
		assertText(t, out, "admin-role-grant-value", "")
	})

	t.Run("CA-176_nodes_IDs_the_grant_value_and_the_role_ID_are_escaped", func(t *testing.T) {
		d := AdminRoleData{
			Role:            Role{ID: "a/b"},
			Catalogue:       []Permission{{ID: "<u>3", Node: "<i>n</i> & 'x'", ValueType: "string"}},
			GrantPermission: "<u>3",
			GrantValue:      `"><b>v</b>`,
		}
		out := renderString(t, AdminRoleGrantForm(d, false))
		assertLacks(t, out, "<i>", "<u>", "<b>")
		options := openTags(out, "option")
		if len(options) != 1 {
			t.Fatalf("found %d options, want 1", len(options))
		}
		attrOfTag(t, options[0], "value", "<u>3")
		if text := regexp.MustCompile(`<option[^>]*>([^<]*)</option>`).FindStringSubmatch(out); text == nil || html.UnescapeString(text[1]) != "<i>n</i> & 'x'" {
			t.Errorf("option text = %q", text)
		}
		attrOfTag(t, inputByName(t, out, "grant_value"), "value", `"><b>v</b>`)
		assertAttr(t, out, "admin-role-grant-form", "hx-post", "/admin/roles/a%2Fb/permissions")
		assertAttr(t, out, "admin-role-grant-permission", "hx-get", "/admin/roles/a%2Fb/grant-value")
		assertTagsEndCleanly(t, append(openTags(out, "input"), options...))
	})
}

func TestAdminRoleGrantUnavailable(t *testing.T) {
	t.Run("CA-177_the_fragment_replaces_the_grant_region_with_a_note_that_the_permissions_could_not_be_loaded", func(t *testing.T) {
		out := renderString(t, AdminRoleGrantUnavailable())
		assertAttr(t, out, "admin-role-grant", "hx-swap-oob", "true")
		assertText(t, out, "admin-role-grant-unavailable", "The permissions to grant could not be loaded. Reload the page to grant one.")
		region, _ := innerHTML(out, "admin-role-grant")
		assertContains(t, region, `id="admin-role-grant-unavailable"`)
	})
}

func TestAdminPermissionsContent(t *testing.T) {
	t.Run("CA-179_the_content_root_drops_a_second_request_and_sends_changes_to_the_list_swapping_it_whole", func(t *testing.T) {
		out := renderString(t, AdminPermissionsContent(AdminPermissionsData{}))
		assertFlag(t, out, "admin-permissions-root", "data-busy-region", true)
		assertAttr(t, out, "admin-permissions-root", "hx-sync:inherited", "this:drop")
		assertAttr(t, out, "admin-permissions-root", "hx-target:inherited", "#admin-permissions-list")
		assertAttr(t, out, "admin-permissions-root", "hx-swap:inherited", "outerHTML")
	})

	t.Run("CA-180_the_root_holds_the_list_and_the_create_form_neither_out_of_band", func(t *testing.T) {
		out := renderString(t, AdminPermissionsContent(AdminPermissionsData{Permissions: []Permission{catalogueC[0]}}))
		root, _ := innerHTML(out, "admin-permissions-root")
		assertContains(t, root, `id="admin-permissions-list"`, `id="admin-permission-create-form"`)
		assertLacks(t, out, "hx-swap-oob")
	})
}

func TestAdminPermissionList(t *testing.T) {
	t.Run("CA-182_the_list_heading_can_take_focus_and_takes_it_only_when_FocusList_is_set", func(t *testing.T) {
		for _, focus := range []bool{true, false} {
			t.Run("CA-182_"+focusName[focus], func(t *testing.T) {
				out := renderString(t, AdminPermissionList(AdminPermissionsData{FocusList: focus}))
				assertElement(t, out, "admin-permissions-title", "h2")
				assertAttr(t, out, "admin-permissions-title", "tabindex", "-1")
				assertFlag(t, out, "admin-permissions-title", "autofocus", focus)
			})
		}
	})

	t.Run("CA-183_an_empty_list_shows_the_empty_note_and_keeps_the_list_element", func(t *testing.T) {
		out := renderString(t, AdminPermissionList(AdminPermissionsData{}))
		assertText(t, out, "admin-permissions-empty", "No permissions")
		assertElement(t, out, "admin-permissions", "ul")
		assertText(t, out, "admin-permissions", "")
	})
}

func TestAdminPermissionRow(t *testing.T) {
	t.Run("CA-184_the_row_has_the_permissions_id_node_description_and_a_delete_button_that_sends_a_DELETE_after_a_confirmation", func(t *testing.T) {
		out := renderString(t, adminPermissionRow(Permission{ID: "7", Node: "pets.read", Description: "Read pets"}))
		assertElement(t, out, "permission-7", "li")
		assertContains(t, out, ">pets.read</span>", ">Read pets</span>")
		button := openTags(out, "button")[0]
		for name, want := range map[string]string{
			"type": "button", "aria-label": "Delete pets.read", "hx-delete": "/admin/permissions/7", "hx-confirm": "Delete the permission pets.read?",
		} {
			attrOfTag(t, button, name, want)
		}
		if _, ok := tagAttr(button, "hx-target"); ok {
			t.Errorf("button %s holds hx-target", button)
		}
		assertContains(t, out, ">Delete</button>")
	})

	t.Run("CA-185_a_valued_permission_shows_its_type_and_merge_rule", func(t *testing.T) {
		cases := []struct {
			name string
			p    Permission
			want string
		}{
			{"int", Permission{ID: "2", Node: "pets.max", ValueType: "int", Merge: "max"}, "int, merge max"},
			{"string_list", Permission{ID: "3", Node: "pets.tags", ValueType: "string_list", Merge: "union"}, "string_list, merge union"},
		}
		for _, tc := range cases {
			t.Run("CA-185_"+tc.name, func(t *testing.T) {
				assertContains(t, renderString(t, adminPermissionRow(tc.p)), ">"+tc.want+"</span>")
			})
		}
	})

	t.Run("CA-186_a_permission_granted_as_is_shows_no_type_chip", func(t *testing.T) {
		assertLacks(t, renderString(t, adminPermissionRow(Permission{ID: "1", Node: "pets.read"})), "merge")
	})

	t.Run("CA-187_the_node_description_and_ID_are_escaped_in_text_and_attributes_and_the_ID_is_path_escaped_in_hx_delete", func(t *testing.T) {
		p := Permission{ID: `a/b"><u>`, Node: "<i>n</i> & 'x'", Description: "<b>d</b>", ValueType: "<s>t</s>", Merge: "<em>m</em>"}
		out := renderString(t, adminPermissionRow(p))
		assertLacks(t, out, "<i>", "<b>", "<u>", "<s>", "<em>")
		attrOfTag(t, openTags(out, "li")[0], "id", "permission-"+p.ID)
		button := openTags(out, "button")[0]
		attrOfTag(t, button, "aria-label", "Delete "+p.Node)
		attrOfTag(t, button, "hx-confirm", "Delete the permission "+p.Node+"?")
		attrOfTag(t, button, "hx-delete", "/admin/permissions/a%2Fb%22%3E%3Cu%3E")
		assertContains(t, visibleText(out), "<i>n</i> & 'x'", "<b>d</b>", "<s>t</s>, merge <em>m</em>")
		assertTagsEndCleanly(t, append(openTags(out, "li"), button))
	})
}

func TestAdminPermissionAdded(t *testing.T) {
	added := Permission{ID: "9", Node: "pets.write", Description: "Write pets"}

	t.Run("CA-188_a_created_permission_is_appended_to_the_list_out_of_band", func(t *testing.T) {
		out := renderString(t, AdminPermissionAdded(added))
		list := openTags(out, "ul")[0]
		attrOfTag(t, list, "hx-swap-oob", "beforeend:#admin-permissions")
		assertInOrder(t, out, list, `id="permission-9"`, ">pets.write</span>", "</ul>")
	})

	t.Run("CA-189_the_empty_note_is_removed_out_of_band", func(t *testing.T) {
		assertContains(t, renderString(t, AdminPermissionAdded(added)), `<div hx-swap-oob="delete:#admin-permissions-empty"></div>`)
	})
}

func TestAdminPermissionCreateNode(t *testing.T) {
	const help = "Lower-case words of letters, digits and underscores joined by dots"

	t.Run("CA-190_the_node_field_is_required_has_a_placeholder_and_a_help_text_the_input_points_to", func(t *testing.T) {
		out := renderString(t, AdminPermissionCreateNode("", "", false))
		assertAttr(t, out, "admin-permission-create-node", "name", "node")
		assertFlag(t, out, "admin-permission-create-node", "required", true)
		assertAttr(t, out, "admin-permission-create-node", "placeholder", "petpictures.admin")
		assertAttr(t, out, "admin-permission-create-node", "aria-describedby", "admin-permission-create-node-help")
		assertFlag(t, out, "admin-permission-create-node", "aria-invalid", false)
		assertText(t, out, "admin-permission-create-node-help", help)
		assertContains(t, out, `for="admin-permission-create-node"`, ">Node</label>")
		assertLacks(t, out, `id="admin-permission-create-node-error"`)
	})

	t.Run("CA-191_a_refusal_shows_the_message_first_in_the_description_and_marks_the_input_invalid", func(t *testing.T) {
		out := renderString(t, AdminPermissionCreateNode(" Bad Node ", "Node is invalid", false))
		assertText(t, out, "admin-permission-create-node-error", "Node is invalid")
		assertAttr(t, out, "admin-permission-create-node", "value", " Bad Node ")
		assertAttr(t, out, "admin-permission-create-node", "aria-invalid", "true")
		assertFlag(t, out, "admin-permission-create-node", "autofocus", true)
		assertAttr(t, out, "admin-permission-create-node", "aria-describedby", "admin-permission-create-node-error admin-permission-create-node-help")
	})
}

func TestAdminPermissionForm(t *testing.T) {
	t.Run("CA-194_the_create_form_posts_to_the_permissions_route", func(t *testing.T) {
		out := renderString(t, AdminPermissionForm(false))
		assertAttr(t, out, "admin-permission-create-form", "hx-post", "/admin/permissions")
		assertFlag(t, out, "admin-permission-create-form", "hx-swap-oob", false)
		assertAttr(t, out, "admin-permission-create-submit", "type", "submit")
		assertText(t, out, "admin-permission-create-submit", "Create permission")
	})

	t.Run("CA-195_the_out_of_band_copy_marks_the_form_for_swapping", func(t *testing.T) {
		for _, oob := range []bool{true, false} {
			t.Run("CA-195_"+oobName[oob], func(t *testing.T) {
				out := renderString(t, AdminPermissionForm(oob))
				if oob {
					assertAttr(t, out, "admin-permission-create-form", "hx-swap-oob", "true")
				} else {
					assertFlag(t, out, "admin-permission-create-form", "hx-swap-oob", false)
				}
			})
		}
	})

	t.Run("CA-196_the_form_is_fresh_with_an_empty_node_and_the_description_under_the_name_the_handler_reads", func(t *testing.T) {
		out := renderString(t, AdminPermissionForm(false))
		assertAttr(t, out, "admin-permission-create-node", "name", "node")
		assertFlag(t, out, "admin-permission-create-node", "value", false)
		assertLacks(t, out, `id="admin-permission-create-node-error"`)
		assertAttr(t, out, "admin-permission-create-description", "name", "description")
		assertFlag(t, out, "admin-permission-create-description", "value", false)
		assertContains(t, out, `for="admin-permission-create-description"`, ">Description</label>", ">New permission</h2>")
	})

	t.Run("CA-197_the_value_type_select_offers_the_four_types_the_API_accepts_none_first", func(t *testing.T) {
		out := renderString(t, AdminPermissionForm(false))
		assertAttr(t, out, "admin-permission-create-type", "name", "value_type")
		assertContains(t, out, `for="admin-permission-create-type"`, ">Value</label>")
		inner, _ := innerHTML(out, "admin-permission-create-type")
		var got []string
		for _, option := range regexp.MustCompile(`<option value="([^"]*)">([^<]*)</option>`).FindAllStringSubmatch(inner, -1) {
			got = append(got, option[1])
		}
		if want := []string{"", "int", "string", "string_list"}; !reflect.DeepEqual(got, want) {
			t.Errorf("option values = %q, want %q", got, want)
		}
		assertContains(t, inner, ">None (granted as is)</option>")
	})

	t.Run("CA-198_the_merge_select_offers_the_highest_value_first_and_the_lowest_second", func(t *testing.T) {
		out := renderString(t, AdminPermissionForm(false))
		assertAttr(t, out, "admin-permission-create-merge", "name", "merge")
		field, _ := innerHTML(out, "admin-permission-create-merge-field")
		assertContains(t, field, `for="admin-permission-create-merge"`, `id="admin-permission-create-merge"`)
		inner, _ := innerHTML(out, "admin-permission-create-merge")
		var got []string
		for _, option := range regexp.MustCompile(`<option value="([^"]*)">`).FindAllStringSubmatch(inner, -1) {
			got = append(got, option[1])
		}
		if want := []string{"max", "min"}; !reflect.DeepEqual(got, want) {
			t.Errorf("option values = %q, want %q", got, want)
		}
	})
}

func TestAdminPermissionsPage(t *testing.T) {
	t.Run("CA-199_the_page_is_a_full_document_with_its_title_that_loads_the_list_and_links_to_the_dashboard_and_the_roles_page", func(t *testing.T) {
		out := renderString(t, AdminPermissionsPage())
		assertContains(t, out, "<title>Permissions - NeuralNexus</title>", ">Permissions</h1>")
		tag := regexp.MustCompile(`<div[^>]*hx-get="/admin/permissions/list"[^>]*>`).FindString(out)
		attrOfTag(t, tag, "hx-trigger", "load")
		for href, text := range map[string]string{"/admin": "Back to the admin dashboard", "/admin/roles": "Roles"} {
			if !regexp.MustCompile(`<a href="` + href + `"[^>]*>` + text + `</a>`).MatchString(out) {
				t.Errorf("no link to %s reading %s in:\n%s", href, text, out)
			}
		}
	})
}
