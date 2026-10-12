package components

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestRolePermissionValueText(t *testing.T) {
	value := func(valueType, raw string) RolePermission {
		p := RolePermission{Permission: Permission{ValueType: valueType}}
		if raw != "" {
			p.Value = json.RawMessage(raw)
		}
		return p
	}
	check := func(t *testing.T, row string, cases []struct{ name, valueType, raw, want string }) {
		for _, tc := range cases {
			t.Run(row+"_"+tc.name, func(t *testing.T) {
				if got := value(tc.valueType, tc.raw).ValueText(); got != tc.want {
					t.Errorf("ValueText() = %q, want %q", got, tc.want)
				}
			})
		}
	}
	type valueCase = struct{ name, valueType, raw, want string }

	t.Run("CA-001_a_string_permission_gives_its_decoded_text", func(t *testing.T) {
		check(t, "CA-001", []valueCase{
			{"plain", "string", `"hello world"`, "hello world"},
			{"with_quotes", "string", `"say \"hi\""`, `say "hi"`},
		})
	})

	t.Run("CA-002_a_string_list_permission_gives_one_item_per_line", func(t *testing.T) {
		check(t, "CA-002", []valueCase{
			{"two_items", "string_list", `["a","b"]`, "a\nb"},
			{"one_item", "string_list", `["only"]`, "only"},
			{"no_items", "string_list", `[]`, ""},
		})
	})

	t.Run("CA-003_an_int_permission_gives_the_number_as_text", func(t *testing.T) {
		check(t, "CA-003", []valueCase{
			{"positive", "int", `7`, "7"},
			{"negative", "int", `-3`, "-3"},
			{"zero", "int", `0`, "0"},
		})
	})

	t.Run("CA-004_a_permission_with_no_value_type_or_an_unknown_one_gives_an_empty_string_whatever_Value_holds", func(t *testing.T) {
		check(t, "CA-004", []valueCase{
			{"no_type_null", "", `null`, ""},
			{"no_type_number", "", `5`, ""},
			{"unknown_type", "bool", `true`, ""},
		})
	})

	t.Run("CA-005_a_valued_permission_without_a_Value_gives_an_empty_string_for_every_type", func(t *testing.T) {
		check(t, "CA-005", []valueCase{
			{"string", "string", "", ""},
			{"string_list", "string_list", "", ""},
			{"int", "int", "", ""},
		})
	})
}

func TestRolePermissionLabel(t *testing.T) {
	t.Run("CA-006_a_permission_without_a_value_type_is_labelled_with_its_node_alone", func(t *testing.T) {
		p := RolePermission{Permission: Permission{Node: "pets.read"}, Value: json.RawMessage("5")}
		if got := p.Label(); got != "pets.read" {
			t.Errorf("Label() = %q", got)
		}
	})

	t.Run("CA-007_a_valued_permission_is_labelled_with_its_node_and_its_value_list_items_joined_by_commas", func(t *testing.T) {
		cases := []struct{ name, node, valueType, raw, want string }{
			{"int", "pets.max", "int", `5`, "pets.max: 5"},
			{"string", "pets.motto", "string", `"hi"`, "pets.motto: hi"},
			{"list", "pets.tags", "string_list", `["a","b","c"]`, "pets.tags: a, b, c"},
			{"one_item_list", "pets.tags", "string_list", `["a"]`, "pets.tags: a"},
		}
		for _, tc := range cases {
			t.Run("CA-007_"+tc.name, func(t *testing.T) {
				p := RolePermission{Permission: Permission{Node: tc.node, ValueType: tc.valueType}, Value: json.RawMessage(tc.raw)}
				if got := p.Label(); got != tc.want {
					t.Errorf("Label() = %q, want %q", got, tc.want)
				}
			})
		}
	})
}

func TestAdminUserDataHeldRole(t *testing.T) {
	t.Run("CA-008_a_role_the_user_holds_is_reported_held_and_one_it_does_not_hold_is_not", func(t *testing.T) {
		d := AdminUserData{User: UserAccount{Roles: []string{"r1", "r3"}}}
		for id, want := range map[string]bool{"r1": true, "r3": true, "r2": false} {
			t.Run("CA-008_"+id, func(t *testing.T) {
				if got := d.HeldRole(id); got != want {
					t.Errorf("HeldRole(%q) = %t, want %t", id, got, want)
				}
			})
		}
	})

	t.Run("CA-009_a_user_with_no_roles_holds_none", func(t *testing.T) {
		for name, id := range map[string]string{"role": "r1", "empty": ""} {
			t.Run("CA-009_"+name, func(t *testing.T) {
				if (AdminUserData{}).HeldRole(id) {
					t.Errorf("HeldRole(%q) = true", id)
				}
			})
		}
	})
}

func TestAdminUserDataUnlistedRoles(t *testing.T) {
	catalogue := []Role{{ID: "r1"}, {ID: "r2"}}

	t.Run("CA-010_held_roles_missing_from_the_catalogue_are_returned_in_the_order_the_user_holds_them", func(t *testing.T) {
		d := AdminUserData{Roles: catalogue, User: UserAccount{Roles: []string{"r2", "r9", "r1", "r3"}}}
		if got, want := d.UnlistedRoles(), []string{"r9", "r3"}; !reflect.DeepEqual(got, want) {
			t.Errorf("UnlistedRoles() = %q, want %q", got, want)
		}
	})

	t.Run("CA-011_when_every_held_role_is_in_the_catalogue_nothing_is_returned", func(t *testing.T) {
		for name, held := range map[string][]string{"holds_listed_roles": {"r1"}, "holds_nothing": nil} {
			t.Run("CA-011_"+name, func(t *testing.T) {
				if got := (AdminUserData{Roles: catalogue, User: UserAccount{Roles: held}}).UnlistedRoles(); len(got) != 0 {
					t.Errorf("UnlistedRoles() = %q, want none", got)
				}
			})
		}
	})

	t.Run("CA-012_with_an_empty_catalogue_every_held_role_is_unlisted", func(t *testing.T) {
		d := AdminUserData{User: UserAccount{Roles: []string{"r1", "r2"}}}
		if got, want := d.UnlistedRoles(), []string{"r1", "r2"}; !reflect.DeepEqual(got, want) {
			t.Errorf("UnlistedRoles() = %q, want %q", got, want)
		}
	})
}

func TestAdminRoleDataAvailable(t *testing.T) {
	t.Run("CA-013_permissions_the_role_already_grants_are_left_out_and_the_rest_keep_the_catalogue_order", func(t *testing.T) {
		d := AdminRoleData{Role: Role{Permissions: grantedIDs("1", "2")}, Catalogue: catalogueC}
		if got, want := permissionIDs(d.Available()), []string{"3", "4"}; !reflect.DeepEqual(got, want) {
			t.Errorf("Available() = %q, want %q", got, want)
		}
	})

	t.Run("CA-014_a_role_with_no_grants_offers_the_whole_catalogue_and_a_role_with_every_grant_offers_nothing", func(t *testing.T) {
		cases := []struct {
			name   string
			grants []RolePermission
			want   []string
		}{
			{"no_grants", nil, []string{"1", "2", "3", "4"}},
			{"every_grant", grantedIDs("1", "2", "3", "4"), []string{}},
		}
		for _, tc := range cases {
			t.Run("CA-014_"+tc.name, func(t *testing.T) {
				d := AdminRoleData{Role: Role{Permissions: tc.grants}, Catalogue: catalogueC}
				if got := permissionIDs(d.Available()); !reflect.DeepEqual(got, tc.want) {
					t.Errorf("Available() = %q, want %q", got, tc.want)
				}
			})
		}
	})

	t.Run("CA-015_a_grant_is_matched_to_the_catalogue_by_ID_not_by_node", func(t *testing.T) {
		d := AdminRoleData{Role: Role{Permissions: []RolePermission{{Permission: Permission{ID: "1"}}}}, Catalogue: catalogueC}
		if got, want := permissionIDs(d.Available()), []string{"2", "3", "4"}; !reflect.DeepEqual(got, want) {
			t.Errorf("Available() = %q, want %q", got, want)
		}
	})

	t.Run("CA-016_a_grant_that_is_not_in_the_catalogue_changes_nothing", func(t *testing.T) {
		d := AdminRoleData{Role: Role{Permissions: []RolePermission{{Permission: Permission{ID: "99"}}}}, Catalogue: catalogueC}
		if got, want := permissionIDs(d.Available()), []string{"1", "2", "3", "4"}; !reflect.DeepEqual(got, want) {
			t.Errorf("Available() = %q, want %q", got, want)
		}
	})
}

func TestAdminRoleDataSelected(t *testing.T) {
	withGrant := func(grantPermission string, granted ...string) AdminRoleData {
		return AdminRoleData{Role: Role{Permissions: grantedIDs(granted...)}, Catalogue: catalogueC, GrantPermission: grantPermission}
	}

	t.Run("CA-017_the_permission_named_in_GrantPermission_is_selected_when_it_is_still_available", func(t *testing.T) {
		if got, ok := withGrant("4", "1", "2").Selected(); !ok || got.ID != "4" {
			t.Errorf("Selected() = %+v, %t, want ID 4 and true", got, ok)
		}
	})

	t.Run("CA-018_a_GrantPermission_that_is_already_granted_falls_back_to_the_first_available_one", func(t *testing.T) {
		if got, ok := withGrant("1", "1", "2").Selected(); !ok || got.ID != "3" {
			t.Errorf("Selected() = %+v, %t, want ID 3 and true", got, ok)
		}
	})

	t.Run("CA-019_an_empty_or_unknown_GrantPermission_selects_the_first_available_permission", func(t *testing.T) {
		for name, choice := range map[string]string{"empty": "", "unknown": "99"} {
			t.Run("CA-019_"+name, func(t *testing.T) {
				if got, ok := withGrant(choice, "1", "2").Selected(); !ok || got.ID != "3" {
					t.Errorf("Selected() = %+v, %t, want ID 3 and true", got, ok)
				}
			})
		}
	})

	t.Run("CA-020_with_nothing_available_no_permission_is_selected", func(t *testing.T) {
		for name, choice := range map[string]string{"granted_choice": "1", "no_choice": ""} {
			t.Run("CA-020_"+name, func(t *testing.T) {
				if got, ok := withGrant(choice, "1", "2", "3", "4").Selected(); ok || got != (Permission{}) {
					t.Errorf("Selected() = %+v, %t, want the zero value and false", got, ok)
				}
			})
		}
	})
}

func TestAdminRoleDataDraft(t *testing.T) {
	t.Run("CA-021_text_typed_into_a_granted_rows_field_wins_over_the_saved_value", func(t *testing.T) {
		d := AdminRoleData{Drafts: map[string]string{"2": "99"}}
		if got := d.Draft(grantOf("2", "7")); got != "99" {
			t.Errorf("Draft() = %q, want 99", got)
		}
	})

	t.Run("CA-022_a_field_the_admin_emptied_keeps_the_empty_text_instead_of_the_saved_value", func(t *testing.T) {
		d := AdminRoleData{Drafts: map[string]string{"2": ""}}
		if got := d.Draft(grantOf("2", "7")); got != "" {
			t.Errorf("Draft() = %q, want empty", got)
		}
	})

	t.Run("CA-023_without_a_draft_for_the_permission_the_saved_value_is_returned", func(t *testing.T) {
		cases := []struct {
			name   string
			drafts map[string]string
			grant  RolePermission
			want   string
		}{
			{"nil_drafts", nil, grantOf("2", "7"), "7"},
			{"draft_for_another_permission", map[string]string{"3": "x"}, grantOf("2", "7"), "7"},
			{"list_value", nil, grantOf("3", `["a","b"]`), "a\nb"},
		}
		for _, tc := range cases {
			t.Run("CA-023_"+tc.name, func(t *testing.T) {
				if got := (AdminRoleData{Drafts: tc.drafts}).Draft(tc.grant); got != tc.want {
					t.Errorf("Draft() = %q, want %q", got, tc.want)
				}
			})
		}
	})
}

var pathEscapes = []struct{ id, escaped string }{
	{"a/b", "a%2Fb"},
	{"a b", "a%20b"},
	{"a?b", "a%3Fb"},
	{"a#b", "a%23b"},
	{"100%", "100%25"},
}

func TestAdminUserPath(t *testing.T) {
	t.Run("CA-027_an_ID_is_appended_to_the_user_route", func(t *testing.T) {
		if got := adminUserPath("u1"); got != "/admin/users/u1" {
			t.Errorf("adminUserPath = %q", got)
		}
	})

	t.Run("CA-028_an_ID_with_reserved_characters_is_path_escaped", func(t *testing.T) {
		for _, tc := range pathEscapes {
			t.Run("CA-028_"+tc.escaped, func(t *testing.T) {
				if got := adminUserPath(tc.id); got != "/admin/users/"+tc.escaped {
					t.Errorf("adminUserPath(%q) = %q", tc.id, got)
				}
			})
		}
	})
}

func TestAdminRolePath(t *testing.T) {
	t.Run("CA-029_an_ID_is_appended_to_the_role_route", func(t *testing.T) {
		if got := adminRolePath("5"); got != "/admin/roles/5" {
			t.Errorf("adminRolePath = %q", got)
		}
	})

	t.Run("CA-030_an_ID_with_reserved_characters_is_path_escaped", func(t *testing.T) {
		for _, tc := range pathEscapes {
			t.Run("CA-030_"+tc.escaped, func(t *testing.T) {
				if got := adminRolePath(tc.id); got != "/admin/roles/"+tc.escaped {
					t.Errorf("adminRolePath(%q) = %q", tc.id, got)
				}
			})
		}
	})
}

func TestAdminRolePermissionPath(t *testing.T) {
	t.Run("CA-031_the_route_for_a_roles_permission_joins_both_IDs", func(t *testing.T) {
		if got := adminRolePermissionPath("5", "2"); got != "/admin/roles/5/permissions/2" {
			t.Errorf("adminRolePermissionPath = %q", got)
		}
	})

	t.Run("CA-032_both_IDs_are_path_escaped", func(t *testing.T) {
		cases := []struct{ name, role, permission, want string }{
			{"slash_and_space", "a/b", "c d", "/admin/roles/a%2Fb/permissions/c%20d"},
			{"question_and_hash", "5", "x?y#z", "/admin/roles/5/permissions/x%3Fy%23z"},
		}
		for _, tc := range cases {
			t.Run("CA-032_"+tc.name, func(t *testing.T) {
				if got := adminRolePermissionPath(tc.role, tc.permission); got != tc.want {
					t.Errorf("adminRolePermissionPath = %q, want %q", got, tc.want)
				}
			})
		}
	})
}

func TestAdminUsername(t *testing.T) {
	t.Run("CA-035_a_username_is_shown_as_it_is", func(t *testing.T) {
		if got := adminUsername("Bob"); got != "Bob" {
			t.Errorf("adminUsername = %q", got)
		}
	})

	t.Run("CA-036_an_account_without_a_username_is_shown_as_No_username", func(t *testing.T) {
		if got := adminUsername(""); got != "No username" {
			t.Errorf("adminUsername = %q", got)
		}
	})
}

func users(n int) []UserAccount { return make([]UserAccount, n) }

func TestUsersSummary(t *testing.T) {
	check := func(t *testing.T, row string, cases []struct {
		name string
		data AdminUsersData
		want string
	}) {
		for _, tc := range cases {
			t.Run(row+"_"+tc.name, func(t *testing.T) {
				if got := usersSummary(tc.data); got != tc.want {
					t.Errorf("usersSummary = %q, want %q", got, tc.want)
				}
			})
		}
	}
	type summary = struct {
		name string
		data AdminUsersData
		want string
	}

	t.Run("CA-057_an_empty_page_after_the_first_one_says_there_are_no_more_users_unless_a_search_stopped_early", func(t *testing.T) {
		check(t, "CA-057", []summary{
			{"plain", AdminUsersData{Start: 200}, "No more users"},
			{"search_reached_the_end", AdminUsersData{Start: 200, Search: "bob"}, "No more users"},
			{"search_stopped_early", AdminUsersData{Start: 200, Search: "bob", More: true, NextOffset: 1000}, "No matches in the first 1000 users"},
		})
	})

	t.Run("CA-058_a_search_with_no_match_that_stopped_before_the_end_says_how_far_it_looked", func(t *testing.T) {
		check(t, "CA-058", []summary{{"stopped_early", AdminUsersData{Search: "zzz", More: true, NextOffset: 1000}, "No matches in the first 1000 users"}})
	})

	t.Run("CA-059_a_search_with_no_match_that_reached_the_end_says_no_matches", func(t *testing.T) {
		check(t, "CA-059", []summary{{"reached_the_end", AdminUsersData{Search: "zzz"}, "No matches"}})
	})

	t.Run("CA-060_an_empty_first_page_with_no_search_says_no_users_were_found", func(t *testing.T) {
		check(t, "CA-060", []summary{{"empty", AdminUsersData{}, "No users found"}})
	})

	t.Run("CA-061_the_first_page_reports_the_count_with_the_singular_for_one_user", func(t *testing.T) {
		check(t, "CA-061", []summary{
			{"one_user", AdminUsersData{Users: users(1)}, "1 user"},
			{"three_users", AdminUsersData{Users: users(3)}, "3 users"},
		})
	})

	t.Run("CA-062_a_later_page_reports_how_many_more_users_it_added", func(t *testing.T) {
		check(t, "CA-062", []summary{
			{"one_more", AdminUsersData{Users: users(1), Start: 200}, "1 more user"},
			{"three_more", AdminUsersData{Users: users(3), Start: 200}, "3 more users"},
		})
	})

	t.Run("CA-063_a_search_that_stopped_before_the_end_adds_how_far_it_looked", func(t *testing.T) {
		check(t, "CA-063", []summary{
			{"first_page", AdminUsersData{Users: users(3), Search: "bob", More: true, NextOffset: 1000}, "3 users. Searched the first 1000 users"},
			{"later_page", AdminUsersData{Users: users(3), Search: "bob", More: true, NextOffset: 1400, Start: 200}, "3 more users. Searched the first 1400 users"},
		})
	})

	t.Run("CA-064_a_search_that_reached_the_end_adds_nothing_after_the_count", func(t *testing.T) {
		check(t, "CA-064", []summary{{"reached_the_end", AdminUsersData{Users: users(3), Search: "bob", NextOffset: 600}, "3 users"}})
	})
}
