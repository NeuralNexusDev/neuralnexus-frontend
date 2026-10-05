package main

import (
	"fmt"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/p0t4t0sandwich/neuralnexus-frontend/components"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/test/testutil"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/test/testutil/fakeapi"
)

const (
	accountTwoLinksJSON = `[{"platform":"discord","platform_username":"Alice_D","platform_id":"111","verified":true,"login_enabled":true},{"platform":"steam","platform_username":"alice_s","platform_id":"222","verified":false,"login_enabled":false}]`
	accountChanged      = "The change was made, but the page could not be refreshed: "
)

var accountPlatformIDs = []string{"discord", "twitch", "microsoft", "xboxlive", "steam"}

func stubAccountAPI(f *fakeapi.FakeAPI) {
	f.On("GET /users/me", 200, `{"username":"alice"}`)
	f.On("GET /users/me/settings", 200, `{"password_auth":true}`)
	f.On("GET /users/me/links", 200, accountTwoLinksJSON)
	f.On("GET /users/me/permissions", 200, `[]`)
}

func assertInputState(t *testing.T, rec *httptest.ResponseRecorder, id string, checked, disabled bool) {
	t.Helper()
	tag := testutil.TagByID(rec.Body.String(), id)
	if tag == "" {
		t.Errorf("the body has no element with id %q", id)
		return
	}
	for attribute, want := range map[string]bool{"checked": checked, "disabled": disabled} {
		if got := regexp.MustCompile(`\s` + attribute + `(\s|/|>|=)`).MatchString(tag); got != want {
			t.Errorf("%s: %s = %t, want %t in %s", id, attribute, got, want, tag)
		}
	}
}

func serveLinkRow(platformID string) *httptest.ResponseRecorder {
	platform, ok := accountPlatform(platformID)
	if !ok {
		panic("unlisted platform " + platformID)
	}
	req := testutil.NewRequest()
	rec := httptest.NewRecorder()
	accountLinkRow(rec, req, newSession(req), platform)
	return rec
}

func TestAccountPlatform(t *testing.T) {
	t.Run("AC-01_each_listed_platform_is_accepted", func(t *testing.T) {
		names := map[string]string{"discord": "Discord", "twitch": "Twitch", "microsoft": "Microsoft", "xboxlive": "Xbox Live", "steam": "Steam"}
		for _, id := range accountPlatformIDs {
			t.Run("AC-01_"+id, func(t *testing.T) {
				got, ok := accountPlatform(id)
				if !ok || got.ID != id || got.Name != names[id] {
					t.Errorf("accountPlatform(%q) = %+v, %t, want %q named %q", id, got, ok, id, names[id])
				}
			})
		}
	})

	t.Run("AC-02_an_unlisted_id_is_refused", func(t *testing.T) {
		for _, id := range []string{"", "minecraft", "Discord", "DISCORD", "discord ", "xbox"} {
			t.Run(fmt.Sprintf("AC-02_%q", id), func(t *testing.T) {
				got, ok := accountPlatform(id)
				if ok || got != (components.AccountPlatform{}) {
					t.Errorf("accountPlatform(%q) = %+v, %t, want the zero value and false", id, got, ok)
				}
			})
		}
	})
}

func TestLoadAccountSettings(t *testing.T) {
	t.Run("AC-03_copies_password_auth_from_a_bodiless_GET", func(t *testing.T) {
		for _, enabled := range []bool{true, false} {
			t.Run(fmt.Sprintf("AC-03_%t", enabled), func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("GET /users/me/settings", 200, fmt.Sprintf(`{"password_auth":%t}`, enabled))
				data, err := loadAccountSettings(newSession(testutil.NewRequest()))
				if err != nil || data.PasswordAuth != enabled || data.Error != "" {
					t.Errorf("data = %+v, err = %v, want PasswordAuth %t and no error", data, err, enabled)
				}
				f.AssertLines(t, "GET /users/me/settings")
			})
		}
	})

	t.Run("AC-04_an_API_failure_returns_an_error_and_empty_data", func(t *testing.T) {
		cases := []struct {
			name    string
			status  int
			detail  string
			message string
		}{
			{"403_with_a_detail", 403, "Missing permission", "Missing permission"},
			{"500_without_a_detail", 500, "", "Failed to load account settings"},
		}
		for _, tc := range cases {
			t.Run("AC-04_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.Refuse("GET /users/me/settings", tc.status, tc.detail)
				data, err := loadAccountSettings(newSession(testutil.NewRequest()))
				assertAPIError(t, err, tc.status, tc.message, "GET", "/users/me/settings")
				if data != (components.AccountSettingsData{}) {
					t.Errorf("data = %+v, want the zero value", data)
				}
			})
		}
	})

	t.Run("AC-05_a_401_returns_the_sign-in_sentinel", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /users/me/settings", 401, ``)
		data, err := loadAccountSettings(newSession(testutil.NewRequest()))
		if err != errUnauthorized {
			t.Errorf("err = %v, want errUnauthorized", err)
		}
		if data != (components.AccountSettingsData{}) {
			t.Errorf("data = %+v, want the zero value", data)
		}
	})
}

func TestLoadAccountLinks(t *testing.T) {
	t.Run("AC-06_the_links_come_back_keyed_by_platform_from_a_bodiless_GET", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /users/me/links", 200, accountTwoLinksJSON)
		links, err := loadAccountLinks(newSession(testutil.NewRequest()))
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		want := map[string]components.LinkedAccount{
			"discord": {Platform: "discord", PlatformUsername: "Alice_D", PlatformID: "111", Verified: true, LoginEnabled: true},
			"steam":   {Platform: "steam", PlatformUsername: "alice_s", PlatformID: "222"},
		}
		if len(links) != len(want) || links["discord"] != want["discord"] || links["steam"] != want["steam"] {
			t.Errorf("links = %+v, want %+v", links, want)
		}
		f.AssertLines(t, "GET /users/me/links")
	})

	t.Run("AC-07_an_empty_array_and_null_give_an_empty_map", func(t *testing.T) {
		for _, body := range []string{`[]`, `null`} {
			t.Run("AC-07_"+body, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("GET /users/me/links", 200, body)
				links, err := loadAccountLinks(newSession(testutil.NewRequest()))
				if err != nil || len(links) != 0 {
					t.Errorf("links = %+v, err = %v, want an empty map and no error", links, err)
				}
			})
		}
	})

	t.Run("AC-08_an_API_failure_returns_an_error_and_an_empty_map", func(t *testing.T) {
		cases := []struct {
			name    string
			detail  string
			message string
		}{
			{"with_a_detail", "database down", "database down"},
			{"without_a_detail", "", "Failed to load linked accounts"},
		}
		for _, tc := range cases {
			t.Run("AC-08_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.Refuse("GET /users/me/links", 500, tc.detail)
				links, err := loadAccountLinks(newSession(testutil.NewRequest()))
				assertAPIError(t, err, 500, tc.message, "GET", "/users/me/links")
				if len(links) != 0 {
					t.Errorf("links = %+v, want an empty map", links)
				}
			})
		}
	})

	t.Run("AC-09_a_401_returns_the_sign-in_sentinel", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /users/me/links", 401, ``)
		links, err := loadAccountLinks(newSession(testutil.NewRequest()))
		if err != errUnauthorized {
			t.Errorf("err = %v, want errUnauthorized", err)
		}
		if len(links) != 0 {
			t.Errorf("links = %+v, want an empty map", links)
		}
	})
}

func TestAccountContentHandler(t *testing.T) {
	const content = "/account/content"
	readCalls := []string{"GET /users/me", "GET /users/me/settings", "GET /users/me/links", "GET /users/me/permissions"}

	t.Run("AC-10_calls_the_four_read_endpoints_in_order_and_renders_the_page", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		stubAccountAPI(f)
		rec := serveRequest("GET", content, nil)
		assertStatusCode(t, rec, 200)
		f.AssertLines(t, readCalls...)
		assertBodyHas(t, rec, "alice", `id="link-discord"`, `id="link-twitch"`, `id="link-microsoft"`, `id="link-xboxlive"`, `id="link-steam"`, "Alice_D")
		assertInputState(t, rec, "password-auth-enabled", true, false)
		assertBodyLacks(t, rec, "admin-dashboard-link", "account-password-error", "account-links-error")
	})

	t.Run("AC-11_each_platform_row_shows_the_state_of_its_own_link", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		stubAccountAPI(f)
		f.On("GET /users/me/links", 200, accountTwoLinksJSON)
		rec := serveRequest("GET", content, nil)
		assertInputState(t, rec, "link-discord-login-enabled", true, false)
		assertInputState(t, rec, "link-steam-login-enabled", false, true)
		assertInputState(t, rec, "link-twitch-login-enabled", false, true)
		assertBodyHas(t, rec, `hx-delete="/account/links/discord"`, `hx-delete="/account/links/steam"`, "Unverified")
		assertBodyLacks(t, rec, `hx-delete="/account/links/twitch"`)
	})

	t.Run("AC-12_a_password_auth_value_of_false_renders_the_toggle_unchecked", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		stubAccountAPI(f)
		f.On("GET /users/me/settings", 200, `{"password_auth":false}`)
		rec := serveRequest("GET", content, nil)
		assertStatusCode(t, rec, 200)
		assertInputState(t, rec, "password-auth-enabled", false, false)
	})

	t.Run("AC-13_either_admin_permission_shows_the_admin_dashboard_link", func(t *testing.T) {
		answers := []string{`["users.admin"]`, `["roles.admin"]`, `["pets.read","users.admin","roles.admin"]`}
		for i, answer := range answers {
			t.Run(fmt.Sprintf("AC-13_%d", i+1), func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				stubAccountAPI(f)
				f.On("GET /users/me/permissions", 200, answer)
				rec := serveRequest("GET", content, nil)
				assertStatusCode(t, rec, 200)
				assertBodyHas(t, rec, "admin-dashboard-link")
			})
		}
	})

	t.Run("AC-14_other_permissions_do_not_show_the_admin_dashboard_link", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		stubAccountAPI(f)
		f.On("GET /users/me/permissions", 200, `["users.read","roles.read","pets.admin"]`)
		rec := serveRequest("GET", content, nil)
		assertStatusCode(t, rec, 200)
		assertBodyLacks(t, rec, "admin-dashboard-link")
	})

	t.Run("AC-15_a_failed_permissions_call_leaves_the_page_whole_without_the_admin_link", func(t *testing.T) {
		cases := []struct {
			name   string
			status int
			detail string
		}{
			{"403", 403, "Missing permission"},
			{"500", 500, ""},
			{"401", 401, ""},
		}
		for _, tc := range cases {
			t.Run("AC-15_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				stubAccountAPI(f)
				f.Refuse("GET /users/me/permissions", tc.status, tc.detail)
				rec := serveRequest("GET", content, nil)
				assertStatusCode(t, rec, 200)
				assertBodyHas(t, rec, "alice", "link-discord")
				assertBodyLacks(t, rec, "admin-dashboard-link")
				if got := rec.Header().Get("HX-Redirect"); got != "" {
					t.Errorf("HX-Redirect = %q, want none", got)
				}
			})
		}
	})

	t.Run("AC-16_a_failed_profile_call_shows_its_message_and_makes_no_other_call", func(t *testing.T) {
		cases := []struct {
			name    string
			status  int
			detail  string
			message string
		}{
			{"403_with_a_detail", 403, "Not allowed", "Not allowed"},
			{"500_without_a_detail", 500, "", "Failed to load your account"},
		}
		for _, tc := range cases {
			t.Run("AC-16_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				stubAccountAPI(f)
				f.Refuse("GET /users/me", tc.status, tc.detail)
				rec := serveRequest("GET", content, nil)
				assertFailure(t, rec, tc.status, tc.message)
				if got := rec.Body.String(); got != tc.message {
					t.Errorf("body = %q, want exactly %q", got, tc.message)
				}
				f.AssertLines(t, "GET /users/me")
			})
		}
	})

	t.Run("AC-17_a_401_on_the_profile_settings_or_links_call_sends_the_browser_to_the_login_page", func(t *testing.T) {
		for i, route := range []string{"GET /users/me", "GET /users/me/settings", "GET /users/me/links"} {
			t.Run("AC-17_"+route, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				stubAccountAPI(f)
				f.On(route, 401, ``)
				assertLoginRedirect(t, serveRequest("GET", content, nil))
				f.AssertLines(t, readCalls[:i+1]...)
			})
		}
	})

	t.Run("AC-18_a_failed_settings_call_shows_its_message_in_the_password_section", func(t *testing.T) {
		cases := []struct {
			name    string
			detail  string
			message string
		}{
			{"with_a_detail", "database down", "database down"},
			{"without_a_detail", "", "Failed to load account settings"},
		}
		for _, tc := range cases {
			t.Run("AC-18_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				stubAccountAPI(f)
				f.Refuse("GET /users/me/settings", 500, tc.detail)
				rec := serveRequest("GET", content, nil)
				assertStatusCode(t, rec, 200)
				assertBodyHas(t, rec, "account-password-error", tc.message, "alice", "link-discord")
				assertInputState(t, rec, "password-auth-enabled", false, true)
				f.AssertLines(t, readCalls...)
			})
		}
	})

	t.Run("AC-19_a_failed_links_call_shows_its_message_in_place_of_the_connected_accounts", func(t *testing.T) {
		cases := []struct {
			name    string
			detail  string
			message string
		}{
			{"with_a_detail", "database down", "database down"},
			{"without_a_detail", "", "Failed to load linked accounts"},
		}
		for _, tc := range cases {
			t.Run("AC-19_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				stubAccountAPI(f)
				f.Refuse("GET /users/me/links", 500, tc.detail)
				f.On("GET /users/me/permissions", 200, `["users.admin"]`)
				rec := serveRequest("GET", content, nil)
				assertStatusCode(t, rec, 200)
				assertBodyHas(t, rec, "account-links-error", tc.message, "alice", "admin-dashboard-link")
				assertBodyLacks(t, rec, "link-discord", `id="account-links"`)
				f.AssertLines(t, readCalls...)
			})
		}
	})

	t.Run("AC-20_failed_settings_and_links_calls_show_both_messages", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		stubAccountAPI(f)
		f.Problem("GET /users/me/settings", 500, "settings down")
		f.Problem("GET /users/me/links", 500, "links down")
		rec := serveRequest("GET", content, nil)
		assertStatusCode(t, rec, 200)
		assertBodyHas(t, rec, "account-password-error", "settings down", "account-links-error", "links down", "alice")
	})

	t.Run("AC-21_a_link_for_an_unlisted_platform_gets_no_row", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		stubAccountAPI(f)
		f.On("GET /users/me/links", 200, `[{"platform":"minecraft","platform_username":"Alice_M","platform_id":"333","verified":true,"login_enabled":true}]`)
		rec := serveRequest("GET", content, nil)
		assertStatusCode(t, rec, 200)
		assertBodyLacks(t, rec, "link-minecraft", "Alice_M")
		for _, id := range accountPlatformIDs {
			assertBodyHas(t, rec, `id="link-`+id+`"`)
		}
	})
}

func TestAccountSettingsHandler(t *testing.T) {
	const settings = "/account/settings"
	const patch = "PATCH /users/me/settings "

	t.Run("AC-22_turning_password_login_on_sends_a_PATCH_then_reloads", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("PATCH /users/me/settings", 200, `{}`)
		f.On("GET /users/me/settings", 200, `{"password_auth":true}`)
		rec := serveRequest("POST", settings, url.Values{"password_auth": {"true"}})
		assertStatusCode(t, rec, 200)
		f.AssertLines(t, patch+`{"password_auth":true}`, "GET /users/me/settings")
		fakeapi.AssertContentType(t, f.Calls()[0], "application/json")
		assertInputState(t, rec, "password-auth-enabled", true, false)
		if tag := testutil.TagByID(rec.Body.String(), "account-password"); tag == "" || strings.Contains(tag, `hx-swap-oob="true"`) {
			t.Errorf("account-password tag = %q, want one without hx-swap-oob", tag)
		}
	})

	t.Run("AC-23_only_the_form_value_true_turns_password_login_on", func(t *testing.T) {
		forms := []struct {
			name string
			form url.Values
		}{
			{"no_field", url.Values{}},
			{"false", url.Values{"password_auth": {"false"}}},
			{"True", url.Values{"password_auth": {"True"}}},
			{"1", url.Values{"password_auth": {"1"}}},
			{"on", url.Values{"password_auth": {"on"}}},
			{"empty", url.Values{"password_auth": {""}}},
		}
		for _, tc := range forms {
			t.Run("AC-23_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("PATCH /users/me/settings", 200, `{}`)
				f.On("GET /users/me/settings", 200, `{"password_auth":false}`)
				serveRequest("POST", settings, tc.form)
				f.AssertLines(t, patch+`{"password_auth":false}`, "GET /users/me/settings")
			})
		}
	})

	t.Run("AC-24_the_rendered_setting_comes_from_the_reload_not_the_form", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("PATCH /users/me/settings", 200, `{}`)
		f.On("GET /users/me/settings", 200, `{"password_auth":false}`)
		rec := serveRequest("POST", settings, url.Values{"password_auth": {"true"}})
		assertStatusCode(t, rec, 200)
		assertInputState(t, rec, "password-auth-enabled", false, false)
	})

	t.Run("AC-25_a_refused_change_shows_the_message_and_restores_the_toggle", func(t *testing.T) {
		cases := []struct {
			name        string
			status      int
			detail      string
			message     string
			form        url.Values
			wantChecked bool
			wantBody    string
		}{
			{"403_after_turning_it_on", 403, "Missing permission", "Missing permission", url.Values{"password_auth": {"true"}}, false, patch + `{"password_auth":true}`},
			{"500_after_turning_it_off", 500, "", "Failed to update account settings", url.Values{}, true, patch + `{"password_auth":false}`},
		}
		for _, tc := range cases {
			t.Run("AC-25_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.Refuse("PATCH /users/me/settings", tc.status, tc.detail)
				rec := serveRequest("POST", settings, tc.form)
				assertFailure(t, rec, tc.status, tc.message)
				if !strings.Contains(testutil.TagByID(rec.Body.String(), "account-password"), `hx-swap-oob="true"`) {
					t.Error("account-password has no hx-swap-oob")
				}
				assertInputState(t, rec, "password-auth-enabled", tc.wantChecked, false)
				f.AssertLines(t, tc.wantBody)
			})
		}
	})

	t.Run("AC-26_a_401_on_the_PATCH_sends_the_browser_to_the_login_page", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("PATCH /users/me/settings", 401, ``)
		assertLoginRedirect(t, serveRequest("POST", settings, url.Values{"password_auth": {"true"}}))
		f.AssertLines(t, patch+`{"password_auth":true}`)
	})

	t.Run("AC-27_a_failed_reload_after_a_change_reports_it", func(t *testing.T) {
		cases := []struct {
			name    string
			detail  string
			message string
		}{
			{"with_a_detail", "database down", accountChanged + "database down"},
			{"without_a_detail", "", accountChanged + "Failed to load account settings"},
		}
		for _, tc := range cases {
			t.Run("AC-27_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("PATCH /users/me/settings", 200, `{}`)
				f.Refuse("GET /users/me/settings", 500, tc.detail)
				rec := serveRequest("POST", settings, url.Values{"password_auth": {"true"}})
				assertFailure(t, rec, 500, tc.message)
				if got := rec.Body.String(); got != tc.message {
					t.Errorf("body = %q, want exactly %q", got, tc.message)
				}
				f.AssertLines(t, patch+`{"password_auth":true}`, "GET /users/me/settings")
			})
		}
	})

	t.Run("AC-28_a_401_on_the_reload_sends_the_browser_to_the_login_page", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("PATCH /users/me/settings", 200, `{}`)
		f.On("GET /users/me/settings", 401, ``)
		rec := serveRequest("POST", settings, url.Values{"password_auth": {"true"}})
		assertLoginRedirect(t, rec)
		assertBodyLacks(t, rec, "The change was made")
	})
}

func TestAccountLinkRow(t *testing.T) {
	t.Run("AC-29_a_linked_platform_renders_its_row_from_the_links_the_API_returns", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /users/me/links", 200, accountTwoLinksJSON)
		rec := serveLinkRow("discord")
		assertStatusCode(t, rec, 200)
		if got := rec.Header().Get("HX-Retarget"); got != "" {
			t.Errorf("HX-Retarget = %q, want none", got)
		}
		assertBodyHas(t, rec, `id="link-discord"`, "Alice_D", `hx-delete="/account/links/discord"`)
		assertInputState(t, rec, "link-discord-login-enabled", true, false)
		if tag := testutil.TagByID(rec.Body.String(), "link-discord"); strings.Contains(tag, `hx-swap-oob="true"`) {
			t.Errorf("link-discord tag = %q, want one without hx-swap-oob", tag)
		}
		f.AssertLines(t, "GET /users/me/links")
	})

	t.Run("AC-30_an_unlinked_platform_renders_unlinked_and_another_link_is_not_shown", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /users/me/links", 200, `[{"platform":"steam","platform_username":"alice_s","platform_id":"222","verified":true,"login_enabled":true}]`)
		rec := serveLinkRow("discord")
		assertBodyHas(t, rec, `id="link-discord"`, `data-platform="discord"`)
		assertBodyLacks(t, rec, "hx-delete", "alice_s", "link-steam")
		assertInputState(t, rec, "link-discord-login-enabled", false, true)
	})

	t.Run("AC-31_a_failed_reload_reports_that_the_change_was_made", func(t *testing.T) {
		cases := []struct {
			name    string
			detail  string
			message string
		}{
			{"with_a_detail", "database down", accountChanged + "database down"},
			{"without_a_detail", "", accountChanged + "Failed to load linked accounts"},
		}
		for _, tc := range cases {
			t.Run("AC-31_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.Refuse("GET /users/me/links", 500, tc.detail)
				rec := serveLinkRow("discord")
				assertStatusCode(t, rec, 500)
				if got := rec.Body.String(); got != tc.message {
					t.Errorf("body = %q, want exactly %q", got, tc.message)
				}
			})
		}
	})

	t.Run("AC-32_a_401_on_the_reload_sends_the_browser_to_the_login_page", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /users/me/links", 401, ``)
		assertLoginRedirect(t, serveLinkRow("discord"))
	})
}

func TestAccountLinkHandler(t *testing.T) {
	const reloaded = "GET /users/me/links"

	t.Run("AC-33_turning_logins_on_sends_a_PATCH_then_reloads_that_row", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("PATCH /users/me/link/discord", 200, `{}`)
		f.On(reloaded, 200, testutil.JSONString(t, []components.LinkedAccount{{Platform: "discord", PlatformUsername: "Alice_D", PlatformID: "111", Verified: true, LoginEnabled: true}}))
		rec := serveRequest("POST", "/account/links/discord", url.Values{"login_enabled": {"true"}})
		assertStatusCode(t, rec, 200)
		f.AssertLines(t, `PATCH /users/me/link/discord {"login_enabled":true}`, reloaded)
		fakeapi.AssertContentType(t, f.Calls()[0], "application/json")
		assertBodyHas(t, rec, `id="link-discord"`, "Alice_D")
		assertInputState(t, rec, "link-discord-login-enabled", true, false)
	})

	t.Run("AC-34_only_the_form_value_true_turns_logins_on", func(t *testing.T) {
		forms := []struct {
			name string
			form url.Values
		}{
			{"no_field", url.Values{}},
			{"false", url.Values{"login_enabled": {"false"}}},
			{"True", url.Values{"login_enabled": {"True"}}},
			{"1", url.Values{"login_enabled": {"1"}}},
			{"on", url.Values{"login_enabled": {"on"}}},
			{"empty", url.Values{"login_enabled": {""}}},
		}
		for _, tc := range forms {
			t.Run("AC-34_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("PATCH /users/me/link/discord", 200, `{}`)
				f.On(reloaded, 200, testutil.JSONString(t, []components.LinkedAccount{{Platform: "discord", PlatformID: "111", Verified: true}}))
				serveRequest("POST", "/account/links/discord", tc.form)
				f.AssertLines(t, `PATCH /users/me/link/discord {"login_enabled":false}`, reloaded)
			})
		}
	})

	t.Run("AC-35_each_listed_platform_is_sent_to_its_own_path_and_renders_its_own_row", func(t *testing.T) {
		for _, id := range accountPlatformIDs {
			t.Run("AC-35_"+id, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("PATCH /users/me/link/"+id, 200, `{}`)
				f.On(reloaded, 200, `[]`)
				rec := serveRequest("POST", "/account/links/"+id, url.Values{"login_enabled": {"true"}})
				assertStatusCode(t, rec, 200)
				f.AssertLines(t, "PATCH /users/me/link/"+id+` {"login_enabled":true}`, reloaded)
				assertBodyHas(t, rec, `id="link-`+id+`"`)
			})
		}
	})

	t.Run("AC-36_an_unlisted_platform_is_refused_before_any_API_call", func(t *testing.T) {
		for _, id := range []string{"nope", "minecraft", "Discord", "discord%20"} {
			t.Run("AC-36_"+id, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				rec := serveRequest("POST", "/account/links/"+id, url.Values{"login_enabled": {"true"}})
				assertFailure(t, rec, 404, "Unknown platform")
				if got := rec.Body.String(); got != "Unknown platform" {
					t.Errorf("body = %q, want exactly Unknown platform", got)
				}
				f.AssertLines(t)
			})
		}
	})

	t.Run("AC-37_a_refused_change_shows_the_message_and_restores_the_login_checkbox", func(t *testing.T) {
		cases := []struct {
			name        string
			status      int
			detail      string
			message     string
			form        url.Values
			wantChecked bool
			wantCall    string
		}{
			{"403_after_turning_it_on", 403, "Missing permission", "Missing permission", url.Values{"login_enabled": {"true"}}, false, `PATCH /users/me/link/discord {"login_enabled":true}`},
			{"404_after_turning_it_on", 404, "Link not found", "Link not found", url.Values{"login_enabled": {"true"}}, false, `PATCH /users/me/link/discord {"login_enabled":true}`},
			{"500_after_turning_it_off", 500, "", "Failed to update platform", url.Values{}, true, `PATCH /users/me/link/discord {"login_enabled":false}`},
		}
		for _, tc := range cases {
			t.Run("AC-37_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.Refuse("PATCH /users/me/link/discord", tc.status, tc.detail)
				rec := serveRequest("POST", "/account/links/discord", tc.form)
				assertFailure(t, rec, tc.status, tc.message)
				if !strings.Contains(testutil.TagByID(rec.Body.String(), "link-discord-login-enabled"), `hx-swap-oob="true"`) {
					t.Error("link-discord-login-enabled has no hx-swap-oob")
				}
				assertInputState(t, rec, "link-discord-login-enabled", tc.wantChecked, false)
				f.AssertLines(t, tc.wantCall)
			})
		}
	})

	t.Run("AC-38_a_401_on_the_PATCH_sends_the_browser_to_the_login_page", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("PATCH /users/me/link/discord", 401, ``)
		assertLoginRedirect(t, serveRequest("POST", "/account/links/discord", url.Values{"login_enabled": {"true"}}))
		f.AssertLines(t, `PATCH /users/me/link/discord {"login_enabled":true}`)
	})

	t.Run("AC-39_a_failed_reload_after_a_change_reports_it", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("PATCH /users/me/link/discord", 200, `{}`)
		f.Problem(reloaded, 500, "database down")
		rec := serveRequest("POST", "/account/links/discord", url.Values{"login_enabled": {"true"}})
		assertStatusCode(t, rec, 500)
		if got := rec.Body.String(); got != accountChanged+"database down" {
			t.Errorf("body = %q, want the change-made message", got)
		}
		f.AssertLines(t, `PATCH /users/me/link/discord {"login_enabled":true}`, reloaded)
	})

	t.Run("AC-40_a_401_on_the_reload_sends_the_browser_to_the_login_page", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("PATCH /users/me/link/discord", 200, `{}`)
		f.On(reloaded, 401, ``)
		rec := serveRequest("POST", "/account/links/discord", url.Values{"login_enabled": {"true"}})
		assertLoginRedirect(t, rec)
		assertBodyLacks(t, rec, "The change was made")
	})
}

func TestAccountUnlinkHandler(t *testing.T) {
	const reloaded = "GET /users/me/links"

	t.Run("AC-41_an_unlink_sends_a_bodiless_DELETE_then_reloads_and_renders_the_row_unlinked", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("DELETE /users/me/link/discord", 204, ``)
		f.On(reloaded, 200, `[{"platform":"steam","platform_username":"alice_s","platform_id":"222","verified":true,"login_enabled":true}]`)
		rec := serveRequest("DELETE", "/account/links/discord", nil)
		assertStatusCode(t, rec, 200)
		f.AssertLines(t, "DELETE /users/me/link/discord", reloaded)
		fakeapi.AssertContentType(t, f.Calls()[0], "")
		assertBodyHas(t, rec, `id="link-discord"`, `data-platform="discord"`)
		assertBodyLacks(t, rec, `hx-delete="/account/links/discord"`)
	})

	t.Run("AC-42_each_listed_platform_is_sent_to_its_own_path_and_renders_its_own_row", func(t *testing.T) {
		for _, id := range accountPlatformIDs {
			t.Run("AC-42_"+id, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("DELETE /users/me/link/"+id, 204, ``)
				f.On(reloaded, 200, `[]`)
				rec := serveRequest("DELETE", "/account/links/"+id, nil)
				assertStatusCode(t, rec, 200)
				f.AssertLines(t, "DELETE /users/me/link/"+id, reloaded)
				assertBodyHas(t, rec, `id="link-`+id+`"`)
			})
		}
	})

	t.Run("AC-43_an_unlisted_platform_is_refused_before_any_API_call", func(t *testing.T) {
		for _, id := range []string{"nope", "minecraft", "Discord", "discord%20"} {
			t.Run("AC-43_"+id, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				rec := serveRequest("DELETE", "/account/links/"+id, nil)
				assertFailure(t, rec, 404, "Unknown platform")
				if got := rec.Body.String(); got != "Unknown platform" {
					t.Errorf("body = %q, want exactly Unknown platform", got)
				}
				f.AssertLines(t)
			})
		}
	})

	t.Run("AC-44_a_refused_unlink_shows_the_message_and_does_not_reload", func(t *testing.T) {
		cases := []struct {
			name    string
			status  int
			detail  string
			message string
		}{
			{"404", 404, "Link not found", "Link not found"},
			{"403", 403, "Missing permission", "Missing permission"},
			{"500_without_a_detail", 500, "", "Failed to unlink platform"},
		}
		for _, tc := range cases {
			t.Run("AC-44_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.Refuse("DELETE /users/me/link/discord", tc.status, tc.detail)
				rec := serveRequest("DELETE", "/account/links/discord", nil)
				assertFailure(t, rec, tc.status, tc.message)
				if got := rec.Body.String(); got != tc.message {
					t.Errorf("body = %q, want exactly %q", got, tc.message)
				}
				f.AssertLines(t, "DELETE /users/me/link/discord")
			})
		}
	})

	t.Run("AC-45_a_401_on_the_DELETE_sends_the_browser_to_the_login_page", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("DELETE /users/me/link/discord", 401, ``)
		assertLoginRedirect(t, serveRequest("DELETE", "/account/links/discord", nil))
		f.AssertLines(t, "DELETE /users/me/link/discord")
	})

	t.Run("AC-46_a_failed_reload_after_an_unlink_reports_it", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("DELETE /users/me/link/discord", 204, ``)
		f.Problem(reloaded, 500, "database down")
		rec := serveRequest("DELETE", "/account/links/discord", nil)
		assertStatusCode(t, rec, 500)
		if got := rec.Body.String(); got != accountChanged+"database down" {
			t.Errorf("body = %q, want the change-made message", got)
		}
		f.AssertLines(t, "DELETE /users/me/link/discord", reloaded)
	})

	t.Run("AC-47_a_401_on_the_reload_after_an_unlink_sends_the_browser_to_the_login_page", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("DELETE /users/me/link/discord", 204, ``)
		f.On(reloaded, 401, ``)
		rec := serveRequest("DELETE", "/account/links/discord", nil)
		assertLoginRedirect(t, rec)
		assertBodyLacks(t, rec, "The change was made")
	})
}
