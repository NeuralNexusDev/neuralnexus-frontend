package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

const accountLinksJSON = `[
	{"platform":"discord","platform_username":"someone#1234","verified":true,"login_enabled":true},
	{"platform":"twitch","platform_username":"streamer99","verified":false,"login_enabled":false},
	{"platform":"steam","platform_username":"","verified":true,"login_enabled":false}
]`

func seedAccount(f *fakeAdmin) {
	f.on("GET /users/me", 200, `{"username":"testuser"}`)
	f.on("GET /users/me/settings", 200, `{"password_auth":true}`)
	f.on("GET /users/me/links", 200, accountLinksJSON)
}

func TestAccountPageIsAShellThatLoadsItsContent(t *testing.T) {
	f := newFakeAdmin(t)
	rec := getPage("/account")
	assertStatus(t, rec, http.StatusOK)
	assertBody(t, rec, `hx-get="/account/content"`, `hx-trigger="load"`, "htmx.v4.0.0.min.js", `id="admin-error"`, `id="auth-error"`,
		`id="link-discord-oauth-base"`, `id="link-xboxlive-oauth-base"`, "const linkRedirect", "showAuthErrorFromQuery()")
	assertNoBody(t, rec, `id="account-username"`, "loadAccountProfile")
	if len(f.uris()) != 0 {
		t.Errorf("the shell called the API: %v", f.uris())
	}
}

func TestAccountContentShowsProfileSettingAndLinks(t *testing.T) {
	f := newFakeAdmin(t)
	seedAccount(f)
	rec := getPage("/account/content")
	assertStatus(t, rec, http.StatusOK)
	assertBody(t, rec,
		`id="account-username">testuser<`,
		`id="password-auth-enabled" name="password_auth" value="true" checked`,
		`hx-post="/account/settings"`,
		`id="link-discord"`, ">someone#1234<", `hx-delete="/account/links/discord"`, `hx-confirm="Unlink Discord from your account?"`,
		`hx-post="/account/links/discord"`,
		">streamer99<", ">Unverified<",
		`id="link-microsoft-title"`, `data-platform="microsoft"`, `onclick="handleLinkAction(this.dataset.platform)"`,
	)
	assertNoBody(t, rec, "<html")
	if got := strings.Count(rec.Body.String(), `id="link-steam-action"`); got != 1 {
		t.Errorf("steam action appears %d times", got)
	}
	for _, p := range []string{"discord", "twitch", "microsoft", "xboxlive", "steam"} {
		assertBody(t, rec, `id="link-`+p+`-login-enabled"`)
	}
}

// accountRow returns the markup of one platform's row, which runs up to the next platform's row.
func accountRow(body string, platform string) string {
	order := []string{"discord", "twitch", "microsoft", "xboxlive", "steam"}
	start := strings.Index(body, `id="link-`+platform+`" `)
	end := len(body)
	for i, p := range order {
		if p == platform && i+1 < len(order) {
			end = strings.Index(body, `id="link-`+order[i+1]+`" `)
		}
	}
	return body[start:end]
}

func TestAccountContentLinkRowStates(t *testing.T) {
	f := newFakeAdmin(t)
	seedAccount(f)
	body := getPage("/account/content").Body.String()
	cases := []struct {
		platform string
		want     []string
		not      []string
	}{
		{"discord", []string{">someone#1234<", `login_enabled" value="true" checked`, ">Unlink<", `id="link-discord-verified-icon" class="w-4 h-4 shrink-0 text-green-600 dark:text-green-500"`}, []string{"disabled hx-post", ">Unverified<"}},
		{"twitch", []string{">streamer99<", ">Unverified<", "disabled hx-post", ">Unlink<"}, []string{`value="true" checked`}},
		{"microsoft", []string{"Microsoft", ">Link<", "disabled hx-post"}, []string{`value="true" checked`, ">Unlink<", ">Unverified<"}},
		{"steam", []string{">Steam<", ">Unlink<"}, []string{`value="true" checked`, "disabled hx-post", ">Unverified<"}},
	}
	for _, tc := range cases {
		t.Run(tc.platform, func(t *testing.T) {
			row := accountRow(body, tc.platform)
			for _, w := range tc.want {
				if !strings.Contains(row, w) {
					t.Errorf("the row is missing %q:\n%s", w, row)
				}
			}
			for _, n := range tc.not {
				if strings.Contains(row, n) {
					t.Errorf("the row should not contain %q:\n%s", n, row)
				}
			}
		})
	}
}

func TestAccountContentFailures(t *testing.T) {
	t.Run("profile signed out", func(t *testing.T) {
		f := newFakeAdmin(t)
		seedAccount(f)
		f.problem("GET /users/me", 401, "sign in")
		rec := getPage("/account/content")
		if got := rec.Header().Get("HX-Redirect"); got != "/login" {
			t.Errorf("HX-Redirect = %q", got)
		}
	})
	t.Run("profile failed", func(t *testing.T) {
		f := newFakeAdmin(t)
		seedAccount(f)
		f.problem("GET /users/me", 500, "boom")
		rec := getPage("/account/content")
		assertStatus(t, rec, http.StatusInternalServerError)
		if got := bannerText(rec); got != "boom" {
			t.Errorf("body = %q", got)
		}
	})
	t.Run("settings failed", func(t *testing.T) {
		f := newFakeAdmin(t)
		seedAccount(f)
		f.problem("GET /users/me/settings", 500, "settings down")
		rec := getPage("/account/content")
		assertStatus(t, rec, http.StatusOK)
		assertBody(t, rec, `id="account-password-error" role="alert"`, "settings down", `id="link-discord"`, `id="password-auth-enabled" name="password_auth" value="true" disabled`)
		assertNoBody(t, rec, `id="password-auth-enabled" name="password_auth" value="true" checked`)
	})
	t.Run("links failed", func(t *testing.T) {
		f := newFakeAdmin(t)
		seedAccount(f)
		f.problem("GET /users/me/links", 500, "links down")
		rec := getPage("/account/content")
		assertStatus(t, rec, http.StatusOK)
		assertBody(t, rec, `id="account-links-error" role="alert"`, "links down", `id="account-username"`)
		assertNoBody(t, rec, `id="link-discord"`)
	})
	t.Run("settings signed out", func(t *testing.T) {
		f := newFakeAdmin(t)
		seedAccount(f)
		f.problem("GET /users/me/settings", 401, "sign in")
		rec := getPage("/account/content")
		if got := rec.Header().Get("HX-Redirect"); got != "/login" {
			t.Errorf("HX-Redirect = %q", got)
		}
	})
}

func TestAccountContentEscapesAPIText(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("GET /users/me", 200, fmt.Sprintf(`{"username":%q}`, hostile))
	f.on("GET /users/me/settings", 200, `{"password_auth":false}`)
	f.on("GET /users/me/links", 200, fmt.Sprintf(`[{"platform":"discord","platform_username":%q,"verified":true}]`, hostile))
	rec := getPage("/account/content")
	assertNoBody(t, rec, "<img src=x")
	assertBody(t, rec, "&lt;img src=x")
}

func TestAccountSettingsToggleSendsTheCheckboxState(t *testing.T) {
	cases := []struct {
		name string
		form url.Values
		want string
	}{
		{"on", url.Values{"password_auth": {"true"}}, `PATCH /users/me/settings {"password_auth":true}`},
		{"off", url.Values{}, `PATCH /users/me/settings {"password_auth":false}`},
		{"other value", url.Values{"password_auth": {"yes"}}, `PATCH /users/me/settings {"password_auth":false}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeAdmin(t)
			f.on("PATCH /users/me/settings", 204, ``)
			f.on("GET /users/me/settings", 200, `{"password_auth":true}`)
			rec := action(http.MethodPost, "/account/settings", tc.form)
			assertStatus(t, rec, http.StatusOK)
			assertWrites(t, f, tc.want)
			assertBody(t, rec, `id="account-password"`, `<div id="admin-error" hx-swap-oob="innerHTML"></div>`)
			assertNoBody(t, rec, "hx-swap-oob=\"true\"")
		})
	}
}

func TestAccountSettingsToggleRefusedShowsTheMessageAndPutsTheCheckboxBack(t *testing.T) {
	f := newFakeAdmin(t)
	f.problem("PATCH /users/me/settings", 409, "Keep one way to sign in")
	f.on("GET /users/me/settings", 200, `{"password_auth":true}`)
	rec := action(http.MethodPost, "/account/settings", url.Values{})
	assertStatus(t, rec, http.StatusConflict)
	assertBody(t, rec, "Keep one way to sign in", `id="account-password" hx-swap-oob="true"`, `value="true" checked`)
	if got := rec.Header().Get("HX-Retarget"); got != "#admin-error" {
		t.Errorf("HX-Retarget = %q", got)
	}
}

func TestAccountSettingsToggleRefusedWithoutAReloadStillShowsTheMessage(t *testing.T) {
	f := newFakeAdmin(t)
	f.problem("PATCH /users/me/settings", 409, "Keep one way to sign in")
	f.problem("GET /users/me/settings", 500, "down")
	rec := action(http.MethodPost, "/account/settings", url.Values{})
	assertStatus(t, rec, http.StatusConflict)
	if got := bannerText(rec); got != "Keep one way to sign in" {
		t.Errorf("body = %q", got)
	}
	assertNoBody(t, rec, "hx-swap-oob=\"true\"")
}

func TestAccountSettingsToggleReloadFailureIsNotReportedAsAFailedChange(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("PATCH /users/me/settings", 204, ``)
	f.problem("GET /users/me/settings", 500, "down")
	rec := action(http.MethodPost, "/account/settings", url.Values{"password_auth": {"true"}})
	assertStatus(t, rec, http.StatusInternalServerError)
	if got := bannerText(rec); got != "The change was made, but the page could not be refreshed: down" {
		t.Errorf("body = %q", got)
	}
}

func TestAccountSettingsToggleSignedOutRedirects(t *testing.T) {
	f := newFakeAdmin(t)
	f.problem("PATCH /users/me/settings", 401, "sign in")
	rec := action(http.MethodPost, "/account/settings", url.Values{})
	if got := rec.Header().Get("HX-Redirect"); got != "/login" {
		t.Errorf("HX-Redirect = %q", got)
	}
}

func TestAccountLinkToggleSendsTheCheckboxStateAndAnswersWithTheRow(t *testing.T) {
	cases := []struct {
		name string
		form url.Values
		want string
	}{
		{"on", url.Values{"login_enabled": {"true"}}, `PATCH /users/me/link/discord {"login_enabled":true}`},
		{"off", url.Values{}, `PATCH /users/me/link/discord {"login_enabled":false}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeAdmin(t)
			seedAccount(f)
			f.on("PATCH /users/me/link/discord", 204, ``)
			rec := action(http.MethodPost, "/account/links/discord", tc.form)
			assertStatus(t, rec, http.StatusOK)
			assertWrites(t, f, tc.want)
			assertBody(t, rec, `id="link-discord"`, ">someone#1234<")
			assertNoBody(t, rec, `id="link-twitch"`, "hx-swap-oob=\"true\"")
		})
	}
}

func TestAccountUnlinkDeletesTheLinkAndAnswersWithTheUnlinkedRow(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("DELETE /users/me/link/discord", 204, ``)
	f.on("GET /users/me/links", 200, `[]`)
	rec := action(http.MethodDelete, "/account/links/discord", nil)
	assertStatus(t, rec, http.StatusOK)
	assertWrites(t, f, "DELETE /users/me/link/discord ")
	assertBody(t, rec, `id="link-discord"`, `id="link-discord-title" class="text-sm font-medium truncate">`, `data-platform="discord"`)
	assertNoBody(t, rec, "Unlink")
}

func TestAccountLinkChangesRefusedShowTheMessageAndPutTheRowBack(t *testing.T) {
	for _, tc := range []struct{ name, method, target, route string }{
		{"toggle", http.MethodPost, "/account/links/discord", "PATCH /users/me/link/discord"},
		{"unlink", http.MethodDelete, "/account/links/discord", "DELETE /users/me/link/discord"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeAdmin(t)
			seedAccount(f)
			f.problem(tc.route, 409, "Keep one way to sign in")
			rec := action(tc.method, tc.target, url.Values{})
			assertStatus(t, rec, http.StatusConflict)
			assertBody(t, rec, "Keep one way to sign in", `id="link-discord" hx-swap-oob="true"`, ">someone#1234<")
		})
	}
}

func TestAccountLinkChangesRefuseAnUnknownPlatform(t *testing.T) {
	f := newFakeAdmin(t)
	for _, target := range []string{"/account/links/minecraft", "/account/links/..%2Fx", "/account/links/Discord"} {
		t.Run(target, func(t *testing.T) {
			rec := action(http.MethodPost, target, url.Values{})
			if rec.Code != http.StatusNotFound && rec.Code != http.StatusMovedPermanently {
				assertStatus(t, rec, http.StatusNotFound)
			}
			assertWrites(t, f)
		})
	}
}

func TestAccountLinkChangeReloadFailureIsNotReportedAsAFailedChange(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("PATCH /users/me/link/discord", 204, ``)
	f.problem("GET /users/me/links", 500, "down")
	rec := action(http.MethodPost, "/account/links/discord", url.Values{"login_enabled": {"true"}})
	assertStatus(t, rec, http.StatusInternalServerError)
	if got := bannerText(rec); got != "The change was made, but the page could not be refreshed: down" {
		t.Errorf("body = %q", got)
	}
}

func TestAccountChangesNeedHTMX(t *testing.T) {
	f := newFakeAdmin(t)
	for _, tc := range []struct{ method, target string }{
		{http.MethodPost, "/account/settings"},
		{http.MethodPost, "/account/links/discord"},
		{http.MethodDelete, "/account/links/discord"},
	} {
		rec := adminReq{method: tc.method, target: tc.target, form: url.Values{}}.do()
		assertStatus(t, rec, http.StatusForbidden)
	}
	assertWrites(t, f)
}

func TestAccountUnlinkRefusesAnUnknownPlatform(t *testing.T) {
	f := newFakeAdmin(t)
	rec := action(http.MethodDelete, "/account/links/minecraft", nil)
	assertStatus(t, rec, http.StatusNotFound)
	assertWrites(t, f)
}
