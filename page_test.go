package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/components"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/test/testutil"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/test/testutil/fakeapi"
)

func serveHandler(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func textComponent(text string) templ.Component {
	return templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		_, err := io.WriteString(w, text)
		return err
	})
}

func failingComponent() templ.Component {
	return templ.ComponentFunc(func(context.Context, io.Writer) error {
		return errors.New("render failed")
	})
}

func TestNoStoreHandler(t *testing.T) {
	t.Run("PG-01 the header is set before the next handler runs and the next handler is called", func(t *testing.T) {
		var during string
		next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			during = w.Header().Get("Cache-Control")
			w.WriteHeader(http.StatusTeapot)
		})
		rec := serveHandler(noStoreHandler(next), testutil.NewRequest())
		if during != noStore {
			t.Errorf("Cache-Control seen by the next handler = %q, want %q", during, noStore)
		}
		assertStatusCode(t, rec, http.StatusTeapot)
		if got := rec.Header().Get("Cache-Control"); got != noStore {
			t.Errorf("Cache-Control = %q, want %q", got, noStore)
		}
	})
}

func TestPageRouteAndAction(t *testing.T) {
	t.Run("PG-02 a route answers with Cache-Control no-store when its handler succeeds", func(t *testing.T) {
		rec := serveHandler(pageRoute(func(w http.ResponseWriter, _ *http.Request, _ apiSession) error {
			renderAll(w, testutil.NewRequest(), textComponent("ok"))
			return nil
		}), testutil.NewRequest())
		assertStatusCode(t, rec, http.StatusOK)
		if got := rec.Header().Get("Cache-Control"); got != noStore {
			t.Errorf("Cache-Control = %q, want %q", got, noStore)
		}
	})

	t.Run("PG-03 an action hands its handler the form from the body and the query, and a DELETE only the query", func(t *testing.T) {
		var got url.Values
		h := pageAction(func(_ http.ResponseWriter, r *http.Request, _ apiSession) error {
			got = r.Form
			return nil
		})
		serve := func(method string) {
			req := httptest.NewRequest(method, "/x?from=query&both=q", strings.NewReader("from_body=1&both=b"))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("HX-Request", "true")
			serveHandler(h, req)
		}
		serve(http.MethodPost)
		for key, want := range map[string]string{"from": "query", "from_body": "1", "both": "b"} {
			if got.Get(key) != want {
				t.Errorf("POST form %s = %q, want %q (form %v)", key, got.Get(key), want, got)
			}
		}
		serve(http.MethodDelete)
		for key, want := range map[string]string{"from": "query", "from_body": "", "both": "q"} {
			if got.Get(key) != want {
				t.Errorf("DELETE form %s = %q, want %q (form %v)", key, got.Get(key), want, got)
			}
		}
	})

	t.Run("PG-04 an action without HX-Request answers a plain 403 and no fragment", func(t *testing.T) {
		rec := serveHandler(pageAction(func(http.ResponseWriter, *http.Request, apiSession) error { return nil }),
			httptest.NewRequest(http.MethodPost, "/x", nil))
		assertStatusCode(t, rec, http.StatusForbidden)
		if got := rec.Body.String(); got != "Forbidden\n" {
			t.Errorf("body = %q, want %q", got, "Forbidden\n")
		}
		if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/plain") {
			t.Errorf("Content-Type = %q, want text/plain", got)
		}
		if rec.Header().Get("HX-Retarget") != "" || rec.Header().Get("HX-Reswap") != "" {
			t.Errorf("headers = %v, want no retarget or reswap", rec.Header())
		}
	})
}

func TestRequireHTMXForWrites(t *testing.T) {
	newMux := func() *http.ServeMux {
		mux := http.NewServeMux()
		noContent := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }
		mux.HandleFunc("POST /unlisted", noContent)
		mux.HandleFunc("GET /unlisted", noContent)
		mux.HandleFunc("DELETE /removable", noContent)
		mux.HandleFunc("GET /only", noContent)
		mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusAccepted) })
		return mux
	}
	serve := func(method, target string, headers map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, target, nil)
		for header, value := range headers {
			req.Header.Set(header, value)
		}
		return serveHandler(requireHTMXForWrites(newMux()), req)
	}

	t.Run("PG-05 reads reach the route, also from another site and without HX-Request", func(t *testing.T) {
		cases := []struct {
			name, method string
			headers      map[string]string
			want         int
		}{
			{"get", http.MethodGet, nil, http.StatusNoContent},
			{"cross-site get", http.MethodGet, map[string]string{"Sec-Fetch-Site": "cross-site"}, http.StatusNoContent},
			{"head", http.MethodHead, nil, http.StatusNoContent},
			{"options", http.MethodOptions, nil, http.StatusAccepted},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				assertStatusCode(t, serve(tc.method, "/unlisted", tc.headers), tc.want)
			})
		}
	})

	t.Run("PG-06 a write without HX-Request true is refused with a plain 403", func(t *testing.T) {
		for name, headers := range map[string]map[string]string{
			"no header":     nil,
			"another value": {"HX-Request": "false"},
			"empty value":   {"HX-Request": ""},
			"same-origin":   {"Sec-Fetch-Site": "same-origin"},
			"address bar":   {"Sec-Fetch-Site": "none"},
		} {
			t.Run(name, func(t *testing.T) {
				rec := serve(http.MethodPost, "/unlisted", headers)
				assertStatusCode(t, rec, http.StatusForbidden)
				if got := rec.Body.String(); got != "Forbidden\n" {
					t.Errorf("body = %q, want %q", got, "Forbidden\n")
				}
			})
		}
		assertStatusCode(t, serve(http.MethodDelete, "/removable", nil), http.StatusForbidden)
	})

	t.Run("PG-07 a write from another site is refused even with HX-Request", func(t *testing.T) {
		for _, site := range []string{"cross-site", "same-site"} {
			t.Run(site, func(t *testing.T) {
				rec := serve(http.MethodPost, "/unlisted", map[string]string{"HX-Request": "true", "Sec-Fetch-Site": site})
				assertStatusCode(t, rec, http.StatusForbidden)
			})
		}
	})

	t.Run("PG-08 a write with HX-Request from the same origin or the address bar reaches the route", func(t *testing.T) {
		for _, site := range []string{"", "same-origin", "none"} {
			t.Run("site "+site, func(t *testing.T) {
				headers := map[string]string{"HX-Request": "true"}
				if site != "" {
					headers["Sec-Fetch-Site"] = site
				}
				assertStatusCode(t, serve(http.MethodPost, "/unlisted", headers), http.StatusNoContent)
			})
		}
	})

	t.Run("PG-09 a route that falls to the catch-all is not checked", func(t *testing.T) {
		assertStatusCode(t, serve(http.MethodPost, "/anywhere", nil), http.StatusAccepted)
		assertStatusCode(t, serve(http.MethodDelete, "/unlisted", nil), http.StatusAccepted)
	})

	t.Run("PG-10 a write to a path whose route does not list the method keeps the mux answer", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /only", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
		rec := serveHandler(requireHTMXForWrites(mux), httptest.NewRequest(http.MethodPost, "/only", nil))
		assertStatusCode(t, rec, http.StatusMethodNotAllowed)
	})
}

func TestShells(t *testing.T) {
	t.Run("PG-11 a shell renders its page with no-store and calls no API", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		rec := serveHandler(shell(components.AdminDashboardPage()), testutil.NewRequest())
		assertStatusCode(t, rec, http.StatusOK)
		assertBodyHas(t, rec, `hx-get="/admin/cards"`)
		if got := rec.Header().Get("Cache-Control"); got != noStore {
			t.Errorf("Cache-Control = %q, want %q", got, noStore)
		}
		f.AssertLines(t)
	})

	t.Run("PG-12 shellFor passes the decoded id from the path to the page and sets no-store", func(t *testing.T) {
		var seen string
		mux := http.NewServeMux()
		mux.Handle("GET /x/{id}", shellFor(func(id string) templ.Component {
			seen = id
			return textComponent(id)
		}))
		rec := serveHandler(mux, httptest.NewRequest(http.MethodGet, "/x/a%2Fb%3Fc", nil))
		if seen != "a/b?c" {
			t.Errorf("id = %q, want %q", seen, "a/b?c")
		}
		if got := rec.Header().Get("Cache-Control"); got != noStore {
			t.Errorf("Cache-Control = %q, want %q", got, noStore)
		}
	})

	t.Run("PG-13 a hostile id reaches the page escaped", func(t *testing.T) {
		const hostile = `<img src=x onerror="window.__xss=1">`
		mux := http.NewServeMux()
		mux.Handle("GET /admin/users/{id}", shellFor(components.AdminUserPage))
		rec := serveHandler(mux, httptest.NewRequest(http.MethodGet, "/admin/users/"+url.PathEscape(hostile+"id"), nil))
		assertBodyLacks(t, rec, "<img src=x")
		assertBodyHas(t, rec, `hx-get="/admin/users/%3Cimg%20src=x%20onerror=%22window.__xss=1%22%3Eid/editor"`)
	})

	t.Run("PG-14 a page that fails to render gets a plain 500 and no fragment headers", func(t *testing.T) {
		rec := serveHandler(shell(failingComponent()), testutil.NewRequest())
		assertStatusCode(t, rec, http.StatusInternalServerError)
		if rec.Header().Get("HX-Retarget") != "" || rec.Header().Get("HX-Reswap") != "" {
			t.Errorf("headers = %v, want no retarget or reswap", rec.Header())
		}
	})
}

func TestRenderHelpers(t *testing.T) {
	t.Run("PG-15 render writes the parts in order", func(t *testing.T) {
		rec := httptest.NewRecorder()
		render(rec, testutil.NewRequest(), textComponent("a"), textComponent("b"), textComponent("c"))
		if got := rec.Body.String(); got != "abc" {
			t.Errorf("body = %q, want %q", got, "abc")
		}
	})

	t.Run("PG-16 render stops at the first part that fails", func(t *testing.T) {
		rec := httptest.NewRecorder()
		render(rec, testutil.NewRequest(), textComponent("a"), failingComponent(), textComponent("c"))
		if got := rec.Body.String(); got != "a" {
			t.Errorf("body = %q, want %q", got, "a")
		}
	})

	t.Run("PG-17 renderAll sets the content type and ends with the cleared banner", func(t *testing.T) {
		rec := httptest.NewRecorder()
		renderAll(rec, testutil.NewRequest(), textComponent("first"), textComponent("second"))
		if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
			t.Errorf("Content-Type = %q, want text/html; charset=utf-8", got)
		}
		body := rec.Body.String()
		const clear = `<div id="page-error" hx-swap-oob="innerHTML"></div>`
		if !strings.HasPrefix(body, "firstsecond") || !strings.HasSuffix(body, clear) {
			t.Errorf("body = %q, want firstsecond then %s", body, clear)
		}
	})

	t.Run("PG-18 renderAll writes only the parts it is given", func(t *testing.T) {
		rec := httptest.NewRecorder()
		renderAll(rec, testutil.NewRequest(), textComponent("part"))
		assertBodyLacks(t, rec, "<html", "<body", "<head")
	})

	t.Run("PG-19 redirectHTMX sets HX-Redirect with an empty 200", func(t *testing.T) {
		rec := httptest.NewRecorder()
		redirectHTMX(rec, "/admin/roles/7")
		assertStatusCode(t, rec, http.StatusOK)
		if got := rec.Header().Get("HX-Redirect"); got != "/admin/roles/7" {
			t.Errorf("HX-Redirect = %q, want /admin/roles/7", got)
		}
		if rec.Body.Len() != 0 {
			t.Errorf("body = %q, want empty", rec.Body.String())
		}
	})

	t.Run("PG-20 nothingToSave tells the status line, keeps the page and clears the banner", func(t *testing.T) {
		rec := httptest.NewRecorder()
		nothingToSave(rec, testutil.NewRequest(), "admin-user-status")
		assertStatusCode(t, rec, http.StatusOK)
		if got := rec.Header().Get("HX-Reswap"); got != "none" {
			t.Errorf("HX-Reswap = %q, want none", got)
		}
		assertBodyHas(t, rec, `<p id="admin-user-status" hx-swap-oob="innerHTML">Nothing to save</p>`, `id="page-error"`)
	})
}

func TestHasPermission(t *testing.T) {
	t.Run("PG-21 a held node matches whole or with a value after a colon", func(t *testing.T) {
		held := []string{"ratelimit:1000", "users.admin", "roles.admin:1"}
		for _, node := range []string{"users.admin", "roles.admin", "ratelimit"} {
			if !hasPermission(held, node) {
				t.Errorf("hasPermission(%v, %q) = false, want true", held, node)
			}
		}
	})

	t.Run("PG-22 a node that only resembles a held one does not match", func(t *testing.T) {
		held := []string{"users.administrator", "xroles.admin", "roles.adminx:1", "Users.Admin", "beenamegenerator|*"}
		for _, node := range []string{"users.admin", "roles.admin", "beenamegenerator.admin", "beenamegenerator"} {
			if hasPermission(held, node) {
				t.Errorf("hasPermission(%v, %q) = true, want false", held, node)
			}
		}
	})

	t.Run("PG-23 no permissions or no node match nothing", func(t *testing.T) {
		if hasPermission(nil, "users.admin") || hasPermission([]string{}, "users.admin") {
			t.Error("an empty list matched")
		}
		if hasPermission([]string{"users.admin"}, "") {
			t.Error("an empty node matched")
		}
	})
}

func TestSecondary(t *testing.T) {
	t.Run("PG-24 no error gives no message and no error", func(t *testing.T) {
		message, err := secondary(nil)
		if message != "" || err != nil {
			t.Errorf("secondary(nil) = %q, %v", message, err)
		}
	})

	t.Run("PG-25 an API error gives its message and no error", func(t *testing.T) {
		message, err := secondary(&apiError{Status: 500, Message: "database down"})
		if message != "database down" || err != nil {
			t.Errorf("secondary = %q, %v", message, err)
		}
		message, err = secondary(fmt.Errorf("load: %w", &apiError{Status: 403, Message: "Forbidden"}))
		if message != "Forbidden" || err != nil {
			t.Errorf("secondary of a wrapped error = %q, %v", message, err)
		}
	})

	t.Run("PG-26 an error that is not an API error gives the generic message", func(t *testing.T) {
		message, err := secondary(errors.New("boom"))
		if message != "Something went wrong" || err != nil {
			t.Errorf("secondary = %q, %v", message, err)
		}
	})

	t.Run("PG-27 a 401 is returned as the error with no message", func(t *testing.T) {
		wrapped := fmt.Errorf("load: %w", errUnauthorized)
		message, err := secondary(wrapped)
		if message != "" || err != wrapped {
			t.Errorf("secondary = %q, %v, want no message and the same error", message, err)
		}
	})
}

func TestPermissionLink(t *testing.T) {
	link := textComponent("LINK")
	serve := func(nodes ...string) *httptest.ResponseRecorder {
		req := testutil.NewRequest(testutil.SessionCookie("jwt-value"))
		return serveHandler(pageRoute(permissionLink(link, nodes...)), req)
	}

	t.Run("PG-28 a held node shows the link after one permissions call that carries the session", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /users/me/permissions", 200, `["ratelimit:5","beenamegenerator.admin:1"]`)
		rec := serve("beenamegenerator.admin")
		assertStatusCode(t, rec, http.StatusOK)
		if got := rec.Body.String(); got != "LINK" {
			t.Errorf("body = %q, want the link", got)
		}
		f.AssertLines(t, "GET /users/me/permissions")
		if cookie := f.Calls()[0].Header.Get("Cookie"); cookie != "session=jwt-value" {
			t.Errorf("Cookie = %q, want session=jwt-value", cookie)
		}
	})

	t.Run("PG-29 any one of several nodes is enough", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /users/me/permissions", 200, `["roles.admin"]`)
		if got := serve("users.admin", "roles.admin").Body.String(); got != "LINK" {
			t.Errorf("body = %q, want the link", got)
		}
	})

	t.Run("PG-30 without a held node the answer is an empty 200", func(t *testing.T) {
		for _, permissions := range []string{`[]`, `["users.admin","roles.admin"]`, `["beenamegenerator|*"]`, `["beenamegenerator.administrator"]`, `["xbeenamegenerator.admin"]`} {
			t.Run(permissions, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("GET /users/me/permissions", 200, permissions)
				rec := serve("beenamegenerator.admin")
				assertStatusCode(t, rec, http.StatusOK)
				if rec.Body.Len() != 0 {
					t.Errorf("body = %q, want empty", rec.Body.String())
				}
			})
		}
	})

	t.Run("PG-31 a failed permissions call is an empty 200 and not a failure response", func(t *testing.T) {
		for name, status := range map[string]int{"500": 500, "403": 403, "401": 401} {
			t.Run(name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.Problem("GET /users/me/permissions", status, "down")
				captureLog(t)
				rec := serve("beenamegenerator.admin")
				assertStatusCode(t, rec, http.StatusOK)
				if rec.Body.Len() != 0 || rec.Header().Get("HX-Redirect") != "" || rec.Header().Get("HX-Retarget") != "" {
					t.Errorf("headers %v, body %q, want an empty answer", rec.Header(), rec.Body.String())
				}
			})
		}
	})
}

func TestRowGone(t *testing.T) {
	t.Run("PG-32 a numeric id gives the directive that deletes the prefixed row", func(t *testing.T) {
		parts := rowGone("admin-permission-", "3541025163146757800")
		if len(parts) != 1 {
			t.Fatalf("parts = %d, want 1", len(parts))
		}
		rec := httptest.NewRecorder()
		render(rec, testutil.NewRequest(), parts...)
		if got, want := rec.Body.String(), `<div hx-swap-oob="delete:#admin-permission-3541025163146757800"></div>`; got != want {
			t.Errorf("body = %q, want %q", got, want)
		}
	})

	t.Run("PG-33 an empty or non-numeric id gives nothing", func(t *testing.T) {
		for _, id := range []string{"", "abc", "12a", "-1", "1 2", `1"><x`, "٣"} {
			if parts := rowGone("row-", id); parts != nil {
				t.Errorf("rowGone(%q) = %d parts, want none", id, len(parts))
			}
		}
	})
}

func TestAdminCardsHandler(t *testing.T) {
	t.Run("PG-34 the cards follow the permissions held", func(t *testing.T) {
		cases := []struct {
			name, permissions string
			users, roles      bool
		}{
			{"users", `["users.admin"]`, true, false},
			{"roles", `["roles.admin"]`, false, true},
			{"both", `["ratelimit:1000","users.admin","roles.admin"]`, true, true},
			{"valued node", `["users.admin:1"]`, true, false},
			{"neither", `["ratelimit:1000"]`, false, false},
			{"lookalikes", `["users.administrator","xroles.admin","roles.adminx:1"]`, false, false},
			{"none", `[]`, false, false},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("GET /users/me/permissions", 200, tc.permissions)
				rec := serveRequest(http.MethodGet, "/admin/cards", nil)
				assertStatusCode(t, rec, http.StatusOK)
				for marker, want := range map[string]bool{
					`id="admin-users-link"`:       tc.users,
					`id="admin-roles-link"`:       tc.roles,
					`id="admin-permissions-link"`: tc.roles,
					`id="admin-denied"`:           !tc.users && !tc.roles,
				} {
					if got := strings.Contains(rec.Body.String(), marker); got != want {
						t.Errorf("%s present = %t, want %t", marker, got, want)
					}
				}
				f.AssertLines(t, "GET /users/me/permissions")
			})
		}
	})

	t.Run("PG-35 a failed permissions call answers with its status and message and no cards", func(t *testing.T) {
		cases := []struct {
			name   string
			answer func(f *fakeapi.FakeAPI)
			status int
			text   string
		}{
			{"refused", func(f *fakeapi.FakeAPI) { f.Problem("GET /users/me/permissions", 403, "You may not") }, 403, "You may not"},
			{"no detail", func(f *fakeapi.FakeAPI) { f.Refuse("GET /users/me/permissions", 500, "") }, 500, "Failed to load your permissions"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				tc.answer(f)
				captureLog(t)
				rec := serveRequest(http.MethodGet, "/admin/cards", nil)
				assertFailure(t, rec, tc.status, tc.text)
				assertBodyLacks(t, rec, "admin-users-link", "admin-denied")
			})
		}
	})

	t.Run("PG-36 an unreachable API answers 502 with the fallback message", func(t *testing.T) {
		fakeapi.PointAPIAtClosed(t)
		captureLog(t)
		assertFailure(t, serveRequest(http.MethodGet, "/admin/cards", nil), http.StatusBadGateway, "Failed to load your permissions")
	})

	t.Run("PG-37 a 401 sends the browser to the login page", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.Problem("GET /users/me/permissions", 401, "sign in")
		assertLoginRedirect(t, serveRequest(http.MethodGet, "/admin/cards", nil))
	})
}
